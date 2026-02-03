package saga

import (
	"context"
	"errors"
)

var (
	// ErrInvalidOperation is raised when an operation is invalid for the current state.
	ErrInvalidOperation = errors.New("invalid operation")
)

// RoutingSlip is the document that flows through the saga.
// Contains:
// - Queue of pending work items (forward path)
// - Stack of completed work logs (backward path)
type RoutingSlip struct {
	completedWorkLogs []WorkLog
	nextWorkItems     []WorkItem
}

// NewRoutingSlip creates a new routing slip with optional work items.
func NewRoutingSlip(workItems []WorkItem) *RoutingSlip {
	rs := &RoutingSlip{
		completedWorkLogs: make([]WorkLog, 0),
		nextWorkItems:     make([]WorkItem, 0),
	}

	if workItems != nil {
		rs.nextWorkItems = append(rs.nextWorkItems, workItems...)
	}

	return rs
}

// IsCompleted returns true if all work items have been processed.
func (rs *RoutingSlip) IsCompleted() bool {
	return len(rs.nextWorkItems) == 0
}

// IsInProgress returns true if some work has been completed (can be compensated).
func (rs *RoutingSlip) IsInProgress() bool {
	return len(rs.completedWorkLogs) > 0
}

// ProcessNext processes the next work item in the queue.
// Returns true if the work was successful, false otherwise.
func (rs *RoutingSlip) ProcessNext(ctx context.Context) (bool, error) {
	if rs.IsCompleted() {
		return false, ErrInvalidOperation
	}

	currentItem := rs.nextWorkItems[0]
	rs.nextWorkItems = rs.nextWorkItems[1:]

	activity := currentItem.ActivityType()()

	result, err := activity.DoWork(ctx, currentItem)
	if err != nil {
		return false, nil
	}

	if result != nil {
		rs.completedWorkLogs = append(rs.completedWorkLogs, *result)
		return true, nil
	}

	return false, nil
}

// ProgressUri returns the address of the next activity's work queue, or empty string if completed.
func (rs *RoutingSlip) ProgressUri() string {
	if rs.IsCompleted() {
		return ""
	}

	activity := rs.nextWorkItems[0].ActivityType()()
	return activity.WorkItemQueueAddress()
}

// CompensationUri returns the address of the last completed activity's compensation queue.
func (rs *RoutingSlip) CompensationUri() string {
	if !rs.IsInProgress() {
		return ""
	}

	activityType := rs.completedWorkLogs[len(rs.completedWorkLogs)-1].ActivityType()
	activity := activityType()
	return activity.CompensationQueueAddress()
}

// UndoLast undoes the last completed work item.
// Returns true if compensation succeeded and should continue backward,
// false if compensation added new work and should resume forward.
func (rs *RoutingSlip) UndoLast(ctx context.Context) (bool, error) {
	if !rs.IsInProgress() {
		return false, ErrInvalidOperation
	}

	currentItem := rs.completedWorkLogs[len(rs.completedWorkLogs)-1]
	rs.completedWorkLogs = rs.completedWorkLogs[:len(rs.completedWorkLogs)-1]

	activity := currentItem.ActivityType()()

	return activity.Compensate(ctx, currentItem, rs)
}

// CompletedWorkLogs returns the list of completed work logs (for inspection/testing).
func (rs *RoutingSlip) CompletedWorkLogs() []WorkLog {
	return rs.completedWorkLogs
}

// PendingWorkItems returns the list of pending work items (for inspection/testing).
func (rs *RoutingSlip) PendingWorkItems() []WorkItem {
	return rs.nextWorkItems
}
