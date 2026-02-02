package batch

import (
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/deferred"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

type DeferredDbExecutor interface {
	Exec(query string, args ...any) (deferred.Deferred[session.Result], error)
}

type DeferredDbQuerier interface {
	Query(query string, args ...any) (deferred.Deferred[session.Rows], error)
}

type DeferredDbSingleQuerier interface {
	QueryRow(query string, args ...any) deferred.Deferred[session.Row]
}

type DeferredDbConnection interface {
	DeferredDbExecutor
	DeferredDbQuerier
	DeferredDbSingleQuerier
}

type DeferredDbSession interface {
	session.Session
	Connection() DeferredDbConnection
}
