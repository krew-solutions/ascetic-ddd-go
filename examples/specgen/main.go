package main

import (
	"fmt"
	"log"
)

func main() {
	users := []User{
		{ID: 1, Age: 30, Active: true, Name: "Alice", Email: "alice@example.com"},
		{ID: 2, Age: 17, Active: true, Name: "Bob", Email: "bob@example.com"},
		{ID: 3, Age: 25, Active: false, Name: "Charlie", Email: ""},
		{ID: 4, Age: 40, Active: true, Name: "", Email: "dave@example.com"},
	}

	fmt.Println("=== In-Memory Filtering (Maximum Performance) ===")

	// In-memory: использует нативные Go функции (самое быстрое)
	fmt.Println("Adult users:")
	for _, u := range users {
		if AdultUserSpec(u) { // Прямой вызов Go функции - максимальная скорость!
			fmt.Printf("  - %s (age: %d)\n", u.Name, u.Age)
		}
	}

	fmt.Println("\nActive users:")
	for _, u := range users {
		if ActiveUserSpec(u) {
			fmt.Printf("  - %s\n", u.Name)
		}
	}

	fmt.Println("\nPremium users (adult + active + has name):")
	for _, u := range users {
		if PremiumUserSpec(u) {
			fmt.Printf("  - %s (age: %d, active: %v)\n", u.Name, u.Age, u.Active)
		}
	}

	fmt.Println("\nInactive users:")
	for _, u := range users {
		if InactiveUserSpec(u) {
			fmt.Printf("  - %s\n", u.Name)
		}
	}

	fmt.Println("\n=== SQL Generation (For Database Queries) ===")

	// SQL: генерируется из AST только когда нужно
	sql, params, err := AdultUserSpecSQL()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("AdultUserSpec SQL:\n  WHERE %s\n  Params: %v\n\n", sql, params)

	sql, params, err = ActiveUserSpecSQL()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("ActiveUserSpec SQL:\n  WHERE %s\n  Params: %v\n\n", sql, params)

	sql, params, err = PremiumUserSpecSQL()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("PremiumUserSpec SQL:\n  WHERE %s\n  Params: %v\n\n", sql, params)

	sql, params, err = InactiveUserSpecSQL()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("InactiveUserSpec SQL:\n  WHERE %s\n  Params: %v\n\n", sql, params)

	fmt.Println("=== Performance Summary ===")
	fmt.Println("✅ In-memory: Direct Go function call - FASTEST")
	fmt.Println("✅ SQL: Generated from AST only when needed - ZERO OVERHEAD")
	fmt.Println("✅ DRY: Specification written once as Go function")
	fmt.Println("✅ Type-safe: Compile-time checks for both in-memory and SQL")
}
