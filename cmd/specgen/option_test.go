package main

import (
	"errors"
	"go/parser"
	"go/token"
	"testing"
)

// An Option inside a predicate: what its methods and its makers are in the
// tree. An Option is what it holds, or a null, to both readers of a tree.

const optionPath = `"github.com/krew-solutions/ascetic-ddd-go/asceticddd/option"`

// generatedOf returns the body generated for the one specification of a file
// with the imports given.
func generatedOf(t *testing.T, imports, body string) (string, error) {
	t.Helper()
	fset := token.NewFileSet()
	source := "package main\n" + imports + "\n" + spec(body)
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	specs := findSpecFunctions(fset, file, "Store")
	if len(specs) != 1 {
		t.Fatalf("found %d specifications", len(specs))
	}
	return visitorOf("Store", specs[0]).Visit(specs[0].Body)
}

const discount = `spec.Field(spec.GlobalScope(), "Discount")`

func TestAnOptionIsAsked(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"whether it holds nothing is the null test", "s.Discount.IsNothing()", "spec.IsNull(" + discount + ")"},
		{"whether it holds anything is the other", "s.Discount.IsSome()", "spec.IsNotNull(" + discount + ")"},
		{"negated", "!s.Discount.IsNothing()", "spec.Not(spec.IsNull(" + discount + "))"},
		{
			"what it holds is the member",
			"s.Discount.IsSome() && s.Discount.Unwrap() > 10",
			"spec.And(spec.IsNotNull(" + discount + "), spec.GreaterThan(" + discount + ", spec.Value(10)))",
		},
		{
			"what it holds is compared by its methods",
			"s.Discount.Unwrap().GreaterThan(limit)",
			"spec.GreaterThan(" + discount + ", spec.Value(limit))",
		},
		{
			"of an item",
			"Any(s.Items, func(item Item) bool { return item.Discount.IsNothing() })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Items"), spec.IsNull(spec.Field(spec.Item(), "Discount")))`,
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

func TestAnOptionIsMade(t *testing.T) {
	cases := []struct {
		name    string
		imports string
		body    string
		want    string
	}{
		{"some is what it holds", "import " + optionPath, "s.Discount == option.Some(5)", "spec.Equal(" + discount + ", spec.Value(5))"},
		{"nothing is the null", "import " + optionPath, "s.Discount == option.Nothing[int]()", "spec.IsNull(" + discount + ")"},
		{"on either side", "import " + optionPath, "option.Nothing[int]() != s.Discount", "spec.IsNotNull(" + discount + ")"},
		{"of a method too", "import " + optionPath, "s.Discount.Equal(option.Nothing[Money]())", "spec.IsNull(" + discount + ")"},
		{
			// What a name from outside is equal to is known when the tree is built.
			"some of a name from outside", "import " + optionPath, "s.Discount == option.Some(limit)",
			`spec.EqualityOrNullTest("=", ` + discount + ", spec.Value(limit))",
		},
		{"under the name the file gives the package", "import opt " + optionPath, "s.Discount == opt.Some(5)", "spec.Equal(" + discount + ", spec.Value(5))"},
		{"under no name", "import . " + optionPath, "s.Discount == Nothing[int]()", "spec.IsNull(" + discount + ")"},
		{
			"inside the predicate of a collection", "import " + optionPath,
			"Any(s.Items, func(item Item) bool { return item.Discount == option.Nothing[int]() })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Items"), spec.IsNull(spec.Field(spec.Item(), "Discount")))`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := generatedOf(t, c.imports, c.body)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if got != c.want {
				t.Errorf("\n got: %s\nwant: %s", got, c.want)
			}
		})
	}
}

// A maker is told by what the file imports, not by its spelling: another
// package's Some makes another Option, which no reader of a tree reads.
func TestAnotherPackagesSomeIsNotTheMaker(t *testing.T) {
	cases := []struct {
		name    string
		imports string
		body    string
	}{
		{"of another package", `import "github.com/samber/mo"`, "s.Discount == mo.Some(5)"},
		{"of another package called option", `import "example.com/other/option"`, "s.Discount == option.Some(5)"},
		{"of no package", "", "s.Discount == Some(5)"},
		{"a method called Some", "import " + optionPath, "s.Discount == s.Maker.Some(5)"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := generatedOf(t, c.imports, c.body)
			var refusal *UnsupportedError
			if !errors.As(err, &refusal) {
				t.Fatalf("generated %s, %v", got, err)
			}
		})
	}
}

// A parameter may be called what the package is: then `option.Some` is its
// method, and the package is out of reach of the predicate.
func TestANameDeclaredInTheSourceIsNotThePackage(t *testing.T) {
	fset := token.NewFileSet()
	source := "package main\nimport " + optionPath + "\n" +
		"//spec:sql\nfunc Spec(s Store, option Maker) bool {\n\treturn s.Discount == option.Some(5)\n}\n"
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	specs := findSpecFunctions(fset, file, "Store")
	if len(specs) != 1 {
		t.Fatalf("found %d specifications", len(specs))
	}
	got, err := visitorOf("Store", specs[0]).Visit(specs[0].Body)
	var refusal *UnsupportedError
	if !errors.As(err, &refusal) {
		t.Fatalf("generated %s, %v", got, err)
	}
}

