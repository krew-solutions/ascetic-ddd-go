package query

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/signals"
)

// =============================================================================
// Test Fixtures
// =============================================================================

type mockSession struct{}

func (m *mockSession) Context() context.Context                                  { return context.Background() }
func (m *mockSession) Atomic(cb session.SessionCallback) error                   { return cb(m) }
func (m *mockSession) OnAtomicStarted() signals.Signal[session.SessionScopeStartedEvent] { return nil }
func (m *mockSession) OnAtomicEnded() signals.Signal[session.SessionScopeEndedEvent]     { return nil }

type stubObjectResolver struct {
	relations map[string]struct {
		storage  map[any]map[string]any
		resolver IObjectResolver
	}
}

func newStubObjectResolver(relations map[string]struct {
	storage  map[any]map[string]any
	resolver IObjectResolver
}) *stubObjectResolver {
	return &stubObjectResolver{relations: relations}
}

func (r *stubObjectResolver) Resolve(s session.Session, field string, fkValue any) (map[string]any, IObjectResolver, error) {
	info, ok := r.relations[field]
	if !ok {
		return nil, nil, nil
	}
	state, ok := info.storage[fkValue]
	if !ok {
		return nil, nil, nil
	}
	return state, info.resolver, nil
}

type relInfo struct {
	storage  map[any]map[string]any
	resolver IObjectResolver
}

func makeResolver(relations map[string]relInfo) *stubObjectResolver {
	m := make(map[string]struct {
		storage  map[any]map[string]any
		resolver IObjectResolver
	}, len(relations))
	for k, v := range relations {
		m[k] = struct {
			storage  map[any]map[string]any
			resolver IObjectResolver
		}{storage: v.storage, resolver: v.resolver}
	}
	return newStubObjectResolver(m)
}

var sess session.Session = &mockSession{}

// =============================================================================
// EvaluateWalker - Basic (no resolver)
// =============================================================================

