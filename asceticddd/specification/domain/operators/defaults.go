package operators

import (
	"cmp"
	"errors"
	"math"
	"time"
)

func registerComparison[T cmp.Ordered](reg *OperatorRegistry) {
	RegisterBinary[T, T](reg, OperatorEq, func(a, b T) (any, error) { return a == b, nil })
	RegisterBinary[T, T](reg, OperatorNe, func(a, b T) (any, error) { return a != b, nil })
	RegisterBinary[T, T](reg, OperatorGt, func(a, b T) (any, error) { return a > b, nil })
	RegisterBinary[T, T](reg, OperatorGte, func(a, b T) (any, error) { return a >= b, nil })
	RegisterBinary[T, T](reg, OperatorLt, func(a, b T) (any, error) { return a < b, nil })
	RegisterBinary[T, T](reg, OperatorLte, func(a, b T) (any, error) { return a <= b, nil })
}

// The arithmetic below is PostgreSQL's where Go's own differs: an integer is a
// bigint and does not wrap, a shift takes its count modulo 64, a float does
// not overflow to an infinity, a NaN equals itself and is the greatest of
// floats, false is less than true. The evaluator and the query of the same
// tree must agree.

var (
	errIntegerOutOfRange = errors.New("bigint out of range")
	errFloatOutOfRange   = errors.New("value out of range: overflow")
	errFloatUnderflow    = errors.New("value out of range: underflow")
	errDivisionByZero    = errors.New("division by zero")
)

type integer interface{ ~int | ~int64 }

// fits returns a result of bigint arithmetic as the type of its operands.
func fits[T integer](value int64, err error) (any, error) {
	if err != nil {
		return nil, err
	}
	if int64(T(value)) != value {
		return nil, errIntegerOutOfRange
	}
	return T(value), nil
}

func addInt64(a, b int64) (int64, error) {
	c := a + b
	if (c > a) != (b > 0) {
		return 0, errIntegerOutOfRange
	}
	return c, nil
}

func subInt64(a, b int64) (int64, error) {
	c := a - b
	if (c < a) != (b > 0) {
		return 0, errIntegerOutOfRange
	}
	return c, nil
}

func mulInt64(a, b int64) (int64, error) {
	if a == 0 || b == 0 {
		return 0, nil
	}
	c := a * b
	if (c < 0) != ((a < 0) != (b < 0)) || c/b != a {
		return 0, errIntegerOutOfRange
	}
	return c, nil
}

func divInt64(a, b int64) (int64, error) {
	if b == 0 {
		return 0, errDivisionByZero
	}
	if a == math.MinInt64 && b == -1 {
		return 0, errIntegerOutOfRange
	}
	return a / b, nil
}

func negInt64(a int64) (int64, error) {
	if a == math.MinInt64 {
		return 0, errIntegerOutOfRange
	}
	return -a, nil
}

func registerIntegerArithmetic[T integer](reg *OperatorRegistry) {
	RegisterBinary[T, T](reg, OperatorAdd, func(a, b T) (any, error) { return fits[T](addInt64(int64(a), int64(b))) })
	RegisterBinary[T, T](reg, OperatorSub, func(a, b T) (any, error) { return fits[T](subInt64(int64(a), int64(b))) })
	RegisterBinary[T, T](reg, OperatorMul, func(a, b T) (any, error) { return fits[T](mulInt64(int64(a), int64(b))) })
	RegisterBinary[T, T](reg, OperatorDiv, func(a, b T) (any, error) { return fits[T](divInt64(int64(a), int64(b))) })
	RegisterUnary[T](reg, OperatorNeg, func(a T) (any, error) { return fits[T](negInt64(int64(a))) })
	// The count of a shift is taken modulo 64, a negative one included, and
	// the bits shifted out are dropped. Go panics on a negative count and
	// gives 0 for one of 64 and more.
	RegisterBinary[T, T](reg, OperatorLshift, func(a, b T) (any, error) { return T(int64(a) << (uint64(b) & 63)), nil })
	RegisterBinary[T, T](reg, OperatorRshift, func(a, b T) (any, error) { return T(int64(a) >> (uint64(b) & 63)), nil })
}

// finite returns a result of float arithmetic, which must not be an overflow
// of finite operands.
func finite(value float64, operands ...float64) (any, error) {
	if !math.IsInf(value, 0) {
		return value, nil
	}
	for _, operand := range operands {
		if math.IsInf(operand, 0) {
			return value, nil
		}
	}
	return nil, errFloatOutOfRange
}

// product returns the product of two floats, which must not be an underflow
// either: a zero of operands that are not zero. IEEE arithmetic rounds a
// result too small to be a float to zero in silence, and PostgreSQL has "value
// out of range: underflow" for it, as it has for one too large.
func product(a, b float64) (any, error) {
	value := a * b
	if value == 0 && a != 0 && b != 0 {
		return nil, errFloatUnderflow
	}
	return finite(value, a, b)
}

