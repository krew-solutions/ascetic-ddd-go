package rop

import (
	"errors"
	"strings"
	"testing"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/option"
	"github.com/stretchr/testify/assert"
)

func TestSucceed(t *testing.T) {
	r := Succeed[int, string](42)
	assert.True(t, r.IsOk())
	assert.False(t, r.IsError())
	assert.Equal(t, 42, r.Unwrap())
	assert.Nil(t, r.Errors())
}

func TestFail(t *testing.T) {
	r := Fail[int, string]("bad")
	assert.False(t, r.IsOk())
	assert.True(t, r.IsError())
	assert.Equal(t, []string{"bad"}, r.Errors())
}

func TestFailMany(t *testing.T) {
	t.Run("non-empty", func(t *testing.T) {
		r := FailMany[int, string]([]string{"a", "b"})
		assert.True(t, r.IsError())
		assert.Equal(t, []string{"a", "b"}, r.Errors())
	})

	t.Run("empty panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "rop.FailMany: errs must be non-empty", func() {
			FailMany[int, string](nil)
		})
	})
}

func TestUnwrap(t *testing.T) {
	t.Run("ok returns value", func(t *testing.T) {
		assert.Equal(t, 42, Succeed[int, string](42).Unwrap())
	})

	t.Run("error panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "called Unwrap on a Failure Result", func() {
			Fail[int, string]("bad").Unwrap()
		})
	})
}

func TestUnwrapOr(t *testing.T) {
	assert.Equal(t, 42, Succeed[int, string](42).UnwrapOr(0))
	assert.Equal(t, 99, Fail[int, string]("bad").UnwrapOr(99))
}

func TestUnwrapOrElse(t *testing.T) {
	t.Run("ok skips closure", func(t *testing.T) {
		called := false
		v := Succeed[int, string](42).UnwrapOrElse(func(errs []string) int {
			called = true
			return 0
		})
		assert.Equal(t, 42, v)
		assert.False(t, called)
	})

	t.Run("error invokes closure with list", func(t *testing.T) {
		v := FailMany[int, string]([]string{"a", "b"}).UnwrapOrElse(func(errs []string) int {
			return len(errs)
		})
		assert.Equal(t, 2, v)
	})
}

func TestUnwrapOrZero(t *testing.T) {
	assert.Equal(t, 42, Succeed[int, string](42).UnwrapOrZero())
	assert.Equal(t, 0, Fail[int, string]("bad").UnwrapOrZero())
	assert.Equal(t, "", Fail[string, string]("bad").UnwrapOrZero())
}

func TestOr(t *testing.T) {
	t.Run("ok returns self", func(t *testing.T) {
		r := Succeed[int, string](1).Or(Succeed[int, string](2))
		assert.Equal(t, 1, r.Unwrap())
	})

	t.Run("error returns alt", func(t *testing.T) {
		r := Fail[int, string]("bad").Or(Succeed[int, string](2))
		assert.Equal(t, 2, r.Unwrap())
	})
}

func TestOrElse(t *testing.T) {
	t.Run("ok skips closure", func(t *testing.T) {
		called := false
		r := Succeed[int, string](1).OrElse(func(errs []string) Result[int, string] {
			called = true
			return Succeed[int, string](2)
		})
		assert.Equal(t, 1, r.Unwrap())
		assert.False(t, called)
	})

	t.Run("error invokes closure", func(t *testing.T) {
		r := Fail[int, string]("bad").OrElse(func(errs []string) Result[int, string] {
			return Succeed[int, string](len(errs))
		})
		assert.Equal(t, 1, r.Unwrap())
	})
}

func TestString(t *testing.T) {
	assert.Equal(t, "Ok(42)", Succeed[int, string](42).String())
	assert.Equal(t, "Error([a b])", FailMany[int, string]([]string{"a", "b"}).String())
}

func TestEither(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		out := Either(Succeed[int, string](42),
			func(v int) string { return "got value" },
			func(errs []string) string { return "got errors" },
		)
		assert.Equal(t, "got value", out)
	})

	t.Run("error", func(t *testing.T) {
		out := Either(Fail[int, string]("bad"),
			func(v int) string { return "got value" },
			func(errs []string) string { return strings.Join(errs, ",") },
		)
		assert.Equal(t, "bad", out)
	})
}

func TestMap(t *testing.T) {
	t.Run("ok applies function", func(t *testing.T) {
		r := Map(Succeed[int, string](3), func(v int) int { return v * v })
		assert.Equal(t, 9, r.Unwrap())
	})

	t.Run("error short-circuits", func(t *testing.T) {
		called := false
		r := Map(Fail[int, string]("bad"), func(v int) int {
			called = true
			return v
		})
		assert.True(t, r.IsError())
		assert.Equal(t, []string{"bad"}, r.Errors())
		assert.False(t, called)
	})

	t.Run("changes value type", func(t *testing.T) {
		r := Map(Succeed[int, string](42), func(v int) string { return "n=42" })
		assert.Equal(t, "n=42", r.Unwrap())
	})
}

