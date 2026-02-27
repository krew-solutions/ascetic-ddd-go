package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainquery "github.com/krew-solutions/ascetic-ddd-go/asceticddd/faker/domain/query"
)

type StubRelationResolver struct {
	relations map[string]*RelationInfo
}

func (r *StubRelationResolver) Resolve(field string) *RelationInfo {
	return r.relations[field]
}

func TestVisitEq(t *testing.T) {
	t.Run("scalar", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.EqOperator{Value: 42})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1", sql)
		assert.Equal(t, 1, len(params))
	})

	t.Run("string", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.EqOperator{Value: "active"})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1", sql)
		assert.Equal(t, 1, len(params))
	})

	t.Run("dict value", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.EqOperator{Value: map[string]any{"tenant": 1, "local": 2}})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1", sql)
		assert.Equal(t, map[string]any{"tenant": 1, "local": 2}, params[0].(Jsonb).Obj)
	})

	t.Run("custom target expr", func(t *testing.T) {
		compiler := NewPgQueryCompiler("rt.value", nil, nil)
		sql, _, err := compiler.Compile(domainquery.EqOperator{Value: 42})
		require.NoError(t, err)
		assert.Equal(t, "rt.value @> $1", sql)
	})
}

func TestVisitComposite(t *testing.T) {
	t.Run("single eq", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"status": domainquery.EqOperator{Value: "active"},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1", sql)
		assert.Equal(t, map[string]any{"status": "active"}, params[0].(Jsonb).Obj)
	})

	t.Run("multiple eq collapsed", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"status": domainquery.EqOperator{Value: "active"},
				"type":   domainquery.EqOperator{Value: "premium"},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1", sql)
		assert.Equal(t, 1, len(params))
		assert.Equal(t, map[string]any{"status": "active", "type": "premium"}, params[0].(Jsonb).Obj)
	})

	t.Run("nested composite preserves field", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"address": domainquery.CompositeQuery{
					Fields: map[string]domainquery.IQueryOperator{
						"city":    domainquery.EqOperator{Value: "Moscow"},
						"country": domainquery.EqOperator{Value: "Russia"},
					},
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1", sql)
		assert.Equal(t, map[string]any{"address": map[string]any{"city": "Moscow", "country": "Russia"}}, params[0].(Jsonb).Obj)
	})

	t.Run("eq with dict value in composite", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"pk": domainquery.EqOperator{Value: map[string]any{"tenant": 1, "local": 2}},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1", sql)
		assert.Equal(t, map[string]any{"pk": map[string]any{"tenant": 1, "local": 2}}, params[0].(Jsonb).Obj)
	})

	t.Run("mixed eq and nested composite", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"status": domainquery.EqOperator{Value: "active"},
				"address": domainquery.CompositeQuery{
					Fields: map[string]domainquery.IQueryOperator{
						"city": domainquery.EqOperator{Value: "Moscow"},
					},
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1", sql)
		assert.Equal(t, map[string]any{"status": "active", "address": map[string]any{"city": "Moscow"}}, params[0].(Jsonb).Obj)
	})

	t.Run("deeply nested composite", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"a": domainquery.CompositeQuery{
					Fields: map[string]domainquery.IQueryOperator{
						"b": domainquery.CompositeQuery{
							Fields: map[string]domainquery.IQueryOperator{
								"c": domainquery.EqOperator{Value: 42},
							},
						},
					},
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1", sql)
		assert.Equal(t, map[string]any{"a": map[string]any{"b": map[string]any{"c": 42}}}, params[0].(Jsonb).Obj)
	})
}

