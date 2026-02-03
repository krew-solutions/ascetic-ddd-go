package specification

import (
	"cmp"
	"errors"
	"fmt"
)

var ErrKeyNotFound = errors.New("key not found")

func NewEvaluateVisitor(context Context) *EvaluateVisitor {
	return &EvaluateVisitor{
		Context: context,
	}
}

type EvaluateVisitor struct {
	currentValue any
	currentItem  Context
	stack        []Context
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
	operand := v.CurrentValue()
	if v.yieldBooleanOperator(n.Operator()) {
		result, err := v.evalYieldBooleanPrefix(operand, n.Operator())
		if err != nil {
			return err
		}
		v.SetCurrentValue(result)
	} else {
		return fmt.Errorf("mathematical operator \"%s\" is not supported", n.Operator())
	}
	return nil
}
func (v EvaluateVisitor) evalYieldBooleanPrefix(operand any, op Operator) (bool, error) {
	switch op {
	case OperatorNot:
		return v.evalNot(operand)
	default:
		return false, fmt.Errorf("operator \"%s\" is not supported", op)
	}
}

func (v EvaluateVisitor) evalNot(operand any) (bool, error) {
	operandTyped, ok := operand.(bool)
	if !ok {
		return false, errors.New("operand is not a bool")
	}
	return !operandTyped, nil
}

func (v *EvaluateVisitor) VisitPostfix(n PostfixNode) error {
	err := n.Operand().Accept(v)
	if err != nil {
		return err
	}
	operand := v.CurrentValue()
	if v.yieldBooleanOperator(n.Operator()) {
		result, err := v.evalYieldBooleanPostfix(operand, n.Operator())
		if err != nil {
			return err
		}
		v.SetCurrentValue(result)
	} else {
		return fmt.Errorf("operator \"%s\" is not supported for postfix", n.Operator())
	}
	return nil
}

func (v EvaluateVisitor) evalYieldBooleanPostfix(operand any, op Operator) (bool, error) {
	switch op {
	case OperatorIsNull:
		return v.evalIsNull(operand)
	case OperatorIsNotNull:
		return v.evalIsNotNull(operand)
	default:
		return false, fmt.Errorf("operator \"%s\" is not supported for postfix", op)
	}
}

func (v EvaluateVisitor) evalIsNull(operand any) (bool, error) {
	return operand == nil, nil
}

func (v EvaluateVisitor) evalIsNotNull(operand any) (bool, error) {
	return operand != nil, nil
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
	if v.yieldBooleanOperator(n.Operator()) {
		result, err := v.evalYieldBooleanInfix(left, n.Operator(), right)
		if err != nil {
			return err
		}
		v.SetCurrentValue(result)
	} else {
		return fmt.Errorf("mathematical operator \"%s\" is not supported", n.Operator())
	}
	return nil
}

func (v EvaluateVisitor) yieldBooleanOperator(op Operator) bool {
	for i := range YieldBooleanOperators {
		if YieldBooleanOperators[i] == op {
			return true
		}
	}
	return false
}

func (v EvaluateVisitor) evalYieldBooleanInfix(left any, op Operator, right any) (bool, error) {
	switch op {
	case OperatorEq:
		return v.evalEq(left, right)
	case OperatorNe:
		return v.evalNe(left, right)
	case OperatorGt:
		return v.evalGt(left, right)
	case OperatorGte:
		return v.evalGte(left, right)
	case OperatorLt:
		return v.evalLt(left, right)
	case OperatorLte:
		return v.evalLte(left, right)
	case OperatorAnd:
		return v.evalAnd(left, right)
	case OperatorOr:
		return v.evalOr(left, right)
	default:
		return false, fmt.Errorf("operator \"%s\" is not supported", op)
	}
}

func (v EvaluateVisitor) evalEq(left, right any) (bool, error) {
	// Try EqualOperand interface first
	leftTyped, ok := left.(EqualOperand)
	if ok {
		rightTyped, ok := right.(EqualOperand)
		if !ok {
			return false, errors.New("right operand is not an EqualOperand")
		}
		return leftTyped.Equal(rightTyped), nil
	}

	// Fallback to simple comparison for basic types
	return left == right, nil
}

