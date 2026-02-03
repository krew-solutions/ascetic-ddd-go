package saga

import (
	"context"
	"encoding/json"
	"testing"
)

// Test activities for serialization
type serializableSuccessActivity struct {
	callCount       *int
	compensateCount *int
}

func newSerializableSuccessActivity(callCount, compensateCount *int) ActivityType {
	return func() Activity {
		return &serializableSuccessActivity{
			callCount:       callCount,
			compensateCount: compensateCount,
		}
	}
}

func (s *serializableSuccessActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	*s.callCount++
	workLog := NewWorkLog(s, WorkResult{"id": *s.callCount})
	return &workLog, nil
}

func (s *serializableSuccessActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	*s.compensateCount++
	return true, nil
}

func (s *serializableSuccessActivity) WorkItemQueueAddress() string {
	return "sb://./success"
}

func (s *serializableSuccessActivity) CompensationQueueAddress() string {
	return "sb://./successCompensation"
}

func (s *serializableSuccessActivity) ActivityType() ActivityType {
	return newSerializableSuccessActivity(s.callCount, s.compensateCount)
}

func (s *serializableSuccessActivity) TypeName() string {
	return "SerializableSuccessActivity"
}

func TestRoutingSlip_ToSerializable_EmptySlip(t *testing.T) {
	resolver := NewMapBasedResolver()
	slip := NewRoutingSlip(nil)

	serializable, err := slip.ToSerializable(resolver)
	if err != nil {
		t.Fatalf("ToSerializable failed: %v", err)
	}

	if len(serializable.CompletedWorkLogs) != 0 {
		t.Errorf("Expected 0 completed work logs, got %d", len(serializable.CompletedWorkLogs))
	}
	if len(serializable.NextWorkItems) != 0 {
		t.Errorf("Expected 0 next work items, got %d", len(serializable.NextWorkItems))
	}
}

func TestRoutingSlip_ToSerializable_WithPendingItems(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSerializableSuccessActivity(&callCount, &compensateCount)

	resolver := NewMapBasedResolver()
	resolver.Register("SerializableSuccessActivity", activityType)

	workItems := []WorkItem{
		NewWorkItem(activityType, WorkItemArguments{"a": 1}),
		NewWorkItem(activityType, WorkItemArguments{"b": 2}),
	}
	slip := NewRoutingSlip(workItems)

	serializable, err := slip.ToSerializable(resolver)
	if err != nil {
		t.Fatalf("ToSerializable failed: %v", err)
	}

	if len(serializable.NextWorkItems) != 2 {
		t.Errorf("Expected 2 next work items, got %d", len(serializable.NextWorkItems))
	}

	if serializable.NextWorkItems[0].ActivityTypeName != "SerializableSuccessActivity" {
		t.Errorf("Expected activity type 'SerializableSuccessActivity', got '%s'",
			serializable.NextWorkItems[0].ActivityTypeName)
	}

	if serializable.NextWorkItems[0].Arguments["a"] != 1 {
		t.Errorf("Expected argument a=1, got %v", serializable.NextWorkItems[0].Arguments["a"])
	}
}

func TestRoutingSlip_ToSerializable_WithCompletedWork(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSerializableSuccessActivity(&callCount, &compensateCount)

	resolver := NewMapBasedResolver()
	resolver.Register("SerializableSuccessActivity", activityType)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{"x": "test"}),
	})

	ctx := context.Background()
	slip.ProcessNext(ctx)

	serializable, err := slip.ToSerializable(resolver)
	if err != nil {
		t.Fatalf("ToSerializable failed: %v", err)
	}

	if len(serializable.CompletedWorkLogs) != 1 {
		t.Errorf("Expected 1 completed work log, got %d", len(serializable.CompletedWorkLogs))
	}

	if serializable.CompletedWorkLogs[0].ActivityTypeName != "SerializableSuccessActivity" {
		t.Errorf("Expected activity type 'SerializableSuccessActivity', got '%s'",
			serializable.CompletedWorkLogs[0].ActivityTypeName)
	}

	if serializable.CompletedWorkLogs[0].Result["id"] != 1 {
		t.Errorf("Expected result id=1, got %v", serializable.CompletedWorkLogs[0].Result["id"])
	}
}