func TestEvaluateWalkerBasic(t *testing.T) {
	walker := NewEvaluateWalker(nil)

	t.Run("eq matches", func(t *testing.T) {
		result, err := walker.Evaluate(sess, EqOperator{Value: 42}, 42)
		assert.NoError(t, err)
		assert.True(t, result)
	})
	t.Run("eq not matches", func(t *testing.T) {
		result, err := walker.Evaluate(sess, EqOperator{Value: 42}, 99)
		assert.NoError(t, err)
		assert.False(t, result)
	})
	t.Run("eq nil", func(t *testing.T) {
		result, err := walker.Evaluate(sess, EqOperator{Value: nil}, nil)
		assert.NoError(t, err)
		assert.True(t, result)
	})
	t.Run("eq string", func(t *testing.T) {
		result, err := walker.Evaluate(sess, EqOperator{Value: "active"}, "active")
		assert.NoError(t, err)
		assert.True(t, result)
	})
	t.Run("comparison ne", func(t *testing.T) {
		op := ComparisonOperator{Op: "$ne", Value: "deleted"}
		r1, _ := walker.Evaluate(sess, op, "active")
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, op, "deleted")
		assert.False(t, r2)
	})
	t.Run("comparison gt", func(t *testing.T) {
		op := ComparisonOperator{Op: "$gt", Value: 10}
		r1, _ := walker.Evaluate(sess, op, 15)
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, op, 10)
		assert.False(t, r2)
		r3, _ := walker.Evaluate(sess, op, 5)
		assert.False(t, r3)
	})
	t.Run("comparison gte", func(t *testing.T) {
		op := ComparisonOperator{Op: "$gte", Value: 10}
		r1, _ := walker.Evaluate(sess, op, 15)
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, op, 10)
		assert.True(t, r2)
		r3, _ := walker.Evaluate(sess, op, 5)
		assert.False(t, r3)
	})
	t.Run("comparison lt", func(t *testing.T) {
		op := ComparisonOperator{Op: "$lt", Value: 10}
		r1, _ := walker.Evaluate(sess, op, 5)
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, op, 10)
		assert.False(t, r2)
		r3, _ := walker.Evaluate(sess, op, 15)
		assert.False(t, r3)
	})
	t.Run("comparison lte", func(t *testing.T) {
		op := ComparisonOperator{Op: "$lte", Value: 10}
		r1, _ := walker.Evaluate(sess, op, 5)
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, op, 10)
		assert.True(t, r2)
		r3, _ := walker.Evaluate(sess, op, 15)
		assert.False(t, r3)
	})
	t.Run("in matches", func(t *testing.T) {
		op := InOperator{Values: []any{"active", "pending"}}
		r1, _ := walker.Evaluate(sess, op, "active")
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, op, "pending")
		assert.True(t, r2)
	})
	t.Run("in not matches", func(t *testing.T) {
		op := InOperator{Values: []any{"active", "pending"}}
		r, _ := walker.Evaluate(sess, op, "deleted")
		assert.False(t, r)
	})
	t.Run("is_null true matches nil", func(t *testing.T) {
		r, _ := walker.Evaluate(sess, IsNullOperator{Value: true}, nil)
		assert.True(t, r)
	})
	t.Run("is_null true not matches value", func(t *testing.T) {
		r, _ := walker.Evaluate(sess, IsNullOperator{Value: true}, 42)
		assert.False(t, r)
	})
	t.Run("is_null false matches value", func(t *testing.T) {
		r, _ := walker.Evaluate(sess, IsNullOperator{Value: false}, 42)
		assert.True(t, r)
	})
	t.Run("is_null false not matches nil", func(t *testing.T) {
		r, _ := walker.Evaluate(sess, IsNullOperator{Value: false}, nil)
		assert.False(t, r)
	})
	t.Run("is_null in composite", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{"name": IsNullOperator{Value: true}}}
		r1, _ := walker.Evaluate(sess, query, map[string]any{"name": nil})
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, query, map[string]any{"name": "Alice"})
		assert.False(t, r2)
	})
	t.Run("and all true", func(t *testing.T) {
		op := AndOperator{Operands: []IQueryOperator{
			ComparisonOperator{Op: "$gt", Value: 5},
			ComparisonOperator{Op: "$lt", Value: 10},
		}}
		r, _ := walker.Evaluate(sess, op, 7)
		assert.True(t, r)
	})
	t.Run("and one false", func(t *testing.T) {
		op := AndOperator{Operands: []IQueryOperator{
			ComparisonOperator{Op: "$gt", Value: 5},
			ComparisonOperator{Op: "$lt", Value: 10},
		}}
		r, _ := walker.Evaluate(sess, op, 12)
		assert.False(t, r)
	})
	t.Run("or one true", func(t *testing.T) {
		op := OrOperator{Operands: []IQueryOperator{
			EqOperator{Value: "active"},
			EqOperator{Value: "pending"},
		}}
		r, _ := walker.Evaluate(sess, op, "pending")
		assert.True(t, r)
	})
	t.Run("or none true", func(t *testing.T) {
		op := OrOperator{Operands: []IQueryOperator{
			EqOperator{Value: "active"},
			EqOperator{Value: "pending"},
		}}
		r, _ := walker.Evaluate(sess, op, "deleted")
		assert.False(t, r)
	})
	t.Run("composite matches", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": EqOperator{Value: "active"},
			"name":   EqOperator{Value: "Alice"},
		}}
		r, _ := walker.Evaluate(sess, query, map[string]any{"status": "active", "name": "Alice", "extra": "ignored"})
		assert.True(t, r)
	})
	t.Run("composite not matches", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": EqOperator{Value: "active"},
			"name":   EqOperator{Value: "Alice"},
		}}
		r, _ := walker.Evaluate(sess, query, map[string]any{"status": "inactive", "name": "Alice"})
		assert.False(t, r)
	})
	t.Run("composite non dict state", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}
		r, _ := walker.Evaluate(sess, query, 42)
		assert.False(t, r)
	})
	t.Run("nested composite without resolver", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"address": CompositeQuery{Fields: map[string]IQueryOperator{
				"city": EqOperator{Value: "Moscow"},
			}},
		}}
		r1, _ := walker.Evaluate(sess, query, map[string]any{"address": map[string]any{"city": "Moscow", "street": "Main"}})
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, query, map[string]any{"address": map[string]any{"city": "London"}})
		assert.False(t, r2)
	})
	t.Run("composite with comparison", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name": EqOperator{Value: "John"},
			"age": AndOperator{Operands: []IQueryOperator{
				ComparisonOperator{Op: "$gte", Value: 18},
				ComparisonOperator{Op: "$lt", Value: 65},
			}},
		}}
		r1, _ := walker.Evaluate(sess, query, map[string]any{"name": "John", "age": 30})
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, query, map[string]any{"name": "John", "age": 15})
		assert.False(t, r2)
	})
	t.Run("composite with or", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": OrOperator{Operands: []IQueryOperator{
				EqOperator{Value: "active"},
				EqOperator{Value: "pending"},
			}},
		}}
		r1, _ := walker.Evaluate(sess, query, map[string]any{"status": "active"})
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, query, map[string]any{"status": "pending"})
		assert.True(t, r2)
		r3, _ := walker.Evaluate(sess, query, map[string]any{"status": "deleted"})
		assert.False(t, r3)
	})
	t.Run("composite with in", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": InOperator{Values: []any{"active", "pending"}},
		}}
		r1, _ := walker.Evaluate(sess, query, map[string]any{"status": "active"})
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, query, map[string]any{"status": "deleted"})
		assert.False(t, r2)
	})
	t.Run("parsed simple pattern", func(t *testing.T) {
		query, _ := QueryParser{}.Parse(map[string]any{"status": "active"})
		r1, _ := walker.Evaluate(sess, query, map[string]any{"status": "active", "name": "test"})
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, query, map[string]any{"status": "inactive", "name": "test"})
		assert.False(t, r2)
	})
	t.Run("rel without resolver delegates to inner", func(t *testing.T) {
		query := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"name": EqOperator{Value: "Active"}}}}
		r1, _ := walker.Evaluate(sess, query, map[string]any{"name": "Active"})
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, query, map[string]any{"name": "Inactive"})
		assert.False(t, r2)
	})
}

// =============================================================================
// EvaluateWalker - Nested Lookup (2 levels)
// =============================================================================

func TestEvaluateWalkerNestedLookup(t *testing.T) {
	statusStorage := map[any]map[string]any{
		"active":   {"id": "active", "name": "Active"},
		"inactive": {"id": "inactive", "name": "Inactive"},
	}
	statusResolver := makeResolver(nil)
	resolver := makeResolver(map[string]relInfo{
		"status_id": {storage: statusStorage, resolver: statusResolver},
	})
	walker := NewEvaluateWalker(resolver)

	t.Run("nested lookup matches", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"name": EqOperator{Value: "Active"},
			}}},
		}}
		r, _ := walker.Evaluate(sess, query, map[string]any{"id": 1, "status_id": "active", "name": "Alice"})
		assert.True(t, r)
	})
	t.Run("nested lookup not matches", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"name": EqOperator{Value: "Active"},
			}}},
		}}
		r, _ := walker.Evaluate(sess, query, map[string]any{"id": 2, "status_id": "inactive", "name": "Bob"})
		assert.False(t, r)
	})
	t.Run("fk is nil", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"name": EqOperator{Value: "Active"},
			}}},
		}}
		r, _ := walker.Evaluate(sess, query, map[string]any{"id": 3, "status_id": nil, "name": "Charlie"})
		assert.False(t, r)
	})
	t.Run("foreign not found", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"name": EqOperator{Value: "Active"},
			}}},
		}}
		r, _ := walker.Evaluate(sess, query, map[string]any{"id": 4, "status_id": "unknown", "name": "Dave"})
		assert.False(t, r)
	})
	t.Run("simple value with nested lookup", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name": EqOperator{Value: "Alice"},
			"status_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"name": EqOperator{Value: "Active"},
			}}},
		}}
		r1, _ := walker.Evaluate(sess, query, map[string]any{"id": 1, "status_id": "active", "name": "Alice"})
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, query, map[string]any{"id": 2, "status_id": "inactive", "name": "Bob"})
		assert.False(t, r2)
	})
	t.Run("nested lookup with comparison", func(t *testing.T) {
		statusStorage["high"] = map[string]any{"id": "high", "name": "High", "priority": 10}
		statusStorage["low"] = map[string]any{"id": "low", "name": "Low", "priority": 1}

		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"priority": ComparisonOperator{Op: "$gte", Value: 5},
			}}},
		}}
		r1, _ := walker.Evaluate(sess, query, map[string]any{"id": 1, "status_id": "high", "name": "Alice"})
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, query, map[string]any{"id": 2, "status_id": "low", "name": "Bob"})
		assert.False(t, r2)
	})
	t.Run("non relation field with resolver", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{"name": EqOperator{Value: "Alice"}}}
		r, _ := walker.Evaluate(sess, query, map[string]any{"id": 1, "status_id": "active", "name": "Alice"})
		assert.True(t, r)
	})
}

