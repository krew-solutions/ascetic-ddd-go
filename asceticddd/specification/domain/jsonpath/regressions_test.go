package jsonpath

import (
	"errors"
	"math"
	"strings"
	"testing"

	spec "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

// Regression tests of the defects found while porting the package to Rust.
// Each test names the defect it pins, and what the code did before the fix.

// BS is a backslash, spelled so that no tool between the author and the file
// reads an escape of its own in the templates below.
var BS = string(rune(92))

func field(name string) spec.FieldNode {
	return spec.Field(spec.GlobalScope(), name)
}

func item(name string) spec.FieldNode {
	return spec.Field(spec.Item(), name)
}

func bound(t *testing.T, template string, params ...any) spec.Visitable {
	t.Helper()
	parsed, err := Parse(template)
	if err != nil {
		t.Fatalf("Parse(%q): %v", template, err)
	}
	tree, err := parsed.Bind(params...)
	if err != nil {
		t.Fatalf("Bind(%q): %v", template, err)
	}
	return tree
}

func syntaxError(t *testing.T, template string) *JSONPathSyntaxError {
	t.Helper()
	parsed, err := Parse(template)
	if err == nil {
		tree, _ := parsed.Bind()
		t.Fatalf("Parse(%q) = %#v, want a syntax error", template, tree)
	}
	var syntax *JSONPathSyntaxError
	if !errors.As(err, &syntax) {
		t.Fatalf("Parse(%q): %v (%T), want a syntax error", template, err, err)
	}
	return syntax
}

func store(items ...map[string]any) spec.Context {
	contexts := make([]spec.Context, 0, len(items))
	for _, values := range items {
		contexts = append(contexts, NewDictContext(values))
	}
	return NewDictContext(map[string]any{
		"name":  "main",
		"limit": 100,
		"a":     1,
		"items": spec.NewCollectionContext(contexts),
	})
}

// Every token the grammar asks for is required, and nothing may follow the
// template. What the parser stepped over "if present" it did not miss where
// there was none.
func TestTheGrammarIsClosed(t *testing.T) {
	refused := map[string]string{
		"a bracket left open":                   "$[?@.age > 1",
		"a parenthesis left open":               "$[?(@.age > 1]",
		"a bracket closed twice":                "$[?@.age > 1]]",
		"a parenthesis closed twice":            "$[?(@.age > 1))]",
		"a filter without its question mark":    "$[@.age > 1]",
		"a template without its root":           "[?@.age > 1]",
		"a filter on a path without [*]":        "$.items[?@.price > 1]",
		"a nested filter without [*]":           "$[?@.items[?@.price > 1]]",
		"a name without @":                      "$[?age > 1]",
		"a comparison of a comparison":          "$[?@.a == 1 == 2]",
		"something after the template":          "$[?@.age > 1] @.b",
		"an operator without its right operand": "$[?@.age >]",
	}
	for name, template := range refused {
		t.Run(name, func(t *testing.T) {
			syntaxError(t, template)
		})
	}
}

// A parenthesis was closed by the first primary that met it, so
// `(a || b) && c` was read `a || (b && c)`.
func TestAParenthesisClosesItsOwnGroup(t *testing.T) {
	a, b, c := spec.Equal(field("a"), spec.Value(1)), spec.Equal(field("b"), spec.Value(2)), spec.Equal(field("c"), spec.Value(3))
	cases := map[string]spec.Visitable{
		"$[?(@.a == 1 || @.b == 2) && @.c == 3]":  spec.And(spec.Or(a, b), c),
		"$[?@.a == 1 || (@.b == 2 && @.c == 3)]":  spec.Or(a, spec.And(b, c)),
		"$[?!(@.a == 1 || @.b == 2) && @.c == 3]": spec.And(spec.Not(spec.Or(a, b)), c),
		"$[?((@.a == 1))]":                        a,
		"$[?!@.a == 1]":                           spec.Not(a),
		"$[?(@.a == 1) == (@.b == 2)]":            spec.Equal(a, b),
		"$[?@.active]":                            field("active"),
		"$[?!@.active]":                           spec.Not(field("active")),
	}
	for template, want := range cases {
		t.Run(template, func(t *testing.T) {
			if got := bound(t, template); got != want {
				t.Errorf("got  %#v\nwant %#v", got, want)
			}
		})
	}
}

// A path before a filter was dropped when it had no [*], and the filter
// applied to the candidate.
func TestAPathIsNotDropped(t *testing.T) {
	dear := spec.GreaterThan(item("price"), spec.Value(100))
	cases := map[string]spec.Visitable{
		"$.items[*][?@.price > 100]":           spec.Wildcard(spec.Object(spec.GlobalScope(), "items"), dear),
		"$.warehouse.items[*][?@.price > 100]": spec.Wildcard(spec.Object(spec.Object(spec.GlobalScope(), "warehouse"), "items"), dear),
		"$[?@.items[*][?@.price > 100]]":       spec.Wildcard(spec.Object(spec.GlobalScope(), "items"), dear),
		"$.categories[*][?@.items[*][?@.price > 100]]": spec.Wildcard(
			spec.Object(spec.GlobalScope(), "categories"),
			spec.Wildcard(spec.Object(spec.Item(), "items"), dear),
		),
	}
	for template, want := range cases {
		t.Run(template, func(t *testing.T) {
			if got := bound(t, template); got != want {
				t.Errorf("got  %#v\nwant %#v", got, want)
			}
		})
	}
}

// "$" is the candidate everywhere, and either side of a comparison is any operand.
func TestTheCandidateInsideAFilter(t *testing.T) {
	got := bound(t, "$.items[*][?@.price > $.limit]")
	want := spec.Wildcard(spec.Object(spec.GlobalScope(), "items"), spec.GreaterThan(item("price"), field("limit")))
	if got != want {
		t.Errorf("got  %#v\nwant %#v", got, want)
	}
	if got := bound(t, "$[?100 < @.price]"); got != spec.LessThan(spec.Value(100), field("price")) {
		t.Errorf("a value on the left: %#v", got)
	}

	parsed := MustParse("$.items[*][?@.price > $.limit]")
	for price, expected := range map[int]bool{500: true, 50: false} {
		matched, err := parsed.Match(store(map[string]any{"price": price}))
		if err != nil || matched != expected {
			t.Errorf("price %d: got %v, %v", price, matched, err)
		}
	}
}

// Named placeholders were listed before positional ones, whatever the order
// they stood in, so a mixed template bound the wrong parameters. Match takes
// the ones and MatchNamed the others: neither can bind a template of both.
func TestPlaceholdersOfOneStyle(t *testing.T) {
	syntax := syntaxError(t, "$[?@.a == %s && @.b == %(b)s]")
	// Where the second style is first met.
	if syntax.Message != "Positional and named placeholders in one template" || syntax.Position != 23 {
		t.Errorf("got %q at %d", syntax.Message, syntax.Position)
	}

	// They bind in the order they stand.
	got := bound(t, "$[?@.a == %d && @.items[*][?@.price > %f] && @.name == %s]", 1, 9.5, "main")
	want := spec.And(
		spec.Equal(field("a"), spec.Value(1)),
		spec.Wildcard(spec.Object(spec.GlobalScope(), "items"), spec.GreaterThan(item("price"), spec.Value(9.5))),
		spec.Equal(field("name"), spec.Value("main")),
	)
	if got != want {
		t.Errorf("got  %#v\nwant %#v", got, want)
	}

	// One inside a string literal is text.
	if got := bound(t, "$[?@.a == '%s' && @.b == %s]", 7); got != spec.And(
		spec.Equal(field("a"), spec.Value("%s")), spec.Equal(field("b"), spec.Value(7)),
	) {
		t.Errorf("got %#v", got)
	}
}

// A placeholder was a Value whose value was a marker, which every reader of
// the tree took for a value: a template that was not bound evaluated to false
// and compiled to a query with the marker for a parameter. A template is a
// function of its parameters: the tree comes of binding, with values in it.
func TestATemplateIsAFunctionOfItsParameters(t *testing.T) {
	parsed := MustParse("$[?@.age > %d]")

	t.Run("a template that is not bound is read by no one", func(t *testing.T) {
		_, err := parsed.Bind()
		var syntax *JSONPathSyntaxError
		if !errors.As(err, &syntax) || !strings.Contains(syntax.Message, "Missing positional parameter") {
			t.Errorf("got %v", err)
		}
		if matched, err := parsed.Match(NewDictContext(map[string]any{"age": 30})); err == nil {
			t.Errorf("matched without its parameter: %v", matched)
		}
		if _, err := MustParse("$[?@.name == %(name)s]").BindNamed(map[string]any{"other": 1}); err == nil {
			t.Errorf("bound without its named parameter")
		}
	})

	t.Run("a bound template has values", func(t *testing.T) {
		first, _ := parsed.Bind(25)
		if first != spec.GreaterThan(field("age"), spec.Value(25)) {
			t.Errorf("got %#v", first)
		}
		// The template is what it was: bound again, to something else.
		again, _ := parsed.Bind(65)
		if again != spec.GreaterThan(field("age"), spec.Value(65)) {
			t.Errorf("got %#v", again)
		}
	})

	t.Run("a placeholder under any operator is bound", func(t *testing.T) {
		template := MustParse("$[?%s == null || !(@.a == %d)]")
		for _, c := range []struct {
			first  any
			second int
			want   bool
		}{{nil, 1, true}, {5, 1, false}, {5, 2, true}} {
			matched, err := template.Match(store(), c.first, c.second)
			if err != nil || matched != c.want {
				t.Errorf("(%v, %v): got %v, %v, want %v", c.first, c.second, matched, err, c.want)
			}
		}
	})
}

// The letter of a placeholder was read and never used.
func TestAPlaceholderSaysWhatItTakes(t *testing.T) {
	refused := []struct {
		template string
		param    any
	}{
		{"$[?@.a == %d]", "1"},
		{"$[?@.a == %d]", 1.5},
		{"$[?@.a == %d]", true},
		{"$[?@.a == %f]", "1.5"},
		{"$[?@.a == %f]", true},
	}
	for _, c := range refused {
		_, err := MustParse(c.template).Bind(c.param)
		var typeError *JSONPathTypeError
		if !errors.As(err, &typeError) {
			t.Errorf("%s of %#v: got %v, want a type error", c.template, c.param, err)
		}
	}
	taken := []struct {
		template string
		param    any
	}{
		{"$[?@.a == %d]", 1},
		{"$[?@.a == %d]", int64(1)},
		{"$[?@.a == %f]", 1.5},
		{"$[?@.a == %f]", 1},
		{"$[?@.a == %s]", "1"},
		{"$[?@.a == %s]", struct{ id int }{1}},
		{"$[?@.a == %d]", nil},
		{"$[?@.a == %f]", nil},
	}
	for _, c := range taken {
		if _, err := MustParse(c.template).Bind(c.param); err != nil {
			t.Errorf("%s of %#v: %v", c.template, c.param, err)
		}
	}
}

// With SQL nulls `@.a == null` is null and true of nothing, so a template
// could not find a null. Equality with the null constant, spelled out or
// bound, is the null test. The rule is of constants, not of data.
func TestNullIsTestedNotCompared(t *testing.T) {
	deletedAt := field("deleted_at")
	cases := []struct {
		template string
		params   []any
		want     spec.Visitable
	}{
		{"$[?@.deleted_at == null]", nil, spec.IsNull(deletedAt)},
		{"$[?null != @.deleted_at]", nil, spec.IsNotNull(deletedAt)},
		{"$[?@.deleted_at == %s]", []any{nil}, spec.IsNull(deletedAt)},
		{"$[?@.deleted_at != %s]", []any{nil}, spec.IsNotNull(deletedAt)},
		{"$[?@.deleted_at == %s]", []any{"now"}, spec.Equal(deletedAt, spec.Value("now"))},
		{"$[?@.deleted_at > null]", nil, spec.GreaterThan(deletedAt, spec.Value(nil))},
		{"$[?@.deleted_at == @.updated_at]", nil, spec.Equal(deletedAt, field("updated_at"))},
	}
	for _, c := range cases {
		t.Run(c.template, func(t *testing.T) {
			if got := bound(t, c.template, c.params...); got != c.want {
				t.Errorf("got  %#v\nwant %#v", got, c.want)
			}
		})
	}

	parsed := MustParse("$[?@.deleted_at == %s]")
	for value, expected := range map[any]bool{nil: true, "now": false} {
		matched, err := parsed.Match(NewDictContext(map[string]any{"deleted_at": value}), nil)
		if err != nil || matched != expected {
			t.Errorf("deleted_at %v: got %v, %v", value, matched, err)
		}
	}
	// A comparison that is null is not satisfied, and is not an error.
	matched, err := MustParse("$[?@.deleted_at > %s]").Match(NewDictContext(map[string]any{"deleted_at": nil}), "now")
	if err != nil || matched {
		t.Errorf("got %v, %v", matched, err)
	}
}

// A string was a quote, anything but that quote, and a quote: nothing could be
// escaped. A number had no exponent: `1e3` was the number 1 and a name. The
// Rust port reads both as RFC 9535 has them, so a template of one port was a
// syntax error in the other.
func TestLiteralsAreThoseOfRfc9535(t *testing.T) {
	parsedLiteral := func(t *testing.T, literal string) any {
		t.Helper()
		tree := bound(t, "$[?@.a == "+literal+"]")
		return tree.(spec.InfixNode).Right().(spec.ValueNode).Value()
	}

	strs := map[string]string{
		"'it" + BS + "'s'":                        "it's",
		`"say ` + BS + `"hi` + BS + `""`:          `say "hi"`,
		`'say "it` + BS + `'s"'`:                  `say "it's"`,
		"'a" + BS + BS + "b'":                     "a" + BS + "b",
		"'a" + BS + "/b'":                         "a/b",
		"'a" + BS + "nb" + BS + "tc" + BS + "rd'": "a\nb\tc\rd",
		"'" + BS + "b" + BS + "f'":                "\b\f",
		"'" + BS + "u0041" + BS + "u00e9'":        "A" + string(rune(0xE9)),
		"'" + BS + "uD83D" + BS + "uDE00'":        string(rune(0x1F600)),
		"'plain'":                                 "plain",
		"''":                                      "",
	}
	for literal, want := range strs {
		t.Run(literal, func(t *testing.T) {
			if got := parsedLiteral(t, literal); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}

	invalid := map[string]int{
		"'a" + BS + "qb'":                  15,
		"'" + BS + "u00zz'":                14,
		"'" + BS + "uD83Dx'":               14,
		"'" + BS + "uDE00'":                14,
		"'" + BS + "uDE00" + BS + "uD83D'": 14,
	}
	for literal, position := range invalid {
		t.Run(literal, func(t *testing.T) {
			syntax := syntaxError(t, "$[?@.name == "+literal+"]")
			if syntax.Message != "Invalid escape" || syntax.Position != position {
				t.Errorf("got %q at %d, want Invalid escape at %d", syntax.Message, syntax.Position, position)
			}
		})
	}

	if syntax := syntaxError(t, "$[?@.name == 'open]"); syntax.Message != "Unterminated string" || syntax.Position != 13 {
		t.Errorf("got %q at %d", syntax.Message, syntax.Position)
	}

	numbers := map[string]any{
		"30":      30,
		"-1":      -1,
		"1.5":     1.5,
		"1e3":     1000.0,
		"3E1":     30.0,
		"300e-1":  30.0,
		"-2.5e+2": -250.0,
	}
	for literal, want := range numbers {
		t.Run(literal, func(t *testing.T) {
			if got := parsedLiteral(t, literal); got != want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, want, want)
			}
		})
	}

	// A literal is a bigint or a double precision, as it is in the query. One
	// that did not fit was read as the greatest integer, its error dropped.
	for _, literal := range []string{"99999999999999999999", "-99999999999999999999", "1e999"} {
		t.Run(literal, func(t *testing.T) {
			syntax := syntaxError(t, "$[?@.a > "+literal+"]")
			if syntax.Message != "Number out of range" || syntax.Position != 9 {
				t.Errorf("got %q at %d", syntax.Message, syntax.Position)
			}
		})
	}
	if got := parsedLiteral(t, "9223372036854775807"); got != math.MaxInt64 {
		t.Errorf("got %v", got)
	}

	// Digits and the letters of a name are those of ASCII.
	three := string(rune(0x0663)) // ARABIC-INDIC DIGIT THREE
	for _, template := range []string{"$[?@.a > " + three + "]", "$[?@.a > 1e" + three + "]", "$[?@.a == %(n" + string(rune(0xE9)) + ")s]"} {
		syntaxError(t, template)
	}

	parsed := MustParse("$[?@.price > 1e3 && @.name == 'it" + BS + "'s']")
	for price, expected := range map[float64]bool{1500: true, 999: false} {
		matched, err := parsed.Match(NewDictContext(map[string]any{"price": price, "name": "it's"}))
		if err != nil || matched != expected {
			t.Errorf("price %v: got %v, %v", price, matched, err)
		}
	}
}
