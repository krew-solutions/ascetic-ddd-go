package saga

import "context"

// FallbackActivity tries alternative RoutingSlips until one succeeds.
// Based on Section 6 "Recovery Blocks" of Garcia-Molina & Salem's "Sagas" (1987).
// Each alternative is a full RoutingSlip with its own forward/backward paths.
//
// Behavior:
// - Tries each alternative RoutingSlip in order
// - Stops on first success
// - If alternative fails, it compensates itself before trying next
// - Only the successful alternative needs compensation
type FallbackActivity struct{}

// NewFallbackActivity creates a new fallback activity instance.
func NewFallbackActivity() Activity {
	return &FallbackActivity{}
}

// DoWork tries alternative RoutingSlips until one succeeds.
// Arguments must contain "alternatives" - slice of *RoutingSlip.
// Returns a WorkLog with successful alternative, or nil if all failed.
func (fa *FallbackActivity) DoWork(ctx context.Context, workItem WorkItem) (*WorkLog, error) {
	alternatives := workItem.Arguments["alternatives"].([]*RoutingSlip)

	for _, alternative := range alternatives {
		success, err := fa.executeAlternative(ctx, alternative)
		if err != nil {
			return nil, err
		}

		if success {
			// Store which alternative succeeded for future compensation
			workLog := NewWorkLog(fa, WorkResult{"_succeeded": alternative})
			return &workLog, nil
		}
	}

	// All alternatives failed
	return nil, nil
}

// executeAlternative executes an alternative RoutingSlip to completion.
func (fa *FallbackActivity) executeAlternative(ctx context.Context, alternative *RoutingSlip) (bool, error) {
	for !alternative.IsCompleted() {
		success, err := alternative.ProcessNext(ctx)
		if err != nil {
			return false, err
		}
		if !success {
			// Alternative failed - compensate and return false
			for alternative.IsInProgress() {
				_, err := alternative.UndoLast(ctx)
				if err != nil {
					return false, err
				}
			}
			return false, nil
		}
	}
	return true, nil
}

// Compensate compensates the successful alternative.
// Returns true to continue backward path.
func (fa *FallbackActivity) Compensate(ctx context.Context, workLog WorkLog, routingSlip *RoutingSlip) (bool, error) {
	succeeded := workLog.Result["_succeeded"].(*RoutingSlip)

	for succeeded.IsInProgress() {
		_, err := succeeded.UndoLast(ctx)
		if err != nil {
			return false, err
		}
	}

	return true, nil
}

// WorkItemQueueAddress returns the work queue address.
func (fa *FallbackActivity) WorkItemQueueAddress() string {
	return "sb://./fallback"
}

// CompensationQueueAddress returns the compensation queue address.
func (fa *FallbackActivity) CompensationQueueAddress() string {
	return "sb://./fallbackCompensation"
}

// ActivityType returns the activity type function.
func (fa *FallbackActivity) ActivityType() ActivityType {
	return NewFallbackActivity
}
