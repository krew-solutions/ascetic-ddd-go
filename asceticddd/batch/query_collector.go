package batch

import (
	"context"
	"errors"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session/result"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/utils"
)

func NewQueryCollector() *QueryCollector {
	r := &QueryCollector{
		multiQueryMap: make(map[string]MultiQuerier),
	}
	r.conn = &connectionCollector{collector: r}
	return r
}

type MultiQuerier interface {
	session.QueryEvaluator
	Exec(query string, args ...any) (session.DeferredResult, error)
}

type QueryCollector struct {
	multiQueryMap map[string]MultiQuerier
	conn          *connectionCollector
}

func (c *QueryCollector) Context() context.Context {
	return context.Background()
}

func (c *QueryCollector) Atomic(callback session.SessionCallback) error {
	return callback(c)
}

func (c *QueryCollector) Connection() session.DeferredDbConnection {
	return c.conn
}

func (c *QueryCollector) collectQuery(query string, args ...any) (session.DeferredResult, error) {
	if _, found := c.multiQueryMap[query]; !found {
		if utils.IsAutoincrementInsertQuery(query) {
			c.multiQueryMap[query] = NewAutoincrementMultiInsertQuery()
		} else if utils.IsInsertQuery(query) {
			c.multiQueryMap[query] = NewMultiInsertQuery()
		}
	}
	if multiQuery, found := c.multiQueryMap[query]; found {
		return multiQuery.Exec(query, args...)
	}
	return nil, errors.New("unknown SQL command")
}

func (c *QueryCollector) Evaluate(s session.DbSession) (session.Result, error) {
	var rowsAffected int64
	for len(c.multiQueryMap) > 0 {
		// Resolve N+1 query problem with auto-increment primary key.
		// Nested queries have got the lastInsertId and can be handled for now
		currentMultiQueryMap := c.multiQueryMap
		c.multiQueryMap = make(map[string]MultiQuerier)
		for k := range currentMultiQueryMap {
			r, err := currentMultiQueryMap[k].Evaluate(s)
			if err != nil {
				return nil, err
			}
			rowsAffectedIncrement, err := r.RowsAffected()
			if err == nil {
				rowsAffected += rowsAffectedIncrement
			}
		}
	}
	return result.NewResult(0, rowsAffected), nil
}

type connectionCollector struct {
	collector *QueryCollector
}

func (c *connectionCollector) Exec(query string, args ...any) (session.DeferredResult, error) {
	return c.collector.collectQuery(query, args...)
}

func (c *connectionCollector) Query(query string, args ...any) (session.DeferredRows, error) {
	return nil, errors.New("Query not supported in batch collector")
}

func (c *connectionCollector) QueryRow(query string, args ...any) session.DeferredRow {
	return nil
}
