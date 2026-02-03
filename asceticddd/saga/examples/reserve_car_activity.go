package examples

import (
	"context"
	"math/rand"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/saga"
)

var carRnd = rand.New(rand.NewSource(2))

// ReserveCarActivity is an activity for reserving a rental car.
// This is typically the least risky step in a travel booking saga.
type ReserveCarActivity struct{}

// NewReserveCarActivity creates a new car reservation activity.
func NewReserveCarActivity() saga.Activity {
	return &ReserveCarActivity{}
}

// DoWork reserves a car.
func (a *ReserveCarActivity) DoWork(ctx context.Context, workItem saga.WorkItem) (*saga.WorkLog, error) {
	_ = workItem.Arguments()["vehicleType"]
	reservationId := carRnd.Intn(100000)
	workLog := saga.NewWorkLog(a, saga.WorkResult{"reservationId": reservationId})
	return &workLog, nil
}

// Compensate cancels the car reservation.
func (a *ReserveCarActivity) Compensate(ctx context.Context, workLog saga.WorkLog, routingSlip *saga.RoutingSlip) (bool, error) {
	_ = workLog.Result()["reservationId"]
	return true, nil
}

// WorkItemQueueAddress returns the queue address for car reservation requests.
func (a *ReserveCarActivity) WorkItemQueueAddress() string {
	return "sb://./carReservations"
}

// CompensationQueueAddress returns the queue address for car cancellation requests.
func (a *ReserveCarActivity) CompensationQueueAddress() string {
	return "sb://./carCancellations"
}

// ActivityType returns the activity type function.
func (a *ReserveCarActivity) ActivityType() saga.ActivityType {
	return NewReserveCarActivity
}

// TypeName returns the type name for serialization.
func (a *ReserveCarActivity) TypeName() string {
	return "ReserveCarActivity"
}
