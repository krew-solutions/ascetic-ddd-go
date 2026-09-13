package inmemory_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus/adapters/inmemory"
)

func quiet() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func setup(t *testing.T) (*bus.Bus, *inmemory.Broker) {
	t.Helper()
	broker := inmemory.New(inmemory.WithLogger(quiet()))
	t.Cleanup(broker.Close)
	b := bus.New(bus.WithLogger(quiet()))
	require.NoError(t, b.Register("in-memory", broker))
	return b, broker
}

func asText(message bus.Message) (string, error) {
	return string(message.Payload()), nil
}

func text(value string) bus.Message {
	return bus.NewMessage([]byte(value))
}

func consumer(t *testing.T, b *bus.Bus, uri string, group string) *bus.Consumer[string] {
	t.Helper()
	c, err := bus.NewConsumer(b, uri, group, asText)
	require.NoError(t, err)
	return c
}

func producer(t *testing.T, b *bus.Bus, uri string) *bus.Producer[string] {
	t.Helper()
	p, err := bus.NewProducer(b, uri, text)
	require.NoError(t, err)
	return p
}

// collect subscribes and returns where the handler puts what it receives.
func collect[T any](t *testing.T, c *bus.Consumer[T]) (<-chan T, bus.Subscription) {
	t.Helper()
	seen := make(chan T, 256)
	subscription, err := c.Subscribe(func(_ context.Context, value T) error {
		seen <- value
		return nil
	})
	require.NoError(t, err)
	return seen, subscription
}

func next[T any](t *testing.T, seen <-chan T) T {
	t.Helper()
	select {
	case value := <-seen:
		return value
	case <-time.After(time.Second):
		t.Fatal("nothing arrived within a second")
		var zero T
		return zero
	}
}

// nothingArrives gives delivery a chance, then reports whether anything
// arrived.
func nothingArrives[T any](seen <-chan T) bool {
	select {
	case <-seen:
		return false
	case <-time.After(50 * time.Millisecond):
		return true
	}
}

func TestAMessageGoesFromProducerToConsumer(t *testing.T) {
	b, _ := setup(t)
	seen, _ := collect(t, consumer(t, b, "in-memory://orders", "billing"))

	require.NoError(t, producer(t, b, "in-memory://orders").Publish(context.Background(), "order-1"))

	assert.Equal(t, "order-1", next(t, seen))
}

func TestPublishingWithoutAConsumerIsNotAnError(t *testing.T) {
	b, _ := setup(t)

	require.NoError(t, producer(t, b, "in-memory://nobody").Publish(context.Background(), "lost"))
}

func TestEveryGroupReceivesTheMessage(t *testing.T) {
	b, _ := setup(t)
	billing, _ := collect(t, consumer(t, b, "in-memory://orders", "billing"))
	shipping, _ := collect(t, consumer(t, b, "in-memory://orders", "shipping"))

	require.NoError(t, producer(t, b, "in-memory://orders").Publish(context.Background(), "order-1"))

	assert.Equal(t, "order-1", next(t, billing))
	assert.Equal(t, "order-1", next(t, shipping))
}

func TestOneConsumerPerGroup(t *testing.T) {
	b, _ := setup(t)
	consumer(t, b, "in-memory://orders", "billing")

	_, err := bus.NewConsumer(b, "in-memory://orders", "billing", asText)
	assert.ErrorIs(t, err, bus.ErrAlreadyInGroup)
}

func TestTopicsDoNotLeakIntoEachOther(t *testing.T) {
	b, _ := setup(t)
	orders, _ := collect(t, consumer(t, b, "in-memory://orders", "g"))
	payments, _ := collect(t, consumer(t, b, "in-memory://payments", "g"))

	require.NoError(t, producer(t, b, "in-memory://orders").Publish(context.Background(), "order-1"))

	assert.Equal(t, "order-1", next(t, orders))
	assert.True(t, nothingArrives(payments))
}

func TestBrokersDoNotLeakIntoEachOther(t *testing.T) {
	first, _ := setup(t)
	second, _ := setup(t)
	onSecond, _ := collect(t, consumer(t, second, "in-memory://orders", "g"))

	require.NoError(t, producer(t, first, "in-memory://orders").Publish(context.Background(), "order-1"))

	assert.True(t, nothingArrives(onSecond))
}

// The wire is the contract: two consumers of one topic read the same bytes
// as different types.
func TestConsumersOfOneTopicMayDecodeDifferently(t *testing.T) {
	b, _ := setup(t)
	asNumber, err := bus.NewConsumer(b, "in-memory://orders", "counting", func(m bus.Message) (int, error) {
		return strconv.Atoi(string(m.Payload()))
	})
	require.NoError(t, err)
	numbers, _ := collect(t, asNumber)
	texts, _ := collect(t, consumer(t, b, "in-memory://orders", "logging"))

	require.NoError(t, producer(t, b, "in-memory://orders").Publish(context.Background(), "7"))

	assert.Equal(t, 7, next(t, numbers))
	assert.Equal(t, "7", next(t, texts))
}

func TestCancellingTwiceIsHarmless(t *testing.T) {
	b, _ := setup(t)
	_, subscription := collect(t, consumer(t, b, "in-memory://orders", "g"))

	subscription.Cancel()
	subscription.Cancel()
}

func TestACancelledSubscriptionReceivesNothingMore(t *testing.T) {
	b, _ := setup(t)
	seen, subscription := collect(t, consumer(t, b, "in-memory://orders", "g"))
	p := producer(t, b, "in-memory://orders")

	require.NoError(t, p.Publish(context.Background(), "before"))
	assert.Equal(t, "before", next(t, seen))

	subscription.Cancel()
	require.NoError(t, p.Publish(context.Background(), "after"))

	assert.True(t, nothingArrives(seen))
}

