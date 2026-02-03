package examples

import (
	"context"
	"errors"
	"math/rand"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/saga"
)

var flightRnd = rand.New(rand.NewSource(3))

// ReserveFlightActivity is an activity for reserving a flight.
// This is the highest risk step in a travel booking saga.
type ReserveFlightActivity struct{}

// NewReserveFlightActivity creates a new flight reservation activity.
func NewReserveFlightActivity() saga.Activity {
	return &ReserveFlightActivity{}
}

// DoWork reserves a flight.
func (a *ReserveFlightActivity) DoWork(ctx context.Context, workItem saga.WorkItem) (*saga.WorkLog, error) {
	_ = workItem.Arguments()["destination"]
	reservationId := flightRnd.Intn(100000)
	workLog := saga.NewWorkLog(a, saga.WorkResult{"reservationId": reservationId})
	return &workLog, nil
}

// Compensate cancels the flight reservation.
func (a *ReserveFlightActivity) Compensate(ctx context.Context, workLog saga.WorkLog, routingSlip *saga.RoutingSlip) (bool, error) {
	_ = workLog.Result()["reservationId"]
	return true, nil
}

// WorkItemQueueAddress returns the queue address for flight reservation requests.
func (a *ReserveFlightActivity) WorkItemQueueAddress() string {
	return "sb://./flightReservations"
}

// CompensationQueueAddress returns the queue address for flight cancellation requests.
func (a *ReserveFlightActivity) CompensationQueueAddress() string {
	return "sb://./flightCancellations"
}

// ActivityType returns the activity type function.
func (a *ReserveFlightActivity) ActivityType() saga.ActivityType {
	return NewReserveFlightActivity
}

// TypeName returns the type name for serialization.
func (a *ReserveFlightActivity) TypeName() string {
	return "ReserveFlightActivity"
}

// FailingReserveFlightActivity is a flight activity that always fails.
type FailingReserveFlightActivity struct {
	ReserveFlightActivity
}

// NewFailingReserveFlightActivity creates a new failing flight reservation activity.
func NewFailingReserveFlightActivity() saga.Activity {
	return &FailingReserveFlightActivity{}
}

// DoWork attempts to reserve a flight (always fails).
func (a *FailingReserveFlightActivity) DoWork(ctx context.Context, workItem saga.WorkItem) (*saga.WorkLog, error) {
	_ = workItem.Arguments()["fatzbatz"] // This will panic with missing key
	return a.ReserveFlightActivity.DoWork(ctx, workItem)
}

// ActivityType returns the activity type function.
func (a *FailingReserveFlightActivity) ActivityType() saga.ActivityType {
	return NewFailingReserveFlightActivity
}

// TypeName returns the type name for serialization.
func (a *FailingReserveFlightActivity) TypeName() string {
	return "FailingReserveFlightActivity"
}

// SafeFailingReserveFlightActivity is a flight activity that fails without panic.
type SafeFailingReserveFlightActivity struct {
	ReserveFlightActivity
}

// NewSafeFailingReserveFlightActivity creates a new safe failing flight reservation activity.
func NewSafeFailingReserveFlightActivity() saga.Activity {
	return &SafeFailingReserveFlightActivity{}
}

// DoWork attempts to reserve a flight (returns error).
func (a *SafeFailingReserveFlightActivity) DoWork(ctx context.Context, workItem saga.WorkItem) (*saga.WorkLog, error) {
	return nil, errors.New("intentional failure")
}

// ActivityType returns the activity type function.
func (a *SafeFailingReserveFlightActivity) ActivityType() saga.ActivityType {
	return NewSafeFailingReserveFlightActivity
}

// TypeName returns the type name for serialization.
func (a *SafeFailingReserveFlightActivity) TypeName() string {
	return "SafeFailingReserveFlightActivity"
}
