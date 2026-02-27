package query

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// EqOperator equality
// =============================================================================

func TestEqOperatorEqual(t *testing.T) {
	t.Run("equal values", func(t *testing.T) {
		assert.True(t, EqOperator{Value: 5}.Equal(EqOperator{Value: 5}))
	})
	t.Run("different values", func(t *testing.T) {
		assert.False(t, EqOperator{Value: 5}.Equal(EqOperator{Value: 10}))
	})
	t.Run("nil values", func(t *testing.T) {
		assert.True(t, EqOperator{Value: nil}.Equal(EqOperator{Value: nil}))
	})
	t.Run("different type", func(t *testing.T) {
		assert.False(t, EqOperator{Value: 5}.Equal(IsNullOperator{Value: true}))
	})
}

// =============================================================================
// ComparisonOperator equality
// =============================================================================

func TestComparisonOperatorEqual(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		a := ComparisonOperator{Op: "$gt", Value: 10}
		b := ComparisonOperator{Op: "$gt", Value: 10}
		assert.True(t, a.Equal(b))
	})
	t.Run("different op", func(t *testing.T) {
		a := ComparisonOperator{Op: "$gt", Value: 10}
		b := ComparisonOperator{Op: "$lt", Value: 10}
		assert.False(t, a.Equal(b))
	})
	t.Run("different value", func(t *testing.T) {
		a := ComparisonOperator{Op: "$gt", Value: 10}
		b := ComparisonOperator{Op: "$gt", Value: 20}
		assert.False(t, a.Equal(b))
	})
}

// =============================================================================
// RelOperator equality
// =============================================================================

func TestRelOperatorEqual(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		a := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}}
		b := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}}
		assert.True(t, a.Equal(b))
	})
	t.Run("different", func(t *testing.T) {
		a := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}}
		b := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "inactive"}}}}
		assert.False(t, a.Equal(b))
	})
}

// =============================================================================
// CompositeQuery equality
// =============================================================================

func TestCompositeQueryEqual(t *testing.T) {
	t.Run("equal", func(t *testing.T) {
		a := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}, "b": EqOperator{Value: 2}}}
		b := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}, "b": EqOperator{Value: 2}}}
		assert.True(t, a.Equal(b))
	})
	t.Run("different", func(t *testing.T) {
		a := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}}
		b := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 2}}}
		assert.False(t, a.Equal(b))
	})
}

// =============================================================================
// EqOperator merge
// =============================================================================

func TestEqOperatorMerge(t *testing.T) {
	t.Run("same value", func(t *testing.T) {
		left := EqOperator{Value: 5}
		right := EqOperator{Value: 5}
		result, err := left.Merge(right)
		assert.NoError(t, err)
		assert.True(t, result.Equal(EqOperator{Value: 5}))
	})
	t.Run("different value raises MergeConflict", func(t *testing.T) {
		left := EqOperator{Value: 5}
		right := EqOperator{Value: 10}
		_, err := left.Merge(right)
		var mc *MergeConflict
		assert.True(t, errors.As(err, &mc))
		assert.Equal(t, 5, mc.ExistingValue)
		assert.Equal(t, 10, mc.NewValue)
	})
	t.Run("nil values", func(t *testing.T) {
		left := EqOperator{Value: nil}
		right := EqOperator{Value: nil}
		result, err := left.Merge(right)
		assert.NoError(t, err)
		assert.True(t, result.Equal(EqOperator{Value: nil}))
	})
	t.Run("wrong type returns ErrUnsupportedMerge", func(t *testing.T) {
		left := EqOperator{Value: 5}
		right := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}}}
		_, err := left.Merge(right)
		assert.ErrorIs(t, err, ErrUnsupportedMerge)
	})
	t.Run("composite values", func(t *testing.T) {
		inner := CompositeQuery{Fields: map[string]IQueryOperator{"tenant": EqOperator{Value: 1}, "local": EqOperator{Value: 2}}}
		result, err := inner.Merge(inner)
		assert.NoError(t, err)
		assert.True(t, result.Equal(inner))
	})
}

// =============================================================================
// RelOperator merge
// =============================================================================

