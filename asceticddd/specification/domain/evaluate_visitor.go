package specification

import (
	"errors"
	"fmt"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

var ErrKeyNotFound = errors.New("key not found")

// Context retrieves values by key during evaluation.
type Context interface {
	Get(string) (any, error)
}

// EvaluateVisitor evaluates a specification AST against a Context.
//
// Fields are immutable per call; collection iteration creates a sub-visitor
// via withItem() rather than mutating in place.
type EvaluateVisitor struct {
	context     Context
	currentItem Context
	registry    *operators.OperatorRegistry
}

func NewEvaluateVisitor(context Context, registry *operators.OperatorRegistry) *EvaluateVisitor {
	return &EvaluateVisitor{
		context:  context,
		registry: registry,
	}
}

// withItem returns a sub-visitor bound to a new current item.
// Used during wildcard iteration to scope @ to the current collection element.
func (v *EvaluateVisitor) withItem(item Context) *EvaluateVisitor {
	return &EvaluateVisitor{
		context:     v.context,
		currentItem: item,
		registry:    v.registry,
	}
}

// Evaluate is the typed entry point. It runs the visitor and asserts the
// final result to bool — top-level specifications must yield a boolean.
func (v *EvaluateVisitor) Evaluate(node Visitable) (bool, error) {
	result, err := Accept[any](node, v)
	if err != nil {
		return false, err
	}
	b, ok := result.(bool)
	if !ok {
		return false, errors.New("the result is not a bool")
	}
	return b, nil
}

func (v *EvaluateVisitor) VisitGlobalScope(_ GlobalScopeNode) (any, error) {
	return v.context, nil
}

func (v *EvaluateVisitor) VisitObject(n ObjectNode) (any, error) {
	parentCtx, err := Accept[any](n.Parent(), v)
	if err != nil {
		return nil, err
	}
	obj, err := parentCtx.(Context).Get(n.Name())
	if err != nil {
		return nil, err
	}
	ctx, ok := obj.(Context)
	if !ok {
		return nil, fmt.Errorf("object %s is not a Context", n.Name())
	}
	return ctx, nil
}

func (v *EvaluateVisitor) VisitItem(_ ItemNode) (any, error) {
	if v.currentItem == nil {
		return nil, errors.New("no current item in context")
	}
	return v.currentItem, nil
}

func (v *EvaluateVisitor) VisitField(n FieldNode) (any, error) {
	objCtx, err := Accept[any](n.Object(), v)
	if err != nil {
		return nil, err
	}
	return objCtx.(Context).Get(n.Name())
}

func (v *EvaluateVisitor) VisitValue(n ValueNode) (any, error) {
	return n.Value(), nil
}

func (v *EvaluateVisitor) VisitCollection(n CollectionNode) (any, error) {
	parentCtx, err := Accept[any](n.Parent(), v)
	if err != nil {
		return nil, err
	}
	items, err := parentCtx.(Context).Get(n.Name())
	if err != nil {
		return nil, err
	}
	itemsTyped, ok := items.([]Context)
	if !ok {
		return nil, errors.New("value is not a collection of Contexts")
	}
	result := false
	for i := range itemsTyped {
		value, err := Accept[any](n.Predicate(), v.withItem(itemsTyped[i]))
		if err != nil {
			return nil, err
		}
		b, ok := value.(bool)
		if !ok {
			return nil, errors.New("predicate did not yield a boolean")
		}
		result = result || b
	}
	return result, nil
}

func (v *EvaluateVisitor) VisitPrefix(n PrefixNode) (any, error) {
	operand, err := Accept[any](n.Operand(), v)
	if err != nil {
		return nil, err
	}
	return v.registry.ExecUnary(n.Operator(), operand)
}

func (v *EvaluateVisitor) VisitPostfix(n PostfixNode) (any, error) {
	operand, err := Accept[any](n.Operand(), v)
	if err != nil {
		return nil, err
	}
	return v.registry.ExecUnary(n.Operator(), operand)
}

func (v *EvaluateVisitor) VisitInfix(n InfixNode) (any, error) {
	left, err := Accept[any](n.Left(), v)
	if err != nil {
		return nil, err
	}
	right, err := Accept[any](n.Right(), v)
	if err != nil {
		return nil, err
	}
	return v.registry.ExecBinary(left, n.Operator(), right)
}

func ExtractFieldPath(n FieldNode) []string {
	path := []string{n.Name()}
	var obj EmptiableObject = n.Object()
	for !obj.IsRoot() {
		path = append([]string{obj.Name()}, path...)
		obj = obj.Parent()
	}
	return path
}

type CollectionContext struct {
	items []Context
}

func NewCollectionContext(items []Context) CollectionContext {
	return CollectionContext{items: items}
}

func (c CollectionContext) Get(slice string) (any, error) {
	if slice == "*" {
		return c.items, nil
	}
	return nil, fmt.Errorf("unsupported slice type \"%s\"", slice)
}