// =============================================================================
// EvaluateWalker - Three Table Cascade
// =============================================================================

func TestEvaluateWalkerThreeTableCascade(t *testing.T) {
	countryStorage := map[any]map[string]any{
		"US": {"id": "US", "code": "US", "continent": "America"},
		"UK": {"id": "UK", "code": "UK", "continent": "Europe"},
		"JP": {"id": "JP", "code": "JP", "continent": "Asia"},
	}
	companyStorage := map[any]map[string]any{
		1: {"id": 1, "country_id": "US", "name": "Acme", "type": "tech", "revenue": 2000000},
		2: {"id": 2, "country_id": "UK", "name": "BritCo", "type": "finance", "revenue": 500000},
		3: {"id": 3, "country_id": "JP", "name": "TokyoTech", "type": "tech", "revenue": 800000},
	}

	countryResolver := makeResolver(nil)
	companyResolver := makeResolver(map[string]relInfo{
		"country_id": {storage: countryStorage, resolver: countryResolver},
	})
	resolver := makeResolver(map[string]relInfo{
		"company_id": {storage: companyStorage, resolver: companyResolver},
	})
	walker := NewEvaluateWalker(resolver)

	t.Run("three table cascade matches", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name":   EqOperator{Value: "John"},
			"status": EqOperator{Value: "active"},
			"company_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"type": EqOperator{Value: "tech"},
				"country_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
					"code": EqOperator{Value: "US"},
				}}},
			}}},
		}}
		r, _ := walker.Evaluate(sess, query, map[string]any{
			"id": 1, "company_id": 1, "name": "John", "age": 30, "status": "active",
		})
		assert.True(t, r)
	})
	t.Run("cascade not matches middle", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"company_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"type": EqOperator{Value: "tech"},
			}}},
		}}
		r, _ := walker.Evaluate(sess, query, map[string]any{
			"id": 2, "company_id": 2, "name": "Jane", "age": 25, "status": "active",
		})
		assert.False(t, r)
	})
	t.Run("cascade not matches deepest", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"company_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"country_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
					"code": EqOperator{Value: "UK"},
				}}},
			}}},
		}}
		r1, _ := walker.Evaluate(sess, query, map[string]any{
			"id": 1, "company_id": 1, "name": "John", "age": 30, "status": "active",
		})
		assert.False(t, r1)
		r2, _ := walker.Evaluate(sess, query, map[string]any{
			"id": 2, "company_id": 2, "name": "Jane", "age": 25, "status": "active",
		})
		assert.True(t, r2)
	})
	t.Run("or in cascade", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"company_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"country_id": OrOperator{Operands: []IQueryOperator{
					RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"code": EqOperator{Value: "US"}}}},
					RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"code": EqOperator{Value: "UK"}}}},
				}},
			}}},
		}}
		r1, _ := walker.Evaluate(sess, query, map[string]any{"id": 1, "company_id": 1, "name": "John", "age": 30, "status": "active"})
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, query, map[string]any{"id": 2, "company_id": 2, "name": "Jane", "age": 25, "status": "active"})
		assert.True(t, r2)
		r3, _ := walker.Evaluate(sess, query, map[string]any{"id": 3, "company_id": 3, "name": "Yuki", "age": 28, "status": "active"})
		assert.False(t, r3)
	})
	t.Run("cascade with all operators", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name": EqOperator{Value: "John"},
			"age":  ComparisonOperator{Op: "$gt", Value: 25},
			"company_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"type":    EqOperator{Value: "tech"},
				"revenue": ComparisonOperator{Op: "$gte", Value: 1000000},
				"country_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
					"code": EqOperator{Value: "US"},
				}}},
			}}},
		}}
		r1, _ := walker.Evaluate(sess, query, map[string]any{
			"id": 1, "company_id": 1, "name": "John", "age": 30, "status": "active",
		})
		assert.True(t, r1)
		r2, _ := walker.Evaluate(sess, query, map[string]any{
			"id": 4, "company_id": 3, "name": "John", "age": 30, "status": "active",
		})
		assert.False(t, r2)
	})
	t.Run("cascade company not found", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"company_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"type": EqOperator{Value: "tech"},
			}}},
		}}
		r, _ := walker.Evaluate(sess, query, map[string]any{
			"id": 5, "company_id": 999, "name": "Ghost", "age": 0, "status": "unknown",
		})
		assert.False(t, r)
	})
}

// =============================================================================
// EvaluateWalker - Sync
// =============================================================================

