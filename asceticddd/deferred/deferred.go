package deferred

import "github.com/hashicorp/go-multierror"

/**
* Simplified version of
* - https://github.com/emacsway/store/blob/devel/polyfill.js#L199
* - https://github.com/emacsway/go-promise
*
* See also:
* - https://promisesaplus.com/
* - http://promises-aplus.github.io/promises-spec/
**/

func Noop[T, R any](_ T) (R, error) {
	var zero R
	return zero, nil
}

type nextDeferred interface {
	resolveAny(any)
	rejectAny(error)
	OccurredErr() error
}

type handler[T any] struct {
	onSuccess func(T) (any, error)
	onError   func(error) (any, error)
	next      nextDeferred
}

type DeferredImp[T any] struct {
	value       T
	err         error
	occurredErr error
	isResolved  bool
	isRejected  bool
	handlers    []handler[T]
}

func (d *DeferredImp[T]) resolveAny(v any) {
	var t T
	if v != nil {
		t = v.(T)
	}
	d.Resolve(t)
}

func (d *DeferredImp[T]) rejectAny(err error) {
	d.Reject(err)
}

func (d *DeferredImp[T]) Resolve(value T) {
	d.value = value
	d.isResolved = true
	for _, h := range d.handlers {
		d.resolveHandler(h)
	}
}

func (d *DeferredImp[T]) Reject(err error) {
	d.err = err
	d.isRejected = true
	for _, h := range d.handlers {
		d.rejectHandler(h)
	}
}

func (d *DeferredImp[T]) addHandler(h handler[T]) {
	d.handlers = append(d.handlers, h)
	if d.isResolved {
		d.resolveHandler(h)
	} else if d.isRejected {
		d.rejectHandler(h)
	}
}

func (d *DeferredImp[T]) Then(onSuccess func(T) (any, error), onError func(error) (any, error)) Deferred[any] {
	next := &DeferredImp[any]{}
	d.addHandler(handler[T]{
		onSuccess: onSuccess,
		onError:   onError,
		next:      next,
	})
	return next
}

// Then registers typed callbacks for success and error cases.
//
// Per Promises/A+ 2.2.7:
//   - If onSuccess returns a value, next deferred is resolved with it.
//   - If onSuccess returns an error, next deferred is rejected with it.
//   - If onError returns a value, next deferred is resolved with it (recovery).
//   - If onError returns an error, next deferred is rejected with it.
//
// This is a free function (not a method) because Go does not support
// type parameters on methods. This allows R to be a concrete type,
// preserving type safety through the chain.
func Then[T, R any](d *DeferredImp[T], onSuccess func(T) (R, error), onError func(error) (R, error)) *DeferredImp[R] {
	next := &DeferredImp[R]{}
	d.addHandler(handler[T]{
		onSuccess: func(v T) (any, error) { return onSuccess(v) },
		onError:   func(err error) (any, error) { return onError(err) },
		next:      next,
	})
	return next
}

func (d *DeferredImp[T]) resolveHandler(h handler[T]) {
	result, err := h.onSuccess(d.value)
	if err == nil {
		h.next.resolveAny(result)
	} else {
		d.occurredErr = multierror.Append(d.occurredErr, err)
		h.next.rejectAny(err)
	}
}

func (d *DeferredImp[T]) rejectHandler(h handler[T]) {
	result, err := h.onError(d.err)
	if err == nil {
		h.next.resolveAny(result)
	} else {
		d.occurredErr = multierror.Append(d.occurredErr, err)
		h.next.rejectAny(err)
	}
}

func (d *DeferredImp[T]) OccurredErr() error {
	err := d.occurredErr
	for _, h := range d.handlers {
		nestedErr := h.next.OccurredErr()
		if nestedErr != nil {
			err = multierror.Append(err, nestedErr)
		}
	}
	return err
}

func All[T any](deferreds []Deferred[T]) *DeferredImp[[]T] {
	result := &DeferredImp[[]T]{}

	if len(deferreds) == 0 {
		result.Resolve([]T{})
		return result
	}

	count := len(deferreds)
	values := make([]T, count)
	resolvedCount := 0
	rejected := false

	for i, d := range deferreds {
		idx := i
		d.Then(func(value T) (any, error) {
			if rejected {
				return nil, nil
			}
			values[idx] = value
			resolvedCount++
			if resolvedCount == count {
				result.Resolve(values)
			}
			return nil, nil
		}, func(err error) (any, error) {
			if !rejected {
				rejected = true
				result.Reject(err)
			}
			return nil, nil
		})
	}

	return result
}
