package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/identitymap"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/signals"
)

type mockRow struct {
	scanFunc func(dest ...any) error
	err      error
}

func (m *mockRow) Scan(dest ...any) error {
	if m.scanFunc != nil {
		return m.scanFunc(dest...)
	}
	return errors.New("no data")
}

func (m *mockRow) Err() error {
	return m.err
}

type mockRows struct {
	rows     [][]any
	index    int
	closeErr error
}

func (m *mockRows) Next() bool {
	if m.index < len(m.rows) {
		m.index++
		return true
	}
	return false
}

func (m *mockRows) Scan(dest ...any) error {
	if m.index == 0 || m.index > len(m.rows) {
		return errors.New("no current row")
	}
	row := m.rows[m.index-1]
	for i, val := range row {
		if i < len(dest) {
			switch d := dest[i].(type) {
			case *int64:
				*d = val.(int64)
			case *string:
				*d = val.(string)
			case *[]byte:
				*d = val.([]byte)
			}
		}
	}
	return nil
}

func (m *mockRows) Close() error {
	return m.closeErr
}

func (m *mockRows) Err() error {
	return nil
}

type mockResult struct {
	lastInsertId int64
	rowsAffected int64
}

func (m *mockResult) LastInsertId() (int64, error) {
	return m.lastInsertId, nil
}

func (m *mockResult) RowsAffected() (int64, error) {
	return m.rowsAffected, nil
}

type mockConnection struct {
	execFunc     func(query string, args ...any) (session.Result, error)
	queryFunc    func(query string, args ...any) (session.Rows, error)
	queryRowFunc func(query string, args ...any) session.Row
	lastQuery    string
	lastArgs     []any
}

func (m *mockConnection) Exec(query string, args ...any) (session.Result, error) {
	m.lastQuery = query
	m.lastArgs = args
	if m.execFunc != nil {
		return m.execFunc(query, args...)
	}
	return &mockResult{}, nil
}

func (m *mockConnection) Query(query string, args ...any) (session.Rows, error) {
	m.lastQuery = query
	m.lastArgs = args
	if m.queryFunc != nil {
		return m.queryFunc(query, args...)
	}
	return &mockRows{}, nil
}

func (m *mockConnection) QueryRow(query string, args ...any) session.Row {
	m.lastQuery = query
	m.lastArgs = args
	if m.queryRowFunc != nil {
		return m.queryRowFunc(query, args...)
	}
	return &mockRow{}
}

type mockDbSession struct {
	conn           *mockConnection
	atomicErr      error
	identityMap    *identitymap.IdentityMap
	onStarted      signals.Signal[session.SessionScopeStartedEvent]
	onEnded        signals.Signal[session.SessionScopeEndedEvent]
	onQueryStarted signals.Signal[session.QueryStartedEvent]
	onQueryEnded   signals.Signal[session.QueryEndedEvent]
}

func (m *mockDbSession) Context() context.Context {
	return context.Background()
}

func (m *mockDbSession) Atomic(callback session.SessionCallback) error {
	if m.atomicErr != nil {
		return m.atomicErr
	}
	return callback(m)
}

func (m *mockDbSession) Connection() session.DbConnection {
	return m.conn
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
	sessionErr       error
	onSessionStarted signals.Signal[session.SessionScopeStartedEvent]
	onSessionEnded   signals.Signal[session.SessionScopeEndedEvent]
}