func TestEvaluateWalkerSync(t *testing.T) {
	walker := NewEvaluateWalker(nil)

	t.Run("eq matches", func(t *testing.T) {
		r, _ := walker.EvaluateSync(EqOperator{Value: 42}, 42)
		assert.True(t, r)
	})
	t.Run("eq not matches", func(t *testing.T) {
		r, _ := walker.EvaluateSync(EqOperator{Value: 42}, 99)
		assert.False(t, r)
	})
	t.Run("eq nil", func(t *testing.T) {
		r, _ := walker.EvaluateSync(EqOperator{Value: nil}, nil)
		assert.True(t, r)
	})
	t.Run("eq string", func(t *testing.T) {
		r, _ := walker.EvaluateSync(EqOperator{Value: "active"}, "active")
		assert.True(t, r)
	})
	t.Run("comparison ne", func(t *testing.T) {
		op := ComparisonOperator{Op: "$ne", Value: "deleted"}
		r1, _ := walker.EvaluateSync(op, "active")
		assert.True(t, r1)
		r2, _ := walker.EvaluateSync(op, "deleted")
		assert.False(t, r2)
	})
	t.Run("comparison gt", func(t *testing.T) {
		op := ComparisonOperator{Op: "$gt", Value: 10}
		r1, _ := walker.EvaluateSync(op, 15)
		assert.True(t, r1)
		r2, _ := walker.EvaluateSync(op, 10)
		assert.False(t, r2)
		r3, _ := walker.EvaluateSync(op, 5)
		assert.False(t, r3)
	})
	t.Run("comparison gte", func(t *testing.T) {
		op := ComparisonOperator{Op: "$gte", Value: 10}
		r1, _ := walker.EvaluateSync(op, 15)
		assert.True(t, r1)
		r2, _ := walker.EvaluateSync(op, 10)
		assert.True(t, r2)
		r3, _ := walker.EvaluateSync(op, 5)
		assert.False(t, r3)
	})
	t.Run("comparison lt", func(t *testing.T) {
		op := ComparisonOperator{Op: "$lt", Value: 10}
		r1, _ := walker.EvaluateSync(op, 5)
		assert.True(t, r1)
		r2, _ := walker.EvaluateSync(op, 10)
		assert.False(t, r2)
		r3, _ := walker.EvaluateSync(op, 15)
		assert.False(t, r3)
	})
	t.Run("comparison lte", func(t *testing.T) {
		op := ComparisonOperator{Op: "$lte", Value: 10}
		r1, _ := walker.EvaluateSync(op, 5)
		assert.True(t, r1)
		r2, _ := walker.EvaluateSync(op, 10)
		assert.True(t, r2)
		r3, _ := walker.EvaluateSync(op, 15)
		assert.False(t, r3)
	})
	t.Run("in matches", func(t *testing.T) {
		op := InOperator{Values: []any{"active", "pending"}}
		r1, _ := walker.EvaluateSync(op, "active")
		assert.True(t, r1)
		r2, _ := walker.EvaluateSync(op, "pending")
		assert.True(t, r2)
	})
	t.Run("in not matches", func(t *testing.T) {
		op := InOperator{Values: []any{"active", "pending"}}
		r, _ := walker.EvaluateSync(op, "deleted")
		assert.False(t, r)
	})
	t.Run("is_null true matches nil", func(t *testing.T) {
		r, _ := walker.EvaluateSync(IsNullOperator{Value: true}, nil)
		assert.True(t, r)
	})
	t.Run("is_null true not matches value", func(t *testing.T) {
		r, _ := walker.EvaluateSync(IsNullOperator{Value: true}, 42)
		assert.False(t, r)
	})
	t.Run("is_null false matches value", func(t *testing.T) {
		r, _ := walker.EvaluateSync(IsNullOperator{Value: false}, 42)
		assert.True(t, r)
	})
	t.Run("is_null false not matches nil", func(t *testing.T) {
		r, _ := walker.EvaluateSync(IsNullOperator{Value: false}, nil)
		assert.False(t, r)
	})
	t.Run("and all true", func(t *testing.T) {
		op := AndOperator{Operands: []IQueryOperator{
			ComparisonOperator{Op: "$gt", Value: 5},
			ComparisonOperator{Op: "$lt", Value: 10},
		}}
		r, _ := walker.EvaluateSync(op, 7)
		assert.True(t, r)
	})
	t.Run("and one false", func(t *testing.T) {
		op := AndOperator{Operands: []IQueryOperator{
			ComparisonOperator{Op: "$gt", Value: 5},
			ComparisonOperator{Op: "$lt", Value: 10},
		}}
		r, _ := walker.EvaluateSync(op, 12)
		assert.False(t, r)
	})
	t.Run("or one true", func(t *testing.T) {
		op := OrOperator{Operands: []IQueryOperator{
			EqOperator{Value: "active"},
			EqOperator{Value: "pending"},
		}}
		r, _ := walker.EvaluateSync(op, "pending")
		assert.True(t, r)
	})
	t.Run("or none true", func(t *testing.T) {
		op := OrOperator{Operands: []IQueryOperator{
			EqOperator{Value: "active"},
			EqOperator{Value: "pending"},
		}}
		r, _ := walker.EvaluateSync(op, "deleted")
		assert.False(t, r)
	})
	t.Run("composite matches", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": EqOperator{Value: "active"},
			"name":   EqOperator{Value: "Alice"},
		}}
		r, _ := walker.EvaluateSync(query, map[string]any{"status": "active", "name": "Alice", "extra": "ignored"})
		assert.True(t, r)
	})
	t.Run("composite not matches", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": EqOperator{Value: "active"},
			"name":   EqOperator{Value: "Alice"},
		}}
		r, _ := walker.EvaluateSync(query, map[string]any{"status": "inactive", "name": "Alice"})
		assert.False(t, r)
	})
	t.Run("composite non dict state", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}
		r, _ := walker.EvaluateSync(query, 42)
		assert.False(t, r)
	})
	t.Run("nested composite", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"address": CompositeQuery{Fields: map[string]IQueryOperator{
				"city": EqOperator{Value: "Moscow"},
			}},
		}}
		r1, _ := walker.EvaluateSync(query, map[string]any{"address": map[string]any{"city": "Moscow", "street": "Main"}})
		assert.True(t, r1)
		r2, _ := walker.EvaluateSync(query, map[string]any{"address": map[string]any{"city": "London"}})
		assert.False(t, r2)
	})
	t.Run("composite with comparison", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name": EqOperator{Value: "John"},
			"age": AndOperator{Operands: []IQueryOperator{
				ComparisonOperator{Op: "$gte", Value: 18},
				ComparisonOperator{Op: "$lt", Value: 65},
			}},
		}}
		r1, _ := walker.EvaluateSync(query, map[string]any{"name": "John", "age": 30})
		assert.True(t, r1)
		r2, _ := walker.EvaluateSync(query, map[string]any{"name": "John", "age": 15})
		assert.False(t, r2)
	})
	t.Run("composite with or", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": OrOperator{Operands: []IQueryOperator{
				EqOperator{Value: "active"},
				EqOperator{Value: "pending"},
			}},
		}}
		r1, _ := walker.EvaluateSync(query, map[string]any{"status": "active"})
		assert.True(t, r1)
		r2, _ := walker.EvaluateSync(query, map[string]any{"status": "pending"})
		assert.True(t, r2)
		r3, _ := walker.EvaluateSync(query, map[string]any{"status": "deleted"})
		assert.False(t, r3)
	})
	t.Run("composite with in", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": InOperator{Values: []any{"active", "pending"}},
		}}
		r1, _ := walker.EvaluateSync(query, map[string]any{"status": "active"})
		assert.True(t, r1)
		r2, _ := walker.EvaluateSync(query, map[string]any{"status": "deleted"})
		assert.False(t, r2)
	})
	t.Run("parsed simple pattern", func(t *testing.T) {
		query, _ := QueryParser{}.Parse(map[string]any{"status": "active"})
		r1, _ := walker.EvaluateSync(query, map[string]any{"status": "active", "name": "test"})
		assert.True(t, r1)
		r2, _ := walker.EvaluateSync(query, map[string]any{"status": "inactive", "name": "test"})
		assert.False(t, r2)
	})
	t.Run("rel delegates to inner", func(t *testing.T) {
		query := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"name": EqOperator{Value: "Active"}}}}
		r1, _ := walker.EvaluateSync(query, map[string]any{"name": "Active"})
		assert.True(t, r1)
		r2, _ := walker.EvaluateSync(query, map[string]any{"name": "Inactive"})
		assert.False(t, r2)
	})
	t.Run("partial criteria matches full state", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"id": EqOperator{Value: "uuid-123"},
		}}
		r, _ := walker.EvaluateSync(query, map[string]any{"id": "uuid-123", "attr2": "some_value"})
		assert.True(t, r)
	})
	t.Run("partial criteria not matches", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"id": EqOperator{Value: "uuid-123"},
		}}
		r, _ := walker.EvaluateSync(query, map[string]any{"id": "uuid-456", "attr2": "some_value"})
		assert.False(t, r)
	})
	t.Run("nested composite partial criteria", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"id": CompositeQuery{Fields: map[string]IQueryOperator{
				"first_model_id": EqOperator{Value: "uuid-A"},
			}},
		}}
		r1, _ := walker.EvaluateSync(query, map[string]any{"id": map[string]any{"id": "local-pk", "first_model_id": "uuid-A"}, "attr2": "foo"})
		assert.True(t, r1)
		r2, _ := walker.EvaluateSync(query, map[string]any{"id": map[string]any{"id": "local-pk", "first_model_id": "uuid-B"}, "attr2": "foo"})
		assert.False(t, r2)
	})
}

