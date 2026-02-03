package public

import (
	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

// Delegating represents a type that can delegate to a Visitable node.
type Delegating interface {
	Delegate() s.Visitable
}

// Nullable represents a type that supports NULL checks.
type Nullable interface {
	Delegating
	IsNull() Logical
	IsNotNull() Logical
}

// Logical represents a type that supports logical operations.
type Logical interface {
	Delegating
	And(other Logical) Logical
	Or(other Logical) Logical
	Is(other Logical) Logical
}

// Comparison represents a type that supports comparison operations.
type Comparison interface {
	Delegating
	Eq(other Comparison) Logical
	Ne(other Comparison) Logical
	Gt(other Comparison) Logical
	Lt(other Comparison) Logical
	Gte(other Comparison) Logical
	Lte(other Comparison) Logical
	Lshift(other Comparison) Logical
	Rshift(other Comparison) Logical
}

// Mathematical represents a type that supports mathematical operations.
type Mathematical interface {
	Delegating
	Add(other Mathematical) Mathematical
	Sub(other Mathematical) Mathematical
	Mul(other Mathematical) Mathematical
	Div(other Mathematical) Mathematical
	Mod(other Mathematical) Mathematical
}