// quotient returns the quotient of two floats, as PostgreSQL has it. A NaN
// divided by zero is a NaN, as it is divided by anything; any other number
// divided by zero is the error. A zero of a dividend that is not zero is an
// underflow, unless the divisor is infinite: one divided by infinity is a
// zero, and right.
func quotient(a, b float64) (any, error) {
	if b == 0 {
		if math.IsNaN(a) {
			return a, nil
		}
		return nil, errDivisionByZero
	}
	value := a / b
	if value == 0 && a != 0 && !math.IsInf(b, 0) {
		return nil, errFloatUnderflow
	}
	return finite(value, a, b)
}

// compareFloat64 orders floats as PostgreSQL does: a NaN equals a NaN and is
// greater than anything else.
func compareFloat64(a, b float64) int {
	switch {
	case math.IsNaN(a) && math.IsNaN(b):
		return 0
	case math.IsNaN(a):
		return 1
	case math.IsNaN(b):
		return -1
	default:
		return cmp.Compare(a, b)
	}
}

func compareBool(a, b bool) int {
	switch {
	case a == b:
		return 0
	case b:
		return -1
	default:
		return 1
	}
}

func registerOrder[T any](reg *OperatorRegistry, compare func(a, b T) int) {
	registerOrderOf[T, T](reg, compare)
}

func registerFloatArithmetic(reg *OperatorRegistry) {
	RegisterBinary[float64, float64](reg, OperatorAdd, func(a, b float64) (any, error) { return finite(a+b, a, b) })
	RegisterBinary[float64, float64](reg, OperatorSub, func(a, b float64) (any, error) { return finite(a-b, a, b) })
	RegisterBinary[float64, float64](reg, OperatorMul, product)
	RegisterBinary[float64, float64](reg, OperatorDiv, quotient)
	RegisterUnary[float64](reg, OperatorNeg, func(a float64) (any, error) { return -a, nil })
}

// registerIntegerPair registers the operators of two integers of different
// types: PostgreSQL has one bigint, and the result here is an int64.
func registerIntegerPair[L, R integer](reg *OperatorRegistry) {
	RegisterBinary[L, R](reg, OperatorAdd, func(a L, b R) (any, error) { return fits[int64](addInt64(int64(a), int64(b))) })
	RegisterBinary[L, R](reg, OperatorSub, func(a L, b R) (any, error) { return fits[int64](subInt64(int64(a), int64(b))) })
	RegisterBinary[L, R](reg, OperatorMul, func(a L, b R) (any, error) { return fits[int64](mulInt64(int64(a), int64(b))) })
	RegisterBinary[L, R](reg, OperatorDiv, func(a L, b R) (any, error) { return fits[int64](divInt64(int64(a), int64(b))) })
	RegisterBinary[L, R](reg, OperatorMod, func(a L, b R) (any, error) {
		if b == 0 {
			return nil, errors.New("modulo by zero")
		}
		return int64(a) % int64(b), nil
	})
	RegisterBinary[L, R](reg, OperatorLshift, func(a L, b R) (any, error) { return int64(a) << (uint64(b) & 63), nil })
	RegisterBinary[L, R](reg, OperatorRshift, func(a L, b R) (any, error) { return int64(a) >> (uint64(b) & 63), nil })
	registerOrderOf[L, R](reg, func(a L, b R) int { return cmp.Compare(int64(a), int64(b)) })
}

// registerFloatPair registers the operators of a number and a float: the
// integer is taken for a double precision, as PostgreSQL takes it.
func registerFloatPair[L, R interface{ ~int | ~int64 | ~float64 }](reg *OperatorRegistry) {
	RegisterBinary[L, R](reg, OperatorAdd, func(a L, b R) (any, error) {
		return finite(float64(a)+float64(b), float64(a), float64(b))
	})
	RegisterBinary[L, R](reg, OperatorSub, func(a L, b R) (any, error) {
		return finite(float64(a)-float64(b), float64(a), float64(b))
	})
	RegisterBinary[L, R](reg, OperatorMul, func(a L, b R) (any, error) { return product(float64(a), float64(b)) })
	RegisterBinary[L, R](reg, OperatorDiv, func(a L, b R) (any, error) { return quotient(float64(a), float64(b)) })
	registerOrderOf[L, R](reg, func(a L, b R) int { return compareFloat64(float64(a), float64(b)) })
}

