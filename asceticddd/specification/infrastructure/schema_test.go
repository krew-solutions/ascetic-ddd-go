package specification

import (
	"testing"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

func TestSchemaRegistry_RelationalSimpleFK(t *testing.T) {
	// Setup: stores table with items in separate table
	schema := NewSchemaRegistry("stores").
		WithAlias("s").
		ForeignKey("items", "store_id", "stores", "id")

	// AST: spec.Wildcard(spec.Object(spec.GlobalScope(), "items"), spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Value(1000)))
	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "items"),
		s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	fragment, err := visitor.Compile(ast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sql, params := fragment.SQL, fragment.Params

	expectedSQL := `EXISTS (SELECT 1 FROM "items" AS "item_1" WHERE "item_1"."store_id" = "s"."id" AND "item_1"."Price" > $1)`
	if sql != expectedSQL {
		t.Errorf("unexpected SQL:\nexpected: %s\ngot:      %s", expectedSQL, sql)
	}

	if len(params) != 1 || params[0] != 1000 {
		t.Errorf("unexpected params: %v", params)
	}
}

func TestSchemaRegistry_RelationalCompositeFK(t *testing.T) {
	// Setup: multi-tenant stores table with items in separate table
	// FK: (tenant_id, store_id) -> (tenant_id, id)
	schema := NewSchemaRegistry("stores").
		WithAlias("s").
		Key(ForeignKey{
			Table: "items", Columns: []string{"tenant_id", "store_id"},
			ReferencedTable: "stores", ReferencedColumns: []string{"tenant_id", "id"},
		})

	// AST: spec.Wildcard(spec.Object(spec.GlobalScope(), "items"), spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Value(1000)))
	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "items"),
		s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	fragment, err := visitor.Compile(ast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sql, params := fragment.SQL, fragment.Params

	expectedSQL := `EXISTS (SELECT 1 FROM "items" AS "item_1" WHERE "item_1"."tenant_id" = "s"."tenant_id" AND "item_1"."store_id" = "s"."id" AND "item_1"."Price" > $1)`
	if sql != expectedSQL {
		t.Errorf("unexpected SQL:\nexpected: %s\ngot:      %s", expectedSQL, sql)
	}

	if len(params) != 1 || params[0] != 1000 {
		t.Errorf("unexpected params: %v", params)
	}
}

func TestSchemaRegistry_RelationalTripleCompositeFK(t *testing.T) {
	// Setup: multi-tenant, multi-region stores with items
	// FK: (tenant_id, region_id, store_id) -> (tenant_id, region_id, id)
	schema := NewSchemaRegistry("stores").
		WithAlias("s").
		Key(ForeignKey{
			Table: "items", Columns: []string{"tenant_id", "region_id", "store_id"},
			ReferencedTable: "stores", ReferencedColumns: []string{"tenant_id", "region_id", "id"},
		})

	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "items"),
		s.Equal(s.Field(s.Item(), "Active"), s.Value(true)),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	fragment, err := visitor.Compile(ast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sql := fragment.SQL

	expectedSQL := `EXISTS (SELECT 1 FROM "items" AS "item_1" WHERE "item_1"."tenant_id" = "s"."tenant_id" AND "item_1"."region_id" = "s"."region_id" AND "item_1"."store_id" = "s"."id" AND "item_1"."Active" = $1)`
	if sql != expectedSQL {
		t.Errorf("unexpected SQL:\nexpected: %s\ngot:      %s", expectedSQL, sql)
	}
}

func TestSchemaRegistry_DefaultToEmbedded(t *testing.T) {
	// Setup: a collection the schema does not mention is an array in the row
	schema := NewSchemaRegistry("stores").WithAlias("s")

	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	fragment, err := visitor.Compile(ast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sql := fragment.SQL

	// Should default to unnest
	expectedSQL := `EXISTS (SELECT 1 FROM unnest("Items") AS "item_1" WHERE "item_1"."Price" > $1)`
	if sql != expectedSQL {
		t.Errorf("unexpected SQL:\nexpected: %s\ngot:      %s", expectedSQL, sql)
	}
}

func TestSchemaRegistry_NoSchema(t *testing.T) {
	// Setup: no schema at all (backwards compatibility)
	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
	)

	visitor := NewPostgresqlVisitor() // No schema
	fragment, err := visitor.Compile(ast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sql := fragment.SQL

	// Should default to unnest
	expectedSQL := `EXISTS (SELECT 1 FROM unnest("Items") AS "item_1" WHERE "item_1"."Price" > $1)`
	if sql != expectedSQL {
		t.Errorf("unexpected SQL:\nexpected: %s\ngot:      %s", expectedSQL, sql)
	}
}

func TestSchemaRegistry_RelationalWithComplexPredicate(t *testing.T) {
	// Setup: relational with AND predicate
	schema := NewSchemaRegistry("stores").
		WithAlias("s").
		ForeignKey("items", "store_id", "stores", "id")

	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "items"),
		s.And(
			s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
			s.Equal(s.Field(s.Item(), "Active"), s.Value(true)),
		),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	fragment, err := visitor.Compile(ast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sql, params := fragment.SQL, fragment.Params

	expectedSQL := `EXISTS (SELECT 1 FROM "items" AS "item_1" WHERE "item_1"."store_id" = "s"."id" AND "item_1"."Price" > $1 AND "item_1"."Active" = $2)`
	if sql != expectedSQL {
		t.Errorf("unexpected SQL:\nexpected: %s\ngot:      %s", expectedSQL, sql)
	}

	if len(params) != 2 {
		t.Errorf("unexpected params count: %d", len(params))
	}
}

func TestSchemaRegistry_MixedCollections(t *testing.T) {
	// Setup: one embedded, one relational
	schema := NewSchemaRegistry("stores").
		WithAlias("s").
		ForeignKey("items", "store_id", "stores", "id")

	// Test relational
	ast1 := s.Wildcard(
		s.Object(s.GlobalScope(), "items"),
		s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(100)),
	)

	visitor1 := NewPostgresqlVisitor(WithSchema(schema))
	fragment1, _ := visitor1.Compile(ast1)
	sql1 := fragment1.SQL

	if sql1 != `EXISTS (SELECT 1 FROM "items" AS "item_1" WHERE "item_1"."store_id" = "s"."id" AND "item_1"."Price" > $1)` {
		t.Errorf("unexpected SQL for Items: %s", sql1)
	}

	// Test embedded
	ast2 := s.Wildcard(
		s.Object(s.GlobalScope(), "Tags"),
		s.Equal(s.Field(s.Item(), "Name"), s.Value("sale")),
	)

	visitor2 := NewPostgresqlVisitor(WithSchema(schema))
	fragment2, _ := visitor2.Compile(ast2)
	sql2 := fragment2.SQL

	if sql2 != `EXISTS (SELECT 1 FROM unnest("Tags") AS "tag_1" WHERE "tag_1"."Name" = $1)` {
		t.Errorf("unexpected SQL for Tags: %s", sql2)
	}
}

func TestSchemaRegistry_NestedRelationalCollections(t *testing.T) {
	// Setup: stores -> categories (separate table) -> items (separate table)
	// stores.id -> categories.store_id
	// categories.id -> items.category_id
	schema := NewSchemaRegistry("stores").
		WithAlias("s").
		ForeignKey("categories", "store_id", "stores", "id").
		// A relation is of a row: the items of a row of categories.
		ForeignKey("items", "category_id", "categories", "id")

	// AST: Store has Category that has Item with Price > 1000
	// spec.Wildcard(
	//   spec.Object(spec.GlobalScope(), "categories"),
	//   spec.Wildcard(
	//     spec.Object(spec.Item(), "items"),
	//     spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Value(1000)),
	//   ),
	// )
	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "categories"),
		s.Wildcard(
			s.Object(s.Item(), "items"),
			s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
		),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	fragment, err := visitor.Compile(ast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sql, params := fragment.SQL, fragment.Params

	// Expected:
	// EXISTS (SELECT 1 FROM categories AS category_1
	//         WHERE category_1.store_id = s.id
	//         AND EXISTS (SELECT 1 FROM items AS item_2
	//                     WHERE item_2.category_id = category_1.id
	//                     AND item_2.Price > $1))
	expectedSQL := `EXISTS (SELECT 1 FROM "categories" AS "category_1" WHERE "category_1"."store_id" = "s"."id" AND EXISTS (SELECT 1 FROM "items" AS "item_2" WHERE "item_2"."category_id" = "category_1"."id" AND "item_2"."Price" > $1))`
	if sql != expectedSQL {
		t.Errorf("unexpected SQL:\nexpected: %s\ngot:      %s", expectedSQL, sql)
	}

	if len(params) != 1 || params[0] != 1000 {
		t.Errorf("unexpected params: %v", params)
	}
}

func TestSchemaRegistry_NestedRelationalWithCompositeFK(t *testing.T) {
	// Multi-tenant: stores -> categories -> items
	// All with tenant_id
	schema := NewSchemaRegistry("stores").
		WithAlias("s").
		Key(ForeignKey{
			Table: "categories", Columns: []string{"tenant_id", "store_id"},
			ReferencedTable: "stores", ReferencedColumns: []string{"tenant_id", "id"},
		}).
		Key(ForeignKey{
			Table: "items", Columns: []string{"tenant_id", "category_id"},
			ReferencedTable: "categories", ReferencedColumns: []string{"tenant_id", "id"},
		})

	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "categories"),
		s.Wildcard(
			s.Object(s.Item(), "items"),
			s.Equal(s.Field(s.Item(), "Active"), s.Value(true)),
		),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	fragment, err := visitor.Compile(ast)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sql := fragment.SQL

	expectedSQL := `EXISTS (SELECT 1 FROM "categories" AS "category_1" WHERE "category_1"."tenant_id" = "s"."tenant_id" AND "category_1"."store_id" = "s"."id" AND EXISTS (SELECT 1 FROM "items" AS "item_2" WHERE "item_2"."tenant_id" = "category_1"."tenant_id" AND "item_2"."category_id" = "category_1"."id" AND "item_2"."Active" = $1))`
	if sql != expectedSQL {
		t.Errorf("unexpected SQL:\nexpected: %s\ngot:      %s", expectedSQL, sql)
	}
}

