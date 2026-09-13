package inbox

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/identitymap"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/signals"
)

// Mock implementations

type mockRow struct {
	values []any
	err    error
}

func (m *mockRow) Err() error {
	return m.err
}

func (m *mockRow) Scan(dest ...any) error {
	if m.err != nil {
		return m.err
	}
	for i, val := range m.values {
		if i >= len(dest) {
			break
		}
		switch d := dest[i].(type) {
		case *string:
			*d = val.(string)
		case *int:
			*d = val.(int)
		case *int64:
			if v, ok := val.(int64); ok {
				*d = v
			} else if v, ok := val.(int); ok {
				*d = int64(v)
			}
		case **int64:
			if val == nil {
				*d = nil
			} else if v, ok := val.(int64); ok {
				*d = &v
			}
		case *[]byte:
			*d = val.([]byte)
		}
	}
	return nil
}

type mockResult struct{}

func (m *mockResult) LastInsertId() (int64, error) { return 0, nil }
func (m *mockResult) RowsAffected() (int64, error) { return 0, nil }

type mockConnection struct {
	execFunc     func(query string, args ...any) (session.Result, error)
	queryRowFunc func(query string, args ...any) session.Row
}

func (m *mockConnection) Exec(query string, args ...any) (session.Result, error) {
	if m.execFunc != nil {
		return m.execFunc(query, args...)
	}
	return &mockResult{}, nil
}

func (m *mockConnection) Query(query string, args ...any) (session.Rows, error) {
	return nil, nil
}

func (m *mockConnection) QueryRow(query string, args ...any) session.Row {
	if m.queryRowFunc != nil {
		return m.queryRowFunc(query, args...)
	}
	return &mockRow{err: &noRowsError{}}
}

type noRowsError struct{}

func (e *noRowsError) Error() string {
	return "no rows in result set"
}

type mockDbSession struct {
	connection     *mockConnection
	identityMap    *identitymap.IdentityMap
	onStarted      signals.Signal[session.SessionScopeStartedEvent]
	onEnded        signals.Signal[session.SessionScopeEndedEvent]
	onQueryStarted signals.Signal[session.QueryStartedEvent]
	onQueryEnded   signals.Signal[session.QueryEndedEvent]
}

func (m *mockDbSession) Context() context.Context {
	return context.Background()
}

func (m *mockDbSession) Connection() session.DbConnection {
	return m.connection
}

func (m *mockDbSession) Atomic(callback session.SessionCallback) error {
	return callback(m)
}

func (m *mockDbSession) IdentityMap() *identitymap.IdentityMap {
	if m.identityMap == nil {
		m.identityMap = identitymap.New(100, identitymap.ReadUncommitted)
	}
	return m.identityMap
}

func (m *mockDbSession) OnAtomicStarted() signals.Signal[session.SessionScopeStartedEvent] {
	if m.onStarted == nil {
		m.onStarted = signals.NewSignal[session.SessionScopeStartedEvent]()
	}
	return m.onStarted
}

func (m *mockDbSession) OnAtomicEnded() signals.Signal[session.SessionScopeEndedEvent] {
	if m.onEnded == nil {
		m.onEnded = signals.NewSignal[session.SessionScopeEndedEvent]()
	}
	return m.onEnded
}

func (m *mockDbSession) OnQueryStarted() signals.Signal[session.QueryStartedEvent] {
	if m.onQueryStarted == nil {
		m.onQueryStarted = signals.NewSignal[session.QueryStartedEvent]()
	}
	return m.onQueryStarted
}

func (m *mockDbSession) OnQueryEnded() signals.Signal[session.QueryEndedEvent] {
	if m.onQueryEnded == nil {
		m.onQueryEnded = signals.NewSignal[session.QueryEndedEvent]()
	}
	return m.onQueryEnded
}

type mockSessionPool struct {
	session          *mockDbSession
	onSessionStarted signals.Signal[session.SessionScopeStartedEvent]
	onSessionEnded   signals.Signal[session.SessionScopeEndedEvent]
}

