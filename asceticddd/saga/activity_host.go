package saga

import "context"

// SendCallback is a function that sends a routing slip to a target URI.
type SendCallback func(ctx context.Context, uri string, routingSlip *RoutingSlip) error

// ActivityHost manages local execution for a specific activity type.
// Processes forward messages to execute DoWork() and backward messages
// to invoke Compensate().
type ActivityHost struct {
	activityType ActivityType
	send         SendCallback
}

// NewActivityHost creates a new activity host for the specified activity type.
func NewActivityHost(activityType ActivityType, send SendCallback) *ActivityHost {
	return &ActivityHost{
		activityType: activityType,
		send:         send,
	}
}

// ProcessForwardMessage processes a forward (DoWork) message.
// If work succeeds, sends to next activity's work queue.
// If work fails, sends to compensation queue for rollback.
func (ah *ActivityHost) ProcessForwardMessage(ctx context.Context, routingSlip *RoutingSlip) error {
	if !routingSlip.IsCompleted() {
		success, err := routingSlip.ProcessNext(ctx)
		if err != nil {
			return err
		}

		if success {
			// Success - continue forward
			if routingSlip.ProgressUri() != "" {
				return ah.send(ctx, routingSlip.ProgressUri(), routingSlip)
			}
		} else {
			// Failure - start compensation
			if routingSlip.CompensationUri() != "" {
				return ah.send(ctx, routingSlip.CompensationUri(), routingSlip)
			}
		}
	}
	return nil
}

// ProcessBackwardMessage processes a backward (compensate) message.
// If compensation succeeds, continues backward to previous activity.
// If compensation returns false (added new work), resumes forward.
func (ah *ActivityHost) ProcessBackwardMessage(ctx context.Context, routingSlip *RoutingSlip) error {
	if routingSlip.IsInProgress() {
		continueBackward, err := routingSlip.UndoLast(ctx)
		if err != nil {
			return err
		}

		if continueBackward {
			// Continue backward
			if routingSlip.CompensationUri() != "" {
				return ah.send(ctx, routingSlip.CompensationUri(), routingSlip)
			}
		} else {
			// Resume forward (compensation added new work)
			if routingSlip.ProgressUri() != "" {
				return ah.send(ctx, routingSlip.ProgressUri(), routingSlip)
			}
		}
	}
	return nil
}

// AcceptMessage accepts and processes a message if it matches this host's queues.
// Returns true if message was accepted and processed, false otherwise.
func (ah *ActivityHost) AcceptMessage(ctx context.Context, uri string, routingSlip *RoutingSlip) (bool, error) {
	activity := ah.activityType()

	if activity.CompensationQueueAddress() == uri {
		err := ah.ProcessBackwardMessage(ctx, routingSlip)
		return true, err
	}

	if activity.WorkItemQueueAddress() == uri {
		err := ah.ProcessForwardMessage(ctx, routingSlip)
		return true, err
	}

	return false, nil
}