func TestVisitRel(t *testing.T) {
	t.Run("without resolver raises", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		_, _, err := compiler.Compile(domainquery.RelOperator{
			Query: domainquery.CompositeQuery{
				Fields: map[string]domainquery.IQueryOperator{
					"status": domainquery.EqOperator{Value: "active"},
				},
			},
		})
		assert.Error(t, err)
	})

	t.Run("rel field without resolver raises", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		_, _, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"fk_id": domainquery.RelOperator{
					Query: domainquery.CompositeQuery{
						Fields: map[string]domainquery.IQueryOperator{
							"name": domainquery.EqOperator{Value: "IT"},
						},
					},
				},
			},
		})
		assert.Error(t, err)
	})

	t.Run("rel simple exists", func(t *testing.T) {
		resolver := &StubRelationResolver{
			relations: map[string]*RelationInfo{
				"company_id": {Table: "companies", PkField: "value_id", NestedResolver: nil},
			},
		}
		compiler := NewPgQueryCompiler("", resolver, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"company_id": domainquery.RelOperator{
					Query: domainquery.CompositeQuery{
						Fields: map[string]domainquery.IQueryOperator{
							"name": domainquery.EqOperator{Value: "Acme"},
						},
					},
				},
			},
		})
		require.NoError(t, err)
		assert.Contains(t, sql, "EXISTS")
		assert.Contains(t, sql, "companies")
		assert.Contains(t, sql, "rt1")
		assert.Equal(t, map[string]any{"name": "Acme"}, params[0].(Jsonb).Obj)
	})

	t.Run("rel non reference fallback", func(t *testing.T) {
		resolver := &StubRelationResolver{relations: map[string]*RelationInfo{}}
		compiler := NewPgQueryCompiler("", resolver, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"address": domainquery.RelOperator{
					Query: domainquery.CompositeQuery{
						Fields: map[string]domainquery.IQueryOperator{
							"city": domainquery.EqOperator{Value: "Moscow"},
						},
					},
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1", sql)
		assert.Equal(t, map[string]any{"address": map[string]any{"city": "Moscow"}}, params[0].(Jsonb).Obj)
	})

	t.Run("rel with eq fields", func(t *testing.T) {
		resolver := &StubRelationResolver{
			relations: map[string]*RelationInfo{
				"dept_id": {Table: "departments", PkField: "value_id", NestedResolver: nil},
			},
		}
		compiler := NewPgQueryCompiler("", resolver, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"status":  domainquery.EqOperator{Value: "active"},
				"name":    domainquery.EqOperator{Value: "John"},
				"dept_id": domainquery.RelOperator{
					Query: domainquery.CompositeQuery{
						Fields: map[string]domainquery.IQueryOperator{
							"type": domainquery.EqOperator{Value: "engineering"},
						},
					},
				},
			},
		})
		require.NoError(t, err)
		assert.Contains(t, sql, "value @> $")
		assert.Contains(t, sql, "EXISTS")
		assert.Contains(t, sql, "departments")
		_ = params
	})
}

func TestVisitComparison(t *testing.T) {
	t.Run("gt in composite", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"age": domainquery.ComparisonOperator{Op: "$gt", Value: 18},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value->'age' > $1", sql)
		assert.Equal(t, []any{18}, params)
	})

	t.Run("gte in composite", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"score": domainquery.ComparisonOperator{Op: "$gte", Value: 100},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value->'score' >= $1", sql)
		assert.Equal(t, []any{100}, params)
	})

	t.Run("lt in composite", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"price": domainquery.ComparisonOperator{Op: "$lt", Value: 50},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value->'price' < $1", sql)
		assert.Equal(t, []any{50}, params)
	})

	t.Run("lte in composite", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"count": domainquery.ComparisonOperator{Op: "$lte", Value: 0},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value->'count' <= $1", sql)
		assert.Equal(t, []any{0}, params)
	})

	t.Run("mixed eq and comparison", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"status": domainquery.EqOperator{Value: "active"},
				"age":    domainquery.ComparisonOperator{Op: "$gt", Value: 18},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1 AND value->'age' > $2", sql)
		assert.Equal(t, map[string]any{"status": "active"}, params[0].(Jsonb).Obj)
		assert.Equal(t, 18, params[1])
	})

	t.Run("nested comparison", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"stats": domainquery.CompositeQuery{
					Fields: map[string]domainquery.IQueryOperator{
						"views": domainquery.ComparisonOperator{Op: "$gte", Value: 1000},
					},
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value->'stats'->'views' >= $1", sql)
		assert.Equal(t, []any{1000}, params)
	})
}

