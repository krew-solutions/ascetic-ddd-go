package specification

import (
	"testing"
)

// TestOperators tests all operators that were not covered by basic tests

func TestNotEqualOperator(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx)

	expression := NotEqual(Value(5), Value(10))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true (5 != 10), got %v", result)
	}
}

func TestNotEqualOperatorFalse(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx)

	expression := NotEqual(Value(5), Value(5))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != false {
		t.Errorf("Expected false (5 != 5), got %v", result)
	}
}

func TestLessThanOperator(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx)

	expression := LessThan(Value(5), Value(10))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true (5 < 10), got %v", result)
	}
}

func TestLessThanOperatorFalse(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx)

	expression := LessThan(Value(10), Value(5))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != false {
		t.Errorf("Expected false (10 < 5), got %v", result)
	}
}

func TestLessThanEqualOperator(t *testing.T) {
	ctx := make(testContext)

	tests := []struct {
		name     string
		left     int
		right    int
		expected bool
	}{
		{"5 <= 10", 5, 10, true},
		{"10 <= 10", 10, 10, true},
		{"15 <= 10", 15, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visitor := NewEvaluateVisitor(ctx)
			expression := LessThanEqual(Value(tt.left), Value(tt.right))
			err := expression.Accept(visitor)
			if err != nil {
				t.Fatalf("Accept failed: %v", err)
			}

			result, err := visitor.Result()
			if err != nil {
				t.Fatalf("Result failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestGreaterThanEqualOperator(t *testing.T) {
	ctx := make(testContext)

	tests := []struct {
		name     string
		left     int
		right    int
		expected bool
	}{
		{"10 >= 5", 10, 5, true},
		{"10 >= 10", 10, 10, true},
		{"5 >= 10", 5, 10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visitor := NewEvaluateVisitor(ctx)
			expression := GreaterThanEqual(Value(tt.left), Value(tt.right))
			err := expression.Accept(visitor)
			if err != nil {
				t.Fatalf("Accept failed: %v", err)
			}

			result, err := visitor.Result()
			if err != nil {
				t.Fatalf("Result failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestOrOperator(t *testing.T) {
	ctx := make(testContext)

	tests := []struct {
		name     string
		left     bool
		right    bool
		expected bool
	}{
		{"true OR true", true, true, true},
		{"true OR false", true, false, true},
		{"false OR true", false, true, true},
		{"false OR false", false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			visitor := NewEvaluateVisitor(ctx)
			expression := Or(Value(tt.left), Value(tt.right))
			err := expression.Accept(visitor)
			if err != nil {
				t.Fatalf("Accept failed: %v", err)
			}

			result, err := visitor.Result()
			if err != nil {
				t.Fatalf("Result failed: %v", err)
			}

			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsOperator(t *testing.T) {
	// IS operator is for SQL (IS TRUE, IS FALSE, IS NULL)
	// For now just test node creation
	expr := Is(Value(true), Value(true))
	if expr.Operator() != OperatorIs {
		t.Errorf("Expected IS operator, got %s", expr.Operator())
	}
}

// TestArithmeticOperators tests arithmetic operators
// Note: These don't yield boolean, so we wrap them in comparisons

func TestAddOperator(t *testing.T) {
	ctx := make(testContext)
	ctx["a"] = 5
	ctx["b"] = 3

	visitor := NewEvaluateVisitor(ctx)

	// (a + b) > 7
	expression := GreaterThan(
		Add(Field(GlobalScope(), "a"), Field(GlobalScope(), "b")),
		Value(7),
	)

	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true (5 + 3 > 7), got %v", result)
	}
}

func TestSubOperator(t *testing.T) {
	ctx := make(testContext)
	ctx["a"] = 10
	ctx["b"] = 3

	visitor := NewEvaluateVisitor(ctx)

	// (a - b) > 5
	expression := GreaterThan(
		Sub(Field(GlobalScope(), "a"), Field(GlobalScope(), "b")),
		Value(5),
	)

	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true (10 - 3 > 5), got %v", result)
	}
}

func TestMulOperator(t *testing.T) {
	ctx := make(testContext)
	ctx["a"] = 5
	ctx["b"] = 3

	visitor := NewEvaluateVisitor(ctx)

	// (a * b) > 10
	expression := GreaterThan(
		Mul(Field(GlobalScope(), "a"), Field(GlobalScope(), "b")),
		Value(10),
	)

	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true (5 * 3 > 10), got %v", result)
	}
}

func TestDivOperator(t *testing.T) {
	ctx := make(testContext)
	ctx["a"] = 10
	ctx["b"] = 2

	visitor := NewEvaluateVisitor(ctx)

	// (a / b) == 5
	expression := Equal(
		Div(Field(GlobalScope(), "a"), Field(GlobalScope(), "b")),
		Value(5),
	)

	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true (10 / 2 == 5), got %v", result)
	}
}

func TestModOperator(t *testing.T) {
	ctx := make(testContext)
	ctx["a"] = 10
	ctx["b"] = 3

	visitor := NewEvaluateVisitor(ctx)

	// (a % b) == 1
	expression := Equal(
		Mod(Field(GlobalScope(), "a"), Field(GlobalScope(), "b")),
		Value(1),
	)

	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true (10 %% 3 == 1), got %v", result)
	}
}

// TestBitwiseOperators tests bitwise operators

func TestLeftShiftOperator(t *testing.T) {
	// Just test node creation for now
	// Bitwise operators are not yet supported in EvaluateVisitor
	expr := LeftShift(Value(2), Value(3))
	if expr.Operator() != OperatorLshift {
		t.Errorf("Expected LSHIFT operator, got %s", expr.Operator())
	}
}

func TestRightShiftOperator(t *testing.T) {
	// Just test node creation for now
	expr := RightShift(Value(16), Value(2))
	if expr.Operator() != OperatorRshift {
		t.Errorf("Expected RSHIFT operator, got %s", expr.Operator())
	}
}

// TestPostfixOperators tests IS NULL / IS NOT NULL

func TestIsNullOperator(t *testing.T) {
	ctx := make(testContext)
	ctx["value"] = nil

	visitor := NewEvaluateVisitor(ctx)

	expression := IsNull(Field(GlobalScope(), "value"))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true (value IS NULL), got %v", result)
	}
}

func TestIsNullOperatorFalse(t *testing.T) {
	ctx := make(testContext)
	ctx["value"] = 42

	visitor := NewEvaluateVisitor(ctx)

	expression := IsNull(Field(GlobalScope(), "value"))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != false {
		t.Errorf("Expected false (value IS NOT NULL), got %v", result)
	}
}

func TestIsNotNullOperator(t *testing.T) {
	ctx := make(testContext)
	ctx["value"] = 42

	visitor := NewEvaluateVisitor(ctx)

	expression := IsNotNull(Field(GlobalScope(), "value"))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true (value IS NOT NULL), got %v", result)
	}
}

func TestIsNotNullOperatorFalse(t *testing.T) {
	ctx := make(testContext)
	ctx["value"] = nil

	visitor := NewEvaluateVisitor(ctx)

	expression := IsNotNull(Field(GlobalScope(), "value"))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != false {
		t.Errorf("Expected false (value IS NULL), got %v", result)
	}
}

// TestExtractFieldPath tests field path extraction

func TestExtractFieldPath(t *testing.T) {
	tests := []struct {
		name     string
		field    FieldNode
		expected []string
	}{
		{
			name:     "Simple field",
			field:    Field(GlobalScope(), "name"),
			expected: []string{"name"},
		},
		{
			name:     "Nested field",
			field:    Field(Object(GlobalScope(), "user"), "name"),
			expected: []string{"user", "name"},
		},
		{
			name:     "Deep nested field",
			field:    Field(Object(Object(GlobalScope(), "user"), "profile"), "age"),
			expected: []string{"user", "profile", "age"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := ExtractFieldPath(tt.field)
			if len(path) != len(tt.expected) {
				t.Fatalf("Expected path length %d, got %d", len(tt.expected), len(path))
			}
			for i, p := range path {
				if p != tt.expected[i] {
					t.Errorf("Expected path[%d]=%s, got %s", i, tt.expected[i], p)
				}
			}
		})
	}
}
