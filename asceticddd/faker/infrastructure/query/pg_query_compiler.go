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
	Resolve(field string) *RelationInfo
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

func (c *PgQueryCompiler) VisitComposite(op domainquery.CompositeQuery) (any, error) {
	for field, fieldOp := range op.Fields {
		if _, ok := fieldOp.(domainquery.RelOperator); ok {
			err := c.compileRelField(field, fieldOp.(domainquery.RelOperator))
			if err != nil {
				return nil, err
			}
		} else {
			c.fieldPath = append(c.fieldPath, field)
			_, err := fieldOp.Accept(c)
			if err != nil {
				return nil, err
			}
			c.fieldPath = c.fieldPath[:len(c.fieldPath)-1]
		}
	}
	return nil, nil
}

func (c *PgQueryCompiler) VisitRel(op domainquery.RelOperator) (any, error) {
	if c.relationResolver == nil {
		return nil, fmt.Errorf("cannot compile $rel without relation_resolver")
	}
	if len(c.fieldPath) > 0 {
		field := c.fieldPath[len(c.fieldPath)-1]
		c.fieldPath = c.fieldPath[:len(c.fieldPath)-1]
		err := c.compileRelField(field, op)
		if err != nil {
			return nil, err
		}
	} else {
		_, err := op.Query.Accept(c)
		if err != nil {
			return nil, err
		}
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

func (c *PgQueryCompiler) compileRelField(field string, op domainquery.RelOperator) error {
	if c.relationResolver == nil {
		return fmt.Errorf("cannot compile $rel without relation_resolver")
	}

	ri := c.relationResolver.Resolve(field)

	if ri != nil {
		c.buildExistsSubquery(field, op, ri)
	} else {
		nested := toDict(op.Query)
		if nested != nil {
			c.sqlParts = append(c.sqlParts, fmt.Sprintf("%s @> ?", c.targetValueExpr))
			c.params = append(c.params, encode(map[string]any{field: nested}))
		}
	}
	return nil
}

func (c *PgQueryCompiler) buildExistsSubquery(field string, op domainquery.RelOperator, ri *RelationInfo) {
	alias := c.nextAlias()

	nested := NewPgQueryCompiler(
		fmt.Sprintf("%s.value", alias),
		ri.NestedResolver,
		c.aliasSeq,
	)
	op.Query.Accept(nested)
	nested.flushEq()

	if nestedSql := nested.sql(); nestedSql != "" {
		sql := fmt.Sprintf(
			"EXISTS (SELECT 1 FROM %s %s WHERE %s AND %s.%s = %s->'%s')",
			ri.Table, alias, nestedSql, alias, ri.PkField, c.targetValueExpr, field,
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
