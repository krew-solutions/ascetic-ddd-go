package operators

import "reflect"

// IsNull tells whether a value is NULL: nil, or a nil pointer, slice or map.
//
// A nullable column is a pointer in Go, and the driver sends a nil pointer, a
// nil slice and a nil map as NULL. Compared with nil, none of them is nil: an
// interface that holds a nil pointer is not a nil interface. IS NULL of one
// was false, and no error was raised.
func IsNull(value any) bool {
	switch value.(type) {
	case nil:
		return true
	case bool, int, int64, float64, string:
		return false
	}
	switch rv := reflect.ValueOf(value); rv.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return rv.IsNil()
	default:
		return false
	}
}

// Indirect returns what a pointer points at, through as many pointers as
// there are, and nil for a nil one. What is not a pointer is returned as it is.
// optional is a value that may hold one or nothing: option.Option[T] of any T.
type optional interface {
	Any() (any, bool)
}

// ReadOption returns what an Option holds, or the null it is if it holds
// nothing; one inside another is read through. Any other value is returned as
// it is.
//
// A member of an aggregate may be an Option of a value, and so may a constant
// of a specification. To a reader of the tree it is the value or a null: taken
// for the value itself it had no operator, IS NULL was false of a Nothing, and
// as a parameter it reached the driver as the text "Some({15})". It is read
// where a value comes to a reader - from the candidate, from a constant - so
// that nothing after meets an Option.
func ReadOption(value any) any {
	for {
		held, ok := value.(optional)
		if !ok {
			return value
		}
		if value, ok = held.Any(); !ok {
			return nil
		}
	}
}

func Indirect(value any) any {
	// The usual values are not pointers, and are told apart without reflection.
	switch value.(type) {
	case nil, bool, int, int64, float64, string:
		return value
	}
	for {
		rv := reflect.ValueOf(value)
		if !rv.IsValid() || rv.Kind() != reflect.Pointer {
			return value
		}
		if rv.IsNil() {
			return nil
		}
		value = rv.Elem().Interface()
	}
}

// Truth reads a condition: true, false, or NULL. A pointer to a bool is read
// through. ok is false if the value is none of them.
func Truth(value any) (truth bool, null bool, ok bool) {
	if truth, ok = value.(bool); ok {
		return truth, false, true
	}
	if IsNull(value) {
		return false, true, true
	}
	truth, ok = Indirect(value).(bool)
	return truth, false, ok
}
