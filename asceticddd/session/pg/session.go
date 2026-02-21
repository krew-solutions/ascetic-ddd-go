package pg

import (
	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/identitymap"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/result"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/utils"
)

const defaultCacheSize = 100

// Session represents a database session without transaction
type Session struct {
	ctx         context.Context
	conn        *pgxpool.Conn
	parent      session.Session
	identityMap *identitymap.IdentityMap
}

func NewSession(ctx context.Context, conn *pgxpool.Conn) *Session {
	return &Session{
		ctx:         ctx,
		conn:        conn,
		parent:      nil,
		identityMap: identitymap.New(defaultCacheSize, identitymap.ReadUncommitted),
	}
}

func (s *Session) Context() context.Context {
	return s.ctx
}

func (s *Session) Connection() session.DbConnection {
	return &connection{ctx: s.ctx, exec: s.conn}
}

func (s *Session) IdentityMap() *identitymap.IdentityMap {
	return s.identityMap
}

func (s *Session) Atomic(callback session.SessionCallback) error {
	tx, err := s.conn.Begin(s.ctx)
	if err != nil {
		return errors.Wrap(err, "unable to start transaction")
	}

	im := identitymap.New(defaultCacheSize, identitymap.Serializable)
	atomicSession := NewAtomicSession(s.ctx, tx, im, s)

	err = callback(atomicSession)
	im.Clear()

	if err != nil {
		if txErr := tx.Rollback(s.ctx); txErr != nil {
			return multierror.Append(err, txErr)
		}
		return err
	}

	if txErr := tx.Commit(s.ctx); txErr != nil {
		return errors.Wrap(txErr, "failed to commit transaction")
	}

	return nil
}

// AtomicSession represents a session inside transaction
type AtomicSession struct {
	ctx         context.Context
	tx          pgx.Tx
	parent      session.Session
	identityMap *identitymap.IdentityMap
}

func NewAtomicSession(ctx context.Context, tx pgx.Tx, identityMap *identitymap.IdentityMap, parent session.Session) *AtomicSession {
	return &AtomicSession{
		ctx:         ctx,
		tx:          tx,
		parent:      parent,
		identityMap: identityMap,
	}
}

func (s *AtomicSession) Context() context.Context {
	return s.ctx
}

func (s *AtomicSession) Connection() session.DbConnection {
	return &connection{ctx: s.ctx, exec: s.tx}
}

func (s *AtomicSession) IdentityMap() *identitymap.IdentityMap {
	return s.identityMap
}

func (s *AtomicSession) Atomic(callback session.SessionCallback) error {
	nestedTx, err := s.tx.Begin(s.ctx)
	if err != nil {
		return errors.Wrap(err, "unable to start savepoint")
	}

	atomicSession := NewAtomicSession(s.ctx, nestedTx, s.identityMap, s)

	err = callback(atomicSession)
	if err != nil {
		if txErr := nestedTx.Rollback(s.ctx); txErr != nil {
			return multierror.Append(err, txErr)
		}
		return err
	}

	if txErr := nestedTx.Commit(s.ctx); txErr != nil {
		return errors.Wrap(txErr, "failed to commit savepoint")
	}

	return nil
}

// executor interface for both *pgxpool.Conn and pgx.Tx
type executor interface {
	Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, query string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, query string, args ...any) pgx.Row
}

// connection implements session.DbConnection
type connection struct {
	ctx  context.Context
	exec executor
}

func (c *connection) Exec(query string, args ...any) (session.Result, error) {
	if utils.IsAutoincrementInsertQuery(query) {
		return c.insert(query, args...)
	}

	tag, err := c.exec.Exec(c.ctx, query, args...)
	if err != nil {
		return nil, err
	}

	return result.NewResult(0, tag.RowsAffected()), nil
}

func (c *connection) insert(query string, args ...any) (session.Result, error) {
	var id int64
	err := c.exec.QueryRow(c.ctx, query, args...).Scan(&id)
	if err != nil {
		return nil, err
	}

	return result.NewResult(id, 0), nil
}

func (c *connection) Query(query string, args ...any) (session.Rows, error) {
	rows, err := c.exec.Query(c.ctx, query, args...)
	if err != nil {
		return nil, err
	}
	return &rowsAdapter{rows: rows}, nil
}

func (c *connection) QueryRow(query string, args ...any) session.Row {
	row := c.exec.QueryRow(c.ctx, query, args...)
	return &rowAdapter{row: row}
}
