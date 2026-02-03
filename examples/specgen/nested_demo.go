package main

import (
	"fmt"
	"log"
)

func runNestedDemo() {
	fmt.Println("\n=== NESTED WILDCARD SPECIFICATIONS ===")

	// Test organization with nested structure
	org := Organization{
		ID:     1,
		Name:   "Global Corp",
		Active: true,
		Regions: []Region{
			{
				ID:     1,
				Name:   "North America",
				Active: true,
				Categories: []Category{
					{
						ID:     1,
						Name:   "Electronics",
						Active: true,
						Items: []Item{
							{ID: 1, Name: "Premium Laptop", Price: 6000, Active: true, Stock: 3},
							{ID: 2, Name: "Budget Mouse", Price: 50, Active: true, Stock: 100},
						},
					},
					{
						ID:     2,
						Name:   "Furniture",
						Active: false,
						Items: []Item{
							{ID: 3, Name: "Office Chair", Price: 500, Active: true, Stock: 10},
						},
					},
				},
			},
			{
				ID:     2,
				Name:   "Europe",
				Active: true,
				Categories: []Category{
					{
						ID:     3,
						Name:   "Luxury",
						Active: true,
						Items: []Item{
							{ID: 4, Name: "Designer Watch", Price: 15000, Active: true, Stock: 2},
						},
					},
				},
			},
		},
	}

	fmt.Println("\n--- Test Organization ---")
	fmt.Printf("Organization: %s (Active: %v)\n", org.Name, org.Active)
	fmt.Printf("Regions: %d\n", len(org.Regions))
	for _, region := range org.Regions {
		fmt.Printf("  Region: %s (Active: %v, Categories: %d)\n", region.Name, region.Active, len(region.Categories))
		for _, category := range region.Categories {
			fmt.Printf("    Category: %s (Active: %v, Items: %d)\n", category.Name, category.Active, len(category.Items))
			for _, item := range category.Items {
				fmt.Printf("      - %s: $%d (Active: %v, Stock: %d)\n",
					item.Name, item.Price, item.Active, item.Stock)
			}
		}
	}

	fmt.Println("\n=== 1. SIMPLE NESTED WILDCARD (3 levels) ===")
	if HasRegionWithExpensiveItemsSpec(org) {
		fmt.Println("✓ Organization HAS region with expensive items (>$5000)")
		sql, params, err := HasRegionWithExpensiveItemsSpecSQL()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  SQL: WHERE %s\n", sql)
		fmt.Printf("  Params: %v\n", params)
	}

	fmt.Println("\n=== 2. NESTED WITH CONDITIONS AT EACH LEVEL ===")
	if HasActiveRegionWithPremiumItemsSpec(org) {
		fmt.Println("✓ Organization HAS active region -> active category -> premium item")
		sql, params, err := HasActiveRegionWithPremiumItemsSpecSQL()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  SQL: WHERE %s\n", sql)
		fmt.Printf("  Params: %v\n", params)
	}

	fmt.Println("\n=== 3. MIXED: ROOT + NESTED ===")
	if ActiveOrgWithExpensiveItemsSpec(org) {
		fmt.Println("✓ Active organization with expensive items (>$10000)")
		sql, params, err := ActiveOrgWithExpensiveItemsSpecSQL()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  SQL: WHERE %s\n", sql)
		fmt.Printf("  Params: %v\n", params)
	}

	fmt.Println("\n=== 4. NEGATION OF NESTED ===")
	if NoRegionWithExpensiveItemsSpec(org) {
		fmt.Println("✓ Organization has NO region with ultra-expensive items (>$100000)")
		sql, params, err := NoRegionWithExpensiveItemsSpecSQL()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("  SQL: WHERE %s\n", sql)
		fmt.Printf("  Params: %v\n", params)
	}

	fmt.Println("\n=== NESTED WILDCARD SUMMARY ===")
	fmt.Println("✅ Triple nesting: Organization -> Regions -> Categories -> Items")
	fmt.Println("✅ Each level gets unique alias: region_1, category_2, item_3")
	fmt.Println("✅ Nested paths: category_1.Items, region_1.Categories")
	fmt.Println("✅ Conditions at each nesting level")
	fmt.Println("✅ SQL: Nested EXISTS subqueries with proper aliasing")
	fmt.Println("\n📝 Generated SQL Pattern:")
	fmt.Println("  EXISTS (SELECT 1 FROM unnest(Regions) AS region_1")
	fmt.Println("    WHERE EXISTS (SELECT 1 FROM unnest(region_1.Categories) AS category_2")
	fmt.Println("      WHERE EXISTS (SELECT 1 FROM unnest(category_2.Items) AS item_3")
	fmt.Println("        WHERE item_3.Price > $1)))")
}
