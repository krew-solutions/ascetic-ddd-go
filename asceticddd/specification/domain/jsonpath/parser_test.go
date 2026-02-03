package jsonpath

import (
	"sync"
	"testing"

	spec "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

func TestNativeParser_SimpleComparisonGreaterThan(t *testing.T) {
	s := MustParse("$[?(@.age > %d)]")
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
	s := MustParse("$[?(@.age < %d)]")
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
	s := MustParse("$[?@.name == %s]")
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
	s := MustParse("$[?@.status != %s]")
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
	s := MustParse("$[?(@.age >= %d)]")
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
	s := MustParse("$[?(@.age <= %d)]")
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
	s := MustParse("$[?(@.age > %(min_age)d)]")
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
	s := MustParse("$[?@.name == %(name)s]")
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
	s := MustParse("$[?(@.price > %f)]")
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
	s := MustParse("$[?(@.age > %d)]")
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
	s := MustParse("$.items[*][?(@.score > %d)]")

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
	s := MustParse("$.users[*][?(@.age >= %(min_age)d)]")

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
	s := MustParse("$.users[*][?@.role == %s]")

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
	s := MustParse("$[?(@.age > %d)]")
	user := NewDictContext(map[string]any{"name": "Alice"}) // No age field

	_, err := s.Match(user, 25)
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func TestNativeParser_LogicalAndOperator(t *testing.T) {
	s := MustParse("$[?@.age > %d && @.active == %s]")
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
	s := MustParse("$[?@.age < %d || @.age > %d]")
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
	s := MustParse("$[?!(@.active == %s)]")
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
	s := MustParse("$[?(@.age >= %d && @.age <= %d) && @.status == %s]")
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
	s := MustParse("$[?@.age == %d]")
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
	s := MustParse("$[?@.active == %s]")
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
	s := MustParse("$.categories[*][?@.items[*][?@.price > %f]]")

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
	s := MustParse("$.categories[*][?@.items[*][?@.price > %f && @.price < %f]]")

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
	s := MustParse("$.categories[*][?@.items[*][?@.price > %f]]")

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
	s := MustParse("$.categories[*][?@.items[*][?@.price > %f]]")

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
	s := MustParse("$.categories[*][?@.items[*][?@.price > %(min_price)f]]")

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
	s := MustParse("$.store.products[*][?@.price > %f]")

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
	s := MustParse("$.company.department.team.members[*][?@.age > %d]")

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
	s := MustParse("$[?@.user.profile.age > %d]")

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
	s := MustParse("$.store.products[*][?@.price > %f && @.stock > %d]")

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
	s := MustParse("$.warehouse.items[*][?@.quantity < %(min_qty)d]")

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
	s := MustParse("$[?@.company.department.manager.level > %d]")

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

// Operator Associativity tests
//
// These tests verify that operators are left-associative:
// - a && b && c should be And(And(a, b), c), not And(a, And(b, c))
// - a || b || c should be Or(Or(a, b), c), not Or(a, Or(b, c))

func TestOperatorAssociativity_AndLeftAssociative(t *testing.T) {
	// Test that && is left-associative: a && b && c -> And(And(a, b), c)
	s := MustParse("$[?@.a == 1 && @.b == 2 && @.c == 3]")

	ast := s.AST()

	// Top level should be And (InfixNode)
	topAnd, ok := ast.(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode at top level, got %T", ast)
	}
	if topAnd.Operator() != spec.OperatorAnd {
		t.Errorf("expected AND operator, got %s", topAnd.Operator())
	}

	// Left child should also be And (left-associative)
	leftAnd, ok := topAnd.Left().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for left child, got %T", topAnd.Left())
	}
	if leftAnd.Operator() != spec.OperatorAnd {
		t.Errorf("expected AND operator for inner, got %s", leftAnd.Operator())
	}

	// Right child of top And should be Equal (the last comparison)
	rightEqual, ok := topAnd.Right().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for right child, got %T", topAnd.Right())
	}
	if rightEqual.Operator() != spec.OperatorEq {
		t.Errorf("expected EQ operator for right, got %s", rightEqual.Operator())
	}

	// The innermost And's children should both be Equal
	_, leftOk := leftAnd.Left().(spec.InfixNode)
	_, rightOk := leftAnd.Right().(spec.InfixNode)
	if !leftOk || !rightOk {
		t.Error("innermost And children should be InfixNodes (Equal)")
	}
}

func TestOperatorAssociativity_OrLeftAssociative(t *testing.T) {
	// Test that || is left-associative: a || b || c -> Or(Or(a, b), c)
	s := MustParse("$[?@.a == 1 || @.b == 2 || @.c == 3]")

	ast := s.AST()

	// Top level should be Or
	topOr, ok := ast.(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode at top level, got %T", ast)
	}
	if topOr.Operator() != spec.OperatorOr {
		t.Errorf("expected OR operator, got %s", topOr.Operator())
	}

	// Left child should also be Or (left-associative)
	leftOr, ok := topOr.Left().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for left child, got %T", topOr.Left())
	}
	if leftOr.Operator() != spec.OperatorOr {
		t.Errorf("expected OR operator for inner, got %s", leftOr.Operator())
	}

	// Right child should be Equal
	rightEqual, ok := topOr.Right().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for right child, got %T", topOr.Right())
	}
	if rightEqual.Operator() != spec.OperatorEq {
		t.Errorf("expected EQ operator for right, got %s", rightEqual.Operator())
	}
}

