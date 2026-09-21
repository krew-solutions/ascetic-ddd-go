package specification

import (
	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

// Mapped is what a field or a value of the domain is in the storage: one
// expression, Scalar, or a composite of several, CompositeExpression.
//
// A composite used to be a Visitable that no visitor knew. One left in the
// tree - beside a node, under an operator other than = and != - failed later,
// inside whatever read the tree next. It is not a node: a context returns a
// Mapped, the transformer returns a Visitable, and a composite left over is
// the transformer's error.
type Mapped interface {
	mapped()
}

// Scalar is one expression.
func Scalar(node s.Visitable) ScalarExpression {
	return ScalarExpression{node: node}
}

type ScalarExpression struct {
	node s.Visitable
}

func (n ScalarExpression) Node() s.Visitable {
	return n.node
}

func (ScalarExpression) mapped() {}

type ExpressionComposer interface {
	Equal(other CompositeExpressionNode) (s.Visitable, error)
	NotEqual(other CompositeExpressionNode) (s.Visitable, error)
	Mapped
}

func CompositeExpression(nodes ...Mapped) CompositeExpressionNode {
	return CompositeExpressionNode{
		nodes: nodes,
	}
}

type CompositeExpressionNode struct {
	nodes []Mapped
}

func (CompositeExpressionNode) mapped() {}

// Equal is the conjunction of the equalities of the parts.
func (n CompositeExpressionNode) Equal(other CompositeExpressionNode) (s.Visitable, error) {
	if len(n.nodes) != len(other.nodes) {
		return nil, ErrCompositeExpressionsDifferentLength
	}
	if len(n.nodes) == 0 {
		return nil, ErrCompositeExpressionIsEmpty
	}
	operands := make([]s.Visitable, 0, len(n.nodes))
	for i := range n.nodes {
		operand, err := equalParts(n.nodes[i], other.nodes[i])
		if err != nil {
			return nil, err
		}
		operands = append(operands, operand)
	}
	// A composite of one part is that part: And takes two operands and more,
	// and used to panic on one.
	if len(operands) == 1 {
		return operands[0], nil
	}
	return s.And(operands[0], operands[1:]...), nil
}

// equalParts compares a part with a part of the same shape. A composite on
// the right alone used to be compared as if it were a node.
func equalParts(left, right Mapped) (s.Visitable, error) {
	switch l := left.(type) {
	case ScalarExpression:
		if r, ok := right.(ScalarExpression); ok {
			// A part the mapping made the storage's null is tested for, as a
			// whole is: `b = $1` with a null is true of nothing.
			return s.EqualityOrNullTest(operators.OperatorEq, l.Node(), r.Node()), nil
		}
	case CompositeExpressionNode:
		if r, ok := right.(CompositeExpressionNode); ok {
			return l.Equal(r)
		}
	}
	return nil, ErrCompositeExpressionsDifferentLength
}

// NotEqual is "not equal in every part", which is not "unequal in every part":
// (1, 2) and (1, 3) differ. It used to be built of NotEqual parts,
// NOT (a1 != b1 AND a2 != b2), by which a composite was unequal to itself and
// equal to one it shares no part with.
func (n CompositeExpressionNode) NotEqual(other CompositeExpressionNode) (s.Visitable, error) {
	equal, err := n.Equal(other)
	if err != nil {
		return nil, err
	}
	return s.Not(equal), nil
}
