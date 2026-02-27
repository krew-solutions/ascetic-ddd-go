package query

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryParserScalar(t *testing.T) {
	parser := QueryParser{}

	t.Run("int", func(t *testing.T) {
		result, err := parser.Parse(5)
		assert.NoError(t, err)
		eq := result.(EqOperator)
		assert.Equal(t, 5, eq.Value)
	})
	t.Run("string", func(t *testing.T) {
		result, err := parser.Parse("hello")
		assert.NoError(t, err)
		eq := result.(EqOperator)
		assert.Equal(t, "hello", eq.Value)
	})
	t.Run("nil", func(t *testing.T) {
		result, err := parser.Parse(nil)
		assert.NoError(t, err)
		eq := result.(EqOperator)
		assert.Nil(t, eq.Value)
	})
	t.Run("bool", func(t *testing.T) {
		result, err := parser.Parse(true)
		assert.NoError(t, err)
		eq := result.(EqOperator)
		assert.Equal(t, true, eq.Value)
	})
}

func TestQueryParserExplicitEq(t *testing.T) {
	parser := QueryParser{}

	t.Run("eq operator", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$eq": 42})
		assert.NoError(t, err)
		eq := result.(EqOperator)
		assert.Equal(t, 42, eq.Value)
	})
	t.Run("eq with nil", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$eq": nil})
		assert.NoError(t, err)
		eq := result.(EqOperator)
		assert.Nil(t, eq.Value)
	})
	t.Run("eq with dict is fully parsed", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$eq": map[string]any{"tenant_id": 1, "local_id": 2}})
		assert.NoError(t, err)
		eq := result.(EqOperator)
		cq := eq.Value.(CompositeQuery)
		assert.Equal(t, 1, cq.Fields["tenant_id"].(EqOperator).Value)
		assert.Equal(t, 2, cq.Fields["local_id"].(EqOperator).Value)
	})
}

func TestQueryParserRel(t *testing.T) {
	parser := QueryParser{}

	t.Run("simple", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$rel": map[string]any{"status": map[string]any{"$eq": "active"}}})
		assert.NoError(t, err)
		rel := result.(RelOperator)
		assert.True(t, rel.Query.Fields["status"].Equal(EqOperator{Value: "active"}))
	})
	t.Run("multiple fields", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{
			"$rel": map[string]any{
				"status": map[string]any{"$eq": "active"},
				"type":   map[string]any{"$eq": "premium"},
			},
		})
		assert.NoError(t, err)
		rel := result.(RelOperator)
		assert.Equal(t, 2, len(rel.Query.Fields))
		assert.True(t, rel.Query.Fields["status"].Equal(EqOperator{Value: "active"}))
		assert.True(t, rel.Query.Fields["type"].Equal(EqOperator{Value: "premium"}))
	})
	t.Run("nested", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{
			"$rel": map[string]any{
				"department": map[string]any{
					"$rel": map[string]any{"name": map[string]any{"$eq": "IT"}},
				},
			},
		})
		assert.NoError(t, err)
		rel := result.(RelOperator)
		dept := rel.Query.Fields["department"].(RelOperator)
		assert.True(t, dept.Query.Fields["name"].Equal(EqOperator{Value: "IT"}))
	})
	t.Run("with implicit eq", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$rel": map[string]any{"id": 42}})
		assert.NoError(t, err)
		rel := result.(RelOperator)
		assert.True(t, rel.Query.Fields["id"].Equal(EqOperator{Value: 42}))
	})
}

func TestQueryParserComposite(t *testing.T) {
	parser := QueryParser{}

	t.Run("basic", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{
			"tenant_id": map[string]any{"$eq": 15},
			"local_id":  map[string]any{"$eq": 27},
		})
		assert.NoError(t, err)
		cq := result.(CompositeQuery)
		assert.Equal(t, 2, len(cq.Fields))
		assert.Equal(t, 15, cq.Fields["tenant_id"].(EqOperator).Value)
		assert.Equal(t, 27, cq.Fields["local_id"].(EqOperator).Value)
	})
	t.Run("implicit eq", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"tenant_id": 15, "local_id": 27})
		assert.NoError(t, err)
		cq := result.(CompositeQuery)
		assert.Equal(t, 15, cq.Fields["tenant_id"].(EqOperator).Value)
		assert.Equal(t, 27, cq.Fields["local_id"].(EqOperator).Value)
	})
	t.Run("nested", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{
			"address": map[string]any{
				"city":    map[string]any{"$eq": "Moscow"},
				"country": map[string]any{"$eq": "Russia"},
			},
		})
		assert.NoError(t, err)
		cq := result.(CompositeQuery)
		addr := cq.Fields["address"].(CompositeQuery)
		assert.Equal(t, "Moscow", addr.Fields["city"].(EqOperator).Value)
	})
}

