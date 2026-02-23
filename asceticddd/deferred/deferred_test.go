package deferred

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoop(t *testing.T) {
	t.Run("returns zero value for any input", func(t *testing.T) {
		result, err := Noop[int, any](42)
		assert.Nil(t, result)
		assert.NoError(t, err)

		sResult, sErr := Noop[string, string]("test")
		assert.Equal(t, "", sResult)
		assert.NoError(t, sErr)

		iResult, iErr := Noop[any, int](nil)
		assert.Equal(t, 0, iResult)
		assert.NoError(t, iErr)
	})
}

func TestDeferredBasics(t *testing.T) {
	t.Run("resolve triggers success handler", func(t *testing.T) {
		d := &DeferredImp[int]{}
		var result []int

		Then(d, func(value int) (int, error) {
			result = append(result, value)
			return value, nil
		}, Noop[error, int])
		d.Resolve(42)

		assert.Equal(t, []int{42}, result)
	})

	t.Run("reject triggers error handler", func(t *testing.T) {
		d := &DeferredImp[int]{}
		var result []error

		testError := errors.New("test error")
		Then(d, Noop[int, any], func(err error) (any, error) {
			result = append(result, err)
			return nil, nil
		})
		d.Reject(testError)

		assert.Equal(t, []error{testError}, result)
	})

	t.Run("resolve before then", func(t *testing.T) {
		d := &DeferredImp[int]{}
		var result []int

		d.Resolve(42)

		Then(d, func(value int) (int, error) {
			result = append(result, value)
			return value, nil
		}, Noop[error, int])

		assert.Equal(t, []int{42}, result)
	})

	t.Run("reject before then", func(t *testing.T) {
		d := &DeferredImp[int]{}
		var result []error

		testError := errors.New("test error")
		d.Reject(testError)

		Then(d, Noop[int, any], func(err error) (any, error) {
			result = append(result, err)
			return nil, nil
		})

		assert.Equal(t, []error{testError}, result)
	})
}

func TestDeferredChaining(t *testing.T) {
	t.Run("chain success handlers", func(t *testing.T) {
		d := &DeferredImp[int]{}
		var results []string

		d2 := Then(d, func(value int) (string, error) {
			results = append(results, fmt.Sprintf("handler1: %d", value))
			return fmt.Sprintf("transformed_%d", value), nil
		}, Noop[error, string])

		Then(d2, func(value string) (string, error) {
			results = append(results, fmt.Sprintf("handler2: %s", value))
			return value, nil
		}, Noop[error, string])

		d.Resolve(42)

		assert.Equal(t, []string{"handler1: 42", "handler2: transformed_42"}, results)
	})

	t.Run("chain with error propagation", func(t *testing.T) {
		d := &DeferredImp[int]{}
		var results []string

		testError := errors.New("test error")

		d2 := Then(d, func(value int) (string, error) {
			results = append(results, fmt.Sprintf("handler1: %d", value))
			return "", testError
		}, Noop[error, string])

		Then(d2, func(value string) (string, error) {
			results = append(results, "handler2: should not be called")
			return value, nil
		}, func(err error) (string, error) {
			results = append(results, fmt.Sprintf("error: %s", err))
			return "", nil
		})

		d.Resolve(42)

		assert.Equal(t, []string{"handler1: 42", fmt.Sprintf("error: %s", testError)}, results)
	})

	t.Run("multiple handlers on same deferred", func(t *testing.T) {
		d := &DeferredImp[int]{}
		var results []string

		Then(d, func(value int) (int, error) {
			results = append(results, fmt.Sprintf("handler1: %d", value))
			return value, nil
		}, Noop[error, int])

		Then(d, func(value int) (int, error) {
			results = append(results, fmt.Sprintf("handler2: %d", value))
			return value, nil
		}, Noop[error, int])

		d.Resolve(42)

		assert.Contains(t, results, "handler1: 42")
		assert.Contains(t, results, "handler2: 42")
		assert.Len(t, results, 2)
	})
}

