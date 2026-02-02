package testutils

import (
	"context"
	"database/sql"
	"errors"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/result"
)

func NewDbSessionStub(rows *RowsStub) *DbSessionStub {
	stub := &DbSessionStub{
		Rows: rows,
	}
	stub.conn = &connectionStub{session: stub}
	return stub
}

type DbSessionStub struct {
	Rows         *RowsStub
	ActualQuery  string
	ActualParams []any
	conn         *connectionStub
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

type connectionStub struct {
	session *DbSessionStub
}

func (c *connectionStub) Exec(query string, args ...any) (session.Result, error) {
	c.session.ActualQuery = query
	c.session.ActualParams = args
	return result.NewDeferredResult(), nil
}

func (c *connectionStub) Query(query string, args ...any) (session.Rows, error) {
	c.session.ActualQuery = query
	c.session.ActualParams = args
	return c.session.Rows, nil
}

func (c *connectionStub) QueryRow(query string, args ...any) session.Row {
	c.session.ActualQuery = query
	c.session.ActualParams = args
	return c.session.Rows
}

func NewRowsStub(rows ...[]any) *RowsStub {
	return &RowsStub{
		rows, 0, false,
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

func (r RowsStub) Err() error {
	return nil
}

func (r *RowsStub) Next() bool {
	r.idx++
	return len(r.rows) < r.idx
}

func (r RowsStub) Scan(dest ...any) error {
	for i, d := range dest {
		dt, ok := d.(sql.Scanner)
		if !ok {
			return errors.New("value should implement sql.Scanner interface")
		}
		err := dt.Scan(r.rows[r.idx][i])
		if err != nil {
			return err
		}
	}
	return nil
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
