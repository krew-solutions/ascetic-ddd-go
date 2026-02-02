package pgx

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
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

	// Acquire connection from pool
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	// Create session with acquired connection
	sess := NewSession(ctx, conn)

	return callback(sess)
}
