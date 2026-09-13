package bus

import (
	"context"
	"fmt"
	"sync"
)

// Target is where a bridge publishes: one fixed URI, or the URI a header
// names.
type Target interface {
	destination(message Message) (string, error)
}

type fixedTarget string

func (t fixedTarget) destination(Message) (string, error) {
	return string(t), nil
}

type headerTarget string

func (t headerTarget) destination(message Message) (string, error) {
	value, ok := message.Header(string(t))
	if !ok {
		return "", fmt.Errorf("bus: no %q header names a destination", string(t))
	}
	return string(value), nil
}

// TargetFixed sends every message to uri.
func TargetFixed(uri string) Target {
	return fixedTarget(uri)
}

// TargetHeader sends each message to the URI in the header of that name, so
// that one channel may feed many.
func TargetHeader(name string) Target {
	return headerTarget(name)
}

// Bridge is a Messaging Bridge: what arrives on one channel is published on
// another, bytes, key and headers untouched. It acknowledges a message only
// after the target accepted it: a failed publish is an error of the handler,
// and the source keeps the message for another try.
//
// An outbox dispatcher is a bridge from the outbox channel to the
// destination each message names; an inbox intake is a bridge from a broker
// channel to the inbox channel.
type Bridge struct {
	bus       *Bus
	mu        sync.Mutex
	producers map[string]*Producer[Message]
}

// NewBridge is a bridge that publishes through b.
func NewBridge(b *Bus) *Bridge {
	return &Bridge{bus: b, producers: map[string]*Producer[Message]{}}
}

// Run starts moving messages from the channel from, as consumer group
// group, to target. It runs until the subscription is cancelled.
func (br *Bridge) Run(from string, group string, target Target) (Subscription, error) {
	consumer, err := NewConsumer(br.bus, from, group, func(message Message) (Message, error) {
		return message, nil
	})
	if err != nil {
		return nil, err
	}
	return consumer.Subscribe(func(ctx context.Context, message Message) error {
		return br.forward(ctx, message, target)
	})
}

func (br *Bridge) forward(ctx context.Context, message Message, target Target) error {
	uri, err := target.destination(message)
	if err != nil {
		return err
	}
	producer, err := br.producer(uri)
	if err != nil {
		return err
	}
	return producer.Publish(ctx, message)
}

// producer is one producer per destination, made on first use.
func (br *Bridge) producer(uri string) (*Producer[Message], error) {
	br.mu.Lock()
	defer br.mu.Unlock()
	if producer, ok := br.producers[uri]; ok {
		return producer, nil
	}
	producer, err := NewProducer(br.bus, uri, func(message Message) Message { return message })
	if err != nil {
		return nil, err
	}
	br.producers[uri] = producer
	return producer, nil
}