func registerOrderOf[L, R any](reg *OperatorRegistry, compare func(a L, b R) int) {
	RegisterBinary[L, R](reg, OperatorEq, func(a L, b R) (any, error) { return compare(a, b) == 0, nil })
	RegisterBinary[L, R](reg, OperatorNe, func(a L, b R) (any, error) { return compare(a, b) != 0, nil })
	RegisterBinary[L, R](reg, OperatorGt, func(a L, b R) (any, error) { return compare(a, b) > 0, nil })
	RegisterBinary[L, R](reg, OperatorGte, func(a L, b R) (any, error) { return compare(a, b) >= 0, nil })
	RegisterBinary[L, R](reg, OperatorLt, func(a L, b R) (any, error) { return compare(a, b) < 0, nil })
	RegisterBinary[L, R](reg, OperatorLte, func(a L, b R) (any, error) { return compare(a, b) <= 0, nil })
}

func registerModulo[T interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}](reg *OperatorRegistry) {
	RegisterBinary[T, T](reg, OperatorMod, func(a, b T) (any, error) {
		if b == 0 {
			return nil, errors.New("modulo by zero")
		}
		return a % b, nil
	})
}

// NewDefaultRegistry creates a registry with PostgreSQL-compatible operators
// for standard Go types.
func NewDefaultRegistry() *OperatorRegistry {
	reg := NewOperatorRegistry()

	// bool
	registerOrder[bool](reg, compareBool)
	RegisterBinary[bool, bool](reg, OperatorIs, func(a, b bool) (any, error) { return a == b, nil })
	RegisterUnary[bool](reg, OperatorNot, func(a bool) (any, error) { return !a, nil })

	// int
	registerComparison[int](reg)
	registerIntegerArithmetic[int](reg)
	registerModulo[int](reg)

	// int64
	registerComparison[int64](reg)
	registerIntegerArithmetic[int64](reg)
	registerModulo[int64](reg)

	// float64
	registerOrder[float64](reg, compareFloat64)
	registerFloatArithmetic(reg)

	// Numbers of different types: one bigint, computed with a double precision
	registerIntegerPair[int, int64](reg)
	registerIntegerPair[int64, int](reg)
	registerFloatPair[int, float64](reg)
	registerFloatPair[float64, int](reg)
	registerFloatPair[int64, float64](reg)
	registerFloatPair[float64, int64](reg)

	// string
	registerComparison[string](reg)

	// time.Duration (interval)
	RegisterBinary[time.Duration, time.Duration](reg, OperatorEq, func(a, b time.Duration) (any, error) { return a == b, nil })
	RegisterBinary[time.Duration, time.Duration](reg, OperatorNe, func(a, b time.Duration) (any, error) { return a != b, nil })
	RegisterBinary[time.Duration, time.Duration](reg, OperatorGt, func(a, b time.Duration) (any, error) { return a > b, nil })
	RegisterBinary[time.Duration, time.Duration](reg, OperatorGte, func(a, b time.Duration) (any, error) { return a >= b, nil })
	RegisterBinary[time.Duration, time.Duration](reg, OperatorLt, func(a, b time.Duration) (any, error) { return a < b, nil })
	RegisterBinary[time.Duration, time.Duration](reg, OperatorLte, func(a, b time.Duration) (any, error) { return a <= b, nil })
	RegisterBinary[time.Duration, time.Duration](reg, OperatorAdd, func(a, b time.Duration) (any, error) { return a + b, nil })
	RegisterBinary[time.Duration, time.Duration](reg, OperatorSub, func(a, b time.Duration) (any, error) { return a - b, nil })
	RegisterUnary[time.Duration](reg, OperatorNeg, func(a time.Duration) (any, error) { return -a, nil })

	// time.Time (timestamp)
	RegisterBinary[time.Time, time.Time](reg, OperatorEq, func(a, b time.Time) (any, error) { return a.Equal(b), nil })
	RegisterBinary[time.Time, time.Time](reg, OperatorNe, func(a, b time.Time) (any, error) { return !a.Equal(b), nil })
	RegisterBinary[time.Time, time.Time](reg, OperatorGt, func(a, b time.Time) (any, error) { return a.After(b), nil })
	RegisterBinary[time.Time, time.Time](reg, OperatorGte, func(a, b time.Time) (any, error) { return !a.Before(b), nil })
	RegisterBinary[time.Time, time.Time](reg, OperatorLt, func(a, b time.Time) (any, error) { return a.Before(b), nil })
	RegisterBinary[time.Time, time.Time](reg, OperatorLte, func(a, b time.Time) (any, error) { return !a.After(b), nil })

	// Mixed: timestamp - timestamp = interval
	RegisterBinary[time.Time, time.Time](reg, OperatorSub, func(a, b time.Time) (any, error) { return a.Sub(b), nil })

	// Mixed: timestamp +/- interval = timestamp
	RegisterBinary[time.Time, time.Duration](reg, OperatorAdd, func(a time.Time, b time.Duration) (any, error) { return a.Add(b), nil })
	RegisterBinary[time.Time, time.Duration](reg, OperatorSub, func(a time.Time, b time.Duration) (any, error) { return a.Add(-b), nil })

	return reg
}
