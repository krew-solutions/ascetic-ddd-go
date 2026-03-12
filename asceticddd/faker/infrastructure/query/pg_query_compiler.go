package query

import (
	"fmt"
	"strings"

	domainquery "github.com/krew-solutions/ascetic-ddd-go/asceticddd/faker/domain/query"
)

type Jsonb struct {
	Obj any
}

type RelationInfo struct {
	Table          string
	PkField        string
	NestedResolver IRelationResolver
}

type IRelationResolver interface {
	Resolve(field *string) *RelationInfo
	// Descend returns a resolver scoped to the child provider for the given field.
	// Used when entering nested CompositeQuery fields to ensure
	// the resolver navigates the correct level of the provider tree.
	Descend(field string) IRelationResolver
}

var sqlOps = map[string]string{
	"$gt":  ">",
	"$gte": ">=",
	"$lt":  "<",
	"$lte": "<=",
}

type PgQueryCompiler struct {
	targetValueExpr  string
	relationResolver IRelationResolver
	aliasSeq         *int
	fieldPath        []string
	eqValues         map[string]any
	sqlParts         []string
	params           []any
}

func NewPgQueryCompiler(targetValueExpr string, relationResolver IRelationResolver, aliasSeq *int) *PgQueryCompiler {
	if targetValueExpr == "" {
		targetValueExpr = "value"
	}
	if aliasSeq == nil {
		seq := 0
		aliasSeq = &seq
	}
	return &PgQueryCompiler{
		targetValueExpr:  targetValueExpr,
		relationResolver: relationResolver,
		aliasSeq:         aliasSeq,
		eqValues:         map[string]any{},
	}
}

func (c *PgQueryCompiler) Compile(query domainquery.IQueryOperator) (string, []any, error) {
	c.fieldPath = nil
	c.eqValues = map[string]any{}
	c.sqlParts = nil
	c.params = nil
	_, err := query.Accept(c)
	if err != nil {
		return "", nil, err
	}
	c.flushEq()
	sql := c.sql()
	sql = replaceParamMarkers(sql)
	return sql, c.params, nil
}

func (c *PgQueryCompiler) sql() string {
	if len(c.sqlParts) == 0 {
		return ""
	}
	return strings.Join(c.sqlParts, " AND ")
}

func (c *PgQueryCompiler) nextAlias() string {
	*c.aliasSeq++
	return fmt.Sprintf("rt%d", *c.aliasSeq)
}

// --- Visitor methods ---

func (c *PgQueryCompiler) VisitEq(op domainquery.EqOperator) (any, error) {
	if len(c.fieldPath) > 0 {
		c.collectEq(op.Value)
	} else {
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("%s @> ?", c.targetValueExpr))
		c.params = append(c.params, encode(op.Value))
	}
	return nil, nil
}

func (c *PgQueryCompiler) VisitComparison(op domainquery.ComparisonOperator) (any, error) {
	if op.Op == "$ne" {
		c.compileNe(op.Value)
		return nil, nil
	}
	sqlOp := sqlOps[op.Op]
	jsonPath := c.jsonPathExpr()
	c.sqlParts = append(c.sqlParts, fmt.Sprintf("%s %s ?", jsonPath, sqlOp))
	c.params = append(c.params, op.Value)
	return nil, nil
}

func (c *PgQueryCompiler) VisitIn(op domainquery.InOperator) (any, error) {
	var orParts []string
	for _, value := range op.Values {
		if len(c.fieldPath) > 0 {
			nested := buildNestedDict(c.fieldPath, value)
			orParts = append(orParts, fmt.Sprintf("%s @> ?", c.targetValueExpr))
			c.params = append(c.params, encode(nested))
		} else {
			orParts = append(orParts, fmt.Sprintf("%s @> ?", c.targetValueExpr))
			c.params = append(c.params, encode(value))
		}
	}
	if len(orParts) == 1 {
		c.sqlParts = append(c.sqlParts, orParts[0])
	} else {
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("(%s)", strings.Join(orParts, " OR ")))
	}
	return nil, nil
}

func (c *PgQueryCompiler) VisitIsNull(op domainquery.IsNullOperator) (any, error) {
	var jsonPath string
	if len(c.fieldPath) > 0 {
		jsonPath = c.jsonPathExpr()
	} else {
		jsonPath = c.targetValueExpr
	}
	if op.Value {
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("%s IS NULL", jsonPath))
	} else {
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("%s IS NOT NULL", jsonPath))
	}
	return nil, nil
}

