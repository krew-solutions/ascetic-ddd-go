package specification

import (
	"testing"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

func TestSchemaRegistry_RelationalSimpleFK(t *testing.T) {
	// Setup: stores table with items in separate table
	schema := NewSchemaRegistry("stores").
		WithParentAlias("s").
		RegisterRelational("Items", "items", "store_id", "id")

	// AST: spec.Wildcard(spec.Object(spec.GlobalScope(), "Items"), spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Value(1000)))
	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSQL := "EXISTS (SELECT 1 FROM items AS item_1 WHERE item_1.store_id = s.id AND item_1.Price > $1)"
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
		WithParentAlias("s").
		RegisterRelationalComposite("Items", "items", []ForeignKeyPair{
			{ChildColumn: "tenant_id", ParentColumn: "tenant_id"},
			{ChildColumn: "store_id", ParentColumn: "id"},
		})

	// AST: spec.Wildcard(spec.Object(spec.GlobalScope(), "Items"), spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Value(1000)))
	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSQL := "EXISTS (SELECT 1 FROM items AS item_1 WHERE item_1.tenant_id = s.tenant_id AND item_1.store_id = s.id AND item_1.Price > $1)"
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
		WithParentAlias("s").
		RegisterRelationalComposite("Items", "items", []ForeignKeyPair{
			{ChildColumn: "tenant_id", ParentColumn: "tenant_id"},
			{ChildColumn: "region_id", ParentColumn: "region_id"},
			{ChildColumn: "store_id", ParentColumn: "id"},
		})

	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.Equal(s.Field(s.Item(), "Active"), s.Value(true)),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql, _, err := visitor.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSQL := "EXISTS (SELECT 1 FROM items AS item_1 WHERE item_1.tenant_id = s.tenant_id AND item_1.region_id = s.region_id AND item_1.store_id = s.id AND item_1.Active = $1)"
	if sql != expectedSQL {
		t.Errorf("unexpected SQL:\nexpected: %s\ngot:      %s", expectedSQL, sql)
	}
}

func TestSchemaRegistry_EmbeddedCollection(t *testing.T) {
	// Setup: stores with embedded Items (JSONB)
	schema := NewSchemaRegistry("stores").
		WithParentAlias("s").
		RegisterEmbedded("Items")

	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql, _, err := visitor.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use unnest for embedded collections
	expectedSQL := "EXISTS (SELECT 1 FROM unnest(Items) AS item_1 WHERE item_1.Price > $1)"
	if sql != expectedSQL {
		t.Errorf("unexpected SQL:\nexpected: %s\ngot:      %s", expectedSQL, sql)
	}
}

func TestSchemaRegistry_DefaultToEmbedded(t *testing.T) {
	// Setup: schema without any collection registered (should default to embedded)
	schema := NewSchemaRegistry("stores").WithParentAlias("s")

	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql, _, err := visitor.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should default to unnest
	expectedSQL := "EXISTS (SELECT 1 FROM unnest(Items) AS item_1 WHERE item_1.Price > $1)"
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
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql, _, err := visitor.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should default to unnest
	expectedSQL := "EXISTS (SELECT 1 FROM unnest(Items) AS item_1 WHERE item_1.Price > $1)"
	if sql != expectedSQL {
		t.Errorf("unexpected SQL:\nexpected: %s\ngot:      %s", expectedSQL, sql)
	}
}

func TestSchemaRegistry_RelationalWithComplexPredicate(t *testing.T) {
	// Setup: relational with AND predicate
	schema := NewSchemaRegistry("stores").
		WithParentAlias("s").
		RegisterRelational("Items", "items", "store_id", "id")

	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.And(
			s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
			s.Equal(s.Field(s.Item(), "Active"), s.Value(true)),
		),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSQL := "EXISTS (SELECT 1 FROM items AS item_1 WHERE item_1.store_id = s.id AND item_1.Price > $1 AND item_1.Active = $2)"
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
		WithParentAlias("s").
		RegisterEmbedded("Tags").
		RegisterRelational("Items", "items", "store_id", "id")

	// Test relational
	ast1 := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(100)),
	)

	visitor1 := NewPostgresqlVisitor(WithSchema(schema))
	_ = ast1.Accept(visitor1)
	sql1, _, _ := visitor1.Result()

	if sql1 != "EXISTS (SELECT 1 FROM items AS item_1 WHERE item_1.store_id = s.id AND item_1.Price > $1)" {
		t.Errorf("unexpected SQL for Items: %s", sql1)
	}

	// Test embedded
	ast2 := s.Wildcard(
		s.Object(s.GlobalScope(), "Tags"),
		s.Equal(s.Field(s.Item(), "Name"), s.Value("sale")),
	)

	visitor2 := NewPostgresqlVisitor(WithSchema(schema))
	_ = ast2.Accept(visitor2)
	sql2, _, _ := visitor2.Result()

	if sql2 != "EXISTS (SELECT 1 FROM unnest(Tags) AS tag_1 WHERE tag_1.Name = $1)" {
		t.Errorf("unexpected SQL for Tags: %s", sql2)
	}
}

