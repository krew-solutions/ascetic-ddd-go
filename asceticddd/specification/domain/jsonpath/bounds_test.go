package jsonpath

import (
	"regexp/syntax"
	"strings"
	"testing"
	"time"

	spec "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

// What a text that is not trusted may cost: the time to read it, and a tree
// within its bounds. The Rust port has the same tests, tests/jsonpath.rs.

const tooDeep = "Expression is nested too deep"

// height returns the levels of the longest way down a tree, by its
// description: a field and a value are a level, an operator or a collection
// is one above what is under it.
func height(description spec.Description) int {
	under := func(i int) int { return height(description[i].(spec.Description)) }
	switch description[0] {
	case "prefix", "postfix", "any":
		return 1 + under(3)
	case "infix":
		return 1 + max(under(3), under(4))
	default:
		return 1
	}
}

func heightOf(t *testing.T, template string) int {
	t.Helper()
	return height(spec.Describe(bound(t, template)))
}

func refusedAsTooDeep(t *testing.T, template string) {
	t.Helper()
	if got := syntaxError(t, template).Message; got != tooDeep {
		t.Errorf("got %q, want %q", got, tooDeep)
	}
}

// groupsOnTheLeft nests groups on the left, each the first operand of a chain
// of `&&` that is the first operand of a chain of `||`. How deep the parser is
// grows by one with a group; how tall the tree is, by two chains.
func groupsOnTheLeft(levels int, links func(level int) int) string {
	inner := "@.a"
	for level := levels; level >= 1; level-- {
		n := links(level)
		inner = "(" + inner + strings.Repeat(" && @.a", n) + strings.Repeat(" || @.a", n) + ")"
	}
	return "$[?" + inner + "]"
}

// The engine is told a pattern is anchored by an anchor in front of the whole
// of it. `^a|^b` has one in each branch and none in front, so every token was
// looked for through all the text left, and a template was read in a time
// that grew as the square of its length: 14 KB in a second, 56 KB in thirteen.
func TestEveryTokenPatternIsAnchoredForTheEngine(t *testing.T) {
	for _, pattern := range tokenPatterns {
		parsed, err := syntax.Parse(pattern.Pattern.String(), syntax.Perl)
		if err != nil {
			t.Fatal(err)
		}
		program, err := syntax.Compile(parsed.Simplify())
		if err != nil {
			t.Fatal(err)
		}
		if program.StartCond()&syntax.EmptyBeginText == 0 {
			t.Errorf("%s: %s is not anchored at its start", pattern.Type, pattern.Pattern)
		}
	}
}

// Whatever the cause, the effect: a text of a quarter of a million tokens is
// read in the time it takes to read it. With the square it took minutes; the
// bound is a hundred times what it takes.
func TestALongTextIsReadInTheTimeItTakesToReadIt(t *testing.T) {
	long := "$[?@.a == %d" + strings.Repeat(" @.a %d", 30000) + "]"
	started := time.Now()
	if got := syntaxError(t, long).Message; got != "Expected ']'" {
		t.Errorf("got %q", got)
	}
	if took := time.Since(started); took > 5*time.Second {
		t.Errorf("took %s", took)
	}
}

// The bounds on height and nesting bound the shape of a tree and not the size
// of a text: a text of megabytes was lexed whole before the parser could
// refuse it, or accepted with a literal of megabytes. The length is the first
// thing looked at, in bytes of UTF-8, so a template is one in every port or in
// none.
func TestATemplateLongerThanTheBoundIsRefusedBeforeItIsRead(t *testing.T) {
	room := "$[?@.a == 1]"
	atTheBound := "$[?@.a == 1" + strings.Repeat(" ", MaxLength-len(room)) + "]"
	if len(atTheBound) != MaxLength {
		t.Fatalf("%d bytes", len(atTheBound))
	}
	if _, err := Parse(atTheBound); err != nil {
		t.Errorf("at the bound: %v", err)
	}
	over := syntaxError(t, atTheBound+" ")
	if over.Error() != "Template too long (at most 262144 bytes of UTF-8, this has 262145)" {
		t.Errorf("got %q", over.Error())
	}
	if _, err := Parse("$[?@.a == '" + strings.Repeat("\u00e9", 131072) + "']"); err == nil {
		t.Error("bytes, not characters")
	}
	started := time.Now()
	syntaxError(t, "$[?"+strings.Repeat("@.a == 1 && ", 400000)+"@.a == 1]")
	if took := time.Since(started); took > time.Second {
		t.Errorf("took %s", took)
	}
}

// balanced returns comparisons with a parameter each, two to the power of
// depth of them, grouped in halves: a wide tree that is neither tall nor deep.
func balanced(depth int) string {
	if depth == 0 {
		return "@.a == %d"
	}
	half := balanced(depth - 1)
	return "(" + half + " && " + half + ")"
}

// The builder of a placeholder's value counted the placeholders among the
// tokens before it, for each placeholder, so a template was parsed in a time
// that grew as the square of their number. A long text that is refused does
// not show it: it is refused before its placeholders are reached.
func TestATemplateOfManyParametersIsReadInTheTimeItTakesToReadIt(t *testing.T) {
	template := "$[?" + balanced(14) + "]" // 16384 of them, under the bound on the length
	started := time.Now()
	parsed, err := Parse(template)
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(started); took > 5*time.Second {
		t.Errorf("took %s", took)
	}
	params := make([]any, 1<<14)
	for i := range params {
		params[i] = 1
	}
	matched, err := parsed.Match(NewDictContext(map[string]any{"a": 1}), params...)
	if err != nil || !matched {
		t.Errorf("got %v, %v", matched, err)
	}
}

// There was no bound at all: a goroutine's stack grows, up to a gigabyte, and
// beyond it the program dies of a fatal error that no recover catches. A text
// of a few megabytes is a tree of millions of levels.
func TestTheBoundsAreHeldToTheLevel(t *testing.T) {
	links := func(operands int) string { return "@.a" + strings.Repeat(" && @.a", operands-1) }

	if got := heightOf(t, "$[?"+links(128)+"]"); got != 128 {
		t.Errorf("a chain of 128: %d levels", got)
	}
	refusedAsTooDeep(t, "$[?"+links(129)+"]")

	// The chain is the LEFT operand of the comparison: it was read before
	// anything knew of an operator over it.
	if got := heightOf(t, "$[?("+links(127)+") == true]"); got != 128 {
		t.Errorf("a comparison over a chain of 127: %d levels", got)
	}
	refusedAsTooDeep(t, "$[?("+links(128)+") == true]")

	nested := func(open, close string, times int) string {
		return "$[?" + strings.Repeat(open, times) + "@.a" + strings.Repeat(close, times) + "]"
	}
	for name, shape := range map[string][2]string{
		"groups":  {"(", ")"},
		"nots":    {"!", ""},
		"filters": {"@.items[*][?", "]"},
	} {
		if _, err := Parse(nested(shape[0], shape[1], 32)); err != nil {
			t.Errorf("32 %s: %v", name, err)
		}
		refusedAsTooDeep(t, nested(shape[0], shape[1], 33))
	}
}

// As long a chain after each group as a count of how deep the parser is would
// let through, had it been passed to the right operands alone - which is how
// the Rust port counted: sixteen thousand levels passed for 128.
func TestAGroupOnTheLeftCountsTowardsTheHeight(t *testing.T) {
	refusedAsTooDeep(t, groupsOnTheLeft(127, func(level int) int { return 128 - level }))
}

// Whatever the shape, a template is refused or its tree is within the bound.
func TestNoTreeOfATemplateIsTallerThanTheBound(t *testing.T) {
	shapes := []func(inner, links string) string{
		func(inner, links string) string { return "(" + inner + links + ")" },
		func(inner, links string) string { return "(@.a" + links + " && " + inner + ")" },
		func(inner, links string) string { return "!(" + inner + links + ")" },
		func(inner, links string) string { return "@.items[*][?" + inner + links + "]" },
	}
	accepted := 0
	for _, wrap := range shapes {
		for _, levels := range []int{1, 2, 3, 7, 20, 60, 127} {
			for _, length := range []int{0, 1, 5, 40, 63, 64, 126, 127, 128} {
				links := strings.Repeat(" && @.a", length) + strings.Repeat(" || @.a", length)
				inner := "@.a"
				for range levels {
					inner = wrap(inner, links)
				}
				parsed, err := Parse("$[?" + inner + "]")
				if err != nil {
					if !strings.Contains(err.Error(), tooDeep) {
						t.Fatalf("%d levels of %d: %v", levels, length, err)
					}
					continue
				}
				accepted++
				tree, err := parsed.Bind()
				if err != nil {
					t.Fatal(err)
				}
				if got := height(spec.Describe(tree)); got > 128 {
					t.Errorf("%d levels of %d: a tree of %d levels", levels, length, got)
				}
			}
		}
	}
	// Not all refused: the property is of trees that were made.
	if accepted < 50 {
		t.Errorf("%d accepted", accepted)
	}
}
