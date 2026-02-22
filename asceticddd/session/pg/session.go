package pg

import (
	"context"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pkg/errors"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/identitymap"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/result"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/signals"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/utils"
)

const defaultCacheSize = 100

// Session represents a database session without transaction
type Session struct {
	ctx            context.Context
	conn           *pgxpool.Conn
	parent         session.Session
	identityMap    *identitymap.IdentityMap
	onStarted      signals.Signal[session.SessionScopeStartedEvent]
	onEnded        signals.Signal[session.SessionScopeEndedEvent]
	onQueryStarted signals.Signal[session.QueryStartedEvent]
	onQueryEnded   signals.Signal[session.QueryEndedEvent]
}

func NewSession(ctx context.Context, conn *pgxpool.Conn) *Session {
	return &Session{
		ctx:            ctx,
		conn:           conn,
		parent:         nil,
		identityMap:    identitymap.New(defaultCacheSize, identitymap.ReadUncommitted),
		onStarted:      signals.NewSignal[session.SessionScopeStartedEvent](),
		onEnded:        signals.NewSignal[session.SessionScopeEndedEvent](),
		onQueryStarted: signals.NewSignal[session.QueryStartedEvent](),
		onQueryEnded:   signals.NewSignal[session.QueryEndedEvent](),
	}
}

func (s *Session) Context() context.Context {
	return s.ctx
}

func (s *Session) Connection() session.DbConnection {
	return &connection{
		ctx:            s.ctx,
		exec:           s.conn,
		dbSession:      s,
		onQueryStarted: s.onQueryStarted,
		onQueryEnded:   s.onQueryEnded,
	}
}

func (s *Session) IdentityMap() *identitymap.IdentityMap {
	return s.identityMap
}

func (s *Session) OnAtomicStarted() signals.Signal[session.SessionScopeStartedEvent] {
	return s.onStarted
}

func (s *Session) OnAtomicEnded() signals.Signal[session.SessionScopeEndedEvent] {
	return s.onEnded
}

func (s *Session) OnQueryStarted() signals.Signal[session.QueryStartedEvent] {
	return s.onQueryStarted
}

func (s *Session) OnQueryEnded() signals.Signal[session.QueryEndedEvent] {
	return s.onQueryEnded
}

func (s *Session) Atomic(callback session.SessionCallback) error {
	tx, err := s.conn.Begin(s.ctx)
	if err != nil {
		return errors.Wrap(err, "unable to start transaction")
	}

	im := identitymap.New(defaultCacheSize, identitymap.Serializable)
	atomicSession := NewAtomicSession(s.ctx, tx, im, s)

	if err := s.onStarted.Notify(session.SessionScopeStartedEvent{Session: atomicSession}); err != nil {
		if txErr := tx.Rollback(s.ctx); txErr != nil {
			return multierror.Append(err, txErr)
		}
		return err
	}

	err = callback(atomicSession)
	im.Clear()

	if endedErr := s.onEnded.Notify(session.SessionScopeEndedEvent{Session: atomicSession}); err == nil {
		err = endedErr
	}

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
	ctx            context.Context
	tx             pgx.Tx
	parent         session.Session
	identityMap    *identitymap.IdentityMap
	onStarted      signals.Signal[session.SessionScopeStartedEvent]
	onEnded        signals.Signal[session.SessionScopeEndedEvent]
	onQueryStarted signals.Signal[session.QueryStartedEvent]
	onQueryEnded   signals.Signal[session.QueryEndedEvent]
}

func NewAtomicSession(ctx context.Context, tx pgx.Tx, identityMap *identitymap.IdentityMap, parent session.Session) *AtomicSession {
	return &AtomicSession{
		ctx:            ctx,
		tx:             tx,
		parent:         parent,
		identityMap:    identityMap,
		onStarted:      signals.NewSignal[session.SessionScopeStartedEvent](),
		onEnded:        signals.NewSignal[session.SessionScopeEndedEvent](),
		onQueryStarted: signals.NewSignal[session.QueryStartedEvent](),
		onQueryEnded:   signals.NewSignal[session.QueryEndedEvent](),
	}
}

