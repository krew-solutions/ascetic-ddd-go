package specification

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/option"
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

// fromCandidate is a member by its whole path from the candidate: what a
// mapping answers with.
func fromCandidate(names ...string) s.FieldNode {
	var owner s.EmptiableObject = s.GlobalScope()
	for _, name := range names[:len(names)-1] {
		owner = s.Object(owner, name)
	}
	return s.Field(owner, names[len(names)-1])
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
	if err != nil || sql != "-$1::bigint" || !reflect.DeepEqual(params, []any{5}) {
		t.Errorf("got %q, %v, %v", sql, params, err)
	}
}

type somebody struct{ id int }

// nobody is the special case: an owner that is nobody, equal to itself in the
// domain.
type nobody struct{}

// pair is known by two numbers, of which the second may be nobody's.
type pair struct {
	a int
	b *int
}

type ownersContext struct{}

func (ownersContext) AttrNode(path []string) (Mapped, error) {
	if len(path) == 1 && path[0] == "pair" {
		return CompositeExpression(Scalar(field("a")), Scalar(field("b"))), nil
	}
	return Scalar(field(strings.Join(path, "."))), nil
}

func (ownersContext) ValueNode(val any) (Mapped, error) {
	switch v := val.(type) {
	case somebody:
		return Scalar(s.Value(v.id)), nil
	case nobody:
		return Scalar(s.Value(nil)), nil
	case pair:
		return CompositeExpression(Scalar(s.Value(v.a)), Scalar(s.Value(v.b))), nil
	default:
		return Scalar(s.Value(val)), nil
	}
}

// A value of the domain that the storage keeps as a null - a special case that
// answers for itself in the domain - is equal to itself there, and `owner = $1`
// with a null is true of nothing: the server selected no row where the
// evaluator was satisfied. The transformer tests for it, where both operands
// are mapped and the node is built; the Context maps operands and knows nothing
// of operators. Only a null the mapping made: a value that was null in the
// domain already stays compared. The rows are in
// TestEqualityWithASpecialCaseKeptAsANullIsTheNullTest.
func TestEqualityWithWhatTheMappingMadeANullIsTheNullTest(t *testing.T) {
	owner := field("owner")
	for _, c := range []struct {
		node s.Visitable
		want string
	}{
		// Somebody is compared, as any value is.
		{s.Equal(owner, s.Value(somebody{7})), `"owner" = $1`},
		{s.Equal(owner, s.Value(nobody{})), `"owner" IS NULL`},
		{s.Equal(s.Value(nobody{}), owner), `"owner" IS NULL`},
		{s.NotEqual(owner, s.Value(nobody{})), `"owner" IS NOT NULL`},
		{s.Equal(s.Value(nobody{}), s.Value(nobody{})), "$1::text IS NULL"},
		// Under any other operator it is the null it was made.
		{s.GreaterThan(owner, s.Value(nobody{})), `"owner" > $1`},
		// A null of the domain's own stays compared.
		{s.Equal(owner, s.Value(nil)), `"owner" = $1`},
		// A part of a composite is tested for as a whole is.
		{s.Equal(field("pair"), s.Value(pair{a: 1})), `"a" = $1 AND "b" IS NULL`},
	} {
		t.Run(c.want, func(t *testing.T) {
			sql, _, err := Compile(ownersContext{}, c.node)
			if err != nil || sql != c.want {
				t.Errorf("got  %s, %v", sql, err)
			}
		})
	}
}

// A constant of a specification may be an Option of a value. The transformer
// handed the wrapper to the Context, which knows the domain's values and not
// their wrappers, and it reached the driver as the text "Some({7})". It is
// read first: the Context is asked of what a Some holds, and a Nothing is the
// null it is in any storage - the domain's own null, so it stays compared, and
// is not taken for a value the mapping made a null of.
func TestTheContextIsAskedOfWhatAnOptionHolds(t *testing.T) {
	owner := field("owner")
	for _, c := range []struct {
		node   s.Visitable
		want   string
		params []any
	}{
		{s.Equal(owner, s.Value(option.Some(somebody{7}))), `"owner" = $1`, []any{7}},
		{s.Equal(owner, s.Value(option.Some(option.Some(somebody{7})))), `"owner" = $1`, []any{7}},
		// The domain's own null: compared, as a nil is.
		{s.Equal(owner, s.Value(option.Nothing[somebody]())), `"owner" = $1`, []any{nil}},
		{s.Is(owner, s.Value(option.Nothing[somebody]())), `"owner" IS NOT DISTINCT FROM $1`, []any{nil}},
		// What it holds may be a special case, which the mapping makes a null of.
		{s.Equal(owner, s.Value(option.Some(nobody{}))), `"owner" IS NULL`, nil},
	} {
		t.Run(c.want, func(t *testing.T) {
			sql, params, err := Compile(ownersContext{}, c.node)
			if err != nil || sql != c.want || !reflect.DeepEqual(params, c.params) {
				t.Errorf("got  %s, %v, %v", sql, params, err)
			}
		})
	}
}