func TestQueryParserComparison(t *testing.T) {
	parser := QueryParser{}

	t.Run("gt", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$gt": 10})
		assert.NoError(t, err)
		c := result.(ComparisonOperator)
		assert.Equal(t, "$gt", c.Op)
		assert.Equal(t, 10, c.Value)
	})
	t.Run("gte", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$gte": 0})
		assert.NoError(t, err)
		c := result.(ComparisonOperator)
		assert.Equal(t, "$gte", c.Op)
		assert.Equal(t, 0, c.Value)
	})
	t.Run("lt", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$lt": 100})
		assert.NoError(t, err)
		c := result.(ComparisonOperator)
		assert.Equal(t, "$lt", c.Op)
		assert.Equal(t, 100, c.Value)
	})
	t.Run("lte", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$lte": 99})
		assert.NoError(t, err)
		c := result.(ComparisonOperator)
		assert.Equal(t, "$lte", c.Op)
		assert.Equal(t, 99, c.Value)
	})
	t.Run("in composite", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{
			"age":    map[string]any{"$gt": 18},
			"status": map[string]any{"$eq": "active"},
		})
		assert.NoError(t, err)
		cq := result.(CompositeQuery)
		age := cq.Fields["age"].(ComparisonOperator)
		assert.Equal(t, "$gt", age.Op)
		assert.Equal(t, 18, age.Value)
		assert.IsType(t, EqOperator{}, cq.Fields["status"])
	})
}

func TestQueryParserOr(t *testing.T) {
	parser := QueryParser{}

	t.Run("simple", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{
			"$or": []any{
				map[string]any{"status": map[string]any{"$eq": "active"}},
				map[string]any{"status": map[string]any{"$eq": "pending"}},
			},
		})
		assert.NoError(t, err)
		or := result.(OrOperator)
		assert.Equal(t, 2, len(or.Operands))
		assert.IsType(t, CompositeQuery{}, or.Operands[0])
		assert.IsType(t, CompositeQuery{}, or.Operands[1])
	})
	t.Run("with scalars", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{
			"$or": []any{map[string]any{"$eq": 1}, map[string]any{"$eq": 2}},
		})
		assert.NoError(t, err)
		or := result.(OrOperator)
		assert.Equal(t, 1, or.Operands[0].(EqOperator).Value)
		assert.Equal(t, 2, or.Operands[1].(EqOperator).Value)
	})
	t.Run("three operands", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{
			"$or": []any{map[string]any{"$eq": "a"}, map[string]any{"$eq": "b"}, map[string]any{"$eq": "c"}},
		})
		assert.NoError(t, err)
		or := result.(OrOperator)
		assert.Equal(t, 3, len(or.Operands))
	})
	t.Run("in composite", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{
			"priority": map[string]any{
				"$or": []any{map[string]any{"$eq": "high"}, map[string]any{"$eq": "critical"}},
			},
		})
		assert.NoError(t, err)
		cq := result.(CompositeQuery)
		assert.IsType(t, OrOperator{}, cq.Fields["priority"])
	})
	t.Run("with comparison", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{
			"$or": []any{
				map[string]any{"age": map[string]any{"$gt": 18}},
				map[string]any{"vip": map[string]any{"$eq": true}},
			},
		})
		assert.NoError(t, err)
		or := result.(OrOperator)
		ageOp := or.Operands[0].(CompositeQuery).Fields["age"].(ComparisonOperator)
		assert.Equal(t, "$gt", ageOp.Op)
	})
	t.Run("non list raises", func(t *testing.T) {
		_, err := parser.Parse(map[string]any{"$or": "invalid"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "$or value must be list")
	})
	t.Run("one operand raises", func(t *testing.T) {
		_, err := parser.Parse(map[string]any{"$or": []any{map[string]any{"$eq": 1}}})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2 operands")
	})
	t.Run("empty raises", func(t *testing.T) {
		_, err := parser.Parse(map[string]any{"$or": []any{}})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least 2 operands")
	})
}

