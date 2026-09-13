# Transactional Outbox Pattern

Go implementation of the Transactional Outbox pattern for reliable message publishing.

For general documentation and pattern description, see [documentation of Python version](https://krew-solutions.github.io/ascetic-ddd-python/modules/outbox/index.html).

## Usage

### Basic Publishing

```go
import (
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/outbox"
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
    pgsession "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/pg"
)

// Setup
pool := pgsession.NewSessionPool(pgxPool)
ob := outbox.NewOutbox(pool, "outbox", "outbox_offsets", 100)

// Create the tables and indexes if they do not exist
err := pool.Session(ctx, func(s session.Session) error {
    return ob.Setup(s)
})

// Publish inside the business transaction
err = pool.Session(ctx, func(s session.Session) error {
    return s.Atomic(func(txSession session.Session) error {
        // ... save the aggregate through txSession ...
        message := &outbox.OutboxMessage{
            URI:      "kafka://orders",
            Payload:  []byte(`{"type": "OrderCreated", "order_id": "123", "amount": 100}`),
            Metadata: map[string]any{
                "message_id": "550e8400-e29b-41d4-a716-446655440001",
            },
        }
        return ob.Publish(txSession, message)
    })
})
```

Either the state change and the message are committed, or neither is. The `payload` is stored as bytes: serialize it before the outbox, and the dispatcher relays it unchanged. The `message_id` must be a UUID; it is unique in the table.

### Dispatching Messages

The dispatcher reads committed messages in `(transaction_id, position)` order and hands each to the subscriber inside the dispatcher's transaction. The consumer group's position is acknowledged after the batch, so delivery is at least once: a crash before the acknowledgement redelivers the batch, and consumers deduplicate on `metadata.message_id`. If the subscriber returns an error, the batch is rolled back and `Run` returns the error.

```go
subscriber := func(msg *outbox.OutboxMessage) error {
    return sendToBroker(msg)
}

err := ob.Run(
    ctx,
    subscriber,
    "broker", // consumer group: each keeps its own position
    "",       // uri filter: "" is every URI; "kafka://orders" is that URI and everything under it
    0,        // process id
    1,        // num processes
    1,        // concurrency: loops in this process
    0.1,      // poll interval in seconds, when there is nothing to dispatch
)
```

`Dispatch` handles one batch and reports whether there was anything:

```go
dispatched, err := ob.Dispatch(ctx, subscriber, "broker", "", 0, 1)
```

### Publishing to Kafka

A subscriber that publishes to Kafka must wait for the delivery report: the batch is acknowledged only after the subscriber returns, so returning before the broker accepted the message would turn at-least-once into at-most-once.

```go
import (
    "strings"

    "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func publishToKafka(producer *kafka.Producer) outbox.Subscriber {
    return func(msg *outbox.OutboxMessage) error {
        // kafka://orders/order-7: topic "orders", partition key "order-7"
        topic, key, _ := strings.Cut(strings.TrimPrefix(msg.URI, "kafka://"), "/")

        delivery := make(chan kafka.Event, 1)
        err := producer.Produce(&kafka.Message{
            TopicPartition: kafka.TopicPartition{
                Topic:     &topic,
                Partition: kafka.PartitionAny,
            },
            Key:   []byte(key),
            Value: msg.Payload,
        }, delivery)
        if err != nil {
            return err
        }

        report := (<-delivery).(*kafka.Message)
        return report.TopicPartition.Error
    }
}
```

### Multiple Workers (Partitioning)

Workers share a selection by the hash of the URI, so all messages of one full URI go to one worker, in order. Each worker keeps a position of its own: with several workers the consumer group `broker` becomes `broker:0`, `broker:1`, and so on. Each loop of `Run` is one worker:

```
effectiveId    = processId * concurrency + localId
effectiveTotal = numProcesses * concurrency
```

```go
// Process 0 of 2, three loops in this process: workers 0, 1 and 2 of 6
err := ob.Run(ctx, subscriber, "broker", "", 0, 2, 3, 0.1)
```

### Consumer Group Positions

Each `(consumer_group, uri)` keeps its own position: the last acknowledged transaction and the offset within it.

```go
err := pool.Session(ctx, func(s session.Session) error {
    txId, offset, err := ob.GetPosition(s, "broker", "")
    if err != nil {
        return err
    }
    log.Printf("broker is at transaction %d, offset %d", txId, offset)

    // Replay from the beginning
    return ob.SetPosition(s, "broker", "", 0, 0)
})
```

### Graceful Shutdown

`Run` checks the context between batches and returns `ctx.Err()` once it is cancelled:

```go
ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer cancel()

err := ob.Run(ctx, subscriber, "broker", "", 0, 1, 1, 0.1)
if err != nil && !errors.Is(err, context.Canceled) {
    log.Fatal(err)
}
```

### Retry Logic

A subscriber error rolls the batch back and `Run` returns the error. The position has not moved, so the next `Run` delivers the batch again:

```go
for ctx.Err() == nil {
    err := ob.Run(ctx, subscriber, "broker", "", 0, 1, 1, 0.1)
    if err != nil && ctx.Err() == nil {
        log.Printf("outbox: dispatch failed, retrying: %v", err)
        time.Sleep(time.Second)
    }
}
```

## Complete Example with Kafka Integration

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
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/outbox"
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
    pgsession "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/pg"
)

func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()

    // Setup database session pool
    pgxPool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
    if err != nil {
        log.Fatal(err)
    }
    defer pgxPool.Close()
    pool := pgsession.NewSessionPool(pgxPool)

    // Setup outbox
    ob := outbox.NewOutbox(pool, "outbox", "outbox_offsets", 100)
    err = pool.Session(ctx, func(s session.Session) error {
        return ob.Setup(s)
    })
    if err != nil {
        log.Fatal(err)
    }

    // Setup Kafka producer
    producer, err := kafka.NewProducer(&kafka.ConfigMap{
        "bootstrap.servers":  "localhost:9092",
        "acks":               "all",
        "enable.idempotence": true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer producer.Close()

    // Dispatch messages from the outbox to Kafka until shutdown
    err = ob.Run(ctx, publishToKafka(producer), "kafka-publisher", "kafka://", 0, 1, 1, 0.1)
    if err != nil && !errors.Is(err, context.Canceled) {
        log.Fatal(err)
    }

    producer.Flush(1000)
    log.Println("Shutdown complete")
}
```

## Testing

Run tests:
```bash
go test ./asceticddd/outbox -v
```

Integration tests require PostgreSQL. The inbox and outbox integration tests share one database. When running both packages, pass `-p 1`: the outbox reads only rows whose transaction is older than every open one, so a transaction held by an inbox test hides the outbox test's own rows.
