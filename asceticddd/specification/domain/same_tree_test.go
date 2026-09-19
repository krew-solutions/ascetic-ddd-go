package specification

import (
	"math"
	"testing"
	"time"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

type money struct {
	amount   int
	currency string
}

func TestSameTree(t *testing.T) {
	a, b, c := "ann", "ann", "bob"
	var noName *string
	noon := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	name := Field(GlobalScope(), "name")
	items := Object(GlobalScope(), "items")

	same := []struct {
		name        string
		left, right Visitable
	}{
		{"plain values", Equal(name, Value("ann")), Equal(name, Value("ann"))},
		// `==` compares the addresses: two pointers to one content differ.
		{"two pointers to one content", Equal(name, Value(&a)), Equal(name, Value(&b))},
		{"a pointer and what it points at", Equal(name, Value(&a)), Equal(name, Value("ann"))},
		// `==` panics: comparing uncomparable type []uint8.
		{"slices", Value([]byte{1, 2}), Value([]byte{1, 2})},
		{"maps", Value(map[string]any{"k": 1}), Value(map[string]any{"k": 1})},
		{"a Value Object", Value(money{10, "EUR"}), Value(money{10, "EUR"})},
		{"a pointer to a Value Object", Value(&money{10, "EUR"}), Value(&money{10, "EUR"})},
		{"nulls", Value(nil), Value(nil)},
		{"a nil pointer is a null", Value(noName), Value(nil)},
		{"a nil slice is a null", Value([]byte(nil)), Value(noName)},
		{"a NaN is itself", Value(math.NaN()), Value(math.NaN())},
		{"an instant, whatever its zone", Value(noon), Value(noon.In(time.FixedZone("x", 3600)))},
		{"nested paths", Field(Object(items, "owner"), "name"), Field(Object(Object(GlobalScope(), "items"), "owner"), "name")},
		{
			"a collection",
			Wildcard(items, And(Field(Item(), "active"), GreaterThan(Field(Item(), "price"), Value(5)))),
			Wildcard(Object(GlobalScope(), "items"), And(Field(Item(), "active"), GreaterThan(Field(Item(), "price"), Value(5)))),
		},
		{"prefix and postfix", Not(IsNull(Neg(name))), Not(IsNull(Neg(name)))},
		{"no trees", nil, nil},
	}
	for _, c := range same {
		t.Run("same: "+c.name, func(t *testing.T) {
			if !SameTree(c.left, c.right) || !SameTree(c.right, c.left) {
				t.Errorf("%#v is not the same as %#v", c.left, c.right)
			}
		})
	}

	different := []struct {
		name        string
		left, right Visitable
	}{
		{"values", Value("ann"), Value("bob")},
		{"pointers to different content", Value(&a), Value(&c)},
		// The query is not the same: a bigint is not a double precision.
		{"an integer and a float", Value(7), Value(7.0)},
		{"an int and an int64", Value(1), Value(int64(1))},
		{"a null and a value", Value(nil), Value(0)},
		{"an empty slice and a nil one", Value([]byte{}), Value([]byte(nil))},
		{"slices", Value([]byte{1}), Value([]byte{2})},
		{"Value Objects", Value(money{10, "EUR"}), Value(money{10, "USD"})},
		{"operators", Equal(name, Value(1)), NotEqual(name, Value(1))},
		{"the order of operands", LessThan(name, Value(1)), LessThan(Value(1), name)},
		{"kinds of node", IsNull(name), Not(name)},
		{"a field and a value", name, Value("name")},
		{"names", name, Field(GlobalScope(), "age")},
		{"paths", Field(Object(GlobalScope(), "a"), "x"), Field(Object(GlobalScope(), "b"), "x")},
		{"what a path starts at", Field(Item(), "name"), name},
		{"collections", Wildcard(items, name), Wildcard(Object(GlobalScope(), "parts"), name)},
		{"predicates", Wildcard(items, name), Wildcard(items, Not(name))},
		{
			"associativity",
			NewInfixNode(name, operators.OperatorSub, Value(1), LeftAssociative),
			NewInfixNode(name, operators.OperatorSub, Value(1), RightAssociative),
		},
		{"a tree and none", name, nil},
	}
	for _, c := range different {
		t.Run("different: "+c.name, func(t *testing.T) {
			if SameTree(c.left, c.right) || SameTree(c.right, c.left) {
				t.Errorf("%#v is the same as %#v", c.left, c.right)
			}
		})
	}
}

func TestDescribe(t *testing.T) {
	a := "ann"
	var noName *string
	name := Field(Object(GlobalScope(), "owner"), "name")
	cases := []struct {
		node Visitable
		want string
	}{
		{Equal(name, Value(&a)), `(infix = NON (field (object ($) owner) name) (value "ann"))`},
		{IsNull(Value(noName)), `(postfix IS NULL NON (value NULL))`},
		{Not(Field(Item(), "active")), `(prefix NOT RIGHT (field (@) active))`},
		{
			Wildcard(Object(GlobalScope(), "items"), GreaterThan(Field(Item(), "price"), Value(int64(5)))),
			`(any (object ($) items) * (infix > NON (field (@) price) (value int64(5))))`,
		},
		{Value(7.5), `(value 7.5)`},
		{Value(math.NaN()), `(value specification.notANumber({}))`},
		{nil, `<no tree>`},
	}
	for _, c := range cases {
		if got := Describe(c.node).String(); got != c.want {
			t.Errorf("got  %s\nwant %s", got, c.want)
		}
	}
}

// foreign is a node from outside the package, which Accept does not know.
type foreign struct {
	VisitableMarker
	id int
}

func TestANodeThatIsNotKnownIsDescribedByWhatItIs(t *testing.T) {
	if !SameTree(foreign{id: 1}, foreign{id: 1}) || SameTree(foreign{id: 1}, foreign{id: 2}) {
		t.Errorf("got %v and %v", Describe(foreign{id: 1}), Describe(foreign{id: 2}))
	}
	if SameTree(Not(foreign{id: 1}), Not(foreign{id: 2})) {
		t.Errorf("under an operator: %v", Describe(Not(foreign{id: 1})))
	}
}
