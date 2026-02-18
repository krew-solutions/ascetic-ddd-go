package signals

import (
	"reflect"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/disposable"
)

type entry[E any] struct {
	id       any
	observer Observer[E]
}

type SignalImp[E any] struct {
	observers []entry[E]
}

func NewSignal[E any]() *SignalImp[E] {
	return &SignalImp[E]{}
}

func (s *SignalImp[E]) Attach(observer Observer[E], observerID ...any) disposable.Disposable {
	id := resolveID(observer, observerID)
	for _, e := range s.observers {
		if e.id == id {
			return disposable.NewDisposable(func() {
				s.Detach(observer, id)
			})
		}
	}
	s.observers = append(s.observers, entry[E]{id: id, observer: observer})
	return disposable.NewDisposable(func() {
		s.Detach(observer, id)
	})
}

func (s *SignalImp[E]) Detach(observer Observer[E], observerID ...any) {
	id := resolveID(observer, observerID)
	for i, e := range s.observers {
		if e.id == id {
			s.observers = append(s.observers[:i], s.observers[i+1:]...)
			return
		}
	}
}

func (s *SignalImp[E]) Notify(event E) {
	for _, e := range s.observers {
		e.observer(event)
	}
}

func resolveID[E any](observer Observer[E], observerID []any) any {
	if len(observerID) > 0 {
		return observerID[0]
	}
	return makeID(observer)
}

func makeID[E any](observer Observer[E]) uintptr {
	return reflect.ValueOf(observer).Pointer()
}
