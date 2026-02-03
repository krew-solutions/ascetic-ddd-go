package saga

import (
	"context"
	"encoding/json"
	"testing"
)

// TestRoutingSlip_JsonSerialization verifies that RoutingSlip can be serialized to JSON.
func TestRoutingSlip_JsonSerialization(t *testing.T) {
	// Create a routing slip with work items
	activityType := newSuccessActivity(new(int), new(int))
	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{"vehicle": "Compact"}),
		NewWorkItem(activityType, WorkItemArguments{"room": "Suite"}),
	})

	// Execute one item to add a work log
	ProcessNextForTest(context.Background(), slip, nil)

	// Serialize to JSON
	jsonData, err := json.Marshal(slip)
	if err != nil {
		t.Fatalf("Failed to marshal routing slip: %v", err)
	}

	// Verify JSON contains expected fields
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if parsed["nextWorkItems"] == nil {
		t.Error("Expected nextWorkItems in JSON")
	}
	if parsed["completedWorkLogs"] == nil {
		t.Error("Expected completedWorkLogs in JSON")
	}
}

// TestWorkItem_JsonSerialization verifies that WorkItem serializes only the queue address.
func TestWorkItem_JsonSerialization(t *testing.T) {
	activityType := newSuccessActivity(new(int), new(int))
	workItem := NewWorkItem(activityType, WorkItemArguments{"key": "value"})

	jsonData, err := json.Marshal(workItem)
	if err != nil {
		t.Fatalf("Failed to marshal work item: %v", err)
	}

	// Verify JSON structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Should have queueAddress and arguments
	if parsed["queueAddress"] == nil {
		t.Error("Expected queueAddress in JSON")
	}
	if parsed["arguments"] == nil {
		t.Error("Expected arguments in JSON")
	}

	// Should NOT have activityType (it's not exported)
	if parsed["activityType"] != nil {
		t.Error("Expected activityType to NOT be in JSON (not exported)")
	}
}

// TestWorkLog_JsonSerialization verifies that WorkLog serializes only the compensation address.
func TestWorkLog_JsonSerialization(t *testing.T) {
	activity := newSuccessActivity(new(int), new(int))()
	workLog := NewWorkLog(activity, WorkResult{"reservationId": 12345})

	jsonData, err := json.Marshal(workLog)
	if err != nil {
		t.Fatalf("Failed to marshal work log: %v", err)
	}

	// Verify JSON structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Should have compensationQueueAddress and result
	if parsed["compensationQueueAddress"] == nil {
		t.Error("Expected compensationQueueAddress in JSON")
	}
	if parsed["result"] == nil {
		t.Error("Expected result in JSON")
	}

	// Should NOT have activityType (it's not exported)
	if parsed["activityType"] != nil {
		t.Error("Expected activityType to NOT be in JSON (not exported)")
	}
}

// TestRoutingSlip_RoundTripSerialization verifies serialize -> deserialize works.
func TestRoutingSlip_RoundTripSerialization(t *testing.T) {
	activityType := newSuccessActivity(new(int), new(int))

	// Create original routing slip
	original := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{"vehicle": "Compact"}),
		NewWorkItem(activityType, WorkItemArguments{"room": "Suite"}),
	})

	// Execute one item
	ProcessNextForTest(context.Background(), original, nil)

	// Serialize
	jsonData, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	// Deserialize
	var deserialized RoutingSlip
	if err := json.Unmarshal(jsonData, &deserialized); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Verify state is preserved
	if len(deserialized.PendingWorkItems()) != len(original.PendingWorkItems()) {
		t.Errorf("Expected %d pending items, got %d",
			len(original.PendingWorkItems()),
			len(deserialized.PendingWorkItems()))
	}

	if len(deserialized.CompletedWorkLogs()) != len(original.CompletedWorkLogs()) {
		t.Errorf("Expected %d completed logs, got %d",
			len(original.CompletedWorkLogs()),
			len(deserialized.CompletedWorkLogs()))
	}

	// Verify queue addresses are preserved
	if deserialized.ProgressUri() != original.ProgressUri() {
		t.Errorf("Expected ProgressUri '%s', got '%s'",
			original.ProgressUri(),
			deserialized.ProgressUri())
	}

	if deserialized.CompensationUri() != original.CompensationUri() {
		t.Errorf("Expected CompensationUri '%s', got '%s'",
			original.CompensationUri(),
			deserialized.CompensationUri())
	}
}
