package bus

import (
	"context"
	"sync"
)

// Handler is what a consumer runs for each wire message. An error means the
// message was not handled: a transport that can, redelivers it. ctx is
// cancelled when the transport shuts down.
type Handler func(ctx context.Context, message Message) error

// Subscription is a handle on a subscription. Cancel detaches the handler
// and is idempotent. Nothing cancels a subscription implicitly: one made at
// the composition root lives with the process, and its handle is usually
// discarded.
type Subscription interface {
	Cancel()
}

// NewSubscription is a subscription that Cancel detaches by running cancel
// once.
func NewSubscription(cancel func()) Subscription {
	return &subscription{cancel: cancel}
}

type subscription struct {
	once   sync.Once
	cancel func()
}

func (s *subscription) Cancel() {
	s.once.Do(s.cancel)
}

// Adapter is a transport, bound to a URI scheme by Bus.Register. It deals
// in wire messages only; the typed layer encodes and decodes around it.
type Adapter interface {
	// Consumer is a consumer of uri in group.
	Consumer(uri string, group string) (WireConsumer, error)
	// Producer is a producer to uri.
	Producer(uri string) (WireProducer, error)
}

// WireConsumer is a consumer of wire messages.
type WireConsumer interface {
	// Subscribe starts delivering messages to handler, replacing the
	// previous handler of this consumer if there was one.
	Subscribe(handler Handler) (Subscription, error)
}

// WireProducer is a producer of wire messages.
type WireProducer interface {
	// Publish sends one message. It waits while the transport applies
	// back-pressure, and gives up when ctx is done.
	Publish(ctx context.Context, message Message) error
}

// TransactionalHandler is given, with the message, the session of the
// transaction the message is acknowledged in, so that its writes through
// the session and the acknowledgement commit together.
type TransactionalHandler[S any] func(session S, message Message) error

// TransactionalWireProducer publishes inside a transaction the caller holds,
// as the outbox does. S is whatever the caller's transaction is; the bus does
// not know sessions, it only passes one through.
type TransactionalWireProducer[S any] interface {
	// Publish sends one message within session's transaction: it is
	// committed with the caller's state change, or not at all.
	Publish(session S, message Message) error
}

// TransactionalWireConsumer runs the handler inside a transaction of its own,
// as the inbox does. The handler's writes through the session it is given
// commit with the acknowledgement of the message, or not at all.
type TransactionalWireConsumer[S any] interface {
	// Subscribe starts delivering messages, each with its transaction, to
	// handler.
	Subscribe(handler TransactionalHandler[S]) (Subscription, error)
}
