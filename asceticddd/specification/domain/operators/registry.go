package operators

import (
	"fmt"
	"reflect"
)

type BinaryOp func(left, right any) (any, error)
type UnaryOp func(operand any) (any, error)

type binaryKey struct {
	left  reflect.Type
	op    Operator
	right reflect.Type
}

type unaryKey struct {
	op      Operator
	operand reflect.Type
}

type OperatorRegistry struct {
	binary map[binaryKey]BinaryOp
	unary  map[unaryKey]UnaryOp
}

func NewOperatorRegistry() *OperatorRegistry {
	return &OperatorRegistry{
		binary: make(map[binaryKey]BinaryOp),
		unary:  make(map[unaryKey]UnaryOp),
	}
}

func RegisterBinary[L, R any](reg *OperatorRegistry, op Operator, fn func(L, R) (any, error)) {
	var zeroL L
	var zeroR R
	key := binaryKey{
		left:  reflect.TypeOf(zeroL),
		op:    op,
		right: reflect.TypeOf(zeroR),
	}
	reg.binary[key] = func(left, right any) (any, error) {
		return fn(left.(L), right.(R))
	}
}

func RegisterUnary[T any](reg *OperatorRegistry, op Operator, fn func(T) (any, error)) {
	var zero T
	key := unaryKey{
		op:      op,
		operand: reflect.TypeOf(zero),
	}
	reg.unary[key] = func(operand any) (any, error) {
		return fn(operand.(T))
	}
}

// ExecBinary executes a binary operator with PostgreSQL NULL semantics.
func (r *OperatorRegistry) ExecBinary(left any, op Operator, right any) (any, error) {
	// Three-valued logic for AND/OR
	if op == OperatorAnd {
		return execAnd(left, right)
	}
	if op == OperatorOr {
		return execOr(left, right)
	}

	// NULL propagation for all other binary operators
	if left == nil || right == nil {
		return nil, nil
	}

	fn, err := r.lookupBinary(left, op, right)
	if err != nil {
		return nil, err
	}
	return fn(left, right)
}

// ExecUnary executes a unary operator with PostgreSQL NULL semantics.
func (r *OperatorRegistry) ExecUnary(op Operator, operand any) (any, error) {
	// IS NULL / IS NOT NULL — definite result for any value including NULL
	if op == OperatorIsNull {
		return operand == nil, nil
	}
	if op == OperatorIsNotNull {
		return operand != nil, nil
	}

	// NULL propagation
	if operand == nil {
		return nil, nil
	}

	fn, err := r.lookupUnary(op, operand)
	if err != nil {
		return nil, err
	}
	return fn(operand)
}

func (r *OperatorRegistry) lookupBinary(left any, op Operator, right any) (BinaryOp, error) {
	key := binaryKey{
		left:  reflect.TypeOf(left),
		op:    op,
		right: reflect.TypeOf(right),
	}
	fn, ok := r.binary[key]
	if ok {
		return fn, nil
	}

	// Fallback: check if operands implement Value Object interfaces
	if fallback := interfaceFallback(left, op, right); fallback != nil {
		return fallback, nil
	}

	return nil, fmt.Errorf("operator \"%s\" is not supported for %T and %T", op, left, right)
}