// A message the consumer cannot decode is skipped, not fatal.
func TestAnUndecodableMessageIsSkipped(t *testing.T) {
	b, _ := setup(t)
	strict, err := bus.NewConsumer(b, "in-memory://orders", "g", func(m bus.Message) (string, error) {
		if string(m.Payload()) == "bad" {
			return "", errors.New("undecodable")
		}
		return string(m.Payload()), nil
	})
	require.NoError(t, err)
	seen, _ := collect(t, strict)
	p := producer(t, b, "in-memory://orders")

	require.NoError(t, p.Publish(context.Background(), "bad"))
	require.NoError(t, p.Publish(context.Background(), "good"))

	assert.Equal(t, "good", next(t, seen))
}

// A handler that panics loses its message, not the topic.
func TestAPanickingHandlerDoesNotStopDelivery(t *testing.T) {
	b, _ := setup(t)
	seen := make(chan string, 8)
	_, err := consumer(t, b, "in-memory://orders", "g").Subscribe(func(_ context.Context, value string) error {
		if value == "first" {
			panic("on purpose")
		}
		seen <- value
		return nil
	})
	require.NoError(t, err)
	p := producer(t, b, "in-memory://orders")

	require.NoError(t, p.Publish(context.Background(), "first"))
	require.NoError(t, p.Publish(context.Background(), "second"))

	assert.Equal(t, "second", next(t, seen))
}

// A handler that fails does not stop delivery of what follows.
func TestAFailingHandlerDoesNotStopDelivery(t *testing.T) {
	b, _ := setup(t)
	seen := make(chan string, 8)
	_, err := consumer(t, b, "in-memory://orders", "g").Subscribe(func(_ context.Context, value string) error {
		if value == "first" {
			return errors.New("on purpose")
		}
		seen <- value
		return nil
	})
	require.NoError(t, err)
	p := producer(t, b, "in-memory://orders")

	require.NoError(t, p.Publish(context.Background(), "first"))
	require.NoError(t, p.Publish(context.Background(), "second"))

	assert.Equal(t, "second", next(t, seen))
}

// Messages of one topic arrive in the order they were published.
func TestMessagesKeepTheirOrder(t *testing.T) {
	b, _ := setup(t)
	seen, _ := collect(t, consumer(t, b, "in-memory://orders", "g"))
	p := producer(t, b, "in-memory://orders")

	for i := 0; i < 100; i++ {
		require.NoError(t, p.Publish(context.Background(), fmt.Sprint(i)))
	}

	for i := 0; i < 100; i++ {
		assert.Equal(t, fmt.Sprint(i), next(t, seen))
	}
}

// scheme://channel/key: the key does not make a topic of its own, and a
// message published with it carries it.
func TestAKeyInTheUriSelectsTheChannelAndKeysTheMessage(t *testing.T) {
	b, _ := setup(t)
	wire, err := bus.NewConsumer(b, "in-memory://orders", "g", func(m bus.Message) (bus.Message, error) { return m, nil })
	require.NoError(t, err)
	seen, _ := collect(t, wire)
	keyed, err := bus.NewProducer(b, "in-memory://orders/order-7", func(m bus.Message) bus.Message { return m })
	require.NoError(t, err)

	require.NoError(t, keyed.Publish(context.Background(), bus.NewMessage([]byte("placed"))))
	require.NoError(t, keyed.Publish(context.Background(), bus.NewMessage([]byte("paid")).WithKey([]byte("own-key"))))

	placed := next(t, seen)
	assert.Equal(t, []byte("placed"), placed.Payload())
	assert.Equal(t, []byte("order-7"), placed.Key(), "the key comes from the URI")
	paid := next(t, seen)
	assert.Equal(t, []byte("own-key"), paid.Key(), "a message's own key is kept")
}

// Closing the broker stops delivery, cancels the handlers' context and
// refuses further publishing.
func TestCloseStopsDeliveryAndRefusesPublishing(t *testing.T) {
	b, broker := setup(t)
	contexts := make(chan context.Context, 1)
	_, err := consumer(t, b, "in-memory://orders", "g").Subscribe(func(ctx context.Context, _ string) error {
		contexts <- ctx
		return nil
	})
	require.NoError(t, err)
	p := producer(t, b, "in-memory://orders")
	require.NoError(t, p.Publish(context.Background(), "before"))
	ctx := next(t, contexts)
	assert.NoError(t, ctx.Err(), "the handler's context is alive while the broker is open")

	broker.Close()

	assert.ErrorIs(t, p.Publish(context.Background(), "after"), bus.ErrClosed)
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("the handler's context is not cancelled by Close")
	}
}

// A full topic makes the producer wait, and the producer's context bounds
// the wait.
func TestAFullQueueAppliesBackPressureBoundedByTheContext(t *testing.T) {
	broker := inmemory.New(inmemory.WithCapacity(1), inmemory.WithLogger(quiet()))
	t.Cleanup(broker.Close)
	b := bus.New(bus.WithLogger(quiet()))
	require.NoError(t, b.Register("in-memory", broker))
	release := make(chan struct{})
	_, err := consumer(t, b, "in-memory://orders", "g").Subscribe(func(_ context.Context, _ string) error {
		<-release
		return nil
	})
	require.NoError(t, err)
	p := producer(t, b, "in-memory://orders")

	// One is being handled and blocks the topic, one fills the queue.
	require.NoError(t, p.Publish(context.Background(), "handled"))
	require.NoError(t, p.Publish(context.Background(), "queued"))

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	assert.ErrorIs(t, p.Publish(ctx, "one too many"), context.DeadlineExceeded)
	close(release)
}
