package specification

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

// Regression tests of the defects found while porting the package to Rust.

func field(name string) s.FieldNode {
	return s.Field(s.GlobalScope(), name)
}

func item(name string) s.FieldNode {
	return s.Field(s.Item(), name)
}

func sqlOf(t *testing.T, node s.Visitable, opts ...PostgresqlVisitorOption) string {
	t.Helper()
	sql, _, err := CompileToSQL(node, opts...)
	if err != nil {
		t.Fatalf("CompileToSQL: %v", err)
	}
	return sql
}

type sqlCase struct {
	node s.Visitable
	want string
}

func checkSql(t *testing.T, cases []sqlCase, opts ...PostgresqlVisitorOption) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			if got := sqlOf(t, c.node, opts...); got != c.want {
				t.Errorf("got  %s\nwant %s", got, c.want)
			}
		})
	}
}

// The operators IS NULL, IS NOT NULL and the unary minus were in no row of the
// precedence table, which spells them ISNULL, NOTNULL and "-": they had the
// precedence of "any other operator", above the comparisons.
func TestParenthesesAreWritten(t *testing.T) {
	a, b, c := field("a"), field("b"), field("c")
	checkSql(t, []sqlCase{
		{s.And(s.Or(a, b), c), `("a" OR "b") AND "c"`},
		{s.Or(s.And(a, b), c), `"a" AND "b" OR "c"`},
		{s.Not(s.And(a, b)), `NOT ("a" AND "b")`},
		{s.Not(s.Equal(a, b)), `NOT "a" = "b"`},
		{s.Mul(s.Add(a, b), c), `("a" + "b") * "c"`},
		{s.Add(s.Mul(a, b), c), `"a" * "b" + "c"`},
		{s.IsNull(s.Or(a, b)), `("a" OR "b") IS NULL`},
		{s.IsNull(s.Equal(a, b)), `"a" = "b" IS NULL`},
		{s.Neg(s.Add(a, b)), `-("a" + "b")`},
		{s.LeftShift(s.Add(a, b), c), `"a" + "b" << "c"`},
		{s.Add(a, s.LeftShift(b, c)), `"a" + ("b" << "c")`},
		{s.Equal(s.Is(a, b), c), `("a" IS NOT DISTINCT FROM "b") = "c"`},
	})
}

// Precedences alone were compared, so an operand as tight as its operator was
// never parenthesised: `a - (b - c)` was written `a - b - c`, and
// `(a = b) = c` was written `a = b = c`, which PostgreSQL does not parse.
func TestParenthesesFollowAssociativity(t *testing.T) {
	a, b, c := field("a"), field("b"), field("c")
	checkSql(t, []sqlCase{
		{s.Sub(s.Sub(a, b), c), `"a" - "b" - "c"`},
		{s.Sub(a, s.Sub(b, c)), `"a" - ("b" - "c")`},
		{s.Sub(a, s.Add(b, c)), `"a" - ("b" + "c")`},
		{s.Div(a, s.Div(b, c)), `"a" / ("b" / "c")`},
		{s.Div(s.Mul(a, b), c), `"a" * "b" / "c"`},
		// A comparison groups to neither side.
		{s.Equal(s.Equal(a, b), c), `("a" = "b") = "c"`},
		{s.Equal(a, s.Equal(b, c)), `"a" = ("b" = "c")`},
		{s.Equal(s.IsNull(a), c), `("a" IS NULL) = "c"`},
		{s.IsNull(s.IsNull(a)), `("a" IS NULL) IS NULL`},
		// AND and OR are associative, nulls included.
		{s.And(a, s.And(b, c)), `"a" AND "b" AND "c"`},
		{s.Or(a, s.Or(b, c)), `"a" OR "b" OR "c"`},
		{s.And(a, b, c), `"a" AND "b" AND "c"`},
	})
}

