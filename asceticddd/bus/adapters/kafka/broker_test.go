package kafka_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus/adapters/kafka"
)

// Integration tests for the Kafka adapter. They need a live broker and are
// skipped without one:
//
//	ASCETIC_DDD_TEST_KAFKA_BROKERS=localhost:9092 go test ./asceticddd/bus/adapters/kafka/

func seeds(t *testing.T) []string {
	t.Helper()
	value := os.Getenv("ASCETIC_DDD_TEST_KAFKA_BROKERS")
	if value == "" {
		t.Skip("ASCETIC_DDD_TEST_KAFKA_BROKERS is not set")
	}
	return strings.Split(value, ",")
}

// unique gives each test topics and groups of its own, so runs do not see
// each other's messages.
func unique(name string) string {
	return fmt.Sprintf("%s-%d", name, time.Now().UnixNano())
}

func setup(t *testing.T) *bus.Bus {
	t.Helper()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))
	broker := kafka.New(seeds(t), kafka.WithLogger(quiet), kafka.WithRetryAfter(50*time.Millisecond))
	t.Cleanup(broker.Close)
	b := bus.New(bus.WithLogger(quiet))
	require.NoError(t, b.Register("kafka", broker))
	return b
}

// topic creates a topic with one partition and returns its URI.
func topic(t *testing.T, name string) string {
	t.Helper()
	client, err := kgo.NewClient(kgo.SeedBrokers(seeds(t)...))
	require.NoError(t, err)
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = kadm.NewClient(client).CreateTopic(ctx, 1, 1, nil, name)
	require.NoError(t, err)
	return "kafka://" + name
}

func asText(message bus.Message) (string, error) {
	return string(message.Payload()), nil
}

func collect(t *testing.T, b *bus.Bus, uri string, group string) (<-chan string, bus.Subscription) {
	t.Helper()
	consumer, err := bus.NewConsumer(b, uri, group, asText)
	require.NoError(t, err)
	seen := make(chan string, 256)
	subscription, err := consumer.Subscribe(func(_ context.Context, value string) error {
		seen <- value
		return nil
	})
	require.NoError(t, err)
	return seen, subscription
}

func producer(t *testing.T, b *bus.Bus, uri string) *bus.Producer[string] {
	t.Helper()
	p, err := bus.NewProducer(b, uri, func(value string) bus.Message { return bus.NewMessage([]byte(value)) })
	require.NoError(t, err)
	return p
}

func next(t *testing.T, seen <-chan string) string {
	t.Helper()
	select {
	case value := <-seen:
		return value
	case <-time.After(30 * time.Second):
		t.Fatal("nothing arrived within thirty seconds")
		return ""
	}
}

func TestAMessageGoesFromProducerToConsumer(t *testing.T) {
	b := setup(t)
	uri := topic(t, unique("orders"))
	seen, _ := collect(t, b, uri, unique("billing"))

	require.NoError(t, producer(t, b, uri).Publish(context.Background(), "order-1"))

	assert.Equal(t, "order-1", next(t, seen))
}

func TestEveryGroupReceivesTheMessage(t *testing.T) {
	b := setup(t)
	uri := topic(t, unique("orders"))
	billing, _ := collect(t, b, uri, unique("billing"))
	shipping, _ := collect(t, b, uri, unique("shipping"))

	require.NoError(t, producer(t, b, uri).Publish(context.Background(), "order-1"))

	assert.Equal(t, "order-1", next(t, billing))
	assert.Equal(t, "order-1", next(t, shipping))
}

