package specification

import (
	"fmt"
	"strings"

	"github.com/jinzhu/inflection"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

// SqlFragment is the typed result produced by PostgresqlVisitor.
type SqlFragment struct {
	SQL    string
	Params []any
}

// Compile transforms a domain specification and then renders it as SQL.
func Compile(context Context, exp s.Visitable) (sql string, params []any, err error) {
	transformed, err := NewTransformVisitor(context).Transform(exp)
	if err != nil {
		return "", nil, err
	}
	fragment, err := NewPostgresqlVisitor().Compile(transformed)
	if err != nil {
		return "", nil, err
	}
	return fragment.SQL, fragment.Params, nil
}

// CompileToSQL compiles AST directly to SQL without context transformation.
// Useful for generated code where AST is already in the right form.
func CompileToSQL(exp s.Visitable, opts ...PostgresqlVisitorOption) (sql string, params []any, err error) {
	fragment, err := NewPostgresqlVisitor(opts...).Compile(exp)
	if err != nil {
		return "", nil, err
	}
	return fragment.SQL, fragment.Params, nil
}

type PostgresqlVisitorOption func(*PostgresqlVisitor)

func PlaceholderIndex(index uint8) PostgresqlVisitorOption {
	return func(v *PostgresqlVisitor) {
		v.counters.placeholderIndex = index
	}
}

// WithSchema sets the schema registry for relational collection support.
func WithSchema(schema *SchemaRegistry) PostgresqlVisitorOption {
	return func(v *PostgresqlVisitor) {
		v.schema = schema
	}
}

// counters holds monotonic counters that cross sub-visitor boundaries:
// every $-placeholder and every wildcard alias must get a globally-unique
// number, so they live in a mutable container shared by reference.
type counters struct {
	placeholderIndex uint8
	wildcardCounter  int
}

func NewPostgresqlVisitor(opts ...PostgresqlVisitorOption) *PostgresqlVisitor {
	v := &PostgresqlVisitor{
		counters:          &counters{},
		precedenceMapping: buildPrecedenceMapping(),
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

// buildPrecedenceMapping returns PostgreSQL operator precedence keyed by
// "operator associativity". Higher numbers bind tighter.
//
// Based on: https://www.postgresql.org/docs/14/sql-syntax-lexical.html#SQL-PRECEDENCE-TABLE
func buildPrecedenceMapping() map[string]int {
	m := make(map[string]int)
	assign := func(prec int, keys ...string) {
		for _, k := range keys {
			m[k] = prec
		}
	}
	assign(160, ". LEFT", ":: LEFT")
	assign(150, "[ LEFT")
	assign(140, "+ RIGHT", "- RIGHT")
	assign(130, "^ LEFT")
	assign(120, "* LEFT", "/ LEFT", "% LEFT")
	assign(110, "+ LEFT", "- LEFT")
	// all other native and user-defined operators 👇️
	assign(100, "(any other operator) LEFT")
	assign(90, "BETWEEN NON", "IN NON", "LIKE NON", "ILIKE NON", "SIMILAR NON")
	assign(80, "< NON", "> NON", "= NON", "<= NON", ">= NON", "!= NON")
	assign(70, "IS NON", "ISNULL NON", "NOTNULL NON")
	assign(60, "NOT RIGHT")
	assign(50, "AND LEFT")
	assign(40, "OR LEFT")
	return m
}

// PostgresqlVisitor renders a specification AST as PostgreSQL.
//
// Functional: each Visit* returns a SqlFragment. Scoped state
// (outerPrecedence, inWildcard, wildcardAlias) is captured immutably and
// changed by constructing a sub-visitor via atPrecedence / enterWildcard.
// Monotonic counters live in a shared *counters container.
type PostgresqlVisitor struct {
	counters          *counters
	schema            *SchemaRegistry
	precedenceMapping map[string]int

	outerPrecedence int
	inWildcard      bool
	wildcardAlias   string
}

// Compile is the typed entry point for top-level callers.
func (v *PostgresqlVisitor) Compile(node s.Visitable) (SqlFragment, error) {
	return s.Accept[SqlFragment](node, v)
}

// atPrecedence returns a sub-visitor with the given outer precedence.
func (v *PostgresqlVisitor) atPrecedence(prec int) *PostgresqlVisitor {
	return &PostgresqlVisitor{
		counters:          v.counters,
		schema:            v.schema,
		precedenceMapping: v.precedenceMapping,
		outerPrecedence:   prec,
		inWildcard:        v.inWildcard,
		wildcardAlias:     v.wildcardAlias,
	}
}

// enterWildcard returns a sub-visitor scoped to a new wildcard context.
// outerPrecedence is reset because the predicate lives inside its own
// EXISTS subquery and is naturally parenthesised.
func (v *PostgresqlVisitor) enterWildcard(alias string) *PostgresqlVisitor {
	return &PostgresqlVisitor{
		counters:          v.counters,
		schema:            v.schema,
		precedenceMapping: v.precedenceMapping,
		outerPrecedence:   0,
		inWildcard:        true,
		wildcardAlias:     alias,
	}
}

// lookupPrecedence returns the inner precedence for an operable node,
// falling back to the "(any other operator) LEFT" bucket and finally to the
// current outer precedence (meaning "no parens needed").
func (v *PostgresqlVisitor) lookupPrecedence(n s.Operable) int {
	key := fmt.Sprintf("%s %s", n.Operator(), n.Associativity())
	if prec, ok := v.precedenceMapping[key]; ok {
		return prec
	}
	if prec, ok := v.precedenceMapping["(any other operator) LEFT"]; ok {
		return prec
	}
	return v.outerPrecedence
}

// wrap adds parentheses if inner precedence is lower than current outer.
func (v *PostgresqlVisitor) wrap(innerPrec int, sql string) string {
	if innerPrec < v.outerPrecedence {
		return "(" + sql + ")"
	}
	return sql
}

func (v *PostgresqlVisitor) VisitGlobalScope(_ s.GlobalScopeNode) (SqlFragment, error) {
	return SqlFragment{}, nil
}

func (v *PostgresqlVisitor) VisitObject(_ s.ObjectNode) (SqlFragment, error) {
	return SqlFragment{}, nil
}

func (v *PostgresqlVisitor) VisitItem(_ s.ItemNode) (SqlFragment, error) {
	// Item() in wildcard context refers to the current item alias; this is
	// handled in VisitField when Item() is detected as the field's parent.
	return SqlFragment{}, nil
}

func (v *PostgresqlVisitor) VisitValue(n s.ValueNode) (SqlFragment, error) {
	v.counters.placeholderIndex++
	return SqlFragment{
		SQL:    fmt.Sprintf("$%d", v.counters.placeholderIndex),
		Params: []any{n.Value()},
	}, nil
}

func (v *PostgresqlVisitor) VisitField(n s.FieldNode) (SqlFragment, error) {
	if v.inWildcard && v.isItemReference(n.Object()) {
		// Field of the current item in a wildcard: item.Price, item.Active, etc.
		return SqlFragment{SQL: v.wildcardAlias + "." + n.Name()}, nil
	}
	// Normal field access
	return SqlFragment{SQL: strings.Join(s.ExtractFieldPath(n), ".")}, nil
}

func (v *PostgresqlVisitor) VisitInfix(n s.InfixNode) (SqlFragment, error) {
	innerPrec := v.lookupPrecedence(n)
	sub := v.atPrecedence(innerPrec)

	left, err := s.Accept[SqlFragment](n.Left(), sub)
	if err != nil {
		return SqlFragment{}, err
	}
	right, err := s.Accept[SqlFragment](n.Right(), sub)
	if err != nil {
		return SqlFragment{}, err
	}

	sql := fmt.Sprintf("%s %s %s", left.SQL, n.Operator(), right.SQL)
	return SqlFragment{
		SQL:    v.wrap(innerPrec, sql),
		Params: append(left.Params, right.Params...),
	}, nil
}

func (v *PostgresqlVisitor) VisitPrefix(n s.PrefixNode) (SqlFragment, error) {
	innerPrec := v.lookupPrecedence(n)
	sub := v.atPrecedence(innerPrec)

	operand, err := s.Accept[SqlFragment](n.Operand(), sub)
	if err != nil {
		return SqlFragment{}, err
	}

	var sql string
	op := n.Operator()
	if op == operators.OperatorPos || op == operators.OperatorNeg {
		// Unary +/- don't need a space.
		sql = fmt.Sprintf("%s%s", op, operand.SQL)
	} else {
		sql = fmt.Sprintf("%s %s", op, operand.SQL)
	}
	return SqlFragment{SQL: v.wrap(innerPrec, sql), Params: operand.Params}, nil
}

func (v *PostgresqlVisitor) VisitPostfix(n s.PostfixNode) (SqlFragment, error) {
	innerPrec := v.lookupPrecedence(n)
	sub := v.atPrecedence(innerPrec)

	operand, err := s.Accept[SqlFragment](n.Operand(), sub)
	if err != nil {
		return SqlFragment{}, err
	}
	sql := fmt.Sprintf("%s %s", operand.SQL, n.Operator())
	return SqlFragment{SQL: v.wrap(innerPrec, sql), Params: operand.Params}, nil
}

func (v *PostgresqlVisitor) VisitCollection(n s.CollectionNode) (SqlFragment, error) {
	// Two modes:
	// 1. Embedded (JSONB/array): EXISTS (SELECT 1 FROM unnest(collection) AS item WHERE predicate)
	// 2. Relational (separate table): EXISTS (SELECT 1 FROM table AS item WHERE fk_conditions AND predicate)
	collectionName := v.extractCollectionName(n)
	fieldName := v.extractFieldName(n)

	if v.schema != nil && v.schema.IsRelational(fieldName) {
		return v.visitRelationalCollection(n, fieldName, collectionName)
	}
	return v.visitEmbeddedCollection(n, collectionName)
}

// visitEmbeddedCollection generates SQL for JSONB/array collections using unnest.
func (v *PostgresqlVisitor) visitEmbeddedCollection(n s.CollectionNode, collectionName string) (SqlFragment, error) {
	collectionPath := v.extractCollectionPath(n)

	v.counters.wildcardCounter++
	alias := fmt.Sprintf("%s_%d", strings.ToLower(collectionName), v.counters.wildcardCounter)

	sub := v.enterWildcard(alias)
	predicate, err := s.Accept[SqlFragment](n.Predicate(), sub)
	if err != nil {
		return SqlFragment{}, err
	}

	return SqlFragment{
		SQL: fmt.Sprintf(
			"EXISTS (SELECT 1 FROM unnest(%s) AS %s WHERE %s)",
			collectionPath, alias, predicate.SQL,
		),
		Params: predicate.Params,
	}, nil
}

// visitRelationalCollection generates SQL for collections in separate tables.
func (v *PostgresqlVisitor) visitRelationalCollection(n s.CollectionNode, fieldName, collectionName string) (SqlFragment, error) {
	mapping, _ := v.schema.Get(fieldName)

	v.counters.wildcardCounter++
	alias := mapping.Alias
	if alias == "" {
		alias = fmt.Sprintf("%s_%d", strings.ToLower(collectionName), v.counters.wildcardCounter)
	} else {
		alias = fmt.Sprintf("%s_%d", alias, v.counters.wildcardCounter)
	}

	// Determine parent reference BEFORE entering new wildcard context.
	// This ensures we reference the outer scope, not the new alias.
	parentRef := v.getParentRefForRelational()

	sub := v.enterWildcard(alias)
	predicate, err := s.Accept[SqlFragment](n.Predicate(), sub)
	if err != nil {
		return SqlFragment{}, err
	}

	// Generate FK conditions (supports composite keys).
	fkParts := make([]string, 0, len(mapping.ForeignKeys))
	for _, fk := range mapping.ForeignKeys {
		fkParts = append(fkParts, fmt.Sprintf(
			"%s.%s = %s.%s", alias, fk.ChildColumn, parentRef, fk.ParentColumn,
		))
	}
	fkConditions := strings.Join(fkParts, " AND ")

	return SqlFragment{
		SQL: fmt.Sprintf(
			"EXISTS (SELECT 1 FROM %s AS %s WHERE %s AND %s)",
			mapping.Table, alias, fkConditions, predicate.SQL,
		),
		Params: predicate.Params,
	}, nil
}

// getParentRefForRelational returns parent reference based on current
// wildcard context. Called BEFORE entering a new wildcard context to get
// the correct outer reference.
func (v *PostgresqlVisitor) getParentRefForRelational() string {
	// If we are inside a nested wildcard, use the outer wildcard alias.
	if v.inWildcard && v.wildcardAlias != "" {
		return v.wildcardAlias
	}
	// Otherwise, use schema's parent reference.
	if v.schema != nil {
		return v.schema.GetParentRef()
	}
	return ""
}

// extractFieldName extracts the field name from collection's parent Object.
func (v *PostgresqlVisitor) extractFieldName(n s.CollectionNode) string {
	parent := n.Parent()
	if !parent.IsRoot() {
		return parent.Name()
	}
	return ""
}

// extractCollectionPath extracts the SQL path to a collection from a CollectionNode.
func (v *PostgresqlVisitor) extractCollectionPath(n s.CollectionNode) string {
	var parts []string

	// Walk up the parent chain to collect path components.
	parent := n.Parent()
	for !parent.IsRoot() {
		parts = append([]string{parent.Name()}, parts...) // prepend
		parent = parent.Parent()
	}

	// If we're in a wildcard context and parent is Item(), prefix with current alias.
	// This handles nested wildcards: category.Items instead of just Items.
	if v.inWildcard && v.isItemReference(parent) {
		if len(parts) > 0 {
			return v.wildcardAlias + "." + strings.Join(parts, ".")
		}
		return v.wildcardAlias
	}

	return strings.Join(parts, ".")
}

// extractCollectionName extracts the collection name for alias generation.
// e.g., "Items" -> "item", "Categories" -> "category", "Series" -> "series".
func (v *PostgresqlVisitor) extractCollectionName(n s.CollectionNode) string {
	parent := n.Parent()
	if !parent.IsRoot() {
		return inflection.Singular(parent.Name())
	}
	return "item" // fallback
}

// isItemReference checks if the object is Item() (current item in wildcard).
func (v *PostgresqlVisitor) isItemReference(obj s.EmptiableObject) bool {
	_, isItem := obj.(s.ItemNode)
	return isItem
}