func (m *mockSessionPool) Session(ctx context.Context, callback session.SessionPoolCallback) error {
	if m.sessionErr != nil {
		return m.sessionErr
	}
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

func TestPublishInsertsMessage(t *testing.T) {
	conn := &mockConnection{}
	dbSession := &mockDbSession{conn: conn}

	outbox := NewOutbox(nil, "outbox", "outbox_offsets", 100)
	message := &OutboxMessage{
		URI: "kafka://orders",
		Payload: map[string]any{
			"type":     "OrderCreated",
			"order_id": "123",
			"amount":   100,
		},
		Metadata: map[string]any{
			"event_id":       "uuid-123",
			"correlation_id": "corr-456",
		},
	}

	err := outbox.Publish(dbSession, message)
	require.NoError(t, err)

	assert.Contains(t, conn.lastQuery, "INSERT INTO")
	assert.Contains(t, conn.lastQuery, "pg_current_xact_id()")
	assert.Contains(t, conn.lastQuery, "outbox")

	require.Len(t, conn.lastArgs, 3)
	assert.Equal(t, "kafka://orders", conn.lastArgs[0])
}

func TestPublishUsesCustomTableName(t *testing.T) {
	conn := &mockConnection{}
	dbSession := &mockDbSession{conn: conn}

	outbox := NewOutbox(nil, "custom_outbox", "custom_offsets", 100)
	message := &OutboxMessage{
		URI: "kafka://orders",
		Payload: map[string]any{
			"type":     "OrderCreated",
			"order_id": "123",
		},
		Metadata: map[string]any{
			"event_id": "uuid-123",
		},
	}

	err := outbox.Publish(dbSession, message)
	require.NoError(t, err)

	assert.Contains(t, conn.lastQuery, "custom_outbox")
}

func TestGetPositionReturnsZerosWhenNotFound(t *testing.T) {
	conn := &mockConnection{
		queryRowFunc: func(query string, args ...any) session.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					return errors.New("no rows")
				},
			}
		},
	}
	dbSession := &mockDbSession{conn: conn}

	outbox := NewOutbox(nil, "outbox", "outbox_offsets", 100)
	txID, offset, err := outbox.GetPosition(dbSession, "test-group", "")

	require.NoError(t, err)
	assert.Equal(t, int64(0), txID)
	assert.Equal(t, int64(0), offset)
}

func TestGetPositionReturnsStoredPosition(t *testing.T) {
	conn := &mockConnection{
		queryRowFunc: func(query string, args ...any) session.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					if len(dest) >= 2 {
						*dest[0].(*int64) = 100
						*dest[1].(*int64) = 50
					}
					return nil
				},
			}
		},
	}
	dbSession := &mockDbSession{conn: conn}

	outbox := NewOutbox(nil, "outbox", "outbox_offsets", 100)
	txID, offset, err := outbox.GetPosition(dbSession, "test-group", "")

	require.NoError(t, err)
	assert.Equal(t, int64(100), txID)
	assert.Equal(t, int64(50), offset)
}

func TestGetPositionWithURI(t *testing.T) {
	conn := &mockConnection{
		queryRowFunc: func(query string, args ...any) session.Row {
			return &mockRow{
				scanFunc: func(dest ...any) error {
					if len(dest) >= 2 {
						*dest[0].(*int64) = 100
						*dest[1].(*int64) = 50
					}
					return nil
				},
			}
		},
	}
	dbSession := &mockDbSession{conn: conn}

	outbox := NewOutbox(nil, "outbox", "outbox_offsets", 100)
	txID, offset, err := outbox.GetPosition(dbSession, "test-group", "kafka://orders")

	require.NoError(t, err)
	assert.Equal(t, int64(100), txID)
	assert.Equal(t, int64(50), offset)

	require.Len(t, conn.lastArgs, 2)
	assert.Equal(t, "test-group", conn.lastArgs[0])
	assert.Equal(t, "kafka://orders", conn.lastArgs[1])
}

func TestSetPositionUpserts(t *testing.T) {
	conn := &mockConnection{}
	dbSession := &mockDbSession{conn: conn}

	outbox := NewOutbox(nil, "outbox", "outbox_offsets", 100)
	err := outbox.SetPosition(dbSession, "test-group", "", 100, 50)

	require.NoError(t, err)

	assert.Contains(t, conn.lastQuery, "INSERT INTO")
	assert.Contains(t, conn.lastQuery, "ON CONFLICT")
	assert.Contains(t, conn.lastQuery, "DO UPDATE")

	require.Len(t, conn.lastArgs, 4)
	assert.Equal(t, "test-group", conn.lastArgs[0])
	assert.Equal(t, "", conn.lastArgs[1])
	assert.Equal(t, int64(50), conn.lastArgs[2])
	assert.Equal(t, "100", conn.lastArgs[3])
}

