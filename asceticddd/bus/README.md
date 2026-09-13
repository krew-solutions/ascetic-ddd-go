# Message Bus

Scheme-dispatched message bus: typed producers and consumers over opaque wire messages, with an in-memory adapter. A port of the `ascetic-ddd-bus` Rust crate, itself a port of `trading.bus` (OCaml).

```go
b := bus.New()
broker := inmemory.New()
defer broker.Close()
b.Register("in-memory", broker)

orders, _ := bus.NewConsumer(b, "in-memory://orders", "billing", decodeOrder)
orders.Subscribe(func(ctx context.Context, order Order) error { return bill(ctx, order) })

producer, _ := bus.NewProducer(b, "in-memory://orders", encodeOrder)
producer.Publish(ctx, order)
```

## Design

The bus is a registry of transports keyed by URI scheme; a call site names the scheme, never the transport, so replacing the in-memory broker with Kafka is one `Register` line at the composition root. The bus carries opaque wire messages: each producer encodes with a function of its own and each consumer decodes with one of its own, so two consumers of one topic may read the same bytes as different types. The wire format is the contract; the types are local to each side.

A URI is `scheme://channel[/key]`. The scheme picks the adapter, the channel is the topic, and what follows is a partition key: `kafka://orders/order-7` is topic `orders`, key `order-7`. A `Message` is bytes plus an optional key plus flat headers; a message published without a key gets the one its producer's URI carries.

Consumers are push: `Subscribe(handler)`, and the handler returns an error when it did not handle the message. What happens then is the adapter's business: the in-memory broker reports it and moves on (at most once); a broker that keeps offsets redelivers (at least once). A message the decoder rejects is reported and skipped by every adapter, so a poison message never stops a topic.

A subscription is cancelled explicitly through `Cancel`, never implicitly: a subscription made at the composition root lives with the process, and its handle is usually discarded.

A producer or consumer that is transactional by nature, the outbox and the inbox, is obtained from its adapter rather than from the registry and names the transaction at the call: `TransactionalProducer.Publish(session, value)` publishes inside the caller's transaction, `TransactionalConsumer.Subscribe(func(session, value) error)` runs the handler inside the transaction that acknowledges the message. The bus never sees a session; it passes one through as a type parameter.

## Bridge

`Bridge` is a Messaging Bridge: a consumer of one channel and a producer of another, with a pass-through handler. It acknowledges a message only after the target accepted it, so delivery across it is at least once.

```go
bridge := bus.NewBridge(b)
// every message goes to one channel
bridge.Run("kafka://orders", "orders-intake", bus.TargetFixed("inbox://orders"))
// each message goes where its header says
bridge.Run("outbox://all", "dispatcher", bus.TargetHeader("destination"))
```

## Adapters

`inmemory.Broker` is the monolithic transport: topics in a process-local registry, a delivery goroutine per topic, one consumer per `(uri, group)`, ordered delivery, a bounded queue of 1024 messages per topic that makes producers wait. A handler that panics loses its message and not the topic. `Close` stops delivery, cancels the context every handler was given, and refuses further publishing.

`kafka.Broker` is the same surface over [franz-go](https://github.com/twmb/franz-go), without cgo. A consumer group of the same name shares the partitions; delivery is at least once: a record's offset is marked after the handler returns and committed in the background, and `Cancel` commits the marked offsets before leaving the group. A handler that fails is retried until it succeeds, so the partition waits and keeps its order; a handler that panics or a message that cannot be decoded is skipped, because a poison message must not stop the partition. TLS, SASL and tuning come from the `kgo` options the broker is built with.

```go
broker := kafka.New([]string{"localhost:9092"}, kafka.WithClientOptions(kgo.DialTLS()))
defer broker.Close()
b.Register("kafka", broker)
```

## Testing

```bash
go test -race ./asceticddd/bus/...

# the Kafka tests need a live broker and are skipped without one
ASCETIC_DDD_TEST_KAFKA_BROKERS=localhost:9092 go test -race ./asceticddd/bus/adapters/kafka/
```
