package public

import (
	"strings"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

// Delegating wraps a Visitable node and implements IDelegating.
type Delegating struct {
	delegate s.Visitable
}

// NewDelegating creates a new Delegating instance.
func NewDelegating(delegate s.Visitable) *Delegating {
	return &Delegating{delegate: delegate}
}

// Delegate returns the wrapped Visitable node.
func (d *Delegating) Delegate() s.Visitable {
	return d.delegate
}

// Note: Factory pattern from Python is implemented via MakeXxxField/MakeXxxValue functions
// in datatypes.go since Go doesn't support Python-style class methods on generic types

// Logical implements ILogical interface.
type Logical struct {
	*Delegating
}

// NewLogical creates a new Logical instance.
func NewLogical(delegate s.Visitable) *Logical {
	return &Logical{Delegating: NewDelegating(delegate)}
}

// And creates an AND operation.
func (l *Logical) And(other ILogical) ILogical {
	return NewLogical(s.And(l.Delegate(), other.Delegate()))
}

// Or creates an OR operation.
func (l *Logical) Or(other ILogical) ILogical {
	return NewLogical(s.Or(l.Delegate(), other.Delegate()))
}

// Is creates an IS operation.
func (l *Logical) Is(other ILogical) ILogical {
	return NewLogical(s.Is(l.Delegate(), other.Delegate()))
}

// Nullable implements INullable interface.
type Nullable struct {
	*Delegating
}

// NewNullable creates a new Nullable instance.
func NewNullable(delegate s.Visitable) *Nullable {
	return &Nullable{Delegating: NewDelegating(delegate)}
}

// IsNull creates an IS NULL check.
func (n *Nullable) IsNull() ILogical {
	return NewLogical(s.IsNull(n.Delegate()))
}

// IsNotNull creates an IS NOT NULL check.
func (n *Nullable) IsNotNull() ILogical {
	return NewLogical(s.IsNotNull(n.Delegate()))
}

// Comparison implements IComparison interface.
type Comparison struct {
	*Delegating
}

// NewComparison creates a new Comparison instance.
func NewComparison(delegate s.Visitable) *Comparison {
	return &Comparison{Delegating: NewDelegating(delegate)}
}

// Eq creates an equality comparison.
func (c *Comparison) Eq(other IComparison) ILogical {
	return NewLogical(s.Equal(c.Delegate(), other.Delegate()))
}

// Ne creates an inequality comparison.
func (c *Comparison) Ne(other IComparison) ILogical {
	return NewLogical(s.NotEqual(c.Delegate(), other.Delegate()))
}

// Gt creates a greater-than comparison.
func (c *Comparison) Gt(other IComparison) ILogical {
	return NewLogical(s.GreaterThan(c.Delegate(), other.Delegate()))
}

// Lt creates a less-than comparison.
func (c *Comparison) Lt(other IComparison) ILogical {
	return NewLogical(s.LessThan(c.Delegate(), other.Delegate()))
}

// Gte creates a greater-than-or-equal comparison.
func (c *Comparison) Gte(other IComparison) ILogical {
	return NewLogical(s.GreaterThanEqual(c.Delegate(), other.Delegate()))
}

// Lte creates a less-than-or-equal comparison.
func (c *Comparison) Lte(other IComparison) ILogical {
	return NewLogical(s.LessThanEqual(c.Delegate(), other.Delegate()))
}

// Lshift creates a left-shift operation.
func (c *Comparison) Lshift(other IComparison) ILogical {
	return NewLogical(s.LeftShift(c.Delegate(), other.Delegate()))
}

// Rshift creates a right-shift operation.
func (c *Comparison) Rshift(other IComparison) ILogical {
	return NewLogical(s.RightShift(c.Delegate(), other.Delegate()))
}

// Mathematical implements IMathematical interface.
type Mathematical struct {
	*Delegating
}

// NewMathematical creates a new Mathematical instance.
func NewMathematical(delegate s.Visitable) *Mathematical {
	return &Mathematical{Delegating: NewDelegating(delegate)}
}

// Add creates an addition operation.
func (m *Mathematical) Add(other IMathematical) IMathematical {
	return NewMathematical(s.Add(m.Delegate(), other.Delegate()))
}

// Sub creates a subtraction operation.
func (m *Mathematical) Sub(other IMathematical) IMathematical {
	return NewMathematical(s.Sub(m.Delegate(), other.Delegate()))
}

// Mul creates a multiplication operation.
func (m *Mathematical) Mul(other IMathematical) IMathematical {
	return NewMathematical(s.Mul(m.Delegate(), other.Delegate()))
}

// Div creates a division operation.
func (m *Mathematical) Div(other IMathematical) IMathematical {
	return NewMathematical(s.Div(m.Delegate(), other.Delegate()))
}

// Mod creates a modulo operation.
func (m *Mathematical) Mod(other IMathematical) IMathematical {
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
