package specification

import (
	"errors"
	"fmt"
	"strings"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

var (
	ErrCompositeExpressionsDifferentLength   = errors.New("composite expressions have different length")
	ErrCompositeExpressionIsEmpty            = errors.New("a composite expression has no parts")
	ErrCompositeExpressionWhereANodeIsNeeded = errors.New("a composite expression where a single one is needed: only = and != compare composites")
)

// Context maps domain field paths and values to infrastructure AST nodes.
//
// AttrNode and ValueNode are what every mapping must say. The rest has an
// answer in ContextDefaults, which a mapping embeds and leaves alone unless it
// has something to add.
type Context interface {
	AttrNode(path []string) (Mapped, error)
	ValueNode(val any) (Mapped, error)
	// TODO: с вложенными контекстами ValueNode не будет работать, т.к. ValueNode может идти первым операндом. Нужно разделять интерфейсы.

	// ItemAttrNode maps a field path of the current item of a collection. A
	// path from Item() and a path from GlobalScope() can be of the same names,
	// and are not the same field.
	ItemAttrNode(path []string) (Mapped, error)
	// CollectionNode maps the path of a collection of the candidate to the
	// object its items are of: ["parts"] -> Object(GlobalScope(), "something_parts").
	CollectionNode(path []string) (s.EmptiableObject, error)
	// ItemCollectionNode maps the path of a collection of the current item:
	// ["parts"] -> Object(Item(), "sub_parts").
	ItemCollectionNode(path []string) (s.EmptiableObject, error)
}

// ContextDefaults is embedded by a Context for what it may leave unsaid.
type ContextDefaults struct{}

// ItemAttrNode has no answer but the mapping's: left as it is, the field
// would reach the query under the domain's name.
func (ContextDefaults) ItemAttrNode(path []string) (Mapped, error) {
	return nil, fmt.Errorf(
		"the context does not map the fields of an item of a collection: %s",
		strings.Join(path, "."),
	)
}

// CollectionNode keeps the collection at the same place.
func (ContextDefaults) CollectionNode(path []string) (s.EmptiableObject, error) {
	return objectChain(s.GlobalScope(), path), nil
}

// ItemCollectionNode keeps the collection at the same place.
func (ContextDefaults) ItemCollectionNode(path []string) (s.EmptiableObject, error) {
	return objectChain(s.Item(), path), nil
}

func objectChain(root s.EmptiableObject, names []string) s.EmptiableObject {
	result := root
	for _, name := range names {
		result = s.Object(result, name)
	}
	return result
}

// node returns a node, which a composite is not. Where one node is needed -
// under any operator but = and !=, as a predicate, as the whole
// specification - a composite is an error.
func node(mapped Mapped) (s.Visitable, error) {
	scalar, ok := mapped.(ScalarExpression)
	if !ok {
		return nil, ErrCompositeExpressionWhereANodeIsNeeded
	}
	return scalar.Node(), nil
}

// TransformVisitor transforms a domain specification AST to an infrastructure
// specification AST. Stateless: each visit method returns what its node is
// mapped to - a node, or, for a field or a value that the context maps so, a
// composite, which the VisitInfix above it turns into nodes.
type TransformVisitor struct {
	context Context
}

func NewTransformVisitor(context Context) *TransformVisitor {
	return &TransformVisitor{context: context}
}

// Transform is the typed entry point. Of a part of the tree the result may be
// a composite; of the whole tree it may not.
func (v *TransformVisitor) Transform(exp s.Visitable) (s.Visitable, error) {
	return v.node(exp)
}

// node transforms an operand that must be a single node.
func (v *TransformVisitor) node(exp s.Visitable) (s.Visitable, error) {
	mapped, err := s.Accept[Mapped](exp, v)
	if err != nil {
		return nil, err
	}
	return node(mapped)
}

func (v *TransformVisitor) VisitGlobalScope(n s.GlobalScopeNode) (Mapped, error) {
	return Scalar(n), nil
}

func (v *TransformVisitor) VisitObject(n s.ObjectNode) (Mapped, error) {
	return Scalar(n), nil
}

// VisitCollection transforms the predicate, and asks the context where the
// collection is kept. It used to be a passthrough.
func (v *TransformVisitor) VisitCollection(n s.CollectionNode) (Mapped, error) {
	parent, err := v.transformCollectionParent(n.Parent())
	if err != nil {
		return nil, err
	}
	predicate, err := v.node(n.Predicate())
	if err != nil {
		return nil, err
	}
	return Scalar(s.Wildcard(parent, predicate)), nil
}

func (v *TransformVisitor) transformCollectionParent(parent s.EmptiableObject) (s.EmptiableObject, error) {
	if _, ok := s.ExtractObjectRoot(parent).(s.ItemNode); ok {
		return v.context.ItemCollectionNode(s.ExtractObjectPath(parent))
	}
	return v.context.CollectionNode(s.ExtractObjectPath(parent))
}

func (v *TransformVisitor) VisitItem(n s.ItemNode) (Mapped, error) {
	return Scalar(n), nil
}

func (v *TransformVisitor) VisitField(n s.FieldNode) (Mapped, error) {
	if _, ok := s.ExtractFieldRoot(n).(s.ItemNode); ok {
		return v.context.ItemAttrNode(s.ExtractFieldPath(n))
	}
	return v.context.AttrNode(s.ExtractFieldPath(n))
}

func (v *TransformVisitor) VisitValue(n s.ValueNode) (Mapped, error) {
	return v.context.ValueNode(n.Value())
}

func (v *TransformVisitor) VisitPrefix(n s.PrefixNode) (Mapped, error) {
	operand, err := v.node(n.Operand())
	if err != nil {
		return nil, err
	}
	return Scalar(s.NewPrefixNode(n.Operator(), operand, n.Associativity())), nil
}

func (v *TransformVisitor) VisitPostfix(n s.PostfixNode) (Mapped, error) {
	operand, err := v.node(n.Operand())
	if err != nil {
		return nil, err
	}
	return Scalar(s.NewPostfixNode(operand, n.Operator(), n.Associativity())), nil
}

func (v *TransformVisitor) VisitInfix(n s.InfixNode) (Mapped, error) {
	left, err := s.Accept[Mapped](n.Left(), v)
	if err != nil {
		return nil, err
	}
	right, err := s.Accept[Mapped](n.Right(), v)
	if err != nil {
		return nil, err
	}

	// A composite on either side: one on the right alone used to go into the
	// tree as it was.
	leftComposite, leftIs := left.(CompositeExpressionNode)
	rightComposite, rightIs := right.(CompositeExpressionNode)
	if leftIs || rightIs {
		if !leftIs || !rightIs {
			return nil, errors.New("not enough composite expressions")
		}
		var result s.Visitable
		switch n.Operator() {
		case operators.OperatorEq:
			result, err = leftComposite.Equal(rightComposite)
		case operators.OperatorNe:
			result, err = leftComposite.NotEqual(rightComposite)
		default:
			return nil, fmt.Errorf("operator %q is not supported for composite expressions", n.Operator())
		}
		if err != nil {
			return nil, err
		}
		return Scalar(result), nil
	}

	leftNode, err := node(left)
	if err != nil {
		return nil, err
	}
	rightNode, err := node(right)
	if err != nil {
		return nil, err
	}
	return Scalar(s.NewInfixNode(leftNode, n.Operator(), rightNode, n.Associativity())), nil
}
