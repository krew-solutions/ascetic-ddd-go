package rest

import (
	"context"
	"net/http"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/signals"
)

type SessionPool struct {
	transport        http.RoundTripper
	onSessionStarted signals.Signal[session.SessionScopeStartedEvent]
	onSessionEnded   signals.Signal[session.SessionScopeEndedEvent]
}

func NewSessionPool(transport http.RoundTripper) *SessionPool {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &SessionPool{
		transport:        transport,
		onSessionStarted: signals.NewSignal[session.SessionScopeStartedEvent](),
		onSessionEnded:   signals.NewSignal[session.SessionScopeEndedEvent](),
	}
}

func (p *SessionPool) OnSessionStarted() signals.Signal[session.SessionScopeStartedEvent] {
	return p.onSessionStarted
}

func (p *SessionPool) OnSessionEnded() signals.Signal[session.SessionScopeEndedEvent] {
	return p.onSessionEnded
}

func (p *SessionPool) Session(ctx context.Context, callback session.SessionPoolCallback) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	sess := NewSession(ctx, p.transport)

	if err := p.onSessionStarted.Notify(session.SessionScopeStartedEvent{Session: sess}); err != nil {
		return err
	}

	err := callback(sess)

	if endedErr := p.onSessionEnded.Notify(session.SessionScopeEndedEvent{Session: sess}); err == nil {
		err = endedErr
	}

	return err
}
