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
	Not() Logical
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
}

// Mathematical represents a type that supports mathematical operations.
type Mathematical interface {
	Delegating
	Add(other Mathematical) Mathematical
	Sub(other Mathematical) Mathematical
	Mul(other Mathematical) Mathematical
	Div(other Mathematical) Mathematical
	Mod(other Mathematical) Mathematical
	// Lshift and Rshift shift the bits of an integer, as the evaluator and
	// PostgreSQL have them: they are arithmetic. They were typed as
	// comparisons, giving a Logical.
	Lshift(other Mathematical) Mathematical
	Rshift(other Mathematical) Mathematical
}
