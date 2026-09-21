package specification

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/option"
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

// A member of an aggregate may be an Option of a value, and so may a constant
// of a specification. The evaluator took the wrapper for the value: there is
// no operator of an Option, and IS NULL was false of a Nothing. It is read as
// what it holds, or as a null, where a value comes to the evaluator: from the
// candidate, and from a constant.
func TestAnOptionIsWhatItHoldsOrANull(t *testing.T) {
	price := Field(GlobalScope(), "price")
	fifteen, nothing := option.Some(15), option.Nothing[int]()
	for _, c := range []struct {
		name string
		ctx  testContext
		node Visitable
		want any
	}{
		{"a member", testContext{"price": fifteen}, GreaterThan(price, Value(10)), true},
		{"a member, equal", testContext{"price": fifteen}, Equal(price, Value(15)), true},
		{"a member that is there is not null", testContext{"price": fifteen}, IsNull(price), false},
		// A Nothing is a null: unknown to a comparison, and to its negation.
		{"nothing, compared", testContext{"price": nothing}, GreaterThan(price, Value(10)), nil},
		{"nothing, compared and negated", testContext{"price": nothing}, Not(GreaterThan(price, Value(10))), nil},
		{"nothing is null", testContext{"price": nothing}, IsNull(price), true},
		{"nothing is not not null", testContext{"price": nothing}, IsNotNull(price), false},
		{"a constant", testContext{"price": 15}, Equal(price, Value(fifteen)), true},
		{"a constant and a member", testContext{"price": fifteen}, Equal(price, Value(fifteen)), true},
		{"a constant that is nothing", testContext{"price": 15}, Equal(price, Value(nothing)), nil},
		{"IS of two nothings", testContext{"price": nothing}, Is(price, Value(nothing)), true},
		// One inside another is read through.
		{"nested", testContext{"price": option.Some(fifteen)}, Equal(price, Value(15)), true},
		{"nested nothing", testContext{"price": option.Some(nothing)}, IsNull(price), true},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := evaluated(t, c.ctx, c.node)
			if err != nil || got != c.want {
				t.Errorf("got %v, %v, want %v", got, err, c.want)
			}
		})
	}
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
		if !SameTree(c.got, c.want) {
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

// email is a Value Object that compares through a pointer receiver: the
// pointer is the value, and is not to be taken for its pointee.
type email struct{ address string }

func (e *email) Equal(other EqualOperand) bool {
	o, ok := other.(*email)
	return ok && e.address == o.address
}

// A nullable column is a pointer in Go, and the driver takes one: a nil
// pointer is NULL, another is what it points at. The evaluator took neither:
// IS NULL of a nil pointer was false, without an error, and a pointer did not
// compare with a value. The two readers disagreed on the usual shape of a row.
func TestAPointerIsAnOptionalValue(t *testing.T) {
	var noName *string
	var noFlag *bool
	name, age, active, inactive := "ann", 30, true, false
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	agePointer := &age
	row := testContext{
		"owner": noName, "name": &name, "age": &age, "age2": &agePointer,
		"active": &active, "inactive": &inactive, "flag": noFlag, "created_at": &since,
		"blob": []byte(nil), "tags": map[string]any(nil), "empty": []byte{},
	}
	f := func(name string) FieldNode { return Field(GlobalScope(), name) }

	cases := []struct {
		name string
		node Visitable
		want any
	}{
		{"a nil pointer is null", IsNull(f("owner")), true},
		{"a nil pointer is not not-null", IsNotNull(f("owner")), false},
		{"a pointer is not null", IsNull(f("name")), false},
		{"a nil slice is null, as the driver sends it", IsNull(f("blob")), true},
		{"a nil map is null", IsNull(f("tags")), true},
		{"an empty slice is not", IsNull(f("empty")), false},
		{"null propagates", Equal(f("owner"), Value("ann")), nil},
		{"a pointer compares with a value", Equal(f("name"), Value("ann")), true},
		{"and a value with a pointer", NotEqual(Value("bob"), f("name")), true},
		{"a pointer computes", Add(f("age"), Value(1)), 31},
		{"a pointer to a pointer", GreaterThan(f("age2"), Value(18)), true},
		{"a pointer to a time", GreaterThan(f("created_at"), Value(since.Add(-time.Hour))), true},
		{"two pointers", Equal(f("age"), f("age2")), true},
		{"IS of two nil pointers", Is(f("owner"), Value(noName)), true},
		{"IS of a nil pointer and a value", Is(f("owner"), Value("ann")), false},
		{"a pointer to a bool is a condition", And(f("active"), Not(f("inactive"))), true},
		{"false decides, through a pointer", And(f("inactive"), failing()), false},
		{"true decides, through a pointer", Or(f("active"), failing()), true},
		{"a nil condition is null", And(f("flag"), f("active")), nil},
		{"null AND false", And(f("flag"), f("inactive")), false},
		{"null OR true", Or(f("flag"), f("active")), true},
		{"NOT null", Not(f("flag")), nil},
		{"a constant that is a nil pointer", Equal(Value(noName), Value("ann")), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := evaluated(t, row, c.node)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("got %v (%T), want %v (%T)", got, got, c.want, c.want)
			}
		})
	}

	t.Run("the result of a specification", func(t *testing.T) {
		for field, want := range map[string]bool{"active": true, "inactive": false, "flag": false} {
			got, err := NewEvaluateVisitor(row, operators.NewDefaultRegistry()).Evaluate(f(field))
			if err != nil || got != want {
				t.Errorf("%s: got %v, %v, want %v", field, got, err, want)
			}
		}
	})

	t.Run("the predicate of a collection", func(t *testing.T) {
		items := testContext{"items": NewCollectionContext([]Context{
			testContext{"active": noFlag}, testContext{"active": &active},
		})}
		some := Wildcard(Object(GlobalScope(), "items"), Field(Item(), "active"))
		if got, err := evaluated(t, items, some); err != nil || got != true {
			t.Errorf("got %v, %v", got, err)
		}
	})

	t.Run("a pointer that is the value stays one", func(t *testing.T) {
		got, err := evaluated(t, row, Equal(Value(&email{"a@b"}), Value(&email{"a@b"})))
		if err != nil || got != true {
			t.Errorf("got %v, %v", got, err)
		}
		got, err = evaluated(t, row, Equal(Value(&email{"a@b"}), Value(&email{"c@d"})))
		if err != nil || got != false {
			t.Errorf("got %v, %v", got, err)
		}
	})

	t.Run("what is not defined is named by its types", func(t *testing.T) {
		_, err := evaluated(t, row, Add(f("name"), Value(1)))
		if err == nil || !strings.Contains(err.Error(), "*string") {
			t.Errorf("got %v", err)
		}
	})

	t.Run("equality with a nil pointer is the null test", func(t *testing.T) {
		if got := EqualityOrNullTest(operators.OperatorEq, f("owner"), Value(noName)); !SameTree(got, IsNull(f("owner"))) {
			t.Errorf("got %#v", got)
		}
		if got := EqualityOrNullTest(operators.OperatorNe, Value([]byte(nil)), f("blob")); !SameTree(got, IsNotNull(f("blob"))) {
			t.Errorf("got %#v", got)
		}
		if got := EqualityOrNullTest(operators.OperatorEq, f("name"), Value(&name)); !SameTree(got, Equal(f("name"), Value(&name))) {
			t.Errorf("got %#v", got)
		}
	})
}
