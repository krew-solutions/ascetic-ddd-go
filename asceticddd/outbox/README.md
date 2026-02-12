# Transactional Outbox Pattern

Go implementation of the Transactional Outbox pattern for reliable message publishing.

For general documentation and pattern description, see [documentation of Python version](https://krew-solutions.github.io/ascetic-ddd-python/modules/outbox/index.html).

## Usage

### Basic Publishing

```go
import (
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/outbox"
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

// Setup
pool, _ := session.NewPgxSessionPool(ctx, "postgres://...")
ob := outbox.NewOutbox(pool, "outbox", "outbox_offsets", 100)
ob.Setup()

// Publish message in transaction
err := pool.Session(ctx, func(s session.Session) error {
    return s.Atomic(func(txSession session.Session) error {
        message := &outbox.OutboxMessage{
            URI: "kafka://orders",
            Payload: map[string]any{
                "type":     "OrderCreated",
                "order_id": "123",
                "amount":   100,
            },
            Metadata: map[string]any{
                "event_id": "550e8400-e29b-41d4-a716-446655440001",
            },
        }
        return ob.Publish(txSession, message)
    })
})
```

### Consuming Messages (Channel API)

The idiomatic Go way using channels:

```go
ctx := context.Background()

// Simple iteration over messages
for message := range ob.Messages(ctx, "my-consumer", "", 0, 1, 0.1) {
    fmt.Printf("Received: %s - %v\n", message.URI, message.Payload)
}
```

### Publishing to Kafka

```go
import (
    "encoding/json"
    "strings"
    "github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

func publishToKafka(ob outbox.Outbox, producer *kafka.Producer) {
    ctx := context.Background()

    for message := range ob.Messages(ctx, "kafka-publisher", "kafka://", 0, 1, 0.1) {
        topic := strings.TrimPrefix(message.URI, "kafka://")
        value, _ := json.Marshal(message.Payload)

        producer.Produce(&kafka.Message{
            TopicPartition: kafka.TopicPartition{
                Topic:     &topic,
                Partition: kafka.PartitionAny,
            },
            Key:   []byte(message.Metadata["event_id"].(string)),
            Value: value,
        }, nil)

        log.Printf("Published to Kafka: %s", topic)
    }
}
```

### Multiple Workers (Partitioning)

```go
numWorkers := 3

for workerID := 0; workerID < numWorkers; workerID++ {
    go func(id int) {
        // Each worker gets its partition of messages
        for message := range ob.Messages(ctx, "workers", "", id, numWorkers, 0.1) {
            fmt.Printf("Worker %d: %s\n", id, message.URI)
            processMessage(message)
        }
    }(workerID)
}
```

### Graceful Shutdown

```go
import (
    "os"
    "os/signal"
    "syscall"
)

ctx, cancel := context.WithCancel(context.Background())

// Handle signals
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
go func() {
    <-sigChan
    cancel() // Closes the channel automatically
}()

processed := 0
for message := range ob.Messages(ctx, "processor", "", 0, 1, 0.1) {
    processMessage(message)
    processed++
}

log.Printf("Gracefully stopped after processing %d messages", processed)
```

### Retry Logic

```go
for message := range ob.Messages(ctx, "processor", "", 0, 1, 0.1) {
    maxRetries := 3
    for attempt := 0; attempt < maxRetries; attempt++ {
        err := processMessage(message)
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
ch1 := outbox.Messages(ctx, "g1", "kafka://orders", 0, 1, 0.1)
ch2 := outbox.Messages(ctx, "g2", "kafka://users", 0, 1, 0.1)

for {
    select {
    case msg, ok := <-ch1:
        if !ok {
            ch1 = nil
            continue
        }
        processOrders(msg)

    case msg, ok := <-ch2:
        if !ok {
            ch2 = nil
            continue
        }
        processUsers(msg)

    case <-time.After(5 * time.Second):
        return
    }

    if ch1 == nil && ch2 == nil {
        break
    }
}
```

### Callback API (Alternative)

If you prefer callback-style:

```go
subscriber := func(msg *outbox.OutboxMessage) error {
    fmt.Printf("Message: %s\n", msg.URI)
    return processMessage(msg)
}

err := ob.Run(
    ctx,
    subscriber,
    "consumer-group",
    "",    // uri filter
    0,     // process id
    1,     // num processes
    3,     // concurrency (3 workers)
    0.1,   // poll interval (100ms)
)
```

## API Comparison

### Channel API (Recommended)

```go
// ✅ Idiomatic Go - like Kafka consumer
for message := range ob.Messages(ctx, "group", "", 0, 1, 0.1) {
    processMessage(message)
}
```

**Advantages:**
- Idiomatic Go pattern (similar to Kafka, database cursors, etc.)
- Works with `select` for multiple sources
- Automatic cleanup via context cancellation
- Easier error handling and retry logic
- Familiar to Go developers

### Callback API

```go
subscriber := func(msg *outbox.OutboxMessage) error {
    return processMessage(msg)
}
ob.Run(ctx, subscriber, "group", "", 0, 1, 1, 0.1)
```

**Advantages:**
- Similar to Python async version
- Slightly less boilerplate for simple cases

## Complete Example with Kafka Integration

```go
package main

import (
    "context"
    "encoding/json"
    "log"
    "os"
    "os/signal"
    "strings"
    "syscall"

    "github.com/confluentinc/confluent-kafka-go/v2/kafka"
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/outbox"
    "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

func main() {
    // Setup database session pool
    pool, err := session.NewPgxSessionPool(
        context.Background(),
        os.Getenv("DATABASE_URL"),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Setup outbox
    ob := outbox.NewOutbox(pool, "outbox", "outbox_offsets", 100)
    if err := ob.Setup(); err != nil {
        log.Fatal(err)
    }

    // Setup Kafka producer
    producer, err := kafka.NewProducer(&kafka.ConfigMap{
        "bootstrap.servers": "localhost:9092",
        "acks":              "all",
        "enable.idempotence": true,
    })
    if err != nil {
        log.Fatal(err)
    }
    defer producer.Close()

    // Handle graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
    go func() {
        <-sigChan
        log.Println("Shutting down...")
        cancel()
    }()

    // Process messages from outbox and publish to Kafka
    for message := range ob.Messages(ctx, "kafka-publisher", "kafka://", 0, 1, 0.1) {
        topic := strings.TrimPrefix(message.URI, "kafka://")
        value, _ := json.Marshal(message.Payload)

        err := producer.Produce(&kafka.Message{
            TopicPartition: kafka.TopicPartition{
                Topic:     &topic,
                Partition: kafka.PartitionAny,
            },
            Key:   []byte(message.Metadata["event_id"].(string)),
            Value: value,
        }, nil)

        if err != nil {
            log.Printf("Failed to publish: %v", err)
            continue
        }

        log.Printf("Published to Kafka topic=%s event_id=%s",
            topic, message.Metadata["event_id"])
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

Integration tests require PostgreSQL.
