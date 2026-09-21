package main

import (
	"testing"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/option"
	spec "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

type dealContext struct {
	deal Deal
}

func (c dealContext) Get(field string) (any, error) {
	switch field {
	case "Discount":
		return c.deal.Discount, nil
	default:
		return nil, nil
	}
}

// The function and the tree generated from it agree on a member that is an
// Option, held or not; and the tree compiles to the null test.
func TestAnOptionInAPredicate(t *testing.T) {
	five, nothing := option.Some(5), option.Nothing[int]()
	cases := []struct {
		name     string
		function func(Deal) bool
		tree     spec.Visitable
		sql      string
	}{
		{"undiscounted", UndiscountedSpec, UndiscountedSpecAST(), `"discount" IS NULL`},
		{"well discounted", WellDiscountedSpec, WellDiscountedSpecAST(), `"discount" IS NOT NULL AND "discount" > $1`},
		{"not well discounted", NotWellDiscountedSpec, NotWellDiscountedSpecAST(), `NOT ("discount" IS NOT NULL AND "discount" > $1)`},
		{"by five", func(d Deal) bool { return DiscountedBySpec(d, five) }, DiscountedBySpecAST(five), `"discount" = $1`},
		{"by nothing", func(d Deal) bool { return DiscountedBySpec(d, nothing) }, DiscountedBySpecAST(nothing), `"discount" IS NULL`},
		{"by some of five", DiscountedByFiveSpec, DiscountedByFiveSpecAST(), `"discount" = $1`},
		{"has a discount", HasDiscountSpec, HasDiscountSpecAST(), `"discount" IS NOT NULL`},
		{"over five", func(d Deal) bool { return DiscountedOverSpec(d, five) }, DiscountedOverSpecAST(five), `$1::bigint IS NOT NULL AND "discount" IS NOT NULL AND "discount" > $2`},
		{"over nothing", func(d Deal) bool { return DiscountedOverSpec(d, nothing) }, DiscountedOverSpecAST(nothing), `$1::text IS NOT NULL AND "discount" IS NOT NULL AND "discount" > $2`},
		{"not over five", func(d Deal) bool { return NotDiscountedOverSpec(d, five) }, NotDiscountedOverSpecAST(five), `NOT ($1::bigint IS NOT NULL AND "discount" IS NOT NULL AND "discount" > $2)`},
		{"not over nothing", func(d Deal) bool { return NotDiscountedOverSpec(d, nothing) }, NotDiscountedOverSpecAST(nothing), `NOT ($1::text IS NOT NULL AND "discount" IS NOT NULL AND "discount" > $2)`},
		{"undiscounted or under ten", UndiscountedOrUnderSpec, UndiscountedOrUnderSpecAST(), `"discount" IS NULL OR "discount" < $1`},
		{"guarded over five", func(d Deal) bool { return GuardedOverSpec(d, five) }, GuardedOverSpecAST(five), `$1::bigint IS NOT NULL AND "discount" IS NOT NULL AND "discount" > $2`},
		{"guarded over nothing", func(d Deal) bool { return GuardedOverSpec(d, nothing) }, GuardedOverSpecAST(nothing), `$1::text IS NOT NULL AND "discount" IS NOT NULL AND "discount" > $2`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			for _, discount := range []option.Option[int]{nothing, five, option.Some(11)} {
				deal := Deal{ID: 1, Discount: discount}
				got, err := spec.NewEvaluateVisitor(dealContext{deal}, operators.NewDefaultRegistry()).Evaluate(c.tree)
				if err != nil {
					t.Fatalf("of %v: %v", discount, err)
				}
				if want := c.function(deal); got != want {
					t.Errorf("of %v: the tree is %v, the function %v", discount, got, want)
				}
			}
			sql, _, err := dealSQL(c.tree)
			if err != nil {
				t.Fatal(err)
			}
			if sql != c.sql {
				t.Errorf("\n got: %s\nwant: %s", sql, c.sql)
			}
		})
	}
}