func TestVisitOr(t *testing.T) {
	t.Run("or with eq", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"status": domainquery.OrOperator{Operands: []domainquery.IQueryOperator{
					domainquery.EqOperator{Value: "active"},
					domainquery.EqOperator{Value: "pending"},
				}},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "(value @> $1 OR value @> $2)", sql)
		assert.Equal(t, map[string]any{"status": "active"}, params[0].(Jsonb).Obj)
		assert.Equal(t, map[string]any{"status": "pending"}, params[1].(Jsonb).Obj)
	})

	t.Run("or with composite", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"x": domainquery.OrOperator{Operands: []domainquery.IQueryOperator{
					domainquery.CompositeQuery{Fields: map[string]domainquery.IQueryOperator{
						"a": domainquery.EqOperator{Value: 1},
					}},
					domainquery.CompositeQuery{Fields: map[string]domainquery.IQueryOperator{
						"b": domainquery.EqOperator{Value: 2},
					}},
				}},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "(value @> $1 OR value @> $2)", sql)
		assert.Equal(t, map[string]any{"x": map[string]any{"a": 1}}, params[0].(Jsonb).Obj)
		assert.Equal(t, map[string]any{"x": map[string]any{"b": 2}}, params[1].(Jsonb).Obj)
	})

	t.Run("or with comparison", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"age": domainquery.OrOperator{Operands: []domainquery.IQueryOperator{
					domainquery.ComparisonOperator{Op: "$lt", Value: 18},
					domainquery.ComparisonOperator{Op: "$gt", Value: 65},
				}},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "(value->'age' < $1 OR value->'age' > $2)", sql)
		assert.Equal(t, []any{18, 65}, params)
	})

	t.Run("or mixed with eq field", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, _, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"type": domainquery.EqOperator{Value: "user"},
				"status": domainquery.OrOperator{Operands: []domainquery.IQueryOperator{
					domainquery.EqOperator{Value: "active"},
					domainquery.EqOperator{Value: "pending"},
				}},
			},
		})
		require.NoError(t, err)
		assert.Contains(t, sql, "value @> $")
		assert.Contains(t, sql, "OR")
	})

	t.Run("or three operands", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"priority": domainquery.OrOperator{Operands: []domainquery.IQueryOperator{
					domainquery.EqOperator{Value: "low"},
					domainquery.EqOperator{Value: "medium"},
					domainquery.EqOperator{Value: "high"},
				}},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, 2, countOccurrences(sql, "OR"))
		assert.Equal(t, 3, len(params))
	})
}

func TestVisitNe(t *testing.T) {
	t.Run("ne bare", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.ComparisonOperator{Op: "$ne", Value: "deleted"})
		require.NoError(t, err)
		assert.Equal(t, "NOT (value @> $1)", sql)
		assert.Equal(t, "deleted", params[0].(Jsonb).Obj)
	})

	t.Run("ne in composite", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"status": domainquery.ComparisonOperator{Op: "$ne", Value: "deleted"},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "NOT (value @> $1)", sql)
		assert.Equal(t, map[string]any{"status": "deleted"}, params[0].(Jsonb).Obj)
	})

	t.Run("ne nested", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"profile": domainquery.CompositeQuery{
					Fields: map[string]domainquery.IQueryOperator{
						"role": domainquery.ComparisonOperator{Op: "$ne", Value: "admin"},
					},
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "NOT (value @> $1)", sql)
		assert.Equal(t, map[string]any{"profile": map[string]any{"role": "admin"}}, params[0].(Jsonb).Obj)
	})

	t.Run("ne mixed with eq", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"status": domainquery.EqOperator{Value: "active"},
				"role":   domainquery.ComparisonOperator{Op: "$ne", Value: "admin"},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1 AND NOT (value @> $2)", sql)
		assert.Equal(t, map[string]any{"status": "active"}, params[0].(Jsonb).Obj)
		assert.Equal(t, map[string]any{"role": "admin"}, params[1].(Jsonb).Obj)
	})
}