func (v EvaluateVisitor) evalNe(left, right any) (bool, error) {
	// Try EqualOperand interface first
	leftTyped, ok := left.(EqualOperand)
	if ok {
		rightTyped, ok := right.(EqualOperand)
		if !ok {
			return false, errors.New("right operand is not an EqualOperand")
		}
		return !leftTyped.Equal(rightTyped), nil
	}

	// Fallback to simple comparison for basic types
	return left != right, nil
}

func (v EvaluateVisitor) evalGt(left, right any) (bool, error) {
	// Try GreaterThanOperand interface first
	leftTyped, ok := left.(GreaterThanOperand)
	if ok {
		rightTyped, ok := right.(GreaterThanOperand)
		if !ok {
			return false, errors.New("right operand is not a GreaterThanOperand")
		}
		return leftTyped.GreaterThan(rightTyped), nil
	}

	// Fallback to cmp.Ordered for basic numeric types
	return compareOrdered(left, right, func(c int) bool { return c > 0 })
}

func (v EvaluateVisitor) evalGte(left, right any) (bool, error) {
	// Try GreaterThanEqualOperand interface first
	leftTyped, ok := left.(GreaterThanEqualOperand)
	if ok {
		rightTyped, ok := right.(GreaterThanEqualOperand)
		if !ok {
			return false, errors.New("right operand is not a GreaterThanEqualOperand")
		}
		return leftTyped.GreaterThanEqual(rightTyped), nil
	}

	// Fallback to cmp.Ordered for basic numeric types
	return compareOrdered(left, right, func(c int) bool { return c >= 0 })
}

// compareOrdered compares two values if they are cmp.Ordered types
func compareOrdered(left, right any, predicate func(int) bool) (bool, error) {
	switch l := left.(type) {
	case int:
		r, ok := right.(int)
		if !ok {
			return false, errors.New("operands are not comparable")
		}
		return predicate(cmp.Compare(l, r)), nil
	case int64:
		r, ok := right.(int64)
		if !ok {
			return false, errors.New("operands are not comparable")
		}
		return predicate(cmp.Compare(l, r)), nil
	case float64:
		r, ok := right.(float64)
		if !ok {
			return false, errors.New("operands are not comparable")
		}
		return predicate(cmp.Compare(l, r)), nil
	case string:
		r, ok := right.(string)
		if !ok {
			return false, errors.New("operands are not comparable")
		}
		return predicate(cmp.Compare(l, r)), nil
	default:
		return false, fmt.Errorf("type %T is not comparable", left)
	}
}

func (v EvaluateVisitor) evalLt(left, right any) (bool, error) {
	// Try LessThanOperand interface first
	leftTyped, ok := left.(LessThanOperand)
	if ok {
		rightTyped, ok := right.(LessThanOperand)
		if !ok {
			return false, errors.New("right operand is not a LessThanOperand")
		}
		return leftTyped.LessThan(rightTyped), nil
	}

	// Fallback to cmp.Ordered for basic numeric types
	return compareOrdered(left, right, func(c int) bool { return c < 0 })
}

func (v EvaluateVisitor) evalLte(left, right any) (bool, error) {
	// Try LessThanEqualOperand interface first
	leftTyped, ok := left.(LessThanEqualOperand)
	if ok {
		rightTyped, ok := right.(LessThanEqualOperand)
		if !ok {
			return false, errors.New("right operand is not a LessThanEqualOperand")
		}
		return leftTyped.LessThanEqual(rightTyped), nil
	}

	// Fallback to cmp.Ordered for basic numeric types
	return compareOrdered(left, right, func(c int) bool { return c <= 0 })
}

func (v EvaluateVisitor) evalAnd(left, right any) (bool, error) {
	leftTyped, ok := left.(bool)
	if !ok {
		return false, errors.New("left operand is not a bool")
	}
	rightTyped, ok := right.(bool)
	if !ok {
		return false, errors.New("right operand is not a bool")
	}
	return leftTyped && rightTyped, nil
}

func (v EvaluateVisitor) evalOr(left, right any) (bool, error) {
	leftTyped, ok := left.(bool)
	if !ok {
		return false, errors.New("left operand is not a bool")
	}
	rightTyped, ok := right.(bool)
	if !ok {
		return false, errors.New("right operand is not a bool")
	}
	return leftTyped || rightTyped, nil
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
