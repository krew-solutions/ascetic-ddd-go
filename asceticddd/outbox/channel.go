package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

// The outbox as a channel of the bus.
//
// Registered under OutboxScheme as OutboxChannel, the outbox offers two
// things. A transactional producer, from PgOutbox.WireProducer: it publishes
// inside the caller's transaction, and the URI it was built for is the
// destination stored with the row, key included. And a consumer, from the
// bus: a consumer of "outbox://all" in a group runs the dispatcher for that
// consumer group and hands every committed row to the handler as a wire
// message whose "destination" header is the row's URI. A Bridge from that
// consumer to bus.TargetHeader("destination") is the dispatcher process.
//
// Headers become metadata: each header is a string field of the JSONB
// object, so message_id keeps its unique index, and every metadata field
// comes back as a header. The channel name after "outbox://" is not read
// yet: the whole outbox is one channel.

// OutboxScheme is the scheme the outbox is registered under.
const OutboxScheme = "outbox"

// DestinationHeader names the channel a message is sent to.
const DestinationHeader = "destination"

// WithPollInterval is the same outbox whose channel consumer waits interval
// when there is nothing to dispatch.
func (o *PgOutbox) WithPollInterval(interval time.Duration) *PgOutbox {
	copied := *o
	copied.pollInterval = interval
	return &copied
}

// WithLogger is the same outbox reporting through logger.
func (o *PgOutbox) WithLogger(logger *slog.Logger) *PgOutbox {
	copied := *o
	copied.logger = logger
	return &copied
}

// WireProducer is a producer to destination that publishes inside the
// caller's transaction. destination is a URI of another channel,
// "kafka://orders/order-7", stored with the row for the dispatcher; a
// message with a key of its own, published to a destination without one,
// gets the key appended.
func (o *PgOutbox) WireProducer(destination string) bus.TransactionalWireProducer[session.Session] {
	return &outboxProducer{outbox: o, destination: destination}
}

// NewProducer is a typed producer to destination that publishes inside the
// caller's transaction, writing values with encode.
func NewProducer[T any](o *PgOutbox, destination string, encode bus.Encoder[T]) *bus.TransactionalProducer[T, session.Session] {
	return bus.NewTransactionalProducer(o.WireProducer(destination), encode)
}

// Channel is the outbox as a bus adapter: what Bus.Register takes.
func (o *PgOutbox) Channel() *OutboxChannel {
	return &OutboxChannel{outbox: o}
}

// OutboxChannel is the outbox as a bus adapter.
type OutboxChannel struct {
	outbox *PgOutbox
}

// Consumer is a consumer of the outbox channel: the dispatcher for group.
func (c *OutboxChannel) Consumer(uri string, group string) (bus.WireConsumer, error) {
	return &outboxConsumer{outbox: c.outbox, group: group}, nil
}

// Producer is refused: the outbox publishes only inside a transaction; see
// PgOutbox.WireProducer.
func (c *OutboxChannel) Producer(uri string) (bus.WireProducer, error) {
	return nil, fmt.Errorf("%q: the outbox publishes only inside a transaction; use PgOutbox.WireProducer", uri)
}

type outboxProducer struct {
	outbox      *PgOutbox
	destination string
}

func (p *outboxProducer) Publish(s session.Session, message bus.Message) error {
	destination := p.destination
	if _, keyed := bus.KeyOf(destination); !keyed && message.Key() != nil {
		key, err := text(message.Key())
		if err != nil {
			return err
		}
		destination = destination + "/" + key
	}
	metadata, err := metadataOf(message)
	if err != nil {
		return err
	}
	return p.outbox.Publish(s, &OutboxMessage{
		Uri:      destination,
		Payload:  message.Payload(),
		Metadata: metadata,
	})
}

type outboxConsumer struct {
	outbox *PgOutbox
	group  string
}

// Subscribe runs the dispatcher in a goroutine until the subscription is
// cancelled. A batch whose handler fails is rolled back and retried after
// the poll interval; the position of the group moves only past messages
// the handler accepted. Cancel stops the dispatcher between batches and
// waits for it.
func (c *outboxConsumer) Subscribe(handler bus.Handler) (bus.Subscription, error) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	subscriber := func(message *OutboxMessage) error {
		return handler(ctx, wireOf(message))
	}
	go func() {
		defer close(done)
		for {
			err := c.outbox.Run(ctx, subscriber, c.group, "", 0, 1, 1, c.outbox.pollInterval.Seconds())
			if ctx.Err() != nil || err == nil {
				return
			}
			c.outbox.logger.Warn("outbox: dispatch failed, retrying", "group", c.group, "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(c.outbox.pollInterval):
			}
		}
	}()
	return bus.NewSubscription(func() {
		cancel()
		<-done
	}), nil
}

// wireOf is the wire message of a row: payload, key from the destination,
// headers from the metadata plus the destination itself.
func wireOf(row *OutboxMessage) bus.Message {
	message := bus.NewMessage(row.Payload).WithHeader(DestinationHeader, []byte(row.Uri))
	if key, ok := bus.KeyOf(row.Uri); ok {
		message = message.WithKey([]byte(key))
	}
	for _, name := range slices.Sorted(maps.Keys(row.Metadata)) {
		message = message.WithHeader(name, headerText(row.Metadata[name]))
	}
	return message
}

// metadataOf is the metadata of a wire message: one string field per header.
func metadataOf(message bus.Message) (map[string]any, error) {
	headers := message.Headers()
	metadata := make(map[string]any, len(headers))
	for _, header := range headers {
		value, err := text(header.Value)
		if err != nil {
			return nil, err
		}
		metadata[header.Name] = value
	}
	return metadata, nil
}

// headerText is a metadata value as header text: a string as it is,
// anything else as JSON.
func headerText(value any) []byte {
	if s, ok := value.(string); ok {
		return []byte(s)
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return []byte(fmt.Sprint(value))
	}
	return encoded
}

func text(value []byte) (string, error) {
	if !utf8.Valid(value) {
		return "", errors.New("outbox: headers and keys are kept as UTF-8 text")
	}
	return string(value), nil
}