func TestRoutingSlip_FromSerializable_EmptySlip(t *testing.T) {
	resolver := NewMapBasedResolver()
	serializable := &SerializableRoutingSlip{
		CompletedWorkLogs: []SerializableWorkLog{},
		NextWorkItems:     []SerializableWorkItem{},
	}

	slip, err := FromSerializable(serializable, resolver)
	if err != nil {
		t.Fatalf("FromSerializable failed: %v", err)
	}

	if !slip.IsCompleted() {
		t.Error("Expected slip to be completed")
	}
	if slip.IsInProgress() {
		t.Error("Expected slip to not be in progress")
	}
}

func TestRoutingSlip_FromSerializable_WithPendingItems(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSerializableSuccessActivity(&callCount, &compensateCount)

	resolver := NewMapBasedResolver()
	resolver.Register("SerializableSuccessActivity", activityType)

	serializable := &SerializableRoutingSlip{
		CompletedWorkLogs: []SerializableWorkLog{},
		NextWorkItems: []SerializableWorkItem{
			{
				ActivityTypeName: "SerializableSuccessActivity",
				Arguments:        WorkItemArguments{"a": 1},
			},
			{
				ActivityTypeName: "SerializableSuccessActivity",
				Arguments:        WorkItemArguments{"b": 2},
			},
		},
	}

	slip, err := FromSerializable(serializable, resolver)
	if err != nil {
		t.Fatalf("FromSerializable failed: %v", err)
	}

	if slip.IsCompleted() {
		t.Error("Expected slip to not be completed")
	}

	if len(slip.PendingWorkItems()) != 2 {
		t.Errorf("Expected 2 pending work items, got %d", len(slip.PendingWorkItems()))
	}

	if slip.PendingWorkItems()[0].Arguments()["a"] != 1 {
		t.Errorf("Expected argument a=1, got %v", slip.PendingWorkItems()[0].Arguments()["a"])
	}
}

func TestRoutingSlip_FromSerializable_WithCompletedWork(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSerializableSuccessActivity(&callCount, &compensateCount)

	resolver := NewMapBasedResolver()
	resolver.Register("SerializableSuccessActivity", activityType)

	serializable := &SerializableRoutingSlip{
		CompletedWorkLogs: []SerializableWorkLog{
			{
				ActivityTypeName: "SerializableSuccessActivity",
				Result:           WorkResult{"id": 42},
			},
		},
		NextWorkItems: []SerializableWorkItem{},
	}

	slip, err := FromSerializable(serializable, resolver)
	if err != nil {
		t.Fatalf("FromSerializable failed: %v", err)
	}

	if !slip.IsInProgress() {
		t.Error("Expected slip to be in progress")
	}

	if len(slip.CompletedWorkLogs()) != 1 {
		t.Errorf("Expected 1 completed work log, got %d", len(slip.CompletedWorkLogs()))
	}

	if slip.CompletedWorkLogs()[0].Result()["id"] != 42 {
		t.Errorf("Expected result id=42, got %v", slip.CompletedWorkLogs()[0].Result()["id"])
	}
}

