# Transactional Inbox Pattern

Go implementation of the Inbox pattern for reliable incoming message processing with idempotency and causal consistency.

For general documentation and pattern description, see [documentation of Python version](https://krew-solutions.github.io/ascetic-ddd-python/modules/inbox/index.html).

## What is the Inbox Pattern?

The Inbox pattern ensures:
- **Idempotency**: Duplicate messages are automatically ignored
- **Causal consistency**: Messages wait for their dependencies before processing
- **Reliable processing**: Each message is processed in the transaction that marks it processed, so the subscriber's writes and the mark commit together or not at all

## Usage

### Basic Setup

```go
import (
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/inbox"
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
    pgsession "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/pg"
)

// Setup session pool
pool := pgsession.NewSessionPool(pgxPool)

// Create inbox with default URI-based partitioning
inb := inbox.NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

// Create the sequence and the table if they do not exist
err := pool.Session(ctx, func(s session.Session) error {
    return inb.Setup(s)
})
```

### Publishing Messages

```go
message := &inbox.InboxMessage{
    TenantId:       "tenant1",
    StreamType:     "Order",
    StreamId:       map[string]any{"id": "order-123"},
    StreamPosition: 1,
    Uri:            "kafka://orders",
    Payload:        []byte(`{"type": "OrderCreated", "order_id": "123", "amount": 100}`),
    Metadata: map[string]any{
        "message_id": "550e8400-e29b-41d4-a716-446655440001",
    },
}

err := inb.Publish(ctx, message)
```

A message with the same `(tenant_id, stream_type, stream_id, stream_position)` as one already stored is ignored. A `message_id` in the metadata is unique in the table as well.

### Processing Messages

The subscriber runs inside the transaction that marks the message processed, and is given that transaction. Writes made through the session it receives commit with the mark, or not at all. If the subscriber returns an error, the transaction is rolled back, the message stays unprocessed, and `Run` returns the error.

```go
subscriber := func(s session.Session, msg *inbox.InboxMessage) error {
    // writes through s land in the transaction that marks msg processed
    return processMessage(s, msg)
}

// Processes messages until ctx is cancelled or the subscriber fails.
// Waits 0.1 seconds when there is nothing to process.
err := inb.Run(ctx, subscriber, 0, 1, 1, 0.1)
```

`Dispatch` processes a single message and reports whether there was one:

```go
processed, err := inb.Dispatch(ctx, subscriber, 0, 1)
```

### Multiple Workers (Partitioning)

Workers share the messages by the hash of a partition key. Each loop of `Run` takes the messages whose key hashes to its worker:

```
effectiveId    = processId * concurrency + localId
effectiveTotal = numProcesses * concurrency
```

```go
// Process 0 of 2, three loops in this process: workers 0, 1 and 2 of 6
err := inb.Run(ctx, subscriber, 0, 2, 3, 0.1)
```

With causal dependencies, partition by stream (see [Partition Strategies](#partition-strategies)) so that a message and what it depends on land with one worker.

### Causal Dependencies

Messages can specify dependencies that must be processed first:

```go
// First message - creates order
orderCreated := &inbox.InboxMessage{
    TenantId:       "tenant1",
    StreamType:     "Order",
    StreamId:       map[string]any{"id": "order-123"},
    StreamPosition: 1,
    Uri:            "kafka://orders",
    Payload:        []byte(`{"type": "OrderCreated"}`),
}
inb.Publish(ctx, orderCreated)

// Second message - depends on order creation
orderShipped := &inbox.InboxMessage{
    TenantId:       "tenant1",
    StreamType:     "Order",
    StreamId:       map[string]any{"id": "order-123"},
    StreamPosition: 2,
    Uri:            "kafka://shipments",
    Payload:        []byte(`{"type": "OrderShipped"}`),
    Metadata: map[string]any{
        "causal_dependencies": []map[string]any{
            {
                "tenant_id":       "tenant1",
                "stream_type":     "Order",
                "stream_id":       map[string]any{"id": "order-123"},
                "stream_position": 1,
            },
        },
    },
}
inb.Publish(ctx, orderShipped)

// OrderShipped will wait until OrderCreated is processed
```

### Graceful Shutdown

`Run` checks the context between messages and returns `ctx.Err()` once it is cancelled:

```go
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

err := inb.Run(ctx, subscriber, 0, 1, 1, 0.1)
if err != nil && !errors.Is(err, context.Canceled) {
    log.Fatal(err)
}
```

### Retry Logic

A subscriber error rolls its transaction back and `Run` returns the error. The message is still unprocessed and is picked up again by the next `Run`:

```go
for ctx.Err() == nil {
    err := inb.Run(ctx, subscriber, 0, 1, 1, 0.1)
    if err != nil && ctx.Err() == nil {
        log.Printf("inbox: processing failed, retrying: %v", err)
        time.Sleep(time.Second)
    }
}
```

## Complete Example: Kafka Consumer Integration

```go
package main

import (
    "context"
    "errors"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/confluentinc/confluent-kafka-go/v2/kafka"
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/inbox"
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
    pgsession "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/pg"
)

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    // Setup inbox
    pool := pgsession.NewSessionPool(pgxPool)
    inb := inbox.NewInbox(pool, "inbox", "inbox_received_position_seq", nil)
    err := pool.Session(ctx, func(s session.Session) error {
        return inb.Setup(s)
    })
    if err != nil {
        log.Fatal(err)
    }

    // Setup Kafka consumer
    consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
        "bootstrap.servers": "localhost:9092",
        "group.id":          "my-group",
        "auto.offset.reset": "earliest",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer consumer.Close()

    consumer.Subscribe("orders", nil)

    // Intake: read from Kafka and store in the inbox
    go func() {
        for ctx.Err() == nil {
            msg := consumer.ReadMessage(100)
            if msg == nil {
                continue
            }

            inboxMsg := &inbox.InboxMessage{
                TenantId:       "default",
                StreamType:     *msg.TopicPartition.Topic,
                StreamId:       map[string]any{"partition": msg.TopicPartition.Partition},
                StreamPosition: int(msg.TopicPartition.Offset),
                Uri:            "kafka://" + *msg.TopicPartition.Topic,
                Payload:        msg.Value,
            }

            if err := inb.Publish(ctx, inboxMsg); err != nil {
                log.Printf("Failed to publish: %v", err)
                continue
            }

            consumer.CommitMessage(msg)
        }
    }()

    // Processing: each message in the transaction that marks it processed
    err = inb.Run(ctx, processOrder, 0, 1, 1, 0.1)
    if err != nil && !errors.Is(err, context.Canceled) {
        log.Fatal(err)
    }
}

func processOrder(s session.Session, msg *inbox.InboxMessage) error {
    // Your business logic here; writes through s commit with the mark
    log.Printf("Order processed: %s", msg.Payload)
    return nil
}
```

## Partition Strategies

### URI-based (Default)

All messages with the same URI go to the same worker:

```go
inb := inbox.NewInbox(pool, "inbox", "inbox_seq", nil)
// or explicitly:
inb := inbox.NewInbox(pool, "inbox", "inbox_seq", &inbox.UriPartitionKeyStrategy{})
```

### Stream-based

All messages for the same stream go to the same worker (preserves causal order):

```go
inb := inbox.NewInbox(
    pool,
    "inbox",
    "inbox_seq",
    &inbox.StreamPartitionKeyStrategy{},
)
```

## Testing

Run tests:
```bash
go test ./asceticddd/inbox -v
```

Integration tests require PostgreSQL with environment variables:
- `DB_USERNAME` (default: "devel")
- `DB_PASSWORD` (default: "devel")
- `DB_HOST` (default: "localhost")
- `DB_PORT` (default: "5432")
- `DB_DATABASE` (default: "devel_grade")

The inbox and outbox integration tests share one database. When running both packages, pass `-p 1`: the outbox reads only rows whose transaction is older than every open one, so a transaction held by an inbox test hides the outbox test's own rows.

## Database Schema

The inbox pattern uses two database objects:

1. **Sequence**: Tracks received and processed positions
2. **Table**: Stores incoming messages with metadata

Primary key: `(tenant_id, stream_type, stream_id, stream_position)` ensures idempotency.

Messages are ordered by `received_position` and marked as processed by setting `processed_position`.