func (s *AtomicSession) Context() context.Context {
	return s.ctx
}

func (s *AtomicSession) Connection() session.DbConnection {
	return &connection{
		ctx:            s.ctx,
		exec:           s.tx,
		dbSession:      s,
		onQueryStarted: s.onQueryStarted,
		onQueryEnded:   s.onQueryEnded,
	}
}

func (s *AtomicSession) IdentityMap() *identitymap.IdentityMap {
	return s.identityMap
}

func (s *AtomicSession) OnAtomicStarted() signals.Signal[session.SessionScopeStartedEvent] {
	return s.onStarted
}

func (s *AtomicSession) OnAtomicEnded() signals.Signal[session.SessionScopeEndedEvent] {
	return s.onEnded
}

func (s *AtomicSession) OnQueryStarted() signals.Signal[session.QueryStartedEvent] {
	return s.onQueryStarted
}

func (s *AtomicSession) OnQueryEnded() signals.Signal[session.QueryEndedEvent] {
	return s.onQueryEnded
}

func (s *AtomicSession) Atomic(callback session.SessionCallback) error {
	nestedTx, err := s.tx.Begin(s.ctx)
	if err != nil {
		return errors.Wrap(err, "unable to start savepoint")
	}

	atomicSession := NewAtomicSession(s.ctx, nestedTx, s.identityMap, s)

	if err := s.onStarted.Notify(session.SessionScopeStartedEvent{Session: atomicSession}); err != nil {
		if txErr := nestedTx.Rollback(s.ctx); txErr != nil {
			return multierror.Append(err, txErr)
		}
		return err
	}

	err = callback(atomicSession)

	if endedErr := s.onEnded.Notify(session.SessionScopeEndedEvent{Session: atomicSession}); err == nil {
		err = endedErr
	}

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
	ctx            context.Context
	exec           executor
	dbSession      session.DbSession
	onQueryStarted signals.Signal[session.QueryStartedEvent]
	onQueryEnded   signals.Signal[session.QueryEndedEvent]
}

func (c *connection) notifyQueryStarted(query string, args []any) error {
	return c.onQueryStarted.Notify(session.QueryStartedEvent{
		Query:   query,
		Params:  args,
		Sender:  c,
		Session: c.dbSession,
	})
}

func (c *connection) notifyQueryEnded(query string, args []any, responseTime time.Duration) error {
	return c.onQueryEnded.Notify(session.QueryEndedEvent{
		Query:        query,
		Params:       args,
		Sender:       c,
		Session:      c.dbSession,
		ResponseTime: responseTime,
	})
}

func (c *connection) Exec(query string, args ...any) (session.Result, error) {
	if err := c.notifyQueryStarted(query, args); err != nil {
		return nil, err
	}

	start := time.Now()

	var r session.Result
	var err error
	if utils.IsAutoincrementInsertQuery(query) {
		r, err = c.insert(query, args...)
	} else {
		var tag pgconn.CommandTag
		tag, err = c.exec.Exec(c.ctx, query, args...)
		if err == nil {
			r = result.NewResult(0, tag.RowsAffected())
		}
	}

	if endErr := c.notifyQueryEnded(query, args, time.Since(start)); endErr != nil && err == nil {
		return r, endErr
	}

	return r, err
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
	if err := c.notifyQueryStarted(query, args); err != nil {
		return nil, err
	}

	start := time.Now()

	rows, err := c.exec.Query(c.ctx, query, args...)

	if endErr := c.notifyQueryEnded(query, args, time.Since(start)); endErr != nil && err == nil {
		if rows != nil {
			rows.Close()
		}
		return nil, endErr
	}

	if err != nil {
		return nil, err
	}
	return &rowsAdapter{rows: rows}, nil
}

func (c *connection) QueryRow(query string, args ...any) session.Row {
	if err := c.notifyQueryStarted(query, args); err != nil {
		return &errorRow{err: err}
	}

	start := time.Now()
	row := c.exec.QueryRow(c.ctx, query, args...)
	responseTime := time.Since(start)

	if err := c.notifyQueryEnded(query, args, responseTime); err != nil {
		return &errorRow{err: err}
	}

	return &rowAdapter{row: row}
}