func TestVisitIsNull(t *testing.T) {
	t.Run("is null true bare", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.IsNullOperator{Value: true})
		require.NoError(t, err)
		assert.Equal(t, "value IS NULL", sql)
		assert.Empty(t, params)
	})

	t.Run("is null false bare", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.IsNullOperator{Value: false})
		require.NoError(t, err)
		assert.Equal(t, "value IS NOT NULL", sql)
		assert.Empty(t, params)
	})

	t.Run("is null in composite", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"name": domainquery.IsNullOperator{Value: true},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value->'name' IS NULL", sql)
		assert.Empty(t, params)
	})

	t.Run("is null false in composite", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"name": domainquery.IsNullOperator{Value: false},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value->'name' IS NOT NULL", sql)
		assert.Empty(t, params)
	})

	t.Run("is null mixed with eq", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, _, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"status":     domainquery.EqOperator{Value: "active"},
				"deleted_at": domainquery.IsNullOperator{Value: true},
			},
		})
		require.NoError(t, err)
		assert.Contains(t, sql, "IS NULL")
		assert.Contains(t, sql, "@>")
	})
}

func TestVisitIn(t *testing.T) {
	t.Run("in bare", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.InOperator{Values: []any{"active", "pending"}})
		require.NoError(t, err)
		assert.Equal(t, "(value @> $1 OR value @> $2)", sql)
		assert.Equal(t, "active", params[0].(Jsonb).Obj)
		assert.Equal(t, "pending", params[1].(Jsonb).Obj)
	})

	t.Run("in single value", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.InOperator{Values: []any{42}})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1", sql)
		assert.Equal(t, 42, params[0].(Jsonb).Obj)
	})

	t.Run("in composite", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"status": domainquery.InOperator{Values: []any{"active", "pending"}},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "(value @> $1 OR value @> $2)", sql)
		assert.Equal(t, map[string]any{"status": "active"}, params[0].(Jsonb).Obj)
		assert.Equal(t, map[string]any{"status": "pending"}, params[1].(Jsonb).Obj)
	})

	t.Run("in nested", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"profile": domainquery.CompositeQuery{
					Fields: map[string]domainquery.IQueryOperator{
						"role": domainquery.InOperator{Values: []any{"admin", "moderator"}},
					},
				},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "(value @> $1 OR value @> $2)", sql)
		assert.Equal(t, map[string]any{"profile": map[string]any{"role": "admin"}}, params[0].(Jsonb).Obj)
		assert.Equal(t, map[string]any{"profile": map[string]any{"role": "moderator"}}, params[1].(Jsonb).Obj)
	})

	t.Run("in mixed with eq", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"type":   domainquery.EqOperator{Value: "user"},
				"status": domainquery.InOperator{Values: []any{"active", "pending"}},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1 AND (value @> $2 OR value @> $3)", sql)
		assert.Equal(t, map[string]any{"type": "user"}, params[0].(Jsonb).Obj)
		assert.Equal(t, map[string]any{"status": "active"}, params[1].(Jsonb).Obj)
		assert.Equal(t, map[string]any{"status": "pending"}, params[2].(Jsonb).Obj)
	})
}

