package specification

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

// Regression tests of the defects found while porting the package to Rust.
// A specification has two readers, the evaluator and PostgreSQL reading the
// query the same tree compiles to, and they must agree.

func evaluated(t *testing.T, ctx Context, node Visitable) (any, error) {
	t.Helper()
	return Accept[any](node, NewEvaluateVisitor(ctx, operators.NewDefaultRegistry()))
}

// failing is a Visitable that must not be evaluated.
func failing() Visitable {
	return Field(GlobalScope(), "not_there")
}

func TestNegIsAnOperatorOfItsOwn(t *testing.T) {
	got, err := evaluated(t, testContext{}, Neg(Value(5)))
	if err != nil {
		t.Fatalf("Neg(5): %v", err)
	}
	if got != -5 {
		t.Errorf("Neg(5) = %v, want -5", got)
	}
	got, err = evaluated(t, testContext{}, Sub(Value(10), Neg(Value(3))))
	if err != nil || got != 13 {
		t.Errorf("10 - (-3) = %v, %v, want 13", got, err)
	}
}

func TestAConnectiveStopsOnceItIsDecided(t *testing.T) {
	cases := []struct {
		name string
		node Visitable
		want any
	}{
		{"false AND (fails)", And(Value(false), failing()), false},
		{"true OR (fails)", Or(Value(true), failing()), true},
		{"a != 0 AND 10 / a > 1", And(NotEqual(Value(0), Value(0)), GreaterThan(Div(Value(10), Value(0)), Value(1))), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := evaluated(t, testContext{}, c.node)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
	// What is not decided by the left operand is decided by both.
	if _, err := evaluated(t, testContext{}, And(Value(true), failing())); !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("true AND (fails): got %v, want ErrKeyNotFound", err)
	}
}

func TestANullIsNotSatisfied(t *testing.T) {
	visitor := NewEvaluateVisitor(testContext{"a": nil}, operators.NewDefaultRegistry())
	got, err := visitor.Evaluate(Equal(Field(GlobalScope(), "a"), Value(1)))
	if err != nil {
		t.Fatalf("a row with a null condition is not selected, and is not an error: %v", err)
	}
	if got {
		t.Errorf("NULL = 1 is satisfied")
	}
}

func TestACollectionPredicateFollowsSql(t *testing.T) {
	items := func(values ...any) testContext {
		contexts := make([]Context, 0, len(values))
		for _, v := range values {
			contexts = append(contexts, testContext{"price": v})
		}
		return testContext{"items": NewCollectionContext(contexts)}
	}
	dear := Wildcard(Object(GlobalScope(), "items"), GreaterThan(Field(Item(), "price"), Value(100)))

	t.Run("a null predicate is no witness", func(t *testing.T) {
		got, err := evaluated(t, items(nil, 50), dear)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != false {
			t.Errorf("got %v, want false", got)
		}
	})
	t.Run("a null beside a witness", func(t *testing.T) {
		got, err := evaluated(t, items(nil, 500), dear)
		if err != nil || got != true {
			t.Errorf("got %v, %v, want true", got, err)
		}
	})
	t.Run("stops at the first witness", func(t *testing.T) {
		got, err := evaluated(t, items(500, "not a number"), dear)
		if err != nil || got != true {
			t.Errorf("got %v, %v, want true", got, err)
		}
	})
}

func TestIsIsEqualityInWhichNullIsAValue(t *testing.T) {
	cases := []struct {
		left, right any
		want        bool
	}{
		{nil, nil, true},
		{nil, 1, false},
		{1, nil, false},
		{1, 1, true},
		{1, 2, false},
		{"a", "a", true},
		{true, false, false},
	}
	for _, c := range cases {
		got, err := evaluated(t, testContext{}, Is(Value(c.left), Value(c.right)))
		if err != nil {
			t.Errorf("%v IS %v: %v", c.left, c.right, err)
			continue
		}
		if got != c.want {
			t.Errorf("%v IS %v = %v, want %v", c.left, c.right, got, c.want)
		}
	}
}

func TestArithmeticIsPostgresqls(t *testing.T) {
	min, max := math.MinInt64, math.MaxInt64
	values := []struct {
		name string
		node Visitable
		want any
	}{
		{"7 / 2", Div(Value(7), Value(2)), 3},
		{"-7 / 2", Div(Value(-7), Value(2)), -3},
		{"-7 % 2", Mod(Value(-7), Value(2)), -1},
		{"7 % -2", Mod(Value(7), Value(-2)), 1},
		{"MIN % -1", Mod(Value(min), Value(-1)), 0},
		{"1 << 3", LeftShift(Value(1), Value(3)), 8},
		{"1 << 64", LeftShift(Value(1), Value(64)), 1},
		{"1 << 65", LeftShift(Value(1), Value(65)), 2},
		{"1 << -1", LeftShift(Value(1), Value(-1)), min},
		{"1 << 63", LeftShift(Value(1), Value(63)), min},
		{"-8 >> 1", RightShift(Value(-8), Value(1)), -4},
		{"-8 >> 65", RightShift(Value(-8), Value(65)), -4},
		{"int64: 1 << 64", LeftShift(Value(int64(1)), Value(int64(64))), int64(1)},
		{"false < true", LessThan(Value(false), Value(true)), true},
		{"true >= true", GreaterThanEqual(Value(true), Value(true)), true},
		{"NaN = NaN", Equal(Value(math.NaN()), Value(math.NaN())), true},
		{"NaN != NaN", NotEqual(Value(math.NaN()), Value(math.NaN())), false},
		{"NaN > 1e300", GreaterThan(Value(math.NaN()), Value(1e300)), true},
		{"1.0 < NaN", LessThan(Value(1.0), Value(math.NaN())), true},
		{"NaN <= NaN", LessThanEqual(Value(math.NaN()), Value(math.NaN())), true},
		{"1.5 + 1.5", Add(Value(1.5), Value(1.5)), 3.0},
	}
	for _, c := range values {
		t.Run(c.name, func(t *testing.T) {
			got, err := evaluated(t, testContext{}, c.node)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, c.want, c.want)
			}
		})
	}

	failures := []struct {
		name string
		node Visitable
		want string
	}{
		{"MAX + 1", Add(Value(max), Value(1)), "out of range"},
		{"MIN - 1", Sub(Value(min), Value(1)), "out of range"},
		{"MAX * 2", Mul(Value(max), Value(2)), "out of range"},
		{"MIN * -1", Mul(Value(min), Value(-1)), "out of range"},
		{"MIN / -1", Div(Value(min), Value(-1)), "out of range"},
		{"-MIN", Neg(Value(min)), "out of range"},
		{"int64: MAX + 1", Add(Value(int64(max)), Value(int64(1))), "out of range"},
		{"1 / 0", Div(Value(1), Value(0)), "by zero"},
		{"1 % 0", Mod(Value(1), Value(0)), "by zero"},
		{"1.0 / 0.0", Div(Value(1.0), Value(0.0)), "by zero"},
		{"1e308 * 10.0", Mul(Value(1e308), Value(10.0)), "out of range"},
		{"1e308 + 1e308", Add(Value(1e308), Value(1e308)), "out of range"},
		{"'a' = 1", Equal(Value("a"), Value(1)), "not supported"},
		{"true + 1", Add(Value(true), Value(1)), "not supported"},
		{"'a' * 3", Mul(Value("a"), Value(3)), "not supported"},
		{"1.5 << 1.0", LeftShift(Value(1.5), Value(1.0)), "not supported"},
	}
	for _, c := range failures {
		t.Run(c.name, func(t *testing.T) {
			got, err := evaluated(t, testContext{}, c.node)
			if err == nil {
				t.Fatalf("got %v, want an error with %q", got, c.want)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("got error %q, want one with %q", err, c.want)
			}
		})
	}

	// An infinity given is an infinity kept: the error is of an overflow.
	got, err := evaluated(t, testContext{}, Add(Value(math.Inf(1)), Value(1.0)))
	if err != nil || got != math.Inf(1) {
		t.Errorf("inf + 1 = %v, %v, want +Inf", got, err)
	}
}

