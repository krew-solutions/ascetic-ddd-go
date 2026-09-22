package main

import (
	"bytes"
	"errors"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// Regression tests of the defects found while porting the package to Rust.

// generated returns the body generated for the one specification of the source.
func generated(t *testing.T, source string) (string, error) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", "package main\n"+source, parser.ParseComments)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	specs := findSpecFunctions(fset, file, "Store")
	if len(specs) != 1 {
		t.Fatalf("found %d specifications", len(specs))
	}
	return NewSpecGenVisitor("Store").withRoot(specs[0].Param).Visit(specs[0].Body)
}

func spec(body string) string {
	return "//spec:sql\nfunc Spec(s Store) bool {\n\treturn " + body + "\n}\n"
}

func TestGenerated(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			// All was generated as Any.
			"all means none fails",
			"All(s.Items, func(item Item) bool { return item.Active })",
			`spec.Not(spec.Wildcard(spec.Object(spec.GlobalScope(), "Items"), spec.Not(spec.Field(spec.Item(), "Active"))))`,
		},
		{
			"any is what it was",
			"Any(s.Items, func(item Item) bool { return item.Active })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Items"), spec.Field(spec.Item(), "Active"))`,
		},
		{
			// spec.Neg was generated before it existed.
			"a negation", "-s.Balance > 0",
			`spec.GreaterThan(spec.Neg(spec.Field(spec.GlobalScope(), "Balance")), spec.Value(0))`,
		},
		{
			"a negative literal is the constant it looks like", "s.Balance > -5",
			`spec.GreaterThan(spec.Field(spec.GlobalScope(), "Balance"), spec.Value(-5))`,
		},
		{
			"a unary plus is its operand", "+s.Balance > 0",
			`spec.GreaterThan(spec.Field(spec.GlobalScope(), "Balance"), spec.Value(0))`,
		},
		{
			// Written as it stands it is `a = NULL`, true of nothing.
			"equality with nil is the null test", "s.DeletedAt == nil",
			`spec.IsNull(spec.Field(spec.GlobalScope(), "DeletedAt"))`,
		},
		{
			"on either side", "nil != s.DeletedAt",
			`spec.IsNotNull(spec.Field(spec.GlobalScope(), "DeletedAt"))`,
		},
		{
			"of a method too", "s.Owner.Equal(nil)",
			`spec.IsNull(spec.Field(spec.GlobalScope(), "Owner"))`,
		},
		{
			// It was read as a field of the candidate of that name.
			"a name from outside is a value", "s.Balance > MinBalance",
			`spec.GreaterThan(spec.Field(spec.GlobalScope(), "Balance"), spec.Value(MinBalance))`,
		},
		{
			"equal to which is decided when the tree is built", "s.Owner == DefaultOwner",
			`spec.EqualityOrNullTest("=", spec.Field(spec.GlobalScope(), "Owner"), spec.Value(DefaultOwner))`,
		},
		{
			"the candidate inside a predicate",
			"Any(s.Items, func(item Item) bool { return item.Price > s.Limit })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Items"), spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Field(spec.GlobalScope(), "Limit")))`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := generated(t, spec(c.body))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("\ngot  %s\nwant %s", got, c.want)
			}
		})
	}
}

// What cannot be a specification was generated as spec.Value(nil) with a TODO
// in a comment: a specification that compiles, and is null.
func TestWhatCannotBeATreeIsReportedWhereItStands(t *testing.T) {
	cases := map[string]string{
		"a bitwise operator":       "s.Flags & 1 == 1",
		"a call":                   "len(s.Items) > 0",
		"an index":                 "s.Items[0].Active",
		"a composite literal":      "s.Owner == Owner{}",
		"a wrong predicate":        "Any(s.Items, isActive)",
		"inside a predicate":       "Any(s.Items, func(item Item) bool { return item.Flags | 1 == 1 })",
		"another name's member":    "other.Balance > 0",
		"the candidate as a value": "s == nil",
		"an outer item as a value": "Any(s.Categories, func(c Category) bool { return Any(c.Items, func(i Item) bool { return i.Owner == c }) })",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := generated(t, spec(body))
			var refused *UnsupportedError
			if !errors.As(err, &refused) {
				t.Fatalf("generated %q, %v", got, err)
			}
			if !refused.Pos.IsValid() {
				t.Errorf("the error has no position: %v", err)
			}
		})
	}
}