func TestRelOperatorMerge(t *testing.T) {
	t.Run("different fields", func(t *testing.T) {
		left := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}}
		right := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"type": EqOperator{Value: "premium"}}}}
		result, err := left.Merge(right)
		assert.NoError(t, err)
		rel := result.(RelOperator)
		assert.Equal(t, 2, len(rel.Query.Fields))
		assert.True(t, rel.Query.Fields["status"].Equal(EqOperator{Value: "active"}))
		assert.True(t, rel.Query.Fields["type"].Equal(EqOperator{Value: "premium"}))
	})
	t.Run("same field same value", func(t *testing.T) {
		left := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}}
		right := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}}
		result, err := left.Merge(right)
		assert.NoError(t, err)
		rel := result.(RelOperator)
		assert.True(t, rel.Query.Fields["status"].Equal(EqOperator{Value: "active"}))
	})
	t.Run("same field different value raises MergeConflict", func(t *testing.T) {
		left := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "active"}}}}
		right := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"status": EqOperator{Value: "inactive"}}}}
		_, err := left.Merge(right)
		var mc *MergeConflict
		assert.True(t, errors.As(err, &mc))
	})
	t.Run("nested deep merge", func(t *testing.T) {
		left := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
			"department": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"name": EqOperator{Value: "IT"}}}},
		}}}
		right := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{
			"department": RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"code": EqOperator{Value: "IT001"}}}},
		}}}
		result, err := left.Merge(right)
		assert.NoError(t, err)
		rel := result.(RelOperator)
		dept := rel.Query.Fields["department"].(RelOperator)
		assert.True(t, dept.Query.Fields["name"].Equal(EqOperator{Value: "IT"}))
		assert.True(t, dept.Query.Fields["code"].Equal(EqOperator{Value: "IT001"}))
	})
	t.Run("wrong type returns ErrUnsupportedMerge", func(t *testing.T) {
		left := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}}}
		_, err := left.Merge(EqOperator{Value: 5})
		assert.ErrorIs(t, err, ErrUnsupportedMerge)
	})
}

// =============================================================================
// CompositeQuery merge
// =============================================================================

func TestCompositeQueryMerge(t *testing.T) {
	t.Run("different fields", func(t *testing.T) {
		left := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}}
		right := CompositeQuery{Fields: map[string]IQueryOperator{"b": EqOperator{Value: 2}}}
		result, err := left.Merge(right)
		assert.NoError(t, err)
		cq := result.(CompositeQuery)
		assert.True(t, cq.Fields["a"].Equal(EqOperator{Value: 1}))
		assert.True(t, cq.Fields["b"].Equal(EqOperator{Value: 2}))
	})
	t.Run("same field same value", func(t *testing.T) {
		left := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}}
		right := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}}
		result, err := left.Merge(right)
		assert.NoError(t, err)
		cq := result.(CompositeQuery)
		assert.True(t, cq.Fields["a"].Equal(EqOperator{Value: 1}))
	})
	t.Run("same field different value raises MergeConflict", func(t *testing.T) {
		left := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}}
		right := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 2}}}
		_, err := left.Merge(right)
		var mc *MergeConflict
		assert.True(t, errors.As(err, &mc))
	})
	t.Run("nested deep merge", func(t *testing.T) {
		left := CompositeQuery{Fields: map[string]IQueryOperator{
			"address": CompositeQuery{Fields: map[string]IQueryOperator{"city": EqOperator{Value: "Moscow"}}},
		}}
		right := CompositeQuery{Fields: map[string]IQueryOperator{
			"address": CompositeQuery{Fields: map[string]IQueryOperator{"country": EqOperator{Value: "Russia"}}},
		}}
		result, err := left.Merge(right)
		assert.NoError(t, err)
		cq := result.(CompositeQuery)
		addr := cq.Fields["address"].(CompositeQuery)
		assert.True(t, addr.Fields["city"].Equal(EqOperator{Value: "Moscow"}))
		assert.True(t, addr.Fields["country"].Equal(EqOperator{Value: "Russia"}))
	})
	t.Run("wrong type returns ErrUnsupportedMerge", func(t *testing.T) {
		left := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}}
		_, err := left.Merge(EqOperator{Value: 5})
		assert.ErrorIs(t, err, ErrUnsupportedMerge)
	})
}

// =============================================================================
// IsNullOperator merge
// =============================================================================