// The unary minus was written as its name, "-neg": `-nega`.
func TestTheUnaryMinusIsSpelled(t *testing.T) {
	a, b := field("a"), field("b")
	checkSql(t, []sqlCase{
		{s.Neg(a), `-"a"`},
		// Two minus signs are a comment.
		{s.Neg(s.Neg(a)), `-(-"a")`},
		{s.Sub(a, s.Neg(b)), `"a" - -"b"`},
		{s.Not(s.Not(a)), `NOT NOT "a"`},
	})
	sql, params, err := CompileToSQL(s.Neg(s.Value(5)))
	if err != nil || sql != "-$1" || !reflect.DeepEqual(params, []any{5}) {
		t.Errorf("got %q, %v, %v", sql, params, err)
	}
}

// IS takes a keyword and not a parameter: `x IS $1` is a syntax error.
func TestIsTakesAParameter(t *testing.T) {
	sql, params, err := CompileToSQL(s.Is(field("active"), s.Value(true)))
	if err != nil {
		t.Fatal(err)
	}
	if sql != `"active" IS NOT DISTINCT FROM $1` || !reflect.DeepEqual(params, []any{true}) {
		t.Errorf("got %q, %v", sql, params)
	}
	checkSql(t, []sqlCase{{s.IsNull(field("deleted_at")), `"deleted_at" IS NULL`}})
}

// The predicate was written after the keys as it was: `fk AND p OR q`, which
// selects through `q` the rows of other parents.
func TestThePredicateOfARelationalCollectionStaysInsideItsKeys(t *testing.T) {
	schema := NewSchemaRegistry("stores").WithParentAlias("s").
		RegisterRelational("Items", "items", "store_id", "id")
	dear := s.GreaterThan(item("Price"), s.Value(500))
	items := s.Object(s.GlobalScope(), "Items")

	checkSql(t, []sqlCase{
		{
			s.Wildcard(items, s.Or(item("Active"), dear)),
			`EXISTS (SELECT 1 FROM "items" AS "item_1" WHERE "item_1"."store_id" = "s"."id"` +
				` AND ("item_1"."Active" OR "item_1"."Price" > $1))`,
		},
		{
			s.Wildcard(items, s.And(item("Active"), dear)),
			`EXISTS (SELECT 1 FROM "items" AS "item_1" WHERE "item_1"."store_id" = "s"."id"` +
				` AND "item_1"."Active" AND "item_1"."Price" > $1)`,
		},
	}, WithSchema(schema))
	// An embedded collection needs none.
	checkSql(t, []sqlCase{{
		s.Wildcard(items, s.Or(item("Active"), dear)),
		`EXISTS (SELECT 1 FROM unnest("Items") AS "item_1" WHERE "item_1"."Active" OR "item_1"."Price" > $1)`,
	}})
}

