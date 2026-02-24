package option

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSome(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		o := Some(42)
		assert.True(t, o.IsSome())
		assert.False(t, o.IsNothing())
		assert.Equal(t, 42, o.Unwrap())
	})

	t.Run("string", func(t *testing.T) {
		o := Some("hello")
		assert.True(t, o.IsSome())
		assert.Equal(t, "hello", o.Unwrap())
	})

	t.Run("zero value is valid", func(t *testing.T) {
		o := Some(0)
		assert.True(t, o.IsSome())
		assert.Equal(t, 0, o.Unwrap())
	})

	t.Run("empty string is valid", func(t *testing.T) {
		o := Some("")
		assert.True(t, o.IsSome())
		assert.Equal(t, "", o.Unwrap())
	})
}

func TestNothing(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		o := Nothing[int]()
		assert.True(t, o.IsNothing())
		assert.False(t, o.IsSome())
	})

	t.Run("string", func(t *testing.T) {
		o := Nothing[string]()
		assert.True(t, o.IsNothing())
	})
}

func TestUnwrap(t *testing.T) {
	t.Run("some returns value", func(t *testing.T) {
		assert.Equal(t, 42, Some(42).Unwrap())
	})

	t.Run("none panics", func(t *testing.T) {
		assert.PanicsWithValue(t, "called Unwrap on a Nothing Option", func() {
			Nothing[int]().Unwrap()
		})
	})
}

func TestUnwrapOr(t *testing.T) {
	t.Run("some returns value", func(t *testing.T) {
		assert.Equal(t, 42, Some(42).UnwrapOr(0))
	})

	t.Run("none returns default", func(t *testing.T) {
		assert.Equal(t, 99, Nothing[int]().UnwrapOr(99))
	})
}

func TestUnwrapOrElse(t *testing.T) {
	t.Run("some returns value without calling closure", func(t *testing.T) {
		called := false
		result := Some(42).UnwrapOrElse(func() int {
			called = true
			return 99
		})
		assert.Equal(t, 42, result)
		assert.False(t, called)
	})

	t.Run("none calls closure", func(t *testing.T) {
		result := Nothing[int]().UnwrapOrElse(func() int {
			return 99
		})
		assert.Equal(t, 99, result)
	})
}

func TestUnwrapOrZero(t *testing.T) {
	t.Run("some returns value", func(t *testing.T) {
		assert.Equal(t, 42, Some(42).UnwrapOrZero())
	})

	t.Run("none returns zero value", func(t *testing.T) {
		assert.Equal(t, 0, Nothing[int]().UnwrapOrZero())
		assert.Equal(t, "", Nothing[string]().UnwrapOrZero())
		assert.False(t, Nothing[bool]().UnwrapOrZero())
	})
}

func TestMap(t *testing.T) {
	t.Run("some applies function", func(t *testing.T) {
		result := Map(Some(42), func(v int) string {
			return "value: 42"
		})
		assert.True(t, result.IsSome())
		assert.Equal(t, "value: 42", result.Unwrap())
	})

	t.Run("none returns none", func(t *testing.T) {
		called := false
		result := Map(Nothing[int](), func(v int) string {
			called = true
			return "should not be called"
		})
		assert.True(t, result.IsNothing())
		assert.False(t, called)
	})
}

func TestMapOr(t *testing.T) {
	t.Run("some applies function", func(t *testing.T) {
		result := MapOr(Some(3), 0, func(v int) int {
			return v * v
		})
		assert.Equal(t, 9, result)
	})

	t.Run("none returns default", func(t *testing.T) {
		result := MapOr(Nothing[int](), 42, func(v int) int {
			return v * v
		})
		assert.Equal(t, 42, result)
	})
}

func TestAndThen(t *testing.T) {
	t.Run("some chains", func(t *testing.T) {
		result := AndThen(Some(2), func(v int) Option[int] {
			return Some(v * v)
		})
		assert.True(t, result.IsSome())
		assert.Equal(t, 4, result.Unwrap())
	})

	t.Run("some to none", func(t *testing.T) {
		result := AndThen(Some(0), func(v int) Option[int] {
			if v == 0 {
				return Nothing[int]()
			}
			return Some(100 / v)
		})
		assert.True(t, result.IsNothing())
	})

	t.Run("none short-circuits", func(t *testing.T) {
		called := false
		result := AndThen(Nothing[int](), func(v int) Option[int] {
			called = true
			return Some(v)
		})
		assert.True(t, result.IsNothing())
		assert.False(t, called)
	})
}

func TestOr(t *testing.T) {
	t.Run("some returns self", func(t *testing.T) {
		result := Some(42).Or(Some(99))
		assert.Equal(t, 42, result.Unwrap())
	})

	t.Run("none returns alternative", func(t *testing.T) {
		result := Nothing[int]().Or(Some(99))
		assert.Equal(t, 99, result.Unwrap())
	})

	t.Run("both none", func(t *testing.T) {
		result := Nothing[int]().Or(Nothing[int]())
		assert.True(t, result.IsNothing())
	})
}

func TestOrElse(t *testing.T) {
	t.Run("some returns self without calling closure", func(t *testing.T) {
		called := false
		result := Some(42).OrElse(func() Option[int] {
			called = true
			return Some(99)
		})
		assert.Equal(t, 42, result.Unwrap())
		assert.False(t, called)
	})

	t.Run("none calls closure", func(t *testing.T) {
		result := Nothing[int]().OrElse(func() Option[int] {
			return Some(99)
		})
		assert.Equal(t, 99, result.Unwrap())
	})
}

func TestString(t *testing.T) {
	t.Run("some", func(t *testing.T) {
		assert.Equal(t, "Some(42)", Some(42).String())
		assert.Equal(t, "Some(hello)", Some("hello").String())
	})

	t.Run("none", func(t *testing.T) {
		assert.Equal(t, "Nothing", Nothing[int]().String())
	})
}

func TestChaining(t *testing.T) {
	t.Run("map then and_then", func(t *testing.T) {
		doubled := Map(Some(5), func(v int) int { return v * 2 })
		result := AndThen(doubled, func(v int) Option[string] {
			if v > 5 {
				return Some("big")
			}
			return Some("small")
		})
		assert.Equal(t, "big", result.Unwrap())
	})

	t.Run("none propagates through chain", func(t *testing.T) {
		mapped := Map(Nothing[int](), func(v int) int { return v * 2 })
		result := AndThen(mapped, func(v int) Option[string] {
			return Some("should not reach")
		})
		assert.True(t, result.IsNothing())
	})
}

func TestStructType(t *testing.T) {
	type User struct {
		Name string
		Age  int
	}

	t.Run("some struct", func(t *testing.T) {
		user := User{Name: "Alice", Age: 30}
		o := Some(user)
		assert.True(t, o.IsSome())
		assert.Equal(t, user, o.Unwrap())
	})

	t.Run("map struct to field", func(t *testing.T) {
		user := User{Name: "Alice", Age: 30}
		name := Map(Some(user), func(u User) string { return u.Name })
		assert.Equal(t, "Alice", name.Unwrap())
	})
}
