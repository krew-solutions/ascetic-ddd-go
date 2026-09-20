package main

import (
	"fmt"
	"log"
	"strings"
)

func runAdvancedDemo() {
	// Test data
	store := Store{
		ID:     1,
		Name:   "Premium Store",
		Active: true,
		Items: []Item{
			{ID: 1, Name: "Laptop", Price: 1500, Active: true, Stock: 5},
			{ID: 2, Name: "Mouse", Price: 50, Active: true, Stock: 100},
			{ID: 3, Name: "Monitor", Price: 800, Active: false, Stock: 0},
			{ID: 4, Name: "Keyboard", Price: 150, Active: true, Stock: 20},
		},
	}

	cheapStore := Store{
		ID:     2,
		Name:   "Budget Store",
		Active: true,
		Items: []Item{
			{ID: 5, Name: "Pencil", Price: 5, Active: true, Stock: 500},
			{ID: 6, Name: "Notebook", Price: 10, Active: true, Stock: 200},
		},
	}

	fmt.Println("\n=== ADVANCED SPECIFICATION FEATURES ===")
	fmt.Println("\n--- Test Store ---")
	fmt.Printf("Store: %s (Active: %v)\n", store.Name, store.Active)
	fmt.Printf("Items: %d\n", len(store.Items))
	for _, item := range store.Items {
		fmt.Printf("  - %s: $%d (Active: %v, Stock: %d)\n",
			item.Name, item.Price, item.Active, item.Stock)
	}

	fmt.Println("\n=== 1. WILDCARD SPECIFICATIONS (Collections) ===")

	// Any - at least one item matches
	fmt.Println("\n✓ spec.Any() - Checks if ANY item matches:")
	if HasExpensiveItemsSpec(store) {
		fmt.Println("  ✓ Store HAS expensive items (>$1000)")
		sql, params, _ := storeSQL(HasExpensiveItemsSpecAST())
		fmt.Printf("  SQL: WHERE %s\n  Params: %v\n", sql, params)
	}

	if HasCheapItemsSpec(store) {
		fmt.Println("  ✓ Store HAS cheap items (<$100)")
		sql, params, _ := storeSQL(HasCheapItemsSpecAST())
		fmt.Printf("  SQL: WHERE %s\n  Params: %v\n", sql, params)
	}

	if HasItemInStockSpec(store) {
		fmt.Println("  ✓ Store HAS items in stock")
		sql, params, _ := storeSQL(HasItemInStockSpecAST())
		fmt.Printf("  SQL: WHERE %s\n  Params: %v\n", sql, params)
	}

	// All - every item must match
	fmt.Println("\n✓ spec.All() - Checks if ALL items match:")
	if AllItemsActiveSpec(cheapStore) {
		fmt.Println("  ✓ Cheap store: ALL items active")
		sql, params, _ := storeSQL(AllItemsActiveSpecAST())
		fmt.Printf("  SQL: WHERE %s\n  Params: %v\n", sql, params)
	}
	if !AllItemsActiveSpec(store) {
		fmt.Println("  ✗ Premium store: NOT all items active (monitor is inactive)")
	}

	fmt.Println("\n=== 2. COMPLEX WILDCARD CONDITIONS ===")

	if HasAffordableActiveItemsSpec(store) {
		fmt.Println("✓ Store has affordable active items (<$500 AND active)")
		sql, params, _ := storeSQL(HasAffordableActiveItemsSpecAST())
		fmt.Printf("  SQL: WHERE %s\n  Params: %v\n", sql, params)
	}

	if HasPremiumItemsSpec(store) {
		fmt.Println("✓ Store has premium items (>$5000 AND in stock AND active)")
		sql, params, _ := storeSQL(HasPremiumItemsSpecAST())
		fmt.Printf("  SQL: WHERE %s\n  Params: %v\n", sql, params)
	}

	fmt.Println("\n=== 3. ARITHMETIC OPERATIONS ===")

	if HasDiscountedExpensiveItemsSpec(store) {
		fmt.Println("✓ Store has items where (price - 100) > 900")
		sql, params, _ := storeSQL(HasDiscountedExpensiveItemsSpecAST())
		fmt.Printf("  SQL: WHERE %s\n  Params: %v\n", sql, params)
	}

	if HasTaxedCheapItemsSpec(store) {
		fmt.Println("✓ Store has items where (price + price/10) < 110")
		sql, params, _ := storeSQL(HasTaxedCheapItemsSpecAST())
		fmt.Printf("  SQL: WHERE %s\n  Params: %v\n", sql, params)
	}

	fmt.Println("\n=== 4. BITWISE OPERATIONS ===")

	if HasItemWithShiftedIDSpec(store) {
		fmt.Println("✓ Store has item where (ID << 2) == 8")
		sql, params, _ := storeSQL(HasItemWithShiftedIDSpecAST())
		fmt.Printf("  SQL: WHERE %s\n  Params: %v\n", sql, params)
	}

	fmt.Println("\n=== 5. COMBINED SPECIFICATIONS ===")

	if PremiumActiveStoreSpec(store) {
		fmt.Println("✓ Premium Active Store: active + named + has expensive items")
		sql, params, _ := storeSQL(PremiumActiveStoreSpecAST())
		fmt.Printf("  SQL: WHERE %s\n  Params: %v\n", sql, params)
	}

	if BudgetFriendlyStoreSpec(cheapStore) {
		fmt.Println("✓ Budget Friendly Store: active + all items < $1000")
		sql, params, _ := storeSQL(BudgetFriendlyStoreSpecAST())
		fmt.Printf("  SQL: WHERE %s\n  Params: %v\n", sql, params)
	}

	fmt.Println("\n=== 6. NEGATION WITH WILDCARDS ===")

	if NoExpensiveItemsSpec(cheapStore) {
		fmt.Println("✓ Budget store: NO items over $5000")
		sql, params, _ := storeSQL(NoExpensiveItemsSpecAST())
		fmt.Printf("  SQL: WHERE %s\n  Params: %v\n", sql, params)
	}

	if NotAllItemsActiveSpec(store) {
		fmt.Println("✓ Premium store: NOT all items are active")
		sql, params, _ := storeSQL(NotAllItemsActiveSpecAST())
		fmt.Printf("  SQL: WHERE %s\n  Params: %v\n", sql, params)
	}

	fmt.Println("\n=== SUMMARY ===")
	fmt.Println("✅ Wildcards: spec.Any() and spec.All() for collections")
	fmt.Println("✅ Nested fields: item.Price, item.Active, etc.")
	fmt.Println("✅ Arithmetic: +, -, *, /, %")
	fmt.Println("✅ Bitwise: <<, >> (AND, OR, XOR - TODO)")
	fmt.Println("✅ Complex predicates: Wildcards + AND + OR + NOT")
	fmt.Println("✅ SQL generation: All features compile to SQL")

	// Show performance note
	fmt.Println("\n💡 PERFORMANCE NOTE:")
	fmt.Println("  In-memory: Direct Go function call (FASTEST)")
	fmt.Println("  SQL: Generated from pre-built AST (ZERO OVERHEAD)")
}

func main() {
	fmt.Println("=== Basic Example (User) ===")
	runBasicDemo()

	fmt.Println("\n" + strings.Repeat("=", 70))
	runAdvancedDemo()

	fmt.Println("\n" + strings.Repeat("=", 70))
	runNestedDemo()
}

func runBasicDemo() {
	users := []User{
		{ID: 1, Age: 30, Active: true, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Age: 17, Active: true, Name: "Bob", Email: "bob@example.com"},
		{ID: 3, Age: 25, Active: false, Name: "Charlie", Email: ""},
		{ID: 4, Age: 40, Active: true, Name: "", Email: "dave@example.com"},
	}

	fmt.Println("\nAdult users:")
	for _, u := range users {
		if AdultUserSpec(u) {
			fmt.Printf("  - %s (age: %d)\n", u.Name, u.Age)
		}
	}

	fmt.Println("\nPremium users:")
	for _, u := range users {
		if PremiumUserSpec(u) {
			fmt.Printf("  - %s (age: %d)\n", u.Name, u.Age)
		}
	}

	sql, params, err := userSQL(AdultUserSpecAST())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nAdultUserSpec SQL: WHERE %s\n  Params: %v\n", sql, params)
}