// A schema named a collection by its last name alone, so the items of a store
// and the items of a category were one collection with one table.
func TestACollectionIsNamedByItsWholePath(t *testing.T) {
	ofTheStore := s.Wildcard(s.Object(s.GlobalScope(), "Items"), item("Active"))
	ofACategory := s.Wildcard(
		s.Object(s.GlobalScope(), "Categories"),
		s.Wildcard(s.Object(s.Item(), "Items"), item("Active")),
	)

	schema := NewSchemaRegistry("stores").WithParentAlias("s").
		RegisterRelational("Items", "store_items", "store_id", "id").
		RegisterRelational("Categories", "categories", "store_id", "id").
		RegisterRelational("Categories.Items", "category_items", "category_id", "id")
	checkSql(t, []sqlCase{
		{
			ofTheStore,
			`EXISTS (SELECT 1 FROM "store_items" AS "item_1"` +
				` WHERE "item_1"."store_id" = "s"."id" AND "item_1"."Active")`,
		},
		{
			ofACategory,
			`EXISTS (SELECT 1 FROM "categories" AS "category_1"` +
				` WHERE "category_1"."store_id" = "s"."id" AND EXISTS (SELECT 1 FROM "category_items" AS "item_2"` +
				` WHERE "item_2"."category_id" = "category_1"."id" AND "item_2"."Active"))`,
		},
	}, WithSchema(schema))

	// A nested collection that is not named is embedded.
	unnamed := NewSchemaRegistry("stores").WithParentAlias("s").
		RegisterRelational("Items", "store_items", "store_id", "id").
		RegisterRelational("Categories", "categories", "store_id", "id")
	checkSql(t, []sqlCase{{
		ofACategory,
		`EXISTS (SELECT 1 FROM "categories" AS "category_1"` +
			` WHERE "category_1"."store_id" = "s"."id" AND EXISTS (SELECT 1 FROM unnest("category_1"."Items") AS "item_2"` +
			` WHERE "item_2"."Active"))`,
	}}, WithSchema(unnamed))

	// The objects on the way are a part of the name.
	warehouse := NewSchemaRegistry("stores").WithParentAlias("s").
		RegisterRelational("Warehouse.Items", "warehouse_items", "store_id", "id")
	checkSql(t, []sqlCase{{
		s.Wildcard(s.Object(s.Object(s.GlobalScope(), "Warehouse"), "Items"), item("Active")),
		`EXISTS (SELECT 1 FROM "warehouse_items" AS "item_1"` +
			` WHERE "item_1"."store_id" = "s"."id" AND "item_1"."Active")`,
	}}, WithSchema(warehouse))
}

// A relational collection inside the predicate of another was joined to the
// enclosing item whatever it was a collection of.
func TestACollectionOfTheCandidateInsideAnotherJoinsToTheRoot(t *testing.T) {
	schema := NewSchemaRegistry("stores").WithParentAlias("s").
		RegisterRelational("Items", "items", "store_id", "id").
		RegisterRelational("Tags", "tags", "store_id", "id").
		RegisterRelational("Items.Tags", "item_tags", "item_id", "id")
	onSale := s.Equal(item("Name"), s.Value("sale"))
	items := s.Object(s.GlobalScope(), "Items")

	checkSql(t, []sqlCase{
		{
			s.Wildcard(items, s.Wildcard(s.Object(s.GlobalScope(), "Tags"), onSale)),
			`EXISTS (SELECT 1 FROM "items" AS "item_1" WHERE "item_1"."store_id" = "s"."id"` +
				` AND EXISTS (SELECT 1 FROM "tags" AS "tag_2" WHERE "tag_2"."store_id" = "s"."id"` +
				` AND "tag_2"."Name" = $1))`,
		},
		{
			s.Wildcard(items, s.Wildcard(s.Object(s.Item(), "Tags"), onSale)),
			`EXISTS (SELECT 1 FROM "items" AS "item_1" WHERE "item_1"."store_id" = "s"."id"` +
				` AND EXISTS (SELECT 1 FROM "item_tags" AS "tag_2" WHERE "tag_2"."item_id" = "item_1"."id"` +
				` AND "tag_2"."Name" = $1))`,
		},
	}, WithSchema(schema))
}

// A name was written into the query as it stood, and PostgreSQL reads a word
// it knows as what it knows: `user = $1` compares the user of the session and
// selects other rows than were asked for, `order > $1` does not parse. Which
// words these are depends on the version of the server, so every name is
// quoted, and between quotes it is the column's to the letter. The rows are in
// TestASpecificationSelectsTheRowsItIsSatisfiedBy.
func TestANameIsTheColumnsAndNothingElse(t *testing.T) {
	checkSql(t, []sqlCase{
		{s.Equal(field("user"), s.Value("ann")), `"user" = $1`},
		{s.GreaterThan(field("order"), s.Value(0)), `"order" > $1`},
		{s.IsNull(field("createdAt")), `"createdAt" IS NULL`},
	})

	// A quote inside a name does not end it.
	if got := quote(`a" OR "b`); got != `"a"" OR ""b"` {
		t.Errorf("got %s", got)
	}
	if got := quote(`"`); got != `""""` {
		t.Errorf("got %s", got)
	}
	// Nor does it get there: the alphabet of names has no quote.
	if _, _, err := CompileToSQL(field(`a" OR "b`)); err == nil {
		t.Error("a name with a quote was compiled")
	}
}

