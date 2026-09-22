package specification

import (
	"reflect"
	"testing"
)

// The item of an enclosing collection is named from an inner predicate by how
// far out it is: OuterItem(1) is the item one collection out, as Item() - the
// nearest - is OuterItem(0). The evaluator used to keep one item.

// shop has categories, each with a limit and products of its own.
func shop() testContext {
	category := func(limit any, prices ...any) testContext {
		products := make([]Context, 0, len(prices))
		for _, price := range prices {
			products = append(products, testContext{"price": price})
		}
		return testContext{"limit": limit, "products": NewCollectionContext(products)}
	}
	return testContext{"limit": 50, "categories": NewCollectionContext([]Context{
		category(10, 5, 20), category(100, 30),
	})}
}

func TestTheItemOfAnEnclosingCollectionIsNamedByHowFarOutItIs(t *testing.T) {
	price := Field(Item(), "price")
	categoryLimit := Field(OuterItem(1), "limit")
	shopLimit := Field(GlobalScope(), "limit")
	overItsCategory := func(predicate Visitable) Visitable {
		return Wildcard(Object(GlobalScope(), "categories"), Wildcard(Object(Item(), "products"), predicate))
	}
	cases := []struct {
		name string
		spec Visitable
		want bool
	}{
		// 20 > 10 in the first category; 30 > 100 is not.
		{"over its category", overItsCategory(GreaterThan(price, categoryLimit)), true},
		// A product is priced over the shop's limit in neither.
		{"over the shop", overItsCategory(GreaterThan(price, shopLimit)), false},
		{"beside the shop's", overItsCategory(And(GreaterThan(price, categoryLimit), LessThan(categoryLimit, shopLimit))), true},
		// The nearest item is still the product; from the outer predicate the
		// category is the item, at depth 0.
		{"the nearest", overItsCategory(GreaterThan(price, Value(25))), true},
		{"the category, from its own predicate", Wildcard(Object(GlobalScope(), "categories"), GreaterThan(Field(Item(), "limit"), Value(50))), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := evaluated(t, shop(), c.spec)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
	if _, err := evaluated(t, shop(), Wildcard(Object(GlobalScope(), "categories"), GreaterThan(categoryLimit, Value(1)))); err == nil {
		t.Error("one collection deep there is no item further out")
	}
	if !reflect.DeepEqual(Describe(categoryLimit), Description{"field", Description{"@1"}, "limit"}) {
		t.Errorf("described as %v", Describe(categoryLimit))
	}
	if !SameTree(Item(), OuterItem(0)) || SameTree(Item(), OuterItem(1)) {
		t.Error("the depth is part of the item")
	}
}
