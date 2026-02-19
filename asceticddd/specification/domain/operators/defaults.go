package operators

import (
	"cmp"
	"errors"
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

func registerArithmetic[T interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}](reg *OperatorRegistry) {
	RegisterBinary[T, T](reg, OperatorAdd, func(a, b T) (any, error) { return a + b, nil })
	RegisterBinary[T, T](reg, OperatorSub, func(a, b T) (any, error) { return a - b, nil })
	RegisterBinary[T, T](reg, OperatorMul, func(a, b T) (any, error) { return a * b, nil })
	RegisterBinary[T, T](reg, OperatorDiv, func(a, b T) (any, error) {
		if b == 0 {
			return nil, errors.New("division by zero")
		}
		return a / b, nil
	})
	RegisterUnary[T](reg, OperatorPos, func(a T) (any, error) { return a, nil })
	RegisterUnary[T](reg, OperatorNeg, func(a T) (any, error) { return -a, nil })
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
	RegisterBinary[bool, bool](reg, OperatorEq, func(a, b bool) (any, error) { return a == b, nil })
	RegisterBinary[bool, bool](reg, OperatorNe, func(a, b bool) (any, error) { return a != b, nil })
	RegisterBinary[bool, bool](reg, OperatorIs, func(a, b bool) (any, error) { return a == b, nil })
	RegisterUnary[bool](reg, OperatorNot, func(a bool) (any, error) { return !a, nil })

	// int
	registerComparison[int](reg)
	registerArithmetic[int](reg)
	registerModulo[int](reg)

	// int64
	registerComparison[int64](reg)
	registerArithmetic[int64](reg)
	registerModulo[int64](reg)

	// float64
	registerComparison[float64](reg)
	registerArithmetic[float64](reg)

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
	RegisterUnary[time.Duration](reg, OperatorPos, func(a time.Duration) (any, error) { return a, nil })
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
