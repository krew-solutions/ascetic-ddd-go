package testutils

import (
	"context"
	"database/sql"
	"errors"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/identitymap"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/result"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/signals"
)

func NewDbSessionStub(rows *RowsStub) *DbSessionStub {
	stub := &DbSessionStub{
		Rows:           rows,
		identityMap:    identitymap.New(100, identitymap.ReadUncommitted),
		onStarted:      signals.NewSignal[session.SessionScopeStartedEvent](),
		onEnded:        signals.NewSignal[session.SessionScopeEndedEvent](),
		onQueryStarted: signals.NewSignal[session.QueryStartedEvent](),
		onQueryEnded:   signals.NewSignal[session.QueryEndedEvent](),
	}
	stub.conn = &connectionStub{session: stub}
	return stub
}

type DbSessionStub struct {
	Rows           *RowsStub
	ActualQuery    string
	ActualParams   []any
	conn           *connectionStub
	identityMap    *identitymap.IdentityMap
	onStarted      signals.Signal[session.SessionScopeStartedEvent]
	onEnded        signals.Signal[session.SessionScopeEndedEvent]
	onQueryStarted signals.Signal[session.QueryStartedEvent]
	onQueryEnded   signals.Signal[session.QueryEndedEvent]
}

func (s *DbSessionStub) Context() context.Context {
	return context.Background()
}

func (s *DbSessionStub) Atomic(callback session.SessionCallback) error {
	return callback(s)
}

func (s *DbSessionStub) Connection() session.DbConnection {
	return s.conn
}

func (s *DbSessionStub) IdentityMap() *identitymap.IdentityMap {
	return s.identityMap
}

func (s *DbSessionStub) OnStarted() signals.Signal[session.SessionScopeStartedEvent] {
	return s.onStarted
}

func (s *DbSessionStub) OnEnded() signals.Signal[session.SessionScopeEndedEvent] {
	return s.onEnded
}

func (s *DbSessionStub) OnQueryStarted() signals.Signal[session.QueryStartedEvent] {
	return s.onQueryStarted
}

func (s *DbSessionStub) OnQueryEnded() signals.Signal[session.QueryEndedEvent] {
	return s.onQueryEnded
}

type connectionStub struct {
	session *DbSessionStub
}

func (c *connectionStub) Exec(query string, args ...any) (session.Result, error) {
	c.session.ActualQuery = query
	c.session.ActualParams = args
	return result.NewResult(0, 0), nil
}

func (c *connectionStub) Query(query string, args ...any) (session.Rows, error) {
	c.session.ActualQuery = query
	c.session.ActualParams = args
	return c.session.Rows, nil
}

func (c *connectionStub) QueryRow(query string, args ...any) session.Row {
	c.session.ActualQuery = query
	c.session.ActualParams = args
	return &RowStub{rows: c.session.Rows}
}

func NewRowsStub(rows ...[]any) *RowsStub {
	return &RowsStub{
		rows:   rows,
		idx:    -1,
		Closed: false,
	}
}

type RowsStub struct {
	rows   [][]any
	idx    int
	Closed bool
}

func (r *RowsStub) Close() error {
	r.Closed = true
	return nil
}

func (r *RowsStub) Err() error {
	return nil
}

func (r *RowsStub) Next() bool {
	r.idx++
	return r.idx < len(r.rows)
}

func (r *RowsStub) Scan(dest ...any) error {
	if r.idx < 0 || r.idx >= len(r.rows) {
		return errors.New("no current row")
	}

	row := r.rows[r.idx]
	for i, val := range row {
		if i >= len(dest) {
			break
		}

		switch d := dest[i].(type) {
		case *int:
			*d = toInt(val)
		case *int64:
			*d = toInt64(val)
		case *int32:
			*d = toInt32(val)
		case *string:
			*d = val.(string)
		case *bool:
			*d = val.(bool)
		case *[]byte:
			*d = val.([]byte)
		case *float64:
			*d = toFloat64(val)
		case *float32:
			*d = toFloat32(val)
		case sql.Scanner:
			if err := d.Scan(val); err != nil {
				return err
			}
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}

func toInt(val any) int {
	switch v := val.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case int32:
		return int(v)
	default:
		panic("cannot convert to int")
	}
}

func toInt64(val any) int64 {
	switch v := val.(type) {
	case int:
		return int64(v)
	case int64:
		return v
	case int32:
		return int64(v)
	default:
		panic("cannot convert to int64")
	}
}

func toInt32(val any) int32 {
	switch v := val.(type) {
	case int:
		return int32(v)
	case int64:
		return int32(v)
	case int32:
		return v
	default:
		panic("cannot convert to int32")
	}
}

func toFloat64(val any) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	default:
		panic("cannot convert to float64")
	}
}

func toFloat32(val any) float32 {
	switch v := val.(type) {
	case float32:
		return v
	case float64:
		return float32(v)
	case int:
		return float32(v)
	case int64:
		return float32(v)
	default:
		panic("cannot convert to float32")
	}
}

type RowStub struct {
	rows *RowsStub
}

func (r *RowStub) Err() error {
	return r.rows.Err()
}

func (r *RowStub) Scan(dest ...any) error {
	return r.rows.Scan(dest...)
}
