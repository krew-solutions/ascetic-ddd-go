package composite

import (
	"context"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/signals"
)

type CompositeSession struct {
	delegates []session.Session
}

func NewCompositeSession(delegates []session.Session) *CompositeSession {
	return &CompositeSession{delegates: delegates}
}

func (s *CompositeSession) Context() context.Context {
	return s.delegates[0].Context()
}

func (s *CompositeSession) OnStarted() signals.Signal[session.SessionScopeStartedEvent] {
	delegates := make([]signals.Signal[session.SessionScopeStartedEvent], len(s.delegates))
	for i, d := range s.delegates {
		delegates[i] = d.OnStarted()
	}
	return signals.NewCompositeSignal(delegates...)
}

func (s *CompositeSession) OnEnded() signals.Signal[session.SessionScopeEndedEvent] {
	delegates := make([]signals.Signal[session.SessionScopeEndedEvent], len(s.delegates))
	for i, d := range s.delegates {
		delegates[i] = d.OnEnded()
	}
	return signals.NewCompositeSignal(delegates...)
}

func (s *CompositeSession) Atomic(callback session.SessionCallback) error {
	return s.atomicRecursive(callback, 0, nil)
}

func (s *CompositeSession) atomicRecursive(callback session.SessionCallback, index int, collected []session.Session) error {
	if index >= len(s.delegates) {
		return callback(NewCompositeSession(collected))
	}
	return s.delegates[index].Atomic(func(inner session.Session) error {
		return s.atomicRecursive(callback, index+1, append(collected, inner))
	})
}

func (s *CompositeSession) Delegate(index int) session.Session {
	return s.delegates[index]
}