// Messages with one key arrive in the order they were published, and the
// key and headers arrive with them.
func TestMessagesWithOneKeyKeepTheirOrder(t *testing.T) {
	b := setup(t)
	uri := topic(t, unique("orders"))
	consumer, err := bus.NewConsumer(b, uri, unique("g"), func(m bus.Message) (bus.Message, error) { return m, nil })
	require.NoError(t, err)
	seen := make(chan bus.Message, 256)
	_, err = consumer.Subscribe(func(_ context.Context, m bus.Message) error {
		seen <- m
		return nil
	})
	require.NoError(t, err)
	keyed, err := bus.NewProducer(b, uri+"/order-7", func(value string) bus.Message {
		return bus.NewMessage([]byte(value)).WithHeader("message_id", []byte("id-"+value))
	})
	require.NoError(t, err)

	for i := 0; i < 20; i++ {
		require.NoError(t, keyed.Publish(context.Background(), fmt.Sprint(i)))
	}

	for i := 0; i < 20; i++ {
		select {
		case m := <-seen:
			assert.Equal(t, fmt.Sprint(i), string(m.Payload()))
			assert.Equal(t, []byte("order-7"), m.Key(), "the key comes from the URI")
			id, _ := m.Header("message_id")
			assert.Equal(t, "id-"+fmt.Sprint(i), string(id))
		case <-time.After(30 * time.Second):
			t.Fatalf("message %d did not arrive", i)
		}
	}
}

func TestACancelledSubscriptionReceivesNothingMore(t *testing.T) {
	b := setup(t)
	uri := topic(t, unique("orders"))
	seen, subscription := collect(t, b, uri, unique("g"))
	p := producer(t, b, uri)

	require.NoError(t, p.Publish(context.Background(), "before"))
	assert.Equal(t, "before", next(t, seen))

	subscription.Cancel()
	require.NoError(t, p.Publish(context.Background(), "after"))

	select {
	case value := <-seen:
		t.Fatalf("received %q after cancelling", value)
	case <-time.After(2 * time.Second):
	}
}

// A handler that fails is retried until it succeeds, and the record is
// delivered once as far as the handler's success is concerned.
func TestAFailingHandlerIsRetriedUntilItSucceeds(t *testing.T) {
	b := setup(t)
	uri := topic(t, unique("orders"))
	var attempts atomic.Int64
	consumer, err := bus.NewConsumer(b, uri, unique("flaky"), asText)
	require.NoError(t, err)
	seen := make(chan string, 8)
	_, err = consumer.Subscribe(func(_ context.Context, value string) error {
		if attempts.Add(1) == 1 {
			return errors.New("the first attempt fails on purpose")
		}
		seen <- value
		return nil
	})
	require.NoError(t, err)

	require.NoError(t, producer(t, b, uri).Publish(context.Background(), "retried"))

	assert.Equal(t, "retried", next(t, seen))
	assert.Equal(t, int64(2), attempts.Load())
}

// The offset of a handled record survives the subscription: a new
// subscription of the same group continues after it, and a record whose
// handler never succeeded is delivered again.
func TestOffsetsAreCommittedForHandledRecordsOnly(t *testing.T) {
	b := setup(t)
	uri := topic(t, unique("orders"))
	group := unique("g")
	p := producer(t, b, uri)

	first, subscription := collect(t, b, uri, group)
	require.NoError(t, p.Publish(context.Background(), "handled"))
	assert.Equal(t, "handled", next(t, first))
	subscription.Cancel()

	// A handler that never succeeds: the record stays unacknowledged.
	stuck, err := bus.NewConsumer(b, uri, group, asText)
	require.NoError(t, err)
	seenByStuck := make(chan string, 8)
	stuckSubscription, err := stuck.Subscribe(func(_ context.Context, value string) error {
		seenByStuck <- value
		return errors.New("never")
	})
	require.NoError(t, err)
	require.NoError(t, p.Publish(context.Background(), "unhandled"))
	assert.Equal(t, "unhandled", next(t, seenByStuck))
	stuckSubscription.Cancel()

	// The next member of the group gets the unhandled record, not the handled one.
	again, _ := collect(t, b, uri, group)
	assert.Equal(t, "unhandled", next(t, again))
}
