package examples

import (
	"context"
	"testing"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/saga"
)

func TestReserveCarActivity_DoWorkCreatesReservation(t *testing.T) {
	activity := NewReserveCarActivity()
	workItem := saga.NewWorkItem(
		NewReserveCarActivity,
		saga.WorkItemArguments{"vehicleType": "Compact"},
	)

	ctx := context.Background()
	result, err := activity.DoWork(ctx, workItem)
	if err != nil {
		t.Fatalf("DoWork returned error: %v", err)
	}

	if _, ok := result.Result()["reservationId"]; !ok {
		t.Error("Expected reservationId in result")
	}
	if _, ok := result.Result()["reservationId"].(int); !ok {
		t.Error("Expected reservationId to be int")
	}
}

func TestReserveCarActivity_CompensateReturnsTrue(t *testing.T) {
	activity := NewReserveCarActivity()
	workItem := saga.NewWorkItem(
		NewReserveCarActivity,
		saga.WorkItemArguments{"vehicleType": "SUV"},
	)

	ctx := context.Background()
	workLog, _ := activity.DoWork(ctx, workItem)
	routingSlip := saga.NewRoutingSlip(nil)

	result, err := activity.Compensate(ctx, *workLog, routingSlip)
	if err != nil {
		t.Fatalf("Compensate returned error: %v", err)
	}

	if !result {
		t.Error("Expected compensate to return true")
	}
}

func TestReserveCarActivity_QueueAddresses(t *testing.T) {
	activity := NewReserveCarActivity()

	if activity.WorkItemQueueAddress() != "sb://./carReservations" {
		t.Errorf("Expected work queue 'sb://./carReservations', got '%s'", activity.WorkItemQueueAddress())
	}
	if activity.CompensationQueueAddress() != "sb://./carCancellations" {
		t.Errorf("Expected compensation queue 'sb://./carCancellations', got '%s'", activity.CompensationQueueAddress())
	}
}

func TestReserveHotelActivity_DoWorkCreatesReservation(t *testing.T) {
	activity := NewReserveHotelActivity()
	workItem := saga.NewWorkItem(
		NewReserveHotelActivity,
		saga.WorkItemArguments{"roomType": "Suite"},
	)

	ctx := context.Background()
	result, err := activity.DoWork(ctx, workItem)
	if err != nil {
		t.Fatalf("DoWork returned error: %v", err)
	}

	if _, ok := result.Result()["reservationId"]; !ok {
		t.Error("Expected reservationId in result")
	}
	if _, ok := result.Result()["reservationId"].(int); !ok {
		t.Error("Expected reservationId to be int")
	}
}

func TestReserveHotelActivity_CompensateReturnsTrue(t *testing.T) {
	activity := NewReserveHotelActivity()
	workItem := saga.NewWorkItem(
		NewReserveHotelActivity,
		saga.WorkItemArguments{"roomType": "Standard"},
	)

	ctx := context.Background()
	workLog, _ := activity.DoWork(ctx, workItem)
	routingSlip := saga.NewRoutingSlip(nil)

	result, err := activity.Compensate(ctx, *workLog, routingSlip)
	if err != nil {
		t.Fatalf("Compensate returned error: %v", err)
	}

	if !result {
		t.Error("Expected compensate to return true")
	}
}

func TestReserveHotelActivity_QueueAddresses(t *testing.T) {
	activity := NewReserveHotelActivity()

	if activity.WorkItemQueueAddress() != "sb://./hotelReservations" {
		t.Errorf("Expected work queue 'sb://./hotelReservations', got '%s'", activity.WorkItemQueueAddress())
	}
	if activity.CompensationQueueAddress() != "sb://./hotelCancellations" {
		t.Errorf("Expected compensation queue 'sb://./hotelCancellations', got '%s'", activity.CompensationQueueAddress())
	}
}

func TestReserveFlightActivity_DoWorkCreatesReservation(t *testing.T) {
	activity := NewReserveFlightActivity()
	workItem := saga.NewWorkItem(
		NewReserveFlightActivity,
		saga.WorkItemArguments{"destination": "DUS"},
	)

	ctx := context.Background()
	result, err := activity.DoWork(ctx, workItem)
	if err != nil {
		t.Fatalf("DoWork returned error: %v", err)
	}

	if _, ok := result.Result()["reservationId"]; !ok {
		t.Error("Expected reservationId in result")
	}
	if _, ok := result.Result()["reservationId"].(int); !ok {
		t.Error("Expected reservationId to be int")
	}
}

