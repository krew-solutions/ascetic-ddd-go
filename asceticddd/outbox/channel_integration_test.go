package outbox

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus/adapters/inmemory"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

// The outbox as a channel of the bus: what is published inside a committed
// transaction reaches its destination through a bridge; what is rolled back
// never leaves the outbox.

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func nextMessage(t *testing.T, seen <-chan bus.Message, within time.Duration) bus.Message {
	t.Helper()
	select {
	case message := <-seen:
		return message
	case <-time.After(within):
		t.Fatal("nothing arrived in time")
		return bus.Message{}
	}
}

func inTransaction(t *testing.T, pool session.SessionPool, work func(tx session.Session) error) error {
	t.Helper()
	return pool.Session(context.Background(), func(s session.Session) error {
		return s.Atomic(work)
	})
}

func TestACommittedMessageCrossesTheBridgeAndARolledBackOneDoesNot(t *testing.T) {
	ob, pool := setupOutbox(t)
	defer dropTables(t, pool)
	ob = ob.WithPollInterval(20 * time.Millisecond).WithLogger(quietLogger())

	b := bus.New(bus.WithLogger(quietLogger()))
	broker := inmemory.New(inmemory.WithLogger(quietLogger()))
	defer broker.Close()
	require.NoError(t, b.Register(OutboxScheme, ob.Channel()))
	require.NoError(t, b.Register("in-memory", broker))

	orders, err := bus.NewConsumer(b, "in-memory://orders", "billing", func(m bus.Message) (bus.Message, error) { return m, nil })
	require.NoError(t, err)
	seen := make(chan bus.Message, 8)
	_, err = orders.Subscribe(func(_ context.Context, message bus.Message) error {
		seen <- message
		return nil
	})
	require.NoError(t, err)

	dispatcher, err := bus.NewBridge(b).Run("outbox://all", "dispatcher", bus.TargetHeader(DestinationHeader))
	require.NoError(t, err)
	defer dispatcher.Cancel()

	producer := NewProducer(ob, "in-memory://orders/order-7", func(order string) bus.Message {
		return bus.NewMessage([]byte(order)).WithHeader("message_id", []byte("00000000-0000-4000-8000-000000000001"))
	})

	// Rolled back: never leaves the outbox.
	rolledBack := inTransaction(t, pool, func(tx session.Session) error {
		require.NoError(t, producer.Publish(tx, "lost"))
		return errors.New("rolled back on purpose")
	})
	require.Error(t, rolledBack)

	// Committed: crosses the bridge.
	require.NoError(t, inTransaction(t, pool, func(tx session.Session) error {
		return producer.Publish(tx, "placed")
	}))

	message := nextMessage(t, seen, 20*time.Second)
	assert.Equal(t, []byte("placed"), message.Payload())
	assert.Equal(t, []byte("order-7"), message.Key())
	destination, _ := message.Header(DestinationHeader)
	assert.Equal(t, []byte("in-memory://orders/order-7"), destination)
	messageId, _ := message.Header("message_id")
	assert.Equal(t, []byte("00000000-0000-4000-8000-000000000001"), messageId)

	select {
	case <-seen:
		t.Fatal("the rolled-back message arrived")
	case <-time.After(200 * time.Millisecond):
	}
}

// A subscriber of the outbox channel that fails leaves the batch
// unacknowledged; it is delivered again after the poll interval.
func TestAFailingSubscriberGetsTheBatchAgain(t *testing.T) {
	ob, pool := setupOutbox(t)
	defer dropTables(t, pool)
	ob = ob.WithPollInterval(20 * time.Millisecond).WithLogger(quietLogger())

	b := bus.New(bus.WithLogger(quietLogger()))
	require.NoError(t, b.Register(OutboxScheme, ob.Channel()))

	var attempts atomic.Int64
	flaky, err := bus.NewConsumer(b, "outbox://all", "flaky", func(m bus.Message) (string, error) { return string(m.Payload()), nil })
	require.NoError(t, err)
	received := make(chan string, 8)
	subscription, err := flaky.Subscribe(func(_ context.Context, order string) error {
		if attempts.Add(1) == 1 {
			return errors.New("the first attempt fails on purpose")
		}
		received <- order
		return nil
	})
	require.NoError(t, err)
	defer subscription.Cancel()

	placed := NewProducer(ob, "in-memory://orders/order-9", func(order string) bus.Message {
		return bus.NewMessage([]byte(order)).WithHeader("message_id", []byte("00000000-0000-4000-8000-000000000009"))
	})
	require.NoError(t, inTransaction(t, pool, func(tx session.Session) error {
		return placed.Publish(tx, "placed")
	}))

	select {
	case delivered := <-received:
		assert.Equal(t, "placed", delivered)
	case <-time.After(20 * time.Second):
		t.Fatal("the batch was not delivered again")
	}
	assert.Equal(t, int64(2), attempts.Load())
}
