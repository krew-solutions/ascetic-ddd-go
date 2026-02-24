package specification

import (
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

// Visitor processes specification nodes.
type Visitor interface {
	VisitQuerySpecification(query any, aggregateProviderAccessor func() any)
	VisitEmptySpecification()
	VisitScopeSpecification(scope any)
}

// Visitable can be visited by a Visitor.
type Visitable interface {
	Accept(visitor Visitor)
}

// Specification filters objects by criteria.
type Specification[T any] interface {
	Visitable
	IsSatisfiedBy(s session.Session, obj T) (bool, error)
}

// ResolvableSpecification requires pre-resolving nested constraints.
type ResolvableSpecification[T any] interface {
	Specification[T]
	ResolveNested(s session.Session) error
}
