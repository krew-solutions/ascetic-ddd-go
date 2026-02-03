package specification

type Associativity string

const (
	LeftAssociative  Associativity = "LEFT"
	RightAssociative Associativity = "RIGHT"
	NonAssociative   Associativity = "NON"
)

type Operator string

const (
	// Comparison

	OperatorEq  Operator = "="
	OperatorGt  Operator = ">"
	OperatorLt  Operator = "<"
	OperatorGte Operator = ">="
	OperatorLte Operator = "<="
	OperatorNe  Operator = "!="
	OperatorIs  Operator = "IS"

	// Logical operators

	OperatorAnd Operator = "AND"
	OperatorOr  Operator = "OR"
	OperatorNot Operator = "NOT"

	// Mathematical

	OperatorAdd Operator = "+"
	OperatorSub Operator = "-"
	OperatorMul Operator = "*"
	OperatorDiv Operator = "/"
	OperatorMod Operator = "%"

	OperatorPos Operator = "+pos"
	OperatorNeg Operator = "-neg"

	// Bitwise

	OperatorLshift Operator = "<<"
	OperatorRshift Operator = ">>"

	// Postfix

	OperatorIsNull    Operator = "IS NULL"
	OperatorIsNotNull Operator = "IS NOT NULL"
)

var YieldBooleanOperators = []Operator{
	OperatorEq,
	OperatorGt,
	OperatorLt,
	OperatorGte,
	OperatorLte,
	OperatorNe,
	OperatorIs,
	OperatorAnd,
	OperatorOr,
	OperatorNot,
	OperatorIsNull,
	OperatorIsNotNull,
}

type Operable interface {
	Associativity() Associativity
	Operator() Operator
}

type Visitable interface {
	Accept(Visitor) error
}

type Visitor interface {
	VisitGlobalScope(GlobalScopeNode) error
	VisitObject(ObjectNode) error
	VisitCollection(CollectionNode) error
	VisitItem(ItemNode) error
	VisitField(FieldNode) error
	VisitValue(ValueNode) error
	VisitPrefix(PrefixNode) error
	VisitInfix(InfixNode) error
	VisitPostfix(PostfixNode) error
}

func Value(value any) ValueNode {
	return ValueNode{
		value: value,
	}
}

type ValueNode struct {
	value any
}

func (n ValueNode) Value() any {
	return n.value
}

func (n ValueNode) Accept(v Visitor) error {
	return v.VisitValue(n)
}

func Not(operand Visitable) PrefixNode {
	return PrefixNode{
		operator:      OperatorNot,
		operand:       operand,
		associativity: RightAssociative,
	}
}

func NewPrefixNode(operator Operator, operand Visitable, associativity Associativity) PrefixNode {
	return PrefixNode{
		operator:      operator,
		operand:       operand,
		associativity: associativity,
	}
}

type PrefixNode struct {
	operator      Operator
	operand       Visitable
	associativity Associativity
}

func (n PrefixNode) Operand() Visitable {
	return n.operand
}
func (n PrefixNode) Operator() Operator {
	return n.operator
}
func (n PrefixNode) Associativity() Associativity {
	return n.associativity
}
func (n PrefixNode) Accept(v Visitor) error {
	return v.VisitPrefix(n)
}

func Equal(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorEq,
		right:         right,
		associativity: NonAssociative,
	}
}

func NotEqual(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorNe,
		right:         right,
		associativity: NonAssociative,
	}
}

func GreaterThan(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorGt,
		right:         right,
		associativity: NonAssociative,
	}
}

func GreaterThanEqual(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorGte,
		right:         right,
		associativity: NonAssociative,
	}
}

func LessThan(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorLt,
		right:         right,
		associativity: NonAssociative,
	}
}

func LessThanEqual(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorLte,
		right:         right,
		associativity: NonAssociative,
	}
}

func Is(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorIs,
		right:         right,
		associativity: NonAssociative,
	}
}

func And(left Visitable, rights ...Visitable) InfixNode {
	left, right := foldRights(And, left, rights...)
	return InfixNode{
		left:          left,
		operator:      OperatorAnd,
		right:         right,
		associativity: LeftAssociative,
	}
}

func Or(left Visitable, rights ...Visitable) InfixNode {
	left, right := foldRights(Or, left, rights...)
	return InfixNode{
		left:          left,
		operator:      OperatorOr,
		right:         right,
		associativity: LeftAssociative,
	}
}

func LeftShift(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorLshift,
		right:         right,
		associativity: LeftAssociative,
	}
}

func RightShift(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorRshift,
		right:         right,
		associativity: LeftAssociative,
	}
}

func Add(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorAdd,
		right:         right,
		associativity: LeftAssociative,
	}
}

func Sub(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorSub,
		right:         right,
		associativity: LeftAssociative,
	}
}

func Mul(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorMul,
		right:         right,
		associativity: LeftAssociative,
	}
}

func Div(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorDiv,
		right:         right,
		associativity: LeftAssociative,
	}
}

