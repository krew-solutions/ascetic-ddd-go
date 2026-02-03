package saga

import (
	"context"
	"errors"
	"testing"
)

type successActivity struct {
	callCount       *int
	compensateCount *int
}

func newSuccessActivity(callCount, compensateCount *int) ActivityType {
	return func() Activity {
		return &successActivity{
			callCount:       callCount,
			compensateCount: compensateCount,
		}
	}
}

func (s *successActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	*s.callCount++
	workLog := NewWorkLog(s, WorkResult{"id": *s.callCount})
	return &workLog, nil
}

func (s *successActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	*s.compensateCount++
	return true, nil
}

func (s *successActivity) WorkItemQueueAddress() string {
	return "sb://./success"
}

func (s *successActivity) CompensationQueueAddress() string {
	return "sb://./successCompensation"
}

func (s *successActivity) ActivityType() ActivityType {
	return newSuccessActivity(s.callCount, s.compensateCount)
}

type failingActivity struct{}

func newFailingActivity() Activity {
	return &failingActivity{}
}

func (f *failingActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	return nil, errors.New("intentional failure")
}

func (f *failingActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	return true, nil
}

func (f *failingActivity) WorkItemQueueAddress() string {
	return "sb://./failing"
}

func (f *failingActivity) CompensationQueueAddress() string {
	return "sb://./failingCompensation"
}

func (f *failingActivity) ActivityType() ActivityType {
	return newFailingActivity
}

func TestRoutingSlip_CreateEmpty(t *testing.T) {
	slip := NewRoutingSlip(nil)
	if !slip.IsCompleted() {
		t.Error("Expected empty routing slip to be completed")
	}
	if slip.IsInProgress() {
		t.Error("Expected empty routing slip to not be in progress")
	}
}

func TestRoutingSlip_CreateWithWorkItems(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSuccessActivity(&callCount, &compensateCount)

	workItems := []WorkItem{
		NewWorkItem(activityType, WorkItemArguments{"a": 1}),
		NewWorkItem(activityType, WorkItemArguments{"b": 2}),
	}
	slip := NewRoutingSlip(workItems)

	if slip.IsCompleted() {
		t.Error("Expected routing slip with items to not be completed")
	}
	if slip.IsInProgress() {
		t.Error("Expected routing slip to not be in progress initially")
	}
}

func TestRoutingSlip_ProcessNextSuccess(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSuccessActivity(&callCount, &compensateCount)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{}),
	})

	ctx := context.Background()
	result, err := ProcessNextForTest(ctx, slip, nil)
	if err != nil {
		t.Fatalf("ProcessNext returned error: %v", err)
	}

	if !result {
		t.Error("Expected ProcessNext to return true on success")
	}
	if !slip.IsCompleted() {
		t.Error("Expected routing slip to be completed")
	}
	if !slip.IsInProgress() {
		t.Error("Expected routing slip to be in progress")
	}
}

func TestRoutingSlip_ProcessNextFailure(t *testing.T) {
	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(newFailingActivity, WorkItemArguments{}),
	})

	ctx := context.Background()
	result, err := ProcessNextForTest(ctx, slip, nil)
	if err != nil {
		t.Fatalf("ProcessNext returned error: %v", err)
	}

	if result {
		t.Error("Expected ProcessNext to return false on failure")
	}
	if !slip.IsCompleted() {
		t.Error("Expected routing slip to be completed")
	}
	if slip.IsInProgress() {
		t.Error("Expected routing slip to not be in progress")
	}
}

func TestRoutingSlip_ProcessNextOnEmptyRaisesError(t *testing.T) {
	slip := NewRoutingSlip(nil)
	ctx := context.Background()

	_, err := ProcessNextForTest(ctx, slip, nil)
	if err != ErrInvalidOperation {
		t.Errorf("Expected ErrInvalidOperation, got %v", err)
	}
}

func TestRoutingSlip_ProcessMultipleItems(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSuccessActivity(&callCount, &compensateCount)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{}),
		NewWorkItem(activityType, WorkItemArguments{}),
		NewWorkItem(activityType, WorkItemArguments{}),
	})

	ctx := context.Background()

	ProcessNextForTest(ctx, slip, nil)
	if slip.IsCompleted() {
		t.Error("Expected routing slip to not be completed after first item")
	}
	if len(slip.CompletedWorkLogs()) != 1 {
		t.Errorf("Expected 1 completed work log, got %d", len(slip.CompletedWorkLogs()))
	}

	ProcessNextForTest(ctx, slip, nil)
	if slip.IsCompleted() {
		t.Error("Expected routing slip to not be completed after second item")
	}
	if len(slip.CompletedWorkLogs()) != 2 {
		t.Errorf("Expected 2 completed work logs, got %d", len(slip.CompletedWorkLogs()))
	}

	ProcessNextForTest(ctx, slip, nil)
	if !slip.IsCompleted() {
		t.Error("Expected routing slip to be completed after third item")
	}
	if len(slip.CompletedWorkLogs()) != 3 {
		t.Errorf("Expected 3 completed work logs, got %d", len(slip.CompletedWorkLogs()))
	}
}

