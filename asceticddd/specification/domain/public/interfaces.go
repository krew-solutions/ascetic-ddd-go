package public

import (
	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

// IDelegating represents a type that can delegate to a Visitable node.
type IDelegating interface {
	Delegate() s.Visitable
}

// INullable represents a type that supports NULL checks.
type INullable interface {
	IDelegating
	IsNull() ILogical
	IsNotNull() ILogical
}

// ILogical represents a type that supports logical operations.
type ILogical interface {
	IDelegating
	And(other ILogical) ILogical
	Or(other ILogical) ILogical
	Is(other ILogical) ILogical
}

// IComparison represents a type that supports comparison operations.
type IComparison interface {
	IDelegating
	Eq(other IComparison) ILogical
	Ne(other IComparison) ILogical
	Gt(other IComparison) ILogical
	Lt(other IComparison) ILogical
	Gte(other IComparison) ILogical
	Lte(other IComparison) ILogical
	Lshift(other IComparison) ILogical
	Rshift(other IComparison) ILogical
}

// IMathematical represents a type that supports mathematical operations.
type IMathematical interface {
	IDelegating
	Add(other IMathematical) IMathematical
	Sub(other IMathematical) IMathematical
	Mul(other IMathematical) IMathematical
	Div(other IMathematical) IMathematical
	Mod(other IMathematical) IMathematical
}