// Names were written into the query as they were: only values are parameters.
func TestANameThatIsNotAnIdentifierIsRefused(t *testing.T) {
	injection := "x; DROP TABLE users; --"
	trees := map[string]s.Visitable{
		"a field":                  field(injection),
		"an object on the way":     s.Field(s.Object(s.GlobalScope(), injection), "name"),
		"a field of an item":       s.Wildcard(s.Object(s.GlobalScope(), "Items"), item(injection)),
		"a collection":             s.Wildcard(s.Object(s.GlobalScope(), injection), item("Active")),
		"a name starting a digit":  field("1st"),
		"an empty name":            field(""),
		"a name with a dot inside": field("a.b"),
	}
	for name, tree := range trees {
		t.Run(name, func(t *testing.T) {
			if name == "a name with a dot inside" {
				// A dotted name is a path of identifiers.
				if got := sqlOf(t, tree); got != `"a"."b"` {
					t.Errorf("got %q", got)
				}
				return
			}
			sql, _, err := CompileToSQL(tree)
			if err == nil {
				t.Fatalf("compiled to %q", sql)
			}
			if !strings.Contains(err.Error(), "not a valid identifier") {
				t.Errorf("got error %q", err)
			}
		})
	}

	active := s.Wildcard(s.Object(s.GlobalScope(), "Items"), item("Active"))
	schemas := map[string]*SchemaRegistry{
		"a table":         NewSchemaRegistry("stores").WithParentAlias("s").RegisterRelational("Items", injection, "store_id", "id"),
		"a child column":  NewSchemaRegistry("stores").WithParentAlias("s").RegisterRelational("Items", "items", injection, "id"),
		"a parent column": NewSchemaRegistry("stores").WithParentAlias("s").RegisterRelational("Items", "items", "store_id", injection),
		"a parent alias":  NewSchemaRegistry("stores").WithParentAlias(injection).RegisterRelational("Items", "items", "store_id", "id"),
	}
	for name, schema := range schemas {
		t.Run(name, func(t *testing.T) {
			if sql, _, err := CompileToSQL(active, WithSchema(schema)); err == nil {
				t.Fatalf("compiled to %q", sql)
			}
		})
	}

	checkSql(t, []sqlCase{
		{s.Field(s.Object(s.GlobalScope(), "users"), "_name1"), `"users"."_name1"`},
	})
	qualified := NewSchemaRegistry("stores").WithParentAlias("s").
		RegisterRelational("Items", "public.items", "store_id", "id")
	checkSql(t, []sqlCase{{
		active,
		`EXISTS (SELECT 1 FROM "public"."items" AS "item_1" WHERE "item_1"."store_id" = "s"."id" AND "item_1"."Active")`,
	}}, WithSchema(qualified))
}

// The count of parameters was a uint8: the 256th was written $0.
func TestParametersAreCountedBeyondAByte(t *testing.T) {
	node := s.Visitable(s.Equal(field("a"), s.Value(0)))
	for i := 1; i < 300; i++ {
		node = s.Or(node, s.Equal(field("a"), s.Value(i)))
	}
	sql, params, err := CompileToSQL(node)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 300 || !strings.HasSuffix(sql, `"a" = $300`) || strings.Contains(sql, "$0") {
		t.Errorf("%d parameters, the query ends with %q", len(params), sql[len(sql)-20:])
	}
}

// --- Transform ---

func compositeOf(nodes ...s.Visitable) CompositeExpressionNode {
	parts := make([]Mapped, 0, len(nodes))
	for _, node := range nodes {
		parts = append(parts, Scalar(node))
	}
	return CompositeExpression(parts...)
}