func TestRoutingSlip_UndoLastSuccess(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSuccessActivity(&callCount, &compensateCount)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{}),
	})
	ctx := context.Background()
	ProcessNextForTest(ctx, slip, nil)

	result, err := UndoLastForTest(ctx, slip, nil)
	if err != nil {
		t.Fatalf("UndoLast returned error: %v", err)
	}

	if !result {
		t.Error("Expected UndoLast to return true")
	}
	if slip.IsInProgress() {
		t.Error("Expected routing slip to not be in progress after undo")
	}
	if compensateCount != 1 {
		t.Errorf("Expected compensate count 1, got %d", compensateCount)
	}
}

func TestRoutingSlip_UndoLastOnEmptyRaisesError(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSuccessActivity(&callCount, &compensateCount)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{}),
	})
	ctx := context.Background()

	_, err := UndoLastForTest(ctx, slip, nil)
	if err != ErrInvalidOperation {
		t.Errorf("Expected ErrInvalidOperation, got %v", err)
	}
}

func TestRoutingSlip_UndoMultipleItems(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSuccessActivity(&callCount, &compensateCount)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{}),
		NewWorkItem(activityType, WorkItemArguments{}),
		NewWorkItem(activityType, WorkItemArguments{}),
	})
	ctx := context.Background()
	ProcessNextForTest(ctx, slip, nil)
	ProcessNextForTest(ctx, slip, nil)
	ProcessNextForTest(ctx, slip, nil)

	if len(slip.CompletedWorkLogs()) != 3 {
		t.Errorf("Expected 3 completed work logs, got %d", len(slip.CompletedWorkLogs()))
	}

	UndoLastForTest(ctx, slip, nil)
	if len(slip.CompletedWorkLogs()) != 2 {
		t.Errorf("Expected 2 completed work logs after first undo, got %d", len(slip.CompletedWorkLogs()))
	}

	UndoLastForTest(ctx, slip, nil)
	if len(slip.CompletedWorkLogs()) != 1 {
		t.Errorf("Expected 1 completed work log after second undo, got %d", len(slip.CompletedWorkLogs()))
	}

	UndoLastForTest(ctx, slip, nil)
	if len(slip.CompletedWorkLogs()) != 0 {
		t.Errorf("Expected 0 completed work logs after third undo, got %d", len(slip.CompletedWorkLogs()))
	}
	if slip.IsInProgress() {
		t.Error("Expected routing slip to not be in progress")
	}
}

func TestRoutingSlip_ProgressUriReturnsNextActivityQueue(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSuccessActivity(&callCount, &compensateCount)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{}),
	})

	if slip.ProgressUri() != "sb://./success" {
		t.Errorf("Expected progress URI 'sb://./success', got '%s'", slip.ProgressUri())
	}
}

func TestRoutingSlip_ProgressUriReturnsEmptyWhenCompleted(t *testing.T) {
	slip := NewRoutingSlip(nil)

	if slip.ProgressUri() != "" {
		t.Errorf("Expected empty progress URI, got '%s'", slip.ProgressUri())
	}
}

func TestRoutingSlip_CompensationUriReturnsLastActivityQueue(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSuccessActivity(&callCount, &compensateCount)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{}),
	})
	ctx := context.Background()
	ProcessNextForTest(ctx, slip, nil)

	if slip.CompensationUri() != "sb://./successCompensation" {
		t.Errorf("Expected compensation URI 'sb://./successCompensation', got '%s'", slip.CompensationUri())
	}
}

func TestRoutingSlip_CompensationUriReturnsEmptyWhenNotStarted(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSuccessActivity(&callCount, &compensateCount)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{}),
	})

	if slip.CompensationUri() != "" {
		t.Errorf("Expected empty compensation URI, got '%s'", slip.CompensationUri())
	}
}

func TestRoutingSlip_SuccessfulSaga(t *testing.T) {
	callCount := 0
	compensateCount := 0
	activityType := newSuccessActivity(&callCount, &compensateCount)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(activityType, WorkItemArguments{}),
		NewWorkItem(activityType, WorkItemArguments{}),
		NewWorkItem(activityType, WorkItemArguments{}),
	})
	ctx := context.Background()

	for !slip.IsCompleted() {
		ProcessNextForTest(ctx, slip, nil)
	}

	if !slip.IsCompleted() {
		t.Error("Expected routing slip to be completed")
	}
	if !slip.IsInProgress() {
		t.Error("Expected routing slip to be in progress")
	}
	if len(slip.CompletedWorkLogs()) != 3 {
		t.Errorf("Expected 3 completed work logs, got %d", len(slip.CompletedWorkLogs()))
	}
}

func TestRoutingSlip_FailedSagaWithCompensation(t *testing.T) {
	callCount := 0
	compensateCount := 0
	successType := newSuccessActivity(&callCount, &compensateCount)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(successType, WorkItemArguments{}),
		NewWorkItem(successType, WorkItemArguments{}),
		NewWorkItem(newFailingActivity, WorkItemArguments{}),
	})
	ctx := context.Background()

	// Process until failure
	for !slip.IsCompleted() {
		success, _ := ProcessNextForTest(ctx, slip, nil)
		if !success {
			break
		}
	}

	// Compensate
	for slip.IsInProgress() {
		UndoLastForTest(ctx, slip, nil)
	}

	if slip.IsInProgress() {
		t.Error("Expected routing slip to not be in progress after compensation")
	}
	if compensateCount != 2 {
		t.Errorf("Expected 2 compensations, got %d", compensateCount)
	}
}