// =============================================================================
// EvaluateVisitor - Basic (no resolver)
// =============================================================================

func evalVisitor(state any, query IQueryOperator, resolver IObjectResolver) (bool, error) {
	v := NewEvaluateVisitor(state, sess, resolver)
	result, err := query.Accept(v)
	if err != nil {
		return false, err
	}
	return result.(bool), nil
}

func TestEvaluateVisitorBasic(t *testing.T) {
	t.Run("eq matches", func(t *testing.T) {
		r, _ := evalVisitor(42, EqOperator{Value: 42}, nil)
		assert.True(t, r)
	})
	t.Run("eq not matches", func(t *testing.T) {
		r, _ := evalVisitor(99, EqOperator{Value: 42}, nil)
		assert.False(t, r)
	})
	t.Run("eq nil", func(t *testing.T) {
		r, _ := evalVisitor(nil, EqOperator{Value: nil}, nil)
		assert.True(t, r)
	})
	t.Run("eq string", func(t *testing.T) {
		r, _ := evalVisitor("active", EqOperator{Value: "active"}, nil)
		assert.True(t, r)
	})
	t.Run("comparison ne", func(t *testing.T) {
		op := ComparisonOperator{Op: "$ne", Value: "deleted"}
		r1, _ := evalVisitor("active", op, nil)
		assert.True(t, r1)
		r2, _ := evalVisitor("deleted", op, nil)
		assert.False(t, r2)
	})
	t.Run("comparison gt", func(t *testing.T) {
		op := ComparisonOperator{Op: "$gt", Value: 10}
		r1, _ := evalVisitor(15, op, nil)
		assert.True(t, r1)
		r2, _ := evalVisitor(10, op, nil)
		assert.False(t, r2)
		r3, _ := evalVisitor(5, op, nil)
		assert.False(t, r3)
	})
	t.Run("comparison gte", func(t *testing.T) {
		op := ComparisonOperator{Op: "$gte", Value: 10}
		r1, _ := evalVisitor(15, op, nil)
		assert.True(t, r1)
		r2, _ := evalVisitor(10, op, nil)
		assert.True(t, r2)
		r3, _ := evalVisitor(5, op, nil)
		assert.False(t, r3)
	})
	t.Run("comparison lt", func(t *testing.T) {
		op := ComparisonOperator{Op: "$lt", Value: 10}
		r1, _ := evalVisitor(5, op, nil)
		assert.True(t, r1)
		r2, _ := evalVisitor(10, op, nil)
		assert.False(t, r2)
		r3, _ := evalVisitor(15, op, nil)
		assert.False(t, r3)
	})
	t.Run("comparison lte", func(t *testing.T) {
		op := ComparisonOperator{Op: "$lte", Value: 10}
		r1, _ := evalVisitor(5, op, nil)
		assert.True(t, r1)
		r2, _ := evalVisitor(10, op, nil)
		assert.True(t, r2)
		r3, _ := evalVisitor(15, op, nil)
		assert.False(t, r3)
	})
	t.Run("in matches", func(t *testing.T) {
		op := InOperator{Values: []any{"active", "pending"}}
		r1, _ := evalVisitor("active", op, nil)
		assert.True(t, r1)
		r2, _ := evalVisitor("pending", op, nil)
		assert.True(t, r2)
	})
	t.Run("in not matches", func(t *testing.T) {
		op := InOperator{Values: []any{"active", "pending"}}
		r, _ := evalVisitor("deleted", op, nil)
		assert.False(t, r)
	})
	t.Run("is_null true matches nil", func(t *testing.T) {
		r, _ := evalVisitor(nil, IsNullOperator{Value: true}, nil)
		assert.True(t, r)
	})
	t.Run("is_null true not matches value", func(t *testing.T) {
		r, _ := evalVisitor(42, IsNullOperator{Value: true}, nil)
		assert.False(t, r)
	})
	t.Run("is_null false matches value", func(t *testing.T) {
		r, _ := evalVisitor(42, IsNullOperator{Value: false}, nil)
		assert.True(t, r)
	})
	t.Run("is_null false not matches nil", func(t *testing.T) {
		r, _ := evalVisitor(nil, IsNullOperator{Value: false}, nil)
		assert.False(t, r)
	})
	t.Run("and all true", func(t *testing.T) {
		op := AndOperator{Operands: []IQueryOperator{
			ComparisonOperator{Op: "$gt", Value: 5},
			ComparisonOperator{Op: "$lt", Value: 10},
		}}
		r, _ := evalVisitor(7, op, nil)
		assert.True(t, r)
	})
	t.Run("and one false", func(t *testing.T) {
		op := AndOperator{Operands: []IQueryOperator{
			ComparisonOperator{Op: "$gt", Value: 5},
			ComparisonOperator{Op: "$lt", Value: 10},
		}}
		r, _ := evalVisitor(12, op, nil)
		assert.False(t, r)
	})
	t.Run("or one true", func(t *testing.T) {
		op := OrOperator{Operands: []IQueryOperator{
			EqOperator{Value: "active"},
			EqOperator{Value: "pending"},
		}}
		r, _ := evalVisitor("pending", op, nil)
		assert.True(t, r)
	})
	t.Run("or none true", func(t *testing.T) {
		op := OrOperator{Operands: []IQueryOperator{
			EqOperator{Value: "active"},
			EqOperator{Value: "pending"},
		}}
		r, _ := evalVisitor("deleted", op, nil)
		assert.False(t, r)
	})
	t.Run("composite matches", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": EqOperator{Value: "active"},
			"name":   EqOperator{Value: "Alice"},
		}}
		r, _ := evalVisitor(map[string]any{"status": "active", "name": "Alice", "extra": "ignored"}, query, nil)
		assert.True(t, r)
	})
	t.Run("composite not matches", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": EqOperator{Value: "active"},
			"name":   EqOperator{Value: "Alice"},
		}}
		r, _ := evalVisitor(map[string]any{"status": "inactive", "name": "Alice"}, query, nil)
		assert.False(t, r)
	})
	t.Run("composite non dict state", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}
		r, _ := evalVisitor(42, query, nil)
		assert.False(t, r)
	})
	t.Run("nested composite without resolver", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"address": CompositeQuery{Fields: map[string]IQueryOperator{
				"city": EqOperator{Value: "Moscow"},
			}},
		}}
		r1, _ := evalVisitor(map[string]any{"address": map[string]any{"city": "Moscow", "street": "Main"}}, query, nil)
		assert.True(t, r1)
		r2, _ := evalVisitor(map[string]any{"address": map[string]any{"city": "London"}}, query, nil)
		assert.False(t, r2)
	})
	t.Run("composite with comparison", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name": EqOperator{Value: "John"},
			"age": AndOperator{Operands: []IQueryOperator{
				ComparisonOperator{Op: "$gte", Value: 18},
				ComparisonOperator{Op: "$lt", Value: 65},
			}},
		}}
		r1, _ := evalVisitor(map[string]any{"name": "John", "age": 30}, query, nil)
		assert.True(t, r1)
		r2, _ := evalVisitor(map[string]any{"name": "John", "age": 15}, query, nil)
		assert.False(t, r2)
	})
	t.Run("composite with or", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": OrOperator{Operands: []IQueryOperator{
				EqOperator{Value: "active"},
				EqOperator{Value: "pending"},
			}},
		}}
		r1, _ := evalVisitor(map[string]any{"status": "active"}, query, nil)
		assert.True(t, r1)
		r2, _ := evalVisitor(map[string]any{"status": "pending"}, query, nil)
		assert.True(t, r2)
		r3, _ := evalVisitor(map[string]any{"status": "deleted"}, query, nil)
		assert.False(t, r3)
	})
	t.Run("composite with in", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": InOperator{Values: []any{"active", "pending"}},
		}}
		r1, _ := evalVisitor(map[string]any{"status": "active"}, query, nil)
		assert.True(t, r1)
		r2, _ := evalVisitor(map[string]any{"status": "deleted"}, query, nil)
		assert.False(t, r2)
	})
	t.Run("parsed simple pattern", func(t *testing.T) {
		query, _ := QueryParser{}.Parse(map[string]any{"status": "active"})
		r1, _ := evalVisitor(map[string]any{"status": "active", "name": "test"}, query, nil)
		assert.True(t, r1)
		r2, _ := evalVisitor(map[string]any{"status": "inactive", "name": "test"}, query, nil)
		assert.False(t, r2)
	})
	t.Run("rel without resolver delegates to inner", func(t *testing.T) {
		query := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"name": EqOperator{Value: "Active"}}}}
		r1, _ := evalVisitor(map[string]any{"name": "Active"}, query, nil)
		assert.True(t, r1)
		r2, _ := evalVisitor(map[string]any{"name": "Inactive"}, query, nil)
		assert.False(t, r2)
	})
}