func TestBind(t *testing.T) {
	// Bind is an alias for AndThen; smoke-test both paths.
	t.Run("ok chains", func(t *testing.T) {
		r := Bind(Succeed[int, string](3), func(v int) Result[int, string] {
			return Succeed[int, string](v * 2)
		})
		assert.Equal(t, 6, r.Unwrap())
	})

	t.Run("error short-circuits", func(t *testing.T) {
		called := false
		r := Bind(Fail[int, string]("upstream"), func(v int) Result[int, string] {
			called = true
			return Succeed[int, string](v)
		})
		assert.Equal(t, []string{"upstream"}, r.Errors())
		assert.False(t, called)
	})
}

func TestFromError(t *testing.T) {
	t.Run("nil error → success", func(t *testing.T) {
		r := FromError(42, nil)
		assert.True(t, r.IsOk())
		assert.Equal(t, 42, r.Unwrap())
	})

	t.Run("non-nil error → failure, value discarded", func(t *testing.T) {
		boom := errors.New("boom")
		r := FromError(42, boom)
		assert.True(t, r.IsError())
		assert.Equal(t, []error{boom}, r.Errors())
	})
}

func TestOfOption(t *testing.T) {
	t.Run("Some → Succeed", func(t *testing.T) {
		r := OfOption(option.Some(7), "missing")
		assert.True(t, r.IsOk())
		assert.Equal(t, 7, r.Unwrap())
	})

	t.Run("Nothing → Fail with provided error", func(t *testing.T) {
		r := OfOption(option.Nothing[int](), "missing")
		assert.True(t, r.IsError())
		assert.Equal(t, []string{"missing"}, r.Errors())
	})
}

func TestAndThen(t *testing.T) {
	t.Run("ok chains", func(t *testing.T) {
		r := AndThen(Succeed[int, string](3), func(v int) Result[int, string] {
			return Succeed[int, string](v * 2)
		})
		assert.Equal(t, 6, r.Unwrap())
	})

	t.Run("ok to error", func(t *testing.T) {
		r := AndThen(Succeed[int, string](0), func(v int) Result[int, string] {
			if v == 0 {
				return Fail[int, string]("div by zero")
			}
			return Succeed[int, string](100 / v)
		})
		assert.True(t, r.IsError())
		assert.Equal(t, []string{"div by zero"}, r.Errors())
	})

	t.Run("error short-circuits", func(t *testing.T) {
		called := false
		r := AndThen(Fail[int, string]("upstream"), func(v int) Result[int, string] {
			called = true
			return Succeed[int, string](v)
		})
		assert.True(t, r.IsError())
		assert.Equal(t, []string{"upstream"}, r.Errors())
		assert.False(t, called)
	})
}

func TestApply(t *testing.T) {
	add := func(a int) func(int) int {
		return func(b int) int { return a + b }
	}

	t.Run("ok ok", func(t *testing.T) {
		rf := Succeed[func(int) int, string](add(2))
		rx := Succeed[int, string](3)
		r := Apply(rf, rx)
		assert.Equal(t, 5, r.Unwrap())
	})

	t.Run("err ok", func(t *testing.T) {
		rf := Fail[func(int) int, string]("f bad")
		rx := Succeed[int, string](3)
		r := Apply(rf, rx)
		assert.Equal(t, []string{"f bad"}, r.Errors())
	})

	t.Run("ok err", func(t *testing.T) {
		rf := Succeed[func(int) int, string](add(2))
		rx := Fail[int, string]("x bad")
		r := Apply(rf, rx)
		assert.Equal(t, []string{"x bad"}, r.Errors())
	})

	t.Run("err err accumulates", func(t *testing.T) {
		rf := Fail[func(int) int, string]("f bad")
		rx := Fail[int, string]("x bad")
		r := Apply(rf, rx)
		assert.Equal(t, []string{"f bad", "x bad"}, r.Errors())
	})
}

func TestMap2(t *testing.T) {
	add := func(a, b int) int { return a + b }

	t.Run("ok ok", func(t *testing.T) {
		r := Map2(Succeed[int, string](2), Succeed[int, string](3), add)
		assert.Equal(t, 5, r.Unwrap())
	})

	t.Run("err ok", func(t *testing.T) {
		r := Map2(Fail[int, string]("a"), Succeed[int, string](3), add)
		assert.Equal(t, []string{"a"}, r.Errors())
	})

	t.Run("err err accumulates", func(t *testing.T) {
		r := Map2(Fail[int, string]("a"), Fail[int, string]("b"), add)
		assert.Equal(t, []string{"a", "b"}, r.Errors())
	})
}

