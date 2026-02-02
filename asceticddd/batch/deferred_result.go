package batch

import (
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/deferred"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/result"
)

func NewDeferredResult() *DeferredResultImp {
	return &DeferredResultImp{}
}

type DeferredResultImp struct {
	result.ResultImp
	deferred.DeferredImp[session.Result]
}

func (r *DeferredResultImp) Resolve(lastInsertId, rowsAffected int64) {
	r.ResultImp = result.NewResult(lastInsertId, rowsAffected)
	r.DeferredImp.Resolve(r)
}