func TestIsNullOperatorMerge(t *testing.T) {
	t.Run("same value true", func(t *testing.T) {
		result, err := IsNullOperator{Value: true}.Merge(IsNullOperator{Value: true})
		assert.NoError(t, err)
		assert.True(t, result.Equal(IsNullOperator{Value: true}))
	})
	t.Run("same value false", func(t *testing.T) {
		result, err := IsNullOperator{Value: false}.Merge(IsNullOperator{Value: false})
		assert.NoError(t, err)
		assert.True(t, result.Equal(IsNullOperator{Value: false}))
	})
	t.Run("different value raises MergeConflict", func(t *testing.T) {
		_, err := IsNullOperator{Value: true}.Merge(IsNullOperator{Value: false})
		var mc *MergeConflict
		assert.True(t, errors.As(err, &mc))
		assert.Equal(t, true, mc.ExistingValue)
		assert.Equal(t, false, mc.NewValue)
	})
	t.Run("wrong type returns ErrUnsupportedMerge", func(t *testing.T) {
		_, err := IsNullOperator{Value: true}.Merge(EqOperator{Value: nil})
		assert.ErrorIs(t, err, ErrUnsupportedMerge)
	})
}

// =============================================================================
// Cross-type merge
// =============================================================================

func TestCrossTypeMerge(t *testing.T) {
	t.Run("eq + rel", func(t *testing.T) {
		_, err := EqOperator{Value: 5}.Merge(RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}}})
		assert.ErrorIs(t, err, ErrUnsupportedMerge)
	})
	t.Run("composite + eq", func(t *testing.T) {
		_, err := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}}.Merge(EqOperator{Value: 5})
		assert.ErrorIs(t, err, ErrUnsupportedMerge)
	})
	t.Run("composite + rel", func(t *testing.T) {
		_, err := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}}.Merge(
			RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"b": EqOperator{Value: 2}}}},
		)
		assert.ErrorIs(t, err, ErrUnsupportedMerge)
	})
}

// =============================================================================
// NormalizeQuery
// =============================================================================

func TestNormalizeQuery(t *testing.T) {
	t.Run("primitive eq unchanged", func(t *testing.T) {
		op := EqOperator{Value: 5}
		result := NormalizeQuery(op)
		assert.True(t, result.Equal(EqOperator{Value: 5}))
	})
	t.Run("eq wrapping composite unwrapped", func(t *testing.T) {
		inner := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: 1}}}
		op := EqOperator{Value: inner}
		result := NormalizeQuery(op)
		cq, ok := result.(CompositeQuery)
		assert.True(t, ok)
		assert.True(t, cq.Fields["a"].Equal(EqOperator{Value: 1}))
	})
	t.Run("eq wrapping nested composite", func(t *testing.T) {
		inner := CompositeQuery{Fields: map[string]IQueryOperator{"b": EqOperator{Value: 2}}}
		op := EqOperator{Value: CompositeQuery{Fields: map[string]IQueryOperator{"a": inner}}}
		result := NormalizeQuery(op)
		cq := result.(CompositeQuery)
		assert.IsType(t, CompositeQuery{}, cq.Fields["a"])
	})
	t.Run("double wrapped eq", func(t *testing.T) {
		op := EqOperator{Value: EqOperator{Value: 5}}
		result := NormalizeQuery(op)
		eq := result.(EqOperator)
		assert.Equal(t, 5, eq.Value)
	})
	t.Run("rel with nested eq wrapping composite", func(t *testing.T) {
		inner := CompositeQuery{Fields: map[string]IQueryOperator{"x": EqOperator{Value: 1}, "y": EqOperator{Value: 2}}}
		op := RelOperator{Query: CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: inner}}}}
		result := NormalizeQuery(op)
		rel := result.(RelOperator)
		a := rel.Query.Fields["a"].(CompositeQuery)
		assert.True(t, a.Fields["x"].Equal(EqOperator{Value: 1}))
		assert.True(t, a.Fields["y"].Equal(EqOperator{Value: 2}))
	})
	t.Run("composite with nested eq wrapping composite", func(t *testing.T) {
		inner := CompositeQuery{Fields: map[string]IQueryOperator{"nested": EqOperator{Value: "value"}}}
		op := CompositeQuery{Fields: map[string]IQueryOperator{"a": EqOperator{Value: inner}}}
		result := NormalizeQuery(op)
		cq := result.(CompositeQuery)
		a := cq.Fields["a"].(CompositeQuery)
		assert.True(t, a.Fields["nested"].Equal(EqOperator{Value: "value"}))
	})
}
