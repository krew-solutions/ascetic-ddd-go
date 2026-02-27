package query

import (
	"fmt"
	"strings"
)

const operatorPrefix = "$"

// QueryParser parses map[string]any / scalar into IQueryOperator tree.
type QueryParser struct{}

func (p QueryParser) Parse(query any) (IQueryOperator, error) {
	m, ok := query.(map[string]any)
	if !ok {
		return EqOperator{Value: query}, nil
	}

	if len(m) == 0 {
		return nil, fmt.Errorf("empty query dict")
	}

	operators := make(map[string]any)
	fields := make(map[string]any)
	for k, v := range m {
		if strings.HasPrefix(k, operatorPrefix) {
			operators[k] = v
		} else {
			fields[k] = v
		}
	}

	if len(operators) > 0 && len(fields) > 0 {
		opKeys := make([]string, 0, len(operators))
		for k := range operators {
			opKeys = append(opKeys, k)
		}
		fieldKeys := make([]string, 0, len(fields))
		for k := range fields {
			fieldKeys = append(fieldKeys, k)
		}
		return nil, fmt.Errorf(
			"cannot mix operators and fields at same level. Operators: %v, Fields: %v",
			opKeys, fieldKeys,
		)
	}

	if len(operators) > 0 {
		return p.parseOperators(operators)
	}
	cq, err := p.parseFields(fields)
	if err != nil {
		return nil, err
	}
	return cq, nil
}

func (p QueryParser) parseOperators(ops map[string]any) (IQueryOperator, error) {
	if len(ops) == 1 {
		for k, v := range ops {
			return p.parseSingleOperator(k, v)
		}
	}

	parsed := make([]IQueryOperator, 0, len(ops))
	for opName, opValue := range ops {
		op, err := p.parseSingleOperator(opName, opValue)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, op)
	}
	return AndOperator{Operands: parsed}, nil
}

func (p QueryParser) parseSingleOperator(opName string, opValue any) (IQueryOperator, error) {
	switch opName {
	case "$eq":
		return p.parseEq(opValue)
	case "$ne", "$gt", "$gte", "$lt", "$lte":
		return ComparisonOperator{Op: opName, Value: opValue}, nil
	case "$in":
		return p.parseIn(opValue)
	case "$or":
		return p.parseOr(opValue)
	case "$is_null":
		return p.parseIsNull(opValue)
	case "$rel":
		return p.parseRel(opValue)
	default:
		return nil, fmt.Errorf("unknown operator: %s", opName)
	}
}

func (p QueryParser) parseEq(value any) (IQueryOperator, error) {
	if m, ok := value.(map[string]any); ok {
		inner, err := p.Parse(m)
		if err != nil {
			return nil, err
		}
		return EqOperator{Value: inner}, nil
	}
	return EqOperator{Value: value}, nil
}

func (p QueryParser) parseOr(operands any) (IQueryOperator, error) {
	list, ok := operands.([]any)
	if !ok {
		return nil, fmt.Errorf("$or value must be list, got: %T", operands)
	}
	if len(list) < 2 {
		return nil, fmt.Errorf("$or requires at least 2 operands, got: %d", len(list))
	}
	parsed := make([]IQueryOperator, len(list))
	for i, item := range list {
		op, err := p.Parse(item)
		if err != nil {
			return nil, err
		}
		parsed[i] = op
	}
	return OrOperator{Operands: parsed}, nil
}

func (p QueryParser) parseIn(values any) (IQueryOperator, error) {
	list, ok := values.([]any)
	if !ok {
		return nil, fmt.Errorf("$in value must be list, got: %T", values)
	}
	if len(list) < 1 {
		return nil, fmt.Errorf("$in requires at least 1 value, got: %d", len(list))
	}
	result := make([]any, len(list))
	copy(result, list)
	return InOperator{Values: result}, nil
}

func (p QueryParser) parseIsNull(value any) (IQueryOperator, error) {
	b, ok := value.(bool)
	if !ok {
		return nil, fmt.Errorf("$is_null value must be bool, got: %T", value)
	}
	return IsNullOperator{Value: b}, nil
}

func (p QueryParser) parseRel(constraints any) (IQueryOperator, error) {
	m, ok := constraints.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("$rel value must be dict, got: %T", constraints)
	}
	cq, err := p.parseFields(m)
	if err != nil {
		return nil, err
	}
	return RelOperator{Query: cq}, nil
}

func (p QueryParser) parseFields(fields map[string]any) (CompositeQuery, error) {
	parsed := make(map[string]IQueryOperator, len(fields))
	for field, value := range fields {
		op, err := p.Parse(value)
		if err != nil {
			return CompositeQuery{}, err
		}
		parsed[field] = op
	}
	return CompositeQuery{Fields: parsed}, nil
}

// NormalizeQuery unwraps redundant EqOperator wrappers.
func NormalizeQuery(op IQueryOperator) IQueryOperator {
	switch o := op.(type) {
	case EqOperator:
		if inner, ok := o.Value.(IQueryOperator); ok {
			return NormalizeQuery(inner)
		}
		return o

	case RelOperator:
		normalized := NormalizeQuery(o.Query)
		return RelOperator{Query: normalized.(CompositeQuery)}

	case AndOperator:
		operands := make([]IQueryOperator, len(o.Operands))
		for i, operand := range o.Operands {
			operands[i] = NormalizeQuery(operand)
		}
		return AndOperator{Operands: operands}

	case OrOperator:
		operands := make([]IQueryOperator, len(o.Operands))
		for i, operand := range o.Operands {
			operands[i] = NormalizeQuery(operand)
		}
		return OrOperator{Operands: operands}

	case CompositeQuery:
		fields := make(map[string]IQueryOperator, len(o.Fields))
		for k, v := range o.Fields {
			fields[k] = NormalizeQuery(v)
		}
		return CompositeQuery{Fields: fields}

	default:
		return op
	}
}

// ParseQuery parses and normalizes a query.
func ParseQuery(query any) (IQueryOperator, error) {
	op, err := QueryParser{}.Parse(query)
	if err != nil {
		return nil, err
	}
	return NormalizeQuery(op), nil
}
