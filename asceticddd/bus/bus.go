// Package bus is a scheme-dispatched message bus: typed producers and
// consumers over opaque wire messages.
//
// The bus is a registry of transports keyed by URI scheme. A call site
// writing NewConsumer(bus, "in-memory://orders", "billing", decode) does not
// name the transport: the scheme picks the adapter registered for it.
// Replacing the transport is one Register call at the composition root, and
// nothing else moves.
//
// A producer or consumer that is transactional by nature, the outbox and
// the inbox, is obtained from its adapter rather than from the registry and
// names the transaction at the call. The bus never sees a session; it passes
// one through.
package bus

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// Bus is a registry of transports by URI scheme. Built once at the
// composition root, then shared. There is no global bus: tests run on buses
// of their own.
type Bus struct {
	mu       sync.RWMutex
	adapters map[string]Adapter
	logger   *slog.Logger
}

// Option configures a Bus.
type Option func(*Bus)

// WithLogger makes the bus report through logger instead of slog.Default().
func WithLogger(logger *slog.Logger) Option {
	return func(b *Bus) { b.logger = logger }
}

// New is a bus with no transports.
func New(options ...Option) *Bus {
	b := &Bus{adapters: map[string]Adapter{}, logger: slog.Default()}
	for _, option := range options {
		option(b)
	}
	return b
}

// Register binds scheme to adapter: every "scheme://…" URI resolves to it.
func (b *Bus) Register(scheme string, adapter Adapter) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, exists := b.adapters[scheme]; exists {
		return fmt.Errorf("%w: %q", ErrAlreadyRegistered, scheme)
	}
	b.adapters[scheme] = adapter
	return nil
}

func (b *Bus) adapter(uri string) (Adapter, error) {
	scheme, err := SchemeOf(uri)
	if err != nil {
		return nil, err
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	adapter, ok := b.adapters[scheme]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownScheme, scheme)
	}
	return adapter, nil
}

// Decoder reads a value off a wire message.
type Decoder[T any] func(message Message) (T, error)

// Encoder writes a value into a wire message.
type Encoder[T any] func(value T) Message

// Consumer is a typed consumer of one (uri, group).
type Consumer[T any] struct {
	uri    string
	group  string
	wire   WireConsumer
	decode Decoder[T]
	logger *slog.Logger
}

// NewConsumer is a consumer of uri in group, reading values with decode.
func NewConsumer[T any](b *Bus, uri string, group string, decode Decoder[T]) (*Consumer[T], error) {
	adapter, err := b.adapter(uri)
	if err != nil {
		return nil, err
	}
	wire, err := adapter.Consumer(uri, group)
	if err != nil {
		return nil, err
	}
	return &Consumer[T]{uri: uri, group: group, wire: wire, decode: decode, logger: b.logger}, nil
}

// Subscribe runs handler for every message, in order, until the
// subscription is cancelled. Subscribing again replaces the handler.
//
// An error from the handler means the message was not handled; a transport
// that can, redelivers it. A message that decode rejects is reported and
// skipped: a poison message must not stop the rest.
func (c *Consumer[T]) Subscribe(handler func(ctx context.Context, value T) error) (Subscription, error) {
	return c.wire.Subscribe(func(ctx context.Context, message Message) error {
		value, err := c.decode(message)
		if err != nil {
			c.logger.Warn("bus: decoding failed", "uri", c.uri, "group", c.group, "error", err)
			return nil
		}
		return handler(ctx, value)
	})
}

// Producer is a typed producer to one URI.
type Producer[T any] struct {
	wire   WireProducer
	encode Encoder[T]
}

// NewProducer is a producer to uri, writing values with encode.
func NewProducer[T any](b *Bus, uri string, encode Encoder[T]) (*Producer[T], error) {
	adapter, err := b.adapter(uri)
	if err != nil {
		return nil, err
	}
	wire, err := adapter.Producer(uri)
	if err != nil {
		return nil, err
	}
	return &Producer[T]{wire: wire, encode: encode}, nil
}

// Publish sends one value. It waits while the transport applies
// back-pressure, and gives up when ctx is done.
func (p *Producer[T]) Publish(ctx context.Context, value T) error {
	return p.wire.Publish(ctx, p.encode(value))
}

// TransactionalProducer is a typed producer that publishes inside the
// caller's transaction. Built once, at the composition root, from the
// adapter that offers it, the outbox, and used with the transaction of the
// moment. The session argument is the guarantee itself, named at the call.
type TransactionalProducer[T, S any] struct {
	wire   TransactionalWireProducer[S]
	encode Encoder[T]
}

// NewTransactionalProducer is a typed producer over a transactional wire
// producer.
func NewTransactionalProducer[T, S any](wire TransactionalWireProducer[S], encode Encoder[T]) *TransactionalProducer[T, S] {
	return &TransactionalProducer[T, S]{wire: wire, encode: encode}
}

// Publish sends one value within session's transaction.
func (p *TransactionalProducer[T, S]) Publish(session S, value T) error {
	return p.wire.Publish(session, p.encode(value))
}

// TransactionalConsumer is a typed consumer that runs the handler inside
// the transport's own transaction. Obtained from the adapter that offers it,
// the inbox. The handler is given the session of the transaction that
// acknowledges the message, so its writes and the acknowledgement commit
// together.
type TransactionalConsumer[T, S any] struct {
	wire   TransactionalWireConsumer[S]
	decode Decoder[T]
	logger *slog.Logger
}

// NewTransactionalConsumer is a typed consumer over a transactional wire
// consumer.
func NewTransactionalConsumer[T, S any](wire TransactionalWireConsumer[S], decode Decoder[T]) *TransactionalConsumer[T, S] {
	return &TransactionalConsumer[T, S]{wire: wire, decode: decode, logger: slog.Default()}
}

// WithLogger is the same consumer reporting through logger.
func (c *TransactionalConsumer[T, S]) WithLogger(logger *slog.Logger) *TransactionalConsumer[T, S] {
	return &TransactionalConsumer[T, S]{wire: c.wire, decode: c.decode, logger: logger}
}

// Subscribe runs handler for every message, with the transaction the
// message is acknowledged in, until the subscription is cancelled.
//
// A message that decode rejects is reported and acknowledged without a
// handler, as Consumer.Subscribe does: a poison message must not stop the
// rest.
func (c *TransactionalConsumer[T, S]) Subscribe(handler func(session S, value T) error) (Subscription, error) {
	return c.wire.Subscribe(func(session S, message Message) error {
		value, err := c.decode(message)
		if err != nil {
			c.logger.Warn("bus: decoding failed", "error", err)
			return nil
		}
		return handler(session, value)
	})
}
