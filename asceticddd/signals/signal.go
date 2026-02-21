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

func (s *SignalImp[E]) Attach(observer Observer[E], observerId ...any) disposable.Disposable {
	id := resolveId(observer, observerId)
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

func (s *SignalImp[E]) Detach(observer Observer[E], observerId ...any) {
	id := resolveId(observer, observerId)
	for i, e := range s.observers {
		if e.id == id {
			s.observers = append(s.observers[:i], s.observers[i+1:]...)
			return
		}
	}
}

func (s *SignalImp[E]) Notify(event E) error {
	for _, e := range s.observers {
		if err := e.observer(event); err != nil {
			return err
		}
	}
	return nil
}

func resolveId[E any](observer Observer[E], observerId []any) any {
	if len(observerId) > 0 {
		return observerId[0]
	}
	return makeId(observer)
}

func makeId[E any](observer Observer[E]) uintptr {
	return reflect.ValueOf(observer).Pointer()
}
