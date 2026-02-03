package inbox

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
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
	connection *mockConnection
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

type mockSessionPool struct {
	session *mockDbSession
}

func (m *mockSessionPool) Session(ctx context.Context, callback session.SessionPoolCallback) error {
	return callback(m.session)
}

func (m *mockSessionPool) Close() {}

// Tests

func TestPublishInsertsMessage(t *testing.T) {
	var executedSQL string
	var executedArgs []any

	conn := &mockConnection{
		execFunc: func(query string, args ...any) (session.Result, error) {
			executedSQL = query
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
		Payload:        map[string]any{"type": "OrderCreated", "amount": 100},
		Metadata:       map[string]any{"event_id": "uuid-123"},
	}

	err := inbox.Publish(message)
	if err != nil {
		t.Fatalf("Publish failed: %v", err)
	}

	if executedSQL == "" {
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
	subscriber := func(s session.DbSession, msg *InboxMessage) error {
		handled = append(handled, msg)
		return nil
	}

	result, err := inbox.Dispatch(subscriber, 0, 1)
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
	streamID := map[string]any{"id": "order-123"}
	streamIDBytes, _ := json.Marshal(streamID)

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
					streamIDBytes,
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
	subscriber := func(s session.DbSession, msg *InboxMessage) error {
		handled = append(handled, msg)
		return nil
	}

	result, err := inbox.Dispatch(subscriber, 0, 1)
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
		Payload:        map[string]any{"type": "OrderCreated"},
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
	var executedSQLs []string

	conn := &mockConnection{
		execFunc: func(query string, args ...any) (session.Result, error) {
			executedSQLs = append(executedSQLs, query)
			return &mockResult{}, nil
		},
	}

	sess := &mockDbSession{connection: conn}
	pool := &mockSessionPool{session: sess}

	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	err := inbox.Setup()
	if err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	if len(executedSQLs) != 2 {
		t.Fatalf("Expected 2 SQL statements, got %d", len(executedSQLs))
	}

	// First should create sequence
	if executedSQLs[0] == "" {
		t.Error("First SQL should create sequence")
	}

	// Second should create table
	if executedSQLs[1] == "" {
		t.Error("Second SQL should create table")
	}
}

func TestMessagesChannelYieldsMessages(t *testing.T) {
	streamID := map[string]any{"id": "order-123"}
	streamIDBytes, _ := json.Marshal(streamID)

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
						streamIDBytes,
						1,
						"kafka://orders",
						payloadBytes,
						[]byte{},
						receivedPos,
						nil,
					},
				}
			}
			// After first message, return no rows
			return &mockRow{err: &noRowsError{}}
		},
		execFunc: func(query string, args ...any) (session.Result, error) {
			return &mockResult{}, nil
		},
	}

	sess := &mockDbSession{connection: conn}
	pool := &mockSessionPool{session: sess}

	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	var received []*InboxMessage
	for sessionMsg := range inbox.Messages(ctx, 0, 1, 0.01) {
		received = append(received, sessionMsg.Message)
	}

	if len(received) != 1 {
		t.Errorf("Expected 1 message, got %d", len(received))
	}

	if len(received) > 0 && received[0].TenantId != "tenant1" {
		t.Errorf("Expected tenant_id=tenant1, got %s", received[0].TenantId)
	}
}

func TestMessagesMarksAsProcessed(t *testing.T) {
	streamID := map[string]any{"id": "order-123"}
	streamIDBytes, _ := json.Marshal(streamID)

	payload := map[string]any{"type": "OrderCreated", "amount": 100}
	payloadBytes, _ := json.Marshal(payload)

	receivedPos := int64(1)

	var executedSQLs []string
	callCount := 0

	conn := &mockConnection{
		queryRowFunc: func(query string, args ...any) session.Row {
			callCount++
			if callCount == 1 {
				return &mockRow{
					values: []any{
						"tenant1",
						"Order",
						streamIDBytes,
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
			executedSQLs = append(executedSQLs, query)
			return &mockResult{}, nil
		},
	}

	sess := &mockDbSession{connection: conn}
	pool := &mockSessionPool{session: sess}

	inbox := NewInbox(pool, "inbox", "inbox_received_position_seq", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	count := 0
	for range inbox.Messages(ctx, 0, 1, 0.01) {
		count++
		break // Get one message and stop
	}

	// Check that UPDATE was executed to mark as processed
	updateCount := 0
	for _, sql := range executedSQLs {
		// Trim whitespace and check if it starts with UPDATE
		trimmed := strings.TrimSpace(sql)
		if strings.HasPrefix(trimmed, "UPDATE") {
			updateCount++
		}
	}

	if updateCount == 0 {
		t.Errorf("Expected UPDATE statement to be executed, got %d SQLs: %v", len(executedSQLs), executedSQLs)
	}
}

func TestRunSingleWorkerProcessesMessages(t *testing.T) {
	streamID := map[string]any{"id": "order-123"}
	streamIDBytes, _ := json.Marshal(streamID)

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
						streamIDBytes,
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
	subscriber := func(s session.DbSession, msg *InboxMessage) error {
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
	streamID := func(i int) map[string]any {
		return map[string]any{"id": "order-" + string(rune('0'+i))}
	}

	callCount := 0
	conn := &mockConnection{
		queryRowFunc: func(query string, args ...any) session.Row {
			callCount++
			if callCount <= 4 {
				id := streamID(callCount - 1)
				streamIDBytes, _ := json.Marshal(id)
				payload := map[string]any{"type": "OrderCreated", "amount": 100}
				payloadBytes, _ := json.Marshal(payload)
				receivedPos := int64(callCount - 1)

				return &mockRow{
					values: []any{
						"tenant1",
						"Order",
						streamIDBytes,
						callCount - 1,
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
	subscriber := func(s session.DbSession, msg *InboxMessage) error {
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
	subscriber := func(s session.DbSession, msg *InboxMessage) error {
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
