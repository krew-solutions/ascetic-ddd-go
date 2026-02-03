package specification

import (
	"testing"
)

// TestCollectionHelpers tests Any and All helper functions

type TestItem struct {
	ID     int
	Name   string
	Price  int
	Active bool
}

func TestAnyHelper(t *testing.T) {
	items := []TestItem{
		{ID: 1, Name: "A", Price: 100, Active: true},
		{ID: 2, Name: "B", Price: 200, Active: false},
		{ID: 3, Name: "C", Price: 300, Active: true},
	}

	// Test: Any item with price > 150
	result := Any(items, func(item TestItem) bool {
		return item.Price > 150
	})

	if !result {
		t.Error("Expected true - at least one item has price > 150")
	}
}

func TestAnyHelperNoneMatch(t *testing.T) {
	items := []TestItem{
		{ID: 1, Name: "A", Price: 100, Active: true},
		{ID: 2, Name: "B", Price: 50, Active: false},
	}

	// Test: Any item with price > 500
	result := Any(items, func(item TestItem) bool {
		return item.Price > 500
	})

	if result {
		t.Error("Expected false - no items have price > 500")
	}
}

func TestAnyHelperEmptySlice(t *testing.T) {
	var items []TestItem

	// Test: Any item in empty slice
	result := Any(items, func(item TestItem) bool {
		return item.Price > 0
	})

	if result {
		t.Error("Expected false for empty slice")
	}
}

func TestAnyHelperFirstMatch(t *testing.T) {
	items := []TestItem{
		{ID: 1, Name: "A", Price: 500, Active: true},
		{ID: 2, Name: "B", Price: 100, Active: false},
		{ID: 3, Name: "C", Price: 50, Active: true},
	}

	// Test: Any item with price > 400 (first item matches)
	result := Any(items, func(item TestItem) bool {
		return item.Price > 400
	})

	if !result {
		t.Error("Expected true - first item matches")
	}
}

func TestAnyHelperLastMatch(t *testing.T) {
	items := []TestItem{
		{ID: 1, Name: "A", Price: 50, Active: true},
		{ID: 2, Name: "B", Price: 100, Active: false},
		{ID: 3, Name: "C", Price: 500, Active: true},
	}

	// Test: Any item with price > 400 (last item matches)
	result := Any(items, func(item TestItem) bool {
		return item.Price > 400
	})

	if !result {
		t.Error("Expected true - last item matches")
	}
}

func TestAllHelper(t *testing.T) {
	items := []TestItem{
		{ID: 1, Name: "A", Price: 100, Active: true},
		{ID: 2, Name: "B", Price: 200, Active: true},
		{ID: 3, Name: "C", Price: 300, Active: true},
	}

	// Test: All items are active
	result := All(items, func(item TestItem) bool {
		return item.Active
	})

	if !result {
		t.Error("Expected true - all items are active")
	}
}

func TestAllHelperOneDoesNotMatch(t *testing.T) {
	items := []TestItem{
		{ID: 1, Name: "A", Price: 100, Active: true},
		{ID: 2, Name: "B", Price: 200, Active: false}, // This one is not active
		{ID: 3, Name: "C", Price: 300, Active: true},
	}

	// Test: All items are active
	result := All(items, func(item TestItem) bool {
		return item.Active
	})

	if result {
		t.Error("Expected false - one item is not active")
	}
}

func TestAllHelperEmptySlice(t *testing.T) {
	var items []TestItem

	// Test: All items in empty slice (vacuous truth)
	result := All(items, func(item TestItem) bool {
		return item.Active
	})

	if !result {
		t.Error("Expected true for empty slice (vacuous truth)")
	}
}

func TestAllHelperFirstDoesNotMatch(t *testing.T) {
	items := []TestItem{
		{ID: 1, Name: "A", Price: 100, Active: false}, // First doesn't match
		{ID: 2, Name: "B", Price: 200, Active: true},
		{ID: 3, Name: "C", Price: 300, Active: true},
	}

	// Test: All items are active
	result := All(items, func(item TestItem) bool {
		return item.Active
	})

	if result {
		t.Error("Expected false - first item is not active")
	}
}

func TestAllHelperLastDoesNotMatch(t *testing.T) {
	items := []TestItem{
		{ID: 1, Name: "A", Price: 100, Active: true},
		{ID: 2, Name: "B", Price: 200, Active: true},
		{ID: 3, Name: "C", Price: 300, Active: false}, // Last doesn't match
	}

	// Test: All items are active
	result := All(items, func(item TestItem) bool {
		return item.Active
	})

	if result {
		t.Error("Expected false - last item is not active")
	}
}

func TestAnyWithComplexPredicate(t *testing.T) {
	items := []TestItem{
		{ID: 1, Name: "A", Price: 100, Active: true},
		{ID: 2, Name: "B", Price: 200, Active: false},
		{ID: 3, Name: "Premium", Price: 500, Active: true},
	}

	// Test: Any item is active AND price > 400 AND name starts with "P"
	result := Any(items, func(item TestItem) bool {
		return item.Active && item.Price > 400 && len(item.Name) > 0 && item.Name[0] == 'P'
	})

	if !result {
		t.Error("Expected true - Premium item matches all conditions")
	}
}

func TestAllWithComplexPredicate(t *testing.T) {
	items := []TestItem{
		{ID: 1, Name: "A", Price: 100, Active: true},
		{ID: 2, Name: "B", Price: 200, Active: true},
		{ID: 3, Name: "C", Price: 300, Active: true},
	}

	// Test: All items are active AND price >= 100
	result := All(items, func(item TestItem) bool {
		return item.Active && item.Price >= 100
	})

	if !result {
		t.Error("Expected true - all items match both conditions")
	}
}

func TestAnyWithIntegers(t *testing.T) {
	numbers := []int{1, 2, 3, 4, 5}

	// Test: Any number > 3
	result := Any(numbers, func(n int) bool {
		return n > 3
	})

	if !result {
		t.Error("Expected true - some numbers are > 3")
	}
}

func TestAllWithIntegers(t *testing.T) {
	numbers := []int{2, 4, 6, 8}

	// Test: All numbers are even
	result := All(numbers, func(n int) bool {
		return n%2 == 0
	})

	if !result {
		t.Error("Expected true - all numbers are even")
	}
}

func TestAnyWithStrings(t *testing.T) {
	words := []string{"apple", "banana", "cherry"}

	// Test: Any word starts with 'b'
	result := Any(words, func(word string) bool {
		return len(word) > 0 && word[0] == 'b'
	})

	if !result {
		t.Error("Expected true - 'banana' starts with 'b'")
	}
}

func TestAllWithStrings(t *testing.T) {
	words := []string{"apple", "apricot", "avocado"}

	// Test: All words start with 'a'
	result := All(words, func(word string) bool {
		return len(word) > 0 && word[0] == 'a'
	})

	if !result {
		t.Error("Expected true - all words start with 'a'")
	}
}
