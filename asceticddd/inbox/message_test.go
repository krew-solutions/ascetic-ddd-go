package inbox

import (
	"testing"
)

func TestCreateMessage(t *testing.T) {
	message := &InboxMessage{
		TenantID:       "tenant1",
		StreamType:     "Order",
		StreamID:       map[string]any{"id": "order-123"},
		StreamPosition: 1,
		URI:            "kafka://orders",
		Payload:        map[string]any{"amount": 100},
	}

	if message.TenantID != "tenant1" {
		t.Errorf("Expected tenant_id=tenant1, got %s", message.TenantID)
	}
	if message.StreamType != "Order" {
		t.Errorf("Expected stream_type=Order, got %s", message.StreamType)
	}
	if message.StreamID["id"] != "order-123" {
		t.Errorf("Expected stream_id.id=order-123, got %v", message.StreamID["id"])
	}
	if message.StreamPosition != 1 {
		t.Errorf("Expected stream_position=1, got %d", message.StreamPosition)
	}
	if message.URI != "kafka://orders" {
		t.Errorf("Expected uri=kafka://orders, got %s", message.URI)
	}
	if message.Payload["amount"] != 100 {
		t.Errorf("Expected payload.amount=100, got %v", message.Payload["amount"])
	}
	if message.Metadata != nil {
		t.Error("Expected metadata to be nil")
	}
	if message.ReceivedPosition != nil {
		t.Error("Expected received_position to be nil")
	}
	if message.ProcessedPosition != nil {
		t.Error("Expected processed_position to be nil")
	}
}

func TestCreateMessageWithMetadata(t *testing.T) {
	message := &InboxMessage{
		TenantID:       "tenant1",
		StreamType:     "Order",
		StreamID:       map[string]any{"id": "order-123"},
		StreamPosition: 1,
		URI:            "kafka://orders",
		Payload:        map[string]any{"amount": 100},
		Metadata: map[string]any{
			"event_id":  "uuid-123",
			"timestamp": "2024-01-01T00:00:00Z",
		},
	}

	if message.Metadata["event_id"] != "uuid-123" {
		t.Errorf("Expected event_id=uuid-123, got %v", message.Metadata["event_id"])
	}
}

func TestCausalDependenciesEmptyWhenNoMetadata(t *testing.T) {
	message := &InboxMessage{
		TenantID:       "tenant1",
		StreamType:     "Order",
		StreamID:       map[string]any{"id": "order-123"},
		StreamPosition: 1,
		URI:            "kafka://orders",
		Payload:        map[string]any{},
	}

	deps := message.CausalDependencies()
	if len(deps) != 0 {
		t.Errorf("Expected empty dependencies, got %d", len(deps))
	}
}

func TestCausalDependenciesEmptyWhenNotPresent(t *testing.T) {
	message := &InboxMessage{
		TenantID:       "tenant1",
		StreamType:     "Order",
		StreamID:       map[string]any{"id": "order-123"},
		StreamPosition: 1,
		URI:            "kafka://orders",
		Payload:        map[string]any{},
		Metadata:       map[string]any{"event_id": "uuid-123"},
	}

	deps := message.CausalDependencies()
	if len(deps) != 0 {
		t.Errorf("Expected empty dependencies, got %d", len(deps))
	}
}

func TestCausalDependenciesReturnsList(t *testing.T) {
	deps := []any{
		map[string]any{
			"tenant_id":       "tenant1",
			"stream_type":     "User",
			"stream_id":       map[string]any{"id": "user-1"},
			"stream_position": 5,
		},
		map[string]any{
			"tenant_id":       "tenant1",
			"stream_type":     "Product",
			"stream_id":       map[string]any{"id": "prod-1"},
			"stream_position": 3,
		},
	}

	message := &InboxMessage{
		TenantID:       "tenant1",
		StreamType:     "Order",
		StreamID:       map[string]any{"id": "order-123"},
		StreamPosition: 1,
		URI:            "kafka://orders",
		Payload:        map[string]any{},
		Metadata:       map[string]any{"causal_dependencies": deps},
	}

	result := message.CausalDependencies()
	if len(result) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(result))
	}

	if result[0]["stream_type"] != "User" {
		t.Errorf("Expected first dependency stream_type=User, got %v", result[0]["stream_type"])
	}
	if result[1]["stream_type"] != "Product" {
		t.Errorf("Expected second dependency stream_type=Product, got %v", result[1]["stream_type"])
	}
}

func TestEventIDNoneWhenNoMetadata(t *testing.T) {
	message := &InboxMessage{
		TenantID:       "tenant1",
		StreamType:     "Order",
		StreamID:       map[string]any{"id": "order-123"},
		StreamPosition: 1,
		URI:            "kafka://orders",
		Payload:        map[string]any{},
	}

	eventID := message.EventID()
	if eventID != nil {
		t.Errorf("Expected nil event_id, got %v", *eventID)
	}
}

func TestEventIDReturnsValue(t *testing.T) {
	message := &InboxMessage{
		TenantID:       "tenant1",
		StreamType:     "Order",
		StreamID:       map[string]any{"id": "order-123"},
		StreamPosition: 1,
		URI:            "kafka://orders",
		Payload:        map[string]any{},
		Metadata:       map[string]any{"event_id": "uuid-456"},
	}

	eventID := message.EventID()
	if eventID == nil {
		t.Fatal("Expected event_id to be non-nil")
	}
	if *eventID != "uuid-456" {
		t.Errorf("Expected event_id=uuid-456, got %s", *eventID)
	}
}

func TestReceivedAndProcessedPositions(t *testing.T) {
	receivedPos := int64(100)
	processedPos := int64(50)

	message := &InboxMessage{
		TenantID:          "tenant1",
		StreamType:        "Order",
		StreamID:          map[string]any{"id": "order-123"},
		StreamPosition:    1,
		URI:               "kafka://orders",
		Payload:           map[string]any{},
		ReceivedPosition:  &receivedPos,
		ProcessedPosition: &processedPos,
	}

	if message.ReceivedPosition == nil || *message.ReceivedPosition != 100 {
		t.Errorf("Expected received_position=100, got %v", message.ReceivedPosition)
	}
	if message.ProcessedPosition == nil || *message.ProcessedPosition != 50 {
		t.Errorf("Expected processed_position=50, got %v", message.ProcessedPosition)
	}
}