// A key whose columns and referenced columns disagree in number, or that has
// none, was registered as it was and failed at the first query that joined
// by it: an index out of range in the compiler, or `WHERE  AND` at the
// server. A schema is declared, not read: such a key is refused where it is
// declared, in PostgreSQL's words.
func TestAKeyWhoseColumnsDisagreeIsRefusedWhereItIsDeclared(t *testing.T) {
	panicOf := func(do func()) (recovered any) {
		defer func() { recovered = recover() }()
		do()
		return nil
	}
	cases := []struct {
		key  ForeignKey
		want string
	}{
		{
			ForeignKey{Table: "items", Columns: []string{"tenant_id", "store_id"}, ReferencedTable: "stores", ReferencedColumns: []string{"id"}},
			"items (tenant_id, store_id) REFERENCES stores (id): number of referencing and referenced columns for foreign key disagree",
		},
		{
			ForeignKey{Table: "items", ReferencedTable: "stores", ReferencedColumns: []string{"id"}},
			"items () REFERENCES stores (id): a foreign key has at least one column",
		},
	}
	for _, c := range cases {
		if got := panicOf(func() { NewSchemaRegistry("stores").Key(c.key) }); got != c.want {
			t.Errorf("got  %v\nwant %q", got, c.want)
		}
	}
	// A key as it can be created is registered.
	NewSchemaRegistry("stores").Key(ForeignKey{Table: "items", Columns: []string{"store_id"}, ReferencedTable: "stores", ReferencedColumns: []string{"id"}})
}
