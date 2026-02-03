# Inbox Pattern

Go implementation of the Inbox pattern for reliable incoming message processing with idempotency and causal consistency.

For general documentation and pattern description, see [Python README](https://github.com/krew-solutions/ascetic-ddd-python/blob/main/ascetic_ddd/inbox/README.md).

## What is the Inbox Pattern?

The Inbox pattern ensures:
- **Idempotency**: Duplicate messages are automatically ignored
- **Causal consistency**: Messages wait for their dependencies before processing
- **Reliable processing**: At-least-once delivery with ordered processing

## Usage

### Basic Setup

```go
import (
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/inbox"
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
    pgxsession "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/pgx"
)

// Setup session pool
pool := pgxsession.NewSessionPool(pgxPool)

// Create inbox with default URI-based partitioning
inb := inbox.NewInbox(pool, "inbox", "inbox_received_position_seq", nil)
inb.Setup()
```

### Publishing Messages

```go
message := &inbox.InboxMessage{
    TenantId:       "tenant1",
    StreamType:     "Order",
    StreamId:       map[string]any{"id": "order-123"},
    StreamPosition: 1,
    Uri:            "kafka://orders",
    Payload: map[string]any{
        "type":     "OrderCreated",
        "order_id": "123",
        "amount":   100,
    },
    Metadata: map[string]any{
        "event_id": "550e8400-e29b-41d4-a716-446655440001",
    },
}

err := inb.Publish(message)
```

### Consuming Messages (Channel API)

The idiomatic Go way using channels:

```go
ctx := context.Background()

// Simple iteration over messages
for sessionMsg := range inb.Messages(ctx, 0, 1, 0.1) {
    // Process message within transaction
    handleMessage(sessionMsg.Session, sessionMsg.Message)
    // Message is automatically marked as processed
}
```

### Callback API (Alternative)

```go
subscriber := func(s session.DbSession, msg *inbox.InboxMessage) error {
    fmt.Printf("Processing: %s\n", msg.Uri)
    return processMessage(s, msg)
}

ctx := context.Background()
err := inb.Run(ctx, subscriber, 0, 1, 1, 0.1)
```

## Complete Examples

### Kafka Consumer Integration

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/confluentinc/confluent-kafka-go/v2/kafka"
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/inbox"
    pgxsession "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/pgx"
)

