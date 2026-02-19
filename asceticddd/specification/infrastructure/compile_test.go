package specification

import (
	"strings"
	"testing"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

// TestCompileToSQL tests the public CompileToSQL function

func TestCompileToSQLSimple(t *testing.T) {
	// Simple expression: age >= 18
	expr := s.GreaterThanEqual(
		s.Field(s.GlobalScope(), "age"),
		s.Value(18),
	)

	sql, params, err := CompileToSQL(expr)
	if err != nil {
		t.Fatalf("CompileToSQL failed: %v", err)
	}

	expected := "age >= $1"
	if sql != expected {
		t.Errorf("Expected SQL: %s, got: %s", expected, sql)
	}

	if len(params) != 1 || params[0] != 18 {
		t.Errorf("Expected params [18], got %v", params)
	}
}

func TestCompileToSQLComplex(t *testing.T) {
	// Complex expression: (active = true) AND (age >= 18) OR (premium = true)
	expr := s.Or(
		s.And(
			s.Equal(s.Field(s.GlobalScope(), "active"), s.Value(true)),
			s.GreaterThanEqual(s.Field(s.GlobalScope(), "age"), s.Value(18)),
		),
		s.Equal(s.Field(s.GlobalScope(), "premium"), s.Value(true)),
	)

	sql, params, err := CompileToSQL(expr)
	if err != nil {
		t.Fatalf("CompileToSQL failed: %v", err)
	}

	// Check that SQL contains expected operators
	if !strings.Contains(sql, "AND") {
		t.Errorf("Expected SQL to contain AND, got: %s", sql)
	}
	if !strings.Contains(sql, "OR") {
		t.Errorf("Expected SQL to contain OR, got: %s", sql)
	}

	if len(params) != 3 {
		t.Errorf("Expected 3 params, got %v", params)
	}
}

func TestCompileToSQLWithWildcard(t *testing.T) {
	// Wildcard expression: EXISTS (SELECT 1 FROM unnest(items) AS item WHERE item.price > 1000)
	expr := s.Wildcard(
		s.Object(s.GlobalScope(), "items"),
		s.GreaterThan(s.Field(s.Item(), "price"), s.Value(1000)),
	)

	sql, params, err := CompileToSQL(expr)
	if err != nil {
		t.Fatalf("CompileToSQL failed: %v", err)
	}

	if !strings.Contains(sql, "EXISTS") {
		t.Errorf("Expected SQL to contain EXISTS, got: %s", sql)
	}
	if !strings.Contains(sql, "unnest") {
		t.Errorf("Expected SQL to contain unnest, got: %s", sql)
	}

	if len(params) != 1 || params[0] != 1000 {
		t.Errorf("Expected params [1000], got %v", params)
	}
}

func TestCompileToSQLNestedObject(t *testing.T) {
	// Nested object: user.profile.age >= 18
	gs := s.GlobalScope()
	user := s.Object(gs, "user")
	profile := s.Object(user, "profile")

	expr := s.GreaterThanEqual(
		s.Field(profile, "age"),
		s.Value(18),
	)

	sql, params, err := CompileToSQL(expr)
	if err != nil {
		t.Fatalf("CompileToSQL failed: %v", err)
	}

	expected := "user.profile.age >= $1"
	if sql != expected {
		t.Errorf("Expected SQL: %s, got: %s", expected, sql)
	}

	if len(params) != 1 || params[0] != 18 {
		t.Errorf("Expected params [18], got %v", params)
	}
}

func TestCompileToSQLArithmetic(t *testing.T) {
	// Arithmetic: (price - discount) > 100
	expr := s.GreaterThan(
		s.Sub(
			s.Field(s.GlobalScope(), "price"),
			s.Field(s.GlobalScope(), "discount"),
		),
		s.Value(100),
	)

	sql, params, err := CompileToSQL(expr)
	if err != nil {
		t.Fatalf("CompileToSQL failed: %v", err)
	}

	if !strings.Contains(sql, "-") {
		t.Errorf("Expected SQL to contain -, got: %s", sql)
	}
	if !strings.Contains(sql, "price") {
		t.Errorf("Expected SQL to contain price, got: %s", sql)
	}
	if !strings.Contains(sql, "discount") {
		t.Errorf("Expected SQL to contain discount, got: %s", sql)
	}

	if len(params) != 1 || params[0] != 100 {
		t.Errorf("Expected params [100], got %v", params)
	}
}

func TestCompileToSQLNegation(t *testing.T) {
	// Negation: NOT (age < 18)
	expr := s.Not(
		s.LessThan(
			s.Field(s.GlobalScope(), "age"),
			s.Value(18),
		),
	)

	sql, params, err := CompileToSQL(expr)
	if err != nil {
		t.Fatalf("CompileToSQL failed: %v", err)
	}

	if !strings.Contains(sql, "NOT") {
		t.Errorf("Expected SQL to contain NOT, got: %s", sql)
	}

	if len(params) != 1 || params[0] != 18 {
		t.Errorf("Expected params [18], got %v", params)
	}
}

// TestPostfixOperators tests IS NULL and IS NOT NULL SQL generation

func TestPostgresqlVisitorIsNull(t *testing.T) {
	expr := s.IsNull(s.Field(s.GlobalScope(), "deleted_at"))

	visitor := NewPostgresqlVisitor()
	err := expr.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	expected := "deleted_at IS NULL"
	if sql != expected {
		t.Errorf("Expected SQL: %s, got: %s", expected, sql)
	}

	if len(params) != 0 {
		t.Errorf("Expected no params, got %v", params)
	}
}

func TestPostgresqlVisitorIsNotNull(t *testing.T) {
	expr := s.IsNotNull(s.Field(s.GlobalScope(), "created_at"))

	visitor := NewPostgresqlVisitor()
	err := expr.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	sql, params, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	expected := "created_at IS NOT NULL"
	if sql != expected {
		t.Errorf("Expected SQL: %s, got: %s", expected, sql)
	}

	if len(params) != 0 {
		t.Errorf("Expected no params, got %v", params)
	}
}

func TestPostgresqlVisitorIsNullWithAnd(t *testing.T) {
	// active = true AND deleted_at IS NULL
	expr := s.And(
		s.Equal(s.Field(s.GlobalScope(), "active"), s.Value(true)),
		s.IsNull(s.Field(s.GlobalScope(), "deleted_at")),
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

	if !strings.Contains(sql, "IS NULL") {
		t.Errorf("Expected SQL to contain IS NULL, got: %s", sql)
	}
	if !strings.Contains(sql, "AND") {
		t.Errorf("Expected SQL to contain AND, got: %s", sql)
	}

	if len(params) != 1 {
		t.Errorf("Expected 1 param, got %v", params)
	}
}

func TestCompileToSQLIsNull(t *testing.T) {
	// Test CompileToSQL with IS NULL
	expr := s.And(
		s.Equal(s.Field(s.GlobalScope(), "active"), s.Value(true)),
		s.IsNull(s.Field(s.GlobalScope(), "deleted_at")),
	)

	sql, params, err := CompileToSQL(expr)
	if err != nil {
		t.Fatalf("CompileToSQL failed: %v", err)
	}

	if !strings.Contains(sql, "IS NULL") {
		t.Errorf("Expected SQL to contain IS NULL, got: %s", sql)
	}

	if len(params) != 1 {
		t.Errorf("Expected 1 param, got %v", params)
	}
}

// TestCompositeExpressionNotEqual tests NotEqual for composite expressions

func TestCompositeExpressionNotEqual(t *testing.T) {
	left := CompositeExpression(s.Value(1), s.Value(2))
	right := CompositeExpression(s.Value(3), s.Value(4))

	result, err := left.NotEqual(right)
	if err != nil {
		t.Fatalf("NotEqual failed: %v", err)
	}

	// Should create: NOT((1 = 3) AND (2 = 4))
	if result == nil {
		t.Error("Expected non-nil result")
	}

	// Verify it's a NOT node wrapping an AND
	notNode, ok := result.(s.PrefixNode)
	if !ok {
		t.Fatalf("Expected PrefixNode (NOT), got %T", result)
	}

	if notNode.Operator() != operators.OperatorNot {
		t.Errorf("Expected NOT operator, got %s", notNode.Operator())
	}
}

func TestCompositeExpressionNotEqualComplex(t *testing.T) {
	// Nested composite: ((a, b), c) != ((d, e), f)
	left := CompositeExpression(
		CompositeExpression(s.Value(1), s.Value(2)),
		s.Value(3),
	)
	right := CompositeExpression(
		CompositeExpression(s.Value(4), s.Value(5)),
		s.Value(6),
	)

	result, err := left.NotEqual(right)
	if err != nil {
		t.Fatalf("NotEqual failed: %v", err)
	}

	if result == nil {
		t.Error("Expected non-nil result")
	}

	// Should be NOT(complex expression)
	notNode, ok := result.(s.PrefixNode)
	if !ok {
		t.Fatalf("Expected PrefixNode (NOT), got %T", result)
	}

	if notNode.Operator() != operators.OperatorNot {
		t.Errorf("Expected NOT operator, got %s", notNode.Operator())
	}
}

func TestCompositeExpressionNotEqualDifferentLength(t *testing.T) {
	left := CompositeExpression(s.Value(1), s.Value(2))
	right := CompositeExpression(s.Value(3))

	_, err := left.NotEqual(right)
	if err == nil {
		t.Error("Expected error for different length composite expressions")
	}
}
