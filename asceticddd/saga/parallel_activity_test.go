package saga

import (
	"context"
	"testing"
)

type branchAActivity struct {
	callCount       *int
	compensateCount *int
}

func newBranchAActivity(callCount, compensateCount *int) ActivityType {
	return func() Activity {
		return &branchAActivity{
			callCount:       callCount,
			compensateCount: compensateCount,
		}
	}
}

func (a *branchAActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	*a.callCount++
	value := "default"
	if v, ok := workItem.Arguments["value"]; ok {
		value = v.(string)
	}
	workLog := NewWorkLog(a, WorkResult{"branch": "A", "value": value})
	return &workLog, nil
}

func (a *branchAActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	*a.compensateCount++
	return true, nil
}

func (a *branchAActivity) WorkItemQueueAddress() string {
	return "sb://./branchA"
}

func (a *branchAActivity) CompensationQueueAddress() string {
	return "sb://./branchACompensation"
}

func (a *branchAActivity) ActivityType() ActivityType {
	return newBranchAActivity(a.callCount, a.compensateCount)
}

type branchBActivity struct {
	callCount       *int
	compensateCount *int
}

func newBranchBActivity(callCount, compensateCount *int) ActivityType {
	return func() Activity {
		return &branchBActivity{
			callCount:       callCount,
			compensateCount: compensateCount,
		}
	}
}

func (a *branchBActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	*a.callCount++
	value := "default"
	if v, ok := workItem.Arguments["value"]; ok {
		value = v.(string)
	}
	workLog := NewWorkLog(a, WorkResult{"branch": "B", "value": value})
	return &workLog, nil
}

func (a *branchBActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	*a.compensateCount++
	return true, nil
}

func (a *branchBActivity) WorkItemQueueAddress() string {
	return "sb://./branchB"
}

func (a *branchBActivity) CompensationQueueAddress() string {
	return "sb://./branchBCompensation"
}

func (a *branchBActivity) ActivityType() ActivityType {
	return newBranchBActivity(a.callCount, a.compensateCount)
}

type failingBranchActivity struct {
	callCount *int
}

func newFailingBranchActivity(callCount *int) ActivityType {
	return func() Activity {
		return &failingBranchActivity{callCount: callCount}
	}
}

func (a *failingBranchActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	*a.callCount++
	return nil, nil
}

func (a *failingBranchActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	return true, nil
}

func (a *failingBranchActivity) WorkItemQueueAddress() string {
	return "sb://./failing"
}

func (a *failingBranchActivity) CompensationQueueAddress() string {
	return "sb://./failingCompensation"
}

func (a *failingBranchActivity) ActivityType() ActivityType {
	return newFailingBranchActivity(a.callCount)
}

func TestParallelActivity_AllBranchesSucceed(t *testing.T) {
	callCountA := 0
	compensateCountA := 0
	branchAType := newBranchAActivity(&callCountA, &compensateCountA)

	callCountB := 0
	compensateCountB := 0
	branchBType := newBranchBActivity(&callCountB, &compensateCountB)

	activity := NewParallelActivity()
	workItem := NewWorkItem(NewParallelActivity, WorkItemArguments{
		"branches": []*RoutingSlip{
			NewRoutingSlip([]WorkItem{NewWorkItem(branchAType, WorkItemArguments{"value": "a1"})}),
			NewRoutingSlip([]WorkItem{NewWorkItem(branchBType, WorkItemArguments{"value": "b1"})}),
		},
	})

	ctx := context.Background()
	result, err := activity.DoWork(ctx, workItem)
	if err != nil {
		t.Fatalf("DoWork returned error: %v", err)
	}

	if result == nil {
		t.Error("Expected non-nil result")
	}
	if callCountA != 1 {
		t.Errorf("Expected branch A call count 1, got %d", callCountA)
	}
	if callCountB != 1 {
		t.Errorf("Expected branch B call count 1, got %d", callCountB)
	}
}