func TestARefusalLeavesNoGeneratedCode(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", "package main\n"+spec("s.Flags & 1 == 1"), parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var code bytes.Buffer
	err = renderCode(&code, "main", "Store", findSpecFunctions(fset, file, "Store"))
	if err == nil || !strings.Contains(err.Error(), "Spec") {
		t.Errorf("got %v, want an error naming the specification", err)
	}
	if strings.Contains(code.String(), "TODO") || strings.Contains(code.String(), "spec.Value(nil)") {
		t.Errorf("generated:\n%s", code.String())
	}
}

// A specification with a parameter is the usual kind: "dearer than this",
// "created since then". A function of more than its candidate was skipped
// with a warning, so a specification could name constants of the package alone.
func TestASpecificationTakesParameters(t *testing.T) {
	source := `package shop

import (
	"time"

	clock "example.com/clock"
	"example.com/unused"
)

var _ = unused.X

//spec:sql
func RecentAndDearSpec(s Store, since time.Time, min, max int, at clock.Instant, owner *string) bool {
	return s.CreatedAt > since && s.Price >= min && s.Price <= max && s.SeenAt < at && s.Owner == owner
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	specs := findSpecFunctions(fset, file, "Store")
	if len(specs) != 1 {
		t.Fatalf("found %d specifications", len(specs))
	}

	var code bytes.Buffer
	if err := renderCode(&code, "shop", "Store", specs); err != nil {
		t.Fatal(err)
	}
	generated := code.String()
	for _, want := range []string{
		// The parameters of the predicate, less its candidate.
		`func RecentAndDearSpecAST(since time.Time, min int, max int, at clock.Instant, owner *string) spec.Visitable {`,
		// They are values; equal to one of which is decided when the tree is built.
		`spec.GreaterThan(spec.Field(spec.GlobalScope(), "CreatedAt"), spec.Value(since))`,
		`spec.EqualityOrNullTest("=", spec.Field(spec.GlobalScope(), "Owner"), spec.Value(owner))`,
		// What their types are of is imported, as the source imports it.
		"\t\"time\"\n",
		"\tclock \"example.com/clock\"\n",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("the generated code lacks %q:\n%s", want, generated)
		}
	}
	if strings.Contains(generated, "example.com/unused") {
		t.Errorf("an import no parameter needs is generated:\n%s", generated)
	}
}

// The first `return` of the body was taken for the predicate and the rest of
// the body was not looked at: `if s.Deleted { return false }; return s.Price >
// 100` was generated as `Price > 100`, so the function refused the deleted and
// the query selected them.
func TestABodyOfMoreThanOneReturnIsRefused(t *testing.T) {
	for name, body := range map[string]string{
		"an early return":    "if s.Deleted {\n\t\treturn false\n\t}\n\treturn s.Price > 100",
		"a statement before": "min := 100\n\treturn s.Price > min",
		"no return":          "panic(s.Price)",
	} {
		t.Run(name, func(t *testing.T) {
			source := "package main\n//spec:sql\nfunc Spec(s Store) bool {\n\t" + body + "\n}\n"
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			var code bytes.Buffer
			err = renderCode(&code, "main", "Store", findSpecFunctions(fset, file, "Store"))
			var refused *UnsupportedError
			if !errors.As(err, &refused) {
				t.Fatalf("generated, %v:\n%s", err, code.String())
			}
			if !refused.Pos.IsValid() || !strings.Contains(err.Error(), "Spec") {
				t.Errorf("the error has no position or no name: %v", err)
			}
		})
	}
}

// A query cannot be written without knowing the table, and what a field of
// the struct is called there is the repository's to say. The generated
// `...SQL()` knew neither: it compiled the tree as it was, under the names of
// Go's fields - which found a column only while PostgreSQL folded an unquoted
// word of one part - and it made the package of the domain import the
// infrastructure. The tree is what is generated; a repository compiles it with
// its own context: specification.Compile(context, XAST(), WithSchema(schema)).
func TestOnlyTheTreeIsGenerated(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", "package main\n"+spec("s.Price > 100"), parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var code bytes.Buffer
	if err := renderCode(&code, "main", "Store", findSpecFunctions(fset, file, "Store")); err != nil {
		t.Fatal(err)
	}
	generated := code.String()
	if !strings.Contains(generated, "SpecAST() spec.Visitable {") {
		t.Errorf("the tree is not generated:\n%s", generated)
	}
	for _, unwanted := range []string{"SQL(", "CompileToSQL", "specification/infrastructure"} {
		if strings.Contains(generated, unwanted) {
			t.Errorf("the generated code has %q:\n%s", unwanted, generated)
		}
	}
}

func TestWhatIsNotASpecificationOfTheTypeIsStillSkipped(t *testing.T) {
	for name, signature := range map[string]string{
		"of another type":          "func Spec(u User, min int) bool",
		"a candidate without name": "func Spec(Store, int) bool",
		"a variadic parameter":     "func Spec(s Store, ids ...int) bool",
		"two candidates":           "func Spec(a, b Store) bool",
	} {
		t.Run(name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", "package main\n//spec:sql\n"+signature+" { return true }\n", parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			if specs := findSpecFunctions(fset, file, "Store"); len(specs) != 0 {
				t.Errorf("found %d specifications", len(specs))
			}
		})
	}
}