func TestRoutingSlip_RoundTrip_Serialization(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSerializableSuccessActivity(&callCount, &compensateCount)

	resolver := NewMapBasedResolver()
	resolver.Register("SerializableSuccessActivity", activityType)

	// Create a routing slip with work
	original := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{"step": 1}),
		NewWorkItem(activityType, WorkItemArguments{"step": 2}),
		NewWorkItem(activityType, WorkItemArguments{"step": 3}),
	})

	ctx := context.Background()
	original.ProcessNext(ctx) // Process first item

	// Serialize
	serializable, err := original.ToSerializable(resolver)
	if err != nil {
		t.Fatalf("ToSerializable failed: %v", err)
	}

	// Deserialize
	restored, err := FromSerializable(serializable, resolver)
	if err != nil {
		t.Fatalf("FromSerializable failed: %v", err)
	}

	// Verify state
	if len(restored.CompletedWorkLogs()) != 1 {
		t.Errorf("Expected 1 completed work log, got %d", len(restored.CompletedWorkLogs()))
	}
	if len(restored.PendingWorkItems()) != 2 {
		t.Errorf("Expected 2 pending work items, got %d", len(restored.PendingWorkItems()))
	}

	// Continue processing
	restored.ProcessNext(ctx)
	restored.ProcessNext(ctx)

	if !restored.IsCompleted() {
		t.Error("Expected restored slip to be completed after processing all items")
	}
	if len(restored.CompletedWorkLogs()) != 3 {
		t.Errorf("Expected 3 completed work logs, got %d", len(restored.CompletedWorkLogs()))
	}
}

func TestRoutingSlip_JSON_Marshaling(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSerializableSuccessActivity(&callCount, &compensateCount)

	resolver := NewMapBasedResolver()
	resolver.Register("SerializableSuccessActivity", activityType)

	// Create routing slip
	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{"key": "value"}),
	})

	ctx := context.Background()
	slip.ProcessNext(ctx)

	// Serialize to SerializableRoutingSlip first
	serializable, err := slip.ToSerializable(resolver)
	if err != nil {
		t.Fatalf("ToSerializable failed: %v", err)
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(serializable)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	// Unmarshal from JSON
	var restored SerializableRoutingSlip
	err = json.Unmarshal(jsonData, &restored)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Convert back to RoutingSlip
	restoredSlip, err := FromSerializable(&restored, resolver)
	if err != nil {
		t.Fatalf("FromSerializable failed: %v", err)
	}

	// Verify
	if len(restoredSlip.CompletedWorkLogs()) != 1 {
		t.Errorf("Expected 1 completed work log, got %d", len(restoredSlip.CompletedWorkLogs()))
	}
}

func TestRoutingSlip_FromSerializable_UnregisteredActivityType(t *testing.T) {
	resolver := NewMapBasedResolver()

	serializable := &SerializableRoutingSlip{
		CompletedWorkLogs: []SerializableWorkLog{},
		NextWorkItems: []SerializableWorkItem{
			{
				ActivityTypeName: "UnregisteredActivity",
				Arguments:        WorkItemArguments{},
			},
		},
	}

	_, err := FromSerializable(serializable, resolver)
	if err == nil {
		t.Error("Expected error when deserializing unregistered activity type")
	}
}

func TestRoutingSlip_ToSerializable_UnregisteredButNamedActivityType(t *testing.T) {
	// Create an activity that is NOT registered but implements NamedActivity
	callCount := 0
	compensateCount := 0
	activityType := newSerializableSuccessActivity(&callCount, &compensateCount)

	resolver := NewMapBasedResolver()
	// Intentionally NOT registering the activity

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{}),
	})

	// Should work because the activity implements NamedActivity (fallback)
	serializable, err := slip.ToSerializable(resolver)
	if err != nil {
		t.Errorf("Expected no error when serializing NamedActivity, got: %v", err)
	}

	if serializable.NextWorkItems[0].ActivityTypeName != "SerializableSuccessActivity" {
		t.Errorf("Expected activity type name 'SerializableSuccessActivity', got '%s'",
			serializable.NextWorkItems[0].ActivityTypeName)
	}

	// However, deserialization should fail without registration
	_, err = FromSerializable(serializable, resolver)
	if err == nil {
		t.Error("Expected error when deserializing unregistered activity type")
	}
}
