package m2o

import (
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

// ValueAppendedEvent is emitted when a value is appended to the distributor.
type ValueAppendedEvent[T any] struct {
	Session  session.Session
	Value    T
	Position int
}