func (c *PgQueryCompiler) VisitAnd(op domainquery.AndOperator) (any, error) {
	for _, operand := range op.Operands {
		_, err := operand.Accept(c)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (c *PgQueryCompiler) VisitOr(op domainquery.OrOperator) (any, error) {
	var orParts []string
	for _, operand := range op.Operands {
		sub := NewPgQueryCompiler(c.targetValueExpr, c.relationResolver, c.aliasSeq)
		sub.fieldPath = make([]string, len(c.fieldPath))
		copy(sub.fieldPath, c.fieldPath)
		_, err := operand.Accept(sub)
		if err != nil {
			return nil, err
		}
		sub.flushEq()
		if subSql := sub.sql(); subSql != "" {
			orParts = append(orParts, subSql)
			c.params = append(c.params, sub.params...)
		}
	}
	if len(orParts) > 0 {
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("(%s)", strings.Join(orParts, " OR ")))
	}
	return nil, nil
}

func (c *PgQueryCompiler) VisitNot(op domainquery.NotOperator) (any, error) {
	sub := NewPgQueryCompiler(c.targetValueExpr, c.relationResolver, c.aliasSeq)
	sub.fieldPath = make([]string, len(c.fieldPath))
	copy(sub.fieldPath, c.fieldPath)
	_, err := op.Operand.Accept(sub)
	if err != nil {
		return nil, err
	}
	sub.flushEq()
	if subSql := sub.sql(); subSql != "" {
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("NOT (%s)", subSql))
		c.params = append(c.params, sub.params...)
	}
	return nil, nil
}

func (c *PgQueryCompiler) VisitAnyElement(op domainquery.AnyElementOperator) (any, error) {
	var jsonPath string
	if len(c.fieldPath) > 0 {
		jsonPath = c.jsonPathExpr()
	} else {
		jsonPath = c.targetValueExpr
	}
	alias := c.nextAlias()
	sub := NewPgQueryCompiler(alias, c.relationResolver, c.aliasSeq)
	_, err := op.Query.Accept(sub)
	if err != nil {
		return nil, err
	}
	sub.flushEq()
	if subSql := sub.sql(); subSql != "" {
		sql := fmt.Sprintf(
			"EXISTS (SELECT 1 FROM jsonb_array_elements(%s) AS %s WHERE %s)",
			jsonPath, alias, subSql,
		)
		c.sqlParts = append(c.sqlParts, sql)
		c.params = append(c.params, sub.params...)
	}
	return nil, nil
}

func (c *PgQueryCompiler) VisitAllElements(op domainquery.AllElementsOperator) (any, error) {
	var jsonPath string
	if len(c.fieldPath) > 0 {
		jsonPath = c.jsonPathExpr()
	} else {
		jsonPath = c.targetValueExpr
	}
	alias := c.nextAlias()
	sub := NewPgQueryCompiler(alias, c.relationResolver, c.aliasSeq)
	_, err := op.Query.Accept(sub)
	if err != nil {
		return nil, err
	}
	sub.flushEq()
	if subSql := sub.sql(); subSql != "" {
		sql := fmt.Sprintf(
			"NOT EXISTS (SELECT 1 FROM jsonb_array_elements(%s) AS %s WHERE NOT (%s))",
			jsonPath, alias, subSql,
		)
		c.sqlParts = append(c.sqlParts, sql)
		c.params = append(c.params, sub.params...)
	}
	return nil, nil
}

func (c *PgQueryCompiler) VisitLen(op domainquery.LenOperator) (any, error) {
	var jsonPath string
	if len(c.fieldPath) > 0 {
		jsonPath = c.jsonPathExpr()
	} else {
		jsonPath = c.targetValueExpr
	}
	lenExpr := fmt.Sprintf("jsonb_array_length(%s)", jsonPath)
	scalar := NewScalarPgQueryCompiler(lenExpr)
	_, err := op.Query.Accept(scalar)
	if err != nil {
		return nil, err
	}
	if scalarSql := scalar.sql(); scalarSql != "" {
		c.sqlParts = append(c.sqlParts, scalarSql)
		c.params = append(c.params, scalar.params...)
	}
	return nil, nil
}

func (c *PgQueryCompiler) VisitComposite(op domainquery.CompositeQuery) (any, error) {
	for field, fieldOp := range op.Fields {
		if _, ok := fieldOp.(domainquery.RelOperator); ok {
			f := field
			err := c.compileRelField(&f, fieldOp.(domainquery.RelOperator))
			if err != nil {
				return nil, err
			}
		} else {
			c.fieldPath = append(c.fieldPath, field)
			oldResolver := c.relationResolver
			if c.relationResolver != nil {
				descended := c.relationResolver.Descend(field)
				if descended != nil {
					c.relationResolver = descended
				}
			}
			_, err := fieldOp.Accept(c)
			if err != nil {
				return nil, err
			}
			c.relationResolver = oldResolver
			c.fieldPath = c.fieldPath[:len(c.fieldPath)-1]
		}
	}
	return nil, nil
}

