// Package rop implements Scott Wlaschin's Railway-Oriented Programming toolkit
// (fsharpforfunandprofit.com/rop) as a thin layer over a two-track Result type.
// Source: https://fsharpforfunandprofit.com/posts/recipe-part2/
//
// The Failure branch carries a non-empty list of errors rather than a single
// error, so independent failures from parallel validations can accumulate via
// Apply / Map2 / Map3 / Map4 / Plus / And without loss — e.g. a form with a
// bad symbol AND a bad side AND a negative quantity reports all three problems
// together.
//
// Naming follows the asceticddd/option vocabulary: AndThen for monadic bind,
// Or / OrElse, Unwrap*. Wlaschin's canonical names are noted in godoc.
package rop

import (
	"fmt"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/option"
)

// Result is a two-track outcome: either a Success value of type A or a Failure
// carrying a non-empty list of errors of type Err.
type Result[A, Err any] struct {
	val  A
	errs []Err
	isOk bool
}

// Succeed creates a Success Result. Wlaschin's succeed / return / pure.
func Succeed[A, Err any](v A) Result[A, Err] {
	return Result[A, Err]{val: v, isOk: true}
}

// Fail creates a Failure Result with a single error. Wlaschin's fail.
func Fail[A, Err any](e Err) Result[A, Err] {
	return Result[A, Err]{errs: []Err{e}}
}

// FailMany creates a Failure Result from a non-empty error list.
// Panics if errs is empty.
func FailMany[A, Err any](errs []Err) Result[A, Err] {
	if len(errs) == 0 {
		panic("rop.FailMany: errs must be non-empty")
	}
	return Result[A, Err]{errs: errs}
}

// FromError lifts Go's idiomatic (value, error) pair into a Result.
// If err is non-nil the value is discarded and the error becomes a Failure;
// otherwise the value becomes a Success.
func FromError[A any](v A, err error) Result[A, error] {
	if err != nil {
		return Fail[A, error](err)
	}
	return Succeed[A, error](v)
}

// OfOption lifts an Option into a Result: Some(v) becomes Succeed(v),
// Nothing becomes Fail(err). The bridge to asceticddd/option.
func OfOption[A, Err any](o option.Option[A], err Err) Result[A, Err] {
	if o.IsSome() {
		return Succeed[A, Err](o.Unwrap())
	}
	return Fail[A, Err](err)
}

// IsOk returns true if the Result is on the Success track.
func (r Result[A, Err]) IsOk() bool { return r.isOk }

// IsError returns true if the Result is on the Failure track.
func (r Result[A, Err]) IsError() bool { return !r.isOk }

// Errors returns the failure list (nil when the Result is on the Success track).
func (r Result[A, Err]) Errors() []Err { return r.errs }

// Unwrap returns the contained value. Panics if the Result is on the Failure track.
func (r Result[A, Err]) Unwrap() A {
	if !r.isOk {
		panic("called Unwrap on a Failure Result")
	}
	return r.val
}

// UnwrapOr returns the contained value or the provided default.
func (r Result[A, Err]) UnwrapOr(def A) A {
	if r.isOk {
		return r.val
	}
	return def
}

// UnwrapOrElse returns the contained value or computes one from the error list.
func (r Result[A, Err]) UnwrapOrElse(f func([]Err) A) A {
	if r.isOk {
		return r.val
	}
	return f(r.errs)
}

// UnwrapOrZero returns the contained value or the zero value of A.
func (r Result[A, Err]) UnwrapOrZero() A {
	return r.val
}

// Or returns the Result if it is on the Success track, otherwise returns alt.
func (r Result[A, Err]) Or(alt Result[A, Err]) Result[A, Err] {
	if r.isOk {
		return r
	}
	return alt
}

// OrElse returns the Result if it is on the Success track, otherwise calls f with the error list.
func (r Result[A, Err]) OrElse(f func([]Err) Result[A, Err]) Result[A, Err] {
	if r.isOk {
		return r
	}
	return f(r.errs)
}