func TestMap3(t *testing.T) {
	join := func(a, b, c int) int { return a + b + c }

	t.Run("all ok", func(t *testing.T) {
		r := Map3(Succeed[int, string](1), Succeed[int, string](2), Succeed[int, string](3), join)
		assert.Equal(t, 6, r.Unwrap())
	})

	t.Run("all error accumulates in order", func(t *testing.T) {
		r := Map3(
			Fail[int, string]("a"),
			Fail[int, string]("b"),
			Fail[int, string]("c"),
			join,
		)
		assert.Equal(t, []string{"a", "b", "c"}, r.Errors())
	})

	t.Run("middle error", func(t *testing.T) {
		r := Map3(
			Succeed[int, string](1),
			Fail[int, string]("b"),
			Succeed[int, string](3),
			join,
		)
		assert.Equal(t, []string{"b"}, r.Errors())
	})
}

func TestMap4(t *testing.T) {
	type Form struct {
		Symbol   string
		Side     string
		Quantity int
		Price    int
	}
	build := func(s, side string, q, p int) Form { return Form{s, side, q, p} }

	t.Run("all ok", func(t *testing.T) {
		r := Map4(
			Succeed[string, string]("BTCUSD"),
			Succeed[string, string]("BUY"),
			Succeed[int, string](10),
			Succeed[int, string](50000),
			build,
		)
		assert.Equal(t, Form{"BTCUSD", "BUY", 10, 50000}, r.Unwrap())
	})

	t.Run("three errors accumulate", func(t *testing.T) {
		r := Map4(
			Fail[string, string]("bad symbol"),
			Fail[string, string]("bad side"),
			Fail[int, string]("negative quantity"),
			Succeed[int, string](50000),
			build,
		)
		assert.Equal(t, []string{"bad symbol", "bad side", "negative quantity"}, r.Errors())
	})
}

func TestSwitch(t *testing.T) {
	double := Switch[int, int, string](func(v int) int { return v * 2 })
	r := double(21)
	assert.Equal(t, 42, r.Unwrap())
}

func TestTee(t *testing.T) {
	seen := 0
	out := Tee(func(v int) { seen = v }, 7)
	assert.Equal(t, 7, seen)
	assert.Equal(t, 7, out)
}

func TestTryCatch(t *testing.T) {
	safe := TryCatch(
		func(v int) (int, error) {
			if v == 0 {
				return 0, errors.New("div by zero")
			}
			return 100 / v, nil
		},
		func(err error) string { return "wrapped: " + err.Error() },
	)

	t.Run("nil error → success", func(t *testing.T) {
		assert.Equal(t, 50, safe(2).Unwrap())
	})

	t.Run("non-nil error → failure with transformed message", func(t *testing.T) {
		r := safe(0)
		assert.Equal(t, []string{"wrapped: div by zero"}, r.Errors())
	})
}

func TestDoubleMap(t *testing.T) {
	t.Run("success transforms value", func(t *testing.T) {
		r := DoubleMap(
			Succeed[int, string](3),
			func(v int) int { return v * v },
			func(s string) error { return errors.New(s) },
		)
		assert.Equal(t, 9, r.Unwrap())
	})

	t.Run("failure transforms every error", func(t *testing.T) {
		r := DoubleMap(
			FailMany[int, string]([]string{"a", "b"}),
			func(v int) int { return v },
			func(s string) string { return "E:" + s },
		)
		assert.Equal(t, []string{"E:a", "E:b"}, r.Errors())
	})
}

func TestPlus(t *testing.T) {
	addInts := func(a, b int) int { return a + b }
	concatErrs := func(a, b []string) []string {
		return append(append([]string{}, a...), b...)
	}

	t.Run("both ok merges values", func(t *testing.T) {
		p := Plus(addInts, concatErrs,
			func(x int) Result[int, string] { return Succeed[int, string](x) },
			func(x int) Result[int, string] { return Succeed[int, string](x * 10) },
		)
		assert.Equal(t, 33, p(3).Unwrap())
	})

	t.Run("both error merges lists", func(t *testing.T) {
		p := Plus(addInts, concatErrs,
			func(x int) Result[int, string] { return Fail[int, string]("a") },
			func(x int) Result[int, string] { return Fail[int, string]("b") },
		)
		assert.Equal(t, []string{"a", "b"}, p(0).Errors())
	})

	t.Run("one error returns its list", func(t *testing.T) {
		p := Plus(addInts, concatErrs,
			func(x int) Result[int, string] { return Succeed[int, string](x) },
			func(x int) Result[int, string] { return Fail[int, string]("b") },
		)
		assert.Equal(t, []string{"b"}, p(5).Errors())
	})
}