// A time is not compared with `<` in Go but with After and Before, which were
// "unsupported call": a predicate on a date could not be generated at all.
func TestATimeIsComparedByItsMethods(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			"after", "s.CreatedAt.After(since)",
			`spec.GreaterThan(spec.Field(spec.GlobalScope(), "CreatedAt"), spec.Value(since))`,
		},
		{
			"before", "s.CreatedAt.Before(until)",
			`spec.LessThan(spec.Field(spec.GlobalScope(), "CreatedAt"), spec.Value(until))`,
		},
		{
			"not after is not later than", "!s.CreatedAt.After(until)",
			`spec.Not(spec.GreaterThan(spec.Field(spec.GlobalScope(), "CreatedAt"), spec.Value(until)))`,
		},
		{
			"the value on the left", "since.Before(s.CreatedAt)",
			`spec.LessThan(spec.Value(since), spec.Field(spec.GlobalScope(), "CreatedAt"))`,
		},
		{
			// A nullable time is a pointer, and After of a nil one panics: the
			// guard is the null test, and stops the tree where it stops Go.
			"behind its guard", "s.DeletedAt != nil && s.DeletedAt.After(since)",
			`spec.And(spec.IsNotNull(spec.Field(spec.GlobalScope(), "DeletedAt")), spec.GreaterThan(spec.Field(spec.GlobalScope(), "DeletedAt"), spec.Value(since)))`,
		},
		{
			"of an item", "Any(s.Items, func(item Item) bool { return item.SoldAt.Before(s.ClosedAt) })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Items"), spec.LessThan(spec.Field(spec.Item(), "SoldAt"), spec.Field(spec.GlobalScope(), "ClosedAt")))`,
		},
		{
			"equal is what it was", "s.CreatedAt.Equal(since)",
			`spec.EqualityOrNullTest("=", spec.Field(spec.GlobalScope(), "CreatedAt"), spec.Value(since))`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := generated(t, spec(c.body))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("\ngot  %s\nwant %s", got, c.want)
			}
		})
	}

	// The clock is not a value of a specification: it comes as a parameter.
	for name, body := range map[string]string{
		"now":           "s.CreatedAt.After(time.Now())",
		"two arguments": "s.CreatedAt.After(since, until)",
		"no argument":   "s.CreatedAt.Before()",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := generated(t, spec(body))
			var refused *UnsupportedError
			if !errors.As(err, &refused) {
				t.Errorf("generated %q, %v", got, err)
			}
		})
	}
}