// != of two composites was NOT (a1 != b1 AND a2 != b2): a composite was
// unequal to itself, and equal to one it shares no part with. It is the
// negation of their equality.
func TestCompositeInequality(t *testing.T) {
	columns := compositeOf(field("tenant_id"), field("member_id"))
	unequalTo := func(tenantId, memberId int) s.Visitable {
		node, err := columns.NotEqual(compositeOf(s.Value(tenantId), s.Value(memberId)))
		if err != nil {
			t.Fatal(err)
		}
		return node
	}

	sql, params, err := CompileToSQL(unequalTo(10, 3))
	if err != nil {
		t.Fatal(err)
	}
	if sql != `NOT ("tenant_id" = $1 AND "member_id" = $2)` || !reflect.DeepEqual(params, []any{10, 3}) {
		t.Errorf("got %q, %v", sql, params)
	}

	left := CompositeExpression(compositeOf(field("a"), field("b")), Scalar(field("c")))
	right := CompositeExpression(compositeOf(s.Value(1), s.Value(2)), Scalar(s.Value(3)))
	nested, err := left.NotEqual(right)
	if err != nil {
		t.Fatal(err)
	}
	checkSql(t, []sqlCase{{nested, `NOT ("a" = $1 AND "b" = $2 AND "c" = $3)`}})

	row := rowContext{"tenant_id": 10, "member_id": 3}
	meaning := []struct {
		tenantId, memberId int
		want               bool
		why                string
	}{
		{10, 3, false, "itself"},
		{10, 4, true, "one part differs"},
		{11, 4, true, "every part differs"},
	}
	for _, c := range meaning {
		t.Run(c.why, func(t *testing.T) {
			got, err := s.NewEvaluateVisitor(row, operators.NewDefaultRegistry()).Evaluate(unequalTo(c.tenantId, c.memberId))
			if err != nil || got != c.want {
				t.Errorf("got %v, %v, want %v", got, err, c.want)
			}
		})
	}
}

func TestACompositeOfOnePartAndOfNone(t *testing.T) {
	one, err := compositeOf(field("id")).Equal(compositeOf(s.Value(1)))
	if err != nil {
		t.Fatal(err)
	}
	other, err := compositeOf(field("id")).NotEqual(compositeOf(s.Value(1)))
	if err != nil {
		t.Fatal(err)
	}
	checkSql(t, []sqlCase{{one, `"id" = $1`}, {other, `NOT "id" = $1`}})

	if _, err := CompositeExpression().Equal(CompositeExpression()); !errors.Is(err, ErrCompositeExpressionIsEmpty) {
		t.Errorf("an empty composite: got %v", err)
	}
	if _, err := CompositeExpression().NotEqual(CompositeExpression()); !errors.Is(err, ErrCompositeExpressionIsEmpty) {
		t.Errorf("an empty composite: got %v", err)
	}
}

type rowContext map[string]any

func (c rowContext) Get(key string) (any, error) {
	value, ok := c[key]
	if !ok {
		return nil, s.ErrKeyNotFound
	}
	return value, nil
}

