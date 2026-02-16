package composite

import (
	"context"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

type CompositeSessionPool struct {
	delegates []session.SessionPool
}

func NewCompositeSessionPool(delegates ...session.SessionPool) *CompositeSessionPool {
	return &CompositeSessionPool{delegates: delegates}
}

func (p *CompositeSessionPool) Session(ctx context.Context, callback session.SessionPoolCallback) error {
	return p.sessionRecursive(ctx, callback, 0, nil)
}

func (p *CompositeSessionPool) sessionRecursive(ctx context.Context, callback session.SessionPoolCallback, index int, collected []session.Session) error {
	if index >= len(p.delegates) {
		return callback(NewCompositeSession(collected))
	}
	return p.delegates[index].Session(ctx, func(inner session.Session) error {
		return p.sessionRecursive(ctx, callback, index+1, append(collected, inner))
	})
}

func (p *CompositeSessionPool) Delegate(index int) session.SessionPool {
	return p.delegates[index]
}
