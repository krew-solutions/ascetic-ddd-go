package signals

import (
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/disposable"
)

type CompositeSignalImp[E any] struct {
	delegates []Signal[E]
}

func NewCompositeSignal[E any](delegates ...Signal[E]) *CompositeSignalImp[E] {
	return &CompositeSignalImp[E]{delegates: delegates}
}

func (s *CompositeSignalImp[E]) Attach(observer Observer[E], observerId ...any) disposable.Disposable {
	disposables := make([]disposable.Disposable, 0, len(s.delegates))
	for _, delegate := range s.delegates {
		disposables = append(disposables, delegate.Attach(observer, observerId...))
	}
	return disposable.NewCompositeDisposable(disposables...)
}

func (s *CompositeSignalImp[E]) Detach(observer Observer[E], observerId ...any) {
	for _, delegate := range s.delegates {
		delegate.Detach(observer, observerId...)
	}
}

func (s *CompositeSignalImp[E]) Notify(event E) {
	for _, delegate := range s.delegates {
		delegate.Notify(event)
	}
}
