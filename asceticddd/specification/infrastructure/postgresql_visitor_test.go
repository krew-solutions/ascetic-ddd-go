package specification

import (
	"strings"
	"testing"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

// TestPostgresqlVisitorBasic tests basic PostgreSQL visitor functionality

func TestSimpleFieldRendering(t *testing.T) {
	obj := s.Object(s.GlobalScope(), "users")
	expr := s.Field(obj, "name")

	visitor := NewPostgresqlVisitor()
	err := expr.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if sql != "users.name" {
		t.Errorf("Expected 'users.name', got %s", sql)
	}

	if len(params) != 0 {
		t.Errorf("Expected no params, got %v", params)
	}
}

func TestValueParameterization(t *testing.T) {
	expr := s.Value(42)

	visitor := NewPostgresqlVisitor()
	err := expr.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if sql != "$1" {
		t.Errorf("Expected '$1', got %s", sql)
	}

	if len(params) != 1 || params[0] != 42 {
		t.Errorf("Expected params [42], got %v", params)
	}
}

func TestInfixOperatorAnd(t *testing.T) {
	obj := s.Object(s.GlobalScope(), "t")
	// Create: a = 1 AND b = 2
	expr := s.And(
		s.Equal(s.Field(obj, "a"), s.Value(1)),
		s.Equal(s.Field(obj, "b"), s.Value(2)),
	)

	visitor := NewPostgresqlVisitor()
	err := expr.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	// Check SQL contains expected parts
	if !strings.Contains(sql, "AND") {
		t.Errorf("Expected SQL to contain 'AND', got %s", sql)
	}
	if !strings.Contains(sql, "t.a") {
		t.Errorf("Expected SQL to contain 't.a', got %s", sql)
	}
	if !strings.Contains(sql, "t.b") {
		t.Errorf("Expected SQL to contain 't.b', got %s", sql)
	}

	if len(params) != 2 || params[0] != 1 || params[1] != 2 {
		t.Errorf("Expected params [1, 2], got %v", params)
	}
}

func TestPrefixNotOperator(t *testing.T) {
	obj := s.Object(s.GlobalScope(), "t")
	expr := s.Not(s.Equal(s.Field(obj, "active"), s.Value(true)))

	visitor := NewPostgresqlVisitor()
	err := expr.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if !strings.Contains(sql, "NOT") {
		t.Errorf("Expected SQL to contain 'NOT', got %s", sql)
	}
	if !strings.Contains(sql, "t.active") {
		t.Errorf("Expected SQL to contain 't.active', got %s", sql)
	}

	if len(params) != 1 || params[0] != true {
		t.Errorf("Expected params [true], got %v", params)
	}
}

func TestOrOperator(t *testing.T) {
	obj := s.Object(s.GlobalScope(), "t")
	expr := s.Or(
		s.Equal(s.Field(obj, "a"), s.Value(1)),
		s.Equal(s.Field(obj, "b"), s.Value(2)),
	)

	visitor := NewPostgresqlVisitor()
	err := expr.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if !strings.Contains(sql, "OR") {
		t.Errorf("Expected SQL to contain 'OR', got %s", sql)
	}

	if len(params) != 2 {
		t.Errorf("Expected 2 params, got %v", params)
	}
}

func TestLessThanOperator(t *testing.T) {
	obj := s.Object(s.GlobalScope(), "t")
	expr := s.LessThan(s.Field(obj, "age"), s.Value(30))

	visitor := NewPostgresqlVisitor()
	err := expr.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if !strings.Contains(sql, "<") {
		t.Errorf("Expected SQL to contain '<', got %s", sql)
	}
	if !strings.Contains(sql, "t.age") {
		t.Errorf("Expected SQL to contain 't.age', got %s", sql)
	}

	if len(params) != 1 || params[0] != 30 {
		t.Errorf("Expected params [30], got %v", params)
	}
}

func TestArithmeticOperators(t *testing.T) {
	obj := s.Object(s.GlobalScope(), "t")

	tests := []struct {
		name     string
		expr     s.Visitable
		operator string
	}{
		{"Add", s.Add(s.Field(obj, "a"), s.Value(10)), "+"},
		{"Sub", s.Sub(s.Field(obj, "a"), s.Value(10)), "-"},
		{"Mul", s.Mul(s.Field(obj, "a"), s.Value(10)), "*"},
		{"Div", s.Div(s.Field(obj, "a"), s.Value(10)), "/"},
		{"Mod", s.Mod(s.Field(obj, "a"), s.Value(10)), "%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Wrap in comparison to make it boolean
			expr := s.GreaterThan(tt.expr, s.Value(0))

			visitor := NewPostgresqlVisitor()
			err := expr.Accept(visitor)
			if err != nil {
				t.Fatalf("Accept failed: %v", err)
			}

			sql, _, err := visitor.Result()
			if err != nil {
				t.Fatalf("Result failed: %v", err)
			}

			if !strings.Contains(sql, tt.operator) {
				t.Errorf("Expected SQL to contain '%s', got %s", tt.operator, sql)
			}
		})
	}
}

func TestNestedFields(t *testing.T) {
	// user.profile.age
	gs := s.GlobalScope()
	user := s.Object(gs, "user")
	profile := s.Object(user, "profile")
	expr := s.Field(profile, "age")

	visitor := NewPostgresqlVisitor()
	err := expr.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	expected := "user.profile.age"
	if sql != expected {
		t.Errorf("Expected '%s', got '%s'", expected, sql)
	}

	if len(params) != 0 {
		t.Errorf("Expected no params, got %v", params)
	}
}

func TestComplexExpression(t *testing.T) {
	// (age >= 18) AND (active = true) OR (premium = true)
	gs := s.GlobalScope()
	expr := s.Or(
		s.And(
			s.GreaterThanEqual(s.Field(gs, "age"), s.Value(18)),
			s.Equal(s.Field(gs, "active"), s.Value(true)),
		),
		s.Equal(s.Field(gs, "premium"), s.Value(true)),
	)

	visitor := NewPostgresqlVisitor()
	err := expr.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	// Check all operators are present
	for _, op := range []string{"AND", "OR", ">="} {
		if !strings.Contains(sql, op) {
			t.Errorf("Expected SQL to contain '%s', got %s", op, sql)
		}
	}

	if len(params) != 3 {
		t.Errorf("Expected 3 params, got %v", params)
	}
}