func TestParallelActivity_MultiStepBranchesSucceed(t *testing.T) {
	callCountA := 0
	compensateCountA := 0
	branchAType := newBranchAActivity(&callCountA, &compensateCountA)

	callCountB := 0
	compensateCountB := 0
	branchBType := newBranchBActivity(&callCountB, &compensateCountB)

	activity := NewParallelActivity()
	workItem := NewWorkItem(NewParallelActivity, WorkItemArguments{
		"branches": []*RoutingSlip{
			NewRoutingSlip([]WorkItem{
				NewWorkItem(branchAType, WorkItemArguments{"value": "a1"}),
				NewWorkItem(branchAType, WorkItemArguments{"value": "a2"}),
			}),
			NewRoutingSlip([]WorkItem{
				NewWorkItem(branchBType, WorkItemArguments{"value": "b1"}),
			}),
		},
	})

	ctx := context.Background()
	result, err := activity.DoWork(ctx, workItem)
	if err != nil {
		t.Fatalf("DoWork returned error: %v", err)
	}

	if result == nil {
		t.Error("Expected non-nil result")
	}
	if callCountA != 2 {
		t.Errorf("Expected branch A call count 2, got %d", callCountA)
	}
	if callCountB != 1 {
		t.Errorf("Expected branch B call count 1, got %d", callCountB)
	}
}

func TestParallelActivity_OneBranchFailsCompensatesAll(t *testing.T) {
	callCountA := 0
	compensateCountA := 0
	branchAType := newBranchAActivity(&callCountA, &compensateCountA)

	callCountB := 0
	compensateCountB := 0
	branchBType := newBranchBActivity(&callCountB, &compensateCountB)

	callCountFail := 0
	failType := newFailingBranchActivity(&callCountFail)

	activity := NewParallelActivity()
	workItem := NewWorkItem(NewParallelActivity, WorkItemArguments{
		"branches": []*RoutingSlip{
			NewRoutingSlip([]WorkItem{
				NewWorkItem(branchAType, WorkItemArguments{"value": "a1"}),
				NewWorkItem(failType, WorkItemArguments{}),
			}),
			NewRoutingSlip([]WorkItem{
				NewWorkItem(branchBType, WorkItemArguments{"value": "b1"}),
			}),
		},
	})

	ctx := context.Background()
	result, err := activity.DoWork(ctx, workItem)
	if err != nil {
		t.Fatalf("DoWork returned error: %v", err)
	}

	if result != nil {
		t.Error("Expected nil result when branch fails")
	}
	if callCountA != 1 {
		t.Errorf("Expected branch A call count 1, got %d", callCountA)
	}
	if compensateCountA != 1 {
		t.Errorf("Expected branch A compensate count 1, got %d", compensateCountA)
	}
}

