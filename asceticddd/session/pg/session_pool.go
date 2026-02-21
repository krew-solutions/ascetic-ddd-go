package pg

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/signals"
)

type SessionPool struct {
	pool             *pgxpool.Pool
	onSessionStarted signals.Signal[session.SessionScopeStartedEvent]
	onSessionEnded   signals.Signal[session.SessionScopeEndedEvent]
}

func NewSessionPool(pool *pgxpool.Pool) *SessionPool {
	return &SessionPool{
		pool:             pool,
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

	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	sess := NewSession(ctx, conn)

	if err := p.onSessionStarted.Notify(session.SessionScopeStartedEvent{Session: sess}); err != nil {
		return err
	}

	err = callback(sess)

	if endedErr := p.onSessionEnded.Notify(session.SessionScopeEndedEvent{Session: sess}); err == nil {
		err = endedErr
	}

	return err
}
