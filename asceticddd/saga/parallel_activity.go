package saga

import (
	"context"
	"sync"
)

// ParallelActivity executes multiple RoutingSlips concurrently (fork/join).
// Based on Section 8 of Garcia-Molina & Salem's "Sagas" (1987).
// Each branch is a full RoutingSlip with its own forward/backward paths.
//
// Behavior:
// - Executes all branch RoutingSlips concurrently
// - Fail-fast: on first failure, compensates completed branches
// - Compensation: all branches compensated concurrently
type ParallelActivity struct{}

// NewParallelActivity creates a new parallel activity instance.
func NewParallelActivity() Activity {
	return &ParallelActivity{}
}

// DoWork executes all branch RoutingSlips in parallel.
// Arguments must contain "branches" - slice of *RoutingSlip.
// Returns a WorkLog with branch references, or nil if any branch failed.
func (pa *ParallelActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	branches := workItem.Arguments()["branches"].([]*RoutingSlip)

	// Execute all branches in parallel
	type result struct {
		index   int
		success bool
		err     error
	}

	results := make(chan result, len(branches))
	var wg sync.WaitGroup

	for i, branch := range branches {
		wg.Add(1)
		go func(idx int, b *RoutingSlip) {
			defer wg.Done()
			success, err := pa.executeBranch(ctx, b)
			results <- result{index: idx, success: success, err: err}
		}(i, branch)
	}

	// Wait for all branches to complete
	wg.Wait()
	close(results)

	// Check for failures
	allSuccess := true
	for r := range results {
		if r.err != nil || !r.success {
			allSuccess = false
			break
		}
	}

	if !allSuccess {
		// Fail-fast: compensate all branches (completed and partial)
		pa.compensateBranches(ctx, branches)
		return nil, nil
	}

	// All succeeded - store branches for future compensation
	workLog := NewWorkLog(pa, WorkResult{"_branches": branches})
	return &workLog, nil
}

// executeBranch executes a single branch RoutingSlip to completion.
func (pa *ParallelActivity) executeBranch(ctx context.Context, branch *RoutingSlip) (bool, error) {
	for !branch.IsCompleted() {
		success, err := branch.ProcessNext(ctx)
		if err != nil {
			return false, err
		}
		if !success {
			// Branch failed - compensate this branch
			for branch.IsInProgress() {
				_, err := branch.UndoLast(ctx)
				if err != nil {
					return false, err
				}
			}
			return false, nil
		}
	}
	return true, nil
}

// compensateBranches compensates all branches concurrently.
func (pa *ParallelActivity) compensateBranches(ctx context.Context, branches []*RoutingSlip) {
	var wg sync.WaitGroup

	for _, branch := range branches {
		wg.Add(1)
		go func(b *RoutingSlip) {
			defer wg.Done()
			pa.compensateBranch(ctx, b)
		}(branch)
	}

	wg.Wait()
}

// compensateBranch compensates a single branch.
func (pa *ParallelActivity) compensateBranch(ctx context.Context, branch *RoutingSlip) {
	for branch.IsInProgress() {
		branch.UndoLast(ctx)
	}
}

// Compensate compensates all branches in parallel.
// Returns true to continue backward path.
func (pa *ParallelActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	branches := workLog.Result()["_branches"].([]*RoutingSlip)
	pa.compensateBranches(ctx, branches)
	return true, nil
}

// WorkItemQueueAddress returns the work queue address.
func (pa *ParallelActivity) WorkItemQueueAddress() string {
	return "sb://./parallel"
}

// CompensationQueueAddress returns the compensation queue address.
func (pa *ParallelActivity) CompensationQueueAddress() string {
	return "sb://./parallelCompensation"
}

// ActivityType returns the activity type function.
func (pa *ParallelActivity) ActivityType() ActivityType {
	return NewParallelActivity
}
