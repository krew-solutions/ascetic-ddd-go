// Package kafka is the Kafka transport, on franz-go.
//
// A URI "kafka://orders" names the topic "orders". A consumer in a group is
// a member of the Kafka consumer group of that name, so several processes
// in one group share the topic's partitions; the in-memory adapter's
// one-consumer-per-group rule does not apply here.
//
// Delivery is at least once. A record's offset is marked after the handler
// returns and committed by the client in the background, so a crash
// mid-handler redelivers the record on restart. A handler that fails is
// retried until it succeeds: the partition waits, which is what keeps its
// order. A handler that panics is reported and its record is skipped, as is
// a record the consumer cannot decode: a poison message must not stop the
// partition.
//
// Every other client setting, TLS, SASL and tuning, comes from the kgo
// options the broker was built with, so they are the caller's, not this
// package's.
package kafka

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus"
)

const (
	// DefaultSendTimeout is how long a producer waits for the broker to
	// acknowledge a record.
	DefaultSendTimeout = 30 * time.Second
	// DefaultRetryAfter is how long a consumer waits before retrying a
	// handler that failed.
	DefaultRetryAfter = time.Second
)

// Broker is a Kafka cluster, as a client configuration to build consumers
// and producers from.
type Broker struct {
	seeds       []string
	options     []kgo.Opt
	sendTimeout time.Duration
	retryAfter  time.Duration
	logger      *slog.Logger

	mu            sync.Mutex
	producer      *kgo.Client
	subscriptions []bus.Subscription
}

// Option configures a Broker.
type Option func(*Broker)

// WithClientOptions adds kgo options to every client the broker builds:
// TLS, SASL, tuning, the offset to start from.
func WithClientOptions(options ...kgo.Opt) Option {
	return func(b *Broker) { b.options = append(b.options, options...) }
}

// WithSendTimeout makes producers give up on a record after timeout.
func WithSendTimeout(timeout time.Duration) Option {
	return func(b *Broker) { b.sendTimeout = timeout }
}

// WithRetryAfter makes consumers wait interval before retrying a handler
// that failed.
func WithRetryAfter(interval time.Duration) Option {
	return func(b *Broker) { b.retryAfter = interval }
}

// WithLogger makes the broker report through logger instead of slog.Default().
func WithLogger(logger *slog.Logger) Option {
	return func(b *Broker) { b.logger = logger }
}

// New is a broker at seeds ("host:port").
func New(seeds []string, options ...Option) *Broker {
	b := &Broker{
		seeds:       seeds,
		sendTimeout: DefaultSendTimeout,
		retryAfter:  DefaultRetryAfter,
		logger:      slog.Default(),
	}
	for _, option := range options {
		option(b)
	}
	return b
}

// Close cancels every subscription made through the broker, waiting for
// their delivery loops, and closes the producer client.
func (b *Broker) Close() {
	b.mu.Lock()
	subscriptions := b.subscriptions
	b.subscriptions = nil
	producer := b.producer
	b.producer = nil
	b.mu.Unlock()
	for _, subscription := range subscriptions {
		subscription.Cancel()
	}
	if producer != nil {
		producer.Close()
	}
}

func (b *Broker) clientOptions(more ...kgo.Opt) []kgo.Opt {
	options := make([]kgo.Opt, 0, 1+len(b.options)+len(more))
	options = append(options, kgo.SeedBrokers(b.seeds...))
	options = append(options, b.options...)
	return append(options, more...)
}

// producerClient is the one producer client of the broker, made on first use.
func (b *Broker) producerClient() (*kgo.Client, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.producer != nil {
		return b.producer, nil
	}
	client, err := kgo.NewClient(b.clientOptions()...)
	if err != nil {
		return nil, fmt.Errorf("kafka: %w", err)
	}
	b.producer = client
	return client, nil
}

func (b *Broker) track(subscription bus.Subscription) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscriptions = append(b.subscriptions, subscription)
}

// Consumer is a consumer of the topic uri names, in the Kafka consumer group
// of that name.
func (b *Broker) Consumer(uri string, group string) (bus.WireConsumer, error) {
	topic, err := bus.ChannelOf(uri)
	if err != nil {
		return nil, err
	}
	return &consumer{broker: b, uri: uri, topic: topic, group: group}, nil
}

// Producer is a producer to the topic uri names. The key the URI carries is
// given to every record published without one.
func (b *Broker) Producer(uri string) (bus.WireProducer, error) {
	topic, err := bus.ChannelOf(uri)
	if err != nil {
		return nil, err
	}
	var key []byte
	if k, ok := bus.KeyOf(uri); ok {
		key = []byte(k)
	}
	return &producer{broker: b, topic: topic, key: key}, nil
}

