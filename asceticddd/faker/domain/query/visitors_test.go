package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// QueryToDictVisitor
// =============================================================================

func TestQueryToDictVisitorEq(t *testing.T) {
	v := QueryToDictVisitor{}

	t.Run("eq int", func(t *testing.T) {
		result, err := v.Visit(EqOperator{Value: 5})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$eq": 5}, result)
	})
	t.Run("eq nil", func(t *testing.T) {
		result, err := v.Visit(EqOperator{Value: nil})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$eq": nil}, result)
	})
	t.Run("eq dict", func(t *testing.T) {
		result, err := v.Visit(EqOperator{Value: map[string]any{"a": 1, "b": 2}})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$eq": map[string]any{"a": 1, "b": 2}}, result)
	})
}

func TestQueryToDictVisitorRel(t *testing.T) {
	v := QueryToDictVisitor{}

	t.Run("simple", func(t *testing.T) {
		query := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$rel": map[string]any{"status": map[string]any{"$eq": "active"}}}, result)
	})
	t.Run("multiple fields", func(t *testing.T) {
		query := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
			"status": EqOperator{Value: "active"},
			"type":   EqOperator{Value: "premium"},
		}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		expected := map[string]any{"$rel": map[string]any{
			"status": map[string]any{"$eq": "active"},
			"type":   map[string]any{"$eq": "premium"},
		}}
		assert.Equal(t, expected, result)
	})
	t.Run("nested", func(t *testing.T) {
		query := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
			"department": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"name": EqOperator{Value: "IT"}}}},
		}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		expected := map[string]any{"$rel": map[string]any{
			"department": map[string]any{"$rel": map[string]any{"name": map[string]any{"$eq": "IT"}}},
		}}
		assert.Equal(t, expected, result)
	})
}

func TestQueryToDictVisitorComposite(t *testing.T) {
	v := QueryToDictVisitor{}

	t.Run("simple", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}, "b": EqOperator{Value: 2}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"a": map[string]any{"$eq": 1}, "b": map[string]any{"$eq": 2}}, result)
	})
	t.Run("nested", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"address": CompositeQuery{Fields: map[string]IQueryOperator{
				"city":    EqOperator{Value: "Moscow"},
				"country": EqOperator{Value: "Russia"},
			}},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		expected := map[string]any{"address": map[string]any{
			"city":    map[string]any{"$eq": "Moscow"},
			"country": map[string]any{"$eq": "Russia"},
		}}
		assert.Equal(t, expected, result)
	})
}

func TestQueryToDictVisitorComparison(t *testing.T) {
	v := QueryToDictVisitor{}

	for _, tc := range []struct {
		op  string
		val any
	}{
		{"$gt", 10}, {"$gte", 0}, {"$lt", 100}, {"$lte", 99},
	} {
		t.Run(tc.op, func(t *testing.T) {
			result, err := v.Visit(ComparisonOperator{Op: tc.op, Value: tc.val})
			assert.NoError(t, err)
			assert.Equal(t, map[string]any{tc.op: tc.val}, result)
		})
	}

	t.Run("in composite", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"age":    ComparisonOperator{Op: "$gt", Value: 18},
			"status": EqOperator{Value: "active"},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{
			"age":    map[string]any{"$gt": 18},
			"status": map[string]any{"$eq": "active"},
		}, result)
	})
}

func TestQueryToDictVisitorOr(t *testing.T) {
	v := QueryToDictVisitor{}

	t.Run("with eq", func(t *testing.T) {
		query := OrOperator{Operands: []IQueryOperator{EqOperator{Value: "a"}, EqOperator{Value: "b"}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$or": []any{map[string]any{"$eq": "a"}, map[string]any{"$eq": "b"}}}, result)
	})
	t.Run("with composite", func(t *testing.T) {
		query := OrOperator{Operands: []IQueryOperator{
			CompositeQuery{Fields: map[string]IQueryOperator{"x": EqOperator{Value: 1}}},
			CompositeQuery{Fields: map[string]IQueryOperator{"y": EqOperator{Value: 2}}},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		expected := map[string]any{"$or": []any{
			map[string]any{"x": map[string]any{"$eq": 1}},
			map[string]any{"y": map[string]any{"$eq": 2}},
		}}
		assert.Equal(t, expected, result)
	})
	t.Run("with comparison", func(t *testing.T) {
		query := OrOperator{Operands: []IQueryOperator{
			ComparisonOperator{Op: "$lt", Value: 18},
			ComparisonOperator{Op: "$gt", Value: 65},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$or": []any{map[string]any{"$lt": 18}, map[string]any{"$gt": 65}}}, result)
	})
	t.Run("three operands", func(t *testing.T) {
		query := OrOperator{Operands: []IQueryOperator{EqOperator{Value: 1}, EqOperator{Value: 2}, EqOperator{Value: 3}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$or": []any{
			map[string]any{"$eq": 1}, map[string]any{"$eq": 2}, map[string]any{"$eq": 3},
		}}, result)
	})
	t.Run("in composite", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": OrOperator{Operands: []IQueryOperator{EqOperator{Value: "active"}, EqOperator{Value: "pending"}}},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{
			"status": map[string]any{"$or": []any{map[string]any{"$eq": "active"}, map[string]any{"$eq": "pending"}}},
		}, result)
	})
}

func TestQueryToDictVisitorIn(t *testing.T) {
	v := QueryToDictVisitor{}

	t.Run("simple", func(t *testing.T) {
		result, err := v.Visit(InOperator{Values: []any{"active", "pending"}})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$in": []any{"active", "pending"}}, result)
	})
	t.Run("single", func(t *testing.T) {
		result, err := v.Visit(InOperator{Values: []any{42}})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$in": []any{42}}, result)
	})
	t.Run("in composite", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{"status": InOperator{Values: []any{"active", "pending"}}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"status": map[string]any{"$in": []any{"active", "pending"}}}, result)
	})
}