func TestOperatorAssociativity_MixedOperators(t *testing.T) {
	// a && b || c && d should be Or(And(a, b), And(c, d))
	s := MustParse("$[?@.a == 1 && @.b == 2 || @.c == 3 && @.d == 4]")

	ast := s.AST()

	// Top level should be Or (lowest precedence)
	topOr, ok := ast.(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode at top level, got %T", ast)
	}
	if topOr.Operator() != spec.OperatorOr {
		t.Errorf("expected OR operator, got %s", topOr.Operator())
	}

	// Both children should be And
	leftAnd, ok := topOr.Left().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for left child, got %T", topOr.Left())
	}
	if leftAnd.Operator() != spec.OperatorAnd {
		t.Errorf("expected AND operator for left, got %s", leftAnd.Operator())
	}

	rightAnd, ok := topOr.Right().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for right child, got %T", topOr.Right())
	}
	if rightAnd.Operator() != spec.OperatorAnd {
		t.Errorf("expected AND operator for right, got %s", rightAnd.Operator())
	}
}

// Operator Precedence tests
//
// These tests verify that && has higher precedence than ||:
// - a || b && c should be Or(a, And(b, c)), not And(Or(a, b), c)

func TestOperatorPrecedence_AndHigherThanOr(t *testing.T) {
	// Test that && binds tighter than ||: a || b && c -> Or(a, And(b, c))
	s := MustParse("$[?@.a == 1 || @.b == 2 && @.c == 3]")

	ast := s.AST()

	// Top level should be Or
	topOr, ok := ast.(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode at top level, got %T", ast)
	}
	if topOr.Operator() != spec.OperatorOr {
		t.Errorf("expected OR operator, got %s", topOr.Operator())
	}

	// Left should be simple Equal
	leftEqual, ok := topOr.Left().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for left child, got %T", topOr.Left())
	}
	if leftEqual.Operator() != spec.OperatorEq {
		t.Errorf("expected EQ operator for left, got %s", leftEqual.Operator())
	}

	// Right should be And (higher precedence bound first)
	rightAnd, ok := topOr.Right().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for right child, got %T", topOr.Right())
	}
	if rightAnd.Operator() != spec.OperatorAnd {
		t.Errorf("expected AND operator for right, got %s", rightAnd.Operator())
	}
}

func TestOperatorPrecedence_AndHigherThanOrReverse(t *testing.T) {
	// Test precedence: a && b || c -> Or(And(a, b), c)
	s := MustParse("$[?@.a == 1 && @.b == 2 || @.c == 3]")

	ast := s.AST()

	// Top level should be Or
	topOr, ok := ast.(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode at top level, got %T", ast)
	}
	if topOr.Operator() != spec.OperatorOr {
		t.Errorf("expected OR operator, got %s", topOr.Operator())
	}

	// Left should be And
	leftAnd, ok := topOr.Left().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for left child, got %T", topOr.Left())
	}
	if leftAnd.Operator() != spec.OperatorAnd {
		t.Errorf("expected AND operator for left, got %s", leftAnd.Operator())
	}

	// Right should be Equal
	rightEqual, ok := topOr.Right().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for right child, got %T", topOr.Right())
	}
	if rightEqual.Operator() != spec.OperatorEq {
		t.Errorf("expected EQ operator for right, got %s", rightEqual.Operator())
	}
}