func TestSetPositionWithURI(t *testing.T) {
	conn := &mockConnection{}
	dbSession := &mockDbSession{conn: conn}

	outbox := NewOutbox(nil, "outbox", "outbox_offsets", 100)
	err := outbox.SetPosition(dbSession, "test-group", "kafka://orders", 100, 50)

	require.NoError(t, err)

	require.Len(t, conn.lastArgs, 4)
	assert.Equal(t, "test-group", conn.lastArgs[0])
	assert.Equal(t, "kafka://orders", conn.lastArgs[1])
	assert.Equal(t, int64(50), conn.lastArgs[2])
}

func TestFetchMessagesWithURIFilter(t *testing.T) {
	payload1, _ := json.Marshal(map[string]any{"type": "OrderCreated", "order_id": "123"})
	metadata1, _ := json.Marshal(map[string]any{"event_id": "uuid-1"})

	conn := &mockConnection{
		queryFunc: func(query string, args ...any) (session.Rows, error) {
			return &mockRows{
				rows: [][]any{
					{int64(1), int64(100), "kafka://orders", payload1, metadata1, "2024-01-01 00:00:00"},
				},
			}, nil
		},
	}
	dbSession := &mockDbSession{conn: conn}

	outbox := NewOutbox(nil, "outbox", "outbox_offsets", 100)
	messages, err := outbox.fetchMessages(dbSession, "test-group", "kafka://orders", 0, 1)

	require.NoError(t, err)
	assert.Len(t, messages, 1)

	require.GreaterOrEqual(t, len(conn.lastArgs), 2)
	assert.Equal(t, "test-group", conn.lastArgs[0])
	assert.Equal(t, "kafka://orders", conn.lastArgs[1])
}

func TestFetchMessagesWithPartitioning(t *testing.T) {
	payload1, _ := json.Marshal(map[string]any{"type": "OrderCreated"})
	metadata1, _ := json.Marshal(map[string]any{})

	conn := &mockConnection{
		queryFunc: func(query string, args ...any) (session.Rows, error) {
			return &mockRows{
				rows: [][]any{
					{int64(1), int64(100), "kafka://orders/order-123", payload1, metadata1, "2024-01-01 00:00:00"},
				},
			}, nil
		},
	}
	dbSession := &mockDbSession{conn: conn}

	outbox := NewOutbox(nil, "outbox", "outbox_offsets", 100)
	messages, err := outbox.fetchMessages(dbSession, "test-group:0", "kafka://orders", 0, 3)

	require.NoError(t, err)
	assert.Len(t, messages, 1)

	assert.Contains(t, conn.lastQuery, "hashtext(uri)")
	require.GreaterOrEqual(t, len(conn.lastArgs), 4)
	assert.Equal(t, 3, conn.lastArgs[len(conn.lastArgs)-2])
	assert.Equal(t, 0, conn.lastArgs[len(conn.lastArgs)-1])
}

func TestConsumerGroupModificationWithWorkers(t *testing.T) {
	consumerGroup := "test-group"
	workerID := 2
	numWorkers := 5

	var effectiveConsumerGroup string
	if numWorkers > 1 {
		effectiveConsumerGroup = fmt.Sprintf("%s:%d", consumerGroup, workerID)
	} else {
		effectiveConsumerGroup = consumerGroup
	}

	assert.Equal(t, "test-group:2", effectiveConsumerGroup)
}

func TestConsumerGroupNoModificationWithSingleWorker(t *testing.T) {
	consumerGroup := "test-group"
	workerID := 0
	numWorkers := 1

	var effectiveConsumerGroup string
	if numWorkers > 1 {
		effectiveConsumerGroup = fmt.Sprintf("%s:%d", consumerGroup, workerID)
	} else {
		effectiveConsumerGroup = consumerGroup
	}

	assert.Equal(t, "test-group", effectiveConsumerGroup)
}

