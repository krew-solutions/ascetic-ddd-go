package examples

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/saga"
)

// This example demonstrates how to serialize and deserialize a routing slip
// for transmission over a message bus.
func TestSerializationExample_TravelBookingSaga(t *testing.T) {
	// Step 1: Create a resolver and register all activity types
	resolver := saga.NewMapBasedResolver()
	resolver.Register("ReserveCarActivity", NewReserveCarActivity)
	resolver.Register("ReserveHotelActivity", NewReserveHotelActivity)
	resolver.Register("ReserveFlightActivity", NewReserveFlightActivity)

	// Step 2: Create a routing slip with multiple activities
	routingSlip := saga.NewRoutingSlip([]saga.WorkItem{
		saga.NewWorkItem(NewReserveCarActivity, saga.WorkItemArguments{
			"vehicleType": "SUV",
			"pickupDate":  "2024-01-15",
		}),
		saga.NewWorkItem(NewReserveHotelActivity, saga.WorkItemArguments{
			"roomType":    "Suite",
			"checkInDate": "2024-01-15",
		}),
		saga.NewWorkItem(NewReserveFlightActivity, saga.WorkItemArguments{
			"destination": "LAX",
			"flightDate":  "2024-01-15",
		}),
	})

	// Step 3: Process the first activity (car reservation)
	ctx := context.Background()
	success, err := routingSlip.ProcessNext(ctx)
	if err != nil {
		t.Fatalf("ProcessNext failed: %v", err)
	}
	if !success {
		t.Fatal("Expected car reservation to succeed")
	}

	// Step 4: Serialize the routing slip for transmission
	serializable, err := routingSlip.ToSerializable(resolver)
	if err != nil {
		t.Fatalf("ToSerializable failed: %v", err)
	}

	jsonData, err := json.Marshal(serializable)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	t.Logf("Serialized routing slip: %s", string(jsonData))

	// Step 5: Simulate sending over message bus and receiving on another service
	// ... bus.Publish("saga/routing-slip", jsonData) ...
	// ... receivedData := bus.Receive("saga/routing-slip") ...

	// Step 6: Deserialize on the receiving end
	var receivedSerializable saga.SerializableRoutingSlip
	err = json.Unmarshal(jsonData, &receivedSerializable)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Step 7: Convert back to RoutingSlip (requires resolver on receiving end too)
	restoredSlip, err := saga.FromSerializable(&receivedSerializable, resolver)
	if err != nil {
		t.Fatalf("FromSerializable failed: %v", err)
	}

	// Step 8: Verify the state was preserved
	if !restoredSlip.IsInProgress() {
		t.Error("Expected restored slip to be in progress")
	}

	if len(restoredSlip.CompletedWorkLogs()) != 1 {
		t.Errorf("Expected 1 completed work log, got %d", len(restoredSlip.CompletedWorkLogs()))
	}

	if len(restoredSlip.PendingWorkItems()) != 2 {
		t.Errorf("Expected 2 pending work items, got %d", len(restoredSlip.PendingWorkItems()))
	}

	// Step 9: Continue processing on the receiving service
	success, err = restoredSlip.ProcessNext(ctx)
	if err != nil {
		t.Fatalf("ProcessNext on restored slip failed: %v", err)
	}
	if !success {
		t.Fatal("Expected hotel reservation to succeed")
	}

	// Verify progress
	if len(restoredSlip.CompletedWorkLogs()) != 2 {
		t.Errorf("Expected 2 completed work logs, got %d", len(restoredSlip.CompletedWorkLogs()))
	}

	// Check next activity queue
	nextQueue := restoredSlip.ProgressUri()
	if nextQueue != "sb://./flightReservations" {
		t.Errorf("Expected next queue 'sb://./flightReservations', got '%s'", nextQueue)
	}
}

