package saga

import (
	"context"
	"testing"
)

type primaryActivity struct {
	callCount       *int
	compensateCount *int
	shouldFail      *bool
}

func newPrimaryActivity(callCount, compensateCount *int, shouldFail *bool) ActivityType {
	return func() Activity {
		return &primaryActivity{
			callCount:       callCount,
			compensateCount: compensateCount,
			shouldFail:      shouldFail,
		}
	}
}

func (a *primaryActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	*a.callCount++
	if *a.shouldFail {
		return nil, nil
	}
	value := "default"
	if v, ok := workItem.Arguments()["value"]; ok {
		value = v.(string)
	}
	workLog := NewWorkLog(a, WorkResult{"provider": "primary", "value": value})
	return &workLog, nil
}

func (a *primaryActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	*a.compensateCount++
	return true, nil
}

func (a *primaryActivity) WorkItemQueueAddress() string {
	return "sb://./primary"
}

func (a *primaryActivity) CompensationQueueAddress() string {
	return "sb://./primaryCompensation"
}

func (a *primaryActivity) ActivityType() ActivityType {
	return newPrimaryActivity(a.callCount, a.compensateCount, a.shouldFail)
}

type backupActivity struct {
	callCount       *int
	compensateCount *int
	shouldFail      *bool
}

func newBackupActivity(callCount, compensateCount *int, shouldFail *bool) ActivityType {
	return func() Activity {
		return &backupActivity{
			callCount:       callCount,
			compensateCount: compensateCount,
			shouldFail:      shouldFail,
		}
	}
}

func (a *backupActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	*a.callCount++
	if *a.shouldFail {
		return nil, nil
	}
	value := "default"
	if v, ok := workItem.Arguments()["value"]; ok {
		value = v.(string)
	}
	workLog := NewWorkLog(a, WorkResult{"provider": "backup", "value": value})
	return &workLog, nil
}

func (a *backupActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	*a.compensateCount++
	return true, nil
}

func (a *backupActivity) WorkItemQueueAddress() string {
	return "sb://./backup"
}

func (a *backupActivity) CompensationQueueAddress() string {
	return "sb://./backupCompensation"
}

func (a *backupActivity) ActivityType() ActivityType {
	return newBackupActivity(a.callCount, a.compensateCount, a.shouldFail)
}

type thirdActivity struct {
	callCount       *int
	compensateCount *int
}

func newThirdActivity(callCount, compensateCount *int) ActivityType {
	return func() Activity {
		return &thirdActivity{
			callCount:       callCount,
			compensateCount: compensateCount,
		}
	}
}

func (a *thirdActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	*a.callCount++
	workLog := NewWorkLog(a, WorkResult{"provider": "third"})
	return &workLog, nil
}

func (a *thirdActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	*a.compensateCount++
	return true, nil
}

func (a *thirdActivity) WorkItemQueueAddress() string {
	return "sb://./third"
}

func (a *thirdActivity) CompensationQueueAddress() string {
	return "sb://./thirdCompensation"
}

func (a *thirdActivity) ActivityType() ActivityType {
	return newThirdActivity(a.callCount, a.compensateCount)
}

type confirmActivity struct {
	callCount       *int
	compensateCount *int
}

func newConfirmActivity(callCount, compensateCount *int) ActivityType {
	return func() Activity {
		return &confirmActivity{
			callCount:       callCount,
			compensateCount: compensateCount,
		}
	}
}

func (a *confirmActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	*a.callCount++
	workLog := NewWorkLog(a, WorkResult{"confirmed": true})
	return &workLog, nil
}

func (a *confirmActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	*a.compensateCount++
	return true, nil
}

func (a *confirmActivity) WorkItemQueueAddress() string {
	return "sb://./confirm"
}

func (a *confirmActivity) CompensationQueueAddress() string {
	return "sb://./confirmCompensation"
}

func (a *confirmActivity) ActivityType() ActivityType {
	return newConfirmActivity(a.callCount, a.compensateCount)
}

