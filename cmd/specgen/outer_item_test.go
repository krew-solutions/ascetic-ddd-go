package main

import "testing"

// The item of an enclosing collection is named from an inner predicate by
// how far out it is: spec.OuterItem(1) is the item one collection out. It
// used to be refused, and before that read as a field of the candidate.
func TestTheItemOfAnOuterCollectionIsNamedFromAnInnerPredicate(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			"one collection out",
			"Any(s.Categories, func(c Category) bool { return Any(c.Items, func(i Item) bool { return i.Price > c.Limit }) })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Categories"), spec.Wildcard(spec.Object(spec.Item(), "Items"), spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Field(spec.OuterItem(1), "Limit"))))`,
		},
		{
			"beside the candidate's",
			"Any(s.Categories, func(c Category) bool { return Any(c.Items, func(i Item) bool { return c.Limit < s.Limit }) })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Categories"), spec.Wildcard(spec.Object(spec.Item(), "Items"), spec.LessThan(spec.Field(spec.OuterItem(1), "Limit"), spec.Field(spec.GlobalScope(), "Limit"))))`,
		},
		{
			"two collections out, through a member",
			"Any(s.Categories, func(c Category) bool { return Any(c.Items, func(i Item) bool { return Any(i.Tags, func(t Tag) bool { return t.Weight > c.Limits.Max }) }) })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Categories"), spec.Wildcard(spec.Object(spec.Item(), "Items"), spec.Wildcard(spec.Object(spec.Item(), "Tags"), spec.GreaterThan(spec.Field(spec.Item(), "Weight"), spec.Field(spec.Object(spec.OuterItem(2), "Limits"), "Max")))))`,
		},
		{
			// What the outer item holds is a member of it, one collection out.
			"what an outer item holds",
			"Any(s.Items, func(i Item) bool { return i.Discount.IsSomeAnd(func(d int) bool { return Any(i.Tags, func(t Tag) bool { return t.Weight > d }) }) })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Items"), spec.And(spec.IsNotNull(spec.Field(spec.Item(), "Discount")), spec.Wildcard(spec.Object(spec.Item(), "Tags"), spec.GreaterThan(spec.Field(spec.Item(), "Weight"), spec.Field(spec.OuterItem(1), "Discount")))))`,
		},
		{
			// A collection of the outer item, from the inner predicate.
			"a collection of the outer item",
			"Any(s.Categories, func(c Category) bool { return Any(c.Items, func(i Item) bool { return Any(c.Tags, func(t Tag) bool { return t.Name == i.Name }) }) })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Categories"), spec.Wildcard(spec.Object(spec.Item(), "Items"), spec.Wildcard(spec.Object(spec.OuterItem(1), "Tags"), spec.Equal(spec.Field(spec.Item(), "Name"), spec.Field(spec.OuterItem(1), "Name")))))`,
		},
		{
			// A name is the nearest of that name.
			"an inner item named as the outer",
			"Any(s.Categories, func(c Category) bool { return Any(c.Items, func(c Item) bool { return c.Price > 1 }) })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Categories"), spec.Wildcard(spec.Object(spec.Item(), "Items"), spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Value(1))))`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := generatedOf(t, "", c.body)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if got != c.want {
				t.Errorf("\n got: %s\nwant: %s", got, c.want)
			}
		})
	}
}
