package jsonpath

import (
	"regexp/syntax"
	"strings"
	"testing"
	"time"
)

// What a text that is not trusted may cost: the time to read it. The Rust
// port has the same test, tests/jsonpath.rs.

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
	long := "$[?@.a == %d" + strings.Repeat(" @.a %d", 60000) + "]"
	started := time.Now()
	if got := syntaxError(t, long).Message; got != "Expected ']'" {
		t.Errorf("got %q", got)
	}
	if took := time.Since(started); took > 5*time.Second {
		t.Errorf("took %s", took)
	}
}