// A constant is a parameter, and the server finds its type from what stands
// beside it. Where every operand of an operator is a constant there is nothing
// beside it - "operator is not unique: unknown + unknown" - so there the text
// says the type, by the kind of the value. Beside a column it does not: the
// value adapts to the column, which a type said would take away.
func TestAConstantWithNothingBesideItHasItsTypeSaid(t *testing.T) {
	price := field("price")
	null := func() s.Visitable { return s.Value(nil) }
	checkSql(t, []sqlCase{
		// Beside a column, or beside what has a type already: as it was.
		{s.GreaterThan(price, s.Value(1)), `"price" > $1`},
		{s.GreaterThan(s.Add(price, s.Value(1)), s.Value(2)), `"price" + $1 > $2`},
		// Both operands constants.
		{s.GreaterThan(price, s.Add(s.Value(1), s.Value(2))), `"price" > $1::bigint + $2::bigint`},
		{s.LessThan(s.Value(1), s.Value(2.5)), "$1::bigint < $2::double precision"},
		{s.Equal(s.Value("a"), s.Value("b")), "$1::text = $2::text"},
		// What was typed so is a type for what stands beside it.
		{s.Mul(s.Add(s.Value(1), s.Value(2)), s.Value(3)), "($1::bigint + $2::bigint) * $3"},
		// PostgreSQL shifts a bigint by an integer.
		{s.LeftShift(s.Value(1), s.Value(4)), "$1::bigint << $2::integer"},
		// Alone under its operator.
		{s.Neg(s.Value(5)), "-$1::bigint"},
		{s.Not(s.Value(true)), "NOT $1::boolean"},
		{s.IsNull(s.Value(7)), "$1::bigint IS NULL"},
		// A null has no kind. Beside a constant it takes that one's type from
		// the server; alone, what its operator is of.
		{s.Add(null(), s.Value(1)), "$1 + $2::bigint"},
		{s.Add(null(), null()), "$1::bigint + $2::bigint"},
		{s.Equal(null(), null()), "$1 = $2"},
		{s.IsNull(null()), "$1::text IS NULL"},
		{s.Neg(null()), "-$1::bigint"},
		{s.Not(null()), "NOT $1"},
	})
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
	schema := NewSchemaRegistry("stores").WithAlias("s").
		ForeignKey("items", "store_id", "stores", "id")
	dear := s.GreaterThan(item("Price"), s.Value(500))
	items := s.Object(s.GlobalScope(), "items")

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
		`EXISTS (SELECT 1 FROM unnest("items") AS "item_1" WHERE "item_1"."Active" OR "item_1"."Price" > $1)`,
	}})
}

// A schema named a collection by its last name alone, so the items of a store
// and the items of a category were one collection with one table; then by its
// whole path in the aggregate, which is the query's. It is the foreign keys of
// the storage, and a tree names a collection by its table: the key of that
// table that references the row the tree stands in.
func TestACollectionIsNamedByItsTable(t *testing.T) {
	schema := NewSchemaRegistry("stores").WithAlias("s").
		ForeignKey("store_items", "store_id", "stores", "id").
		ForeignKey("categories", "store_id", "stores", "id").
		ForeignKey("category_items", "category_id", "categories", "id")
	ofTheStore := s.Wildcard(s.Object(s.GlobalScope(), "store_items"), item("Active"))
	ofACategory := s.Wildcard(
		s.Object(s.GlobalScope(), "categories"),
		s.Wildcard(s.Object(s.Item(), "category_items"), item("Active")),
	)
	checkSql(t, []sqlCase{
		{
			ofTheStore,
			`EXISTS (SELECT 1 FROM "store_items" AS "store_item_1"` +
				` WHERE "store_item_1"."store_id" = "s"."id" AND "store_item_1"."Active")`,
		},
		{
			ofACategory,
			`EXISTS (SELECT 1 FROM "categories" AS "category_1"` +
				` WHERE "category_1"."store_id" = "s"."id" AND EXISTS (SELECT 1 FROM "category_items" AS "category_item_2"` +
				` WHERE "category_item_2"."category_id" = "category_1"."id" AND "category_item_2"."Active"))`,
		},
		// A name without a key to the row is an array in it.
		{
			s.Wildcard(
				s.Object(s.GlobalScope(), "categories"),
				s.Wildcard(s.Object(s.Item(), "Items"), item("Active")),
			),
			`EXISTS (SELECT 1 FROM "categories" AS "category_1"` +
				` WHERE "category_1"."store_id" = "s"."id" AND EXISTS (SELECT 1 FROM unnest("category_1"."Items") AS "item_2"` +
				` WHERE "item_2"."Active"))`,
		},
	}, WithSchema(schema))
}