// A composite stands for several expressions and is given a meaning by = and
// != alone. It was a Visitable that no visitor knew: one left in the tree -
// on the right of a node, under another operator - failed later, inside
// whatever read the tree next.
func TestACompositeIsNotANode(t *testing.T) {
	var composite any = compositeOf(field("a"), field("b"))
	if _, ok := composite.(s.Visitable); ok {
		t.Errorf("a composite is a Visitable")
	}

	context := membersContext{}
	memberId := memberId{tenant: 10, member: 3}

	transformed, err := NewTransformVisitor(context).Transform(s.Equal(field("id"), s.Value(memberId)))
	if err != nil {
		t.Fatal(err)
	}
	sql, params, err := CompileToSQL(transformed)
	if err != nil || sql != `"tenant_id" = $1 AND "member_id" = $2` || !reflect.DeepEqual(params, []any{10, 3}) {
		t.Errorf("got %q, %v, %v", sql, params, err)
	}

	refused := map[string]s.Visitable{
		"the whole specification":      field("id"),
		"under another operator":       s.GreaterThan(field("id"), s.Value(memberId)),
		"beside a node, on the right":  s.Equal(field("name"), s.Value(memberId)),
		"beside a node, on the left":   s.Equal(field("id"), s.Value("ann")),
		"under a prefix operator":      s.Not(field("id")),
		"under a postfix operator":     s.IsNull(field("id")),
		"as a predicate":               s.Wildcard(s.Object(s.GlobalScope(), "members"), item("id")),
		"of different shapes":          s.Equal(field("id"), s.Value(nestedId{memberId, 7})),
		"of different shapes, swapped": s.Equal(s.Value(nestedId{memberId, 7}), field("id")),
	}
	for name, specification := range refused {
		t.Run(name, func(t *testing.T) {
			got, err := NewTransformVisitor(context).Transform(specification)
			if err == nil {
				t.Errorf("transformed to %#v", got)
			}
		})
	}

	// Inside the predicate of a collection.
	inside, err := NewTransformVisitor(context).Transform(
		s.Wildcard(s.Object(s.GlobalScope(), "members"), s.NotEqual(item("id"), s.Value(memberId))),
	)
	if err != nil {
		t.Fatal(err)
	}
	checkSql(t, []sqlCase{{
		inside,
		`EXISTS (SELECT 1 FROM unnest("members") AS "member_1"` +
			` WHERE NOT ("member_1"."tenant_id" = $1 AND "member_1"."member_id" = $2))`,
	}})
}

type memberId struct{ tenant, member int }

type nestedId struct {
	member memberId
	number int
}

type membersContext struct{ ContextDefaults }

func (membersContext) AttrNode(path []string) (Mapped, error) {
	if len(path) == 1 && path[0] == "id" {
		return compositeOf(field("tenant_id"), field("member_id")), nil
	}
	return Scalar(field(strings.Join(path, "."))), nil
}

func (membersContext) ItemAttrNode(path []string) (Mapped, error) {
	if len(path) == 1 && path[0] == "id" {
		return compositeOf(item("tenant_id"), item("member_id")), nil
	}
	return Scalar(item(path[len(path)-1])), nil
}

func (c membersContext) ValueNode(val any) (Mapped, error) {
	switch v := val.(type) {
	case memberId:
		return compositeOf(s.Value(v.tenant), s.Value(v.member)), nil
	case nestedId:
		member, _ := c.ValueNode(v.member)
		return CompositeExpression(member, Scalar(s.Value(v.number))), nil
	default:
		return Scalar(s.Value(val)), nil
	}
}

type weight struct{ grams int }

// partsContext maps a domain whose parts have a weight, kept in grams.
type partsContext struct{ ContextDefaults }

func (partsContext) AttrNode(path []string) (Mapped, error) {
	if len(path) == 1 && path[0] == "rank" {
		return Scalar(field("rank")), nil
	}
	return nil, fmt.Errorf("unknown field: %s", strings.Join(path, "."))
}

func (partsContext) ItemAttrNode(path []string) (Mapped, error) {
	if len(path) == 1 && path[0] == "weight" {
		return Scalar(item("weight_grams")), nil
	}
	return nil, fmt.Errorf("unknown field of an item: %s", strings.Join(path, "."))
}

func (partsContext) ValueNode(val any) (Mapped, error) {
	if w, ok := val.(weight); ok {
		return Scalar(s.Value(w.grams)), nil
	}
	return Scalar(s.Value(val)), nil
}