func TestDispatchReturnsTrue(t *testing.T) {
	payload1, _ := json.Marshal(map[string]any{"type": "OrderCreated", "order_id": "123"})
	metadata1, _ := json.Marshal(map[string]any{"event_id": "uuid-1"})
	payload2, _ := json.Marshal(map[string]any{"type": "OrderShipped", "order_id": "123"})
	metadata2, _ := json.Marshal(map[string]any{"event_id": "uuid-2"})

	conn := &mockConnection{
		execFunc: func(query string, args ...any) (session.Result, error) {
			return &mockResult{}, nil
		},
		queryFunc: func(query string, args ...any) (session.Rows, error) {
			return &mockRows{
				rows: [][]any{
					{int64(1), int64(100), "kafka://orders", payload1, metadata1, "2024-01-01 00:00:00"},
					{int64(2), int64(100), "kafka://orders", payload2, metadata2, "2024-01-01 00:00:01"},
				},
			}, nil
		},
	}
	dbSession := &mockDbSession{conn: conn}
	pool := &mockSessionPool{session: dbSession}

	outbox := NewOutbox(pool, "outbox", "outbox_offsets", 100)

	var published []*OutboxMessage
	subscriber := func(msg *OutboxMessage) error {
		published = append(published, msg)
		return nil
	}

	result, err := outbox.Dispatch(subscriber, "", "", 0, 1)
	require.NoError(t, err)

	assert.True(t, result)
	assert.Len(t, published, 2)
	assert.Equal(t, "kafka://orders", published[0].URI)
	assert.Equal(t, "OrderCreated", published[0].Payload["type"])
	assert.Equal(t, "OrderShipped", published[1].Payload["type"])
}

func TestDispatchAcknowledgesLastMessage(t *testing.T) {
	payload1, _ := json.Marshal(map[string]any{"type": "OrderCreated", "order_id": "123"})
	metadata1, _ := json.Marshal(map[string]any{"event_id": "uuid-1"})

	ackCalled := false
	conn := &mockConnection{
		execFunc: func(query string, args ...any) (session.Result, error) {
			if strings.Contains(query, "ON CONFLICT") && strings.Contains(query, "offset_acked") {
				ackCalled = true
			}
			return &mockResult{}, nil
		},
		queryFunc: func(query string, args ...any) (session.Rows, error) {
			return &mockRows{
				rows: [][]any{
					{int64(5), int64(100), "kafka://orders", payload1, metadata1, "2024-01-01 00:00:00"},
				},
			}, nil
		},
	}
	dbSession := &mockDbSession{conn: conn}
	pool := &mockSessionPool{session: dbSession}

	outbox := NewOutbox(pool, "outbox", "outbox_offsets", 100)

	subscriber := func(msg *OutboxMessage) error {
		return nil
	}

	_, err := outbox.Dispatch(subscriber, "test-group", "", 0, 1)
	require.NoError(t, err)

	assert.True(t, ackCalled)
}

func TestMessageCreation(t *testing.T) {
	message := &OutboxMessage{
		URI: "kafka://orders",
		Payload: map[string]any{
			"type":     "OrderCreated",
			"order_id": "123",
		},
		Metadata: map[string]any{
			"event_id": "uuid-123",
		},
	}

	assert.Equal(t, "kafka://orders", message.URI)
	assert.Equal(t, "OrderCreated", message.Payload["type"])
	assert.Equal(t, "123", message.Payload["order_id"])
	assert.Equal(t, "uuid-123", message.Metadata["event_id"])
	assert.Nil(t, message.Position)
	assert.Nil(t, message.TransactionID)
}

func TestMessageWithAllFields(t *testing.T) {
	createdAt := "2024-01-01 00:00:00"
	position := int64(5)
	transactionID := int64(100)

	message := &OutboxMessage{
		URI: "kafka://orders",
		Payload: map[string]any{
			"type":     "OrderCreated",
			"order_id": "123",
		},
		Metadata: map[string]any{
			"event_id": "uuid-123",
		},
		CreatedAt:     &createdAt,
		Position:      &position,
		TransactionID: &transactionID,
	}

	assert.Equal(t, "2024-01-01 00:00:00", *message.CreatedAt)
	assert.Equal(t, int64(5), *message.Position)
	assert.Equal(t, int64(100), *message.TransactionID)
}
