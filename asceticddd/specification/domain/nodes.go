package specification

import (
	"fmt"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

type Associativity string

const (
	LeftAssociative  Associativity = "LEFT"
	RightAssociative Associativity = "RIGHT"
	NonAssociative   Associativity = "NON"
)

type Operable interface {
	Associativity() Associativity
	Operator() operators.Operator
}

// Visitable is a sealed marker for AST nodes.
//
// Dispatch happens through the free function Accept[T] rather than a method
// on Visitable because Go does not allow generic methods on interfaces.
//
// Types in other packages satisfy this interface by embedding VisitableMarker.
type Visitable interface {
	visitableNode()
}

// VisitableMarker is embedded in node types defined outside the domain
// package to satisfy the Visitable marker. The private method makes the
// marker effectively sealed against accidental implementations while still
// allowing intentional ones via explicit embedding.
type VisitableMarker struct{}

func (VisitableMarker) visitableNode() {}

// Visitor is the parameterised visitor interface for AST traversal.
// T is the result type produced by each visit method.
type Visitor[T any] interface {
	VisitGlobalScope(GlobalScopeNode) (T, error)
	VisitObject(ObjectNode) (T, error)
	VisitCollection(CollectionNode) (T, error)
	VisitItem(ItemNode) (T, error)
	VisitField(FieldNode) (T, error)
	VisitValue(ValueNode) (T, error)
	VisitPrefix(PrefixNode) (T, error)
	VisitInfix(InfixNode) (T, error)
	VisitPostfix(PostfixNode) (T, error)
}

// Accept dispatches a Visitable node to the matching visit method on v.
//
// This is a free function (not a method on Visitable) because Go interface
// methods cannot be generic. Type inference recovers T from the visitor
// argument, so call sites usually omit the explicit type parameter.
func Accept[T any](node Visitable, v Visitor[T]) (T, error) {
	switch n := node.(type) {
	case ValueNode:
		return v.VisitValue(n)
	case FieldNode:
		return v.VisitField(n)
	case ObjectNode:
		return v.VisitObject(n)
	case ItemNode:
		return v.VisitItem(n)
	case GlobalScopeNode:
		return v.VisitGlobalScope(n)
	case CollectionNode:
		return v.VisitCollection(n)
	case PrefixNode:
		return v.VisitPrefix(n)
	case InfixNode:
		return v.VisitInfix(n)
	case PostfixNode:
		return v.VisitPostfix(n)
	default:
		var zero T
		return zero, fmt.Errorf("unknown visitable node type: %T", node)
	}
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

func (ValueNode) visitableNode() {}

func Not(operand Visitable) PrefixNode {
	return PrefixNode{
		operator:      operators.OperatorNot,
		operand:       operand,
		associativity: RightAssociative,
	}
}

func NewPrefixNode(operator operators.Operator, operand Visitable, associativity Associativity) PrefixNode {
	return PrefixNode{
		operator:      operator,
		operand:       operand,
		associativity: associativity,
	}
}

type PrefixNode struct {
	operator      operators.Operator
	operand       Visitable
	associativity Associativity
}

func (n PrefixNode) Operand() Visitable {
	return n.operand
}
func (n PrefixNode) Operator() operators.Operator {
	return n.operator
}
func (n PrefixNode) Associativity() Associativity {
	return n.associativity
}
func (PrefixNode) visitableNode() {}

func Equal(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorEq,
		right:         right,
		associativity: NonAssociative,
	}
}

func NotEqual(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorNe,
		right:         right,
		associativity: NonAssociative,
	}
}

func GreaterThan(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorGt,
		right:         right,
		associativity: NonAssociative,
	}
}

func GreaterThanEqual(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorGte,
		right:         right,
		associativity: NonAssociative,
	}
}

func LessThan(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorLt,
		right:         right,
		associativity: NonAssociative,
	}
}

func LessThanEqual(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorLte,
		right:         right,
		associativity: NonAssociative,
	}
}

func Is(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorIs,
		right:         right,
		associativity: NonAssociative,
	}
}

func And(left Visitable, rights ...Visitable) InfixNode {
	left, right := foldRights(And, left, rights...)
	return InfixNode{
		left:          left,
		operator:      operators.OperatorAnd,
		right:         right,
		associativity: LeftAssociative,
	}
}

func Or(left Visitable, rights ...Visitable) InfixNode {
	left, right := foldRights(Or, left, rights...)
	return InfixNode{
		left:          left,
		operator:      operators.OperatorOr,
		right:         right,
		associativity: LeftAssociative,
	}
}

func LeftShift(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorLshift,
		right:         right,
		associativity: LeftAssociative,
	}
}

func RightShift(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorRshift,
		right:         right,
		associativity: LeftAssociative,
	}
}

func Add(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorAdd,
		right:         right,
		associativity: LeftAssociative,
	}
}

func Sub(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorSub,
		right:         right,
		associativity: LeftAssociative,
	}
}

func Mul(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorMul,
		right:         right,
		associativity: LeftAssociative,
	}
}

func Div(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorDiv,
		right:         right,
		associativity: LeftAssociative,
	}
}

func Mod(left, right Visitable) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operators.OperatorMod,
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

func NewInfixNode(left Visitable, operator operators.Operator, right Visitable, associativity Associativity) InfixNode {
	return InfixNode{
		left:          left,
		operator:      operator,
		right:         right,
		associativity: associativity,
	}
}

type InfixNode struct {
	left          Visitable
	operator      operators.Operator
	right         Visitable
	associativity Associativity
}

func (n InfixNode) Left() Visitable {
	return n.left
}

func (n InfixNode) Operator() operators.Operator {
	return n.operator
}

func (n InfixNode) Right() Visitable {
	return n.right
}

func (n InfixNode) Associativity() Associativity {
	return n.associativity
}

func (InfixNode) visitableNode() {}

func IsNull(operand Visitable) PostfixNode {
	return PostfixNode{
		operand:       operand,
		operator:      operators.OperatorIsNull,
		associativity: NonAssociative,
	}
}

func IsNotNull(operand Visitable) PostfixNode {
	return PostfixNode{
		operand:       operand,
		operator:      operators.OperatorIsNotNull,
		associativity: NonAssociative,
	}
}

func NewPostfixNode(operand Visitable, operator operators.Operator, associativity Associativity) PostfixNode {
	return PostfixNode{
		operand:       operand,
		operator:      operator,
		associativity: associativity,
	}
}

type PostfixNode struct {
	operand       Visitable
	operator      operators.Operator
	associativity Associativity
}

func (n PostfixNode) Operand() Visitable {
	return n.operand
}

func (n PostfixNode) Operator() operators.Operator {
	return n.operator
}

func (n PostfixNode) Associativity() Associativity {
	return n.associativity
}

func (PostfixNode) visitableNode() {}

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

func (GlobalScopeNode) visitableNode() {}

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

func (ObjectNode) visitableNode() {}

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

func (CollectionNode) visitableNode() {}

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

func (ItemNode) visitableNode() {}

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

func (FieldNode) visitableNode() {}