// What an Option from outside holds is the Option, which is its value or the
// null. It used to be unwrapped when the tree was built, and panicked of a
// Nothing the predicate never unwraps: its guard comes first.
func TestWhatAnOptionFromOutsideHoldsIsTheOption(t *testing.T) {
	got, err := generatedOf(t, "", "limit.IsSome() && s.Discount.Unwrap() > limit.Unwrap()")
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	want := "spec.And(spec.IsNotNull(spec.Value(limit)), spec.GreaterThan(" + discount + ", spec.Value(limit)))"
	if got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

// What an Option holds is asked under a name: the null test and the predicate
// of the same Option, which makes the whole of two values, as it is in Go.
func TestWhatAnOptionHoldsIsAskedUnderAName(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			"of a member",
			"s.Discount.IsSomeAnd(func(d int) bool { return d > 10 })",
			"spec.And(spec.IsNotNull(" + discount + "), spec.GreaterThan(" + discount + ", spec.Value(10)))",
		},
		{
			"or it holds nothing",
			"s.Discount.IsNothingOr(func(d int) bool { return d > 10 })",
			"spec.Or(spec.IsNull(" + discount + "), spec.GreaterThan(" + discount + ", spec.Value(10)))",
		},
		{
			"a member of what is held",
			"s.Discount.IsSomeAnd(func(d Discount) bool { return d.Percent > 10 })",
			"spec.And(spec.IsNotNull(" + discount + `), spec.GreaterThan(spec.Field(spec.Object(spec.GlobalScope(), "Discount"), "Percent"), spec.Value(10)))`,
		},
		{
			"compared as what it is, not as a name from outside",
			"s.Discount.IsSomeAnd(func(d int) bool { return d == 10 })",
			"spec.And(spec.IsNotNull(" + discount + "), spec.Equal(" + discount + ", spec.Value(10)))",
		},
		{
			"the name is the nearest of that name",
			"s.Discount.IsSomeAnd(func(s int) bool { return s > 10 })",
			"spec.And(spec.IsNotNull(" + discount + "), spec.GreaterThan(" + discount + ", spec.Value(10)))",
		},
		{
			"of the member of an item beside the item",
			"Any(s.Items, func(item Item) bool { return item.Discount.IsSomeAnd(func(d int) bool { return d*10 > item.Price }) })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Items"), spec.And(spec.IsNotNull(spec.Field(spec.Item(), "Discount")), spec.GreaterThan(spec.Mul(spec.Field(spec.Item(), "Discount"), spec.Value(10)), spec.Field(spec.Item(), "Price"))))`,
		},
		{
			"of an Option from outside",
			"limit.IsSomeAnd(func(held int) bool { return s.Discount.IsSomeAnd(func(d int) bool { return d > held }) })",
			"spec.And(spec.IsNotNull(spec.Value(limit)), spec.And(spec.IsNotNull(" + discount + "), spec.GreaterThan(" + discount + ", spec.Value(limit))))",
		},
		{
			"what the candidate holds is in reach of the predicate of a collection",
			"s.Limit.IsSomeAnd(func(limit int) bool { return Any(s.Items, func(item Item) bool { return item.Price > limit }) })",
			`spec.And(spec.IsNotNull(spec.Field(spec.GlobalScope(), "Limit")), spec.Wildcard(spec.Object(spec.GlobalScope(), "Items"), spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Field(spec.GlobalScope(), "Limit"))))`,
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

func TestWhatAnOptionHoldsIsOutOfReachWhereTheOptionIs(t *testing.T) {
	for name, body := range map[string]string{
		"a predicate that is not written here":          "s.Discount.IsSomeAnd(over)",
		"a predicate of nothing":                        "s.Discount.IsSomeAnd(func() bool { return true })",
		"a predicate of two":                            "s.Discount.IsSomeAnd(func(a, b int) bool { return a > b })",
		"a predicate that does more":                    "s.Discount.IsSomeAnd(func(d int) bool { d++; return d > 10 })",
		"a predicate that does nothing":                 "s.Discount.IsSomeAnd(func(d int) bool {})",
		"no predicate":                                  "s.Discount.IsSomeAnd()",
		"an Option that is a call":                      "find(s).IsSomeAnd(func(d int) bool { return d > 10 })",
		"a member of what an Option from outside holds": "limit.IsSomeAnd(func(held Money) bool { return s.Price > held.Amount })",
		// What the item of an outer collection holds is a member of it.
		"what an outer item holds": "Any(s.Items, func(i Item) bool { return i.Discount.IsSomeAnd(func(d int) bool { return Any(i.Tags, func(t Tag) bool { return t.Weight > d }) }) })",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := generatedOf(t, "", body)
			var refusal *UnsupportedError
			if !errors.As(err, &refusal) {
				t.Fatalf("generated %s, %v", got, err)
			}
			if !refusal.Pos.IsValid() {
				t.Errorf("the refusal does not say where: %v", err)
			}
		})
	}
}

func TestWhatOfAnOptionHasNoNodeIsRefused(t *testing.T) {
	for _, body := range []string{
		"s.Discount.UnwrapOr(0) > 10",
		"s.Discount.IsNothing(1)",
		"s.Discount.Unwrap(1) > 10",
		"s.Discount == option.Some()",
		"s.Discount == option.Some(1, 2)",
		"s.Discount == option.Nothing[int](1)",
	} {
		t.Run(body, func(t *testing.T) {
			got, err := generatedOf(t, "import "+optionPath, body)
			var refusal *UnsupportedError
			if !errors.As(err, &refusal) {
				t.Fatalf("generated %s, %v", got, err)
			}
			if !refusal.Pos.IsValid() {
				t.Errorf("the refusal does not say where: %v", err)
			}
		})
	}
}