func TestReserveFlightActivity_CompensateReturnsTrue(t *testing.T) {
	activity := NewReserveFlightActivity()
	workItem := saga.NewWorkItem(
		NewReserveFlightActivity,
		saga.WorkItemArguments{"destination": "FRA"},
	)

	ctx := context.Background()
	workLog, _ := activity.DoWork(ctx, workItem)
	routingSlip := saga.NewRoutingSlip(nil)

	result, err := activity.Compensate(ctx, *workLog, routingSlip)
	if err != nil {
		t.Fatalf("Compensate returned error: %v", err)
	}

	if !result {
		t.Error("Expected compensate to return true")
	}
}

func TestReserveFlightActivity_QueueAddresses(t *testing.T) {
	activity := NewReserveFlightActivity()

	if activity.WorkItemQueueAddress() != "sb://./flightReservations" {
		t.Errorf("Expected work queue 'sb://./flightReservations', got '%s'", activity.WorkItemQueueAddress())
	}
	if activity.CompensationQueueAddress() != "sb://./flightCancellations" {
		t.Errorf("Expected compensation queue 'sb://./flightCancellations', got '%s'", activity.CompensationQueueAddress())
	}
}

func TestSafeFailingReserveFlightActivity_DoWorkFails(t *testing.T) {
	activity := NewSafeFailingReserveFlightActivity()
	workItem := saga.NewWorkItem(
		NewSafeFailingReserveFlightActivity,
		saga.WorkItemArguments{"destination": "DUS"},
	)

	ctx := context.Background()
	result, err := activity.DoWork(ctx, workItem)

	if err == nil {
		t.Error("Expected DoWork to return error")
	}
	if result != nil {
		t.Error("Expected nil result on failure")
	}
}

func TestSafeFailingReserveFlightActivity_InheritsQueueAddresses(t *testing.T) {
	activity := NewSafeFailingReserveFlightActivity()

	if activity.WorkItemQueueAddress() != "sb://./flightReservations" {
		t.Errorf("Expected inherited work queue, got '%s'", activity.WorkItemQueueAddress())
	}
	if activity.CompensationQueueAddress() != "sb://./flightCancellations" {
		t.Errorf("Expected inherited compensation queue, got '%s'", activity.CompensationQueueAddress())
	}
}

func TestTravelBookingSaga_SuccessfulBooking(t *testing.T) {
	slip := saga.NewRoutingSlip([]saga.WorkItem{
		saga.NewWorkItem(NewReserveCarActivity, saga.WorkItemArguments{"vehicleType": "Compact"}),
		saga.NewWorkItem(NewReserveHotelActivity, saga.WorkItemArguments{"roomType": "Suite"}),
		saga.NewWorkItem(NewReserveFlightActivity, saga.WorkItemArguments{"destination": "DUS"}),
	})

	ctx := context.Background()

	for !slip.IsCompleted() {
		result, err := slip.ProcessNext(ctx)
		if err != nil {
			t.Fatalf("ProcessNext returned error: %v", err)
		}
		if !result {
			t.Error("Expected all steps to succeed")
		}
	}

	if !slip.IsCompleted() {
		t.Error("Expected routing slip to be completed")
	}
	if len(slip.CompletedWorkLogs()) != 3 {
		t.Errorf("Expected 3 completed work logs, got %d", len(slip.CompletedWorkLogs()))
	}
}

func TestTravelBookingSaga_FailedBookingTriggersCompensation(t *testing.T) {
	slip := saga.NewRoutingSlip([]saga.WorkItem{
		saga.NewWorkItem(NewReserveCarActivity, saga.WorkItemArguments{"vehicleType": "Compact"}),
		saga.NewWorkItem(NewReserveHotelActivity, saga.WorkItemArguments{"roomType": "Suite"}),
		saga.NewWorkItem(NewSafeFailingReserveFlightActivity, saga.WorkItemArguments{"destination": "DUS"}),
	})

	ctx := context.Background()

	// Process until failure
	completedBeforeFailure := 0
	for !slip.IsCompleted() {
		result, _ := slip.ProcessNext(ctx)
		if result {
			completedBeforeFailure++
		} else {
			break
		}
	}

	if completedBeforeFailure != 2 {
		t.Errorf("Expected 2 completions before failure, got %d", completedBeforeFailure)
	}

	// Compensate
	compensated := 0
	for slip.IsInProgress() {
		slip.UndoLast(ctx)
		compensated++
	}

	if compensated != 2 {
		t.Errorf("Expected 2 compensations, got %d", compensated)
	}
	if slip.IsInProgress() {
		t.Error("Expected routing slip to not be in progress after compensation")
	}
}
