package query

// QueryToDictVisitor converts IQueryOperator to map[string]any with operators.
type QueryToDictVisitor struct{}

func (v QueryToDictVisitor) Visit(op IQueryOperator) (map[string]any, error) {
	result, err := op.Accept(v)
	if err != nil {
		return nil, err
	}
	return result.(map[string]any), nil
}

func (v QueryToDictVisitor) VisitEq(op EqOperator) (any, error) {
	if inner, ok := op.Value.(IQueryOperator); ok {
		innerResult, err := inner.Accept(v)
		if err != nil {
			return nil, err
		}
		return map[string]any{"$eq": innerResult}, nil
	}
	return map[string]any{"$eq": op.Value}, nil
}

func (v QueryToDictVisitor) VisitComparison(op ComparisonOperator) (any, error) {
	return map[string]any{op.Op: op.Value}, nil
}

func (v QueryToDictVisitor) VisitIn(op InOperator) (any, error) {
	values := make([]any, len(op.Values))
	copy(values, op.Values)
	return map[string]any{"$in": values}, nil
}

func (v QueryToDictVisitor) VisitIsNull(op IsNullOperator) (any, error) {
	return map[string]any{"$is_null": op.Value}, nil
}

func (v QueryToDictVisitor) VisitAnd(op AndOperator) (any, error) {
	result := make(map[string]any)
	for _, operand := range op.Operands {
		accepted, err := operand.Accept(v)
		if err != nil {
			return nil, err
		}
		for k, val := range accepted.(map[string]any) {
			result[k] = val
		}
	}
	return result, nil
}

func (v QueryToDictVisitor) VisitOr(op OrOperator) (any, error) {
	items := make([]any, len(op.Operands))
	for i, operand := range op.Operands {
		item, err := operand.Accept(v)
		if err != nil {
			return nil, err
		}
		items[i] = item
	}
	return map[string]any{"$or": items}, nil
}

func (v QueryToDictVisitor) VisitRel(op RelOperator) (any, error) {
	inner, err := op.Query.Accept(v)
	if err != nil {
		return nil, err
	}
	return map[string]any{"$rel": inner}, nil
}

func (v QueryToDictVisitor) VisitComposite(op CompositeQuery) (any, error) {
	result := make(map[string]any, len(op.Fields))
	for k, fieldOp := range op.Fields {
		val, err := fieldOp.Accept(v)
		if err != nil {
			return nil, err
		}
		result[k] = val
	}
	return result, nil
}

// QueryToPlainValueVisitor converts IQueryOperator to plain values without operators.
type QueryToPlainValueVisitor struct{}

func (v QueryToPlainValueVisitor) Visit(op IQueryOperator) (any, error) {
	return op.Accept(v)
}

func (v QueryToPlainValueVisitor) VisitEq(op EqOperator) (any, error) {
	if inner, ok := op.Value.(IQueryOperator); ok {
		return inner.Accept(v)
	}
	return op.Value, nil
}

func (v QueryToPlainValueVisitor) VisitComparison(op ComparisonOperator) (any, error) {
	return map[string]any{op.Op: op.Value}, nil
}

func (v QueryToPlainValueVisitor) VisitIn(op InOperator) (any, error) {
	values := make([]any, len(op.Values))
	copy(values, op.Values)
	return map[string]any{"$in": values}, nil
}

func (v QueryToPlainValueVisitor) VisitIsNull(op IsNullOperator) (any, error) {
	return map[string]any{"$is_null": op.Value}, nil
}

func (v QueryToPlainValueVisitor) VisitAnd(op AndOperator) (any, error) {
	result := make(map[string]any)
	for _, operand := range op.Operands {
		accepted, err := operand.Accept(v)
		if err != nil {
			return nil, err
		}
		for k, val := range accepted.(map[string]any) {
			result[k] = val
		}
	}
	return result, nil
}

func (v QueryToPlainValueVisitor) VisitOr(op OrOperator) (any, error) {
	items := make([]any, len(op.Operands))
	for i, operand := range op.Operands {
		item, err := operand.Accept(v)
		if err != nil {
			return nil, err
		}
		items[i] = item
	}
	return map[string]any{"$or": items}, nil
}

func (v QueryToPlainValueVisitor) VisitRel(op RelOperator) (any, error) {
	return op.Query.Accept(v)
}

func (v QueryToPlainValueVisitor) VisitComposite(op CompositeQuery) (any, error) {
	result := make(map[string]any, len(op.Fields))
	for k, fieldOp := range op.Fields {
		val, err := fieldOp.Accept(v)
		if err != nil {
			return nil, err
		}
		result[k] = val
	}
	return result, nil
}

var (
	queryToDictVisitor       = QueryToDictVisitor{}
	queryToPlainValueVisitor = QueryToPlainValueVisitor{}
)

// QueryToDict converts IQueryOperator to map[string]any with operators.
func QueryToDict(op IQueryOperator) (map[string]any, error) {
	return queryToDictVisitor.Visit(op)
}

// QueryToPlainValue converts IQueryOperator to plain value without operators.
func QueryToPlainValue(op IQueryOperator) (any, error) {
	return queryToPlainValueVisitor.Visit(op)
}

// DictToQuery converts plain dict to query format with $eq operators.
func DictToQuery(value any) any {
	if m, ok := value.(map[string]any); ok {
		result := make(map[string]any, len(m))
		for k, v := range m {
			result[k] = DictToQuery(v)
		}
		return result
	}
	return map[string]any{"$eq": value}
}
