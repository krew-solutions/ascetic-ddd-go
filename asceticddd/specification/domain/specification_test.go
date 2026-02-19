package specification

import (
	"testing"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

// TestNodes tests AST node creation
func TestValueNode(t *testing.T) {
	valNode := Value(42)
	if valNode.Value() != 42 {
		t.Errorf("Expected value 42, got %v", valNode.Value())
	}
}

func TestNotNode(t *testing.T) {
	valNode := Value(true)
	notNode := Not(valNode)
	if notNode.Operand() != valNode {
		t.Error("NOT node operand mismatch")
	}
}

func TestEqualNode(t *testing.T) {
	left := Value(5)
	right := Value(5)
	eqNode := Equal(left, right)

	if eqNode.Left() != left {
		t.Error("Equal node left operand mismatch")
	}
	if eqNode.Right() != right {
		t.Error("Equal node right operand mismatch")
	}
}

func TestAndNode(t *testing.T) {
	left := Value(true)
	right := Value(true)
	andNode := And(left, right)

	if andNode.Left() != left {
		t.Error("AND node left operand mismatch")
	}
	if andNode.Right() != right {
		t.Error("AND node right operand mismatch")
	}
}

func TestAndNodeMultiple(t *testing.T) {
	a := Value(true)
	b := Value(true)
	c := Value(true)

	// Should create nested structure: (a AND b) AND c
	andNode := And(And(a, b), c)

	// Just verify it was created
	if andNode.Operator() != operators.OperatorAnd {
		t.Error("Expected AND operator")
	}
}

func TestFieldNode(t *testing.T) {
	gs := GlobalScope()
	obj := Object(gs, "user")
	fieldNode := Field(obj, "name")

	if fieldNode.Name() != "name" {
		t.Errorf("Expected field name 'name', got %s", fieldNode.Name())
	}
	if fieldNode.Object() != obj {
		t.Error("Field object mismatch")
	}
}

// TestEvaluateVisitor tests evaluation visitor

type testContext map[string]interface{}

func (c testContext) Get(key string) (interface{}, error) {
	val, ok := c[key]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return val, nil
}

func TestSimpleValue(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx, operators.NewDefaultRegistry())

	valNode := Value(true)
	err := valNode.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestNotOperator(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx, operators.NewDefaultRegistry())

	expression := Not(Value(true))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != false {
		t.Errorf("Expected false, got %v", result)
	}
}

func TestAndOperator(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx, operators.NewDefaultRegistry())

	expression := And(Value(true), Value(true))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestAndOperatorFalse(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx, operators.NewDefaultRegistry())

	expression := And(Value(true), Value(false))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != false {
		t.Errorf("Expected false, got %v", result)
	}
}

func TestEqualOperator(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx, operators.NewDefaultRegistry())

	expression := Equal(Value(5), Value(5))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestEqualOperatorNotEqual(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx, operators.NewDefaultRegistry())

	expression := Equal(Value(5), Value(10))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != false {
		t.Errorf("Expected false, got %v", result)
	}
}

func TestGreaterThanOperator(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx, operators.NewDefaultRegistry())

	expression := GreaterThan(Value(10), Value(5))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestGreaterThanOperatorFalse(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx, operators.NewDefaultRegistry())

	expression := GreaterThan(Value(5), Value(10))
	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != false {
		t.Errorf("Expected false, got %v", result)
	}
}

func TestFieldAccess(t *testing.T) {
	ctx := testContext{"age": 25}
	visitor := NewEvaluateVisitor(ctx, operators.NewDefaultRegistry())

	fieldNode := Field(GlobalScope(), "age")
	err := fieldNode.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result := visitor.CurrentValue()
	if result != 25 {
		t.Errorf("Expected 25, got %v", result)
	}
}

func TestObjectNavigation(t *testing.T) {
	userCtx := testContext{"name": "Alice"}
	rootCtx := testContext{"user": userCtx}
	visitor := NewEvaluateVisitor(rootCtx, operators.NewDefaultRegistry())

	obj := Object(GlobalScope(), "user")
	fieldNode := Field(obj, "name")
	err := fieldNode.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result := visitor.CurrentValue()
	if result != "Alice" {
		t.Errorf("Expected 'Alice', got %v", result)
	}
}

func TestComplexExpression(t *testing.T) {
	ctx := testContext{"age": 25, "active": true}
	visitor := NewEvaluateVisitor(ctx, operators.NewDefaultRegistry())

	// (age > 18) AND active
	ageField := Field(GlobalScope(), "age")
	ageCheck := GreaterThan(ageField, Value(18))
	activeField := Field(GlobalScope(), "active")
	expression := And(ageCheck, activeField)

	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	if result != true {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestCollectionWildcard(t *testing.T) {
	item1 := testContext{"score": 90}
	item2 := testContext{"score": 75}
	item3 := testContext{"score": 85}

	collection := NewCollectionContext([]Context{item1, item2, item3})
	rootCtx := testContext{"items": collection}

	visitor := NewEvaluateVisitor(rootCtx, operators.NewDefaultRegistry())

	// items[*].score > 80
	itemsObj := Object(GlobalScope(), "items")
	scoreField := Field(Item(), "score")
	predicate := GreaterThan(scoreField, Value(80))
	wildcardNode := Wildcard(itemsObj, predicate)

	err := wildcardNode.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	// Should be true because at least one item has score > 80
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}
}

func TestCollectionAllFalse(t *testing.T) {
	item1 := testContext{"score": 70}
	item2 := testContext{"score": 75}

	collection := NewCollectionContext([]Context{item1, item2})
	rootCtx := testContext{"items": collection}

	visitor := NewEvaluateVisitor(rootCtx, operators.NewDefaultRegistry())

	// items[*].score > 80
	itemsObj := Object(GlobalScope(), "items")
	scoreField := Field(Item(), "score")
	predicate := GreaterThan(scoreField, Value(80))
	wildcardNode := Wildcard(itemsObj, predicate)

	err := wildcardNode.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	// Should be false because no items have score > 80
	if result != false {
		t.Errorf("Expected false, got %v", result)
	}
}

// TestErrorHandling tests error handling

func TestMissingKey(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx, operators.NewDefaultRegistry())

	fieldNode := Field(GlobalScope(), "nonexistent")

	err := fieldNode.Accept(visitor)
	if err == nil {
		t.Error("Expected error for missing key, got nil")
	}
}

func TestTypeCheckingInComparison(t *testing.T) {
	ctx := make(testContext)
	visitor := NewEvaluateVisitor(ctx, operators.NewDefaultRegistry())

	// Strings should work with Equal
	expression := Equal(Value("hello"), Value("world"))

	err := expression.Accept(visitor)
	if err != nil {
		t.Fatalf("Accept failed: %v", err)
	}

	result, err := visitor.Result()
	if err != nil {
		t.Fatalf("Result failed: %v", err)
	}

	// Result should be false since strings are different
	if result != false {
		t.Errorf("Expected false, got %v", result)
	}
}
