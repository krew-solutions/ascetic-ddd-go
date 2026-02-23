package deferred

type Deferred[T any] interface {
	Resolve(T)
	Reject(error)
	Then(func(T) (any, error), func(error) (any, error)) Deferred[any]
	OccurredErr() error
}
