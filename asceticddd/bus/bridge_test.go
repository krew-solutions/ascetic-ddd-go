package bus_test

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus/adapters/inmemory"
)

// Tests for the bridge, over two in-memory brokers: what arrives on "a://"
// is published on "b://".

func twoBrokers(t *testing.T) *bus.Bus {
	t.Helper()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := bus.New(bus.WithLogger(quiet))
	for _, scheme := range []string{"a", "b"} {
		broker := inmemory.New(inmemory.WithLogger(quiet))
		t.Cleanup(broker.Close)
		require.NoError(t, b.Register(scheme, broker))
	}
	return b
}

// collect is a consumer on uri that hands every wire message to a channel.
func collect(t *testing.T, b *bus.Bus, uri string) <-chan bus.Message {
	t.Helper()
	consumer, err := bus.NewConsumer(b, uri, "collector", func(m bus.Message) (bus.Message, error) { return m, nil })
	require.NoError(t, err)
	seen := make(chan bus.Message, 64)
	_, err = consumer.Subscribe(func(_ context.Context, message bus.Message) error {
		seen <- message
		return nil
	})
	require.NoError(t, err)
	return seen
}

func next(t *testing.T, seen <-chan bus.Message) bus.Message {
	t.Helper()
	select {
	case message := <-seen:
		return message
	case <-time.After(time.Second):
		t.Fatal("nothing arrived within a second")
		return bus.Message{}
	}
}

func nothingArrives(seen <-chan bus.Message) bool {
	select {
	case <-seen:
		return false
	case <-time.After(50 * time.Millisecond):
		return true
	}
}

func publish(t *testing.T, b *bus.Bus, uri string, message bus.Message) {
	t.Helper()
	producer, err := bus.NewProducer(b, uri, func(m bus.Message) bus.Message { return m })
	require.NoError(t, err)
	require.NoError(t, producer.Publish(context.Background(), message))
}

// The destination is read from a header, so one channel feeds many; key,
// payload and headers arrive untouched.
func TestABridgeForwardsToTheDestinationAHeaderNames(t *testing.T) {
	b := twoBrokers(t)
	orders := collect(t, b, "b://orders")
	payments := collect(t, b, "b://payments")
	_, err := bus.NewBridge(b).Run("a://outbox", "dispatcher", bus.TargetHeader("destination"))
	require.NoError(t, err)

	publish(t, b, "a://outbox", bus.NewMessage([]byte("order placed")).WithHeader("destination", []byte("b://orders/order-7")))
	publish(t, b, "a://outbox", bus.NewMessage([]byte("paid")).WithHeader("destination", []byte("b://payments")))

	order := next(t, orders)
	assert.Equal(t, []byte("order placed"), order.Payload())
	assert.Equal(t, []byte("order-7"), order.Key(), "the key comes from the destination URI")
	destination, _ := order.Header("destination")
	assert.Equal(t, []byte("b://orders/order-7"), destination)
	assert.Equal(t, []byte("paid"), next(t, payments).Payload())
}

func TestABridgeForwardsToAFixedTarget(t *testing.T) {
	b := twoBrokers(t)
	mirror := collect(t, b, "b://mirror")
	_, err := bus.NewBridge(b).Run("a://events", "mirror", bus.TargetFixed("b://mirror"))
	require.NoError(t, err)

	publish(t, b, "a://events", bus.NewMessage([]byte("x")))

	assert.Equal(t, []byte("x"), next(t, mirror).Payload())
}

// A message without a destination cannot be forwarded: the handler fails,
// and nothing is published anywhere.
func TestAMessageWithoutADestinationIsNotForwarded(t *testing.T) {
	b := twoBrokers(t)
	orders := collect(t, b, "b://orders")
	_, err := bus.NewBridge(b).Run("a://outbox", "dispatcher", bus.TargetHeader("destination"))
	require.NoError(t, err)

	publish(t, b, "a://outbox", bus.NewMessage([]byte("lost")))

	assert.True(t, nothingArrives(orders))
}