func TestParallelActivity_CompensateAllBranches(t *testing.T) {
	callCountA := 0
	compensateCountA := 0
	branchAType := newBranchAActivity(&callCountA, &compensateCountA)

	callCountB := 0
	compensateCountB := 0
	branchBType := newBranchBActivity(&callCountB, &compensateCountB)

	activity := NewParallelActivity()
	workItem := NewWorkItem(NewParallelActivity, WorkItemArguments{
		"branches": []*RoutingSlip{
			NewRoutingSlip([]WorkItem{
				NewWorkItem(branchAType, WorkItemArguments{"value": "a"}),
				NewWorkItem(branchAType, WorkItemArguments{"value": "a2"}),
			}),
			NewRoutingSlip([]WorkItem{
				NewWorkItem(branchBType, WorkItemArguments{"value": "b"}),
			}),
		},
	})

	ctx := context.Background()
	result, err := activity.DoWork(ctx, workItem)
	if err != nil {
		t.Fatalf("DoWork returned error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if callCountA != 2 {
		t.Errorf("Expected branch A call count 2, got %d", callCountA)
	}
	if callCountB != 1 {
		t.Errorf("Expected branch B call count 1, got %d", callCountB)
	}

	// Compensate
	compensateResult, err := activity.Compensate(ctx, *result, NewRoutingSlip(nil))
	if err != nil {
		t.Fatalf("Compensate returned error: %v", err)
	}

	if !compensateResult {
		t.Error("Expected compensate to return true")
	}
	if compensateCountA != 2 {
		t.Errorf("Expected branch A compensate count 2, got %d", compensateCountA)
	}
	if compensateCountB != 1 {
		t.Errorf("Expected branch B compensate count 1, got %d", compensateCountB)
	}
}

func TestParallelActivity_QueueAddresses(t *testing.T) {
	activity := NewParallelActivity()
	if activity.WorkItemQueueAddress() != "sb://./parallel" {
		t.Errorf("Expected work queue 'sb://./parallel', got '%s'", activity.WorkItemQueueAddress())
	}
	if activity.CompensationQueueAddress() != "sb://./parallelCompensation" {
		t.Errorf("Expected compensation queue 'sb://./parallelCompensation', got '%s'", activity.CompensationQueueAddress())
	}
}

func TestParallelActivity_InRoutingSlip(t *testing.T) {
	callCountA := 0
	compensateCountA := 0
	branchAType := newBranchAActivity(&callCountA, &compensateCountA)

	callCountB := 0
	compensateCountB := 0
	branchBType := newBranchBActivity(&callCountB, &compensateCountB)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(branchAType, WorkItemArguments{"value": "before"}),
		NewWorkItem(NewParallelActivity, WorkItemArguments{
			"branches": []*RoutingSlip{
				NewRoutingSlip([]WorkItem{
					NewWorkItem(branchAType, WorkItemArguments{"value": "p1"}),
					NewWorkItem(branchAType, WorkItemArguments{"value": "p2"}),
				}),
				NewRoutingSlip([]WorkItem{
					NewWorkItem(branchBType, WorkItemArguments{"value": "p3"}),
				}),
			},
		}),
		NewWorkItem(branchBType, WorkItemArguments{"value": "after"}),
	})

	ctx := context.Background()

	// Execute all steps
	for !slip.IsCompleted() {
		result, err := ProcessNextForTest(ctx, slip, nil)
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
	// BranchA: 1 (before) + 2 (parallel) = 3
	if callCountA != 3 {
		t.Errorf("Expected branch A call count 3, got %d", callCountA)
	}
	// BranchB: 1 (parallel) + 1 (after) = 2
	if callCountB != 2 {
		t.Errorf("Expected branch B call count 2, got %d", callCountB)
	}
}

func TestParallelActivity_FailureTriggersSagaCompensation(t *testing.T) {
	callCountA := 0
	compensateCountA := 0
	branchAType := newBranchAActivity(&callCountA, &compensateCountA)

	callCountB := 0
	compensateCountB := 0
	branchBType := newBranchBActivity(&callCountB, &compensateCountB)

	callCountFail := 0
	failType := newFailingBranchActivity(&callCountFail)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(branchAType, WorkItemArguments{"value": "first"}),
		NewWorkItem(NewParallelActivity, WorkItemArguments{
			"branches": []*RoutingSlip{
				NewRoutingSlip([]WorkItem{
					NewWorkItem(branchBType, WorkItemArguments{"value": "ok"}),
				}),
				NewRoutingSlip([]WorkItem{
					NewWorkItem(failType, WorkItemArguments{}),
				}),
			},
		}),
	})

	ctx := context.Background()

	// First step succeeds
	result1, err := ProcessNextForTest(ctx, slip, nil)
	if err != nil {
		t.Fatalf("ProcessNext returned error: %v", err)
	}
	if !result1 {
		t.Error("Expected first step to succeed")
	}
	if callCountA != 1 {
		t.Errorf("Expected branch A call count 1, got %d", callCountA)
	}

	// Second step (parallel) fails
	result2, err := ProcessNextForTest(ctx, slip, nil)
	if err != nil {
		t.Fatalf("ProcessNext returned error: %v", err)
	}
	if result2 {
		t.Error("Expected parallel step to fail")
	}

	// Compensate first step
	for slip.IsInProgress() {
		UndoLastForTest(ctx, slip, nil)
	}

	if compensateCountA != 1 {
		t.Errorf("Expected branch A compensate count 1, got %d", compensateCountA)
	}
}
