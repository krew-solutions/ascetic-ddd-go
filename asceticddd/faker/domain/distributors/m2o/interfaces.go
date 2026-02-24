package m2o

import (
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/faker/domain/specification"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/option"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/signals"
)

// IExternalSource is a marker interface for external data sources.
type IExternalSource interface{}

// ICursor signals that a new value should be created.
// Returned from Next when the distribution mean is reached.
type ICursor[T any] interface {
	Position() int
	Append(s session.Session, value T) error
}

// IM2ODistributor manages Many-to-One value distribution.
type IM2ODistributor[T any] interface {
	OnAppended() signals.Signal[ValueAppendedEvent[T]]
	Next(s session.Session, spec specification.Specification[T]) (option.Option[T], ICursor[T], error)
	Append(s session.Session, value T) error
	ProviderName() string
	SetProviderName(name string)
	Setup(s session.Session) error
	Cleanup(s session.Session) error
	Clone() IM2ODistributor[T]
	BindExternalSource(externalSource IExternalSource)
}