// String implements fmt.Stringer.
func (r Result[A, Err]) String() string {
	if r.isOk {
		return fmt.Sprintf("Ok(%v)", r.val)
	}
	return fmt.Sprintf("Error(%v)", r.errs)
}

// Either applies successFn on the Success track or failureFn on the Failure track.
// Wlaschin's either.
func Either[A, B, Err any](r Result[A, Err], successFn func(A) B, failureFn func([]Err) B) B {
	if r.isOk {
		return successFn(r.val)
	}
	return failureFn(r.errs)
}

// Map applies a pure function on the Success track. Wlaschin's map.
func Map[A, B, Err any](r Result[A, Err], f func(A) B) Result[B, Err] {
	if r.isOk {
		return Succeed[B, Err](f(r.val))
	}
	return Result[B, Err]{errs: r.errs}
}

// AndThen is monadic bind: short-circuits on the first Failure. Wlaschin's bind.
func AndThen[A, B, Err any](r Result[A, Err], f func(A) Result[B, Err]) Result[B, Err] {
	if r.isOk {
		return f(r.val)
	}
	return Result[B, Err]{errs: r.errs}
}

// Bind is an alias for AndThen using Wlaschin's canonical name.
func Bind[A, B, Err any](r Result[A, Err], f func(A) Result[B, Err]) Result[B, Err] {
	return AndThen(r, f)
}

// Apply applies a wrapped function to a wrapped value. On two Failures their
// error lists are concatenated — this is the core of accumulation.
func Apply[A, B, Err any](rf Result[func(A) B, Err], rx Result[A, Err]) Result[B, Err] {
	switch {
	case rf.isOk && rx.isOk:
		return Succeed[B, Err](rf.val(rx.val))
	case !rf.isOk && !rx.isOk:
		return Result[B, Err]{errs: concatErrs(rf.errs, rx.errs)}
	case !rf.isOk:
		return Result[B, Err]{errs: rf.errs}
	default:
		return Result[B, Err]{errs: rx.errs}
	}
}

// Map2 applies f to two Results, accumulating errors if either branch fails.
func Map2[A, B, C, Err any](ra Result[A, Err], rb Result[B, Err], f func(A, B) C) Result[C, Err] {
	if ra.isOk && rb.isOk {
		return Succeed[C, Err](f(ra.val, rb.val))
	}
	var errs []Err
	if !ra.isOk {
		errs = append(errs, ra.errs...)
	}
	if !rb.isOk {
		errs = append(errs, rb.errs...)
	}
	return Result[C, Err]{errs: errs}
}

// Map3 applies f to three Results, accumulating errors from every failing branch.
func Map3[A, B, C, D, Err any](
	ra Result[A, Err], rb Result[B, Err], rc Result[C, Err],
	f func(A, B, C) D,
) Result[D, Err] {
	if ra.isOk && rb.isOk && rc.isOk {
		return Succeed[D, Err](f(ra.val, rb.val, rc.val))
	}
	var errs []Err
	if !ra.isOk {
		errs = append(errs, ra.errs...)
	}
	if !rb.isOk {
		errs = append(errs, rb.errs...)
	}
	if !rc.isOk {
		errs = append(errs, rc.errs...)
	}
	return Result[D, Err]{errs: errs}
}

// Map4 applies f to four Results, accumulating errors from every failing branch.
func Map4[A, B, C, D, E, Err any](
	ra Result[A, Err], rb Result[B, Err], rc Result[C, Err], rd Result[D, Err],
	f func(A, B, C, D) E,
) Result[E, Err] {
	if ra.isOk && rb.isOk && rc.isOk && rd.isOk {
		return Succeed[E, Err](f(ra.val, rb.val, rc.val, rd.val))
	}
	var errs []Err
	if !ra.isOk {
		errs = append(errs, ra.errs...)
	}
	if !rb.isOk {
		errs = append(errs, rb.errs...)
	}
	if !rc.isOk {
		errs = append(errs, rc.errs...)
	}
	if !rd.isOk {
		errs = append(errs, rd.errs...)
	}
	return Result[E, Err]{errs: errs}
}

