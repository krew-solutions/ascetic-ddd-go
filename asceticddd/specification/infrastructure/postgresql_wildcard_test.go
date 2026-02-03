package specification

import (
	"testing"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

func TestPostgresqlVisitor_Wildcard_Any(t *testing.T) {
	// spec.Any(store.Items, func(item Item) bool { return item.Price > 1000 })
	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.GreaterThan(
			s.Field(s.Item(), "Price"),
			s.Value(1000),
		),
	)

	visitor := NewPostgresqlVisitor()
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	expectedSQL := "EXISTS (SELECT 1 FROM unnest(Items) AS item_1 WHERE item_1.Price > $1)"
	if sql != expectedSQL {
		t.Errorf("Expected SQL:\n  %s\nGot:\n  %s", expectedSQL, sql)
	}

	if len(params) != 1 || params[0] != 1000 {
		t.Errorf("Expected params [1000], got %v", params)
	}
}

func TestPostgresqlVisitor_Wildcard_All(t *testing.T) {
	// spec.All(store.Items, func(item Item) bool { return item.Active })
	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.Field(s.Item(), "Active"),
	)

	visitor := NewPostgresqlVisitor()
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	expectedSQL := "EXISTS (SELECT 1 FROM unnest(Items) AS item_1 WHERE item_1.Active)"
	if sql != expectedSQL {
		t.Errorf("Expected SQL:\n  %s\nGot:\n  %s", expectedSQL, sql)
	}

	if len(params) != 0 {
		t.Errorf("Expected no params, got %v", params)
	}
}

func TestPostgresqlVisitor_Wildcard_ComplexPredicate(t *testing.T) {
	// spec.Any(store.Items, func(item Item) bool {
	//     return item.Price > 1000 && item.Active && item.Stock > 0
	// })
	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.And(
			s.And(
				s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
				s.Field(s.Item(), "Active"),
			),
			s.GreaterThan(s.Field(s.Item(), "Stock"), s.Value(0)),
		),
	)

	visitor := NewPostgresqlVisitor()
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	expectedSQL := "EXISTS (SELECT 1 FROM unnest(Items) AS item_1 WHERE item_1.Price > $1 AND item_1.Active AND item_1.Stock > $2)"
	if sql != expectedSQL {
		t.Errorf("Expected SQL:\n  %s\nGot:\n  %s", expectedSQL, sql)
	}

	if len(params) != 2 || params[0] != 1000 || params[1] != 0 {
		t.Errorf("Expected params [1000, 0], got %v", params)
	}
}

func TestPostgresqlVisitor_Wildcard_WithRootCondition(t *testing.T) {
	// store.Active && spec.Any(store.Items, func(item Item) bool { return item.Price > 1000 })
	ast := s.And(
		s.Field(s.GlobalScope(), "Active"),
		s.Wildcard(
			s.Object(s.GlobalScope(), "Items"),
			s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
		),
	)

	visitor := NewPostgresqlVisitor()
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	expectedSQL := "Active AND EXISTS (SELECT 1 FROM unnest(Items) AS item_1 WHERE item_1.Price > $1)"
	if sql != expectedSQL {
		t.Errorf("Expected SQL:\n  %s\nGot:\n  %s", expectedSQL, sql)
	}

	if len(params) != 1 || params[0] != 1000 {
		t.Errorf("Expected params [1000], got %v", params)
	}
}

func TestPostgresqlVisitor_Wildcard_Negated(t *testing.T) {
	// !spec.Any(store.Items, func(item Item) bool { return item.Price > 5000 })
	ast := s.Not(
		s.Wildcard(
			s.Object(s.GlobalScope(), "Items"),
			s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(5000)),
		),
	)

	visitor := NewPostgresqlVisitor()
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	expectedSQL := "NOT EXISTS (SELECT 1 FROM unnest(Items) AS item_1 WHERE item_1.Price > $1)"
	if sql != expectedSQL {
		t.Errorf("Expected SQL:\n  %s\nGot:\n  %s", expectedSQL, sql)
	}

	if len(params) != 1 || params[0] != 5000 {
		t.Errorf("Expected params [5000], got %v", params)
	}
}

func TestPostgresqlVisitor_Wildcard_Arithmetic(t *testing.T) {
	// spec.Any(store.Items, func(item Item) bool { return item.Price - 100 > 900 })
	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.GreaterThan(
			s.Sub(s.Field(s.Item(), "Price"), s.Value(100)),
			s.Value(900),
		),
	)

	visitor := NewPostgresqlVisitor()
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	expectedSQL := "EXISTS (SELECT 1 FROM unnest(Items) AS item_1 WHERE item_1.Price - $1 > $2)"
	if sql != expectedSQL {
		t.Errorf("Expected SQL:\n  %s\nGot:\n  %s", expectedSQL, sql)
	}

	if len(params) != 2 || params[0] != 100 || params[1] != 900 {
		t.Errorf("Expected params [100, 900], got %v", params)
	}
}

func TestPostgresqlVisitor_Wildcard_MultipleWildcards(t *testing.T) {
	// store.Active && spec.Any(store.Items, ...) && spec.Any(store.Items, ...)
	ast := s.And(
		s.And(
			s.Field(s.GlobalScope(), "Active"),
			s.Wildcard(
				s.Object(s.GlobalScope(), "Items"),
				s.GreaterThan(s.Field(s.Item(), "Price"), s.Value(1000)),
			),
		),
		s.Wildcard(
			s.Object(s.GlobalScope(), "Items"),
			s.LessThan(s.Field(s.Item(), "Price"), s.Value(100)),
		),
	)

	visitor := NewPostgresqlVisitor()
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	expectedSQL := "Active AND EXISTS (SELECT 1 FROM unnest(Items) AS item_1 WHERE item_1.Price > $1) AND EXISTS (SELECT 1 FROM unnest(Items) AS item_2 WHERE item_2.Price < $2)"
	if sql != expectedSQL {
		t.Errorf("Expected SQL:\n  %s\nGot:\n  %s", expectedSQL, sql)
	}

	if len(params) != 2 || params[0] != 1000 || params[1] != 100 {
		t.Errorf("Expected params [1000, 100], got %v", params)
	}
}

func TestPostgresqlVisitor_Wildcard_LessThan(t *testing.T) {
	// spec.Any(store.Items, func(item Item) bool { return item.Price < 100 })
	ast := s.Wildcard(
		s.Object(s.GlobalScope(), "Items"),
		s.LessThan(s.Field(s.Item(), "Price"), s.Value(100)),
	)

	visitor := NewPostgresqlVisitor()
	err := ast.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	expectedSQL := "EXISTS (SELECT 1 FROM unnest(Items) AS item_1 WHERE item_1.Price < $1)"
	if sql != expectedSQL {
		t.Errorf("Expected SQL:\n  %s\nGot:\n  %s", expectedSQL, sql)
	}

	if len(params) != 1 || params[0] != 100 {
		t.Errorf("Expected params [100], got %v", params)
	}
}