func TestQueryParserNe(t *testing.T) {
	parser := QueryParser{}

	t.Run("ne", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$ne": "deleted"})
		assert.NoError(t, err)
		c := result.(ComparisonOperator)
		assert.Equal(t, "$ne", c.Op)
		assert.Equal(t, "deleted", c.Value)
	})
	t.Run("ne in composite", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"status": map[string]any{"$ne": "deleted"}})
		assert.NoError(t, err)
		cq := result.(CompositeQuery)
		c := cq.Fields["status"].(ComparisonOperator)
		assert.Equal(t, "$ne", c.Op)
		assert.Equal(t, "deleted", c.Value)
	})
}

func TestQueryParserIn(t *testing.T) {
	parser := QueryParser{}

	t.Run("simple", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$in": []any{"active", "pending"}})
		assert.NoError(t, err)
		in := result.(InOperator)
		assert.Equal(t, []any{"active", "pending"}, in.Values)
	})
	t.Run("single", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$in": []any{42}})
		assert.NoError(t, err)
		in := result.(InOperator)
		assert.Equal(t, []any{42}, in.Values)
	})
	t.Run("in composite", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"status": map[string]any{"$in": []any{"active", "pending"}}})
		assert.NoError(t, err)
		cq := result.(CompositeQuery)
		in := cq.Fields["status"].(InOperator)
		assert.Equal(t, []any{"active", "pending"}, in.Values)
	})
	t.Run("non list raises", func(t *testing.T) {
		_, err := parser.Parse(map[string]any{"$in": "invalid"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "$in value must be list")
	})
	t.Run("empty raises", func(t *testing.T) {
		_, err := parser.Parse(map[string]any{"$in": []any{}})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "at least 1 value")
	})
}

func TestQueryParserIsNull(t *testing.T) {
	parser := QueryParser{}

	t.Run("true", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$is_null": true})
		assert.NoError(t, err)
		isn := result.(IsNullOperator)
		assert.True(t, isn.Value)
	})
	t.Run("false", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$is_null": false})
		assert.NoError(t, err)
		isn := result.(IsNullOperator)
		assert.False(t, isn.Value)
	})
	t.Run("in composite", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"name": map[string]any{"$is_null": true}})
		assert.NoError(t, err)
		cq := result.(CompositeQuery)
		isn := cq.Fields["name"].(IsNullOperator)
		assert.True(t, isn.Value)
	})
	t.Run("non bool raises", func(t *testing.T) {
		_, err := parser.Parse(map[string]any{"$is_null": 1})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "$is_null value must be bool")
	})
	t.Run("string raises", func(t *testing.T) {
		_, err := parser.Parse(map[string]any{"$is_null": "true"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "$is_null value must be bool")
	})
}

func TestQueryParserAnd(t *testing.T) {
	parser := QueryParser{}

	t.Run("range", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$gt": 5, "$lt": 10})
		assert.NoError(t, err)
		and := result.(AndOperator)
		assert.Equal(t, 2, len(and.Operands))
		ops := map[string]any{}
		for _, op := range and.Operands {
			c := op.(ComparisonOperator)
			ops[c.Op] = c.Value
		}
		assert.Equal(t, map[string]any{"$gt": 5, "$lt": 10}, ops)
	})
	t.Run("in composite", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"age": map[string]any{"$gt": 5, "$lt": 10}})
		assert.NoError(t, err)
		cq := result.(CompositeQuery)
		and := cq.Fields["age"].(AndOperator)
		assert.Equal(t, 2, len(and.Operands))
	})
	t.Run("three operators", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$gt": 0, "$lt": 100, "$ne": 50})
		assert.NoError(t, err)
		and := result.(AndOperator)
		assert.Equal(t, 3, len(and.Operands))
	})
	t.Run("ne with comparison", func(t *testing.T) {
		result, err := parser.Parse(map[string]any{"$ne": "deleted", "$gt": 0})
		assert.NoError(t, err)
		and := result.(AndOperator)
		assert.Equal(t, 2, len(and.Operands))
	})
}

func TestQueryParserErrors(t *testing.T) {
	parser := QueryParser{}

	t.Run("empty dict", func(t *testing.T) {
		_, err := parser.Parse(map[string]any{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty query")
	})
	t.Run("mixed operators and fields", func(t *testing.T) {
		_, err := parser.Parse(map[string]any{"$eq": 5, "field": 10})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "cannot mix operators and fields")
	})
	t.Run("unknown operator", func(t *testing.T) {
		_, err := parser.Parse(map[string]any{"$unknown": 5})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown operator")
	})
	t.Run("rel with non dict", func(t *testing.T) {
		_, err := parser.Parse(map[string]any{"$rel": "invalid"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "$rel value must be dict")
	})
}