// A schema is the foreign keys of a storage and nothing of any query. A tree
// names a collection by its table, and where two keys of that table reference
// the row it is named from - the transfers from an account and the transfers
// to it - by the key's name, which is what PostgreSQL calls it. An object is
// named by the key's column. A row of an array, which has no table, is named
// by the array's column; and what the compiler calls a row in a query is its
// own.
func TestASchemaIsTheForeignKeysOfTheStorage(t *testing.T) {
	schema := NewSchemaRegistry("accounts").WithAlias("a").
		ForeignKey("transfers", "from_account_id", "accounts", "id").
		ForeignKey("transfers", "to_account_id", "accounts", "id").
		ForeignKey("accounts", "owner_id", "owners", "id").
		ForeignKey("accounts.cards", "issuer_id", "banks", "id")
	over := func(what string) s.Visitable {
		return s.Wildcard(s.Object(s.GlobalScope(), what), s.GreaterThan(item("amount"), s.Value(100)))
	}
	refused := func(t *testing.T, tree s.Visitable, schema *SchemaRegistry, want string) {
		t.Helper()
		sql, _, err := CompileToSQL(tree, WithSchema(schema))
		if err == nil {
			t.Fatalf("compiled to %q", sql)
		}
		if err.Error() != want {
			t.Errorf("got error %q\nwant      %q", err, want)
		}
	}

	// A key is named as PostgreSQL names it, unless named.
	names := map[string]ForeignKey{
		"transfers_from_account_id_fkey":    {Table: "transfers", Columns: []string{"from_account_id"}, ReferencedTable: "accounts", ReferencedColumns: []string{"id"}},
		"orders_tenant_id_customer_id_fkey": {Table: "public.orders", Columns: []string{"tenant_id", "customer_id"}, ReferencedTable: "tenants", ReferencedColumns: []string{"tenant_id", "id"}},
		"outgoing":                          {ConstraintName: "outgoing", Table: "transfers", Columns: []string{"from_account_id"}, ReferencedTable: "accounts", ReferencedColumns: []string{"id"}},
	}
	for want, key := range names {
		if got := key.Name(); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}

	// Two keys to one row are told apart by name.
	checkSql(t, []sqlCase{
		{
			over("transfers_from_account_id_fkey"),
			`EXISTS (SELECT 1 FROM "transfers" AS "transfer_1"` +
				` WHERE "transfer_1"."from_account_id" = "a"."id" AND "transfer_1"."amount" > $1)`,
		},
		{
			over("transfers_to_account_id_fkey"),
			`EXISTS (SELECT 1 FROM "transfers" AS "transfer_1"` +
				` WHERE "transfer_1"."to_account_id" = "a"."id" AND "transfer_1"."amount" > $1)`,
		},
		// A key on a row of an array.
		{
			s.Wildcard(s.Object(s.GlobalScope(), "cards"), s.Equal(s.Field(s.Object(s.Item(), "issuer_id"), "name"), s.Value("x"))),
			`EXISTS (SELECT 1 FROM unnest("cards") AS "card_1" WHERE` +
				` (SELECT "bank_2"."name" FROM "banks" AS "bank_2" WHERE "bank_2"."id" = "card_1"."issuer_id") = $1)`,
		},
	}, WithSchema(schema))
	// By the table alone, the name fits two keys.
	refused(t, over("transfers"), schema, "transfers has 2 keys to accounts: transfers_from_account_id_fkey, transfers_to_account_id_fkey; name the key")
	// A key given a name goes by it.
	named := NewSchemaRegistry("accounts").WithAlias("a").Key(ForeignKey{
		ConstraintName: "outgoing", Table: "transfers", Columns: []string{"from_account_id"},
		ReferencedTable: "accounts", ReferencedColumns: []string{"id"},
	})
	checkSql(t, []sqlCase{{
		over("outgoing"),
		`EXISTS (SELECT 1 FROM "transfers" AS "transfer_1"` +
			` WHERE "transfer_1"."from_account_id" = "a"."id" AND "transfer_1"."amount" > $1)`,
	}}, WithSchema(named))
	// A key named where it does not go is refused.
	refused(t, s.Wildcard(s.Object(s.GlobalScope(), "accounts_owner_id_fkey"), item("x")), schema, "the key accounts_owner_id_fkey references owners, not accounts")

	// A column of two keys.
	shared := NewSchemaRegistry("stores").
		ForeignKey("stores", "tenant_id", "tenants", "id").
		Key(ForeignKey{
			Table: "stores", Columns: []string{"tenant_id", "owner_id"},
			ReferencedTable: "owners", ReferencedColumns: []string{"tenant_id", "id"},
		})
	refused(
		t, s.Equal(s.Field(s.Object(s.GlobalScope(), "tenant_id"), "name"), s.Value("x")), shared,
		"tenant_id is a column of 2 keys of stores: stores_tenant_id_fkey, stores_tenant_id_owner_id_fkey; name the key",
	)
	checkSql(t, []sqlCase{{
		s.Equal(s.Field(s.Object(s.GlobalScope(), "owner_id"), "name"), s.Value("x")),
		`(SELECT "owner_1"."name" FROM "owners" AS "owner_1"` +
			` WHERE "owner_1"."tenant_id" = "stores"."tenant_id" AND "owner_1"."id" = "stores"."owner_id") = $1`,
	}}, WithSchema(shared))
}

