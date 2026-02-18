package signals

import (
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/disposable"
)

type Observer[E any] func(E)

type Signal[E any] interface {
	Attach(observer Observer[E], observerID ...any) disposable.Disposable
	Detach(observer Observer[E], observerID ...any)
	Notify(event E)
}