func TestAnd(t *testing.T) {
	notEmpty := func(s string) Result[string, string] {
		if s == "" {
			return Fail[string, string]("must not be empty")
		}
		return Succeed[string, string](s)
	}
	notTooLong := func(s string) Result[string, string] {
		if len(s) > 5 {
			return Fail[string, string]("must be <= 5 chars")
		}
		return Succeed[string, string](s)
	}
	validate := And(notEmpty, notTooLong)

	t.Run("both pass", func(t *testing.T) {
		assert.Equal(t, "ok", validate("ok").Unwrap())
	})

	t.Run("first fails", func(t *testing.T) {
		assert.Equal(t, []string{"must not be empty"}, validate("").Errors())
	})

	t.Run("second fails", func(t *testing.T) {
		assert.Equal(t, []string{"must be <= 5 chars"}, validate("toolong").Errors())
	})

	t.Run("both fail accumulate", func(t *testing.T) {
		// First switch fails on empty; we need both to fail on the same input.
		// Use validators that both fail on the same input.
		neg := func(n int) Result[int, string] {
			if n < 0 {
				return Fail[int, string]("negative")
			}
			return Succeed[int, string](n)
		}
		even := func(n int) Result[int, string] {
			if n%2 != 0 {
				return Fail[int, string]("odd")
			}
			return Succeed[int, string](n)
		}
		v := And(neg, even)
		assert.Equal(t, []string{"negative", "odd"}, v(-3).Errors())
	})
}

func TestPipe(t *testing.T) {
	addOne := func(n int) int { return n + 1 }
	toStr := func(n int) string { return strings.Repeat("x", n) }
	p := Pipe(addOne, toStr)
	assert.Equal(t, "xxx", p(2))
}

func TestCompose(t *testing.T) {
	parse := func(s string) Result[int, string] {
		if s == "bad" {
			return Fail[int, string]("parse failed")
		}
		return Succeed[int, string](len(s))
	}
	check := func(n int) Result[int, string] {
		if n == 0 {
			return Fail[int, string]("zero")
		}
		return Succeed[int, string](n * 2)
	}
	pipeline := Compose(parse, check)

	t.Run("success path", func(t *testing.T) {
		assert.Equal(t, 6, pipeline("foo").Unwrap())
	})

	t.Run("first stage fails short-circuits", func(t *testing.T) {
		assert.Equal(t, []string{"parse failed"}, pipeline("bad").Errors())
	})

	t.Run("second stage fails", func(t *testing.T) {
		assert.Equal(t, []string{"zero"}, pipeline("").Errors())
	})
}

func TestPipelineMixingAccumulationAndShortCircuit(t *testing.T) {
	type Form struct {
		Symbol   string
		Side     string
		Quantity int
	}
	validSymbol := func(s string) Result[string, string] {
		if s == "" {
			return Fail[string, string]("symbol required")
		}
		return Succeed[string, string](s)
	}
	validSide := func(s string) Result[string, string] {
		if s != "BUY" && s != "SELL" {
			return Fail[string, string]("side must be BUY or SELL")
		}
		return Succeed[string, string](s)
	}
	validQty := func(q int) Result[int, string] {
		if q <= 0 {
			return Fail[int, string]("quantity must be > 0")
		}
		return Succeed[int, string](q)
	}

	// Applicative phase: accumulate field-level errors.
	build := func(symbol, side string, qty int) Form { return Form{symbol, side, qty} }

	t.Run("all three fail — all errors reported", func(t *testing.T) {
		form := Map3(
			validSymbol(""),
			validSide("X"),
			validQty(-1),
			build,
		)
		assert.Equal(t, []string{
			"symbol required",
			"side must be BUY or SELL",
			"quantity must be > 0",
		}, form.Errors())
	})

	t.Run("validate then chain to monadic step", func(t *testing.T) {
		// Validation passes → continue with a step that depends on the result.
		form := Map3(
			validSymbol("BTCUSD"),
			validSide("BUY"),
			validQty(10),
			build,
		)
		final := AndThen(form, func(f Form) Result[string, string] {
			if f.Quantity > 100 {
				return Fail[string, string]("over limit")
			}
			return Succeed[string, string](f.Symbol + ":" + f.Side)
		})
		assert.Equal(t, "BTCUSD:BUY", final.Unwrap())
	})

	t.Run("validation fails → monadic step skipped", func(t *testing.T) {
		called := false
		form := Map3(
			validSymbol(""),
			validSide("BUY"),
			validQty(10),
			build,
		)
		final := AndThen(form, func(f Form) Result[string, string] {
			called = true
			return Succeed[string, string]("never")
		})
		assert.Equal(t, []string{"symbol required"}, final.Errors())
		assert.False(t, called)
	})
}
