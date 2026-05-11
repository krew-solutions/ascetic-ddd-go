package specification

import (
	"errors"
	"fmt"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

var (
	ErrCompositeExpressionsDifferentLength = errors.New("composite expressions have different length")
)

// Context maps domain field paths and values to infrastructure AST nodes.
type Context interface {
	AttrNode(path []string) (s.Visitable, error)
	ValueNode(val any) (s.Visitable, error)
	// TODO: с вложенными контекстами ValueNode не будет работать, т.к. ValueNode может идти первым операндом. Нужно разделять интерфейсы.
}

// TransformVisitor transforms a domain specification AST to an infrastructure
// specification AST. Stateless: each visit method returns a new Visitable
// rather than mutating accumulated state.
type TransformVisitor struct {
	context Context
}

func NewTransformVisitor(context Context) *TransformVisitor {
	return &TransformVisitor{context: context}
}

// Transform is the typed entry point.
func (v *TransformVisitor) Transform(node s.Visitable) (s.Visitable, error) {
	return s.Accept[s.Visitable](node, v)
}

func (v *TransformVisitor) VisitGlobalScope(n s.GlobalScopeNode) (s.Visitable, error) {
	return n, nil
}

func (v *TransformVisitor) VisitObject(n s.ObjectNode) (s.Visitable, error) {
	return n, nil
}

func (v *TransformVisitor) VisitCollection(n s.CollectionNode) (s.Visitable, error) {
	return n, nil
}

func (v *TransformVisitor) VisitItem(n s.ItemNode) (s.Visitable, error) {
	return n, nil
}

func (v *TransformVisitor) VisitField(n s.FieldNode) (s.Visitable, error) {
	return v.context.AttrNode(s.ExtractFieldPath(n))
}

func (v *TransformVisitor) VisitValue(n s.ValueNode) (s.Visitable, error) {
	return v.context.ValueNode(n.Value())
}

func (v *TransformVisitor) VisitPrefix(n s.PrefixNode) (s.Visitable, error) {
	operand, err := s.Accept[s.Visitable](n.Operand(), v)
	if err != nil {
		return nil, err
	}
	return s.NewPrefixNode(n.Operator(), operand, n.Associativity()), nil
}

func (v *TransformVisitor) VisitPostfix(n s.PostfixNode) (s.Visitable, error) {
	operand, err := s.Accept[s.Visitable](n.Operand(), v)
	if err != nil {
		return nil, err
	}
	return s.NewPostfixNode(operand, n.Operator(), n.Associativity()), nil
}

func (v *TransformVisitor) VisitInfix(n s.InfixNode) (s.Visitable, error) {
	left, err := s.Accept[s.Visitable](n.Left(), v)
	if err != nil {
		return nil, err
	}
	right, err := s.Accept[s.Visitable](n.Right(), v)
	if err != nil {
		return nil, err
	}

	if leftComposite, ok := left.(CompositeExpressionNode); ok {
		rightComposite, ok := right.(CompositeExpressionNode)
		if !ok {
			return nil, errors.New("not enough composite expressions")
		}
		switch n.Operator() {
		case operators.OperatorEq:
			return leftComposite.Equal(rightComposite)
		case operators.OperatorNe:
			return leftComposite.NotEqual(rightComposite)
		default:
			return nil, fmt.Errorf("operator %q is not supported for composite expressions", n.Operator())
		}
	}
	return s.NewInfixNode(left, n.Operator(), right, n.Associativity()), nil
}
