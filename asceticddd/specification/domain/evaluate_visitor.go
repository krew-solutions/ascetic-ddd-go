package specification

import (
	"errors"
	"fmt"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

var ErrKeyNotFound = errors.New("key not found")

func NewEvaluateVisitor(context Context, registry *operators.OperatorRegistry) *EvaluateVisitor {
	return &EvaluateVisitor{
		Context:  context,
		registry: registry,
	}
}

type EvaluateVisitor struct {
	currentValue any
	currentItem  Context
	stack        []Context
	registry     *operators.OperatorRegistry
	Context
}

func (v *EvaluateVisitor) push(ctx Context) {
	v.stack = append(v.stack, v.Context)
	v.Context = ctx
}

func (v *EvaluateVisitor) pop() {
	v.Context = v.stack[len(v.stack)-1]
	v.stack = v.stack[:len(v.stack)-1]
}

func (v EvaluateVisitor) CurrentValue() any {
	return v.currentValue
}

func (v *EvaluateVisitor) SetCurrentValue(val any) {
	v.currentValue = val
}

func (v *EvaluateVisitor) VisitGlobalScope(n GlobalScopeNode) error {
	v.push(v.Context)
	return nil
}

func (v *EvaluateVisitor) VisitObject(n ObjectNode) error {
	err := n.Parent().Accept(v)
	if err != nil {
		return err
	}
	obj, err := v.Context.Get(n.Name())
	v.pop()
	if err != nil {
		return err
	}
	v.push(obj.(Context))
	return nil
}

func (v *EvaluateVisitor) VisitCollection(n CollectionNode) error {
	err := n.Parent().Accept(v)
	if err != nil {
		return err
	}
	items, err := v.Context.Get(n.Name())
	v.pop()
	if err != nil {
		return err
	}
	itemsTyped, ok := items.([]Context)
	if !ok {
		return errors.New("currentValue is not a collection of Contexts")
	}
	result := false
	for i := range itemsTyped {
		v.currentItem = itemsTyped[i]
		err := n.Predicate().Accept(v)
		if err != nil {
			return err
		}
		result = result || v.CurrentValue().(bool)
	}
	v.SetCurrentValue(result)
	return nil
}

func (v *EvaluateVisitor) VisitItem(n ItemNode) error {
	v.push(v.currentItem)
	return nil
}

func (v *EvaluateVisitor) VisitField(n FieldNode) error {
	err := n.Object().Accept(v)
	if err != nil {
		return err
	}
	value, err := v.Context.Get(n.Name())
	v.pop()
	if err != nil {
		return err
	}
	v.SetCurrentValue(value)
	return nil
}

func (v *EvaluateVisitor) VisitValue(n ValueNode) error {
	v.SetCurrentValue(n.Value())
	return nil
}

func (v *EvaluateVisitor) VisitPrefix(n PrefixNode) error {
	err := n.Operand().Accept(v)
	if err != nil {
		return err
	}
	result, err := v.registry.ExecUnary(n.Operator(), v.CurrentValue())
	if err != nil {
		return err
	}
	v.SetCurrentValue(result)
	return nil
}

func (v *EvaluateVisitor) VisitPostfix(n PostfixNode) error {
	err := n.Operand().Accept(v)
	if err != nil {
		return err
	}
	result, err := v.registry.ExecUnary(n.Operator(), v.CurrentValue())
	if err != nil {
		return err
	}
	v.SetCurrentValue(result)
	return nil
}

func (v *EvaluateVisitor) VisitInfix(n InfixNode) error {
	err := n.Left().Accept(v)
	if err != nil {
		return err
	}
	left := v.CurrentValue()
	err = n.Right().Accept(v)
	if err != nil {
		return err
	}
	right := v.CurrentValue()
	result, err := v.registry.ExecBinary(left, n.Operator(), right)
	if err != nil {
		return err
	}
	v.SetCurrentValue(result)
	return nil
}

func (v EvaluateVisitor) Result() (bool, error) {
	result := v.CurrentValue()
	resultTyped, ok := result.(bool)
	if !ok {
		return false, errors.New("the result is not a bool")
	}
	return resultTyped, nil
}

type Context interface {
	Get(string) (any, error)
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
