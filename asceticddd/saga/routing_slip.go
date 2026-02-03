package saga

import (
	"context"
	"encoding/json"
	"errors"
)

var (
	// ErrInvalidOperation is raised when an operation is invalid for the current state.
	ErrInvalidOperation = errors.New("invalid operation")
)

// RoutingSlip is the document that flows through the saga.
// It is a pure data structure that contains:
// - Queue of pending work items (forward path)
// - Stack of completed work logs (backward path)
// This structure is fully serializable for distributed message passing.
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

// NextWorkItem returns and removes the next work item from the queue.
// This is a pure data operation - it doesn't execute any business logic.
func (rs *RoutingSlip) NextWorkItem() (WorkItem, error) {
	if rs.IsCompleted() {
		return WorkItem{}, ErrInvalidOperation
	}

	item := rs.nextWorkItems[0]
	rs.nextWorkItems = rs.nextWorkItems[1:]
	return item, nil
}

// AddCompletedWork adds a work log to the completed stack.
func (rs *RoutingSlip) AddCompletedWork(workLog WorkLog) {
	rs.completedWorkLogs = append(rs.completedWorkLogs, workLog)
}

// LastCompletedWork returns and removes the last completed work log from the stack.
// This is a pure data operation - it doesn't execute any business logic.
func (rs *RoutingSlip) LastCompletedWork() (WorkLog, error) {
	if !rs.IsInProgress() {
		return WorkLog{}, ErrInvalidOperation
	}

	log := rs.completedWorkLogs[len(rs.completedWorkLogs)-1]
	rs.completedWorkLogs = rs.completedWorkLogs[:len(rs.completedWorkLogs)-1]
	return log, nil
}

// ProgressUri returns the address of the next activity's work queue, or empty string if completed.
func (rs *RoutingSlip) ProgressUri() string {
	if rs.IsCompleted() {
		return ""
	}
	return rs.nextWorkItems[0].QueueAddress
}

// CompensationUri returns the address of the last completed activity's compensation queue.
func (rs *RoutingSlip) CompensationUri() string {
	if !rs.IsInProgress() {
		return ""
	}
	return rs.completedWorkLogs[len(rs.completedWorkLogs)-1].CompensationQueueAddress
}

// CompletedWorkLogsRef returns the list of completed work logs (for inspection/testing).
// Deprecated: Use CompletedWorkLogs field directly.
func (rs *RoutingSlip) CompletedWorkLogsRef() []WorkLog {
	return rs.completedWorkLogs
}

// PendingWorkItems returns the list of pending work items (for inspection/testing).
func (rs *RoutingSlip) PendingWorkItems() []WorkItem {
	return rs.nextWorkItems
}

// ProcessNext processes the next work item using the deprecated activityType field.
// This is for backward compatibility and local execution only.
// In distributed scenarios, use ActivityHost which resolves activities by queue address.
// Returns true if work succeeded, false otherwise.
func (rs *RoutingSlip) ProcessNext(ctx context.Context) (bool, error) {
	if rs.IsCompleted() {
		return false, ErrInvalidOperation
	}

	// Peek at next item to get activityType before removing it
	activityType := rs.nextWorkItems[0].activityType
	if activityType == nil {
		return false, nil
	}

	workItem, err := rs.NextWorkItem()
	if err != nil {
		return false, err
	}

	activity := activityType()
	workLog, err := activity.DoWork(ctx, workItem)
	if err != nil || workLog == nil {
		return false, nil
	}

	rs.AddCompletedWork(*workLog)
	return true, nil
}

// ProcessNextWith processes the next work item using the provided activity.
// This is a helper method for local execution (e.g., in composite activities like FallbackActivity).
// Returns true if work succeeded, false otherwise.
func (rs *RoutingSlip) ProcessNextWith(ctx context.Context, activityFactory func(queueAddress string) Activity) (bool, error) {
	if rs.IsCompleted() {
		return false, ErrInvalidOperation
	}

	workItem, err := rs.NextWorkItem()
	if err != nil {
		return false, err
	}

	activity := activityFactory(workItem.QueueAddress)
	if activity == nil {
		return false, nil
	}

	workLog, err := activity.DoWork(ctx, workItem)
	if err != nil || workLog == nil {
		return false, nil
	}

	rs.AddCompletedWork(*workLog)
	return true, nil
}

// UndoLast undoes the last completed work using the deprecated activityType field.
// This is for backward compatibility and local execution only.
// In distributed scenarios, use ActivityHost which resolves activities by queue address.
// Returns true if should continue backward, false if should resume forward.
func (rs *RoutingSlip) UndoLast(ctx context.Context) (bool, error) {
	if !rs.IsInProgress() {
		return false, ErrInvalidOperation
	}

	// Peek at last log to get activityType before removing it
	activityType := rs.completedWorkLogs[len(rs.completedWorkLogs)-1].activityType
	if activityType == nil {
		return false, nil
	}

	workLog, err := rs.LastCompletedWork()
	if err != nil {
		return false, err
	}

	activity := activityType()
	return activity.Compensate(ctx, workLog, rs)
}

// UndoLastWith undoes the last completed work using the provided activity.
// This is a helper method for local execution (e.g., in composite activities like FallbackActivity).
// Returns true if should continue backward, false if should resume forward.
func (rs *RoutingSlip) UndoLastWith(ctx context.Context, activityFactory func(compensationQueueAddress string) Activity) (bool, error) {
	if !rs.IsInProgress() {
		return false, ErrInvalidOperation
	}

	workLog, err := rs.LastCompletedWork()
	if err != nil {
		return false, err
	}

	activity := activityFactory(workLog.CompensationQueueAddress)
	if activity == nil {
		return false, nil
	}

	return activity.Compensate(ctx, workLog, rs)
}

// MarshalJSON implements json.Marshaler interface.
// The WorkItem and WorkLog types have custom marshalers that handle serialization properly.
func (rs *RoutingSlip) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		CompletedWorkLogs []WorkLog  `json:"completedWorkLogs"`
		NextWorkItems     []WorkItem `json:"nextWorkItems"`
	}{
		CompletedWorkLogs: rs.completedWorkLogs,
		NextWorkItems:     rs.nextWorkItems,
	})
}

// UnmarshalJSON implements json.Unmarshaler interface.
// The WorkItem and WorkLog types have custom unmarshalers that handle deserialization properly.
func (rs *RoutingSlip) UnmarshalJSON(data []byte) error {
	var raw struct {
		CompletedWorkLogs []WorkLog  `json:"completedWorkLogs"`
		NextWorkItems     []WorkItem `json:"nextWorkItems"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	rs.completedWorkLogs = raw.CompletedWorkLogs
	rs.nextWorkItems = raw.NextWorkItems

	return nil
}

// CompletedWorkLogs returns the list of completed work logs (for inspection/testing).
func (rs *RoutingSlip) CompletedWorkLogs() []WorkLog {
	return rs.completedWorkLogs
}