func TestThereIsNoUnaryPlus(t *testing.T) {
	// It had no constructor, and no frontend produced it: the generator's
	// `+x` is `x`. An operator has a node and a notation that writes it.
	reg := operators.NewDefaultRegistry()
	if _, err := reg.ExecUnary(operators.Operator("+pos"), 1); err == nil {
		t.Errorf("unary plus is still an operator of the registry")
	}
}

func TestEqualityWithTheNullConstantIsTheNullTest(t *testing.T) {
	a := Field(GlobalScope(), "a")
	cases := []struct {
		name string
		got  Visitable
		want Visitable
	}{
		{"a == null", EqualityOrNullTest(operators.OperatorEq, a, Value(nil)), IsNull(a)},
		{"null == a", EqualityOrNullTest(operators.OperatorEq, Value(nil), a), IsNull(a)},
		{"a != null", EqualityOrNullTest(operators.OperatorNe, a, Value(nil)), IsNotNull(a)},
		{"a == 1", EqualityOrNullTest(operators.OperatorEq, a, Value(1)), Equal(a, Value(1))},
		{"a > null", EqualityOrNullTest(operators.OperatorGt, a, Value(nil)), GreaterThan(a, Value(nil))},
		// The rule is of constants, not of data.
		{"a == b", EqualityOrNullTest(operators.OperatorEq, a, Field(GlobalScope(), "b")), Equal(a, Field(GlobalScope(), "b"))},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %#v, want %#v", c.name, c.got, c.want)
		}
	}
}

