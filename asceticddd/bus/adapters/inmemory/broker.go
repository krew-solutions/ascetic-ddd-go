// Package inmemory is the in-memory transport: topics in a process-local
// registry, no network. It is the monolithic counterpart of a broker
// adapter: same surface, same group semantics.
//
// Each Broker is an isolated world: two brokers do not share topics or
// subscribers, so tests run in parallel on brokers of their own.
//
// One consumer per (uri, group) is an invariant: in a monolith each logical
// role is one instance, and a second consumer in the same group is a
// configuration bug. Kafka has no such rule; there a group is several
// instances sharing the work.
//
// Delivery is a goroutine per topic: it takes messages off a bounded queue
// and hands each, in order, to the handler of every group. A handler that
// panics is reported and the next message is delivered; a handler that is
// slow holds the topic, which is the back-pressure the queue exists for.
// Delivery is at most once: a handler that fails is reported and the
// message is not delivered again.
package inmemory

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus"
)

// DefaultCapacity is how many messages a topic queues before producers wait.
const DefaultCapacity = 1024

// Broker is a process-local broker: a registry of topics with a delivery
// goroutine each.
type Broker struct {
	capacity int
	logger   *slog.Logger
	ctx      context.Context
	close    context.CancelFunc
	mu       sync.Mutex
	topics   map[string]*topic
}

// Option configures a Broker.
type Option func(*Broker)

// WithCapacity makes topics queue capacity messages before producers wait.
func WithCapacity(capacity int) Option {
	return func(b *Broker) { b.capacity = max(capacity, 1) }
}

// WithLogger makes the broker report through logger instead of slog.Default().
func WithLogger(logger *slog.Logger) Option {
	return func(b *Broker) { b.logger = logger }
}

// New is a broker whose topics queue DefaultCapacity messages.
func New(options ...Option) *Broker {
	ctx, cancel := context.WithCancel(context.Background())
	b := &Broker{
		capacity: DefaultCapacity,
		logger:   slog.Default(),
		ctx:      ctx,
		close:    cancel,
		topics:   map[string]*topic{},
	}
	for _, option := range options {
		option(b)
	}
	return b
}

// Close stops delivery on every topic and cancels the context every handler
// was given. Publishing afterwards fails with bus.ErrClosed; messages still
// queued are dropped.
func (b *Broker) Close() {
	b.close()
}

type topic struct {
	queue chan bus.Message
	mu    sync.Mutex
	// Joined groups; a nil handler is a group that has not subscribed yet, or
	// has cancelled.
	groups map[string]bus.Handler
}

// topicFor is the topic for uri, its channel whatever key the URI carries,
// started on first use.
func (b *Broker) topicFor(uri string) *topic {
	name := bus.WithoutKey(uri)
	b.mu.Lock()
	defer b.mu.Unlock()
	t, ok := b.topics[name]
	if !ok {
		t = &topic{queue: make(chan bus.Message, b.capacity), groups: map[string]bus.Handler{}}
		b.topics[name] = t
		go b.deliver(name, t)
	}
	return t
}

// deliver is the delivery goroutine of one topic. It ends when the broker
// is closed.
func (b *Broker) deliver(name string, t *topic) {
	for {
		select {
		case <-b.ctx.Done():
			return
		case message := <-t.queue:
			for group, handler := range t.handlers() {
				b.handle(name, group, handler, message.Clone())
			}
		}
	}
}

func (b *Broker) handle(name string, group string, handler bus.Handler, message bus.Message) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Warn("in-memory: handler panicked", "uri", name, "group", group, "panic", r)
		}
	}()
	if err := handler(b.ctx, message); err != nil {
		b.logger.Warn("in-memory: handler failed", "uri", name, "group", group, "error", err)
	}
}

// handlers is a snapshot of the groups that have a handler attached.
func (t *topic) handlers() map[string]bus.Handler {
	t.mu.Lock()
	defer t.mu.Unlock()
	attached := make(map[string]bus.Handler, len(t.groups))
	for group, handler := range t.groups {
		if handler != nil {
			attached[group] = handler
		}
	}
	return attached
}

// Consumer joins group on uri; a second consumer in the same group is
// refused with bus.ErrAlreadyInGroup.
func (b *Broker) Consumer(uri string, group string) (bus.WireConsumer, error) {
	t := b.topicFor(uri)
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, joined := t.groups[group]; joined {
		return nil, fmt.Errorf("%w: %q on %q", bus.ErrAlreadyInGroup, group, bus.WithoutKey(uri))
	}
	t.groups[group] = nil
	return &consumer{topic: t, group: group}, nil
}

// Producer is a producer to uri. The key the URI carries is given to every
// message published without one.
func (b *Broker) Producer(uri string) (bus.WireProducer, error) {
	t := b.topicFor(uri)
	var key []byte
	if k, ok := bus.KeyOf(uri); ok {
		key = []byte(k)
	}
	return &producer{broker: b, topic: t, key: key}, nil
}

type consumer struct {
	topic *topic
	group string
}

// Subscribe attaches handler to the group, replacing the previous one.
func (c *consumer) Subscribe(handler bus.Handler) (bus.Subscription, error) {
	c.topic.mu.Lock()
	c.topic.groups[c.group] = handler
	c.topic.mu.Unlock()
	return bus.NewSubscription(func() {
		c.topic.mu.Lock()
		defer c.topic.mu.Unlock()
		if _, joined := c.topic.groups[c.group]; joined {
			c.topic.groups[c.group] = nil
		}
	}), nil
}

type producer struct {
	broker *Broker
	topic  *topic
	key    []byte
}

// Publish queues the message; it waits while the topic's queue is full.
func (p *producer) Publish(ctx context.Context, message bus.Message) error {
	if p.broker.ctx.Err() != nil {
		return bus.ErrClosed
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if message.Key() == nil && p.key != nil {
		message = message.WithKey(p.key)
	}
	select {
	case <-p.broker.ctx.Done():
		return bus.ErrClosed
	case <-ctx.Done():
		return ctx.Err()
	case p.topic.queue <- message:
		return nil
	}
}
