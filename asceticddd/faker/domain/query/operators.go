package query

import (
	"errors"
	"fmt"
	"reflect"
)

var ErrUnsupportedMerge = errors.New("unsupported merge between different operator types")

type MergeConflict struct {
	ExistingValue any
	NewValue      any
}

func (e *MergeConflict) Error() string {
	return fmt.Sprintf("cannot merge %v with %v", e.ExistingValue, e.NewValue)
}

type IQueryVisitor interface {
	VisitEq(op EqOperator) (any, error)
	VisitComparison(op ComparisonOperator) (any, error)
	VisitIn(op InOperator) (any, error)
	VisitIsNull(op IsNullOperator) (any, error)
	VisitAnd(op AndOperator) (any, error)
	VisitOr(op OrOperator) (any, error)
	VisitRel(op RelOperator) (any, error)
	VisitComposite(op CompositeQuery) (any, error)
}

type IQueryOperator interface {
	Accept(visitor IQueryVisitor) (any, error)
	Equal(other IQueryOperator) bool
	Merge(other IQueryOperator) (IQueryOperator, error)
}

// EqOperator represents equality check: {'$eq': value}
type EqOperator struct {
	Value any
}

func (o EqOperator) Accept(visitor IQueryVisitor) (any, error) {
	return visitor.VisitEq(o)
}

func (o EqOperator) Equal(other IQueryOperator) bool {
	oo, ok := other.(EqOperator)
	if !ok {
		return false
	}
	return reflect.DeepEqual(o.Value, oo.Value)
}

func (o EqOperator) Merge(other IQueryOperator) (IQueryOperator, error) {
	oo, ok := other.(EqOperator)
	if !ok {
		return nil, ErrUnsupportedMerge
	}
	if reflect.DeepEqual(o.Value, oo.Value) {
		return o, nil
	}
	return nil, &MergeConflict{ExistingValue: o.Value, NewValue: oo.Value}
}

func (o EqOperator) String() string {
	return fmt.Sprintf("EqOperator(%v)", o.Value)
}

// ComparisonOperator represents comparison: {'$ne': value}, {'$gt': value}, etc.
type ComparisonOperator struct {
	Op    string
	Value any
}

var comparisonSupportedOps = map[string]struct{}{
	"$ne": {}, "$gt": {}, "$gte": {}, "$lt": {}, "$lte": {},
}

func (o ComparisonOperator) Accept(visitor IQueryVisitor) (any, error) {
	return visitor.VisitComparison(o)
}

func (o ComparisonOperator) Equal(other IQueryOperator) bool {
	oo, ok := other.(ComparisonOperator)
	if !ok {
		return false
	}
	return o.Op == oo.Op && reflect.DeepEqual(o.Value, oo.Value)
}

func (o ComparisonOperator) Merge(other IQueryOperator) (IQueryOperator, error) {
	oo, ok := other.(ComparisonOperator)
	if !ok {
		return nil, ErrUnsupportedMerge
	}
	if o.Op == oo.Op && reflect.DeepEqual(o.Value, oo.Value) {
		return o, nil
	}
	return nil, &MergeConflict{
		ExistingValue: [2]any{o.Op, o.Value},
		NewValue:      [2]any{oo.Op, oo.Value},
	}
}

func (o ComparisonOperator) String() string {
	return fmt.Sprintf("ComparisonOperator(%s, %v)", o.Op, o.Value)
}

// InOperator represents membership check: {'$in': [value1, value2, ...]}
type InOperator struct {
	Values []any
}

func (o InOperator) Accept(visitor IQueryVisitor) (any, error) {
	return visitor.VisitIn(o)
}

func (o InOperator) Equal(other IQueryOperator) bool {
	oo, ok := other.(InOperator)
	if !ok {
		return false
	}
	return reflect.DeepEqual(o.Values, oo.Values)
}

func (o InOperator) Merge(other IQueryOperator) (IQueryOperator, error) {
	oo, ok := other.(InOperator)
	if !ok {
		return nil, ErrUnsupportedMerge
	}
	if reflect.DeepEqual(o.Values, oo.Values) {
		return o, nil
	}
	return nil, &MergeConflict{ExistingValue: o.Values, NewValue: oo.Values}
}

func (o InOperator) String() string {
	return fmt.Sprintf("InOperator(%v)", o.Values)
}

// IsNullOperator represents null check: {'$is_null': true/false}
type IsNullOperator struct {
	Value bool
}

func (o IsNullOperator) Accept(visitor IQueryVisitor) (any, error) {
	return visitor.VisitIsNull(o)
}

func (o IsNullOperator) Equal(other IQueryOperator) bool {
	oo, ok := other.(IsNullOperator)
	if !ok {
		return false
	}
	return o.Value == oo.Value
}