// A relational collection inside the predicate of another was joined to the
// enclosing item whatever it was a collection of.
func TestACollectionOfTheCandidateInsideAnotherJoinsToTheRoot(t *testing.T) {
	schema := NewSchemaRegistry("stores").WithAlias("s").
		ForeignKey("items", "store_id", "stores", "id").
		ForeignKey("tags", "store_id", "stores", "id").
		ForeignKey("item_tags", "item_id", "items", "id")
	onSale := s.Equal(item("Name"), s.Value("sale"))
	items := s.Object(s.GlobalScope(), "items")

	checkSql(t, []sqlCase{
		{
			s.Wildcard(items, s.Wildcard(s.Object(s.GlobalScope(), "tags"), onSale)),
			`EXISTS (SELECT 1 FROM "items" AS "item_1" WHERE "item_1"."store_id" = "s"."id"` +
				` AND EXISTS (SELECT 1 FROM "tags" AS "tag_2" WHERE "tag_2"."store_id" = "s"."id"` +
				` AND "tag_2"."Name" = $1))`,
		},
		{
			s.Wildcard(items, s.Wildcard(s.Object(s.Item(), "item_tags"), onSale)),
			`EXISTS (SELECT 1 FROM "items" AS "item_1" WHERE "item_1"."store_id" = "s"."id"` +
				` AND EXISTS (SELECT 1 FROM "item_tags" AS "item_tag_2" WHERE "item_tag_2"."item_id" = "item_1"."id"` +
				` AND "item_tag_2"."Name" = $1))`,
		},
	}, WithSchema(schema))
}

// The visitor asked whether the field's immediate parent was the item, not
// what its path started at: the parent of `name` in `@.maker.name` is the
// object `maker`, so the item's alias was dropped and `"maker"."name"` written
// - to PostgreSQL a table and a column, an error if there is no such table and
// the column of another table if the query has one of that name. A member of
// a Value Object inside an item is a member of a composite kept in the item's
// row. The rows are in TestASpecificationSelectsTheRowsItIsSatisfiedBy.
func TestAMemberOfAnObjectInsideAnItemIsAMemberOfAComposite(t *testing.T) {
	maker := func(names ...string) s.Visitable {
		var object s.EmptiableObject = s.Object(s.Item(), "maker")
		for _, name := range names[:len(names)-1] {
			object = s.Object(object, name)
		}
		return s.Equal(s.Field(object, names[len(names)-1]), s.Value("x"))
	}
	items := s.Object(s.GlobalScope(), "items")
	checkSql(t, []sqlCase{
		{
			s.Wildcard(items, maker("name")),
			`EXISTS (SELECT 1 FROM unnest("items") AS "item_1" WHERE ("item_1"."maker")."name" = $1)`,
		},
		{
			s.Wildcard(items, maker("country", "code")),
			`EXISTS (SELECT 1 FROM unnest("items") AS "item_1" WHERE (("item_1"."maker")."country")."code" = $1)`,
		},
		// The item of an inner collection, which is itself a member of the outer item.
		{
			s.Wildcard(items, s.Wildcard(s.Object(s.Item(), "parts"), maker("name"))),
			`EXISTS (SELECT 1 FROM unnest("items") AS "item_1" WHERE EXISTS (SELECT 1 FROM unnest("item_1"."parts") AS "part_2" WHERE ("part_2"."maker")."name" = $1))`,
		},
		// From the candidate the dots stay: a qualified name, the column `maker` of `s`.
		{s.Equal(s.Field(s.Object(s.GlobalScope(), "s"), "maker"), s.Value("x")), `"s"."maker" = $1`},
	})
	// In a table of its own an item is a row as well, and its column a composite.
	schema := NewSchemaRegistry("stores").WithAlias("s").
		ForeignKey("store_items", "store_id", "stores", "id")
	checkSql(t, []sqlCase{{
		s.Wildcard(s.Object(s.GlobalScope(), "store_items"), maker("name")),
		`EXISTS (SELECT 1 FROM "store_items" AS "store_item_1" WHERE "store_item_1"."store_id" = "s"."id" AND ("store_item_1"."maker")."name" = $1)`,
	}}, WithSchema(schema))
}

