package public

import (
	"testing"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

// Regression tests of the defects found while porting the package to Rust.

// << and >> were typed as comparisons: their result was a Logical, which can
// be conjoined and cannot be added to.
func TestAShiftIsArithmetic(t *testing.T) {
	flags := MakeNumberField("flags")
	var shifted Mathematical = flags.Lshift(MakeNumberValue(2)).Add(MakeNumberValue(1))
	want := s.Add(s.LeftShift(Field("flags"), s.Value(2)), s.Value(1))
	if shifted.Delegate() != want {
		t.Errorf("got %#v", shifted.Delegate())
	}
	var _ Mathematical = MakeNumberField("flags").Rshift(MakeNumberValue(1))
}

// Equality with a null constant is the null test: `a = NULL` is null, and true
// of nothing.
func TestEqualityWithANullConstantIsTheNullTest(t *testing.T) {
	deletedAt := MakeNullDatetimeField("deleted_at")
	cases := map[string]struct{ got, want s.Visitable }{
		"eq null":  {deletedAt.Eq(MakeNullDatetimeValue(nil)).Delegate(), s.IsNull(Field("deleted_at"))},
		"ne null":  {deletedAt.Ne(MakeNullDatetimeValue(nil)).Delegate(), s.IsNotNull(Field("deleted_at"))},
		"null eq":  {MakeNullDatetimeValue(nil).Eq(deletedAt).Delegate(), s.IsNull(Field("deleted_at"))},
		"eq value": {MakeNumberField("age").Eq(MakeNumberValue(30)).Delegate(), s.Equal(Field("age"), s.Value(30))},
		"number eq null": {
			MakeNullNumberField("age").Eq(MakeNullNumberValue(nil)).Delegate(), s.IsNull(Field("age")),
		},
		"adapter eq null": {
			NewComparison(Field("a")).Eq(NewComparison(s.Value(nil))).Delegate(), s.IsNull(Field("a")),
		},
	}
	for name, c := range cases {
		if c.got != c.want {
			t.Errorf("%s: got %#v, want %#v", name, c.got, c.want)
		}
	}
}
