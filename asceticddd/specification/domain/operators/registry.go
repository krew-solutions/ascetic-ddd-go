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

	// IS is the equality in which NULL is a value: never NULL itself.
	if op == OperatorIs {
		return r.execIs(left, right)
	}

	// NULL propagation for all other binary operators
	if IsNull(left) || IsNull(right) {
		return nil, nil
	}

	fn, left, right, err := r.lookupBinary(left, op, right)
	if err != nil {
		return nil, err
	}
	return fn(left, right)
}

// execIs is IS NOT DISTINCT FROM: what is registered for IS, or else for =.
func (r *OperatorRegistry) execIs(left, right any) (any, error) {
	if IsNull(left) || IsNull(right) {
		return IsNull(left) && IsNull(right), nil
	}
	fn, l, rt, err := r.lookupBinary(left, OperatorIs, right)
	if err != nil {
		fn, l, rt, err = r.lookupBinary(left, OperatorEq, right)
	}
	if err != nil {
		return nil, fmt.Errorf("operator \"%s\" is not supported for %T and %T", OperatorIs, left, right)
	}
	return fn(l, rt)
}

// ExecUnary executes a unary operator with PostgreSQL NULL semantics.
func (r *OperatorRegistry) ExecUnary(op Operator, operand any) (any, error) {
	// IS NULL / IS NOT NULL — definite result for any value including NULL
	if op == OperatorIsNull {
		return IsNull(operand), nil
	}
	if op == OperatorIsNotNull {
		return !IsNull(operand), nil
	}

	// NULL propagation
	if IsNull(operand) {
		return nil, nil
	}

	fn, operand, err := r.lookupUnary(op, operand)
	if err != nil {
		return nil, err
	}
	return fn(operand)
}

// lookupBinary finds the operator of the operands as they are, and returns the
// operands to call it with.
//
// A pointer that is a value of its own - a Value Object with pointer
// receivers, a type an operator is registered for - is found as it is. Any
// other is an optional value, and is read through: the driver sends a pointer
// as what it points at, and the evaluator used to refuse it.
func (r *OperatorRegistry) lookupBinary(left any, op Operator, right any) (BinaryOp, any, any, error) {
	if fn := r.findBinary(left, op, right); fn != nil {
		return fn, left, right, nil
	}
	if l, rt := Indirect(left), Indirect(right); l != left || rt != right {
		if fn := r.findBinary(l, op, rt); fn != nil {
			return fn, l, rt, nil
		}
	}
	return nil, nil, nil, fmt.Errorf("operator \"%s\" is not supported for %T and %T", op, left, right)
}

func (r *OperatorRegistry) findBinary(left any, op Operator, right any) BinaryOp {
	key := binaryKey{
		left:  reflect.TypeOf(left),
		op:    op,
		right: reflect.TypeOf(right),
	}
	if fn, ok := r.binary[key]; ok {
		return fn
	}

	// Fallback: check if operands implement Value Object interfaces
	return interfaceFallback(left, op, right)
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

// lookupUnary finds the operator of the operand as it is, or else of what it
// points at, and returns the operand to call it with.
func (r *OperatorRegistry) lookupUnary(op Operator, operand any) (UnaryOp, any, error) {
	for _, candidate := range []any{operand, Indirect(operand)} {
		key := unaryKey{
			op:      op,
			operand: reflect.TypeOf(candidate),
		}
		if fn, ok := r.unary[key]; ok {
			return fn, candidate, nil
		}
	}
	return nil, nil, fmt.Errorf("operator \"%s\" is not supported for %T", op, operand)
}

// Three-valued logic: NULL AND FALSE = FALSE, NULL AND TRUE = NULL
func execAnd(left, right any) (any, error) {
	l, leftNull, ok := Truth(left)
	if !ok {
		return nil, fmt.Errorf("operator \"AND\" requires bool, got %T", left)
	}
	r, rightNull, ok := Truth(right)
	if !ok {
		return nil, fmt.Errorf("operator \"AND\" requires bool, got %T", right)
	}
	switch {
	case (!leftNull && !l) || (!rightNull && !r):
		return false, nil
	case leftNull || rightNull:
		return nil, nil
	default:
		return true, nil
	}
}

// Three-valued logic: NULL OR TRUE = TRUE, NULL OR FALSE = NULL
func execOr(left, right any) (any, error) {
	l, leftNull, ok := Truth(left)
	if !ok {
		return nil, fmt.Errorf("operator \"OR\" requires bool, got %T", left)
	}
	r, rightNull, ok := Truth(right)
	if !ok {
		return nil, fmt.Errorf("operator \"OR\" requires bool, got %T", right)
	}
	switch {
	case (!leftNull && l) || (!rightNull && r):
		return true, nil
	case leftNull || rightNull:
		return nil, nil
	default:
		return false, nil
	}
}