func interfaceFallback(left any, op Operator, right any) BinaryOp {
	switch op {
	case OperatorEq:
		if _, ok := left.(EqualOperand); ok {
			return func(left, right any) (any, error) {
				l, ok := left.(EqualOperand)
				if !ok {
					return nil, fmt.Errorf("left operand %T does not implement EqualOperand", left)
				}
				r, ok := right.(EqualOperand)
				if !ok {
					return nil, fmt.Errorf("right operand %T does not implement EqualOperand", right)
				}
				return l.Equal(r), nil
			}
		}
	case OperatorNe:
		if _, ok := left.(EqualOperand); ok {
			return func(left, right any) (any, error) {
				l, ok := left.(EqualOperand)
				if !ok {
					return nil, fmt.Errorf("left operand %T does not implement EqualOperand", left)
				}
				r, ok := right.(EqualOperand)
				if !ok {
					return nil, fmt.Errorf("right operand %T does not implement EqualOperand", right)
				}
				return !l.Equal(r), nil
			}
		}
	case OperatorGt:
		if _, ok := left.(GreaterThanOperand); ok {
			return func(left, right any) (any, error) {
				l, ok := left.(GreaterThanOperand)
				if !ok {
					return nil, fmt.Errorf("left operand %T does not implement GreaterThanOperand", left)
				}
				r, ok := right.(GreaterThanOperand)
				if !ok {
					return nil, fmt.Errorf("right operand %T does not implement GreaterThanOperand", right)
				}
				return l.GreaterThan(r), nil
			}
		}
	case OperatorGte:
		if _, ok := left.(GreaterThanEqualOperand); ok {
			return func(left, right any) (any, error) {
				l, ok := left.(GreaterThanEqualOperand)
				if !ok {
					return nil, fmt.Errorf("left operand %T does not implement GreaterThanEqualOperand", left)
				}
				r, ok := right.(GreaterThanEqualOperand)
				if !ok {
					return nil, fmt.Errorf("right operand %T does not implement GreaterThanEqualOperand", right)
				}
				return l.GreaterThanEqual(r), nil
			}
		}
	case OperatorLt:
		if _, ok := left.(LessThanOperand); ok {
			return func(left, right any) (any, error) {
				l, ok := left.(LessThanOperand)
				if !ok {
					return nil, fmt.Errorf("left operand %T does not implement LessThanOperand", left)
				}
				r, ok := right.(LessThanOperand)
				if !ok {
					return nil, fmt.Errorf("right operand %T does not implement LessThanOperand", right)
				}
				return l.LessThan(r), nil
			}
		}
	case OperatorLte:
		if _, ok := left.(LessThanEqualOperand); ok {
			return func(left, right any) (any, error) {
				l, ok := left.(LessThanEqualOperand)
				if !ok {
					return nil, fmt.Errorf("left operand %T does not implement LessThanEqualOperand", left)
				}
				r, ok := right.(LessThanEqualOperand)
				if !ok {
					return nil, fmt.Errorf("right operand %T does not implement LessThanEqualOperand", right)
				}
				return l.LessThanEqual(r), nil
			}
		}
	}
	return nil
}

func (r *OperatorRegistry) lookupUnary(op Operator, operand any) (UnaryOp, error) {
	key := unaryKey{
		op:      op,
		operand: reflect.TypeOf(operand),
	}
	fn, ok := r.unary[key]
	if !ok {
		return nil, fmt.Errorf("operator \"%s\" is not supported for %T", op, operand)
	}
	return fn, nil
}

// Three-valued logic: NULL AND FALSE = FALSE, NULL AND TRUE = NULL
func execAnd(left, right any) (any, error) {
	if left == nil {
		if val, ok := right.(bool); ok && !val {
			return false, nil
		}
		return nil, nil
	}
	if right == nil {
		if val, ok := left.(bool); ok && !val {
			return false, nil
		}
		return nil, nil
	}
	l, ok := left.(bool)
	if !ok {
		return nil, fmt.Errorf("operator \"AND\" requires bool, got %T", left)
	}
	r, ok := right.(bool)
	if !ok {
		return nil, fmt.Errorf("operator \"AND\" requires bool, got %T", right)
	}
	return l && r, nil
}

// Three-valued logic: NULL OR TRUE = TRUE, NULL OR FALSE = NULL
func execOr(left, right any) (any, error) {
	if left == nil {
		if val, ok := right.(bool); ok && val {
			return true, nil
		}
		return nil, nil
	}
	if right == nil {
		if val, ok := left.(bool); ok && val {
			return true, nil
		}
		return nil, nil
	}
	l, ok := left.(bool)
	if !ok {
		return nil, fmt.Errorf("operator \"OR\" requires bool, got %T", left)
	}
	r, ok := right.(bool)
	if !ok {
		return nil, fmt.Errorf("operator \"OR\" requires bool, got %T", right)
	}
	return l || r, nil
}