// PostgreSQL has one bigint and computes it with a double precision. Here an
// int is not an int64 and neither is a float64, and the registry is keyed by
// type: `1 + 0.5` was "not supported", and so was a literal of a template, an
// int, against a column the driver reads as an int64.
func TestNumbersOfDifferentTypesCompute(t *testing.T) {
	values := []struct {
		name string
		node Visitable
		want any
	}{
		{"1 + 0.5", Add(Value(1), Value(0.5)), 1.5},
		{"7.0 / 2", Div(Value(7.0), Value(2)), 3.5},
		{"2.5 * 4", Mul(Value(2.5), Value(4)), 10.0},
		{"1 = 1.0", Equal(Value(1), Value(1.0)), true},
		{"1 < 1.5", LessThan(Value(1), Value(1.5)), true},
		{"int64 > int", GreaterThan(Value(int64(30)), Value(25)), true},
		{"int = int64", Equal(Value(30), Value(int64(30))), true},
		{"int64 + int", Add(Value(int64(1)), Value(2)), int64(3)},
		{"int % int64", Mod(Value(7), Value(int64(-2))), int64(1)},
		{"int64 << int", LeftShift(Value(int64(1)), Value(64)), int64(1)},
		{"int64 + float", Add(Value(int64(1)), Value(0.5)), 1.5},
		{"NaN > int", GreaterThan(Value(math.NaN()), Value(1)), true},
	}
	for _, c := range values {
		t.Run(c.name, func(t *testing.T) {
			got, err := evaluated(t, testContext{}, c.node)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, c.want, c.want)
			}
		})
	}
	failures := []struct {
		name string
		node Visitable
		want string
	}{
		{"MAX + int64(1)", Add(Value(math.MaxInt64), Value(int64(1))), "out of range"},
		{"1 / 0.0", Div(Value(1), Value(0.0)), "by zero"},
		{"5.5 % 2", Mod(Value(5.5), Value(2)), "not supported"},
		{"1.5 << 1", LeftShift(Value(1.5), Value(1)), "not supported"},
	}
	for _, c := range failures {
		t.Run(c.name, func(t *testing.T) {
			got, err := evaluated(t, testContext{}, c.node)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, %v, want an error with %q", got, err, c.want)
			}
		})
	}
}