// This example demonstrates compensation serialization
func TestSerializationExample_CompensationFlow(t *testing.T) {
	resolver := saga.NewMapBasedResolver()
	resolver.Register("ReserveCarActivity", NewReserveCarActivity)
	resolver.Register("ReserveHotelActivity", NewReserveHotelActivity)
	resolver.Register("SafeFailingReserveFlightActivity", NewSafeFailingReserveFlightActivity)

	// Create saga with a failing activity at the end
	routingSlip := saga.NewRoutingSlip([]saga.WorkItem{
		saga.NewWorkItem(NewReserveCarActivity, saga.WorkItemArguments{"vehicleType": "SUV"}),
		saga.NewWorkItem(NewReserveHotelActivity, saga.WorkItemArguments{"roomType": "Suite"}),
		saga.NewWorkItem(NewSafeFailingReserveFlightActivity, saga.WorkItemArguments{"destination": "LAX"}),
	})

	ctx := context.Background()

	// Process activities until failure
	for !routingSlip.IsCompleted() {
		success, err := routingSlip.ProcessNext(ctx)
		if err != nil {
			t.Fatalf("ProcessNext failed: %v", err)
		}
		if !success {
			// Failure detected, need to compensate
			t.Logf("Activity failed, starting compensation")
			break
		}
	}

	// At this point, we need to send the routing slip to compensation queue
	// Serialize before sending
	serializable, err := routingSlip.ToSerializable(resolver)
	if err != nil {
		t.Fatalf("ToSerializable failed: %v", err)
	}

	jsonData, err := json.Marshal(serializable)
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	t.Logf("Serialized routing slip for compensation: %s", string(jsonData))

	// Simulate receiving on compensation service
	var receivedSerializable saga.SerializableRoutingSlip
	err = json.Unmarshal(jsonData, &receivedSerializable)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	restoredSlip, err := saga.FromSerializable(&receivedSerializable, resolver)
	if err != nil {
		t.Fatalf("FromSerializable failed: %v", err)
	}

	// Verify state before compensation
	if !restoredSlip.IsInProgress() {
		t.Error("Expected slip to be in progress before compensation")
	}

	if len(restoredSlip.CompletedWorkLogs()) != 2 {
		t.Errorf("Expected 2 completed work logs, got %d", len(restoredSlip.CompletedWorkLogs()))
	}

	// Perform compensation
	for restoredSlip.IsInProgress() {
		continueBackward, err := restoredSlip.UndoLast(ctx)
		if err != nil {
			t.Fatalf("UndoLast failed: %v", err)
		}
		if !continueBackward {
			break
		}
	}

	// Verify compensation completed
	if restoredSlip.IsInProgress() {
		t.Error("Expected slip to not be in progress after compensation")
	}

	if len(restoredSlip.CompletedWorkLogs()) != 0 {
		t.Errorf("Expected 0 completed work logs after full compensation, got %d",
			len(restoredSlip.CompletedWorkLogs()))
	}
}

// This example shows multiple resolver instances for different contexts
func TestSerializationExample_MultipleResolvers(t *testing.T) {
	// Resolver for service A (only knows about car and hotel)
	resolverA := saga.NewMapBasedResolver()
	resolverA.Register("ReserveCarActivity", NewReserveCarActivity)
	resolverA.Register("ReserveHotelActivity", NewReserveHotelActivity)

	// Resolver for service B (only knows about flight)
	resolverB := saga.NewMapBasedResolver()
	resolverB.Register("ReserveFlightActivity", NewReserveFlightActivity)

	// Resolver for orchestrator (knows about all)
	resolverOrchestrator := saga.NewMapBasedResolver()
	resolverOrchestrator.Register("ReserveCarActivity", NewReserveCarActivity)
	resolverOrchestrator.Register("ReserveHotelActivity", NewReserveHotelActivity)
	resolverOrchestrator.Register("ReserveFlightActivity", NewReserveFlightActivity)

	// Create routing slip on orchestrator
	routingSlip := saga.NewRoutingSlip([]saga.WorkItem{
		saga.NewWorkItem(NewReserveCarActivity, saga.WorkItemArguments{"vehicleType": "SUV"}),
		saga.NewWorkItem(NewReserveFlightActivity, saga.WorkItemArguments{"destination": "LAX"}),
	})

	ctx := context.Background()

	// Process car on service A
	routingSlip.ProcessNext(ctx)
	serializable, _ := routingSlip.ToSerializable(resolverOrchestrator)
	jsonData, _ := json.Marshal(serializable)

	// Service B receives and processes flight
	var receivedSerializable saga.SerializableRoutingSlip
	json.Unmarshal(jsonData, &receivedSerializable)

	// Service B can restore because it has the flight activity registered
	restoredSlip, err := saga.FromSerializable(&receivedSerializable, resolverOrchestrator)
	if err != nil {
		t.Fatalf("FromSerializable failed: %v", err)
	}

	restoredSlip.ProcessNext(ctx)

	if !restoredSlip.IsCompleted() {
		t.Error("Expected slip to be completed")
	}
}