func (c *PgQueryCompiler) VisitRel(op domainquery.RelOperator) (any, error) {
	if c.relationResolver == nil {
		return nil, fmt.Errorf("cannot compile $rel without relation_resolver")
	}
	var field *string
	if len(c.fieldPath) > 0 {
		f := c.fieldPath[len(c.fieldPath)-1]
		c.fieldPath = c.fieldPath[:len(c.fieldPath)-1]
		field = &f
	}
	err := c.compileRelField(field, op)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

// --- Eq collection ---

func (c *PgQueryCompiler) collectEq(value any) {
	target := c.eqValues
	for _, key := range c.fieldPath[:len(c.fieldPath)-1] {
		if _, ok := target[key]; !ok {
			target[key] = map[string]any{}
		}
		target = target[key].(map[string]any)
	}
	target[c.fieldPath[len(c.fieldPath)-1]] = value
}

func (c *PgQueryCompiler) flushEq() {
	if len(c.eqValues) > 0 {
		c.sqlParts = append([]string{fmt.Sprintf("%s @> ?", c.targetValueExpr)}, c.sqlParts...)
		c.params = append([]any{encode(c.eqValues)}, c.params...)
	}
}

// --- $rel compilation ---

func (c *PgQueryCompiler) compileRelField(field *string, op domainquery.RelOperator) error {
	if c.relationResolver == nil {
		return fmt.Errorf("cannot compile $rel without relation_resolver")
	}

	ri := c.relationResolver.Resolve(field)

	if ri != nil {
		c.buildExistsSubquery(field, op, ri)
	} else if field != nil {
		nested := toDict(op.Query)
		if nested != nil {
			c.sqlParts = append(c.sqlParts, fmt.Sprintf("%s @> ?", c.targetValueExpr))
			c.params = append(c.params, encode(map[string]any{*field: nested}))
		}
	}
	return nil
}

func (c *PgQueryCompiler) buildExistsSubquery(field *string, op domainquery.RelOperator, ri *RelationInfo) {
	alias := c.nextAlias()

	nested := NewPgQueryCompiler(
		fmt.Sprintf("%s.value", alias),
		ri.NestedResolver,
		c.aliasSeq,
	)
	op.Query.Accept(nested)
	nested.flushEq()

	if nestedSql := nested.sql(); nestedSql != "" {
		var joinExpr string
		if field != nil {
			joinExpr = fmt.Sprintf("%s->'%s'", c.jsonPathExpr(), *field)
		} else {
			joinExpr = c.targetValueExpr
		}
		sql := fmt.Sprintf(
			"EXISTS (SELECT 1 FROM %s %s WHERE %s AND %s.%s = %s)",
			ri.Table, alias, nestedSql, alias, ri.PkField, joinExpr,
		)
		c.sqlParts = append(c.sqlParts, sql)
		c.params = append(c.params, nested.params...)
	}
}

// --- Helpers ---

func (c *PgQueryCompiler) jsonPathExpr() string {
	expr := c.targetValueExpr
	for _, key := range c.fieldPath {
		expr += fmt.Sprintf("->'%s'", key)
	}
	return expr
}

func (c *PgQueryCompiler) compileNe(value any) {
	if len(c.fieldPath) > 0 {
		nested := buildNestedDict(c.fieldPath, value)
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("NOT (%s @> ?)", c.targetValueExpr))
		c.params = append(c.params, encode(nested))
	} else {
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("NOT (%s @> ?)", c.targetValueExpr))
		c.params = append(c.params, encode(value))
	}
}

func toDict(op domainquery.IQueryOperator) any {
	switch o := op.(type) {
	case domainquery.EqOperator:
		return o.Value
	case domainquery.CompositeQuery:
		result := map[string]any{}
		for field, fieldOp := range o.Fields {
			val := toDict(fieldOp)
			if val != nil {
				result[field] = val
			}
		}
		if len(result) == 0 {
			return nil
		}
		return result
	default:
		return nil
	}
}

func encode(obj any) Jsonb {
	return Jsonb{Obj: obj}
}

func buildNestedDict(fieldPath []string, value any) map[string]any {
	nested := map[string]any{}
	target := nested
	for _, key := range fieldPath[:len(fieldPath)-1] {
		target[key] = map[string]any{}
		target = target[key].(map[string]any)
	}
	target[fieldPath[len(fieldPath)-1]] = value
	return nested
}

func replaceParamMarkers(sql string) string {
	var b strings.Builder
	idx := 1
	for i := 0; i < len(sql); i++ {
		if sql[i] == '?' {
			b.WriteString(fmt.Sprintf("$%d", idx))
			idx++
		} else {
			b.WriteByte(sql[i])
		}
	}
	return b.String()
}

