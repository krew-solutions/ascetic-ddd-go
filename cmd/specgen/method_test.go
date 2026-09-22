package main

import (
	"bytes"
	"errors"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strings"
	"testing"
)

// A specification is a type the author writes, and its predicate a method:
// `IsSatisfiedBy` marked `//spec:sql`, of which `Expression` is generated.
// The receiver is the specification, and its fields are the constants.

// method returns the body generated for a specification written as a method.
func method(t *testing.T, receiver, body string) (string, error) {
	t.Helper()
	fset := token.NewFileSet()
	source := "package main\n//spec:sql\nfunc (" + receiver + ") IsSatisfiedBy(s Store) bool {\n\treturn " + body + "\n}\n"
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

const price = `spec.Field(spec.GlobalScope(), "Price")`

func TestTheFieldsOfTheReceiverAreTheConstants(t *testing.T) {
	cases := []struct {
		name     string
		receiver string
		body     string
		want     string
	}{
		{"a field is a value", "c Dearer", "s.Price > c.Min", "spec.GreaterThan(" + price + ", spec.Value(c.Min))"},
		{"reached by its fields", "c Dearer", "s.Price > c.Limits.Min", "spec.GreaterThan(" + price + ", spec.Value(c.Limits.Min))"},
		{"through a pointer", "c *Dearer", "s.Price > c.Min", "spec.GreaterThan(" + price + ", spec.Value(c.Min))"},
		{
			// Whether it is the null test is known when the tree is built.
			"equality with one is decided then", "c Dearer", "s.Owner == c.Owner",
			`spec.EqualityOrNullTest("=", spec.Field(spec.GlobalScope(), "Owner"), spec.Value(c.Owner))`,
		},
		{"of a method too", "c Dearer", "s.Owner.Equal(c.Owner)",
			`spec.EqualityOrNullTest("=", spec.Field(spec.GlobalScope(), "Owner"), spec.Value(c.Owner))`},
		{
			"an Option of it is asked what it holds", "c Dearer",
			"c.Min.IsSomeAnd(func(min int) bool { return s.Price > min })",
			"spec.And(spec.IsNotNull(spec.Value(c.Min)), spec.GreaterThan(" + price + ", spec.Value(c.Min)))",
		},
		{
			"inside the predicate of a collection", "c Dearer",
			"Any(s.Items, func(item Item) bool { return item.Price > c.Min })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Items"), spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Value(c.Min)))`,
		},
		{
			// A name is the nearest of that name.
			"an item named as the receiver is the item", "c Dearer",
			"Any(s.Items, func(c Item) bool { return c.Price > 1 })",
			`spec.Wildcard(spec.Object(spec.GlobalScope(), "Items"), spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Value(1)))`,
		},
		{
			"what is held under the receiver's name is what is held", "c Dearer",
			"s.Discount.IsSomeAnd(func(c int) bool { return c > 1 })",
			`spec.And(spec.IsNotNull(spec.Field(spec.GlobalScope(), "Discount")), spec.GreaterThan(spec.Field(spec.GlobalScope(), "Discount"), spec.Value(1)))`,
		},
		{
			// The generated file imports the package as `spec`: a receiver
			// of that name is written as `spec_` there.
			"a receiver called spec", "spec Dearer", "s.Price > spec.Min",
			"spec.GreaterThan(" + price + ", spec.Value(spec_.Min))",
		},
		{"a receiver without a name", "Dearer", "s.Price > 1", "spec.GreaterThan(" + price + ", spec.Value(1))"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := method(t, c.receiver, c.body)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if got != c.want {
				t.Errorf("\n got: %s\nwant: %s", got, c.want)
			}
		})
	}
}

func TestTheReceiverAsAWholeIsNotAValue(t *testing.T) {
	for name, body := range map[string]string{
		"compared":       "s.Owner == c",
		"asked":          "c.IsSomeAnd(func(d Dearer) bool { return s.Price > d.Min })",
		"its method":     "s.Price > c.Min()",
		"its collection": "Any(c.Items, func(item Item) bool { return item.Price > 1 })",
	} {
		t.Run(name, func(t *testing.T) {
			got, err := method(t, "c Dearer", body)
			var refusal *UnsupportedError
			if !errors.As(err, &refusal) {
				t.Fatalf("generated %s, %v", got, err)
			}
		})
	}
}

// A parameter called `spec` made a file that did not compile: `spec.Value(spec)`.
func TestAParameterCalledSpecIsRenamedInTheGeneratedCode(t *testing.T) {
	got, err := generated(t, "//spec:sql\nfunc Dearer(s Store, spec int) bool {\n\treturn s.Price > spec\n}\n")
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if want := "spec.GreaterThan(" + price + ", spec.Value(spec_))"; got != want {
		t.Errorf("\n got: %s\nwant: %s", got, want)
	}
}

func TestTheTreeOfAMethodIsItsExpression(t *testing.T) {
	source := `package shop

import "time"

type DearSince struct {
	Since time.Time
	Min   int
}

//spec:sql
func (spec *DearSince) IsSatisfiedBy(s Store) bool {
	return s.Price > spec.Min && s.CreatedAt.After(spec.Since)
}

//spec:sql
func Dearer(s Store, spec int) bool {
	return s.Price > spec
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	specs := findSpecFunctions(fset, file, "Store")
	if len(specs) != 2 {
		t.Fatalf("found %d specifications", len(specs))
	}
	var code bytes.Buffer
	if err := renderCode(&code, "shop", "Store", specs); err != nil {
		t.Fatal(err)
	}
	generated := code.String()
	for _, want := range []string{
		// The receiver as written, the method's name fixed: the pair is the
		// interface a repository takes.
		"func (spec_ *DearSince) Expression() spec.Visitable {\n",
		`spec.GreaterThan(spec.Field(spec.GlobalScope(), "Price"), spec.Value(spec_.Min))`,
		`spec.GreaterThan(spec.Field(spec.GlobalScope(), "CreatedAt"), spec.Value(spec_.Since))`,
		"func DearerAST(spec_ int) spec.Visitable {\n",
		"spec.Value(spec_))",
	} {
		if !strings.Contains(generated, want) {
			t.Errorf("the generated code lacks %q:\n%s", want, generated)
		}
	}
	if strings.Contains(generated, "IsSatisfiedByAST") || strings.Contains(generated, "\"time\"") {
		t.Errorf("generated as a function, or with an import no parameter needs:\n%s", generated)
	}
}

func TestWhatIsNotTheMethodOfASpecificationIsSkippedAndSaidSo(t *testing.T) {
	for name, signature := range map[string]string{
		"a method of another name":         "func (c Dearer) Holds(s Store) bool",
		"a parameter beside the candidate": "func (c Dearer) IsSatisfiedBy(s Store, min int) bool",
	} {
		t.Run(name, func(t *testing.T) {
			var logged bytes.Buffer
			log.SetOutput(&logged)
			defer log.SetOutput(os.Stderr)
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", "package main\n//spec:sql\n"+signature+" { return true }\n", parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			if specs := findSpecFunctions(fset, file, "Store"); len(specs) != 0 {
				t.Errorf("found %d specifications", len(specs))
			}
			if !strings.Contains(logged.String(), "IsSatisfiedBy") {
				t.Errorf("not said: %q", logged.String())
			}
		})
	}
	// Of another type: skipped as a function of another type is, in silence.
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", "package main\n//spec:sql\nfunc (c Dearer) IsSatisfiedBy(u User) bool { return true }\n", parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	if specs := findSpecFunctions(fset, file, "Store"); len(specs) != 0 {
		t.Errorf("found %d specifications", len(specs))
	}
}