func TestSchemaRegistry_NestedRelationalCollections(t *testing.T) {
	// Setup: stores -> categories (separate table) -> items (separate table)
	// stores.id -> categories.store_id
	// categories.id -> items.category_id
	schema := NewSchemaRegistry("stores").
		WithParentAlias("s").
		RegisterRelational("Categories", "categories", "store_id", "id").
		RegisterRelational("Items", "items", "category_id", "id")

	// AST: Store has Category that has Item with Price > 1000
	// spec.Wildcard(
	//   spec.Object(spec.GlobalScope(), "Categories"),
	//   spec.Wildcard(
	//     spec.Object(spec.Item(), "Items"),
	//     spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Value(1000)),
	//   ),
	// )
	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Categories"),
		s.Wildcard(
			s.Object(s.Item(), "Items"),
			s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
		),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expected:
	// EXISTS (SELECT 1 FROM categories AS category_1
	//         WHERE category_1.store_id = s.id
	//         AND EXISTS (SELECT 1 FROM items AS item_2
	//                     WHERE item_2.category_id = category_1.id
	//                     AND item_2.Price > $1))
	expectedSQL := "EXISTS (SELECT 1 FROM categories AS category_1 WHERE category_1.store_id = s.id AND EXISTS (SELECT 1 FROM items AS item_2 WHERE item_2.category_id = category_1.id AND item_2.Price > $1))"
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
		WithParentAlias("s").
		RegisterRelationalComposite("Categories", "categories", []ForeignKeyPair{
			{ChildColumn: "tenant_id", ParentColumn: "tenant_id"},
			{ChildColumn: "store_id", ParentColumn: "id"},
		}).
		RegisterRelationalComposite("Items", "items", []ForeignKeyPair{
			{ChildColumn: "tenant_id", ParentColumn: "tenant_id"},
			{ChildColumn: "category_id", ParentColumn: "id"},
		})

	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Categories"),
		s.Wildcard(
			s.Object(s.Item(), "Items"),
			s.Equal(s.Field(s.Item(), "Active"), s.Value(true)),
		),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql, _, err := visitor.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSQL := "EXISTS (SELECT 1 FROM categories AS category_1 WHERE category_1.tenant_id = s.tenant_id AND category_1.store_id = s.id AND EXISTS (SELECT 1 FROM items AS item_2 WHERE item_2.tenant_id = category_1.tenant_id AND item_2.category_id = category_1.id AND item_2.Active = $1))"
	if sql != expectedSQL {
		t.Errorf("unexpected SQL:\nexpected: %s\ngot:      %s", expectedSQL, sql)
	}
}

func TestSchemaRegistry_CustomAlias(t *testing.T) {
	// Setup: relational with custom alias
	schema := NewSchemaRegistry("stores").
		WithParentAlias("s").
		Register("Items", CollectionMapping{
			Storage: StorageRelational,
			Table:   "store_items",
			ForeignKeys: []ForeignKeyPair{
				{ChildColumn: "store_id", ParentColumn: "id"},
			},
			Alias: "si",
		})

	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
	)

	visitor := NewPostgresqlVisitor(WithSchema(schema))
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	sql, _, err := visitor.Result()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedSQL := "EXISTS (SELECT 1 FROM store_items AS si_1 WHERE si_1.store_id = s.id AND si_1.Price > $1)"
	if sql != expectedSQL {
		t.Errorf("unexpected SQL:\nexpected: %s\ngot:      %s", expectedSQL, sql)
	}
}