func TestQueryToDictVisitorIsNull(t *testing.T) {
	v := QueryToDictVisitor{}

	t.Run("true", func(t *testing.T) {
		result, err := v.Visit(IsNullOperator{Value: true})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$is_null": true}, result)
	})
	t.Run("false", func(t *testing.T) {
		result, err := v.Visit(IsNullOperator{Value: false})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$is_null": false}, result)
	})
	t.Run("in composite", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{"name": IsNullOperator{Value: true}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"name": map[string]any{"$is_null": true}}, result)
	})
}

func TestQueryToDictVisitorAnd(t *testing.T) {
	v := QueryToDictVisitor{}

	t.Run("range", func(t *testing.T) {
		query := AndOperator{Operands: []IQueryOperator{
			ComparisonOperator{Op: "$gt", Value: 5},
			ComparisonOperator{Op: "$lt", Value: 10},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$gt": 5, "$lt": 10}, result)
	})
	t.Run("three operands", func(t *testing.T) {
		query := AndOperator{Operands: []IQueryOperator{
			ComparisonOperator{Op: "$gt", Value: 0},
			ComparisonOperator{Op: "$lt", Value: 100},
			ComparisonOperator{Op: "$ne", Value: 50},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$gt": 0, "$lt": 100, "$ne": 50}, result)
	})
	t.Run("in composite", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"age": AndOperator{Operands: []IQueryOperator{
				ComparisonOperator{Op: "$gt", Value: 5},
				ComparisonOperator{Op: "$lt", Value: 10},
			}},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"age": map[string]any{"$gt": 5, "$lt": 10}}, result)
	})
}

// =============================================================================
// QueryToPlainValueVisitor
// =============================================================================

func TestQueryToPlainValueVisitorEq(t *testing.T) {
	v := QueryToPlainValueVisitor{}

	t.Run("int", func(t *testing.T) {
		result, err := v.Visit(EqOperator{Value: 5})
		assert.NoError(t, err)
		assert.Equal(t, 5, result)
	})
	t.Run("nil", func(t *testing.T) {
		result, err := v.Visit(EqOperator{Value: nil})
		assert.NoError(t, err)
		assert.Nil(t, result)
	})
	t.Run("dict", func(t *testing.T) {
		result, err := v.Visit(EqOperator{Value: map[string]any{"a": 1, "b": 2}})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"a": 1, "b": 2}, result)
	})
}

func TestQueryToPlainValueVisitorRel(t *testing.T) {
	v := QueryToPlainValueVisitor{}

	t.Run("simple", func(t *testing.T) {
		query := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"status": "active"}, result)
	})
	t.Run("multiple fields", func(t *testing.T) {
		query := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
			"status": EqOperator{Value: "active"},
			"type":   EqOperator{Value: "premium"},
		}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"status": "active", "type": "premium"}, result)
	})
	t.Run("nested", func(t *testing.T) {
		query := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
			"department": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"name": EqOperator{Value: "IT"}}}},
		}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"department": map[string]any{"name": "IT"}}, result)
	})
}

func TestQueryToPlainValueVisitorComposite(t *testing.T) {
	v := QueryToPlainValueVisitor{}

	t.Run("simple", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}, "b": EqOperator{Value: 2}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"a": 1, "b": 2}, result)
	})
	t.Run("nested", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"address": CompositeQuery{Fields: map[string]IQueryOperator{
				"city":    EqOperator{Value: "Moscow"},
				"country": EqOperator{Value: "Russia"},
			}},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"address": map[string]any{"city": "Moscow", "country": "Russia"}}, result)
	})
}

func TestQueryToPlainValueVisitorComparison(t *testing.T) {
	v := QueryToPlainValueVisitor{}

	for _, tc := range []struct {
		op  string
		val any
	}{
		{"$gt", 10}, {"$gte", 0}, {"$lt", 100}, {"$lte", 99},
	} {
		t.Run(tc.op, func(t *testing.T) {
			result, err := v.Visit(ComparisonOperator{Op: tc.op, Value: tc.val})
			assert.NoError(t, err)
			assert.Equal(t, map[string]any{tc.op: tc.val}, result)
		})
	}

	t.Run("in composite", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"age":    ComparisonOperator{Op: "$gt", Value: 18},
			"status": EqOperator{Value: "active"},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"age": map[string]any{"$gt": 18}, "status": "active"}, result)
	})
}

