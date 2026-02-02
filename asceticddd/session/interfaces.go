package session

import (
	"context"
)

type SessionCallback func(Session) error

type Session interface {
	Context() context.Context
	Atomic(SessionCallback) error
}

type SessionPoolCallback func(Session) error

type SessionPool interface {
	Session(context.Context, SessionPoolCallback) error
}

// Db

type Result interface {
	LastInsertId() (int64, error)
	RowsAffected() (int64, error)
}

type Rows interface {
	Close() error
	Err() error
	Next() bool
	Scan(dest ...any) error
}

type Row interface {
	Err() error
	Scan(dest ...any) error
}

type DbExecutor interface {
	Exec(query string, args ...any) (Result, error)
}

type DbQuerier interface {
	Query(query string, args ...any) (Rows, error)
}

type DbSingleQuerier interface {
	QueryRow(query string, args ...any) Row
}

type DbConnection interface {
	DbExecutor
	DbQuerier
	DbSingleQuerier
}

type DbSession interface {
	Session
	Connection() DbConnection
}

type QueryEvaluator interface {
	Evaluate(s DbSession) (Result, error)
}