// The transformer returned a collection as it was, so the values of its
// predicate reached the query as the domain's objects, and its fields under
// the domain's names. And a field was mapped by its names alone: a mapping
// could not tell a member of the item from a member of the candidate.
func TestThePredicateOfACollectionIsTransformed(t *testing.T) {
	parts := s.Object(s.GlobalScope(), "parts")
	heavy := s.And(
		s.GreaterThan(field("rank"), s.Value(3)),
		s.Wildcard(parts, s.GreaterThan(item("weight"), s.Value(weight{100}))),
	)
	cases := []struct {
		name string
		from s.Visitable
		want s.Visitable
	}{
		{
			"the tree", heavy,
			s.And(
				s.GreaterThan(field("rank"), s.Value(3)),
				s.Wildcard(parts, s.GreaterThan(item("weight_grams"), s.Value(100))),
			),
		},
		{
			"a member of the candidate inside the predicate",
			s.Wildcard(parts, s.GreaterThan(item("weight"), field("rank"))),
			s.Wildcard(parts, s.GreaterThan(item("weight_grams"), field("rank"))),
		},
		{
			"nested",
			s.Wildcard(parts, s.Wildcard(s.Object(s.Item(), "parts"), s.GreaterThan(item("weight"), s.Value(weight{5})))),
			s.Wildcard(parts, s.Wildcard(s.Object(s.Item(), "parts"), s.GreaterThan(item("weight_grams"), s.Value(5)))),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := NewTransformVisitor(partsContext{}).Transform(c.from)
			if err != nil {
				t.Fatal(err)
			}
			if !s.SameTree(got, c.want) {
				t.Errorf("got  %#v\nwant %#v", got, c.want)
			}
		})
	}

	sql, params, err := Compile(partsContext{}, heavy)
	if err != nil {
		t.Fatal(err)
	}
	want := `"rank" > $1 AND EXISTS (SELECT 1 FROM unnest("parts") AS "part_1" WHERE "part_1"."weight_grams" > $2)`
	if sql != want || !reflect.DeepEqual(params, []any{3, 100}) {
		t.Errorf("got %q, %v", sql, params)
	}
}

// storedPartsContext keeps the collections under names of the storage.
type storedPartsContext struct {
	partsContext
	asked *[][]string
}

func (c storedPartsContext) CollectionNode(path []string) (s.EmptiableObject, error) {
	if c.asked != nil {
		*c.asked = append(*c.asked, path)
	}
	if len(path) == 1 && path[0] == "parts" {
		return s.Object(s.GlobalScope(), "something_parts"), nil
	}
	if len(path) == 1 && path[0] == "secret" {
		return nil, errors.New("no such collection")
	}
	return c.partsContext.CollectionNode(path)
}

func (c storedPartsContext) ItemCollectionNode(path []string) (s.EmptiableObject, error) {
	if len(path) == 1 && path[0] == "parts" {
		return s.Object(s.Item(), "sub_parts"), nil
	}
	return c.partsContext.ItemCollectionNode(path)
}