type consumer struct {
	broker  *Broker
	uri     string
	topic   string
	group   string
	mu      sync.Mutex
	current bus.Subscription
}

// Subscribe joins the group with a client of its own and starts a delivery
// goroutine; a previous subscription of this consumer is cancelled first.
// Cancel stops the loop, commits the offsets marked so far and leaves the
// group.
func (c *consumer) Subscribe(handler bus.Handler) (bus.Subscription, error) {
	client, err := kgo.NewClient(c.broker.clientOptions(
		kgo.ConsumerGroup(c.group),
		kgo.ConsumeTopics(c.topic),
		kgo.AutoCommitMarks(),
	)...)
	if err != nil {
		return nil, fmt.Errorf("kafka: %w", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go c.deliver(ctx, client, handler, done)
	subscription := bus.NewSubscription(func() {
		cancel()
		<-done
		commit, stop := context.WithTimeout(context.Background(), c.broker.sendTimeout)
		defer stop()
		if err := client.CommitMarkedOffsets(commit); err != nil {
			c.broker.logger.Warn("kafka: committing the marked offsets failed", "uri", c.uri, "group", c.group, "error", err)
		}
		client.Close()
	})

	c.mu.Lock()
	previous := c.current
	c.current = subscription
	c.mu.Unlock()
	if previous != nil {
		previous.Cancel()
	}
	c.broker.track(subscription)
	return subscription, nil
}

func (c *consumer) deliver(ctx context.Context, client *kgo.Client, handler bus.Handler, done chan<- struct{}) {
	defer close(done)
	for {
		fetches := client.PollFetches(ctx)
		if ctx.Err() != nil || fetches.IsClientClosed() {
			return
		}
		for _, fetchError := range fetches.Errors() {
			c.broker.logger.Warn("kafka: receiving failed", "uri", c.uri, "group", c.group,
				"partition", fetchError.Partition, "error", fetchError.Err)
		}
		stopped := false
		fetches.EachRecord(func(record *kgo.Record) {
			if stopped {
				return
			}
			if !c.handle(ctx, handler, record) {
				stopped = true
				return
			}
			client.MarkCommitRecords(record)
		})
		if stopped {
			return
		}
	}
}

// handle runs the handler for record until it succeeds, and reports whether
// the record may be marked: false only when the context ended first.
func (c *consumer) handle(ctx context.Context, handler bus.Handler, record *kgo.Record) bool {
	message := messageOf(record)
	for {
		err, panicked := c.call(ctx, handler, message)
		if panicked {
			c.broker.logger.Warn("kafka: handler panicked, skipping", "uri", c.uri, "group", c.group, "panic", err)
			return true
		}
		if err == nil {
			return true
		}
		c.broker.logger.Warn("kafka: handler failed, retrying", "uri", c.uri, "group", c.group, "error", err)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(c.broker.retryAfter):
		}
	}
}

func (c *consumer) call(ctx context.Context, handler bus.Handler, message bus.Message) (err error, panicked bool) {
	defer func() {
		if r := recover(); r != nil {
			err, panicked = fmt.Errorf("%v", r), true
		}
	}()
	return handler(ctx, message), false
}

func messageOf(record *kgo.Record) bus.Message {
	message := bus.NewMessage(record.Value)
	if len(record.Key) > 0 {
		message = message.WithKey(record.Key)
	}
	for _, header := range record.Headers {
		message = message.WithHeader(header.Key, header.Value)
	}
	return message
}

type producer struct {
	broker *Broker
	topic  string
	key    []byte
}

// Publish sends one record and waits for the broker to acknowledge it,
// within the send timeout.
func (p *producer) Publish(ctx context.Context, message bus.Message) error {
	client, err := p.broker.producerClient()
	if err != nil {
		return err
	}
	key := message.Key()
	if key == nil {
		key = p.key
	}
	headers := message.Headers()
	record := &kgo.Record{Topic: p.topic, Key: key, Value: message.Payload(), Headers: make([]kgo.RecordHeader, len(headers))}
	for i, header := range headers {
		record.Headers[i] = kgo.RecordHeader{Key: header.Name, Value: header.Value}
	}
	ctx, cancel := context.WithTimeout(ctx, p.broker.sendTimeout)
	defer cancel()
	if err := client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("kafka: %w", err)
	}
	return nil
}
