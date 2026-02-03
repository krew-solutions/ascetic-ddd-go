package examples

import (
	"context"
	"math/rand"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/saga"
)

var hotelRnd = rand.New(rand.NewSource(1))

// ReserveHotelActivity is an activity for reserving a hotel room.
// This is a moderate risk step in a travel booking saga.
type ReserveHotelActivity struct{}

// NewReserveHotelActivity creates a new hotel reservation activity.
func NewReserveHotelActivity() saga.Activity {
	return &ReserveHotelActivity{}
}

// DoWork reserves a hotel room.
func (a *ReserveHotelActivity) DoWork(ctx context.Context, workItem saga.WorkItem) (*saga.WorkLog, error) {
	_ = workItem.Arguments["roomType"]
	reservationId := hotelRnd.Intn(100000)
	workLog := saga.NewWorkLog(a, saga.WorkResult{"reservationId": reservationId})
	return &workLog, nil
}

// Compensate cancels the hotel reservation.
func (a *ReserveHotelActivity) Compensate(ctx context.Context, workLog saga.WorkLog, routingSlip *saga.RoutingSlip) (bool, error) {
	_ = workLog.Result["reservationId"]
	return true, nil
}

// WorkItemQueueAddress returns the queue address for hotel reservation requests.
func (a *ReserveHotelActivity) WorkItemQueueAddress() string {
	return "sb://./hotelReservations"
}

// CompensationQueueAddress returns the queue address for hotel cancellation requests.
func (a *ReserveHotelActivity) CompensationQueueAddress() string {
	return "sb://./hotelCancellations"
}

// ActivityType returns the activity type function.
func (a *ReserveHotelActivity) ActivityType() saga.ActivityType {
	return NewReserveHotelActivity
}
