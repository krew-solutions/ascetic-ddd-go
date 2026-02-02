package result

import (
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/deferred"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
)

func NewResult(lastInsertId, rowsAffected int64) *DeferredResultImp {
	r := &DeferredResultImp{}
	r.Resolve(lastInsertId, rowsAffected)
	return r
}

func NewDeferredResult() *DeferredResultImp {
	return &DeferredResultImp{}
}

type DeferredResultImp struct {
	ResultImp
	deferred.DeferredImp[session.Result]
}

func (r *DeferredResultImp) Resolve(lastInsertId, rowsAffected int64) {
	r.ResultImp = ResultImp{lastInsertId, rowsAffected}
	r.DeferredImp.Resolve(r)
}