func TestOperatorPrecedence_ParenthesesOverride(t *testing.T) {
	// Test that parentheses override default precedence.
	// (a || b) && c - parentheses force Or to bind first
	s := MustParse("$[?((@.a == 1 || @.b == 2)) && @.c == 3]")

	ast := s.AST()

	// Top level should be And (due to parentheses)
	topAnd, ok := ast.(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode at top level, got %T", ast)
	}
	if topAnd.Operator() != spec.OperatorAnd {
		t.Errorf("expected AND operator, got %s", topAnd.Operator())
	}

	// Left should be Or (grouped by parentheses)
	leftOr, ok := topAnd.Left().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for left child, got %T", topAnd.Left())
	}
	if leftOr.Operator() != spec.OperatorOr {
		t.Errorf("expected OR operator for left, got %s", leftOr.Operator())
	}

	// Right should be Equal
	rightEqual, ok := topAnd.Right().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for right child, got %T", topAnd.Right())
	}
	if rightEqual.Operator() != spec.OperatorEq {
		t.Errorf("expected EQ operator for right, got %s", rightEqual.Operator())
	}
}

func TestOperatorPrecedence_Complex(t *testing.T) {
	// Test complex expression: a || b && c || d && e
	// Should be: Or(Or(a, And(b, c)), And(d, e))
	s := MustParse("$[?@.a == 1 || @.b == 2 && @.c == 3 || @.d == 4 && @.e == 5]")

	ast := s.AST()

	// Top level Or
	topOr, ok := ast.(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode at top level, got %T", ast)
	}
	if topOr.Operator() != spec.OperatorOr {
		t.Errorf("expected OR operator, got %s", topOr.Operator())
	}

	// Left is Or
	leftOr, ok := topOr.Left().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for left child, got %T", topOr.Left())
	}
	if leftOr.Operator() != spec.OperatorOr {
		t.Errorf("expected OR operator for left, got %s", leftOr.Operator())
	}

	// Right is And
	rightAnd, ok := topOr.Right().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for right child, got %T", topOr.Right())
	}
	if rightAnd.Operator() != spec.OperatorAnd {
		t.Errorf("expected AND operator for right, got %s", rightAnd.Operator())
	}

	// Left Or's right is And
	leftOrRight, ok := leftOr.Right().(spec.InfixNode)
	if !ok {
		t.Fatalf("expected InfixNode for leftOr.Right, got %T", leftOr.Right())
	}
	if leftOrRight.Operator() != spec.OperatorAnd {
		t.Errorf("expected AND operator for leftOr.Right, got %s", leftOrRight.Operator())
	}
}

// Error Messages tests
//
// These tests verify that:
// 1. Correct exception types are raised
// 2. Error messages contain useful information
// 3. Position information is accurate

func TestErrorMessages_SyntaxErrorType(t *testing.T) {
	// Test that syntax errors raise JSONPathSyntaxError
	_, err := Parse("$[?@.age ~ 25]") // Invalid operator
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	_, ok := err.(*JSONPathSyntaxError)
	if !ok {
		t.Errorf("expected JSONPathSyntaxError, got %T", err)
	}
}

func TestErrorMessages_SyntaxErrorHasPosition(t *testing.T) {
	// Test that syntax error includes position information
	_, err := Parse("$[?@.age ~ 25]")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	syntaxErr, ok := err.(*JSONPathSyntaxError)
	if !ok {
		t.Fatalf("expected JSONPathSyntaxError, got %T", err)
	}

	if syntaxErr.Position < 0 {
		t.Error("expected non-negative position")
	}
	// Position of '~' is 9
	if syntaxErr.Position != 9 {
		t.Errorf("expected position 9, got %d", syntaxErr.Position)
	}
}