// A collection stayed under the domain's name whatever the storage calls it:
// `unnest(parts)` of a column that is `something_parts`.
func TestACollectionIsKeptWhereTheContextSays(t *testing.T) {
	heavyPart := s.GreaterThan(item("weight"), s.Value(weight{100}))
	ofTheCandidate := s.Wildcard(s.Object(s.GlobalScope(), "parts"), heavyPart)
	ofAnItem := s.Wildcard(s.Object(s.GlobalScope(), "parts"), s.Wildcard(s.Object(s.Item(), "parts"), heavyPart))

	transformed := func(context Context, from s.Visitable) s.Visitable {
		t.Helper()
		got, err := NewTransformVisitor(context).Transform(from)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	checkSql(t, []sqlCase{
		{
			transformed(storedPartsContext{}, ofTheCandidate),
			`EXISTS (SELECT 1 FROM unnest("something_parts") AS "something_part_1" WHERE "something_part_1"."weight_grams" > $1)`,
		},
		{
			transformed(storedPartsContext{}, ofAnItem),
			`EXISTS (SELECT 1 FROM unnest("something_parts") AS "something_part_1"` +
				` WHERE EXISTS (SELECT 1 FROM unnest("something_part_1"."sub_parts") AS "sub_part_2" WHERE "sub_part_2"."weight_grams" > $1))`,
		},
		// A context that does not say keeps it where it is.
		{
			transformed(partsContext{}, ofAnItem),
			`EXISTS (SELECT 1 FROM unnest("parts") AS "part_1"` +
				` WHERE EXISTS (SELECT 1 FROM unnest("part_1"."parts") AS "part_2" WHERE "part_2"."weight_grams" > $1))`,
		},
	})

	// The whole path is asked about.
	var asked [][]string
	transformed(
		storedPartsContext{asked: &asked},
		s.Wildcard(s.Object(s.Object(s.GlobalScope(), "warehouse"), "shelves"), heavyPart),
	)
	if !reflect.DeepEqual(asked, [][]string{{"warehouse", "shelves"}}) {
		t.Errorf("asked about %v", asked)
	}

	// A refusal of the context is not hidden.
	if got, err := NewTransformVisitor(storedPartsContext{}).Transform(s.Wildcard(s.Object(s.GlobalScope(), "secret"), heavyPart)); err == nil {
		t.Errorf("transformed to %#v", got)
	}
}

// leastContext says what a context must, and nothing of what it may.
type leastContext struct{ ContextDefaults }

func (leastContext) AttrNode(path []string) (Mapped, error) {
	return Scalar(field(strings.Join(path, "_"))), nil
}

func (leastContext) ValueNode(val any) (Mapped, error) {
	return Scalar(s.Value(val)), nil
}

// A mapping and a schema could not be given together: Compile took a context
// and no options, CompileToSQL options and no context.
func TestAMappingAndASchemaAreGivenTogether(t *testing.T) {
	schema := NewSchemaRegistry("stores").WithParentAlias("s").
		RegisterRelational("store_items", "items", "store_id", "id")
	dear := s.Wildcard(s.Object(s.GlobalScope(), "Items"), s.GreaterThan(item("Price"), s.Value(500)))
	sql, params, err := Compile(storeItemsContext{}, dear, WithSchema(schema))
	if err != nil {
		t.Fatal(err)
	}
	want := `EXISTS (SELECT 1 FROM "items" AS "store_item_1" WHERE "store_item_1"."store_id" = "s"."id" AND "store_item_1"."price_cents" > $1)`
	if sql != want || !reflect.DeepEqual(params, []any{500}) {
		t.Errorf("got  %s, %v\nwant %s", sql, params, want)
	}
}

// storeItemsContext names the members of a store as the storage does.
type storeItemsContext struct{ ContextDefaults }

func (storeItemsContext) AttrNode(path []string) (Mapped, error) {
	return nil, fmt.Errorf("no such member of a store: %s", strings.Join(path, "."))
}

func (storeItemsContext) ItemAttrNode(path []string) (Mapped, error) {
	if len(path) == 1 && path[0] == "Price" {
		return Scalar(item("price_cents")), nil
	}
	return nil, fmt.Errorf("no such member of an item: %s", strings.Join(path, "."))
}

func (storeItemsContext) CollectionNode(path []string) (s.EmptiableObject, error) {
	if len(path) == 1 && path[0] == "Items" {
		return s.Object(s.GlobalScope(), "store_items"), nil
	}
	return nil, fmt.Errorf("no such collection of a store: %s", strings.Join(path, "."))
}

func (storeItemsContext) ValueNode(val any) (Mapped, error) {
	return Scalar(s.Value(val)), nil
}

func TestWhatAContextMustSayAndWhatItMay(t *testing.T) {
	var _ Context = leastContext{}

	sql, _, err := Compile(leastContext{}, s.Equal(s.Field(s.Object(s.GlobalScope(), "profile"), "age"), s.Value(30)))
	if err != nil || sql != `"profile_age" = $1` {
		t.Errorf("got %q, %v", sql, err)
	}

	// The fields of an item have no answer but the context's: left as it is,
	// the field would reach the query under the domain's name.
	_, _, err = Compile(leastContext{}, s.Wildcard(s.Object(s.GlobalScope(), "items"), item("price")))
	if err == nil || !strings.Contains(err.Error(), "price") {
		t.Errorf("got %v, want an error naming the field", err)
	}
}