func (o IsNullOperator) Merge(other IQueryOperator) (IQueryOperator, error) {
	oo, ok := other.(IsNullOperator)
	if !ok {
		return nil, ErrUnsupportedMerge
	}
	if o.Value == oo.Value {
		return o, nil
	}
	return nil, &MergeConflict{ExistingValue: o.Value, NewValue: oo.Value}
}

func (o IsNullOperator) String() string {
	return fmt.Sprintf("IsNullOperator(%v)", o.Value)
}

// AndOperator represents implicit AND of operators at the same level.
type AndOperator struct {
	Operands []IQueryOperator
}

func (o AndOperator) Accept(visitor IQueryVisitor) (any, error) {
	return visitor.VisitAnd(o)
}

func (o AndOperator) Equal(other IQueryOperator) bool {
	oo, ok := other.(AndOperator)
	if !ok {
		return false
	}
	if len(o.Operands) != len(oo.Operands) {
		return false
	}
	for i := range o.Operands {
		if !o.Operands[i].Equal(oo.Operands[i]) {
			return false
		}
	}
	return true
}

func (o AndOperator) Merge(other IQueryOperator) (IQueryOperator, error) {
	oo, ok := other.(AndOperator)
	if !ok {
		return nil, ErrUnsupportedMerge
	}
	if o.Equal(oo) {
		return o, nil
	}
	return nil, &MergeConflict{ExistingValue: o.Operands, NewValue: oo.Operands}
}

func (o AndOperator) String() string {
	return fmt.Sprintf("AndOperator(%v)", o.Operands)
}

// OrOperator represents logical OR: {'$or': [expr1, expr2, ...]}
type OrOperator struct {
	Operands []IQueryOperator
}

func (o OrOperator) Accept(visitor IQueryVisitor) (any, error) {
	return visitor.VisitOr(o)
}

func (o OrOperator) Equal(other IQueryOperator) bool {
	oo, ok := other.(OrOperator)
	if !ok {
		return false
	}
	if len(o.Operands) != len(oo.Operands) {
		return false
	}
	for i := range o.Operands {
		if !o.Operands[i].Equal(oo.Operands[i]) {
			return false
		}
	}
	return true
}

func (o OrOperator) Merge(other IQueryOperator) (IQueryOperator, error) {
	oo, ok := other.(OrOperator)
	if !ok {
		return nil, ErrUnsupportedMerge
	}
	if o.Equal(oo) {
		return o, nil
	}
	return nil, &MergeConflict{ExistingValue: o.Operands, NewValue: oo.Operands}
}

func (o OrOperator) String() string {
	return fmt.Sprintf("OrOperator(%v)", o.Operands)
}

// RelOperator represents constraints on a related aggregate: {'$rel': {...}}
type RelOperator struct {
	Query CompositeQuery
}

func (o RelOperator) Accept(visitor IQueryVisitor) (any, error) {
	return visitor.VisitRel(o)
}

func (o RelOperator) Equal(other IQueryOperator) bool {
	oo, ok := other.(RelOperator)
	if !ok {
		return false
	}
	return o.Query.Equal(oo.Query)
}

func (o RelOperator) Merge(other IQueryOperator) (IQueryOperator, error) {
	oo, ok := other.(RelOperator)
	if !ok {
		return nil, ErrUnsupportedMerge
	}
	merged, err := o.Query.Merge(oo.Query)
	if err != nil {
		return nil, err
	}
	return RelOperator{Query: merged.(CompositeQuery)}, nil
}

func (o RelOperator) String() string {
	return fmt.Sprintf("RelOperator(%v)", o.Query)
}

// CompositeQuery represents a multi-field query: {'field1': op1, 'field2': op2, ...}
type CompositeQuery struct {
	Fields map[string]IQueryOperator
}

func (o CompositeQuery) Accept(visitor IQueryVisitor) (any, error) {
	return visitor.VisitComposite(o)
}

func (o CompositeQuery) Equal(other IQueryOperator) bool {
	oo, ok := other.(CompositeQuery)
	if !ok {
		return false
	}
	if len(o.Fields) != len(oo.Fields) {
		return false
	}
	for k, v := range o.Fields {
		ov, exists := oo.Fields[k]
		if !exists {
			return false
		}
		if !v.Equal(ov) {
			return false
		}
	}
	return true
}

func (o CompositeQuery) Merge(other IQueryOperator) (IQueryOperator, error) {
	oo, ok := other.(CompositeQuery)
	if !ok {
		return nil, ErrUnsupportedMerge
	}
	merged := make(map[string]IQueryOperator, len(o.Fields))
	for k, v := range o.Fields {
		merged[k] = v
	}
	for field, op := range oo.Fields {
		if existing, exists := merged[field]; exists {
			m, err := existing.Merge(op)
			if err != nil {
				return nil, err
			}
			merged[field] = m
		} else {
			merged[field] = op
		}
	}
	return CompositeQuery{Fields: merged}, nil
}

func (o CompositeQuery) String() string {
	return fmt.Sprintf("CompositeQuery(%v)", o.Fields)
}
