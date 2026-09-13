package bus_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus"
)

// The transactional producer and consumer pass the caller's transaction
// through to the wire, untouched and unknown to the bus.

// tx stands in for a session: the bus only passes it through.
type tx struct{ name string }

// recording records what it was asked to publish, and in which transaction.
type recording struct {
	seen []struct {
		tx      string
		payload string
	}
}

func (r *recording) Publish(session tx, message bus.Message) error {
	r.seen = append(r.seen, struct {
		tx      string
		payload string
	}{session.name, string(message.Payload())})
	return nil
}

// handing keeps the handler it is given, so the test can call it with
// transactions of its own.
type handing struct {
	handler bus.TransactionalHandler[tx]
}

func (h *handing) Subscribe(handler bus.TransactionalHandler[tx]) (bus.Subscription, error) {
	h.handler = handler
	return bus.NewSubscription(func() {}), nil
}

func TestTheHandlerGetsTheTransactionWithTheDecodedValue(t *testing.T) {
	wire := &handing{}
	consumer := bus.NewTransactionalConsumer(wire, func(message bus.Message) (int, error) {
		return strconv.Atoi(string(message.Payload()))
	})
	var seen []struct {
		tx    string
		order int
	}
	_, err := consumer.Subscribe(func(session tx, order int) error {
		seen = append(seen, struct {
			tx    string
			order int
		}{session.name, order})
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, wire.handler(tx{"tx-1"}, bus.NewMessage([]byte("7"))))
	// undecodable: reported and acknowledged, not an error of the transport
	require.NoError(t, wire.handler(tx{"tx-2"}, bus.NewMessage([]byte("not a number"))))
	require.NoError(t, wire.handler(tx{"tx-3"}, bus.NewMessage([]byte("8"))))

	assert.Equal(t, []struct {
		tx    string
		order int
	}{{"tx-1", 7}, {"tx-3", 8}}, seen)
}

func TestAFailingHandlerFailsTheTransactionalDelivery(t *testing.T) {
	wire := &handing{}
	consumer := bus.NewTransactionalConsumer(wire, func(message bus.Message) (string, error) {
		return string(message.Payload()), nil
	})
	boom := errors.New("boom")
	_, err := consumer.Subscribe(func(tx, string) error { return boom })
	require.NoError(t, err)

	assert.ErrorIs(t, wire.handler(tx{"tx-1"}, bus.NewMessage([]byte("x"))), boom)
}

func TestTheValueIsEncodedAndPublishedInTheGivenTransaction(t *testing.T) {
	wire := &recording{}
	producer := bus.NewTransactionalProducer(wire, func(order int) bus.Message {
		return bus.NewMessage([]byte(strconv.Itoa(order)))
	})

	require.NoError(t, producer.Publish(tx{"tx-1"}, 7))
	require.NoError(t, producer.Publish(tx{"tx-2"}, 8))

	assert.Equal(t, []struct {
		tx      string
		payload string
	}{{"tx-1", "7"}, {"tx-2", "8"}}, wire.seen)
}
