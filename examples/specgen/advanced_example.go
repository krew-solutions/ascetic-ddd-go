package main

import (
	spec "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

//go:generate go run github.com/krew-solutions/ascetic-ddd-go/cmd/specgen -type=Store

// Item represents an item in a store
type Item struct {
	ID     int64
	Name   string
	Price  int
	Active bool
	Stock  int
}

// Store represents a store with items
type Store struct {
	ID     int64
	Name   string
	Active bool
	Items  []Item
}

// === Simple Specifications ===

// ActiveStoreSpec checks if store is active
//spec:sql
func ActiveStoreSpec(s Store) bool {
	return s.Active
}

// NamedStoreSpec checks if store has a name
//spec:sql
func NamedStoreSpec(s Store) bool {
	return s.Name != ""
}

// === Wildcard Specifications (Collections) ===

// HasExpensiveItemsSpec checks if store has any item with price > 1000
//spec:sql
func HasExpensiveItemsSpec(s Store) bool {
	return spec.Any(s.Items, func(item Item) bool {
		return item.Price > 1000
	})
}

// AllItemsActiveSpec checks if all items are active
//spec:sql
func AllItemsActiveSpec(s Store) bool {
	return spec.All(s.Items, func(item Item) bool {
		return item.Active
	})
}

// HasCheapItemsSpec checks if store has any cheap item (price < 100)
//spec:sql
func HasCheapItemsSpec(s Store) bool {
	return spec.Any(s.Items, func(item Item) bool {
		return item.Price < 100
	})
}

// HasItemInStockSpec checks if store has any item in stock
//spec:sql
func HasItemInStockSpec(s Store) bool {
	return spec.Any(s.Items, func(item Item) bool {
		return item.Stock > 0
	})
}

// === Complex Wildcard Specifications ===

// HasAffordableActiveItemsSpec checks for active items under 500
//spec:sql
func HasAffordableActiveItemsSpec(s Store) bool {
	return spec.Any(s.Items, func(item Item) bool {
		return item.Price < 500 && item.Active
	})
}

// HasPremiumItemsSpec checks for items over 5000 with stock
//spec:sql
func HasPremiumItemsSpec(s Store) bool {
	return spec.Any(s.Items, func(item Item) bool {
		return item.Price > 5000 && item.Stock > 0 && item.Active
	})
}

// === Arithmetic Operations ===

// HasDiscountedExpensiveItemsSpec checks items where price-100 > 900
//spec:sql
func HasDiscountedExpensiveItemsSpec(s Store) bool {
	return spec.Any(s.Items, func(item Item) bool {
		return item.Price-100 > 900
	})
}

// HasTaxedCheapItemsSpec checks items where price*1.2 < 120
//spec:sql
func HasTaxedCheapItemsSpec(s Store) bool {
	return spec.Any(s.Items, func(item Item) bool {
		return item.Price+item.Price/10 < 110 // price + 10% tax
	})
}

// === Bitwise Operations ===

// HasItemWithFlagSpec checks if any item has specific bit flag (example)
//spec:sql
func HasItemWithFlagSpec(s Store) bool {
	return spec.Any(s.Items, func(item Item) bool {
		return item.Stock&1 == 1 // odd stock number
	})
}

// HasItemWithShiftedIDSpec checks if any item has ID that when shifted equals 8
//spec:sql
func HasItemWithShiftedIDSpec(s Store) bool {
	return spec.Any(s.Items, func(item Item) bool {
		return item.ID<<2 == 8
	})
}

// === Combined Complex Specifications ===

// PremiumActiveStoreSpec combines multiple conditions
//spec:sql
func PremiumActiveStoreSpec(s Store) bool {
	return s.Active && s.Name != "" && spec.Any(s.Items, func(item Item) bool {
		return item.Price > 1000
	})
}

// BudgetFriendlyStoreSpec checks for active store with cheap items
//spec:sql
func BudgetFriendlyStoreSpec(s Store) bool {
	return s.Active && spec.All(s.Items, func(item Item) bool {
		return item.Price < 1000
	})
}

// === Negation with Wildcards ===

// NoExpensiveItemsSpec checks that no items are expensive
//spec:sql
func NoExpensiveItemsSpec(s Store) bool {
	return !spec.Any(s.Items, func(item Item) bool {
		return item.Price > 5000
	})
}

// NotAllItemsActiveSpec checks that not all items are active
//spec:sql
func NotAllItemsActiveSpec(s Store) bool {
	return !spec.All(s.Items, func(item Item) bool {
		return item.Active
	})
}