func TestErrorMessages_SyntaxErrorHasExpression(t *testing.T) {
	// Test that syntax error includes original expression
	expr := "$[?@.age ~ 25]"
	_, err := Parse(expr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	syntaxErr, ok := err.(*JSONPathSyntaxError)
	if !ok {
		t.Fatalf("expected JSONPathSyntaxError, got %T", err)
	}

	if syntaxErr.Expression != expr {
		t.Errorf("expected expression %q, got %q", expr, syntaxErr.Expression)
	}
}

func TestErrorMessages_SyntaxErrorHasContext(t *testing.T) {
	// Test that syntax error includes context hint
	_, err := Parse("$[?@.age ~ 25]")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	syntaxErr, ok := err.(*JSONPathSyntaxError)
	if !ok {
		t.Fatalf("expected JSONPathSyntaxError, got %T", err)
	}

	if syntaxErr.Context == "" {
		t.Error("expected non-empty context")
	}
}

func TestErrorMessages_SyntaxErrorMessageFormatting(t *testing.T) {
	// Test that error message is well-formatted
	_, err := Parse("$[?@.age ~ 25]")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	message := err.Error()

	// Should contain position
	if !containsSubstring(message, "9") {
		t.Error("error message should contain position")
	}

	// Should contain the expression
	if !containsSubstring(message, "$[?@.age ~ 25]") {
		t.Error("error message should contain expression")
	}

	// Should contain pointer
	if !containsSubstring(message, "^") {
		t.Error("error message should contain pointer")
	}
}

func TestErrorMessages_UnexpectedEndOfExpression(t *testing.T) {
	// Test error when expression ends unexpectedly
	_, err := Parse("$[?@.age >")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !containsSubstring(err.Error(), "end") && !containsSubstring(err.Error(), "End") {
		t.Error("error message should mention 'end'")
	}
}

func TestErrorMessages_MissingFieldNameError(t *testing.T) {
	// Test error when field name is missing
	_, err := Parse("$[?@. > 25]")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !containsSubstring(err.Error(), "field") && !containsSubstring(err.Error(), "Field") {
		t.Error("error message should mention 'field'")
	}
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (containsSubstringHelper(s, substr)))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Thread Safety tests
//
// These tests verify that a single specification instance can be
// safely used from multiple threads concurrently.

func TestThreadSafety_ConcurrentMatchCalls(t *testing.T) {
	// Test that Match() is thread-safe
	s := MustParse("$[?@.value > %d]")
	var errors []string
	var results []struct {
		threadID int
		i        int
		result   bool
	}
	var mu sync.Mutex
	var wg sync.WaitGroup

	for threadID := 0; threadID < 10; threadID++ {
		wg.Add(1)
		go func(tid int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				data := NewDictContext(map[string]any{"value": tid*10 + i})
				result, err := s.Match(data, tid*10)
				if err != nil {
					mu.Lock()
					errors = append(errors, err.Error())
					mu.Unlock()
					return
				}
				mu.Lock()
				results = append(results, struct {
					threadID int
					i        int
					result   bool
				}{tid, i, result})
				mu.Unlock()
			}
		}(threadID)
	}

	wg.Wait()

	if len(errors) != 0 {
		t.Errorf("errors occurred: %v", errors)
	}
	if len(results) != 1000 {
		t.Errorf("expected 1000 results, got %d", len(results))
	}
}

func TestThreadSafety_ConcurrentDifferentParams(t *testing.T) {
	// Test concurrent calls with different parameters
	s := MustParse("$[?@.x == %d && @.y == %s]")
	var errors []string
	var results []struct {
		threadID int
		result   bool
	}
	var mu sync.Mutex
	var wg sync.WaitGroup

	for threadID := 0; threadID < 10; threadID++ {
		wg.Add(1)
		go func(tid int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				data := NewDictContext(map[string]any{"x": tid, "y": "val_" + string(rune('0'+tid))})
				result, err := s.Match(data, tid, "val_"+string(rune('0'+tid)))
				if err != nil {
					mu.Lock()
					errors = append(errors, err.Error())
					mu.Unlock()
					return
				}
				mu.Lock()
				results = append(results, struct {
					threadID int
					result   bool
				}{tid, result})
				// All should match since we use the same values
				if !result {
					errors = append(errors, "unexpected false result")
				}
				mu.Unlock()
			}
		}(threadID)
	}

	wg.Wait()

	if len(errors) != 0 {
		t.Errorf("errors occurred: %v", errors)
	}

	for _, r := range results {
		if !r.result {
			t.Error("expected all results to be true")
			break
		}
	}
}

// AST Caching tests
//
// These tests verify that the AST is parsed once and cached,
// not re-parsed on every Match() call.

func TestASTCaching_ASTIsCached(t *testing.T) {
	// Test that AST is stored after parsing
	s := MustParse("$[?@.age > %d]")

	// AST should be cached
	if s.AST() == nil {
		t.Error("expected AST to be cached")
	}
}

