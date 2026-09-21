package main

//go:generate go run github.com/krew-solutions/ascetic-ddd-go/cmd/specgen -type=Deal

import "github.com/krew-solutions/ascetic-ddd-go/asceticddd/option"

// Deal has a member that may be absent: an Option, which is what it holds, or
// a null, to both readers of a tree.
type Deal struct {
	ID       int64
	Discount option.Option[int]
}

// UndiscountedSpec: to ask an Option whether it holds anything is the null test.
//
//spec:sql
func UndiscountedSpec(d Deal) bool {
	return d.Discount.IsNothing()
}

// WellDiscountedSpec: what an Option holds is the member itself.
//
//spec:sql
func WellDiscountedSpec(d Deal) bool {
	return d.Discount.IsSome() && d.Discount.Unwrap() > 10
}

// NotWellDiscountedSpec is true of a deal without a discount, to the function
// and to the tree: the null test is of two values, and guards the comparison.
//
//spec:sql
func NotWellDiscountedSpec(d Deal) bool {
	return !(d.Discount.IsSome() && d.Discount.Unwrap() > 10)
}

// DiscountedBySpec takes an Option from outside: whether it is the null test
// is known when the tree is built.
//
//spec:sql
func DiscountedBySpec(d Deal, by option.Option[int]) bool {
	return d.Discount == by
}

// DiscountedByFiveSpec: Some(x) is x.
//
//spec:sql
func DiscountedByFiveSpec(d Deal) bool {
	return d.Discount == option.Some(5)
}

// HasDiscountSpec: Nothing is the null, and equality with it the null test.
//
//spec:sql
func HasDiscountSpec(d Deal) bool {
	return d.Discount != option.Nothing[int]()
}

// DiscountedOverSpec asks what two Options hold under names, one of them from
// outside: the null test beside the predicate makes the whole of two values,
// and nothing is unwrapped.
//
//spec:sql
func DiscountedOverSpec(d Deal, limit option.Option[int]) bool {
	return limit.IsSomeAnd(func(limit int) bool {
		return d.Discount.IsSomeAnd(func(discount int) bool { return discount > limit })
	})
}

// NotDiscountedOverSpec is the same under a `!`: true of a deal without a
// discount, and of any deal where no limit is given.
//
//spec:sql
func NotDiscountedOverSpec(d Deal, limit option.Option[int]) bool {
	return !limit.IsSomeAnd(func(limit int) bool {
		return d.Discount.IsSomeAnd(func(discount int) bool { return discount > limit })
	})
}

// UndiscountedOrUnderSpec: there is no discount, or it is under ten.
//
//spec:sql
func UndiscountedOrUnderSpec(d Deal) bool {
	return d.Discount.IsNothingOr(func(discount int) bool { return discount < 10 })
}

// GuardedOverSpec unwraps an Option from outside behind its guard. What it
// holds is the Option, which is its value or the null - not what it holds when
// the tree is built, where a Nothing the function never unwraps would panic.
//
//spec:sql
func GuardedOverSpec(d Deal, limit option.Option[int]) bool {
	return limit.IsSome() && d.Discount.IsSome() && d.Discount.Unwrap() > limit.Unwrap()
}
