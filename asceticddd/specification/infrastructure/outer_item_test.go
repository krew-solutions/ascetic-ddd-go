package specification

import (
	"reflect"
	"testing"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

// The item of an enclosing collection has an alias of its own, which the
// inner query names as SQL lets it: OuterItem(1) is that alias.
func TestTheItemOfAnEnclosingCollectionIsItsAlias(t *testing.T) {
	overItsCategory := s.Wildcard(s.Object(s.GlobalScope(), "categories"), s.Wildcard(
		s.Object(s.Item(), "products"), s.GreaterThan(item("price"), s.Field(s.OuterItem(1), "limit")),
	))
	checkSql(t, []sqlCase{{
		overItsCategory,
		`EXISTS (SELECT 1 FROM unnest("categories") AS "category_1"` +
			` WHERE EXISTS (SELECT 1 FROM unnest("category_1"."products") AS "product_2"` +
			` WHERE "product_2"."price" > "category_1"."limit"))`,
	}})
	// In tables of their own, the enclosing row is the one the keys point at,
	// and its columns are named the same way.
	schema := NewSchemaRegistry("shops").
		RegisterRelational("categories", "categories", "shop_id", "id").
		RegisterRelational("categories.products", "products", "category_id", "id")
	checkSql(t, []sqlCase{{
		overItsCategory,
		`EXISTS (SELECT 1 FROM "categories" AS "category_1" WHERE "category_1"."shop_id" = "shops"."id"` +
			` AND EXISTS (SELECT 1 FROM "products" AS "product_2" WHERE "product_2"."category_id" = "category_1"."id"` +
			` AND "product_2"."price" > "category_1"."limit"))`,
	}}, WithSchema(schema))
	// Two collections out, the candidate's own row beside.
	threeDeep := s.Wildcard(s.Object(s.GlobalScope(), "categories"), s.Wildcard(
		s.Object(s.Item(), "products"), s.Wildcard(s.Object(s.Item(), "tags"), s.And(
			s.GreaterThan(item("weight"), s.Field(s.OuterItem(2), "limit")),
			s.LessThan(s.Field(s.OuterItem(1), "price"), field("limit")),
		)),
	))
	checkSql(t, []sqlCase{{
		threeDeep,
		`EXISTS (SELECT 1 FROM unnest("categories") AS "category_1"` +
			` WHERE EXISTS (SELECT 1 FROM unnest("category_1"."products") AS "product_2"` +
			` WHERE EXISTS (SELECT 1 FROM unnest("product_2"."tags") AS "tag_3"` +
			` WHERE "tag_3"."weight" > "category_1"."limit" AND "product_2"."price" < "shops"."limit")))`,
	}}, WithSchema(NewSchemaRegistry("shops")))
	if _, _, err := CompileToSQL(s.Wildcard(s.Object(s.GlobalScope(), "categories"), s.GreaterThan(item("limit"), s.Field(s.OuterItem(1), "limit")))); err == nil {
		t.Error("one collection deep there is no item further out")
	}
}

// Inside a collection's predicate the candidate's column is qualified with
// its row. Unqualified, PostgreSQL read it from the innermost row that has a
// column of that name: a category with a `limit` of its own hid the shop's,
// and the query selected other rows than the evaluator was satisfied by. The
// row is what the schema calls it, so without a schema there is no query.
func TestTheCandidatesColumnInsideAPredicateIsQualifiedWithItsRow(t *testing.T) {
	overTheShopsLimit := s.Wildcard(s.Object(s.GlobalScope(), "categories"), s.GreaterThan(item("limit"), field("limit")))
	checkSql(t, []sqlCase{{
		overTheShopsLimit,
		`EXISTS (SELECT 1 FROM unnest("categories") AS "category_1" WHERE "category_1"."limit" > "shops"."limit")`,
	}}, WithSchema(NewSchemaRegistry("shops")))
	checkSql(t, []sqlCase{{
		overTheShopsLimit,
		`EXISTS (SELECT 1 FROM unnest("categories") AS "category_1" WHERE "category_1"."limit" > "s"."limit")`,
	}}, WithSchema(NewSchemaRegistry("public.shops").WithParentAlias("s")))
	if _, _, err := CompileToSQL(overTheShopsLimit); err == nil {
		t.Error("without a schema there is no query")
	}
	// A name of several parts the author qualified, and it stays as written;
	// outside a collection's predicate a name is unqualified, as it was.
	checkSql(t, []sqlCase{
		{
			s.Wildcard(s.Object(s.GlobalScope(), "categories"), s.GreaterThan(item("limit"), field("s.limit"))),
			`EXISTS (SELECT 1 FROM unnest("categories") AS "category_1" WHERE "category_1"."limit" > "s"."limit")`,
		},
		{s.GreaterThan(field("limit"), s.Value(1)), `"limit" > $1`},
	})
}

// A mapping names the column of an item's member from Item(), not knowing how
// far out the item is: the transformer puts the column where the member was.
type rowMapping struct{ ContextDefaults }

func (rowMapping) AttrNode(path []string) (Mapped, error) {
	return Scalar(field(path[0])), nil
}

func (rowMapping) ItemAttrNode(path []string) (Mapped, error) {
	if path[0] == "id" {
		return compositeOf(item("tenant_id"), item("member_id")), nil
	}
	return Scalar(s.Field(s.Object(s.Item(), "row"), path[0])), nil
}

func (rowMapping) ValueNode(val any) (Mapped, error) {
	return Scalar(s.Value(val)), nil
}

func TestAMappingKeepsTheItemWhereItWas(t *testing.T) {
	transformed, err := NewTransformVisitor(rowMapping{}).Transform(s.Wildcard(s.Object(s.GlobalScope(), "categories"), s.Wildcard(
		s.Object(s.Item(), "products"), s.And(
			s.GreaterThan(item("price"), s.Field(s.OuterItem(1), "limit")),
			s.Equal(s.Field(s.OuterItem(1), "id"), s.Field(s.Item(), "id")),
		),
	)))
	if err != nil {
		t.Fatal(err)
	}
	want := s.Wildcard(s.Object(s.GlobalScope(), "categories"), s.Wildcard(
		s.Object(s.Item(), "products"), s.And(
			s.GreaterThan(s.Field(s.Object(s.Item(), "row"), "price"), s.Field(s.Object(s.OuterItem(1), "row"), "limit")),
			s.And(
				s.Equal(s.Field(s.OuterItem(1), "tenant_id"), item("tenant_id")),
				s.Equal(s.Field(s.OuterItem(1), "member_id"), item("member_id")),
			),
		),
	))
	if !reflect.DeepEqual(s.Describe(transformed), s.Describe(want)) {
		t.Errorf("got  %v\nwant %v", s.Describe(transformed), s.Describe(want))
	}
}