func (m *mockSessionPool) Session(ctx context.Context, callback session.SessionPoolCallback) error {
	return callback(m.session)
}

func (m *mockSessionPool) OnSessionStarted() signals.Signal[session.SessionScopeStartedEvent] {
	if m.onSessionStarted == nil {
		m.onSessionStarted = signals.NewSignal[session.SessionScopeStartedEvent]()
	}
	return m.onSessionStarted
}

func (m *mockSessionPool) OnSessionEnded() signals.Signal[session.SessionScopeEndedEvent] {
	if m.onSessionEnded == nil {
		m.onSessionEnded = signals.NewSignal[session.SessionScopeEndedEvent]()
	}
	return m.onSessionEnded
}

func (m *mockSessionPool) Close() {}

// Tests

func TestPublishInsertsMessage(t *testing.T) {
	var executedSql string
	var executedArgs []any

	conn := &mockConnection{
		execFunc: func(query string, args ...any) (session.Result, error) {
			executedSql = query
			executedArgs = args
			return &mockResult{}, nil
		},
	}

	sess := &mockDbSession{connection: conn}
	pool := &mockSessionPool{session: sess}

	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	message := &InboxMessage{
		TenantId:       "tenant1",
		StreamType:     "Order",
		StreamId:       map[string]any{"id": "order-123"},
		StreamPosition: 1,
		Uri:            "kafka://orders",
		Payload:        jsonPayload(map[string]any{"type": "OrderCreated", "amount": 100}),
		Metadata:       map[string]any{"message_id": "uuid-123"},
	}

	err := inbox.Publish(context.Background(), message)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if executedSql == "" {
		t.Fatal("No SQL was executed")
	}

	if len(executedArgs) != 7 {
		t.Fatalf("Expected 7 args, got %d", len(executedArgs))
	}

	if executedArgs[0] != "tenant1" {
		t.Errorf("Expected tenant_id=tenant1, got %v", executedArgs[0])
	}

	if executedArgs[1] != "Order" {
		t.Errorf("Expected stream_type=Order, got %v", executedArgs[1])
	}
}

func TestDispatchReturnsFalseWhenNoMessages(t *testing.T) {
	conn := &mockConnection{
		queryRowFunc: func(query string, args ...any) session.Row {
			return &mockRow{err: &noRowsError{}}
		},
	}

	sess := &mockDbSession{connection: conn}
	pool := &mockSessionPool{session: sess}

	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	var handled []*InboxMessage
	subscriber := func(s session.Session, msg *InboxMessage) error {
		handled = append(handled, msg)
		return nil
	}

	result, err := inbox.Dispatch(context.Background(), subscriber, 0, 1)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if result {
		t.Error("Expected dispatch to return false when no messages")
	}

	if len(handled) != 0 {
		t.Errorf("Expected 0 handled messages, got %d", len(handled))
	}
}

func TestDispatchProcessesMessageWithoutDependencies(t *testing.T) {
	streamId := map[string]any{"id": "order-123"}
	streamIdBytes, _ := json.Marshal(streamId)

	payload := map[string]any{"type": "OrderCreated", "amount": 100}
	payloadBytes, _ := json.Marshal(payload)

	receivedPos := int64(1)

	conn := &mockConnection{
		queryRowFunc: func(query string, args ...any) session.Row {
			// First call: fetch message
			return &mockRow{
				values: []any{
					"tenant1",
					"Order",
					streamIdBytes,
					1,
					"kafka://orders",
					payloadBytes,
					[]byte{},
					receivedPos,
					nil,
				},
			}
		},
		execFunc: func(query string, args ...any) (session.Result, error) {
			// Mark as processed
			return &mockResult{}, nil
		},
	}

	sess := &mockDbSession{connection: conn}
	pool := &mockSessionPool{session: sess}

	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	var handled []*InboxMessage
	subscriber := func(s session.Session, msg *InboxMessage) error {
		handled = append(handled, msg)
		return nil
	}

	result, err := inbox.Dispatch(context.Background(), subscriber, 0, 1)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}

	if !result {
		t.Error("Expected dispatch to return true")
	}

	if len(handled) != 1 {
		t.Fatalf("Expected 1 handled message, got %d", len(handled))
	}

	if handled[0].TenantId != "tenant1" {
		t.Errorf("Expected tenant_id=tenant1, got %s", handled[0].TenantId)
	}
}