// =============================================================================
// EvaluateVisitor - Nested Lookup (2 levels)
// =============================================================================

func TestEvaluateVisitorNestedLookup(t *testing.T) {
	statusStorage := map[any]map[string]any{
		"active":   {"id": "active", "name": "Active"},
		"inactive": {"id": "inactive", "name": "Inactive"},
	}
	statusResolver := makeResolver(nil)
	resolver := makeResolver(map[string]relInfo{
		"status_id": {storage: statusStorage, resolver: statusResolver},
	})

	eval := func(state any, query IQueryOperator) (bool, error) {
		return evalVisitor(state, query, resolver)
	}

	t.Run("nested lookup matches", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"name": EqOperator{Value: "Active"}}}},
		}}
		r, _ := eval(map[string]any{"id": 1, "status_id": "active", "name": "Alice"}, query)
		assert.True(t, r)
	})
	t.Run("nested lookup not matches", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"name": EqOperator{Value: "Active"}}}},
		}}
		r, _ := eval(map[string]any{"id": 2, "status_id": "inactive", "name": "Bob"}, query)
		assert.False(t, r)
	})
	t.Run("fk is nil", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"name": EqOperator{Value: "Active"}}}},
		}}
		r, _ := eval(map[string]any{"id": 3, "status_id": nil, "name": "Charlie"}, query)
		assert.False(t, r)
	})
	t.Run("foreign not found", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"name": EqOperator{Value: "Active"}}}},
		}}
		r, _ := eval(map[string]any{"id": 4, "status_id": "unknown", "name": "Dave"}, query)
		assert.False(t, r)
	})
	t.Run("simple value with nested lookup", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name":      EqOperator{Value: "Alice"},
			"status_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"name": EqOperator{Value: "Active"}}}},
		}}
		r1, _ := eval(map[string]any{"id": 1, "status_id": "active", "name": "Alice"}, query)
		assert.True(t, r1)
		r2, _ := eval(map[string]any{"id": 2, "status_id": "inactive", "name": "Bob"}, query)
		assert.False(t, r2)
	})
	t.Run("nested lookup with comparison", func(t *testing.T) {
		statusStorage["high"] = map[string]any{"id": "high", "name": "High", "priority": 10}
		statusStorage["low"] = map[string]any{"id": "low", "name": "Low", "priority": 1}

		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"priority": ComparisonOperator{Op: "$gte", Value: 5},
			}}},
		}}
		r1, _ := eval(map[string]any{"id": 1, "status_id": "high", "name": "Alice"}, query)
		assert.True(t, r1)
		r2, _ := eval(map[string]any{"id": 2, "status_id": "low", "name": "Bob"}, query)
		assert.False(t, r2)
	})
	t.Run("non relation field with resolver", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{"name": EqOperator{Value: "Alice"}}}
		r, _ := eval(map[string]any{"id": 1, "status_id": "active", "name": "Alice"}, query)
		assert.True(t, r)
	})
}