func TestErrorCollection(t *testing.T) {
	t.Run("occurred err empty when no errors", func(t *testing.T) {
		d := &DeferredImp[int]{}

		Then(d, func(value int) (int, error) {
			return value, nil
		}, Noop[error, int])
		d.Resolve(42)

		assert.Nil(t, d.OccurredErr())
	})

	t.Run("occurred err collects handler errors", func(t *testing.T) {
		d := &DeferredImp[int]{}
		error1 := errors.New("error 1")

		Then(d, func(_ int) (int, error) {
			return 0, error1
		}, Noop[error, int])
		d.Resolve(42)

		err := d.OccurredErr()
		assert.Error(t, err)
		assert.ErrorIs(t, err, error1)
	})

	t.Run("occurred err collects nested errors", func(t *testing.T) {
		d := &DeferredImp[int]{}
		error1 := errors.New("error 1")
		error2 := errors.New("error 2")

		d2 := Then(d, func(_ int) (int, error) {
			return 0, error1
		}, Noop[error, int])

		Then(d2, Noop[int, int], func(err error) (int, error) {
			return 0, error2
		})
		d.Resolve(42)

		err := d.OccurredErr()
		assert.Error(t, err)
		assert.ErrorIs(t, err, error1)
		assert.ErrorIs(t, err, error2)
	})

	t.Run("occurred err with multiple branches", func(t *testing.T) {
		d := &DeferredImp[int]{}
		error1 := errors.New("error 1")
		error2 := errors.New("error 2")

		Then(d, func(_ int) (int, error) {
			return 0, error1
		}, Noop[error, int])

		Then(d, func(_ int) (int, error) {
			return 0, error2
		}, Noop[error, int])

		d.Resolve(42)

		err := d.OccurredErr()
		assert.Error(t, err)
		assert.ErrorIs(t, err, error1)
		assert.ErrorIs(t, err, error2)
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("resolve with nil", func(t *testing.T) {
		d := &DeferredImp[any]{}
		var result []any

		Then(d, func(value any) (any, error) {
			result = append(result, value)
			return nil, nil
		}, Noop[error, any])
		d.Resolve(nil)

		assert.Equal(t, []any{nil}, result)
	})

	t.Run("error handler recovers with value", func(t *testing.T) {
		d := &DeferredImp[int]{}
		var results []string

		testError := errors.New("test error")

		d2 := Then(d, Noop[int, string], func(err error) (string, error) {
			results = append(results, "error handled")
			return "recovered", nil
		})

		Then(d2, func(value string) (string, error) {
			results = append(results, fmt.Sprintf("next handler: %s", value))
			return value, nil
		}, Noop[error, string])

		d.Reject(testError)

		assert.Equal(t, []string{"error handled", "next handler: recovered"}, results)
	})

	t.Run("error handler returning zero recovers", func(t *testing.T) {
		d := &DeferredImp[int]{}
		var results []string

		testError := errors.New("test error")

		d2 := Then(d, Noop[int, any], func(err error) (any, error) {
			results = append(results, "error handled")
			return nil, nil
		})

		Then(d2, func(value any) (any, error) {
			results = append(results, "next handler called")
			return nil, nil
		}, Noop[error, any])

		d.Reject(testError)

		assert.Equal(t, []string{"error handled", "next handler called"}, results)
	})

	t.Run("multiple resolves triggers handlers multiple times", func(t *testing.T) {
		d := &DeferredImp[int]{}
		var results []int

		Then(d, func(value int) (int, error) {
			results = append(results, value)
			return value, nil
		}, Noop[error, int])
		d.Resolve(42)
		d.Resolve(100)

		assert.Equal(t, []int{42, 100}, results)
	})
}

func TestComplexScenarios(t *testing.T) {
	t.Run("cleanup chain", func(t *testing.T) {
		d := &DeferredImp[string]{}
		var cleanupLog []string

		d2 := Then(d, func(resource string) (string, error) {
			cleanupLog = append(cleanupLog, fmt.Sprintf("cleanup1: %s", resource))
			return resource, nil
		}, Noop[error, string])

		d3 := Then(d2, func(value string) (string, error) {
			cleanupLog = append(cleanupLog, fmt.Sprintf("cleanup2: %s", value))
			return value, nil
		}, Noop[error, string])

		Then(d3, func(value string) (string, error) {
			cleanupLog = append(cleanupLog, fmt.Sprintf("cleanup3: %s", value))
			return value, nil
		}, Noop[error, string])

		d.Resolve("database_connection")

		assert.Equal(t, []string{
			"cleanup1: database_connection",
			"cleanup2: database_connection",
			"cleanup3: database_connection",
		}, cleanupLog)
	})

	t.Run("value transformation chain", func(t *testing.T) {
		d := &DeferredImp[int]{}
		var results []string

		d2 := Then(d, func(value int) (int, error) {
			results = append(results, fmt.Sprintf("double: %d", value))
			return value * 2, nil
		}, Noop[error, int])

		d3 := Then(d2, func(value int) (int, error) {
			results = append(results, fmt.Sprintf("add_ten: %d", value))
			return value + 10, nil
		}, Noop[error, int])

		Then(d3, func(value int) (string, error) {
			results = append(results, fmt.Sprintf("to_string: %d", value))
			return fmt.Sprintf("result_%d", value), nil
		}, Noop[error, string])

		d.Resolve(5)

		assert.Equal(t, []string{"double: 5", "add_ten: 10", "to_string: 20"}, results)
	})

	t.Run("partial failure chain", func(t *testing.T) {
		d := &DeferredImp[int]{}
		var results []string

		stepError := errors.New("step 2 failed")

		d2 := Then(d, func(value int) (int, error) {
			results = append(results, fmt.Sprintf("step1: %d", value))
			return value, nil
		}, Noop[error, int])

		d3 := Then(d2, func(value int) (int, error) {
			results = append(results, "step2: failing")
			return 0, stepError
		}, Noop[error, int])

		Then(d3, func(value int) (any, error) {
			results = append(results, "step3: should not execute")
			return value, nil
		}, func(err error) (any, error) {
			results = append(results, fmt.Sprintf("error handler: %s", err))
			return nil, nil
		})

		d.Resolve(42)

		assert.Equal(t, []string{
			"step1: 42",
			"step2: failing",
			fmt.Sprintf("error handler: %s", stepError),
		}, results)
	})
}

func TestAll(t *testing.T) {
	t.Run("resolves when all resolved", func(t *testing.T) {
		d1 := &DeferredImp[int]{}
		d2 := &DeferredImp[int]{}
		d3 := &DeferredImp[int]{}

		combined := All([]Deferred[int]{d1, d2, d3})
		var result []int

		Then(combined, func(values []int) (any, error) {
			result = append(result, values...)
			return nil, nil
		}, Noop[error, any])

		d1.Resolve(1)
		d2.Resolve(2)
		d3.Resolve(3)

		assert.Equal(t, []int{1, 2, 3}, result)
	})

	t.Run("rejects on first error", func(t *testing.T) {
		d1 := &DeferredImp[int]{}
		d2 := &DeferredImp[int]{}
		d3 := &DeferredImp[int]{}

		combined := All([]Deferred[int]{d1, d2, d3})
		var errs []error

		Then(combined, Noop[[]int, any], func(err error) (any, error) {
			errs = append(errs, err)
			return nil, nil
		})

		testError := errors.New("fail")
		d1.Resolve(1)
		d2.Reject(testError)

		assert.Equal(t, []error{testError}, errs)
	})

	t.Run("empty list", func(t *testing.T) {
		combined := All([]Deferred[int]{})
		var result [][]int

		Then(combined, func(values []int) (any, error) {
			result = append(result, values)
			return nil, nil
		}, Noop[error, any])

		assert.Equal(t, [][]int{{}}, result)
	})

	t.Run("preserves order", func(t *testing.T) {
		d1 := &DeferredImp[string]{}
		d2 := &DeferredImp[string]{}
		d3 := &DeferredImp[string]{}

		combined := All([]Deferred[string]{d1, d2, d3})
		var result []string

		Then(combined, func(values []string) (any, error) {
			result = append(result, values...)
			return nil, nil
		}, Noop[error, any])

		// Resolve in reverse order
		d3.Resolve("third")
		d1.Resolve("first")
		d2.Resolve("second")

		assert.Equal(t, []string{"first", "second", "third"}, result)
	})

	t.Run("single deferred", func(t *testing.T) {
		d := &DeferredImp[int]{}

		combined := All([]Deferred[int]{d})
		var result []int

		Then(combined, func(values []int) (any, error) {
			result = append(result, values...)
			return nil, nil
		}, Noop[error, any])

		d.Resolve(42)

		assert.Equal(t, []int{42}, result)
	})
}