func Mod(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      OperatorMod,
		right:         right,
		associativity: LeftAssociative,
	}
}

func foldRights(
	aCallable func(Visitable, ...Visitable) InfixNode,
	aLeft Visitable,
	aRights ...Visitable,
) (left, right Visitable) {
	for len(aRights) > 1 {
		aLeft = aCallable(aLeft, aRights[0])
		aRights = aRights[1:]
	}
	return aLeft, aRights[0]
}

func NewInfixNode(left Visitable, operator Operator, right Visitable, associativity Associativity) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operator,
		right:         right,
		associativity: associativity,
	}
}

type InfixNode struct {
	left          Visitable
	operator      Operator
	right         Visitable
	associativity Associativity
}

func (n InfixNode) Left() Visitable {
	return n.left
}

func (n InfixNode) Operator() Operator {
	return n.operator
}

func (n InfixNode) Right() Visitable {
	return n.right
}

func (n InfixNode) Associativity() Associativity {
	return n.associativity
}

func (n InfixNode) Accept(v Visitor) error {
	return v.VisitInfix(n)
}

func IsNull(operand Visitable) PostfixNode {
	return PostfixNode{
		operand:       operand,
		operator:      OperatorIsNull,
		associativity: NonAssociative,
	}
}

func IsNotNull(operand Visitable) PostfixNode {
	return PostfixNode{
		operand:       operand,
		operator:      OperatorIsNotNull,
		associativity: NonAssociative,
	}
}

func NewPostfixNode(operand Visitable, operator Operator, associativity Associativity) PostfixNode {
	return PostfixNode{
		operand:       operand,
		operator:      operator,
		associativity: associativity,
	}
}

type PostfixNode struct {
	operand       Visitable
	operator      Operator
	associativity Associativity
}

func (n PostfixNode) Operand() Visitable {
	return n.operand
}

func (n PostfixNode) Operator() Operator {
	return n.operator
}

func (n PostfixNode) Associativity() Associativity {
	return n.associativity
}

func (n PostfixNode) Accept(v Visitor) error {
	return v.VisitPostfix(n)
}

// TODO: Rename me to Scope?
type EmptiableObject interface {
	Visitable
	Parent() EmptiableObject
	Name() string
	IsRoot() bool
}

func GlobalScope() GlobalScopeNode {
	return GlobalScopeNode{}
}

type GlobalScopeNode struct{}

func (n GlobalScopeNode) Parent() EmptiableObject {
	return n
}

func (n GlobalScopeNode) Name() string {
	return "Empty"
}

func (n GlobalScopeNode) IsRoot() bool {
	return true
}
func (n GlobalScopeNode) Accept(v Visitor) error {
	return v.VisitGlobalScope(n)
}

func Object(parent EmptiableObject, name string) ObjectNode {
	return ObjectNode{
		parent: parent,
		name:   name,
	}
}

type ObjectNode struct {
	parent EmptiableObject
	name   string
}

func (n ObjectNode) Parent() EmptiableObject {
	return n.parent
}

func (n ObjectNode) Name() string {
	return n.name
}

func (n ObjectNode) IsRoot() bool {
	return false
}

func (n ObjectNode) Accept(v Visitor) error {
	return v.VisitObject(n)
}

func Wildcard(parent EmptiableObject, predicate Visitable) CollectionNode {
	return CollectionNode{
		parent:    parent,
		name:      "*",
		predicate: predicate,
	}
}

// Deprecated: Use Wildcard instead
func Wilcard(parent EmptiableObject, predicate Visitable) CollectionNode {
	return Wildcard(parent, predicate)
}

// See JSONPath specification for * and @, for example jsonb_path_match() in PostgreSQL.
// TODO: should it implement Field interface?
type CollectionNode struct {
	parent    EmptiableObject
	name      string // TODO: rename to slice?
	predicate Visitable
}

func (n CollectionNode) Parent() EmptiableObject {
	return n.parent
}

func (n CollectionNode) Name() string {
	return n.name
}

func (n CollectionNode) IsRoot() bool {
	return false
}

func (n CollectionNode) Predicate() Visitable {
	return n.predicate
}

func (n CollectionNode) Accept(v Visitor) error {
	return v.VisitCollection(n)
}

func Item() ItemNode {
	return ItemNode{}
}

type ItemNode struct{}

func (n ItemNode) Parent() EmptiableObject {
	return GlobalScope() // FIXME: is it correct?
}

func (n ItemNode) Name() string {
	return "@"
}

func (n ItemNode) IsRoot() bool {
	return true
}

func (n ItemNode) Accept(v Visitor) error {
	return v.VisitItem(n)
}

func Field(object EmptiableObject, name string) FieldNode {
	return FieldNode{
		object: object,
		name:   name,
	}
}

type FieldNode struct {
	object EmptiableObject
	name   string
}

func (n FieldNode) Name() string {
	return n.name
}

func (n FieldNode) Object() EmptiableObject {
	return n.object
}

func (n FieldNode) Accept(v Visitor) error {
	return v.VisitField(n)
}