// ScalarPgQueryCompiler compiles IQueryOperator tree against a scalar SQL expression.
// Unlike PgQueryCompiler which uses JSONB containment (@>),
// this generates standard SQL comparisons (=, >, <, etc.)
// for plain values like jsonb_array_length().
type ScalarPgQueryCompiler struct {
	targetExpr string
	sqlParts   []string
	params     []any
}

func NewScalarPgQueryCompiler(targetExpr string) *ScalarPgQueryCompiler {
	return &ScalarPgQueryCompiler{targetExpr: targetExpr}
}

func (c *ScalarPgQueryCompiler) Compile(query domainquery.IQueryOperator) (string, []any, error) {
	c.sqlParts = nil
	c.params = nil
	_, err := query.Accept(c)
	if err != nil {
		return "", nil, err
	}
	sql := c.sql()
	sql = replaceParamMarkers(sql)
	return sql, c.params, nil
}

func (c *ScalarPgQueryCompiler) sql() string {
	if len(c.sqlParts) == 0 {
		return ""
	}
	return strings.Join(c.sqlParts, " AND ")
}

func (c *ScalarPgQueryCompiler) VisitEq(op domainquery.EqOperator) (any, error) {
	c.sqlParts = append(c.sqlParts, fmt.Sprintf("%s = ?", c.targetExpr))
	c.params = append(c.params, op.Value)
	return nil, nil
}

func (c *ScalarPgQueryCompiler) VisitComparison(op domainquery.ComparisonOperator) (any, error) {
	if op.Op == "$ne" {
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("%s != ?", c.targetExpr))
		c.params = append(c.params, op.Value)
	} else {
		sqlOp := sqlOps[op.Op]
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("%s %s ?", c.targetExpr, sqlOp))
		c.params = append(c.params, op.Value)
	}
	return nil, nil
}

func (c *ScalarPgQueryCompiler) VisitIn(op domainquery.InOperator) (any, error) {
	var orParts []string
	for _, value := range op.Values {
		orParts = append(orParts, fmt.Sprintf("%s = ?", c.targetExpr))
		c.params = append(c.params, value)
	}
	if len(orParts) == 1 {
		c.sqlParts = append(c.sqlParts, orParts[0])
	} else {
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("(%s)", strings.Join(orParts, " OR ")))
	}
	return nil, nil
}

func (c *ScalarPgQueryCompiler) VisitIsNull(op domainquery.IsNullOperator) (any, error) {
	if op.Value {
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("%s IS NULL", c.targetExpr))
	} else {
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("%s IS NOT NULL", c.targetExpr))
	}
	return nil, nil
}

func (c *ScalarPgQueryCompiler) VisitNot(op domainquery.NotOperator) (any, error) {
	sub := NewScalarPgQueryCompiler(c.targetExpr)
	_, err := op.Operand.Accept(sub)
	if err != nil {
		return nil, err
	}
	if subSql := sub.sql(); subSql != "" {
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("NOT (%s)", subSql))
		c.params = append(c.params, sub.params...)
	}
	return nil, nil
}

func (c *ScalarPgQueryCompiler) VisitAnd(op domainquery.AndOperator) (any, error) {
	for _, operand := range op.Operands {
		_, err := operand.Accept(c)
		if err != nil {
			return nil, err
		}
	}
	return nil, nil
}

func (c *ScalarPgQueryCompiler) VisitOr(op domainquery.OrOperator) (any, error) {
	var orParts []string
	for _, operand := range op.Operands {
		sub := NewScalarPgQueryCompiler(c.targetExpr)
		_, err := operand.Accept(sub)
		if err != nil {
			return nil, err
		}
		if subSql := sub.sql(); subSql != "" {
			orParts = append(orParts, subSql)
			c.params = append(c.params, sub.params...)
		}
	}
	if len(orParts) > 0 {
		c.sqlParts = append(c.sqlParts, fmt.Sprintf("(%s)", strings.Join(orParts, " OR ")))
	}
	return nil, nil
}

func (c *ScalarPgQueryCompiler) VisitAnyElement(op domainquery.AnyElementOperator) (any, error) {
	return nil, fmt.Errorf("$any is not supported in scalar predicate context")
}

func (c *ScalarPgQueryCompiler) VisitAllElements(op domainquery.AllElementsOperator) (any, error) {
	return nil, fmt.Errorf("$all is not supported in scalar predicate context")
}

func (c *ScalarPgQueryCompiler) VisitLen(op domainquery.LenOperator) (any, error) {
	return nil, fmt.Errorf("$len is not supported in scalar predicate context")
}

func (c *ScalarPgQueryCompiler) VisitRel(op domainquery.RelOperator) (any, error) {
	return nil, fmt.Errorf("$rel is not supported in scalar predicate context")
}

func (c *ScalarPgQueryCompiler) VisitComposite(op domainquery.CompositeQuery) (any, error) {
	return nil, fmt.Errorf("CompositeQuery is not supported in scalar predicate context")
}
