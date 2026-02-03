package public

import (
	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

// Boolean represents a boolean field that supports logical operations.
type Boolean struct {
	*LogicalImp
}

// NewBoolean creates a new Boolean instance.
func NewBoolean(delegate s.Visitable) *Boolean {
	return &Boolean{LogicalImp: NewLogical(delegate)}
}

// MakeBooleanField creates a Boolean field from a field name.
func MakeBooleanField(name string) *Boolean {
	return NewBoolean(Field(name))
}

// MakeBooleanValue creates a Boolean value.
func MakeBooleanValue(value bool) *Boolean {
	return NewBoolean(s.Value(value))
}

// NullBoolean represents a nullable boolean field.
type NullBoolean struct {
	*Boolean
}

// NewNullBoolean creates a new NullBoolean instance.
func NewNullBoolean(delegate s.Visitable) *NullBoolean {
	return &NullBoolean{
		Boolean: NewBoolean(delegate),
	}
}

// MakeNullBooleanField creates a NullBoolean field from a field name.
func MakeNullBooleanField(name string) *NullBoolean {
	return NewNullBoolean(Field(name))
}

// MakeNullBooleanValue creates a NullBoolean value.
func MakeNullBooleanValue(value any) *NullBoolean {
	return NewNullBoolean(s.Value(value))
}

// Nullable methods for NullBoolean
func (n *NullBoolean) IsNull() Logical {
	return NewLogical(s.IsNull(n.Delegate()))
}

func (n *NullBoolean) IsNotNull() Logical {
	return NewLogical(s.IsNotNull(n.Delegate()))
}

// Number represents a numeric field that supports comparison and mathematical operations.
type Number struct {
	*DelegatingImp
}

// NewNumber creates a new Number instance.
func NewNumber(delegate s.Visitable) *Number {
	return &Number{
		DelegatingImp: NewDelegating(delegate),
	}
}

// MakeNumberField creates a Number field from a field name.
func MakeNumberField(name string) *Number {
	return NewNumber(Field(name))
}

// MakeNumberValue creates a Number value.
func MakeNumberValue(value any) *Number {
	return NewNumber(s.Value(value))
}

// Comparison methods
func (n *Number) Eq(other Comparison) Logical {
	return NewLogical(s.Equal(n.Delegate(), other.Delegate()))
}

func (n *Number) Ne(other Comparison) Logical {
	return NewLogical(s.NotEqual(n.Delegate(), other.Delegate()))
}

func (n *Number) Gt(other Comparison) Logical {
	return NewLogical(s.GreaterThan(n.Delegate(), other.Delegate()))
}

func (n *Number) Lt(other Comparison) Logical {
	return NewLogical(s.LessThan(n.Delegate(), other.Delegate()))
}

func (n *Number) Gte(other Comparison) Logical {
	return NewLogical(s.GreaterThanEqual(n.Delegate(), other.Delegate()))
}

func (n *Number) Lte(other Comparison) Logical {
	return NewLogical(s.LessThanEqual(n.Delegate(), other.Delegate()))
}

func (n *Number) Lshift(other Comparison) Logical {
	return NewLogical(s.LeftShift(n.Delegate(), other.Delegate()))
}

func (n *Number) Rshift(other Comparison) Logical {
	return NewLogical(s.RightShift(n.Delegate(), other.Delegate()))
}

// Mathematical methods
func (n *Number) Add(other Mathematical) Mathematical {
	return NewNumber(s.Add(n.Delegate(), other.Delegate()))
}

func (n *Number) Sub(other Mathematical) Mathematical {
	return NewNumber(s.Sub(n.Delegate(), other.Delegate()))
}

func (n *Number) Mul(other Mathematical) Mathematical {
	return NewNumber(s.Mul(n.Delegate(), other.Delegate()))
}

func (n *Number) Div(other Mathematical) Mathematical {
	return NewNumber(s.Div(n.Delegate(), other.Delegate()))
}

func (n *Number) Mod(other Mathematical) Mathematical {
	return NewNumber(s.Mod(n.Delegate(), other.Delegate()))
}

// NullNumber represents a nullable numeric field.
type NullNumber struct {
	*Number
}

// NewNullNumber creates a new NullNumber instance.
func NewNullNumber(delegate s.Visitable) *NullNumber {
	return &NullNumber{
		Number: NewNumber(delegate),
	}
}

// MakeNullNumberField creates a NullNumber field from a field name.
func MakeNullNumberField(name string) *NullNumber {
	return NewNullNumber(Field(name))
}

// MakeNullNumberValue creates a NullNumber value.
func MakeNullNumberValue(value any) *NullNumber {
	return NewNullNumber(s.Value(value))
}

// Nullable methods for NullNumber
func (n *NullNumber) IsNull() Logical {
	return NewLogical(s.IsNull(n.Delegate()))
}

func (n *NullNumber) IsNotNull() Logical {
	return NewLogical(s.IsNotNull(n.Delegate()))
}

// Datetime represents a datetime field that supports comparison and mathematical operations.
type Datetime struct {
	*DelegatingImp
}

// NewDatetime creates a new Datetime instance.
func NewDatetime(delegate s.Visitable) *Datetime {
	return &Datetime{
		DelegatingImp: NewDelegating(delegate),
	}
}

// MakeDatetimeField creates a Datetime field from a field name.
func MakeDatetimeField(name string) *Datetime {
	return NewDatetime(Field(name))
}

// MakeDatetimeValue creates a Datetime value.
func MakeDatetimeValue(value any) *Datetime {
	return NewDatetime(s.Value(value))
}

// Comparison methods for Datetime
func (d *Datetime) Eq(other Comparison) Logical {
	return NewLogical(s.Equal(d.Delegate(), other.Delegate()))
}

func (d *Datetime) Ne(other Comparison) Logical {
	return NewLogical(s.NotEqual(d.Delegate(), other.Delegate()))
}

func (d *Datetime) Gt(other Comparison) Logical {
	return NewLogical(s.GreaterThan(d.Delegate(), other.Delegate()))
}

func (d *Datetime) Lt(other Comparison) Logical {
	return NewLogical(s.LessThan(d.Delegate(), other.Delegate()))
}

func (d *Datetime) Gte(other Comparison) Logical {
	return NewLogical(s.GreaterThanEqual(d.Delegate(), other.Delegate()))
}

func (d *Datetime) Lte(other Comparison) Logical {
	return NewLogical(s.LessThanEqual(d.Delegate(), other.Delegate()))
}

func (d *Datetime) Lshift(other Comparison) Logical {
	return NewLogical(s.LeftShift(d.Delegate(), other.Delegate()))
}

func (d *Datetime) Rshift(other Comparison) Logical {
	return NewLogical(s.RightShift(d.Delegate(), other.Delegate()))
}

// Mathematical methods for Datetime (for date arithmetic)
func (d *Datetime) Add(other Mathematical) Mathematical {
	return NewMathematical(s.Add(d.Delegate(), other.Delegate()))
}

func (d *Datetime) Sub(other Mathematical) Mathematical {
	return NewMathematical(s.Sub(d.Delegate(), other.Delegate()))
}

func (d *Datetime) Mul(other Mathematical) Mathematical {
	return NewMathematical(s.Mul(d.Delegate(), other.Delegate()))
}

func (d *Datetime) Div(other Mathematical) Mathematical {
	return NewMathematical(s.Div(d.Delegate(), other.Delegate()))
}

func (d *Datetime) Mod(other Mathematical) Mathematical {
	return NewMathematical(s.Mod(d.Delegate(), other.Delegate()))
}

// NullDatetime represents a nullable datetime field.
type NullDatetime struct {
	*Datetime
}

// NewNullDatetime creates a new NullDatetime instance.
func NewNullDatetime(delegate s.Visitable) *NullDatetime {
	return &NullDatetime{
		Datetime: NewDatetime(delegate),
	}
}

// MakeNullDatetimeField creates a NullDatetime field from a field name.
func MakeNullDatetimeField(name string) *NullDatetime {
	return NewNullDatetime(Field(name))
}

// MakeNullDatetimeValue creates a NullDatetime value.
func MakeNullDatetimeValue(value any) *NullDatetime {
	return NewNullDatetime(s.Value(value))
}

// Nullable methods for NullDatetime
func (n *NullDatetime) IsNull() Logical {
	return NewLogical(s.IsNull(n.Delegate()))
}

func (n *NullDatetime) IsNotNull() Logical {
	return NewLogical(s.IsNotNull(n.Delegate()))
}

// Text represents a text field that supports comparison operations.
type Text struct {
	*ComparisonImp
}

// NewText creates a new Text instance.
func NewText(delegate s.Visitable) *Text {
	return &Text{ComparisonImp: NewComparison(delegate)}
}

// MakeTextField creates a Text field from a field name.
func MakeTextField(name string) *Text {
	return NewText(Field(name))
}

// MakeTextValue creates a Text value.
func MakeTextValue(value string) *Text {
	return NewText(s.Value(value))
}

// NullText represents a nullable text field.
type NullText struct {
	*Text
}

// NewNullText creates a new NullText instance.
func NewNullText(delegate s.Visitable) *NullText {
	return &NullText{
		Text: NewText(delegate),
	}
}

// MakeNullTextField creates a NullText field from a field name.
func MakeNullTextField(name string) *NullText {
	return NewNullText(Field(name))
}

// MakeNullTextValue creates a NullText value.
func MakeNullTextValue(value any) *NullText {
	return NewNullText(s.Value(value))
}

// Nullable methods for NullText
func (n *NullText) IsNull() Logical {
	return NewLogical(s.IsNull(n.Delegate()))
}

func (n *NullText) IsNotNull() Logical {
	return NewLogical(s.IsNotNull(n.Delegate()))
}