// An object on the way to a member is looked up in the schema, as a collection
// is. Kept in a table of its own it is read through its key, by a subquery in
// the column's place: at most the one row the key names, and null if there is
// none. There was no way to say so: the dots were written as they stood, which
// PostgreSQL reads as a table and a column.
func TestAMemberOfAnObjectKeptInATableOfItsOwnIsReadThroughTheKey(t *testing.T) {
	items := s.Object(s.GlobalScope(), "items")
	ownerName := s.Field(s.Object(s.Item(), "owner_id"), "name")
	named := s.Wildcard(items, s.Equal(ownerName, s.Value("ann")))
	stores := func() *SchemaRegistry { return NewSchemaRegistry("stores").WithAlias("s") }

	// Whether the items are an array or a table, their owner is a table. A
	// row of the items array has no table: it is named by the array's column.
	checkSql(t, []sqlCase{{
		named,
		`EXISTS (SELECT 1 FROM unnest("items") AS "item_1" WHERE ` +
			`(SELECT "owner_2"."name" FROM "owners" AS "owner_2" ` +
			`WHERE "owner_2"."id" = "item_1"."owner_id") = $1)`,
	}}, WithSchema(stores().ForeignKey("stores.items", "owner_id", "owners", "id")))

	relational := stores().
		ForeignKey("store_items", "store_id", "stores", "id").
		ForeignKey("store_items", "owner_id", "owners", "id")
	checkSql(t, []sqlCase{{
		s.Wildcard(s.Object(s.GlobalScope(), "store_items"), s.Equal(ownerName, s.Value("ann"))),
		`EXISTS (SELECT 1 FROM "store_items" AS "store_item_1" ` +
			`WHERE "store_item_1"."store_id" = "s"."id" AND ` +
			`(SELECT "owner_2"."name" FROM "owners" AS "owner_2" ` +
			`WHERE "owner_2"."id" = "store_item_1"."owner_id") = $1)`,
	}}, WithSchema(relational))

	// A key of two columns is named by either of them, unless another key has
	// it too; and what is inside the owner's row is a composite.
	compositeKey := stores().Key(ForeignKey{
		Table: "stores.items", Columns: []string{"tenant_id", "owner_id"},
		ReferencedTable: "public.owners", ReferencedColumns: []string{"tenant_id", "id"},
	})
	city := s.Field(s.Object(s.Object(s.Item(), "owner_id"), "address"), "city")
	checkSql(t, []sqlCase{{
		s.Wildcard(items, s.IsNull(city)),
		`EXISTS (SELECT 1 FROM unnest("items") AS "item_1" WHERE ` +
			`(SELECT ("owner_2"."address")."city" FROM "public"."owners" AS "owner_2" ` +
			`WHERE "owner_2"."tenant_id" = "item_1"."tenant_id" ` +
			`AND "owner_2"."id" = "item_1"."owner_id") IS NULL)`,
	}}, WithSchema(compositeKey))

	// Of the candidate itself, the key is the root row's; and each object read
	// so has an alias of its own.
	ofBoth := stores().
		ForeignKey("stores", "owner_id", "owners", "id").
		ForeignKey("stores.items", "owner_id", "owners", "id")
	ofTheStore := s.Field(s.Object(s.GlobalScope(), "owner_id"), "name")
	checkSql(t, []sqlCase{
		{
			s.Wildcard(items, s.Equal(ownerName, ofTheStore)),
			`EXISTS (SELECT 1 FROM unnest("items") AS "item_1" WHERE ` +
				`(SELECT "owner_2"."name" FROM "owners" AS "owner_2" ` +
				`WHERE "owner_2"."id" = "item_1"."owner_id") = ` +
				`(SELECT "owner_3"."name" FROM "owners" AS "owner_3" ` +
				`WHERE "owner_3"."id" = "s"."owner_id"))`,
		},
		// What the schema does not mention stays what the dots have meant.
		{s.Equal(s.Field(s.Object(s.GlobalScope(), "s"), "name"), s.Value("x")), `"s"."name" = $1`},
		{
			s.Wildcard(items, s.Equal(s.Field(s.Object(s.Item(), "maker"), "name"), s.Value("x"))),
			`EXISTS (SELECT 1 FROM unnest("items") AS "item_1" WHERE ("item_1"."maker")."name" = $1)`,
		},
	}, WithSchema(ofBoth))
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

	schema := func(table, column, referencedColumn, alias string) *SchemaRegistry {
		return NewSchemaRegistry("stores").WithAlias(alias).ForeignKey(table, column, "stores", referencedColumn)
	}
	schemas := map[string]struct {
		schema *SchemaRegistry
		table  string
	}{
		"a table":             {schema(injection, "store_id", "id", "s"), injection},
		"a column":            {schema("items", injection, "id", "s"), "items"},
		"a referenced column": {schema("items", "store_id", injection, "s"), "items"},
		"an alias":            {schema("items", "store_id", "id", injection), "items"},
	}
	for name, c := range schemas {
		t.Run(name, func(t *testing.T) {
			active := s.Wildcard(s.Object(s.GlobalScope(), c.table), item("Active"))
			if sql, _, err := CompileToSQL(active, WithSchema(c.schema)); err == nil {
				t.Fatalf("compiled to %q", sql)
			}
		})
	}

	checkSql(t, []sqlCase{
		{s.Field(s.Object(s.GlobalScope(), "users"), "_name1"), `"users"."_name1"`},
	})
	qualified := NewSchemaRegistry("stores").ForeignKey("public.items", "store_id", "stores", "id")
	checkSql(t, []sqlCase{{
		s.Wildcard(s.Object(s.GlobalScope(), "public.items"), item("Active")),
		`EXISTS (SELECT 1 FROM "public"."items" AS "item_1" WHERE "item_1"."store_id" = "stores"."id" AND "item_1"."Active")`,
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

	transformed, err := NewMappingVisitor(context).Transform(s.Equal(field("id"), s.Value(memberId)))
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
			got, err := NewMappingVisitor(context).Transform(specification)
			if err == nil {
				t.Errorf("transformed to %#v", got)
			}
		})
	}

	// Inside the predicate of a collection.
	inside, err := NewMappingVisitor(context).Transform(
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

// membersContext maps a domain whose id is composite, of a member and of an
// item.
type membersContext struct{}

func (membersContext) AttrNode(path []string) (Mapped, error) {
	owner := path[:len(path)-1]
	if path[len(path)-1] == "id" {
		return compositeOf(
			fromCandidate(append(append([]string{}, owner...), "tenant_id")...),
			fromCandidate(append(append([]string{}, owner...), "member_id")...),
		), nil
	}
	return Scalar(fromCandidate(path...)), nil
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

// partsContext maps a domain whose parts have a weight, kept in grams: by
// the whole path from the candidate, the parts of a part included.
type partsContext struct{}

func (partsContext) AttrNode(path []string) (Mapped, error) {
	if len(path) == 1 && path[0] == "rank" {
		return Scalar(field("rank")), nil
	}
	// The parts, of the candidate or of a part, and the weight of one.
	last := path[len(path)-1]
	ofParts := last == "parts" || last == "weight"
	for _, name := range path[:len(path)-1] {
		ofParts = ofParts && name == "parts"
	}
	if ofParts {
		names := append([]string{}, path...)
		if last == "weight" {
			names[len(names)-1] = "weight_grams"
		}
		return Scalar(fromCandidate(names...)), nil
	}
	return nil, fmt.Errorf("unknown field: %s", strings.Join(path, "."))
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
			got, err := NewMappingVisitor(partsContext{}).Transform(c.from)
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

// storedPartsContext says as well where the collections are kept: a
// collection is a member, and where it is a path from the candidate.
type storedPartsContext struct{ partsContext }

func (c storedPartsContext) AttrNode(path []string) (Mapped, error) {
	storage := map[string][]string{
		"parts":              {"something_parts"},
		"parts.weight":       {"something_parts", "weight_grams"},
		"parts.parts":        {"something_parts", "detail", "sub_parts"},
		"parts.parts.weight": {"something_parts", "detail", "sub_parts", "weight_grams"},
	}
	if names, ok := storage[strings.Join(path, ".")]; ok {
		return Scalar(fromCandidate(names...)), nil
	}
	return c.partsContext.AttrNode(path)
}

// recordingMapping keeps what it is asked about.
type recordingMapping struct{ asked *[][]string }

func (m recordingMapping) AttrNode(path []string) (Mapped, error) {
	*m.asked = append(*m.asked, path)
	if len(path) == 2 {
		return Scalar(field("warehouse_shelves")), nil
	}
	return Scalar(fromCandidate("warehouse_shelves", "weight_grams")), nil
}

func (recordingMapping) ValueNode(val any) (Mapped, error) {
	return Scalar(s.Value(val)), nil
}

// The transformer mapped the fields and the values of a specification and
// left the collection under the domain's name: `unnest(parts)` of a column
// that is `something_parts`. A collection is a member like any other: the
// mapping says where it is, by a path from the candidate, and a member of its
// item under it.
func TestACollectionIsKeptWhereTheMappingSays(t *testing.T) {
	heavy := s.Wildcard(s.Object(s.GlobalScope(), "parts"), s.GreaterThan(item("weight"), s.Value(weight{100})))
	nested := s.Wildcard(
		s.Object(s.GlobalScope(), "parts"),
		s.Wildcard(s.Object(s.Item(), "parts"), s.GreaterThan(item("weight"), s.Value(weight{5}))),
	)
	transformed := func(mapping Mapping, from s.Visitable) s.Visitable {
		t.Helper()
		got, err := NewMappingVisitor(mapping).Transform(from)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	trees := []struct {
		name string
		got  s.Visitable
		want s.Visitable
	}{
		{
			"a collection of the candidate", transformed(storedPartsContext{}, heavy),
			s.Wildcard(s.Object(s.GlobalScope(), "something_parts"), s.GreaterThan(item("weight_grams"), s.Value(100))),
		},
		// Under the item's collection in the answer, and from the item in
		// the tree.
		{
			"a collection of an item", transformed(storedPartsContext{}, nested),
			s.Wildcard(
				s.Object(s.GlobalScope(), "something_parts"),
				s.Wildcard(s.Object(s.Object(s.Item(), "detail"), "sub_parts"), s.GreaterThan(item("weight_grams"), s.Value(5))),
			),
		},
		{
			"a mapping that renames the members alone keeps a collection where it is",
			transformed(partsContext{}, nested),
			s.Wildcard(
				s.Object(s.GlobalScope(), "parts"),
				s.Wildcard(s.Object(s.Item(), "parts"), s.GreaterThan(item("weight_grams"), s.Value(5))),
			),
		},
	}
	for _, c := range trees {
		t.Run(c.name, func(t *testing.T) {
			if !s.SameTree(c.got, c.want) {
				t.Errorf("got  %#v\nwant %#v", c.got, c.want)
			}
		})
	}

	checkSql(t, []sqlCase{
		{
			transformed(storedPartsContext{}, heavy),
			`EXISTS (SELECT 1 FROM unnest("something_parts") AS "something_part_1" WHERE "something_part_1"."weight_grams" > $1)`,
		},
		{
			transformed(partsContext{}, nested),
			`EXISTS (SELECT 1 FROM unnest("parts") AS "part_1"` +
				` WHERE EXISTS (SELECT 1 FROM unnest("part_1"."parts") AS "part_2" WHERE "part_2"."weight_grams" > $1))`,
		},
	})

	// The whole path is asked about.
	var asked [][]string
	inTheStore := transformed(
		recordingMapping{&asked},
		s.Wildcard(s.Object(s.Object(s.GlobalScope(), "warehouse"), "shelves"), item("weight")),
	)
	if !reflect.DeepEqual(asked, [][]string{{"warehouse", "shelves"}, {"warehouse", "shelves", "weight"}}) {
		t.Errorf("asked about %v", asked)
	}
	if want := s.Wildcard(s.Object(s.GlobalScope(), "warehouse_shelves"), item("weight_grams")); !s.SameTree(inTheStore, want) {
		t.Errorf("got  %#v\nwant %#v", inTheStore, want)
	}

	// A refusal of the mapping is not hidden.
	if got, err := NewMappingVisitor(storedPartsContext{}).Transform(s.Wildcard(s.Object(s.GlobalScope(), "wheels"), item("weight"))); err == nil {
		t.Errorf("transformed to %#v", got)
	}

	// The query is compiled of the transformed tree: a schema is of the
	// storage, and knows the collection by the name it has there.
	schema := NewSchemaRegistry("things").WithAlias("t").ForeignKey("something_parts", "thing_id", "things", "id")
	sql, params, err := Compile(storedPartsContext{}, heavy, WithSchema(schema))
	if err != nil {
		t.Fatal(err)
	}
	want := `EXISTS (SELECT 1 FROM "something_parts" AS "something_part_1" WHERE "something_part_1"."thing_id" = "t"."id" AND "something_part_1"."weight_grams" > $1)`
	if sql != want || !reflect.DeepEqual(params, []any{100}) {
		t.Errorf("got  %s, %v\nwant %s", sql, params, want)
	}
}

// leastContext says what a mapping must, and nothing more: a renaming of
// every name of a path, the same for a member and for the collection on its
// way.
type leastContext struct{}

func (leastContext) AttrNode(path []string) (Mapped, error) {
	names := make([]string, 0, len(path))
	for _, name := range path {
		names = append(names, "stored_"+name)
	}
	return Scalar(fromCandidate(names...)), nil
}

func (leastContext) ValueNode(val any) (Mapped, error) {
	return Scalar(s.Value(val)), nil
}

// A mapping and a schema could not be given together: Compile took a context
// and no options, CompileToSQL options and no context. Both are the
// repository's to know - a query cannot be written without knowing the table
// - and it gives both.
func TestAMappingAndASchemaAreGivenTogether(t *testing.T) {
	schema := NewSchemaRegistry("stores").WithAlias("s").
		ForeignKey("store_items", "store_id", "stores", "id")
	dear := s.Wildcard(s.Object(s.GlobalScope(), "Items"), s.GreaterThan(item("Price"), s.Value(500)))
	sql, params, err := Compile(storeItemsContext{}, dear, WithSchema(schema))
	if err != nil {
		t.Fatal(err)
	}
	want := `EXISTS (SELECT 1 FROM "store_items" AS "store_item_1" WHERE "store_item_1"."store_id" = "s"."id" AND "store_item_1"."price_cents" > $1)`
	if sql != want || !reflect.DeepEqual(params, []any{500}) {
		t.Errorf("got  %s, %v\nwant %s", sql, params, want)
	}
}

// storeItemsContext names the members of a store as the storage does.
type storeItemsContext struct{}

func (storeItemsContext) AttrNode(path []string) (Mapped, error) {
	switch strings.Join(path, ".") {
	case "Items":
		return Scalar(field("store_items")), nil
	case "Items.Price":
		return Scalar(fromCandidate("store_items", "price_cents")), nil
	}
	return nil, fmt.Errorf("no such member of a store: %s", strings.Join(path, "."))
}

func (storeItemsContext) ValueNode(val any) (Mapped, error) {
	return Scalar(s.Value(val)), nil
}

// The mapping is asked about a member by its whole path from the candidate,
// the collection on the way included, and puts the answer where the member
// was; it used to be asked about "the item" by the names alone, under a
// method of its own.
func TestWhatAMappingMustSayAndWhatItMay(t *testing.T) {
	var _ Mapping = leastContext{}

	sql, _, err := Compile(leastContext{}, s.Equal(s.Field(s.Object(s.GlobalScope(), "profile"), "age"), s.Value(30)))
	if err != nil || sql != `"stored_profile"."stored_age" = $1` {
		t.Errorf("got %q, %v", sql, err)
	}

	// A collection is a member like any other, and the least said of it
	// renames it too - at any depth and from either root.
	deep := s.Wildcard(
		s.Object(s.Object(s.GlobalScope(), "warehouse"), "shelves"),
		s.Wildcard(s.Object(s.Object(s.Item(), "box"), "parts"), s.GreaterThan(item("rank"), s.Value(1))),
	)
	got, err := NewMappingVisitor(leastContext{}).Transform(deep)
	if err != nil {
		t.Fatal(err)
	}
	want := s.Wildcard(
		s.Object(s.Object(s.GlobalScope(), "stored_warehouse"), "stored_shelves"),
		s.Wildcard(s.Object(s.Object(s.Item(), "stored_box"), "stored_parts"), s.GreaterThan(item("stored_rank"), s.Value(1))),
	)
	if !s.SameTree(got, want) {
		t.Errorf("got  %#v\nwant %#v", got, want)
	}

	// A mapping that answers with the last name alone puts the member of an
	// item beside the candidate's columns, outside its collection.
	_, _, err = Compile(lastNameMapping{}, s.Wildcard(s.Object(s.GlobalScope(), "items"), item("price")))
	if err == nil || !strings.Contains(err.Error(), "price") {
		t.Errorf("got %v, want an error naming the field", err)
	}
}

// lastNameMapping answers with the last name of a path alone.
type lastNameMapping struct{}

func (lastNameMapping) AttrNode(path []string) (Mapped, error) {
	return Scalar(field(path[len(path)-1])), nil
}

func (lastNameMapping) ValueNode(val any) (Mapped, error) {
	return Scalar(s.Value(val)), nil
}
