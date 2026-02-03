package saga

import "context"

// ActivityType is a function that creates an activity instance.
type ActivityType func() Activity

// Activity is the interface for saga activities.
// Each activity encapsulates two operations:
// - DoWork: Performs the actual business operation
// - Compensate: Reverses the operation if the saga fails
type Activity interface {
	// DoWork executes the activity's business logic.
	// Returns a WorkLog containing the result of the work, or nil if failed.
	DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error)

	// Compensate compensates (undoes) the previously completed work.
	// Called during the backward path when the saga needs to be rolled back.
	// Returns true if compensation was successful and should continue backward,
	// false if compensation added new work and should resume forward path.
	Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error)

	// WorkItemQueueAddress returns the address of the queue for processing work items (forward path).
	WorkItemQueueAddress() string

	// CompensationQueueAddress returns the address of the queue for processing compensation (backward path).
	CompensationQueueAddress() string

	// ActivityType returns the activity type function for this activity.
	ActivityType() ActivityType
}
