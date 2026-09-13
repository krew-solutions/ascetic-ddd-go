package inbox

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/bus/adapters/inmemory"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/outbox"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

// The inbox as a channel of the bus: a bridge from a broker channel is the
// intake, and the transactional consumer processes each message once, in
// the transaction that marks it.

const handledTable = "inbox_test_handled"

func quietLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// channelFixture is the inbox with a fast poll interval, a table the
// handler writes into, and a cleanup for both.
func channelFixture(t *testing.T) (*PgInbox, session.SessionPool, func()) {
	t.Helper()
	inbox, pool, cleanup := setupInboxIntegrationTest(t)
	inbox = inbox.WithPollInterval(20 * time.Millisecond).WithLogger(quietLogger())
	exec(t, pool, "DROP TABLE IF EXISTS "+handledTable+"; CREATE TABLE "+handledTable+" (payload text NOT NULL)")
	return inbox, pool, func() {
		exec(t, pool, "DROP TABLE IF EXISTS "+handledTable)
		cleanup()
	}
}

func exec(t *testing.T, pool session.SessionPool, sql string) {
	t.Helper()
	err := pool.Session(context.Background(), func(s session.Session) error {
		_, err := s.(session.DbSession).Connection().Exec(sql)
		return err
	})
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func queryInt(t *testing.T, pool session.SessionPool, sql string) int {
	t.Helper()
	var n int
	err := pool.Session(context.Background(), func(s session.Session) error {
		return s.(session.DbSession).Connection().QueryRow(sql).Scan(&n)
	})
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

func queryString(t *testing.T, pool session.SessionPool, sql string) string {
	t.Helper()
	var v string
	err := pool.Session(context.Background(), func(s session.Session) error {
		return s.(session.DbSession).Connection().QueryRow(sql).Scan(&v)
	})
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return v
}

func processed(t *testing.T, pool session.SessionPool) int {
	return queryInt(t, pool, "SELECT count(*) FROM inbox_test WHERE processed_position IS NOT NULL")
}

// processedSoon waits for the mark: the handler reports before it commits.
func processedSoon(t *testing.T, pool session.SessionPool, expected int) int {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		n := processed(t, pool)
		if n == expected || time.Now().After(deadline) {
			return n
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func handled(t *testing.T, pool session.SessionPool) int {
	return queryInt(t, pool, "SELECT count(*) FROM "+handledTable)
}

// consume processes with a handler that writes through the transaction it
// is given and reports the payload.
func consume(t *testing.T, inbox *PgInbox) (<-chan string, bus.Subscription) {
	t.Helper()
	received := make(chan string, 8)
	orders := NewConsumer(inbox, func(m bus.Message) (string, error) { return string(m.Payload()), nil })
	subscription, err := orders.Subscribe(func(tx session.Session, order string) error {
		_, err := tx.(session.DbSession).Connection().Exec("INSERT INTO "+handledTable+" (payload) VALUES ($1)", order)
		if err != nil {
			return err
		}
		received <- order
		return nil
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	return received, subscription
}

// order is a wire message of order 7 with the headers the inbox needs.
func order(payload string, position int, messageId string) bus.Message {
	return bus.NewMessage([]byte(payload)).
		WithKey([]byte("order-7")).
		WithHeader(TenantIdHeader, []byte("t1")).
		WithHeader(StreamTypeHeader, []byte("orders.Order")).
		WithHeader(StreamIdHeader, []byte(`{"id": "7"}`)).
		WithHeader(StreamPositionHeader, []byte(strconv.Itoa(position))).
		WithHeader("message_id", []byte(messageId))
}

func nextOrder(t *testing.T, received <-chan string) string {
	t.Helper()
	select {
	case order := <-received:
		return order
	case <-time.After(20 * time.Second):
		t.Fatal("the message was not processed in time")
		return ""
	}
}

func TestAMessageFromABrokerIsProcessedOnceInTheMarkingTransaction(t *testing.T) {
	inbox, pool, cleanup := channelFixture(t)
	defer cleanup()

	b := bus.New(bus.WithLogger(quietLogger()))
	broker := inmemory.New(inmemory.WithLogger(quietLogger()))
	defer broker.Close()
	if err := b.Register("in-memory", broker); err != nil {
		t.Fatal(err)
	}
	if err := b.Register(InboxScheme, inbox.Channel()); err != nil {
		t.Fatal(err)
	}
	intake, err := bus.NewBridge(b).Run("in-memory://orders", "intake", bus.TargetFixed("inbox://orders"))
	if err != nil {
		t.Fatal(err)
	}
	defer intake.Cancel()
	received, processing := consume(t, inbox)
	defer processing.Cancel()

	producer, err := bus.NewProducer(b, "in-memory://orders", func(m bus.Message) bus.Message { return m })
	if err != nil {
		t.Fatal(err)
	}
	placed := order("placed", 1, "00000000-0000-4000-8000-000000000001")
	ctx := context.Background()
	if err := producer.Publish(ctx, placed); err != nil {
		t.Fatal(err)
	}
	if err := producer.Publish(ctx, placed); err != nil { // delivered twice: the same identity
		t.Fatal(err)
	}

	if got := nextOrder(t, received); got != "placed" {
		t.Fatalf("Expected placed, got %q", got)
	}
	select {
	case again := <-received:
		t.Fatalf("the duplicate was processed again: %q", again)
	case <-time.After(300 * time.Millisecond):
	}
	if n := processedSoon(t, pool, 1); n != 1 {
		t.Errorf("Expected 1 processed message, got %d", n)
	}
	if n := handled(t, pool); n != 1 {
		t.Errorf("Expected the handler's write to commit with the mark once, got %d", n)
	}
	if uri := queryString(t, pool, "SELECT uri FROM inbox_test LIMIT 1"); uri != "inbox://orders/order-7" {
		t.Errorf("Expected uri inbox://orders/order-7, got %q", uri)
	}
}

func TestTheOutboxFeedsTheInboxWithoutABroker(t *testing.T) {
	inbox, pool, cleanup := channelFixture(t)
	defer cleanup()

	ob := outbox.NewOutbox(pool, "inbox_bridge_out", "inbox_bridge_out_offsets", 100).
		WithPollInterval(20 * time.Millisecond).
		WithLogger(quietLogger())
	exec(t, pool, "DROP TABLE IF EXISTS inbox_bridge_out; DROP TABLE IF EXISTS inbox_bridge_out_offsets")
	defer exec(t, pool, "DROP TABLE IF EXISTS inbox_bridge_out; DROP TABLE IF EXISTS inbox_bridge_out_offsets")
	err := pool.Session(context.Background(), func(s session.Session) error {
		return s.Atomic(ob.Setup)
	})
	if err != nil {
		t.Fatal(err)
	}

	b := bus.New(bus.WithLogger(quietLogger()))
	if err := b.Register(outbox.OutboxScheme, ob.Channel()); err != nil {
		t.Fatal(err)
	}
	if err := b.Register(InboxScheme, inbox.Channel()); err != nil {
		t.Fatal(err)
	}
	dispatcher, err := bus.NewBridge(b).Run("outbox://all", "dispatcher", bus.TargetHeader(outbox.DestinationHeader))
	if err != nil {
		t.Fatal(err)
	}
	defer dispatcher.Cancel()
	received, processing := consume(t, inbox)
	defer processing.Cancel()

	shipped := outbox.NewProducer(ob, "inbox://orders/order-7", func(payload string) bus.Message {
		return order(payload, 2, "00000000-0000-4000-8000-000000000002")
	})
	err = pool.Session(context.Background(), func(s session.Session) error {
		return s.Atomic(func(tx session.Session) error {
			return shipped.Publish(tx, "shipped")
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	if got := nextOrder(t, received); got != "shipped" {
		t.Fatalf("Expected shipped, got %q", got)
	}
	if n := processedSoon(t, pool, 1); n != 1 {
		t.Errorf("Expected 1 processed message, got %d", n)
	}
	if n := handled(t, pool); n != 1 {
		t.Errorf("Expected 1 handled row, got %d", n)
	}
	if uri := queryString(t, pool, "SELECT uri FROM inbox_test LIMIT 1"); uri != "inbox://orders/order-7" {
		t.Errorf("Expected the destination the outbox stamped, got %q", uri)
	}
}

// A handler that fails leaves the message unprocessed and its own writes
// rolled back; the loop retries after the poll interval, and the second
// attempt goes through.
func TestAFailingHandlerIsRetriedAndItsWritesAreRolledBack(t *testing.T) {
	inbox, pool, cleanup := channelFixture(t)
	defer cleanup()

	b := bus.New(bus.WithLogger(quietLogger()))
	broker := inmemory.New(inmemory.WithLogger(quietLogger()))
	defer broker.Close()
	if err := b.Register("in-memory", broker); err != nil {
		t.Fatal(err)
	}
	if err := b.Register(InboxScheme, inbox.Channel()); err != nil {
		t.Fatal(err)
	}
	intake, err := bus.NewBridge(b).Run("in-memory://orders", "intake", bus.TargetFixed("inbox://orders"))
	if err != nil {
		t.Fatal(err)
	}
	defer intake.Cancel()

	var attempts atomic.Int64
	received := make(chan string, 8)
	orders := NewConsumer(inbox, func(m bus.Message) (string, error) { return string(m.Payload()), nil })
	processing, err := orders.Subscribe(func(tx session.Session, order string) error {
		_, err := tx.(session.DbSession).Connection().Exec("INSERT INTO "+handledTable+" (payload) VALUES ($1)", order)
		if err != nil {
			return err
		}
		if attempts.Add(1) == 1 {
			return errors.New("the first attempt fails on purpose")
		}
		received <- order
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer processing.Cancel()

	producer, err := bus.NewProducer(b, "in-memory://orders", func(m bus.Message) bus.Message { return m })
	if err != nil {
		t.Fatal(err)
	}
	if err := producer.Publish(context.Background(), order("retried", 3, "00000000-0000-4000-8000-000000000003")); err != nil {
		t.Fatal(err)
	}

	if got := nextOrder(t, received); got != "retried" {
		t.Fatalf("Expected retried, got %q", got)
	}
	if n := processedSoon(t, pool, 1); n != 1 {
		t.Errorf("Expected 1 processed message, got %d", n)
	}
	if n := attempts.Load(); n != 2 {
		t.Errorf("Expected 2 attempts, got %d", n)
	}
	if n := handled(t, pool); n != 1 {
		t.Errorf("Expected the failed attempt's write to be rolled back, got %d handled rows", n)
	}
}
