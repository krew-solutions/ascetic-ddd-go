package inbox

import (
	"context"
	"testing"
	"time"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/utils/testutils"
)

func setupInboxIntegrationTest(t *testing.T) (*PgInbox, session.SessionPool, func()) {
	t.Helper()

	pool, err := testutils.NewPgxSessionPool()
	if err != nil {
		t.Fatalf("Failed to create session pool: %v", err)
	}

	inbox := NewInbox(pool, "inbox_test", "inbox_test_received_position_seq", nil)
	if err := inbox.Setup(); err != nil {
		t.Fatalf("Failed to setup inbox: %v", err)
	}

	// Truncate table
	ctx := context.Background()
	err = pool.Session(ctx, func(s session.Session) error {
		return s.Atomic(func(txSession session.Session) error {
			_, err := txSession.(session.DbSession).Connection().Exec("TRUNCATE TABLE inbox_test")
			return err
		})
	})
	if err != nil {
		t.Fatalf("Failed to truncate table: %v", err)
	}

	cleanup := func() {
		ctx := context.Background()
		_ = pool.Session(ctx, func(s session.Session) error {
			return s.Atomic(func(txSession session.Session) error {
				dbSession := txSession.(session.DbSession)
				_, _ = dbSession.Connection().Exec("DROP TABLE IF EXISTS inbox_test")
				_, _ = dbSession.Connection().Exec("DROP SEQUENCE IF EXISTS inbox_test_received_position_seq")
				return nil
			})
		})
	}

	return inbox, pool, cleanup
}

func TestPublishAndDispatch(t *testing.T) {
	inbox, _, cleanup := setupInboxIntegrationTest(t)
	defer cleanup()

	message := &InboxMessage{
		TenantId:       "tenant1",
		StreamType:     "Order",
		StreamId:       map[string]any{"id": "order-123"},
		StreamPosition: 1,
		Uri:            "kafka://orders",
		Payload:        map[string]any{"amount": 100},
		Metadata:       map[string]any{"event_id": "uuid-123"},
	}

	err := inbox.Publish(message)
	if err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

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
		t.Fatalf("Expected 1 message, got %d", len(handled))
	}

	if handled[0].TenantId != "tenant1" {
		t.Errorf("Expected tenant_id=tenant1, got %s", handled[0].TenantId)
	}

	if handled[0].StreamId["id"] != "order-123" {
		t.Errorf("Expected stream_id.id=order-123, got %v", handled[0].StreamId["id"])
	}
}

func TestIdempotency(t *testing.T) {
	inbox, _, cleanup := setupInboxIntegrationTest(t)
	defer cleanup()

	message := &InboxMessage{
		TenantId:       "tenant1",
		StreamType:     "Order",
		StreamId:       map[string]any{"id": "order-123"},
		StreamPosition: 1,
		Uri:            "kafka://orders",
		Payload:        map[string]any{"amount": 100},
	}

	// Publish same message twice
	if err := inbox.Publish(message); err != nil {
		t.Fatalf("First publish failed: %v", err)
	}
	if err := inbox.Publish(message); err != nil {
		t.Fatalf("Second publish failed: %v", err)
	}

	var handled []*InboxMessage
	subscriber := func(s session.DbSession, msg *InboxMessage) error {
		handled = append(handled, msg)
		return nil
	}

	// Process first message
	result1, err := inbox.Dispatch(subscriber, 0, 1)
	if err != nil {
		t.Fatalf("First dispatch failed: %v", err)
	}
	if !result1 {
		t.Error("Expected first dispatch to return true")
	}

	// Try to process second message (should not exist)
	result2, err := inbox.Dispatch(subscriber, 0, 1)
	if err != nil {
		t.Fatalf("Second dispatch failed: %v", err)
	}
	if result2 {
		t.Error("Expected second dispatch to return false")
	}

	if len(handled) != 1 {
		t.Errorf("Expected 1 message, got %d", len(handled))
	}
}

