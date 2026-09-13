package inbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

// The inbox as a channel of the bus.
//
// Registered under InboxScheme as InboxChannel, the inbox offers two things.
// A producer, from the bus: a producer to "inbox://orders" stores each wire
// message under the identity its headers name, so a Bridge from a broker
// channel to bus.TargetFixed("inbox://orders") is the intake. And a
// transactional consumer, from PgInbox.WireConsumer: it runs the handler
// inside the transaction that marks a message processed, and hands it that
// transaction. A bus consumer of "inbox://…" is refused, because through it
// the handler's writes would fall outside that transaction.
//
// Headers become columns: tenant_id, stream_type, stream_id as the JSON text
// of an object, stream_position, and destination for uri: the channel the
// message was sent to, as the outbox stamps it; without one, the inbox
// channel it was published to, key included. Every other header is a field
// of metadata, as text, or structured again when the text is a JSON array
// or object, so causal_dependencies come back as they left. The channel
// name after "inbox://" is not read yet: the whole inbox is one channel.

// InboxScheme is the scheme the inbox is registered under.
const InboxScheme = "inbox"

// The headers that are columns of the inbox: its identity, and the channel
// the message was sent to, which the outbox stamps.
const (
	TenantIdHeader       = "tenant_id"
	StreamTypeHeader     = "stream_type"
	StreamIdHeader       = "stream_id"
	StreamPositionHeader = "stream_position"
	DestinationHeader    = "destination"
)

var columnHeaders = map[string]bool{
	TenantIdHeader:       true,
	StreamTypeHeader:     true,
	StreamIdHeader:       true,
	StreamPositionHeader: true,
	DestinationHeader:    true,
}

// WithPollInterval is the same inbox whose channel consumer waits interval
// when there is nothing to process.
func (i *PgInbox) WithPollInterval(interval time.Duration) *PgInbox {
	copied := *i
	copied.pollInterval = interval
	return &copied
}

// WithLogger is the same inbox reporting through logger.
func (i *PgInbox) WithLogger(logger *slog.Logger) *PgInbox {
	copied := *i
	copied.logger = logger
	return &copied
}

// WireConsumer is a consumer whose handler runs inside the transaction that
// marks each message processed, and is given that transaction.
func (i *PgInbox) WireConsumer() bus.TransactionalWireConsumer[session.Session] {
	return &inboxConsumer{inbox: i}
}

// NewConsumer is a typed consumer whose handler runs inside the transaction
// that marks each message processed, reading values with decode.
func NewConsumer[T any](i *PgInbox, decode bus.Decoder[T]) *bus.TransactionalConsumer[T, session.Session] {
	return bus.NewTransactionalConsumer(i.WireConsumer(), decode)
}

// Channel is the inbox as a bus adapter: what Bus.Register takes.
func (i *PgInbox) Channel() *InboxChannel {
	return &InboxChannel{inbox: i}
}

// InboxChannel is the inbox as a bus adapter.
type InboxChannel struct {
	inbox *PgInbox
}

// Consumer is refused: the inbox hands the handler its transaction, which a
// bus consumer cannot; see PgInbox.WireConsumer.
func (c *InboxChannel) Consumer(uri string, group string) (bus.WireConsumer, error) {
	return nil, fmt.Errorf("%q: the inbox hands the handler its transaction; use PgInbox.WireConsumer", uri)
}

// Producer is a producer into the inbox: the intake of uri.
func (c *InboxChannel) Producer(uri string) (bus.WireProducer, error) {
	return &inboxProducer{inbox: c.inbox, uri: uri}, nil
}

type inboxProducer struct {
	inbox *PgInbox
	uri   string
}

// Publish stores the message in a transaction of its own; the same identity
// again is ignored.
func (p *inboxProducer) Publish(ctx context.Context, message bus.Message) error {
	row, err := inboxMessageOf(p.uri, message)
	if err != nil {
		return err
	}
	return p.inbox.Publish(ctx, row)
}

type inboxConsumer struct {
	inbox *PgInbox
}

