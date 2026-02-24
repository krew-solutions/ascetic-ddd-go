package option

import "fmt"

// Option represents an optional value: every Option is either Some (contains a value) or Nothing (does not).
type Option[T any] struct {
	val   T
	valid bool
}

// Some creates an Option containing the given value.
func Some[T any](val T) Option[T] {
	return Option[T]{val: val, valid: true}
}

// Nothing creates an empty Option.
func Nothing[T any]() Option[T] {
	return Option[T]{}
}

// IsSome returns true if the Option contains a value.
func (o Option[T]) IsSome() bool {
	return o.valid
}

// IsNothing returns true if the Option does not contain a value.
func (o Option[T]) IsNothing() bool {
	return !o.valid
}

// Unwrap returns the contained value.
// Panics if the Option is Nothing.
func (o Option[T]) Unwrap() T {
	if !o.valid {
		panic("called Unwrap on a Nothing Option")
	}
	return o.val
}

// UnwrapOr returns the contained value or the provided default.
func (o Option[T]) UnwrapOr(def T) T {
	if o.valid {
		return o.val
	}
	return def
}

// UnwrapOrElse returns the contained value or computes it from the closure.
func (o Option[T]) UnwrapOrElse(f func() T) T {
	if o.valid {
		return o.val
	}
	return f()
}

// UnwrapOrZero returns the contained value or the zero value of T.
func (o Option[T]) UnwrapOrZero() T {
	return o.val
}

// Map applies a function to the contained value (if Some), or returns Nothing (if Nothing).
func Map[T any, U any](o Option[T], f func(T) U) Option[U] {
	if o.valid {
		return Some(f(o.val))
	}
	return Nothing[U]()
}

// MapOr applies a function to the contained value (if Some), or returns the provided default (if Nothing).
func MapOr[T any, U any](o Option[T], def U, f func(T) U) U {
	if o.valid {
		return f(o.val)
	}
	return def
}

// AndThen returns Nothing if the Option is Nothing, otherwise calls f with the contained value and returns the result.
func AndThen[T any, U any](o Option[T], f func(T) Option[U]) Option[U] {
	if o.valid {
		return f(o.val)
	}
	return Nothing[U]()
}

// Or returns the Option if it contains a value, otherwise returns optb.
func (o Option[T]) Or(optb Option[T]) Option[T] {
	if o.valid {
		return o
	}
	return optb
}

// OrElse returns the Option if it contains a value, otherwise calls f and returns the result.
func (o Option[T]) OrElse(f func() Option[T]) Option[T] {
	if o.valid {
		return o
	}
	return f()
}

// String implements fmt.Stringer.
func (o Option[T]) String() string {
	if o.valid {
		return fmt.Sprintf("Some(%v)", o.val)
	}
	return "Nothing"
}
