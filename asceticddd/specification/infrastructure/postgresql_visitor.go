package specification

import (
	"errors"
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
func Compile(mapping Mapping, exp s.Visitable, opts ...PostgresqlVisitorOption) (sql string, params []any, err error) {
	transformed, err := NewMappingVisitor(mapping).Transform(exp)
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
// in the storage is for the Mapping to say.
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
	outerApart bool
	// wildcards is the collection whose item is under test, last, and before
	// it the enclosing ones: OuterItem(depth) is the item of the one depth
	// steps out.
	wildcards []wildcard
}

// wildcard is a collection whose predicate is being compiled: what its item's
// row is called in the query, and what that row is a row of to the schema - a
// table, or the composite at a column of one, `stores.items`.
type wildcard struct {
	alias string
	row   string
}

// inWildcard tells whether this is the predicate of a collection.
func (v *PostgresqlVisitor) inWildcard() bool {
	return len(v.wildcards) > 0
}

// wildcardOf returns the collection whose item is item: the one its depth
// steps out.
func (v *PostgresqlVisitor) wildcardOf(item s.ItemNode) (wildcard, error) {
	if item.Depth() >= len(v.wildcards) {
		return wildcard{}, fmt.Errorf("no current item in context: the item %d collections out", item.Depth())
	}
	return v.wildcards[len(v.wildcards)-1-item.Depth()], nil
}

// candidatesColumn returns the column name of the candidate's row, inside the
// predicate of a collection. Unqualified, PostgreSQL reads it from the
// innermost row that has a column of that name, and a category with a `limit`
// of its own hid the shop's. The row is what the schema calls it.
func (v *PostgresqlVisitor) candidatesColumn(name string) (string, error) {
	if v.schema == nil {
		return "", errors.New("a member of the candidate inside a collection's predicate needs the candidate's table: compile with a schema")
	}
	row, err := identifier(v.schema.Row())
	if err != nil {
		return "", err
	}
	column, err := identifier(name)
	if err != nil {
		return "", err
	}
	return row + "." + column, nil
}

// keyOfCollection returns the key a collection named name in a row of row is
// joined by: the one of that name, if it references the row; else the one key
// on the table name that does. None: the name is an array in the row. A name
// that fits two keys is refused, and so is a key named where the tree does
// not stand.
func (v *PostgresqlVisitor) keyOfCollection(row, name string) (*ForeignKey, error) {
	if v.schema == nil {
		return nil, nil
	}
	if key, ok := v.schema.KeyNamed(name); ok {
		if key.ReferencedTable != row {
			return nil, fmt.Errorf("the key %s references %s, not %s", name, key.ReferencedTable, row)
		}
		return &key, nil
	}
	keys := v.schema.KeysReferencing(name, row)
	switch len(keys) {
	case 0:
		return nil, nil
	case 1:
		return &keys[0], nil
	}
	return nil, fmt.Errorf("%s has %d keys to %s: %s; name the key", name, len(keys), row, keyNames(keys))
}

// keyOfObject returns the key an object named name in a row of row is read
// through: the one of that name, if it is on the row; else the one key on the
// row that name is a column of. None: the name is a composite in the row.
func (v *PostgresqlVisitor) keyOfObject(row, name string) (*ForeignKey, error) {
	if v.schema == nil {
		return nil, nil
	}
	if key, ok := v.schema.KeyNamed(name); ok {
		if key.Table != row {
			return nil, fmt.Errorf("the key %s is on %s, not %s", name, key.Table, row)
		}
		return &key, nil
	}
	keys := v.schema.KeysOn(row, name)
	switch len(keys) {
	case 0:
		return nil, nil
	case 1:
		return &keys[0], nil
	}
	return nil, fmt.Errorf("%s is a column of %d keys of %s: %s; name the key", name, len(keys), row, keyNames(keys))
}

func keyNames(keys []ForeignKey) string {
	names := make([]string, 0, len(keys))
	for _, key := range keys {
		names = append(names, key.Name())
	}
	return strings.Join(names, ", ")
}

// singularOf returns the singular of the last name of table, in lower case:
// what an alias is made of.
func singularOf(table string) string {
	return strings.ToLower(inflection.Singular(table[strings.LastIndex(table, ".")+1:]))
}

// keyConditions returns the equalities of a key's columns, of the row written
// child, with the referenced columns, of the row written parent.
func keyConditions(key *ForeignKey, child, parent string) (string, error) {
	parts := make([]string, 0, len(key.Columns))
	for i, column := range key.Columns {
		c, err := identifier(column)
		if err != nil {
			return "", err
		}
		r, err := identifier(key.ReferencedColumns[i])
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("%s.%s = %s.%s", child, c, parent, r))
	}
	return strings.Join(parts, " AND "), nil
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
		wildcards:         v.wildcards,
	}
}