func TestDependenciesSatisfiedWhenEmpty(t *testing.T) {
	message := &InboxMessage{
		TenantId:       "tenant1",
		StreamType:     "Order",
		StreamId:       map[string]any{"id": "order-123"},
		StreamPosition: 1,
		Uri:            "kafka://orders",
		Payload:        jsonPayload(map[string]any{"type": "OrderCreated"}),
		Metadata:       nil,
	}

	conn := &mockConnection{}
	sess := &mockDbSession{connection: conn}
	pool := &mockSessionPool{session: sess}

	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	satisfied, err := inbox.areDependenciesSatisfied(sess, message)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if !satisfied {
		t.Error("Expected dependencies to be satisfied when empty")
	}
}

func TestDependencySatisfiedWhenProcessed(t *testing.T) {
	conn := &mockConnection{
		queryRowFunc: func(query string, args ...any) session.Row {
			return &mockRow{values: []any{1}}
		},
	}

	sess := &mockDbSession{connection: conn}
	pool := &mockSessionPool{session: sess}

	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	dep := map[string]any{
		"tenant_id":       "tenant1",
		"stream_type":     "User",
		"stream_id":       map[string]any{"id": "user-1"},
		"stream_position": 5,
	}

	processed, err := inbox.isDependencyProcessed(sess, dep)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if !processed {
		t.Error("Expected dependency to be satisfied")
	}
}

func TestDependencyNotSatisfiedWhenMissing(t *testing.T) {
	conn := &mockConnection{
		queryRowFunc: func(query string, args ...any) session.Row {
			return &mockRow{err: &noRowsError{}}
		},
	}

	sess := &mockDbSession{connection: conn}
	pool := &mockSessionPool{session: sess}

	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	dep := map[string]any{
		"tenant_id":       "tenant1",
		"stream_type":     "User",
		"stream_id":       map[string]any{"id": "user-1"},
		"stream_position": 5,
	}

	processed, err := inbox.isDependencyProcessed(sess, dep)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}

	if processed {
		t.Error("Expected dependency to not be satisfied")
	}
}

func TestSetupCreatesSequenceAndTable(t *testing.T) {
	var executedSqls []string

	conn := &mockConnection{
		execFunc: func(query string, args ...any) (session.Result, error) {
			executedSqls = append(executedSqls, query)
			return &mockResult{}, nil
		},
	}

	sess := &mockDbSession{connection: conn}
	pool := &mockSessionPool{session: sess}

	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	err := inbox.Setup(sess)
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	if len(executedSqls) != 2 {
		t.Fatalf("Expected 2 SQL statements, got %d", len(executedSqls))
	}

	// First should create sequence
	if executedSqls[0] == "" {
		t.Error("First SQL should create sequence")
	}

	// Second should create table
	if executedSqls[1] == "" {
		t.Error("Second SQL should create table")
	}
}

