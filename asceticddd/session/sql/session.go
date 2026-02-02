package sql

import (
	"database/sql"

	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/result"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/utils"
)

func NewSession(db *sql.DB) *Session {
	return &Session{
		db:         db,
		dbExecutor: db,
	}
}

type Session struct {
	db         *sql.DB
	dbExecutor DbExecutor
}

func (s *Session) Atomic(callback session.SessionCallback) error {
	// TODO: Add support for SavePoint:
	// https://github.com/golang/go/issues/7898#issuecomment-580080390
	if s.db == nil {
		return errors.New("savePoint is not currently supported")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return errors.Wrap(err, "unable to start transaction")
	}
	newSession := &Session{
		dbExecutor: tx,
	}
	err = callback(newSession)
	if err != nil {
		if txErr := tx.Rollback(); txErr != nil {
			return multierror.Append(err, txErr)
		}
		return err
	}
	if txErr := tx.Commit(); txErr != nil {
		return errors.Wrap(err, "failed to commit tx")
	}
	return nil
}

func (s *Session) Exec(query string, args ...any) (session.Result, error) {
	if utils.IsAutoincrementInsertQuery(query) {
		return s.insert(query, args...)
	}
	return s.dbExecutor.Exec(query, args...)
}

func (s *Session) insert(query string, args ...any) (session.Result, error) {
	var id int64
	err := s.dbExecutor.QueryRow(query, args...).Scan(&id)
	if err != nil {
		return nil, err
	}
	return result.NewResult(id, 0), nil
}

func (s *Session) Query(query string, args ...any) (session.Rows, error) {
	return s.dbExecutor.Query(query, args...)
}

func (s *Session) QueryRow(query string, args ...any) session.Row {
	return s.dbExecutor.QueryRow(query, args...)
}

type DbExecutor interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}