func TestASTCaching_ASTNotReparsedOnMatch(t *testing.T) {
	// Test that AST is not re-created on Match()
	s := MustParse("$[?@.age > %d]")
	originalAST := s.AST()

	// Multiple match calls
	data := NewDictContext(map[string]any{"age": 30})
	s.Match(data, 25)
	s.Match(data, 35)
	s.Match(data, 20)

	// AST should be the same object
	if s.AST() != originalAST {
		t.Error("AST should be the same object after multiple Match calls")
	}
}

func TestASTCaching_DifferentParamsSameAST(t *testing.T) {
	// Test that different parameters don't affect cached AST
	s := MustParse("$[?@.value == %s]")
	originalAST := s.AST()

	data := NewDictContext(map[string]any{"value": "test"})

	// Call with different param types
	s.Match(data, "test")
	s.Match(data, "other")
	s.Match(data, "third")

	// AST should remain unchanged
	if s.AST() != originalAST {
		t.Error("AST should remain unchanged after calls with different params")
	}
}

// Helper Methods tests
//
// These tests verify that helper methods work correctly.

func TestHelperMethods_ParseIdentifierChainSingle(t *testing.T) {
	// Test parsing single identifier
	s := MustParse("$[?@.field > 1]")
	lexer := NewLexer("field")
	tokens, _ := lexer.Tokenize()

	chain, pos := s.parseIdentifierChain(tokens, 0)

	if len(chain) != 1 || chain[0] != "field" {
		t.Errorf("expected [\"field\"], got %v", chain)
	}
	if pos != 1 {
		t.Errorf("expected pos 1, got %d", pos)
	}
}

func TestHelperMethods_ParseIdentifierChainMultiple(t *testing.T) {
	// Test parsing multiple identifiers
	s := MustParse("$[?@.a.b.c > 1]")
	lexer := NewLexer("a.b.c")
	tokens, _ := lexer.Tokenize()

	chain, _ := s.parseIdentifierChain(tokens, 0)

	expected := []string{"a", "b", "c"}
	if len(chain) != len(expected) {
		t.Errorf("expected %v, got %v", expected, chain)
		return
	}
	for i, v := range expected {
		if chain[i] != v {
			t.Errorf("expected %v, got %v", expected, chain)
			break
		}
	}
}

func TestHelperMethods_IsWildcardPatternTrue(t *testing.T) {
	// Test wildcard pattern detection - positive case
	s := MustParse("$[?@.x > 1]")
	lexer := NewLexer("[*]")
	tokens, _ := lexer.Tokenize()

	if !s.isWildcardPattern(tokens, 0) {
		t.Error("expected isWildcardPattern to return true for [*]")
	}
}

func TestHelperMethods_IsWildcardPatternFalse(t *testing.T) {
	// Test wildcard pattern detection - negative case
	s := MustParse("$[?@.x > 1]")
	lexer := NewLexer("[?@.x]")
	tokens, _ := lexer.Tokenize()

	if s.isWildcardPattern(tokens, 0) {
		t.Error("expected isWildcardPattern to return false for [?@.x]")
	}
}

func TestHelperMethods_BuildObjectChainEmpty(t *testing.T) {
	// Test building object chain with empty list
	s := MustParse("$[?@.x > 1]")
	parent := spec.GlobalScope()

	result := s.buildObjectChain(parent, []string{})

	if result != parent {
		t.Error("expected result to be parent for empty chain")
	}
}

func TestHelperMethods_BuildObjectChainSingle(t *testing.T) {
	// Test building object chain with single name
	s := MustParse("$[?@.x > 1]")
	parent := spec.GlobalScope()

	result := s.buildObjectChain(parent, []string{"field"})

	_, ok := result.(spec.ObjectNode)
	if !ok {
		t.Errorf("expected ObjectNode, got %T", result)
	}
}

func TestHelperMethods_BuildObjectChainMultiple(t *testing.T) {
	// Test building object chain with multiple names
	s := MustParse("$[?@.x > 1]")
	parent := spec.GlobalScope()

	result := s.buildObjectChain(parent, []string{"a", "b", "c"})

	// Should be Object(Object(Object(parent, "a"), "b"), "c")
	_, ok := result.(spec.ObjectNode)
	if !ok {
		t.Errorf("expected ObjectNode, got %T", result)
		return
	}

	// Verify nesting depth by traversing
	depth := 0
	current := result
	for {
		obj, ok := current.(spec.ObjectNode)
		if !ok {
			break
		}
		depth++
		current = obj.Parent()
	}

	if depth != 3 {
		t.Errorf("expected depth 3, got %d", depth)
	}
}