func TestVisitAnd(t *testing.T) {
	t.Run("and range", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"age": domainquery.AndOperator{Operands: []domainquery.IQueryOperator{
					domainquery.ComparisonOperator{Op: "$gt", Value: 5},
					domainquery.ComparisonOperator{Op: "$lt", Value: 10},
				}},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value->'age' > $1 AND value->'age' < $2", sql)
		assert.Equal(t, []any{5, 10}, params)
	})

	t.Run("and ne with gt", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"age": domainquery.AndOperator{Operands: []domainquery.IQueryOperator{
					domainquery.ComparisonOperator{Op: "$ne", Value: 0},
					domainquery.ComparisonOperator{Op: "$gt", Value: 18},
				}},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "NOT (value @> $1) AND value->'age' > $2", sql)
		assert.Equal(t, map[string]any{"age": 0}, params[0].(Jsonb).Obj)
		assert.Equal(t, 18, params[1])
	})

	t.Run("and mixed with eq field", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		sql, params, err := compiler.Compile(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"status": domainquery.EqOperator{Value: "active"},
				"age": domainquery.AndOperator{Operands: []domainquery.IQueryOperator{
					domainquery.ComparisonOperator{Op: "$gte", Value: 18},
					domainquery.ComparisonOperator{Op: "$lt", Value: 65},
				}},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "value @> $1 AND value->'age' >= $2 AND value->'age' < $3", sql)
		assert.Equal(t, map[string]any{"status": "active"}, params[0].(Jsonb).Obj)
		assert.Equal(t, 18, params[1])
		assert.Equal(t, 65, params[2])
	})
}

func TestToDict(t *testing.T) {
	t.Run("eq", func(t *testing.T) {
		assert.Equal(t, 42, toDict(domainquery.EqOperator{Value: 42}))
	})

	t.Run("composite", func(t *testing.T) {
		result := toDict(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"a": domainquery.EqOperator{Value: 1},
				"b": domainquery.EqOperator{Value: 2},
			},
		})
		assert.Equal(t, map[string]any{"a": 1, "b": 2}, result)
	})

	t.Run("nested composite", func(t *testing.T) {
		result := toDict(domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"address": domainquery.CompositeQuery{
					Fields: map[string]domainquery.IQueryOperator{
						"city": domainquery.EqOperator{Value: "Moscow"},
					},
				},
				"status": domainquery.EqOperator{Value: "active"},
			},
		})
		assert.Equal(t, map[string]any{
			"address": map[string]any{"city": "Moscow"},
			"status":  "active",
		}, result)
	})

	t.Run("rel returns nil", func(t *testing.T) {
		result := toDict(domainquery.RelOperator{
			Query: domainquery.CompositeQuery{
				Fields: map[string]domainquery.IQueryOperator{
					"status": domainquery.EqOperator{Value: "active"},
				},
			},
		})
		assert.Nil(t, result)
	})
}

func TestCompilerReuse(t *testing.T) {
	t.Run("compile resets state", func(t *testing.T) {
		compiler := NewPgQueryCompiler("", nil, nil)
		_, params1, err := compiler.Compile(domainquery.EqOperator{Value: 1})
		require.NoError(t, err)
		assert.Equal(t, 1, len(params1))
		_, params2, err := compiler.Compile(domainquery.EqOperator{Value: 2})
		require.NoError(t, err)
		assert.Equal(t, 1, len(params2))
	})
}

