package specification

import (
	"fmt"
	"regexp"
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

// Compile transforms a domain specification and then renders it as SQL: what
// a repository does with a specification. The context says what the members
// and the values of the domain are in the storage, the options how the
// storage is laid out - WithSchema - and both are the repository's to know:
// a query cannot be written without knowing the table. The options used not
// to be taken, so a mapping and a schema could not be given together.
func Compile(context Context, exp s.Visitable, opts ...PostgresqlVisitorOption) (sql string, params []any, err error) {
	transformed, err := NewTransformVisitor(context).Transform(exp)
	if err != nil {
		return "", nil, err
	}
	fragment, err := NewPostgresqlVisitor(opts...).Compile(transformed)
	if err != nil {
		return "", nil, err
	}
	return fragment.SQL, fragment.Params, nil
}

// CompileToSQL compiles AST directly to SQL without context transformation:
// for a tree that is in the storage's names already. A tree built of a Go
// predicate is not: it has the names of the fields of a struct. It goes
// through Compile.
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
		v.counters.placeholderIndex = int(index)
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
	// An int: as a uint8 it wrapped, and the 256th parameter was written $0.
	placeholderIndex int
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

// tableSpelling is how the precedence table spells the operators that Operator
// names otherwise. The rest are spelled in the table as their value is.
var tableSpelling = map[operators.Operator]string{
	operators.OperatorNeg:       "-",
	operators.OperatorIsNull:    "ISNULL",
	operators.OperatorIsNotNull: "NOTNULL",
}

// sqlSpelling is how PostgreSQL spells the operators that Operator names
// otherwise. The rest are spelled in a query as their value is.
//
// IS takes a keyword - TRUE, NULL - and not a parameter: `x IS $1` is a syntax
// error. IS NOT DISTINCT FROM is the same equality, in which null is a value,
// takes any expression, and binds as IS does.
var sqlSpelling = map[operators.Operator]string{
	operators.OperatorNeg: "-",
	operators.OperatorIs:  "IS NOT DISTINCT FROM",
}

// regrouping holds the operators a run of which can be regrouped without a
// change of its value, nulls included, so that `a AND (b AND c)` needs no
// parentheses. True of the logical connectives and of nothing else:
// `a - (b - c)` is not `a - b - c`, and even `+` overflows and rounds one way
// and not the other.
var regrouping = map[operators.Operator]bool{
	operators.OperatorAnd: true,
	operators.OperatorOr:  true,
}

var identifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// quote returns the name between double quotes, a double quote of its own
// doubled.
//
// A word PostgreSQL knows is read as what PostgreSQL knows: `user` without
// quotes is the session's user, so `user = $1` parses and selects other rows
// than were asked for, and `order` does not parse. Which words these are
// depends on the server's version, which a library does not know; so no name
// is looked up in a list, and every name is quoted.
//
// Between quotes a name is the column's to the letter: `"createdAt"` is the
// column created as `"createdAt"`, which `createdAt` without quotes is not -
// PostgreSQL folds that to `createdat`. What a member of the domain is called
// in the storage is for the transform Context to say.
func quote(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// identifier returns a name as the query has it: a name, or names joined
// with dots, checked and then quoted part by part. Two things keep SQL of a
// tree's own out of the text, and neither rests on the other: a name outside
// the alphabet is refused, and a double quote inside a name is doubled. Only
// values are parameters.
func identifier(name string) (string, error) {
	parts := strings.Split(name, ".")
	for i, part := range parts {
		if !identifierPattern.MatchString(part) {
			return "", fmt.Errorf("%q is not a valid identifier", name)
		}
		parts[i] = quote(part)
	}
	return strings.Join(parts, "."), nil
}

// PostgresqlVisitor renders a specification AST as PostgreSQL.
//
// Functional: each Visit* returns a SqlFragment. Scoped state
// (outerPrecedence, outerApart, inWildcard, wildcardAlias, wildcardPath) is
// captured immutably and changed by constructing a sub-visitor via
// atPrecedence / enterWildcard.
// Monotonic counters live in a shared *counters container.
type PostgresqlVisitor struct {
	counters          *counters
	schema            *SchemaRegistry
	precedenceMapping map[string]int

	outerPrecedence int
	// outerApart tells whether an operand as tight as the outer operator is
	// parenthesised: it is on the side the operator does not group to.
	outerApart    bool
	inWildcard    bool
	wildcardAlias string
	// wildcardPath is the names from the aggregate to the collection of the
	// current item, through the collections on the way: what a schema names
	// it by.
	wildcardPath []string
}

// Compile is the typed entry point for top-level callers.
func (v *PostgresqlVisitor) Compile(node s.Visitable) (SqlFragment, error) {
	return s.Accept[SqlFragment](node, v)
}

// atPrecedence returns a sub-visitor with the given outer precedence. apart
// tells whether the operand is on the side the operator does not group to,
// where one as tight as the operator is parenthesised.
func (v *PostgresqlVisitor) atPrecedence(prec int, apart bool) *PostgresqlVisitor {
	return &PostgresqlVisitor{
		counters:          v.counters,
		schema:            v.schema,
		precedenceMapping: v.precedenceMapping,
		outerPrecedence:   prec,
		outerApart:        apart,
		inWildcard:        v.inWildcard,
		wildcardAlias:     v.wildcardAlias,
		wildcardPath:      v.wildcardPath,
	}
}

// enterWildcard returns a sub-visitor scoped to a new wildcard context.
// prec is the precedence of the operator the predicate is an operand of: none
// in `WHERE predicate`, AND in `WHERE keys AND predicate`.
func (v *PostgresqlVisitor) enterWildcard(alias string, path []string, prec int) *PostgresqlVisitor {
	return &PostgresqlVisitor{
		counters:          v.counters,
		schema:            v.schema,
		precedenceMapping: v.precedenceMapping,
		outerPrecedence:   prec,
		inWildcard:        true,
		wildcardAlias:     alias,
		wildcardPath:      path,
	}
}

// spell returns the operator as PostgreSQL spells it.
func spell(operator operators.Operator) string {
	if spelling, ok := sqlSpelling[operator]; ok {
		return spelling
	}
	return string(operator)
}

// lookupPrecedence returns the inner precedence for an operable node,
// falling back to the "(any other operator) LEFT" bucket and finally to the
// current outer precedence (meaning "no parens needed").
func (v *PostgresqlVisitor) lookupPrecedence(n s.Operable) int {
	spelling, ok := tableSpelling[n.Operator()]
	if !ok {
		spelling = string(n.Operator())
	}
	key := fmt.Sprintf("%s %s", spelling, n.Associativity())
	if prec, ok := v.precedenceMapping[key]; ok {
		return prec
	}
	if prec, ok := v.precedenceMapping["(any other operator) LEFT"]; ok {
		return prec
	}
	return v.outerPrecedence
}

// wrap adds parentheses if inner precedence is lower than current outer, or is
// the same on the side the outer operator does not group to.
func (v *PostgresqlVisitor) wrap(innerPrec int, sql string) string {
	if innerPrec < v.outerPrecedence {
		return "(" + sql + ")"
	}
	if innerPrec == v.outerPrecedence && v.outerApart {
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
	path := s.ExtractFieldPath(n)

	// What the path starts at, not what the field's immediate parent is: the
	// parent of `name` in `@.maker.name` is the object `maker`, so the item's
	// alias was dropped and `"maker"."name"` written - the column of another
	// table, if the query had one of that name.
	if v.inWildcard && v.isItemReference(s.ExtractFieldRoot(n)) {
		// Field of the current item in a wildcard: item.Price, item.Active, etc.
		member, err := v.memberOfRow(quote(v.wildcardAlias), v.wildcardPath, path)
		return SqlFragment{SQL: member}, err
	}
	// An object of the candidate kept in a table of its own
	if len(path) > 1 && v.schema != nil && v.schema.IsRelational(path[0]) {
		row, err := identifier(v.schema.GetParentRef())
		if err != nil {
			return SqlFragment{}, err
		}
		member, err := v.memberOfRow(row, nil, path)
		return SqlFragment{SQL: member}, err
	}
	// Normal field access: from the candidate the dots stay, a qualified name
	// - `"s"."price"` is the column `price` of `s`.
	qualified, err := identifier(strings.Join(path, "."))
	if err != nil {
		return SqlFragment{}, err
	}
	return SqlFragment{SQL: qualified}, nil
}

// memberOfRow returns the member at names of the row written row, whose
// object is named by logical in the schema.
//
// An object on the way to the member is looked up in the schema, as a
// collection is, by the names that lead to it. Kept in a table of its own, it
// is read through its key, by a subquery in the column's place: it has at most
// the one row the key names, and is null if there is none, as a member of a
// composite that is null is. Not mentioned, it is a composite kept in its row
// - a Value Object - and the parentheses are what makes it that: with dots
// alone PostgreSQL reads a schema, a table and a column, and there is no such
// table.
func (v *PostgresqlVisitor) memberOfRow(row string, logical, names []string) (string, error) {
	name, err := identifier(names[0])
	if err != nil {
		return "", err
	}
	if len(names) == 1 {
		return row + "." + name, nil
	}

	logical = append(append([]string{}, logical...), names[0])
	fieldName := strings.Join(logical, ".")
	if v.schema == nil || !v.schema.IsRelational(fieldName) {
		return v.memberOfRow("("+row+"."+name+")", logical, names[1:])
	}
	mapping, _ := v.schema.Get(fieldName)

	v.counters.wildcardCounter++
	alias := mapping.Alias
	if alias == "" {
		alias = strings.ToLower(names[0])
	}
	aliasRef, err := identifier(fmt.Sprintf("%s_%d", alias, v.counters.wildcardCounter))
	if err != nil {
		return "", err
	}
	table, err := identifier(mapping.Table)
	if err != nil {
		return "", err
	}
	keys := make([]string, 0, len(mapping.ForeignKeys))
	for _, fk := range mapping.ForeignKeys {
		childColumn, err := identifier(fk.ChildColumn)
		if err != nil {
			return "", err
		}
		parentColumn, err := identifier(fk.ParentColumn)
		if err != nil {
			return "", err
		}
		keys = append(keys, fmt.Sprintf("%s.%s = %s.%s", aliasRef, childColumn, row, parentColumn))
	}
	member, err := v.memberOfRow(aliasRef, logical, names[1:])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"(SELECT %s FROM %s AS %s WHERE %s)", member, table, aliasRef, strings.Join(keys, " AND "),
	), nil
}

func (v *PostgresqlVisitor) VisitInfix(n s.InfixNode) (SqlFragment, error) {
	innerPrec := v.lookupPrecedence(n)
	// An operand as tight as the operator is parenthesised on the side the
	// operator does not group to: `a - (b - c)`, `(a = b) = c`.
	regroups := regrouping[n.Operator()]
	leftSub := v.atPrecedence(innerPrec, !regroups && n.Associativity() != s.LeftAssociative)
	rightSub := v.atPrecedence(innerPrec, !regroups && n.Associativity() != s.RightAssociative)

	left, err := s.Accept[SqlFragment](n.Left(), leftSub)
	if err != nil {
		return SqlFragment{}, err
	}
	right, err := s.Accept[SqlFragment](n.Right(), rightSub)
	if err != nil {
		return SqlFragment{}, err
	}

	ofLeft, ofRight := typesOfBoth(n.Left(), n.Operator(), n.Right())
	left.SQL, right.SQL = ofType(left.SQL, ofLeft), ofType(right.SQL, ofRight)

	sql := fmt.Sprintf("%s %s %s", left.SQL, spell(n.Operator()), right.SQL)
	return SqlFragment{
		SQL:    v.wrap(innerPrec, sql),
		Params: append(left.Params, right.Params...),
	}, nil
}

func (v *PostgresqlVisitor) VisitPrefix(n s.PrefixNode) (SqlFragment, error) {
	innerPrec := v.lookupPrecedence(n)
	op := n.Operator()
	// `NOT NOT a` reads as it should; `--a` reads as a comment.
	sub := v.atPrecedence(innerPrec, op == operators.OperatorNeg)

	operand, err := s.Accept[SqlFragment](n.Operand(), sub)
	if err != nil {
		return SqlFragment{}, err
	}
	operand.SQL = ofType(operand.SQL, typeUnderPrefix(op, n.Operand()))

	var sql string
	if op == operators.OperatorNeg {
		// Unary - doesn't need a space.
		sql = fmt.Sprintf("%s%s", spell(op), operand.SQL)
	} else {
		sql = fmt.Sprintf("%s %s", spell(op), operand.SQL)
	}
	return SqlFragment{SQL: v.wrap(innerPrec, sql), Params: operand.Params}, nil
}

func (v *PostgresqlVisitor) VisitPostfix(n s.PostfixNode) (SqlFragment, error) {
	innerPrec := v.lookupPrecedence(n)
	sub := v.atPrecedence(innerPrec, true)

	operand, err := s.Accept[SqlFragment](n.Operand(), sub)
	if err != nil {
		return SqlFragment{}, err
	}
	operand.SQL = ofType(operand.SQL, typeUnderPostfix(n.Operand()))
	sql := fmt.Sprintf("%s %s", operand.SQL, spell(n.Operator()))
	return SqlFragment{SQL: v.wrap(innerPrec, sql), Params: operand.Params}, nil
}

func (v *PostgresqlVisitor) VisitCollection(n s.CollectionNode) (SqlFragment, error) {
	// Two modes:
	// 1. Embedded (JSONB/array): EXISTS (SELECT 1 FROM unnest(collection) AS item WHERE predicate)
	// 2. Relational (separate table): EXISTS (SELECT 1 FROM table AS item WHERE fk_conditions AND predicate)
	collectionName := v.extractCollectionName(n)
	fieldName := strings.Join(v.extractLogicalPath(n), ".")

	if v.schema != nil && v.schema.IsRelational(fieldName) {
		return v.visitRelationalCollection(n, fieldName, collectionName)
	}
	return v.visitEmbeddedCollection(n, collectionName)
}

// visitEmbeddedCollection generates SQL for JSONB/array collections using unnest.
func (v *PostgresqlVisitor) visitEmbeddedCollection(n s.CollectionNode, collectionName string) (SqlFragment, error) {
	collectionPath, err := v.extractCollectionPath(n)
	if err != nil {
		return SqlFragment{}, err
	}

	v.counters.wildcardCounter++
	// The alias goes on as it is: a name is quoted where it is written.
	alias := fmt.Sprintf("%s_%d", strings.ToLower(collectionName), v.counters.wildcardCounter)
	aliasRef, err := identifier(alias)
	if err != nil {
		return SqlFragment{}, err
	}

	sub := v.enterWildcard(alias, v.extractLogicalPath(n), 0)
	predicate, err := s.Accept[SqlFragment](n.Predicate(), sub)
	if err != nil {
		return SqlFragment{}, err
	}

	return SqlFragment{
		SQL: fmt.Sprintf(
			"EXISTS (SELECT 1 FROM unnest(%s) AS %s WHERE %s)",
			collectionPath, aliasRef, predicate.SQL,
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
	aliasRef, err := identifier(alias)
	if err != nil {
		return SqlFragment{}, err
	}

	// Determine parent reference BEFORE entering new wildcard context.
	// This ensures we reference the outer scope, not the new alias.
	parentRef, err := identifier(v.getParentRefForRelational(n))
	if err != nil {
		return SqlFragment{}, err
	}
	table, err := identifier(mapping.Table)
	if err != nil {
		return SqlFragment{}, err
	}

	// The predicate is an operand of the AND after the keys: written as it
	// is, `fk AND p OR q` selects through `q` the rows of other parents.
	sub := v.enterWildcard(alias, v.extractLogicalPath(n), v.precedenceMapping["AND LEFT"])
	predicate, err := s.Accept[SqlFragment](n.Predicate(), sub)
	if err != nil {
		return SqlFragment{}, err
	}

	// Generate FK conditions (supports composite keys).
	fkParts := make([]string, 0, len(mapping.ForeignKeys))
	for _, fk := range mapping.ForeignKeys {
		childColumn, err := identifier(fk.ChildColumn)
		if err != nil {
			return SqlFragment{}, err
		}
		parentColumn, err := identifier(fk.ParentColumn)
		if err != nil {
			return SqlFragment{}, err
		}
		fkParts = append(fkParts, fmt.Sprintf(
			"%s.%s = %s.%s", aliasRef, childColumn, parentRef, parentColumn,
		))
	}
	fkConditions := strings.Join(fkParts, " AND ")

	return SqlFragment{
		SQL: fmt.Sprintf(
			"EXISTS (SELECT 1 FROM %s AS %s WHERE %s AND %s)",
			table, aliasRef, fkConditions, predicate.SQL,
		),
		Params: predicate.Params,
	}, nil
}

// getParentRefForRelational returns parent reference based on what the path
// to the collection starts at. Called BEFORE entering a new wildcard context
// to get the correct outer reference.
func (v *PostgresqlVisitor) getParentRefForRelational(n s.CollectionNode) string {
	// If the collection is one of the current item (a nested wildcard), use
	// the outer wildcard alias. A collection of the candidate named inside
	// the predicate of another is joined to the root row: it used to be
	// joined to the enclosing item, whatever it was a collection of.
	if v.inWildcard && v.isItemReference(v.extractRoot(n)) {
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

// extractRoot extracts what the path to the collection starts at: GlobalScope
// or Item.
func (v *PostgresqlVisitor) extractRoot(n s.CollectionNode) s.EmptiableObject {
	parent := n.Parent()
	for !parent.IsRoot() {
		parent = parent.Parent()
	}
	return parent
}

// extractLogicalPath extracts the names from the aggregate to the collection:
// what a schema names the collection by. ["Categories", "Items"] for the items
// of a category, ["Items"] for the items of the store. The last name alone,
// extractFieldName, does not tell the two apart.
func (v *PostgresqlVisitor) extractLogicalPath(n s.CollectionNode) []string {
	var parts []string
	parent := n.Parent()
	for !parent.IsRoot() {
		parts = append([]string{parent.Name()}, parts...) // prepend
		parent = parent.Parent()
	}
	// A path from the current item goes on from the path to its collection.
	if v.inWildcard && v.isItemReference(parent) {
		return append(append([]string{}, v.wildcardPath...), parts...)
	}
	return parts
}

// extractCollectionPath extracts the SQL path to a collection from a CollectionNode.
func (v *PostgresqlVisitor) extractCollectionPath(n s.CollectionNode) (string, error) {
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
			path, err := identifier(strings.Join(parts, "."))
			if err != nil {
				return "", err
			}
			return quote(v.wildcardAlias) + "." + path, nil
		}
		return quote(v.wildcardAlias), nil
	}

	return identifier(strings.Join(parts, "."))
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
