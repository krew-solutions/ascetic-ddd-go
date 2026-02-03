package main

//go:generate go run github.com/krew-solutions/ascetic-ddd-go/cmd/specgen -type=Organization

import (
	spec "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

// Nested collection structure: Organization -> Regions -> Categories -> Items
type Organization struct {
	ID      int
	Name    string
	Active  bool
	Regions []Region
}

type Region struct {
	ID         int
	Name       string
	Active     bool
	Categories []Category
}

type Category struct {
	ID     int
	Name   string
	Active bool
	Items  []Item
}

// Simple nested: Region has expensive items
//spec:sql
func HasRegionWithExpensiveItemsSpec(o Organization) bool {
	return spec.Any(o.Regions, func(region Region) bool {
		return spec.Any(region.Categories, func(category Category) bool {
			return spec.Any(category.Items, func(item Item) bool {
				return item.Price > 5000
			})
		})
	})
}

// Nested with conditions at each level
//spec:sql
func HasActiveRegionWithPremiumItemsSpec(o Organization) bool {
	return spec.Any(o.Regions, func(region Region) bool {
		return region.Active && spec.Any(region.Categories, func(category Category) bool {
			return category.Active && spec.Any(category.Items, func(item Item) bool {
				return item.Price > 5000 && item.Active
			})
		})
	})
}

// Mixed: root + nested wildcards
//spec:sql
func ActiveOrgWithExpensiveItemsSpec(o Organization) bool {
	return o.Active && spec.Any(o.Regions, func(region Region) bool {
		return spec.Any(region.Categories, func(category Category) bool {
			return spec.Any(category.Items, func(item Item) bool {
				return item.Price > 10000
			})
		})
	})
}

// Negation of nested
//spec:sql
func NoRegionWithExpensiveItemsSpec(o Organization) bool {
	return !spec.Any(o.Regions, func(region Region) bool {
		return spec.Any(region.Categories, func(category Category) bool {
			return spec.Any(category.Items, func(item Item) bool {
				return item.Price > 100000
			})
		})
	})
}
