package saga

import (
	"context"
	"fmt"
)

// SendCallback is a function that sends a routing slip to a target URI.
type SendCallback func(ctx context.Context, uri string, routingSlip *RoutingSlip) error

// ActivityHost manages local execution for multiple activity types.
// It orchestrates the execution flow by:
// - Routing messages to the appropriate activity based on queue address
// - Creating activity instances from registered factories
// - Invoking DoWork() or Compensate() methods
// - Sending messages to the next destination
type ActivityHost struct {
	activities map[string]ActivityType // queueAddress -> factory
	send       SendCallback
}

// NewActivityHost creates a new activity host.
func NewActivityHost(send SendCallback) *ActivityHost {
	return &ActivityHost{
		activities: make(map[string]ActivityType),
		send:       send,
	}
}

// Register registers an activity type with this host.
// The host will handle messages for both the work queue and compensation queue.
func (ah *ActivityHost) Register(factory ActivityType) {
	activity := factory()
	ah.activities[activity.WorkItemQueueAddress()] = factory
	ah.activities[activity.CompensationQueueAddress()] = factory
}

// HandleMessage handles an incoming message from the message bus.
// Returns error if no handler is registered for the queue address.
func (ah *ActivityHost) HandleMessage(ctx context.Context, queueAddress string, routingSlip *RoutingSlip) error {
	factory, ok := ah.activities[queueAddress]
	if !ok {
		return fmt.Errorf("no handler registered for queue: %s", queueAddress)
	}

	activity := factory()

	if activity.CompensationQueueAddress() == queueAddress {
		return ah.processBackward(ctx, activity, routingSlip)
	}

	return ah.processForward(ctx, activity, routingSlip)
}

// processForward processes a forward (DoWork) message.
func (ah *ActivityHost) processForward(ctx context.Context, activity Activity, routingSlip *RoutingSlip) error {
	if routingSlip.IsCompleted() {
		return nil
	}

	workItem, err := routingSlip.NextWorkItem()
	if err != nil {
		return err
	}

	workLog, err := activity.DoWork(ctx, workItem)
	if err != nil || workLog == nil {
		if routingSlip.CompensationUri() != "" {
			return ah.send(ctx, routingSlip.CompensationUri(), routingSlip)
		}
		return err
	}

	routingSlip.AddCompletedWork(*workLog)

	if routingSlip.ProgressUri() != "" {
		return ah.send(ctx, routingSlip.ProgressUri(), routingSlip)
	}

	return nil
}

// processBackward processes a backward (compensate) message.
func (ah *ActivityHost) processBackward(ctx context.Context, activity Activity, routingSlip *RoutingSlip) error {
	if !routingSlip.IsInProgress() {
		return nil
	}

	workLog, err := routingSlip.LastCompletedWork()
	if err != nil {
		return err
	}

	continueBackward, err := activity.Compensate(ctx, workLog, routingSlip)
	if err != nil {
		return err
	}

	if continueBackward {
		if routingSlip.CompensationUri() != "" {
			return ah.send(ctx, routingSlip.CompensationUri(), routingSlip)
		}
	} else {
		if routingSlip.ProgressUri() != "" {
			return ah.send(ctx, routingSlip.ProgressUri(), routingSlip)
		}
	}

	return nil
}

// Queues returns the list of queue addresses this host can handle.
func (ah *ActivityHost) Queues() []string {
	queues := make([]string, 0, len(ah.activities))
	for addr := range ah.activities {
		queues = append(queues, addr)
	}
	return queues
}