// Subscribe runs the processing loop in a goroutine until the subscription
// is cancelled. A message whose handler fails is left unprocessed and
// retried after the poll interval. Cancel stops the loop between messages
// and waits for it.
func (c *inboxConsumer) Subscribe(handler bus.TransactionalHandler[session.Session]) (bus.Subscription, error) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	subscriber := func(s session.Session, row *InboxMessage) error {
		return handler(s, wireOf(row))
	}
	go func() {
		defer close(done)
		for {
			err := c.inbox.Run(ctx, subscriber, 0, 1, 1, c.inbox.pollInterval.Seconds())
			if ctx.Err() != nil || err == nil {
				return
			}
			c.inbox.logger.Warn("inbox: processing failed, retrying", "error", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(c.inbox.pollInterval):
			}
		}
	}()
	return bus.NewSubscription(func() {
		cancel()
		<-done
	}), nil
}

// inboxMessageOf is the row a wire message becomes, published to channel.
func inboxMessageOf(channel string, message bus.Message) (*InboxMessage, error) {
	column := func(name string) (string, error) {
		value, ok := message.Header(name)
		if !ok {
			return "", fmt.Errorf("inbox: the inbox needs a %q header", name)
		}
		return text(value)
	}
	tenantId, err := column(TenantIdHeader)
	if err != nil {
		return nil, err
	}
	streamType, err := column(StreamTypeHeader)
	if err != nil {
		return nil, err
	}
	streamIdText, err := column(StreamIdHeader)
	if err != nil {
		return nil, err
	}
	var streamId map[string]any
	if err := json.Unmarshal([]byte(streamIdText), &streamId); err != nil {
		return nil, fmt.Errorf("inbox: %q is not a JSON object: %w", StreamIdHeader, err)
	}
	positionText, err := column(StreamPositionHeader)
	if err != nil {
		return nil, err
	}
	position, err := strconv.Atoi(positionText)
	if err != nil {
		return nil, fmt.Errorf("inbox: %q is not an integer: %w", StreamPositionHeader, err)
	}

	var uri string
	if destination, ok := message.Header(DestinationHeader); ok {
		if uri, err = text(destination); err != nil {
			return nil, err
		}
	} else {
		key, keyed := bus.KeyOf(channel)
		if message.Key() != nil {
			if key, err = text(message.Key()); err != nil {
				return nil, err
			}
			keyed = true
		}
		uri = channel
		if keyed {
			uri = bus.WithoutKey(channel) + "/" + key
		}
	}

	metadata := map[string]any{}
	for _, header := range message.Headers() {
		if columnHeaders[header.Name] {
			continue
		}
		value, err := text(header.Value)
		if err != nil {
			return nil, err
		}
		metadata[header.Name] = structured(value)
	}

	return &InboxMessage{
		TenantId:       tenantId,
		StreamType:     streamType,
		StreamId:       streamId,
		StreamPosition: position,
		Uri:            uri,
		Payload:        message.Payload(),
		Metadata:       metadata,
	}, nil
}

// wireOf is the wire message of a row: payload, key from the URI, the
// columns and the metadata as headers.
func wireOf(row *InboxMessage) bus.Message {
	streamId, _ := json.Marshal(row.StreamId)
	message := bus.NewMessage(row.Payload).
		WithHeader(TenantIdHeader, []byte(fmt.Sprint(row.TenantId))).
		WithHeader(StreamTypeHeader, []byte(row.StreamType)).
		WithHeader(StreamIdHeader, streamId).
		WithHeader(StreamPositionHeader, []byte(strconv.Itoa(row.StreamPosition))).
		WithHeader(DestinationHeader, []byte(row.Uri))
	if key, ok := bus.KeyOf(row.Uri); ok {
		message = message.WithKey([]byte(key))
	}
	for _, name := range slices.Sorted(maps.Keys(row.Metadata)) {
		message = message.WithHeader(name, headerText(row.Metadata[name]))
	}
	return message
}

// structured is a header's text as a metadata field: structured again when
// it is a JSON array or object, text otherwise.
func structured(value string) any {
	var decoded any
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		return value
	}
	switch decoded.(type) {
	case []any, map[string]any:
		return decoded
	default:
		return value
	}
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
		return "", errors.New("inbox: headers and keys are kept as UTF-8 text")
	}
	return string(value), nil
}
