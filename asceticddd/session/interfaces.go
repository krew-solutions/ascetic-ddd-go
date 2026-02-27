package session

import (
	"context"
	"net/http"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/identitymap"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/signals"
)

type SessionCallback func(Session) error

type Session interface {
	Context() context.Context
	Atomic(SessionCallback) error
	OnAtomicStarted() signals.Signal[SessionScopeStartedEvent]
	OnAtomicEnded() signals.Signal[SessionScopeEndedEvent]
}

type SessionPoolCallback func(Session) error

type SessionPool interface {
	Session(context.Context, SessionPoolCallback) error
	OnSessionStarted() signals.Signal[SessionScopeStartedEvent]
	OnSessionEnded() signals.Signal[SessionScopeEndedEvent]
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
	IdentityMap() *identitymap.IdentityMap
	OnQueryStarted() signals.Signal[QueryStartedEvent]
	OnQueryEnded() signals.Signal[QueryEndedEvent]
}

type RestSession interface {
	Session
	HttpClient() *http.Client
	IdentityMap() *identitymap.IdentityMap
	OnRequestStarted() signals.Signal[RequestStartedEvent]
	OnRequestEnded() signals.Signal[RequestEndedEvent]
}

type QueryEvaluator interface {
	Evaluate(s Session) (Result, error)
}
