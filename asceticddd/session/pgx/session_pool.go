package pgx

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	sessioncontext "github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/context"
)

type SessionPool struct {
	pool *pgxpool.Pool
}

func NewSessionPool(pool *pgxpool.Pool) *SessionPool {
	return &SessionPool{pool: pool}
}

func (p *SessionPool) Session(ctx context.Context, callback session.SessionPoolCallback) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Захватываем соединение из пула
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	// Создаём сессию с захваченным соединением
	sess := NewSession(ctx, conn)

	// Оборачиваем в SessionContext
	sctx := sessioncontext.NewSessionContext(ctx, sess)

	return callback(sctx)
}