func TestCascadingRelations(t *testing.T) {
	makeResolvers := func() *StubRelationResolver {
		countryResolver := &StubRelationResolver{relations: map[string]*RelationInfo{}}
		companyResolver := &StubRelationResolver{
			relations: map[string]*RelationInfo{
				"country_id": {Table: "countries", PkField: "value_id", NestedResolver: countryResolver},
			},
		}
		return &StubRelationResolver{
			relations: map[string]*RelationInfo{
				"company_id": {Table: "companies", PkField: "value_id", NestedResolver: companyResolver},
			},
		}
	}

	t.Run("three table cascade", func(t *testing.T) {
		resolver := makeResolvers()
		compiler := NewPgQueryCompiler("", resolver, nil)

		query := domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"name":   domainquery.EqOperator{Value: "John"},
				"status": domainquery.EqOperator{Value: "active"},
				"age":    domainquery.ComparisonOperator{Op: "$gt", Value: 25},
				"company_id": domainquery.RelOperator{
					Query: domainquery.CompositeQuery{
						Fields: map[string]domainquery.IQueryOperator{
							"type":    domainquery.EqOperator{Value: "tech"},
							"size":    domainquery.EqOperator{Value: "large"},
							"revenue": domainquery.ComparisonOperator{Op: "$gte", Value: 1000000},
							"country_id": domainquery.OrOperator{Operands: []domainquery.IQueryOperator{
								domainquery.RelOperator{
									Query: domainquery.CompositeQuery{
										Fields: map[string]domainquery.IQueryOperator{
											"code": domainquery.EqOperator{Value: "US"},
										},
									},
								},
								domainquery.RelOperator{
									Query: domainquery.CompositeQuery{
										Fields: map[string]domainquery.IQueryOperator{
											"code": domainquery.EqOperator{Value: "UK"},
										},
									},
								},
							}},
						},
					},
				},
			},
		}

		sql, params, err := compiler.Compile(query)
		require.NoError(t, err)

		assert.Contains(t, sql, "value @> $")
		assert.Contains(t, sql, "value->'age' > $")
		assert.Contains(t, sql, "EXISTS (SELECT 1 FROM companies")
		assert.Equal(t, 2, countOccurrences(sql, "EXISTS (SELECT 1 FROM countries"))
		assert.Contains(t, sql, "rt1")
		assert.Contains(t, sql, "rt2")
		assert.Contains(t, sql, "rt3")

		// Find the collapsed eq param
		foundCollapsedEq := false
		for _, p := range params {
			if j, ok := p.(Jsonb); ok {
				if m, ok := j.Obj.(map[string]any); ok {
					if m["name"] == "John" && m["status"] == "active" {
						foundCollapsedEq = true
					}
				}
			}
		}
		assert.True(t, foundCollapsedEq, "should have collapsed eq param with name=John and status=active")
	})

	t.Run("unique aliases no collision", func(t *testing.T) {
		resolver := makeResolvers()
		compiler := NewPgQueryCompiler("", resolver, nil)

		query := domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"company_id": domainquery.RelOperator{
					Query: domainquery.CompositeQuery{
						Fields: map[string]domainquery.IQueryOperator{
							"name": domainquery.EqOperator{Value: "Acme"},
							"country_id": domainquery.RelOperator{
								Query: domainquery.CompositeQuery{
									Fields: map[string]domainquery.IQueryOperator{
										"code": domainquery.EqOperator{Value: "US"},
									},
								},
							},
						},
					},
				},
			},
		}

		sql, _, err := compiler.Compile(query)
		require.NoError(t, err)

		assert.Contains(t, sql, "rt1")
		assert.Contains(t, sql, "rt2")
		assert.Regexp(t, `FROM companies rt1`, sql)
		assert.Regexp(t, `FROM countries rt2`, sql)
	})

	t.Run("or both branches reference same table", func(t *testing.T) {
		resolver := makeResolvers()
		compiler := NewPgQueryCompiler("", resolver, nil)

		query := domainquery.CompositeQuery{
			Fields: map[string]domainquery.IQueryOperator{
				"company_id": domainquery.RelOperator{
					Query: domainquery.CompositeQuery{
						Fields: map[string]domainquery.IQueryOperator{
							"country_id": domainquery.OrOperator{Operands: []domainquery.IQueryOperator{
								domainquery.RelOperator{
									Query: domainquery.CompositeQuery{
										Fields: map[string]domainquery.IQueryOperator{
											"code": domainquery.EqOperator{Value: "US"},
										},
									},
								},
								domainquery.RelOperator{
									Query: domainquery.CompositeQuery{
										Fields: map[string]domainquery.IQueryOperator{
											"code": domainquery.EqOperator{Value: "UK"},
										},
									},
								},
							}},
						},
					},
				},
			},
		}

		sql, _, err := compiler.Compile(query)
		require.NoError(t, err)

		assert.Equal(t, 2, countOccurrences(sql, "FROM countries"))
		assert.Contains(t, sql, " OR ")
	})
}

func countOccurrences(s, substr string) int {
	count := 0
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			count++
		}
	}
	return count
}
