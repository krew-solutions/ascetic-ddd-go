package jsonpath

import (
	"testing"

	spec "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

func TestNativeParser_SimpleComparisonGreaterThan(t *testing.T) {
	s := Parse("$[?(@.age > %d)]")
	user := NewDictContext(map[string]any{"age": 30})

	result, err := s.Match(user, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.Match(user, 35)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_SimpleComparisonLessThan(t *testing.T) {
	s := Parse("$[?(@.age < %d)]")
	user := NewDictContext(map[string]any{"age": 25})

	result, err := s.Match(user, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.Match(user, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_SimpleComparisonEqual(t *testing.T) {
	s := Parse("$[?@.name == %s]")
	user := NewDictContext(map[string]any{"name": "Alice"})

	result, err := s.Match(user, "Alice")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.Match(user, "Bob")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_SimpleComparisonNotEqual(t *testing.T) {
	s := Parse("$[?@.status != %s]")
	user := NewDictContext(map[string]any{"status": "active"})

	result, err := s.Match(user, "inactive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.Match(user, "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_GreaterThanOrEqual(t *testing.T) {
	s := Parse("$[?(@.age >= %d)]")
	user := NewDictContext(map[string]any{"age": 30})

	// Equal
	result, err := s.Match(user, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true for equal, got false")
	}

	// Greater
	result, err = s.Match(user, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true for greater, got false")
	}

	// Less
	result, err = s.Match(user, 35)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false for less, got true")
	}
}

func TestNativeParser_LessThanOrEqual(t *testing.T) {
	s := Parse("$[?(@.age <= %d)]")
	user := NewDictContext(map[string]any{"age": 30})

	// Equal
	result, err := s.Match(user, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true for equal, got false")
	}

	// Less
	result, err = s.Match(user, 35)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true for less, got false")
	}

	// Greater
	result, err = s.Match(user, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false for greater, got true")
	}
}

func TestNativeParser_NamedPlaceholder(t *testing.T) {
	s := Parse("$[?(@.age > %(min_age)d)]")
	user := NewDictContext(map[string]any{"age": 30})

	result, err := s.MatchNamed(user, map[string]any{"min_age": 25})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.MatchNamed(user, map[string]any{"min_age": 35})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_StringPlaceholder(t *testing.T) {
	s := Parse("$[?@.name == %(name)s]")
	user := NewDictContext(map[string]any{"name": "Alice"})

	result, err := s.MatchNamed(user, map[string]any{"name": "Alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.MatchNamed(user, map[string]any{"name": "Bob"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_FloatPlaceholder(t *testing.T) {
	s := Parse("$[?(@.price > %f)]")
	product := NewDictContext(map[string]any{"price": 99.99})

	result, err := s.Match(product, 50.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.Match(product, 100.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_ReuseSpecification(t *testing.T) {
	s := Parse("$[?(@.age > %d)]")
	user := NewDictContext(map[string]any{"age": 30})

	// Multiple calls with different parameters
	result, _ := s.Match(user, 25)
	if !result {
		t.Error("first call: expected true, got false")
	}

	result, _ = s.Match(user, 35)
	if result {
		t.Error("second call: expected false, got true")
	}

	result, _ = s.Match(user, 20)
	if !result {
		t.Error("third call: expected true, got false")
	}
}

func TestNativeParser_WildcardCollection(t *testing.T) {
	s := Parse("$.items[*][?(@.score > %d)]")

	item1 := NewDictContext(map[string]any{"name": "Alice", "score": 90})
	item2 := NewDictContext(map[string]any{"name": "Bob", "score": 75})
	item3 := NewDictContext(map[string]any{"name": "Charlie", "score": 85})

	collection := spec.NewCollectionContext([]spec.Context{item1, item2, item3})
	root := NewDictContext(map[string]any{"items": collection})

	// At least one item has score > 80
	result, err := s.Match(root, 80)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	// No items have score > 95
	result, err = s.Match(root, 95)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_WildcardWithNamedPlaceholder(t *testing.T) {
	s := Parse("$.users[*][?(@.age >= %(min_age)d)]")

	user1 := NewDictContext(map[string]any{"name": "Alice", "age": 30})
	user2 := NewDictContext(map[string]any{"name": "Bob", "age": 25})

	collection := spec.NewCollectionContext([]spec.Context{user1, user2})
	root := NewDictContext(map[string]any{"users": collection})

	result, err := s.MatchNamed(root, map[string]any{"min_age": 28})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.MatchNamed(root, map[string]any{"min_age": 35})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_WildcardStringComparison(t *testing.T) {
	s := Parse("$.users[*][?@.role == %s]")

	user1 := NewDictContext(map[string]any{"name": "Alice", "role": "admin"})
	user2 := NewDictContext(map[string]any{"name": "Bob", "role": "user"})

	collection := spec.NewCollectionContext([]spec.Context{user1, user2})
	root := NewDictContext(map[string]any{"users": collection})

	result, err := s.Match(root, "admin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.Match(root, "guest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_ErrorOnMissingField(t *testing.T) {
	s := Parse("$[?(@.age > %d)]")
	user := NewDictContext(map[string]any{"name": "Alice"}) // No age field

	_, err := s.Match(user, 25)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestNativeParser_LogicalAndOperator(t *testing.T) {
	s := Parse("$[?@.age > %d && @.active == %s]")
	user := NewDictContext(map[string]any{"age": 30, "active": true})

	result, err := s.Match(user, 25, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.Match(user, 35, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}

	result, err = s.Match(user, 25, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_LogicalOrOperator(t *testing.T) {
	s := Parse("$[?@.age < %d || @.age > %d]")
	userYoung := NewDictContext(map[string]any{"age": 15})
	userOld := NewDictContext(map[string]any{"age": 70})
	userMiddle := NewDictContext(map[string]any{"age": 40})

	result, err := s.Match(userYoung, 18, 65)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true for young, got false")
	}

	result, err = s.Match(userOld, 18, 65)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true for old, got false")
	}

	result, err = s.Match(userMiddle, 18, 65)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false for middle, got true")
	}
}

func TestNativeParser_LogicalNotOperator(t *testing.T) {
	s := Parse("$[?!(@.active == %s)]")
	userActive := NewDictContext(map[string]any{"active": true})
	userInactive := NewDictContext(map[string]any{"active": false})

	result, err := s.Match(userActive, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.Match(userActive, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}

	result, err = s.Match(userInactive, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}

	result, err = s.Match(userInactive, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}
}

func TestNativeParser_ComplexLogicalExpression(t *testing.T) {
	s := Parse("$[?(@.age >= %d && @.age <= %d) && @.status == %s]")
	user := NewDictContext(map[string]any{"age": 30, "status": "active"})

	result, err := s.Match(user, 25, 35, "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.Match(user, 25, 35, "inactive")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}

	result, err = s.Match(user, 35, 40, "active")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_RFC9535EqualityOperator(t *testing.T) {
	s := Parse("$[?@.age == %d]")
	user := NewDictContext(map[string]any{"age": 30})

	result, err := s.Match(user, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.Match(user, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_BooleanValues(t *testing.T) {
	s := Parse("$[?@.active == %s]")
	userActive := NewDictContext(map[string]any{"active": true})
	userInactive := NewDictContext(map[string]any{"active": false})

	result, err := s.Match(userActive, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.Match(userActive, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}

	result, err = s.Match(userInactive, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.Match(userInactive, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

// Lexer tests

func TestLexer_TokenizeSimpleExpression(t *testing.T) {
	lexer := NewLexer("$[?(@.age > 25)]")
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify we have tokens
	if len(tokens) == 0 {
		t.Error("expected tokens, got none")
	}

	// Verify token types
	tokenTypes := make(map[TokenType]bool)
	for _, tok := range tokens {
		tokenTypes[tok.Type] = true
	}

	expectedTypes := []TokenType{TokenDollar, TokenAt, TokenIdentifier, TokenGt, TokenNumber}
	for _, expected := range expectedTypes {
		if !tokenTypes[expected] {
			t.Errorf("expected token type %s not found", expected)
		}
	}
}

func TestLexer_TokenizeWithPlaceholder(t *testing.T) {
	lexer := NewLexer("$[?(@.age > %d)]")
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	hasPlaceholder := false
	for _, tok := range tokens {
		if tok.Type == TokenPlaceholder {
			hasPlaceholder = true
			break
		}
	}

	if !hasPlaceholder {
		t.Error("expected PLACEHOLDER token not found")
	}
}

func TestLexer_TokenizeNamedPlaceholder(t *testing.T) {
	lexer := NewLexer("$[?(@.age > %(min_age)d)]")
	tokens, err := lexer.Tokenize()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var placeholderTokens []Token
	for _, tok := range tokens {
		if tok.Type == TokenPlaceholder {
			placeholderTokens = append(placeholderTokens, tok)
		}
	}

	if len(placeholderTokens) != 1 {
		t.Errorf("expected 1 placeholder token, got %d", len(placeholderTokens))
	}

	if placeholderTokens[0].Value != "%(min_age)d" {
		t.Errorf("expected placeholder value '%%(min_age)d', got '%s'", placeholderTokens[0].Value)
	}
}

// Nested wildcards tests

func TestNativeParser_NestedWildcardSimple(t *testing.T) {
	s := Parse("$.categories[*][?@.items[*][?@.price > %f]]")

	// Create nested data structure
	item1 := NewDictContext(map[string]any{"name": "Laptop", "price": 999.0})
	item2 := NewDictContext(map[string]any{"name": "Mouse", "price": 29.0})
	items1 := spec.NewCollectionContext([]spec.Context{item1, item2})
	category1 := NewDictContext(map[string]any{"name": "Electronics", "items": items1})

	item3 := NewDictContext(map[string]any{"name": "Shirt", "price": 49.0})
	item4 := NewDictContext(map[string]any{"name": "Jeans", "price": 89.0})
	items2 := spec.NewCollectionContext([]spec.Context{item3, item4})
	category2 := NewDictContext(map[string]any{"name": "Clothing", "items": items2})

	categories := spec.NewCollectionContext([]spec.Context{category1, category2})
	store := NewDictContext(map[string]any{"categories": categories})

	// Should match: category1 has laptop with price > 500
	result, err := s.Match(store, 500.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	// Should not match: no items with price > 1000
	result, err = s.Match(store, 1000.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_NestedWildcardWithLogicalOperators(t *testing.T) {
	s := Parse("$.categories[*][?@.items[*][?@.price > %f && @.price < %f]]")

	// Create test data
	item1 := NewDictContext(map[string]any{"name": "Laptop", "price": 999.0})
	item2 := NewDictContext(map[string]any{"name": "Mouse", "price": 29.0})
	items1 := spec.NewCollectionContext([]spec.Context{item1, item2})
	category1 := NewDictContext(map[string]any{"name": "Electronics", "items": items1})

	categories := spec.NewCollectionContext([]spec.Context{category1})
	store := NewDictContext(map[string]any{"categories": categories})

	// Should match: laptop price is between 500 and 1000
	result, err := s.Match(store, 500.0, 1000.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	// Should not match: no items between 1000 and 2000
	result, err = s.Match(store, 1000.0, 2000.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_NestedWildcardEmptyCollection(t *testing.T) {
	s := Parse("$.categories[*][?@.items[*][?@.price > %f]]")

	// Category with no items
	items := spec.NewCollectionContext([]spec.Context{})
	category := NewDictContext(map[string]any{"name": "Empty", "items": items})

	categories := spec.NewCollectionContext([]spec.Context{category})
	store := NewDictContext(map[string]any{"categories": categories})

	// Should not match: no items at all
	result, err := s.Match(store, 100.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_NestedWildcardMultipleMatches(t *testing.T) {
	s := Parse("$.categories[*][?@.items[*][?@.price > %f]]")

	// Category 1 with expensive items
	item1 := NewDictContext(map[string]any{"name": "Laptop", "price": 999.0})
	items1 := spec.NewCollectionContext([]spec.Context{item1})
	category1 := NewDictContext(map[string]any{"name": "Electronics", "items": items1})

	// Category 2 with expensive items
	item2 := NewDictContext(map[string]any{"name": "Designer Jeans", "price": 299.0})
	items2 := spec.NewCollectionContext([]spec.Context{item2})
	category2 := NewDictContext(map[string]any{"name": "Clothing", "items": items2})

	categories := spec.NewCollectionContext([]spec.Context{category1, category2})
	store := NewDictContext(map[string]any{"categories": categories})

	// Should match: both categories have items > 200
	result, err := s.Match(store, 200.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}
}

func TestNativeParser_NestedWildcardWithNamedPlaceholder(t *testing.T) {
	s := Parse("$.categories[*][?@.items[*][?@.price > %(min_price)f]]")

	// Create test data
	item1 := NewDictContext(map[string]any{"name": "Laptop", "price": 999.0})
	items := spec.NewCollectionContext([]spec.Context{item1})
	category := NewDictContext(map[string]any{"name": "Electronics", "items": items})

	categories := spec.NewCollectionContext([]spec.Context{category})
	store := NewDictContext(map[string]any{"categories": categories})

	// Should match with named parameter
	result, err := s.MatchNamed(store, map[string]any{"min_price": 500.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	result, err = s.MatchNamed(store, map[string]any{"min_price": 1000.0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

// Nested paths tests

func TestNativeParser_NestedPathSimple(t *testing.T) {
	s := Parse("$.store.products[*][?@.price > %f]")

	// Create nested structure
	product1 := NewDictContext(map[string]any{"name": "Laptop", "price": 999.0})
	product2 := NewDictContext(map[string]any{"name": "Mouse", "price": 29.0})
	products := spec.NewCollectionContext([]spec.Context{product1, product2})

	data := NewNestedDictContext(map[string]any{
		"store": map[string]any{
			"name":     "MyStore",
			"products": products,
		},
	})

	// Should match: laptop price > 500
	result, err := s.Match(data, 500.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	// Should not match: no products > 1000
	result, err = s.Match(data, 1000.0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_NestedPathDeep(t *testing.T) {
	s := Parse("$.company.department.team.members[*][?@.age > %d]")

	member1 := NewDictContext(map[string]any{"name": "Alice", "age": 30})
	member2 := NewDictContext(map[string]any{"name": "Bob", "age": 25})
	members := spec.NewCollectionContext([]spec.Context{member1, member2})

	data := NewNestedDictContext(map[string]any{
		"company": map[string]any{
			"name": "TechCorp",
			"department": map[string]any{
				"name": "Engineering",
				"team": map[string]any{
					"name":    "Backend",
					"members": members,
				},
			},
		},
	})

	// Should match: Alice age > 28
	result, err := s.Match(data, 28)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	// Should not match: no members > 35
	result, err = s.Match(data, 35)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_NestedPathInFilter(t *testing.T) {
	s := Parse("$[?@.user.profile.age > %d]")

	data := NewNestedDictContext(map[string]any{
		"user": map[string]any{
			"name": "Alice",
			"profile": map[string]any{
				"age":  30,
				"city": "NYC",
			},
		},
	})

	// Should match: age > 25
	result, err := s.Match(data, 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	// Should not match: age not > 35
	result, err = s.Match(data, 35)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_NestedPathWithLogicalOperators(t *testing.T) {
	s := Parse("$.store.products[*][?@.price > %f && @.stock > %d]")

	product1 := NewDictContext(map[string]any{"name": "Laptop", "price": 999.0, "stock": 5})
	product2 := NewDictContext(map[string]any{"name": "Mouse", "price": 29.0, "stock": 100})
	product3 := NewDictContext(map[string]any{"name": "Monitor", "price": 599.0, "stock": 10})
	products := spec.NewCollectionContext([]spec.Context{product1, product2, product3})

	data := NewNestedDictContext(map[string]any{
		"store": map[string]any{
			"products": products,
		},
	})

	// Should match: Monitor (price > 500 && stock > 5)
	result, err := s.Match(data, 500.0, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	// Should not match: no products with price > 1000
	result, err = s.Match(data, 1000.0, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_NestedPathWithNamedPlaceholder(t *testing.T) {
	s := Parse("$.warehouse.items[*][?@.quantity < %(min_qty)d]")

	item1 := NewDictContext(map[string]any{"name": "Widget", "quantity": 5})
	item2 := NewDictContext(map[string]any{"name": "Gadget", "quantity": 50})
	items := spec.NewCollectionContext([]spec.Context{item1, item2})

	data := NewNestedDictContext(map[string]any{
		"warehouse": map[string]any{
			"location": "East",
			"items":    items,
		},
	})

	// Should match: Widget quantity < 10
	result, err := s.MatchNamed(data, map[string]any{"min_qty": 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	// Should not match: no items < 3
	result, err = s.MatchNamed(data, map[string]any{"min_qty": 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}

func TestNativeParser_DeeplyNestedFilterField(t *testing.T) {
	s := Parse("$[?@.company.department.manager.level > %d]")

	data := NewNestedDictContext(map[string]any{
		"company": map[string]any{
			"name": "TechCorp",
			"department": map[string]any{
				"name": "Engineering",
				"manager": map[string]any{
					"name":  "Alice",
					"level": 5,
				},
			},
		},
	})

	// Should match: manager level > 3
	result, err := s.Match(data, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result {
		t.Error("expected true, got false")
	}

	// Should not match: manager level not > 10
	result, err = s.Match(data, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result {
		t.Error("expected false, got true")
	}
}