// enterWildcard returns a sub-visitor scoped to a new wildcard context.
// prec is the precedence of the operator the predicate is an operand of: none
// in `WHERE predicate`, AND in `WHERE keys AND predicate`.
func (v *PostgresqlVisitor) enterWildcard(alias, row string, prec int) *PostgresqlVisitor {
	return &PostgresqlVisitor{
		counters:          v.counters,
		schema:            v.schema,
		precedenceMapping: v.precedenceMapping,
		outerPrecedence:   prec,
		wildcards:         append(append([]wildcard{}, v.wildcards...), wildcard{alias: alias, row: row}),
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
	if root, ok := s.ExtractFieldRoot(n).(s.ItemNode); ok {
		// Field of an item: item.Price, item.Active, etc.
		w, err := v.wildcardOf(root)
		if err != nil {
			return SqlFragment{}, err
		}
		member, err := v.memberOfRow(quote(w.alias), w.row, path)
		return SqlFragment{SQL: member}, err
	}
	// An object of the candidate kept in a table of its own, or a composite
	// column of its row - a Value Object - that the schema says is one: the
	// dots of an undeclared name are a qualifier.
	if len(path) > 1 && v.schema != nil {
		key, err := v.keyOfObject(v.schema.Table, path[0])
		if err != nil {
			return SqlFragment{}, err
		}
		if key != nil || v.schema.IsComposite(v.schema.Table, path[0]) {
			row, err := identifier(v.schema.Row())
			if err != nil {
				return SqlFragment{}, err
			}
			member, err := v.memberOfRow(row, v.schema.Table, path)
			return SqlFragment{SQL: member}, err
		}
	}
	// Inside a collection's predicate the candidate's column is qualified
	// with its row; a name of several parts the author qualified.
	if v.inWildcard() && len(path) == 1 && !strings.Contains(path[0], ".") {
		column, err := v.candidatesColumn(path[0])
		return SqlFragment{SQL: column}, err
	}
	// Normal field access: from the candidate the dots stay, a qualified name
	// - `"s"."price"` is the column `price` of `s`.
	qualified, err := identifier(strings.Join(path, "."))
	if err != nil {
		return SqlFragment{}, err
	}
	return SqlFragment{SQL: qualified}, nil
}

// memberOfRow returns the member at names of the row written row, which is
// a row of `of` to the schema: a table, or the composite at a column of one.
//
// An object on the way to the member is looked up in the schema, as a
// collection is, by the key its name is a column of. Kept in a table of its
// own, it is read through the key, by a subquery in the column's place: it has
// at most the one row the key names, and is null if there is none, as a member
// of a composite that is null is. Not mentioned, it is a composite kept in its
// row - a Value Object - and the parentheses are what makes it that: with dots
// alone PostgreSQL reads a schema, a table and a column, and there is no such
// table.
func (v *PostgresqlVisitor) memberOfRow(row, of string, names []string) (string, error) {
	name, err := identifier(names[0])
	if err != nil {
		return "", err
	}
	if len(names) == 1 {
		return row + "." + name, nil
	}

	key, err := v.keyOfObject(of, names[0])
	if err != nil {
		return "", err
	}
	if key == nil {
		return v.memberOfRow("("+row+"."+name+")", of+"."+names[0], names[1:])
	}

	// The row read is one of the referenced table, and its alias says so.
	v.counters.wildcardCounter++
	aliasRef, err := identifier(fmt.Sprintf("%s_%d", singularOf(key.ReferencedTable), v.counters.wildcardCounter))
	if err != nil {
		return "", err
	}
	table, err := identifier(key.ReferencedTable)
	if err != nil {
		return "", err
	}
	// The referenced columns are the read row's, the key's the row's here.
	keys := make([]string, 0, len(key.Columns))
	for i, column := range key.Columns {
		c, err := identifier(column)
		if err != nil {
			return "", err
		}
		r, err := identifier(key.ReferencedColumns[i])
		if err != nil {
			return "", err
		}
		keys = append(keys, fmt.Sprintf("%s.%s = %s.%s", aliasRef, r, row, c))
	}
	member, err := v.memberOfRow(aliasRef, key.ReferencedTable, names[1:])
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
	ofLeft, ofRight := typesOfBoth(n.Left(), n.Operator(), n.Right())
	if isACountToCast(n.Operator(), n.Right()) {
		// A cast binds tighter than any operator: the count is compiled
		// under the cast's precedence, so what is not an atom is
		// parenthesised, `("b" + $1)::integer`.
		rightSub = v.atPrecedence(v.precedenceMapping[":: LEFT"], false)
		ofRight = "integer"
	}

	left, err := s.Accept[SqlFragment](n.Left(), leftSub)
	if err != nil {
		return SqlFragment{}, err
	}
	right, err := s.Accept[SqlFragment](n.Right(), rightSub)
	if err != nil {
		return SqlFragment{}, err
	}
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
	test, err := v.nullTest(n)
	if err != nil {
		return SqlFragment{}, err
	}
	sql := fmt.Sprintf("%s %s", operand.SQL, test)
	return SqlFragment{SQL: v.wrap(innerPrec, sql), Params: operand.Params}, nil
}

// nullTest returns the words of a null test: of the value as a whole, for a
// column the schema declares a composite.
//
// Of a composite IS NULL is true when all its members are null and IS NOT
// NULL when none is - the standard's null predicate over a row value - so a
// row with a null member is neither. An Option of a Value Object is Some or
// Nothing whatever its members hold, and so is the column: null, or a row.
// IS DISTINCT FROM NULL tests that, as the manual advises; a Nothing is
// written as a null column, not as a row of nulls.
func (v *PostgresqlVisitor) nullTest(n s.PostfixNode) (string, error) {
	composite, err := v.isCompositeColumn(n.Operand())
	if err != nil || !composite {
		return spell(n.Operator()), err
	}
	if n.Operator() == operators.OperatorIsNull {
		return "IS NOT DISTINCT FROM NULL", nil
	}
	return "IS DISTINCT FROM NULL", nil
}

// isCompositeColumn reports whether operand is a column the schema declares
// a composite. The column is named as a key names it: by its table, or by
// the array it is a row of, `stores.items`; a composite inside a composite
// by the column, `stores.discount`.
func (v *PostgresqlVisitor) isCompositeColumn(operand s.Visitable) (bool, error) {
	field, ok := operand.(s.FieldNode)
	if !ok || v.schema == nil {
		return false, nil
	}
	path := s.ExtractFieldPath(field)
	of := v.schema.Table
	if root, ok := s.ExtractFieldRoot(field).(s.ItemNode); ok {
		w, err := v.wildcardOf(root)
		if err != nil {
			return false, err
		}
		of = w.row
	}
	names := append([]string{of}, path[:len(path)-1]...)
	return v.schema.IsComposite(strings.Join(names, "."), path[len(path)-1]), nil
}

func (v *PostgresqlVisitor) VisitCollection(n s.CollectionNode) (SqlFragment, error) {
	// Two modes:
	// 1. Embedded (an array of a composite type): EXISTS (SELECT 1 FROM unnest(collection) AS item WHERE predicate)
	// 2. Relational (a table of its own): EXISTS (SELECT 1 FROM table AS item WHERE keys AND predicate)
	//
	// The row the collection is a name of, to the schema: the enclosing
	// item's, or the query's table. Without a schema there is no key to look
	// for, and the name is an array in the row.
	of, known := v.rowOf(n)
	name := strings.Join(v.extractNames(n), ".")
	var key *ForeignKey
	if known {
		var err error
		if key, err = v.keyOfCollection(of, name); err != nil {
			return SqlFragment{}, err
		}
	}
	if key != nil {
		return v.visitRelationalCollection(n, key)
	}
	return v.visitEmbeddedCollection(n, of+"."+name)
}

// rowOf returns what the row a collection is a name of is a row of, to the
// schema; false without a schema, outside any collection.
func (v *PostgresqlVisitor) rowOf(n s.CollectionNode) (string, bool) {
	if root, ok := v.extractRoot(n).(s.ItemNode); ok {
		if w, err := v.wildcardOf(root); err == nil {
			return w.row, true
		}
	}
	if v.schema != nil {
		return v.schema.Table, true
	}
	return "", false
}

// visitEmbeddedCollection generates SQL for collections kept as an array of
// a composite type, using unnest. row is what the array's rows are rows of,
// to the schema: the array's column by its table.
func (v *PostgresqlVisitor) visitEmbeddedCollection(n s.CollectionNode, row string) (SqlFragment, error) {
	collectionPath, err := v.extractCollectionPath(n)
	if err != nil {
		return SqlFragment{}, err
	}

	v.counters.wildcardCounter++
	// The alias is the compiler's own: the singular of the array's name,
	// numbered. It goes on as it is: a name is quoted where it is written.
	alias := fmt.Sprintf("%s_%d", singularOf(v.extractCollectionName(n)), v.counters.wildcardCounter)
	aliasRef, err := identifier(alias)
	if err != nil {
		return SqlFragment{}, err
	}

	sub := v.enterWildcard(alias, row, 0)
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

// visitRelationalCollection generates SQL for collections in tables of their
// own, joined by key.
func (v *PostgresqlVisitor) visitRelationalCollection(n s.CollectionNode, key *ForeignKey) (SqlFragment, error) {
	v.counters.wildcardCounter++
	// The alias is the compiler's own: the singular of the row's table,
	// numbered.
	alias := fmt.Sprintf("%s_%d", singularOf(key.Table), v.counters.wildcardCounter)
	aliasRef, err := identifier(alias)
	if err != nil {
		return SqlFragment{}, err
	}

	// The row the collection is joined to is named BEFORE entering the new
	// wildcard context: the outer scope, not the new alias.
	parentRef, err := identifier(v.rowJoinedTo(n))
	if err != nil {
		return SqlFragment{}, err
	}
	table, err := identifier(key.Table)
	if err != nil {
		return SqlFragment{}, err
	}

	// The predicate is an operand of the AND after the keys: written as it
	// is, `fk AND p OR q` selects through `q` the rows of other parents.
	sub := v.enterWildcard(alias, key.Table, v.precedenceMapping["AND LEFT"])
	predicate, err := s.Accept[SqlFragment](n.Predicate(), sub)
	if err != nil {
		return SqlFragment{}, err
	}

	conditions, err := keyConditions(key, aliasRef, parentRef)
	if err != nil {
		return SqlFragment{}, err
	}

	return SqlFragment{
		SQL: fmt.Sprintf(
			"EXISTS (SELECT 1 FROM %s AS %s WHERE %s AND %s)",
			table, aliasRef, conditions, predicate.SQL,
		),
		Params: predicate.Params,
	}, nil
}

// rowJoinedTo returns what the query calls the row a collection is joined
// to: the enclosing item's alias if the path to the collection starts at the
// item, else the query's own table. Called BEFORE entering a new wildcard
// context to get the correct outer reference.
func (v *PostgresqlVisitor) rowJoinedTo(n s.CollectionNode) string {
	// If the collection is one of the current item (a nested wildcard), use
	// the outer wildcard alias. A collection of the candidate named inside
	// the predicate of another is joined to the root row: it used to be
	// joined to the enclosing item, whatever it was a collection of.
	if root, ok := v.extractRoot(n).(s.ItemNode); ok {
		if w, err := v.wildcardOf(root); err == nil {
			return w.alias
		}
	}
	if v.schema != nil {
		return v.schema.Row()
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

// extractNames extracts the names from what the path to the collection
// starts at.
func (v *PostgresqlVisitor) extractNames(n s.CollectionNode) []string {
	var parts []string
	parent := n.Parent()
	for !parent.IsRoot() {
		parts = append([]string{parent.Name()}, parts...) // prepend
		parent = parent.Parent()
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

	// A collection of an item is under the alias of that item's row. This
	// handles nested wildcards: category.Items instead of just Items.
	if root, ok := parent.(s.ItemNode); ok {
		w, err := v.wildcardOf(root)
		if err != nil {
			return "", err
		}
		if len(parts) > 0 {
			path, err := identifier(strings.Join(parts, "."))
			if err != nil {
				return "", err
			}
			return quote(w.alias) + "." + path, nil
		}
		return quote(w.alias), nil
	}

	if v.inWildcard() && len(parts) == 1 && !strings.Contains(parts[0], ".") {
		return v.candidatesColumn(parts[0])
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