func main() {
    // Setup inbox
    pool := pgxsession.NewSessionPool(pgxPool)
    inb := inbox.NewInbox(pool, "inbox", "inbox_received_position_seq", nil)
    if err := inb.Setup(); err != nil {
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

    // Process Kafka messages
    ctx, cancel := context.WithCancel(context.Background())
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

    go func() {
        <-sigChan
        cancel()
    }()

    // Read from Kafka and publish to inbox
    go func() {
        for {
            select {
            case <-ctx.Done():
                return
            default:
            }

            msg := consumer.ReadMessage(100)
            if msg == nil {
                continue
            }

            var payload map[string]any
            json.Unmarshal(msg.Value, &payload)

            inboxMsg := &inbox.InboxMessage{
                TenantId:       "default",
                StreamType:     *msg.TopicPartition.Topic,
                StreamId:       map[string]any{"partition": msg.TopicPartition.Partition},
                StreamPosition: int(msg.TopicPartition.Offset),
                Uri:            "kafka://" + *msg.TopicPartition.Topic,
                Payload:        payload,
            }

            if err := inb.Publish(inboxMsg); err != nil {
                log.Printf("Failed to publish: %v", err)
            }

            consumer.CommitMessage(msg)
        }
    }()

    // Process from inbox
    for sessionMsg := range inb.Messages(ctx, 0, 1, 0.1) {
        log.Printf("Processing: %s", sessionMsg.Message.Uri)
        // Process message within transaction
        processOrder(sessionMsg.Session, sessionMsg.Message)
    }
}

func processOrder(s session.DbSession, msg *inbox.InboxMessage) {
    // Your business logic here
    log.Printf("Order processed: %v", msg.Payload)
}
```

### Multiple Workers (Partitioning)

```go
numWorkers := 3

for workerID := 0; workerID < numWorkers; workerID++ {
    go func(id int) {
        // Each worker gets its partition of messages
        for sessionMsg := range inb.Messages(ctx, id, numWorkers, 0.1) {
            fmt.Printf("Worker %d: %s\n", id, sessionMsg.Message.Uri)
            processMessage(sessionMsg.Session, sessionMsg.Message)
        }
    }(workerID)
}
```

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
    Payload:        map[string]any{"type": "OrderCreated"},
}
inb.Publish(orderCreated)

// Second message - depends on order creation
orderShipped := &inbox.InboxMessage{
    TenantId:       "tenant1",
    StreamType:     "Order",
    StreamId:       map[string]any{"id": "order-123"},
    StreamPosition: 2,
    Uri:            "kafka://shipments",
    Payload:        map[string]any{"type": "OrderShipped"},
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
inb.Publish(orderShipped)

// OrderShipped will wait until OrderCreated is processed
```

### Graceful Shutdown

```go
ctx, cancel := context.WithCancel(context.Background())

sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
go func() {
    <-sigChan
    cancel() // Closes the channel automatically
}()

processed := 0
for sessionMsg := range inb.Messages(ctx, 0, 1, 0.1) {
    processMessage(sessionMsg.Session, sessionMsg.Message)
    processed++
}

log.Printf("Gracefully stopped after processing %d messages", processed)
```

### Retry Logic

```go
for sessionMsg := range inb.Messages(ctx, 0, 1, 0.1) {
    maxRetries := 3
    for attempt := 0; attempt < maxRetries; attempt++ {
        err := processMessage(sessionMsg.Session, sessionMsg.Message)
        if err == nil {
            break
        }

        if attempt < maxRetries-1 {
            log.Printf("Retry %d/%d", attempt+1, maxRetries)
            time.Sleep(time.Duration(attempt+1) * time.Second)
        } else {
            log.Printf("Failed after %d retries", maxRetries)
        }
    }
}
```

### Select with Multiple Sources

```go
ch1 := inb1.Messages(ctx, 0, 1, 0.1)
ch2 := inb2.Messages(ctx, 0, 1, 0.1)

for {
    select {
    case sessionMsg, ok := <-ch1:
        if !ok {
            ch1 = nil
            continue
        }
        processOrders(sessionMsg.Session, sessionMsg.Message)

    case sessionMsg, ok := <-ch2:
        if !ok {
            ch2 = nil
            continue
        }
        processUsers(sessionMsg.Session, sessionMsg.Message)

    case <-time.After(5 * time.Second):
        return
    }

    if ch1 == nil && ch2 == nil {
        break
    }
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

## API Comparison

### Channel API (Recommended)

```go
// ✅ Idiomatic Go - like Kafka consumer
for sessionMsg := range inb.Messages(ctx, 0, 1, 0.1) {
    processMessage(sessionMsg.Session, sessionMsg.Message)
}
```

**Advantages:**
- Idiomatic Go pattern
- Works with `select` for multiple sources
- Automatic cleanup via context cancellation
- Easier error handling and retry logic

### Callback API

```go
subscriber := func(s session.DbSession, msg *inbox.InboxMessage) error {
    return processMessage(s, msg)
}
inb.Run(ctx, subscriber, 0, 1, 1, 0.1)
```

**Advantages:**
- Similar to Python async version
- Slightly less boilerplate for simple cases

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

## Database Schema

The inbox pattern uses two database objects:

1. **Sequence**: Tracks received and processed positions
2. **Table**: Stores incoming messages with metadata

Primary key: `(tenant_id, stream_type, stream_id, stream_position)` ensures idempotency.

Messages are ordered by `received_position` and marked as processed by setting `processed_position`.