// =============================================================================
// EvaluateVisitor - Three Table Cascade
// =============================================================================

func TestEvaluateVisitorThreeTableCascade(t *testing.T) {
	countryStorage := map[any]map[string]any{
		"US": {"id": "US", "code": "US", "continent": "America"},
		"UK": {"id": "UK", "code": "UK", "continent": "Europe"},
		"JP": {"id": "JP", "code": "JP", "continent": "Asia"},
	}
	companyStorage := map[any]map[string]any{
		1: {"id": 1, "country_id": "US", "name": "Acme", "type": "tech", "revenue": 2000000},
		2: {"id": 2, "country_id": "UK", "name": "BritCo", "type": "finance", "revenue": 500000},
		3: {"id": 3, "country_id": "JP", "name": "TokyoTech", "type": "tech", "revenue": 800000},
	}

	countryResolver := makeResolver(nil)
	companyResolver := makeResolver(map[string]relInfo{
		"country_id": {storage: countryStorage, resolver: countryResolver},
	})
	resolver := makeResolver(map[string]relInfo{
		"company_id": {storage: companyStorage, resolver: companyResolver},
	})

	eval := func(state any, query IQueryOperator) (bool, error) {
		return evalVisitor(state, query, resolver)
	}

	t.Run("three table cascade matches", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name":   EqOperator{Value: "John"},
			"status": EqOperator{Value: "active"},
			"company_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"type": EqOperator{Value: "tech"},
				"country_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
					"code": EqOperator{Value: "US"},
				}}},
			}}},
		}}
		r, _ := eval(map[string]any{"id": 1, "company_id": 1, "name": "John", "age": 30, "status": "active"}, query)
		assert.True(t, r)
	})
	t.Run("cascade not matches middle", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"company_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"type": EqOperator{Value: "tech"},
			}}},
		}}
		r, _ := eval(map[string]any{"id": 2, "company_id": 2, "name": "Jane", "age": 25, "status": "active"}, query)
		assert.False(t, r)
	})
	t.Run("cascade not matches deepest", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"company_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"country_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
					"code": EqOperator{Value: "UK"},
				}}},
			}}},
		}}
		r1, _ := eval(map[string]any{"id": 1, "company_id": 1, "name": "John", "age": 30, "status": "active"}, query)
		assert.False(t, r1)
		r2, _ := eval(map[string]any{"id": 2, "company_id": 2, "name": "Jane", "age": 25, "status": "active"}, query)
		assert.True(t, r2)
	})
	t.Run("or in cascade", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"company_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"country_id": OrOperator{Operands: []IQueryOperator{
					RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"code": EqOperator{Value: "US"}}}},
					RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"code": EqOperator{Value: "UK"}}}},
				}},
			}}},
		}}
		r1, _ := eval(map[string]any{"id": 1, "company_id": 1, "name": "John", "age": 30, "status": "active"}, query)
		assert.True(t, r1)
		r2, _ := eval(map[string]any{"id": 2, "company_id": 2, "name": "Jane", "age": 25, "status": "active"}, query)
		assert.True(t, r2)
		r3, _ := eval(map[string]any{"id": 3, "company_id": 3, "name": "Yuki", "age": 28, "status": "active"}, query)
		assert.False(t, r3)
	})
	t.Run("cascade with all operators", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name": EqOperator{Value: "John"},
			"age":  ComparisonOperator{Op: "$gt", Value: 25},
			"company_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"type":    EqOperator{Value: "tech"},
				"revenue": ComparisonOperator{Op: "$gte", Value: 1000000},
				"country_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
					"code": EqOperator{Value: "US"},
				}}},
			}}},
		}}
		r1, _ := eval(map[string]any{"id": 1, "company_id": 1, "name": "John", "age": 30, "status": "active"}, query)
		assert.True(t, r1)
		r2, _ := eval(map[string]any{"id": 4, "company_id": 3, "name": "John", "age": 30, "status": "active"}, query)
		assert.False(t, r2)
	})
	t.Run("cascade company not found", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"company_id": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
				"type": EqOperator{Value: "tech"},
			}}},
		}}
		r, _ := eval(map[string]any{"id": 5, "company_id": 999, "name": "Ghost", "age": 0, "status": "unknown"}, query)
		assert.False(t, r)
	})
}

