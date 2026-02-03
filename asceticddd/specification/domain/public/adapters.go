package public

import (
	"strings"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

// DelegatingImp wraps a Visitable node and implements Delegating.
type DelegatingImp struct {
	delegate s.Visitable
}

// NewDelegating creates a new DelegatingImp instance.
func NewDelegating(delegate s.Visitable) DelegatingImp {
	return DelegatingImp{delegate: delegate}
}

// Delegate returns the wrapped Visitable node.
func (d DelegatingImp) Delegate() s.Visitable {
	return d.delegate
}

// Note: Factory pattern from Python is implemented via MakeXxxField/MakeXxxValue functions
// in datatypes.go since Go doesn't support Python-style class methods on generic types

// LogicalImp implements Logical interface.
type LogicalImp struct {
	DelegatingImp
}

// NewLogical creates a new LogicalImp instance.
func NewLogical(delegate s.Visitable) LogicalImp {
	return LogicalImp{DelegatingImp: NewDelegating(delegate)}
}

// And creates an AND operation.
func (l LogicalImp) And(other Logical) Logical {
	return NewLogical(s.And(l.Delegate(), other.Delegate()))
}

// Or creates an OR operation.
func (l LogicalImp) Or(other Logical) Logical {
	return NewLogical(s.Or(l.Delegate(), other.Delegate()))
}

// Is creates an IS operation.
func (l LogicalImp) Is(other Logical) Logical {
	return NewLogical(s.Is(l.Delegate(), other.Delegate()))
}

// NullableImp implements Nullable interface.
type NullableImp struct {
	DelegatingImp
}

// NewNullable creates a new NullableImp instance.
func NewNullable(delegate s.Visitable) NullableImp {
	return NullableImp{DelegatingImp: NewDelegating(delegate)}
}

// IsNull creates an IS NULL check.
func (n NullableImp) IsNull() Logical {
	return NewLogical(s.IsNull(n.Delegate()))
}

// IsNotNull creates an IS NOT NULL check.
func (n NullableImp) IsNotNull() Logical {
	return NewLogical(s.IsNotNull(n.Delegate()))
}

// ComparisonImp implements Comparison interface.
type ComparisonImp struct {
	DelegatingImp
}

// NewComparison creates a new ComparisonImp instance.
func NewComparison(delegate s.Visitable) ComparisonImp {
	return ComparisonImp{DelegatingImp: NewDelegating(delegate)}
}

// Eq creates an equality comparison.
func (c ComparisonImp) Eq(other Comparison) Logical {
	return NewLogical(s.Equal(c.Delegate(), other.Delegate()))
}

// Ne creates an inequality comparison.
func (c ComparisonImp) Ne(other Comparison) Logical {
	return NewLogical(s.NotEqual(c.Delegate(), other.Delegate()))
}

// Gt creates a greater-than comparison.
func (c ComparisonImp) Gt(other Comparison) Logical {
	return NewLogical(s.GreaterThan(c.Delegate(), other.Delegate()))
}

// Lt creates a less-than comparison.
func (c ComparisonImp) Lt(other Comparison) Logical {
	return NewLogical(s.LessThan(c.Delegate(), other.Delegate()))
}

// Gte creates a greater-than-or-equal comparison.
func (c ComparisonImp) Gte(other Comparison) Logical {
	return NewLogical(s.GreaterThanEqual(c.Delegate(), other.Delegate()))
}

// Lte creates a less-than-or-equal comparison.
func (c ComparisonImp) Lte(other Comparison) Logical {
	return NewLogical(s.LessThanEqual(c.Delegate(), other.Delegate()))
}

// Lshift creates a left-shift operation.
func (c ComparisonImp) Lshift(other Comparison) Logical {
	return NewLogical(s.LeftShift(c.Delegate(), other.Delegate()))
}

// Rshift creates a right-shift operation.
func (c ComparisonImp) Rshift(other Comparison) Logical {
	return NewLogical(s.RightShift(c.Delegate(), other.Delegate()))
}

// MathematicalImp implements Mathematical interface.
type MathematicalImp struct {
	DelegatingImp
}

// NewMathematical creates a new MathematicalImp instance.
func NewMathematical(delegate s.Visitable) MathematicalImp {
	return MathematicalImp{DelegatingImp: NewDelegating(delegate)}
}

// Add creates an addition operation.
func (m MathematicalImp) Add(other Mathematical) Mathematical {
	return NewMathematical(s.Add(m.Delegate(), other.Delegate()))
}

// Sub creates a subtraction operation.
func (m MathematicalImp) Sub(other Mathematical) Mathematical {
	return NewMathematical(s.Sub(m.Delegate(), other.Delegate()))
}

// Mul creates a multiplication operation.
func (m MathematicalImp) Mul(other Mathematical) Mathematical {
	return NewMathematical(s.Mul(m.Delegate(), other.Delegate()))
}

// Div creates a division operation.
func (m MathematicalImp) Div(other Mathematical) Mathematical {
	return NewMathematical(s.Div(m.Delegate(), other.Delegate()))
}

// Mod creates a modulo operation.
func (m MathematicalImp) Mod(other Mathematical) Mathematical {
	return NewMathematical(s.Mod(m.Delegate(), other.Delegate()))
}

// Object_ creates an Object node from a dotted path string.
// Example: Object_("user.profile") creates nested Object nodes.
func Object_(name string) s.ObjectNode {
	var parent s.EmptiableObject = s.GlobalScope()
	parts := strings.Split(name, ".")
	for _, part := range parts {
		parent = s.Object(parent, part)
	}
	return parent.(s.ObjectNode)
}

// Field creates a Field node from a dotted path string.
// Example: Field("user.name") creates Object("user") with Field("name").
func Field(name string) s.FieldNode {
	idx := strings.LastIndex(name, ".")
	if idx != -1 {
		return s.Field(Object_(name[:idx]), name[idx+1:])
	}
	return s.Field(s.GlobalScope(), name)
}
