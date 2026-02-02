package pgx

import (
	"context"

	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/result"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/utils"
)

// Session represents a database session without transaction
type Session struct {
	ctx    context.Context
	conn   *pgxpool.Conn
	parent session.Session
}

func NewSession(ctx context.Context, conn *pgxpool.Conn) *Session {
	return &Session{
		ctx:    ctx,
		conn:   conn,
		parent: nil,
	}
}

func (s *Session) Context() context.Context {
	return s.ctx
}

func (s *Session) Connection() session.DbConnection {
	return &connection{ctx: s.ctx, exec: s.conn}
}

func (s *Session) Atomic(callback session.SessionCallback) error {
	// Start new transaction
	tx, err := s.conn.Begin(s.ctx)
	if err != nil {
		return errors.Wrap(err, "unable to start transaction")
	}

	txSession := NewTransactionSession(s.ctx, tx, s)

	err = callback(txSession)
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

// TransactionSession represents a session inside transaction
type TransactionSession struct {
	ctx    context.Context
	tx     pgx.Tx
	parent session.Session
}

func NewTransactionSession(ctx context.Context, tx pgx.Tx, parent session.Session) *TransactionSession {
	return &TransactionSession{
		ctx:    ctx,
		tx:     tx,
		parent: parent,
	}
}

func (s *TransactionSession) Context() context.Context {
	return s.ctx
}

func (s *TransactionSession) Connection() session.DbConnection {
	return &connection{ctx: s.ctx, exec: s.tx}
}

func (s *TransactionSession) Atomic(callback session.SessionCallback) error {
	// Create savepoint (nested transaction)
	nestedTx, err := s.tx.Begin(s.ctx)
	if err != nil {
		return errors.Wrap(err, "unable to start savepoint")
	}

	savepointSession := NewSavepointSession(s.ctx, nestedTx, s)

	err = callback(savepointSession)
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

// SavepointSession represents a session inside savepoint (nested transaction)
type SavepointSession struct {
	ctx    context.Context
	tx     pgx.Tx
	parent session.Session
}

func NewSavepointSession(ctx context.Context, tx pgx.Tx, parent session.Session) *SavepointSession {
	return &SavepointSession{
		ctx:    ctx,
		tx:     tx,
		parent: parent,
	}
}

func (s *SavepointSession) Context() context.Context {
	return s.ctx
}

func (s *SavepointSession) Connection() session.DbConnection {
	return &connection{ctx: s.ctx, exec: s.tx}
}

func (s *SavepointSession) Atomic(callback session.SessionCallback) error {
	// Create nested savepoint
	nestedTx, err := s.tx.Begin(s.ctx)
	if err != nil {
		return errors.Wrap(err, "unable to start nested savepoint")
	}

	nestedSession := NewSavepointSession(s.ctx, nestedTx, s)

	err = callback(nestedSession)
	if err != nil {
		if txErr := nestedTx.Rollback(s.ctx); txErr != nil {
			return multierror.Append(err, txErr)
		}
		return err
	}

	if txErr := nestedTx.Commit(s.ctx); txErr != nil {
		return errors.Wrap(txErr, "failed to commit nested savepoint")
	}

	return nil
}

// executor interface for both *pgxpool.Conn and pgx.Tx
type executor interface {
	Exec(ctx context.Context, query string, args ...any) (pgx.CommandTag, error)
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
	return c.exec.Query(c.ctx, query, args...)
}

func (c *connection) QueryRow(query string, args ...any) session.Row {
	return c.exec.QueryRow(c.ctx, query, args...)
}