func TestRunSingleWorkerProcessesMessages(t *testing.T) {
	streamId := map[string]any{"id": "order-123"}
	streamIdBytes, _ := json.Marshal(streamId)

	payload := map[string]any{"type": "OrderCreated", "amount": 100}
	payloadBytes, _ := json.Marshal(payload)

	receivedPos := int64(1)

	callCount := 0
	conn := &mockConnection{
		queryRowFunc: func(query string, args ...any) session.Row {
			callCount++
			if callCount == 1 {
				return &mockRow{
					values: []any{
						"tenant1",
						"Order",
						streamIdBytes,
						1,
						"kafka://orders",
						payloadBytes,
						[]byte{},
						receivedPos,
						nil,
					},
				}
			}
			return &mockRow{err: &noRowsError{}}
		},
		execFunc: func(query string, args ...any) (session.Result, error) {
			return &mockResult{}, nil
		},
	}

	sess := &mockDbSession{connection: conn}
	pool := &mockSessionPool{session: sess}

	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	var handled []*InboxMessage
	subscriber := func(s session.Session, msg *InboxMessage) error {
		handled = append(handled, msg)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = inbox.Run(ctx, subscriber, 0, 1, 1, 0.01)

	if len(handled) != 1 {
		t.Errorf("Expected 1 handled message, got %d", len(handled))
	}
}

func TestRunMultipleWorkersSpawnsTasks(t *testing.T) {
	streamId := func(i int) map[string]any {
		return map[string]any{"id": "order-" + string(rune('0'+i))}
	}

	var callCount atomic.Int64 // shared by the workers
	conn := &mockConnection{
		queryRowFunc: func(query string, args ...any) session.Row {
			n := int(callCount.Add(1))
			if n <= 4 {
				id := streamId(n - 1)
				streamIdBytes, _ := json.Marshal(id)
				payload := map[string]any{"type": "OrderCreated", "amount": 100}
				payloadBytes, _ := json.Marshal(payload)
				receivedPos := int64(n - 1)

				return &mockRow{
					values: []any{
						"tenant1",
						"Order",
						streamIdBytes,
						n - 1,
						"kafka://orders",
						payloadBytes,
						[]byte{},
						receivedPos,
						nil,
					},
				}
			}
			return &mockRow{err: &noRowsError{}}
		},
		execFunc: func(query string, args ...any) (session.Result, error) {
			return &mockResult{}, nil
		},
	}

	sess := &mockDbSession{connection: conn}
	pool := &mockSessionPool{session: sess}

	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	var (
		mu      sync.Mutex // the workers append concurrently
		handled []*InboxMessage
	)
	subscriber := func(s session.Session, msg *InboxMessage) error {
		mu.Lock()
		defer mu.Unlock()
		handled = append(handled, msg)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = inbox.Run(ctx, subscriber, 0, 1, 2, 0.01)

	// Multiple messages should be processed
	if len(handled) < 1 {
		t.Errorf("Expected at least 1 message to be processed, got %d", len(handled))
	}
}

func TestRunWorkerSleepsWhenNoMessages(t *testing.T) {
	conn := &mockConnection{
		queryRowFunc: func(query string, args ...any) session.Row {
			return &mockRow{err: &noRowsError{}}
		},
	}

	sess := &mockDbSession{connection: conn}
	pool := &mockSessionPool{session: sess}

	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	var handled []*InboxMessage
	subscriber := func(s session.Session, msg *InboxMessage) error {
		handled = append(handled, msg)
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = inbox.Run(ctx, subscriber, 0, 1, 1, 0.05)

	// No messages processed
	if len(handled) != 0 {
		t.Errorf("Expected 0 messages, got %d", len(handled))
	}
}

func TestRunFailingWorkerStopsTheOthers(t *testing.T) {
	boom := errors.New("worker 1 fails")
	var dispatches atomic.Int64

	conn := &mockConnection{
		queryRowFunc: func(query string, args ...any) session.Row {
			dispatches.Add(1)
			// args: offset, numWorkers, workerId
			if len(args) == 3 && args[2] == 1 {
				return &mockRow{err: boom}
			}
			return &mockRow{err: &noRowsError{}}
		},
	}
	pool := &mockSessionPool{session: &mockDbSession{connection: conn}}
	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	subscriber := func(s session.Session, msg *InboxMessage) error { return nil }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := inbox.Run(ctx, subscriber, 0, 1, 3, 0.001)
	if !errors.Is(err, boom) {
		t.Fatalf("Expected the worker's error, got %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("Run should have returned before the context deadline")
	}

	// Every worker has stopped: nothing is dispatched after Run returned.
	settled := dispatches.Load()
	time.Sleep(50 * time.Millisecond)
	if after := dispatches.Load(); after != settled {
		t.Errorf("Expected no dispatches after Run returned, got %d more", after-settled)
	}
}
