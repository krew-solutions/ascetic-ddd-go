package session

import (
	"context"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/deferred"
)

type SessionCallback func(Session) error

type Session interface {
	Atomic(SessionCallback) error
}

type SessionContextCallback func(SessionContext) error

type SessionContext interface {
	context.Context
	Atomic(SessionContextCallback) error
}

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

// Db

type DbSessionExecutor interface {
	Exec(query string, args ...any) (Result, error)
}

type DbSessionQuerier interface {
	Query(query string, args ...any) (Rows, error)
}

type DbSessionSingleQuerier interface {
	QueryRow(query string, args ...any) Row
}

type DbSession interface {
	Session
	DbSessionExecutor
	DbSessionQuerier
	DbSessionSingleQuerier
}

type QueryEvaluator interface {
	Evaluate(s DbSession) (Result, error)
}

type EventSourcedQueryEvaluator interface {
	QueryEvaluator
	SetStreamType(string)
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

type DeferredDbSessionExecutor interface {
	Exec(query string, args ...any) (DeferredResult, error)
}

type DeferredDbSessionQuerier interface {
	Query(query string, args ...any) (DeferredRows, error)
}

type DeferredDbSessionSingleQuerier interface {
	QueryRow(query string, args ...any) DeferredRow
}

type DeferredDbSession interface {
	DeferredDbSessionExecutor
	DeferredDbSessionQuerier
	DeferredDbSessionSingleQuerier
}
