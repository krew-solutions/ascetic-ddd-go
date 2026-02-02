package session

import (
	"context"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/deferred"
)

type SessionCallback func(Session) error

type Session interface {
	Context() context.Context
	Atomic(SessionCallback) error
}

type SessionPoolCallback func(SessionContext) error

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
	DbSessionExecutor
	DbSessionQuerier
	DbSessionSingleQuerier
}

type DbSession interface {
	Session
	Connection() DbConnection
}

type QueryEvaluator interface {
	Evaluate(s DbSession) (Result, error)
}

// Deferred

type DeferredResult interface {
	deferred.Deferred[Result]
}

type DeferredRows interface {
	deferred.Deferred[Rows]
}

type DeferredRow interface {
	deferred.Deferred[Row]
}

type DeferredDbExecutor interface {
	Exec(query string, args ...any) (DeferredResult, error)
}

type DeferredDbQuerier interface {
	Query(query string, args ...any) (DeferredRows, error)
}

type DeferredDbSingleQuerier interface {
	QueryRow(query string, args ...any) DeferredRow
}

type DeferredDbConnection interface {
	DeferredDbSessionExecutor
	DeferredDbSessionQuerier
	DeferredDbSessionSingleQuerier
}

type DeferredDbSession interface {
	Session
	Connection() Connection
}