func TestQueryToPlainValueVisitorOr(t *testing.T) {
	v := QueryToPlainValueVisitor{}

	t.Run("with eq", func(t *testing.T) {
		query := OrOperator{Operands: []IQueryOperator{EqOperator{Value: "a"}, EqOperator{Value: "b"}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$or": []any{"a", "b"}}, result)
	})
	t.Run("with composite", func(t *testing.T) {
		query := OrOperator{Operands: []IQueryOperator{
			CompositeQuery{Fields: map[string]IQueryOperator{"x": EqOperator{Value: 1}}},
			CompositeQuery{Fields: map[string]IQueryOperator{"y": EqOperator{Value: 2}}},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		expected := map[string]any{"$or": []any{
			map[string]any{"x": 1},
			map[string]any{"y": 2},
		}}
		assert.Equal(t, expected, result)
	})
	t.Run("with comparison", func(t *testing.T) {
		query := OrOperator{Operands: []IQueryOperator{
			ComparisonOperator{Op: "$lt", Value: 18},
			ComparisonOperator{Op: "$gt", Value: 65},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$or": []any{map[string]any{"$lt": 18}, map[string]any{"$gt": 65}}}, result)
	})
	t.Run("three operands", func(t *testing.T) {
		query := OrOperator{Operands: []IQueryOperator{EqOperator{Value: 1}, EqOperator{Value: 2}, EqOperator{Value: 3}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$or": []any{1, 2, 3}}, result)
	})
	t.Run("in composite", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"status": OrOperator{Operands: []IQueryOperator{EqOperator{Value: "active"}, EqOperator{Value: "pending"}}},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"status": map[string]any{"$or": []any{"active", "pending"}}}, result)
	})
}

func TestQueryToPlainValueVisitorIn(t *testing.T) {
	v := QueryToPlainValueVisitor{}

	t.Run("simple", func(t *testing.T) {
		result, err := v.Visit(InOperator{Values: []any{"active", "pending"}})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$in": []any{"active", "pending"}}, result)
	})
	t.Run("in composite", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{"status": InOperator{Values: []any{"active", "pending"}}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"status": map[string]any{"$in": []any{"active", "pending"}}}, result)
	})
}

func TestQueryToPlainValueVisitorIsNull(t *testing.T) {
	v := QueryToPlainValueVisitor{}

	t.Run("true", func(t *testing.T) {
		result, err := v.Visit(IsNullOperator{Value: true})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$is_null": true}, result)
	})
	t.Run("false", func(t *testing.T) {
		result, err := v.Visit(IsNullOperator{Value: false})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$is_null": false}, result)
	})
	t.Run("in composite", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{"name": IsNullOperator{Value: true}}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"name": map[string]any{"$is_null": true}}, result)
	})
}

func TestQueryToPlainValueVisitorAnd(t *testing.T) {
	v := QueryToPlainValueVisitor{}

	t.Run("range", func(t *testing.T) {
		query := AndOperator{Operands: []IQueryOperator{
			ComparisonOperator{Op: "$gt", Value: 5},
			ComparisonOperator{Op: "$lt", Value: 10},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$gt": 5, "$lt": 10}, result)
	})
	t.Run("in composite", func(t *testing.T) {
		query := CompositeQuery{Fields: map[string]IQueryOperator{
			"age": AndOperator{Operands: []IQueryOperator{
				ComparisonOperator{Op: "$gt", Value: 5},
				ComparisonOperator{Op: "$lt", Value: 10},
			}},
		}}
		result, err := v.Visit(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"age": map[string]any{"$gt": 5, "$lt": 10}}, result)
	})
}

// =============================================================================
// Convenience functions
// =============================================================================

func TestConvenienceFunctions(t *testing.T) {
	t.Run("QueryToDict eq", func(t *testing.T) {
		result, err := QueryToDict(EqOperator{Value: 5})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$eq": 5}, result)
	})
	t.Run("QueryToDict rel", func(t *testing.T) {
		query := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}}
		result, err := QueryToDict(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"$rel": map[string]any{"status": map[string]any{"$eq": "active"}}}, result)
	})
	t.Run("QueryToDict composite", func(t *testing.T) {
		result, err := QueryToDict(CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"a": map[string]any{"$eq": 1}}, result)
	})
	t.Run("QueryToPlainValue eq", func(t *testing.T) {
		result, err := QueryToPlainValue(EqOperator{Value: 5})
		assert.NoError(t, err)
		assert.Equal(t, 5, result)
	})
	t.Run("QueryToPlainValue rel", func(t *testing.T) {
		query := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}}
		result, err := QueryToPlainValue(query)
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"status": "active"}, result)
	})
	t.Run("QueryToPlainValue composite", func(t *testing.T) {
		result, err := QueryToPlainValue(CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}})
		assert.NoError(t, err)
		assert.Equal(t, map[string]any{"a": 1}, result)
	})
}