func TestFallbackActivity_PrimarySucceeds(t *testing.T) {
	callCountPrimary := 0
	compensateCountPrimary := 0
	shouldFailPrimary := false
	primaryType := newPrimaryActivity(&callCountPrimary, &compensateCountPrimary, &shouldFailPrimary)

	callCountBackup := 0
	compensateCountBackup := 0
	shouldFailBackup := false
	backupType := newBackupActivity(&callCountBackup, &compensateCountBackup, &shouldFailBackup)

	activity := NewFallbackActivity()
	workItem := NewWorkItem(NewFallbackActivity, WorkItemArguments{
		"alternatives": []*RoutingSlip{
			NewRoutingSlip([]WorkItem{NewWorkItem(primaryType, WorkItemArguments{"value": "test"})}),
			NewRoutingSlip([]WorkItem{NewWorkItem(backupType, WorkItemArguments{"value": "test"})}),
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
	if callCountPrimary != 1 {
		t.Errorf("Expected primary call count 1, got %d", callCountPrimary)
	}
	if callCountBackup != 0 {
		t.Errorf("Expected backup call count 0, got %d", callCountBackup)
	}
}

func TestFallbackActivity_PrimaryFailsBackupSucceeds(t *testing.T) {
	callCountPrimary := 0
	compensateCountPrimary := 0
	shouldFailPrimary := true
	primaryType := newPrimaryActivity(&callCountPrimary, &compensateCountPrimary, &shouldFailPrimary)

	callCountBackup := 0
	compensateCountBackup := 0
	shouldFailBackup := false
	backupType := newBackupActivity(&callCountBackup, &compensateCountBackup, &shouldFailBackup)

	activity := NewFallbackActivity()
	workItem := NewWorkItem(NewFallbackActivity, WorkItemArguments{
		"alternatives": []*RoutingSlip{
			NewRoutingSlip([]WorkItem{NewWorkItem(primaryType, WorkItemArguments{"value": "test"})}),
			NewRoutingSlip([]WorkItem{NewWorkItem(backupType, WorkItemArguments{"value": "test"})}),
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
	if callCountPrimary != 1 {
		t.Errorf("Expected primary call count 1, got %d", callCountPrimary)
	}
	if callCountBackup != 1 {
		t.Errorf("Expected backup call count 1, got %d", callCountBackup)
	}
}

func TestFallbackActivity_MultiStepAlternative(t *testing.T) {
	callCountPrimary := 0
	compensateCountPrimary := 0
	shouldFailPrimary := false
	primaryType := newPrimaryActivity(&callCountPrimary, &compensateCountPrimary, &shouldFailPrimary)

	callCountConfirm := 0
	compensateCountConfirm := 0
	confirmType := newConfirmActivity(&callCountConfirm, &compensateCountConfirm)

	activity := NewFallbackActivity()
	workItem := NewWorkItem(NewFallbackActivity, WorkItemArguments{
		"alternatives": []*RoutingSlip{
			NewRoutingSlip([]WorkItem{
				NewWorkItem(primaryType, WorkItemArguments{"value": "step1"}),
				NewWorkItem(confirmType, WorkItemArguments{}),
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
	if callCountPrimary != 1 {
		t.Errorf("Expected primary call count 1, got %d", callCountPrimary)
	}
	if callCountConfirm != 1 {
		t.Errorf("Expected confirm call count 1, got %d", callCountConfirm)
	}
}

func TestFallbackActivity_AllAlternativesFail(t *testing.T) {
	callCountPrimary := 0
	compensateCountPrimary := 0
	shouldFailPrimary := true
	primaryType := newPrimaryActivity(&callCountPrimary, &compensateCountPrimary, &shouldFailPrimary)

	callCountBackup := 0
	compensateCountBackup := 0
	shouldFailBackup := true
	backupType := newBackupActivity(&callCountBackup, &compensateCountBackup, &shouldFailBackup)

	activity := NewFallbackActivity()
	workItem := NewWorkItem(NewFallbackActivity, WorkItemArguments{
		"alternatives": []*RoutingSlip{
			NewRoutingSlip([]WorkItem{NewWorkItem(primaryType, WorkItemArguments{"value": "test"})}),
			NewRoutingSlip([]WorkItem{NewWorkItem(backupType, WorkItemArguments{"value": "test"})}),
		},
	})

	ctx := context.Background()
	result, err := activity.DoWork(ctx, workItem)
	if err != nil {
		t.Fatalf("DoWork returned error: %v", err)
	}

	if result != nil {
		t.Error("Expected nil result when all alternatives fail")
	}
	if callCountPrimary != 1 {
		t.Errorf("Expected primary call count 1, got %d", callCountPrimary)
	}
	if callCountBackup != 1 {
		t.Errorf("Expected backup call count 1, got %d", callCountBackup)
	}
}

func TestFallbackActivity_ThirdAlternativeSucceeds(t *testing.T) {
	callCountPrimary := 0
	compensateCountPrimary := 0
	shouldFailPrimary := true
	primaryType := newPrimaryActivity(&callCountPrimary, &compensateCountPrimary, &shouldFailPrimary)

	callCountBackup := 0
	compensateCountBackup := 0
	shouldFailBackup := true
	backupType := newBackupActivity(&callCountBackup, &compensateCountBackup, &shouldFailBackup)

	callCountThird := 0
	compensateCountThird := 0
	thirdType := newThirdActivity(&callCountThird, &compensateCountThird)

	activity := NewFallbackActivity()
	workItem := NewWorkItem(NewFallbackActivity, WorkItemArguments{
		"alternatives": []*RoutingSlip{
			NewRoutingSlip([]WorkItem{NewWorkItem(primaryType, WorkItemArguments{})}),
			NewRoutingSlip([]WorkItem{NewWorkItem(backupType, WorkItemArguments{})}),
			NewRoutingSlip([]WorkItem{NewWorkItem(thirdType, WorkItemArguments{})}),
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
	if callCountPrimary != 1 {
		t.Errorf("Expected primary call count 1, got %d", callCountPrimary)
	}
	if callCountBackup != 1 {
		t.Errorf("Expected backup call count 1, got %d", callCountBackup)
	}
	if callCountThird != 1 {
		t.Errorf("Expected third call count 1, got %d", callCountThird)
	}
}

func TestFallbackActivity_CompensatePrimary(t *testing.T) {
	callCountPrimary := 0
	compensateCountPrimary := 0
	shouldFailPrimary := false
	primaryType := newPrimaryActivity(&callCountPrimary, &compensateCountPrimary, &shouldFailPrimary)

	callCountBackup := 0
	compensateCountBackup := 0
	shouldFailBackup := false
	backupType := newBackupActivity(&callCountBackup, &compensateCountBackup, &shouldFailBackup)

	activity := NewFallbackActivity()
	workItem := NewWorkItem(NewFallbackActivity, WorkItemArguments{
		"alternatives": []*RoutingSlip{
			NewRoutingSlip([]WorkItem{NewWorkItem(primaryType, WorkItemArguments{"value": "test"})}),
			NewRoutingSlip([]WorkItem{NewWorkItem(backupType, WorkItemArguments{"value": "test"})}),
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

	compensateResult, err := activity.Compensate(ctx, *result, NewRoutingSlip(nil))
	if err != nil {
		t.Fatalf("Compensate returned error: %v", err)
	}

	if !compensateResult {
		t.Error("Expected compensate to return true")
	}
	if compensateCountPrimary != 1 {
		t.Errorf("Expected primary compensate count 1, got %d", compensateCountPrimary)
	}
	if compensateCountBackup != 0 {
		t.Errorf("Expected backup compensate count 0, got %d", compensateCountBackup)
	}
}

func TestFallbackActivity_CompensateBackup(t *testing.T) {
	callCountPrimary := 0
	compensateCountPrimary := 0
	shouldFailPrimary := true
	primaryType := newPrimaryActivity(&callCountPrimary, &compensateCountPrimary, &shouldFailPrimary)

	callCountBackup := 0
	compensateCountBackup := 0
	shouldFailBackup := false
	backupType := newBackupActivity(&callCountBackup, &compensateCountBackup, &shouldFailBackup)

	activity := NewFallbackActivity()
	workItem := NewWorkItem(NewFallbackActivity, WorkItemArguments{
		"alternatives": []*RoutingSlip{
			NewRoutingSlip([]WorkItem{NewWorkItem(primaryType, WorkItemArguments{"value": "test"})}),
			NewRoutingSlip([]WorkItem{NewWorkItem(backupType, WorkItemArguments{"value": "test"})}),
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

	compensateResult, err := activity.Compensate(ctx, *result, NewRoutingSlip(nil))
	if err != nil {
		t.Fatalf("Compensate returned error: %v", err)
	}

	if !compensateResult {
		t.Error("Expected compensate to return true")
	}
	if compensateCountPrimary != 0 {
		t.Errorf("Expected primary compensate count 0, got %d", compensateCountPrimary)
	}
	if compensateCountBackup != 1 {
		t.Errorf("Expected backup compensate count 1, got %d", compensateCountBackup)
	}
}

func TestFallbackActivity_CompensateMultiStepAlternative(t *testing.T) {
	callCountPrimary := 0
	compensateCountPrimary := 0
	shouldFailPrimary := false
	primaryType := newPrimaryActivity(&callCountPrimary, &compensateCountPrimary, &shouldFailPrimary)

	callCountConfirm := 0
	compensateCountConfirm := 0
	confirmType := newConfirmActivity(&callCountConfirm, &compensateCountConfirm)

	activity := NewFallbackActivity()
	workItem := NewWorkItem(NewFallbackActivity, WorkItemArguments{
		"alternatives": []*RoutingSlip{
			NewRoutingSlip([]WorkItem{
				NewWorkItem(primaryType, WorkItemArguments{}),
				NewWorkItem(confirmType, WorkItemArguments{}),
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
	if callCountPrimary != 1 {
		t.Errorf("Expected primary call count 1, got %d", callCountPrimary)
	}
	if callCountConfirm != 1 {
		t.Errorf("Expected confirm call count 1, got %d", callCountConfirm)
	}

	compensateResult, err := activity.Compensate(ctx, *result, NewRoutingSlip(nil))
	if err != nil {
		t.Fatalf("Compensate returned error: %v", err)
	}

	if !compensateResult {
		t.Error("Expected compensate to return true")
	}
	if compensateCountPrimary != 1 {
		t.Errorf("Expected primary compensate count 1, got %d", compensateCountPrimary)
	}
	if compensateCountConfirm != 1 {
		t.Errorf("Expected confirm compensate count 1, got %d", compensateCountConfirm)
	}
}

func TestFallbackActivity_QueueAddresses(t *testing.T) {
	activity := NewFallbackActivity()
	if activity.WorkItemQueueAddress() != "sb://./fallback" {
		t.Errorf("Expected work queue 'sb://./fallback', got '%s'", activity.WorkItemQueueAddress())
	}
	if activity.CompensationQueueAddress() != "sb://./fallbackCompensation" {
		t.Errorf("Expected compensation queue 'sb://./fallbackCompensation', got '%s'", activity.CompensationQueueAddress())
	}
}

func TestFallbackActivity_InRoutingSlip(t *testing.T) {
	callCountPrimary := 0
	compensateCountPrimary := 0
	shouldFailPrimary := true
	primaryType := newPrimaryActivity(&callCountPrimary, &compensateCountPrimary, &shouldFailPrimary)

	callCountBackup := 0
	compensateCountBackup := 0
	shouldFailBackup := false
	backupType := newBackupActivity(&callCountBackup, &compensateCountBackup, &shouldFailBackup)

	callCountThird := 0
	compensateCountThird := 0
	thirdType := newThirdActivity(&callCountThird, &compensateCountThird)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(thirdType, WorkItemArguments{}),
		NewWorkItem(NewFallbackActivity, WorkItemArguments{
			"alternatives": []*RoutingSlip{
				NewRoutingSlip([]WorkItem{NewWorkItem(primaryType, WorkItemArguments{"value": "try1"})}),
				NewRoutingSlip([]WorkItem{NewWorkItem(backupType, WorkItemArguments{"value": "try2"})}),
			},
		}),
		NewWorkItem(thirdType, WorkItemArguments{}),
	})

	ctx := context.Background()

	// Execute all steps
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
	if callCountThird != 2 {
		t.Errorf("Expected third call count 2, got %d", callCountThird)
	}
	if callCountPrimary != 1 {
		t.Errorf("Expected primary call count 1 (tried and failed), got %d", callCountPrimary)
	}
	if callCountBackup != 1 {
		t.Errorf("Expected backup call count 1 (succeeded), got %d", callCountBackup)
	}
}

func TestFallbackActivity_AllFallbacksFailTriggersCompensation(t *testing.T) {
	callCountPrimary := 0
	compensateCountPrimary := 0
	shouldFailPrimary := true
	primaryType := newPrimaryActivity(&callCountPrimary, &compensateCountPrimary, &shouldFailPrimary)

	callCountBackup := 0
	compensateCountBackup := 0
	shouldFailBackup := true
	backupType := newBackupActivity(&callCountBackup, &compensateCountBackup, &shouldFailBackup)

	callCountThird := 0
	compensateCountThird := 0
	thirdType := newThirdActivity(&callCountThird, &compensateCountThird)

	slip := NewRoutingSlip([]WorkItem{
		NewWorkItem(thirdType, WorkItemArguments{}),
		NewWorkItem(NewFallbackActivity, WorkItemArguments{
			"alternatives": []*RoutingSlip{
				NewRoutingSlip([]WorkItem{NewWorkItem(primaryType, WorkItemArguments{})}),
				NewRoutingSlip([]WorkItem{NewWorkItem(backupType, WorkItemArguments{})}),
			},
		}),
	})

	ctx := context.Background()

	// First step succeeds
	result1, err := slip.ProcessNext(ctx)
	if err != nil {
		t.Fatalf("ProcessNext returned error: %v", err)
	}
	if !result1 {
		t.Error("Expected first step to succeed")
	}

	// Second step (fallback) fails
	result2, err := slip.ProcessNext(ctx)
	if err != nil {
		t.Fatalf("ProcessNext returned error: %v", err)
	}
	if result2 {
		t.Error("Expected fallback step to fail")
	}

	// Compensate first step
	for slip.IsInProgress() {
		slip.UndoLast(ctx)
	}

	if compensateCountThird != 1 {
		t.Errorf("Expected third compensate count 1, got %d", compensateCountThird)
	}
}