func TestCausalDependencies(t *testing.T) {
	inbox, _, cleanup := setupInboxIntegrationTest(t)
	defer cleanup()

	// Message that depends on another
	dependentMessage := &InboxMessage{
		TenantId:       "tenant1",
		StreamType:     "Order",
		StreamId:       map[string]any{"id": "order-123"},
		StreamPosition: 2,
		Uri:            "kafka://shipments",
		Payload:        map[string]any{"tracking": "123"},
		Metadata: map[string]any{
			"causal_dependencies": []any{
				map[string]any{
					"tenant_id":       "tenant1",
					"stream_type":     "Order",
					"stream_id":       map[string]any{"id": "order-123"},
					"stream_position": 1,
				},
			},
		},
	}

	// Publish dependent message first
	if err := inbox.Publish(dependentMessage); err != nil {
		t.Fatalf("Failed to publish dependent message: %v", err)
	}

	var handled []*InboxMessage
	subscriber := func(s session.DbSession, msg *InboxMessage) error {
		handled = append(handled, msg)
		return nil
	}

	// Should not process (dependency not met)
	result, err := inbox.Dispatch(subscriber, 0, 1)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}
	if result {
		t.Error("Expected dispatch to return false (dependency not met)")
	}

	// Now publish the dependency
	dependencyMessage := &InboxMessage{
		TenantId:       "tenant1",
		StreamType:     "Order",
		StreamId:       map[string]any{"id": "order-123"},
		StreamPosition: 1,
		Uri:            "kafka://orders",
		Payload:        map[string]any{"amount": 100},
	}
	if err := inbox.Publish(dependencyMessage); err != nil {
		t.Fatalf("Failed to publish dependency: %v", err)
	}

	// Process dependency first
	result, err = inbox.Dispatch(subscriber, 0, 1)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}
	if !result {
		t.Error("Expected dispatch to return true")
	}
	if len(handled) != 1 || handled[0].Uri != "kafka://orders" {
		t.Error("Expected to process dependency first")
	}

	// Now dependent message can be processed
	result, err = inbox.Dispatch(subscriber, 0, 1)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}
	if !result {
		t.Error("Expected dispatch to return true")
	}
	if len(handled) != 2 || handled[1].Uri != "kafka://shipments" {
		t.Error("Expected to process dependent message second")
	}
}

func TestOrderingByReceivedPosition(t *testing.T) {
	inbox, _, cleanup := setupInboxIntegrationTest(t)
	defer cleanup()

	// Publish 3 messages
	for i := 0; i < 3; i++ {
		message := &InboxMessage{
			TenantId:       "tenant1",
			StreamType:     "Order",
			StreamId:       map[string]any{"id": "order-" + string(rune('0'+i))},
			StreamPosition: 1,
			Uri:            "kafka://orders",
			Payload:        map[string]any{"type": "OrderCreated", "order": i},
		}
		if err := inbox.Publish(message); err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	var handled []*InboxMessage
	subscriber := func(s session.DbSession, msg *InboxMessage) error {
		handled = append(handled, msg)
		return nil
	}

	// Process all messages
	for {
		result, err := inbox.Dispatch(subscriber, 0, 1)
		if err != nil {
			t.Fatalf("Dispatch failed: %v", err)
		}
		if !result {
			break
		}
	}

	if len(handled) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(handled))
	}

	// Check order
	for i, msg := range handled {
		order := int(msg.Payload["order"].(float64))
		if order != i {
			t.Errorf("Expected message %d to have order=%d, got %d", i, i, order)
		}
	}
}

func TestRoutingByUri(t *testing.T) {
	inbox, _, cleanup := setupInboxIntegrationTest(t)
	defer cleanup()

	// Publish messages with different URIs
	if err := inbox.Publish(&InboxMessage{
		TenantId:       "tenant1",
		StreamType:     "Order",
		StreamId:       map[string]any{"id": "order-1"},
		StreamPosition: 1,
		Uri:            "kafka://orders",
		Payload:        map[string]any{"type": "OrderCreated"},
	}); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	if err := inbox.Publish(&InboxMessage{
		TenantId:       "tenant1",
		StreamType:     "Order",
		StreamId:       map[string]any{"id": "order-2"},
		StreamPosition: 1,
		Uri:            "kafka://shipments",
		Payload:        map[string]any{"type": "OrderShipped"},
	}); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	var handledEvents []struct {
		uri      string
		streamID map[string]any
	}

	subscriber := func(s session.DbSession, message *InboxMessage) error {
		if message.Uri == "kafka://orders" {
			handledEvents = append(handledEvents, struct {
				uri      string
				streamID map[string]any
			}{"orders", message.StreamId})
		} else if message.Uri == "kafka://shipments" {
			handledEvents = append(handledEvents, struct {
				uri      string
				streamID map[string]any
			}{"shipments", message.StreamId})
		}
		return nil
	}

	// Process messages
	if _, err := inbox.Dispatch(subscriber, 0, 1); err != nil {
		t.Fatalf("First dispatch failed: %v", err)
	}
	if _, err := inbox.Dispatch(subscriber, 0, 1); err != nil {
		t.Fatalf("Second dispatch failed: %v", err)
	}

	if len(handledEvents) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(handledEvents))
	}

	if handledEvents[0].uri != "orders" || handledEvents[0].streamID["id"] != "order-1" {
		t.Error("First event mismatch")
	}
	if handledEvents[1].uri != "shipments" || handledEvents[1].streamID["id"] != "order-2" {
		t.Error("Second event mismatch")
	}
}

