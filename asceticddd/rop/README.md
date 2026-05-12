# rop

Scott Wlaschin's **Railway-Oriented Programming** toolkit ported to Go.

- Original article: https://fsharpforfunandprofit.com/posts/recipe-part2/
- OCaml reference implementation: https://github.com/krew-solutions/trading-ml/tree/main/shared/lib/rop

The Failure branch carries a non-empty list of errors (not a single error), so
independent failures from parallel validations can accumulate without loss via
`Apply` / `Map2` / `Map3` / `Map4` / `Plus` / `And`.

## Mapping to Wlaschin's canon

| Wlaschin / OCaml         | Go (this package)              | Notes                                                          |
|--------------------------|--------------------------------|----------------------------------------------------------------|
| `succeed` / `return`     | `Succeed[A, Err]`              | Success constructor                                            |
| `fail`                   | `Fail[A, Err]`                 | Single-error failure constructor                               |
| —                        | `FailMany[A, Err]`             | Multi-error constructor; panics on empty (invariant)           |
| `either`                 | `Either[A, B, Err]`            | Core eliminator                                                |
| `map`                    | `Map[A, B, Err]`               | Functor map on the Success track                               |
| `bind` / `>>=`           | `AndThen[A, B, Err]` / `Bind`  | Monadic bind; short-circuits on first Failure (`Bind` = alias) |
| `apply` / `<*>`          | `Apply[A, B, Err]`             | Applicative; concatenates error lists on dual Failure          |
| `both`                   | — (subsumed by `Map2`)         | Pair-of-Results not idiomatic in Go                            |
| `let+` / `and+`          | `Map2`, `Map3`, `Map4`         | No syntactic sugar; fixed-arity helpers cover the common cases |
| `let*`                   | `AndThen` chains               | Sequential bind; no Go-side sugar                              |
| `switch`                 | `Switch[A, B, Err]`            | Lifts a plain function into a switch (always-success)          |
| `tee` / `tap`            | `Tee[A]`                       | Side-effect pass-through                                       |
| `tryCatch`               | `TryCatch[A, B, Err]`          | Lifts `func(A) (B, error)` into a switch — Go's failure model is `error`, not panic |
| `doubleMap` / `bimap`    | `DoubleMap[A, B, Err1, Err2]`  | Maps Success and each Failure element                          |
| `plus` / `++` / `<+>`    | `Plus[X, A, Err]`              | Combine two switches; pluggable success/failure mergers        |
| `&&&`                    | `And[X, A, Err]`               | Validation flavour of `Plus`: first success, concat failures   |
| `>=>`                    | `Compose[A, B, C, Err]`        | Kleisli composition of two switch functions                    |
| `>>`                     | `Pipe[A, B, C]`                | Plain function composition (`g ∘ f`) — Go's stdlib lacks `Fun.compose` |
| `>>=` / `<*>` / `<!>`    | — (no operator overloading)    | Use the named functions above                                  |
| `of_result`              | `FromError[A](v, err)`         | Bridge to Go's `(T, error)` idiom — `err != nil` → Failure     |
| —                        | `OfOption[A, Err](o, err)`     | Bridge to `asceticddd/option`: `Nothing` → Fail with given err |

## Methods on `Result` (no new type parameters)

Following the `asceticddd/option` vocabulary:

`IsOk`, `IsError`, `Errors`, `Unwrap`, `UnwrapOr`, `UnwrapOrElse`,
`UnwrapOrZero`, `Or`, `OrElse`, `String`.

Functions that introduce a new value type (`Map`, `AndThen`, `Apply`,
`MapN`, …) are package-level — Go does not allow methods to introduce type
parameters beyond those of the receiver.

## Why a list of errors

```go
form := rop.Map3(
    validateSymbol(""),     // → Fail("symbol required")
    validateSide("X"),      // → Fail("side must be BUY or SELL")
    validateQuantity(-1),   // → Fail("quantity must be > 0")
    func(s, side string, q int) Order { return Order{s, side, q} },
)
// form.Errors() == ["symbol required", "side must be BUY or SELL", "quantity must be > 0"]
```

All three problems are reported together. With a single-error failure branch
the user would have to fix-and-resubmit three times.

## Mixing accumulation with short-circuiting

Applicative phase accumulates; monadic phase short-circuits. Mix freely:

```go
order := rop.Map3(validateSymbol(s), validateSide(side), validateQuantity(q), buildOrder)
saved := rop.AndThen(order, persistToDb)    // skipped entirely if any validation failed
```

## See also

- [samber/mo](https://github.com/samber/mo) -- Monads and popular FP abstractions, powered by Go 1.18+ Generics (Option, Result, Either...)
- [Railway Oriented Programming in Golang](https://github.com/samber/cc-skills-golang/blob/main/skills/golang-samber-mo/references/monads-guide.md)