type personWithTags struct {
	Name   string `json:"name"`
	Age    int    `json:"age"`
	Status string `json:"status"`
}

type personWithoutTags struct {
	Name   string
	Age    int
	Status string
}

func TestEvaluateWalker_Struct(t *testing.T) {
	walker := NewEvaluateWalker(nil)

	t.Run("struct with json tags", func(t *testing.T) {
		state := personWithTags{Name: "Alice", Age: 30, Status: "active"}
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name":   EqOperator{Value: "Alice"},
			"age":    ComparisonOperator{Op: "$gte", Value: 30},
			"status": EqOperator{Value: "active"},
		}}
		result, err := walker.Evaluate(nil, query, state)
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("struct with json tags no match", func(t *testing.T) {
		state := personWithTags{Name: "Alice", Age: 30, Status: "active"}
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name": EqOperator{Value: "Bob"},
		}}
		result, err := walker.Evaluate(nil, query, state)
		assert.NoError(t, err)
		assert.False(t, result)
	})

	t.Run("struct field name match without json tags", func(t *testing.T) {
		state := personWithoutTags{Name: "Alice", Age: 30, Status: "active"}
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"Name":   EqOperator{Value: "Alice"},
			"Age":    ComparisonOperator{Op: "$gt", Value: 25},
			"Status": EqOperator{Value: "active"},
		}}
		result, err := walker.Evaluate(nil, query, state)
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("pointer to struct", func(t *testing.T) {
		state := &personWithTags{Name: "Alice", Age: 30, Status: "active"}
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name": EqOperator{Value: "Alice"},
		}}
		result, err := walker.Evaluate(nil, query, state)
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("missing field returns nil", func(t *testing.T) {
		state := personWithTags{Name: "Alice", Age: 30, Status: "active"}
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"nonexistent": IsNullOperator{Value: true},
		}}
		result, err := walker.Evaluate(nil, query, state)
		assert.NoError(t, err)
		assert.True(t, result)
	})
}

func TestEvaluateWalker_StructSync(t *testing.T) {
	walker := NewEvaluateWalker(nil)

	t.Run("struct with json tags sync", func(t *testing.T) {
		state := personWithTags{Name: "Alice", Age: 30, Status: "active"}
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name": EqOperator{Value: "Alice"},
			"age":  ComparisonOperator{Op: "$gte", Value: 30},
		}}
		result, err := walker.EvaluateSync(query, state)
		assert.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("pointer to struct sync", func(t *testing.T) {
		state := &personWithTags{Name: "Bob", Age: 25, Status: "inactive"}
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name":   EqOperator{Value: "Bob"},
			"status": EqOperator{Value: "inactive"},
		}}
		result, err := walker.EvaluateSync(query, state)
		assert.NoError(t, err)
		assert.True(t, result)
	})
}

func TestEvaluateVisitor_Struct(t *testing.T) {
	t.Run("struct with json tags visitor", func(t *testing.T) {
		state := personWithTags{Name: "Alice", Age: 30, Status: "active"}
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name": EqOperator{Value: "Alice"},
			"age":  ComparisonOperator{Op: "$gte", Value: 30},
		}}
		v := NewEvaluateVisitor(state, nil, nil)
		result, err := query.Accept(v)
		assert.NoError(t, err)
		assert.True(t, result.(bool))
	})

	t.Run("pointer to struct visitor", func(t *testing.T) {
		state := &personWithTags{Name: "Alice", Age: 30, Status: "active"}
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"name": EqOperator{Value: "Alice"},
		}}
		v := NewEvaluateVisitor(state, nil, nil)
		result, err := query.Accept(v)
		assert.NoError(t, err)
		assert.True(t, result.(bool))
	})

	t.Run("struct without tags visitor", func(t *testing.T) {
		state := personWithoutTags{Name: "Alice", Age: 30, Status: "active"}
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"Name": EqOperator{Value: "Alice"},
		}}
		v := NewEvaluateVisitor(state, nil, nil)
		result, err := query.Accept(v)
		assert.NoError(t, err)
		assert.True(t, result.(bool))
	})
}