func TestRunWithSingleWorker(t *testing.T) {
	inbox, _, cleanup := setupInboxIntegrationTest(t)
	defer cleanup()

	// Publish 3 messages
	for i := 0; i < 3; i++ {
		if err := inbox.Publish(&InboxMessage{
			TenantId:       "tenant1",
			StreamType:     "Order",
			StreamId:       map[string]any{"id": "order-" + string(rune('0'+i))},
			StreamPosition: 1,
			Uri:            "kafka://orders",
			Payload:        map[string]any{"type": "OrderCreated", "order": i},
		}); err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	var handled []*InboxMessage
	subscriber := func(s session.DbSession, msg *InboxMessage) error {
		handled = append(handled, msg)
		return nil
	}

	// Run with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_ = inbox.Run(ctx, subscriber, 0, 1, 1, 0.01)

	if len(handled) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(handled))
	}
}

func TestMessagesChannelAPI(t *testing.T) {
	inbox, _, cleanup := setupInboxIntegrationTest(t)
	defer cleanup()

	// Publish 2 messages
	for i := 0; i < 2; i++ {
		if err := inbox.Publish(&InboxMessage{
			TenantId:       "tenant1",
			StreamType:     "Order",
			StreamId:       map[string]any{"id": "order-" + string(rune('0'+i))},
			StreamPosition: 1,
			Uri:            "kafka://orders",
			Payload:        map[string]any{"type": "OrderCreated", "order": i},
		}); err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	var receivedMessages []*InboxMessage
	for sessionMsg := range inbox.Messages(ctx, 0, 1, 0.01) {
		receivedMessages = append(receivedMessages, sessionMsg.Message)
	}

	if len(receivedMessages) != 2 {
		t.Errorf("Expected 2 messages, got %d", len(receivedMessages))
	}

	for i, msg := range receivedMessages {
		order := int(msg.Payload["order"].(float64))
		if order != i {
			t.Errorf("Expected message %d to have order=%d, got %d", i, i, order)
		}
	}
}

func TestForUpdateSkipLocked(t *testing.T) {
	inbox, _, cleanup := setupInboxIntegrationTest(t)
	defer cleanup()

	// Publish a single message
	if err := inbox.Publish(&InboxMessage{
		TenantId:       "tenant1",
		StreamType:     "Order",
		StreamId:       map[string]any{"id": "order-1"},
		StreamPosition: 1,
		Uri:            "kafka://orders",
		Payload:        map[string]any{},
	}); err != nil {
		t.Fatalf("Failed to publish: %v", err)
	}

	var handled []*InboxMessage
	subscriber := func(s session.DbSession, msg *InboxMessage) error {
		handled = append(handled, msg)
		return nil
	}

	// Run with multiple workers
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_ = inbox.Run(ctx, subscriber, 0, 1, 3, 0.01)

	// Message should be processed exactly once
	if len(handled) != 1 {
		t.Errorf("Expected message to be processed exactly once, got %d", len(handled))
	}
}

func TestRunWithMultipleWorkers(t *testing.T) {
	inbox, _, cleanup := setupInboxIntegrationTest(t)
	defer cleanup()

	// Publish 10 messages
	for i := 0; i < 10; i++ {
		if err := inbox.Publish(&InboxMessage{
			TenantId:       "tenant1",
			StreamType:     "Order",
			StreamId:       map[string]any{"id": "order-" + string(rune('0'+i))},
			StreamPosition: 1,
			Uri:            "kafka://orders",
			Payload:        map[string]any{"type": "OrderCreated", "order": i},
		}); err != nil {
			t.Fatalf("Failed to publish: %v", err)
		}
	}

	var handled []*InboxMessage
	subscriber := func(s session.DbSession, msg *InboxMessage) error {
		handled = append(handled, msg)
		return nil
	}

	// Run with multiple workers
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	_ = inbox.Run(ctx, subscriber, 0, 1, 3, 0.01)

	// All messages should be processed
	if len(handled) != 10 {
		t.Errorf("Expected 10 messages, got %d", len(handled))
	}
}