// Switch lifts a one-track function into a switch (always-success). Wlaschin's switch / lift.
func Switch[A, B, Err any](f func(A) B) func(A) Result[B, Err] {
	return func(x A) Result[B, Err] {
		return Succeed[B, Err](f(x))
	}
}

// Tee runs f for its side effect and returns x unchanged. Wlaschin's tee / Unix tee / tap.
func Tee[A any](f func(A), x A) A {
	f(x)
	return x
}

// TryCatch lifts a function with Go's idiomatic (value, error) signature into
// a switch. A non-nil error is routed through errHandler onto the Failure
// track. The Go-idiomatic counterpart of Wlaschin's tryCatch: in F#/OCaml the
// failure mechanism is exceptions, in Go it is the error return value.
func TryCatch[A, B, Err any](f func(A) (B, error), errHandler func(error) Err) func(A) Result[B, Err] {
	return func(x A) Result[B, Err] {
		v, err := f(x)
		if err != nil {
			return Fail[B, Err](errHandler(err))
		}
		return Succeed[B, Err](v)
	}
}

// DoubleMap is a bifunctor map: successFn on the Success track and failureFn
// element-wise across the failure list. Wlaschin's doubleMap / bimap.
func DoubleMap[A, B, Err1, Err2 any](
	r Result[A, Err1], successFn func(A) B, failureFn func(Err1) Err2,
) Result[B, Err2] {
	if r.isOk {
		return Succeed[B, Err2](successFn(r.val))
	}
	errs := make([]Err2, len(r.errs))
	for i, e := range r.errs {
		errs[i] = failureFn(e)
	}
	return Result[B, Err2]{errs: errs}
}

// Plus combines two switch functions over the same input. Successes are merged
// via addSuccess; failure lists via addFailure. Wlaschin's plus (++, <+>).
func Plus[X, A, Err any](
	addSuccess func(A, A) A,
	addFailure func([]Err, []Err) []Err,
	s1, s2 func(X) Result[A, Err],
) func(X) Result[A, Err] {
	return func(x X) Result[A, Err] {
		r1 := s1(x)
		r2 := s2(x)
		switch {
		case r1.isOk && r2.isOk:
			return Succeed[A, Err](addSuccess(r1.val, r2.val))
		case !r1.isOk && !r2.isOk:
			return Result[A, Err]{errs: addFailure(r1.errs, r2.errs)}
		case !r1.isOk:
			return Result[A, Err]{errs: r1.errs}
		default:
			return Result[A, Err]{errs: r2.errs}
		}
	}
}

// And is the validation-flavoured Plus: returns the first success value,
// concatenates failure lists. Wlaschin's &&&.
func And[X, A, Err any](v1, v2 func(X) Result[A, Err]) func(X) Result[A, Err] {
	return Plus[X, A, Err](
		func(a, _ A) A { return a },
		concatErrs[Err],
		v1, v2,
	)
}

// Pipe composes two plain functions: Pipe(f, g)(x) == g(f(x)). The Go
// counterpart of F#'s >> and OCaml's Fun.compose — not Result-specific,
// included because Go's stdlib does not provide it.
func Pipe[A, B, C any](f func(A) B, g func(B) C) func(A) C {
	return func(x A) C {
		return g(f(x))
	}
}

// Compose chains two switch functions into a new switch function (Kleisli
// composition). Wlaschin's >=>.
func Compose[A, B, C, Err any](
	f func(A) Result[B, Err], g func(B) Result[C, Err],
) func(A) Result[C, Err] {
	return func(x A) Result[C, Err] {
		return AndThen(f(x), g)
	}
}

func concatErrs[Err any](a, b []Err) []Err {
	out := make([]Err, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}
