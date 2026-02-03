package specification

// Collection helper functions for use in specification predicates.
// These functions are recognized by specgen and converted to Wildcard nodes.

// Any returns true if at least one item in the collection satisfies the predicate.
// This is a marker function for code generation - it will be converted to Wildcard AST node.
//
// Example:
//
//	//spec:sql
//	func HasExpensiveItemsSpec(store Store) bool {
//	    return Any(store.Items, func(item Item) bool {
//	        return item.Price > 500
//	    })
//	}
//
// Generates: Wildcard(Object(GlobalScope(), "Items"), GreaterThan(Field(Item(), "Price"), Value(500)))
func Any[T any](collection []T, predicate func(T) bool) bool {
	for _, item := range collection {
		if predicate(item) {
			return true
		}
	}
	return false
}

// All returns true if all items in the collection satisfy the predicate.
// This is a marker function for code generation - it will be converted to Wildcard AST node.
//
// Example:
//
//	//spec:sql
//	func AllActiveItemsSpec(store Store) bool {
//	    return All(store.Items, func(item Item) bool {
//	        return item.Active
//	    })
//	}
//
// Generates: Wildcard(Object(GlobalScope(), "Items"), Field(Item(), "Active"))
func All[T any](collection []T, predicate func(T) bool) bool {
	for _, item := range collection {
		if !predicate(item) {
			return false
		}
	}
	return true
}
