package operators

import (
	"errors"
	"testing"
)

type Money struct {
	amount   int
	currency string
}

func (m Money) Equal(other EqualOperand) bool {
	o, ok := other.(Money)
	if !ok {
		return false
	}
	return m.amount == o.amount && m.currency == o.currency
}

func (m Money) GreaterThan(other GreaterThanOperand) bool {
	o, ok := other.(Money)
	if !ok {
		return false
	}
	return m.amount > o.amount
}

func (m Money) GreaterThanEqual(other GreaterThanEqualOperand) bool {
	o, ok := other.(Money)
	if !ok {
		return false
	}
	return m.amount >= o.amount
}

func (m Money) LessThan(other LessThanOperand) bool {
	o, ok := other.(Money)
	if !ok {
		return false
	}
	return m.amount < o.amount
}

func (m Money) LessThanEqual(other LessThanEqualOperand) bool {
	o, ok := other.(Money)
	if !ok {
		return false
	}
	return m.amount <= o.amount
}

func TestInterfaceFallback_Equal(t *testing.T) {
	reg := NewDefaultRegistry()

	result, err := reg.ExecBinary(Money{100, "USD"}, OperatorEq, Money{100, "USD"})
	if err != nil {
		t.Fatalf("ExecBinary failed: %v", err)
	}
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}

	result, err = reg.ExecBinary(Money{100, "USD"}, OperatorEq, Money{200, "USD"})
	if err != nil {
		t.Fatalf("ExecBinary failed: %v", err)
	}
	if result != false {
		t.Errorf("Expected false, got %v", result)
	}

	result, err = reg.ExecBinary(Money{100, "USD"}, OperatorEq, Money{100, "EUR"})
	if err != nil {
		t.Fatalf("ExecBinary failed: %v", err)
	}
	if result != false {
		t.Errorf("Expected false, got %v", result)
	}
}

func TestInterfaceFallback_NotEqual(t *testing.T) {
	reg := NewDefaultRegistry()

	result, err := reg.ExecBinary(Money{100, "USD"}, OperatorNe, Money{200, "USD"})
	if err != nil {
		t.Fatalf("ExecBinary failed: %v", err)
	}
	if result != true {
		t.Errorf("Expected true, got %v", result)
	}

	result, err = reg.ExecBinary(Money{100, "USD"}, OperatorNe, Money{100, "USD"})
	if err != nil {
		t.Fatalf("ExecBinary failed: %v", err)
	}
	if result != false {
		t.Errorf("Expected false, got %v", result)
	}
}

func TestInterfaceFallback_Comparison(t *testing.T) {
	reg := NewDefaultRegistry()

	tests := []struct {
		name     string
		left     Money
		op       Operator
		right    Money
		expected bool
	}{
		{"100 > 50", Money{100, "USD"}, OperatorGt, Money{50, "USD"}, true},
		{"50 > 100", Money{50, "USD"}, OperatorGt, Money{100, "USD"}, false},
		{"100 >= 100", Money{100, "USD"}, OperatorGte, Money{100, "USD"}, true},
		{"50 >= 100", Money{50, "USD"}, OperatorGte, Money{100, "USD"}, false},
		{"50 < 100", Money{50, "USD"}, OperatorLt, Money{100, "USD"}, true},
		{"100 < 50", Money{100, "USD"}, OperatorLt, Money{50, "USD"}, false},
		{"100 <= 100", Money{100, "USD"}, OperatorLte, Money{100, "USD"}, true},
		{"100 <= 50", Money{100, "USD"}, OperatorLte, Money{50, "USD"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := reg.ExecBinary(tt.left, tt.op, tt.right)
			if err != nil {
				t.Fatalf("ExecBinary failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestInterfaceFallback_NullPropagation(t *testing.T) {
	reg := NewDefaultRegistry()

	result, err := reg.ExecBinary(nil, OperatorEq, Money{100, "USD"})
	if err != nil {
		t.Fatalf("ExecBinary failed: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil (NULL), got %v", result)
	}

	result, err = reg.ExecBinary(Money{100, "USD"}, OperatorGt, nil)
	if err != nil {
		t.Fatalf("ExecBinary failed: %v", err)
	}
	if result != nil {
		t.Errorf("Expected nil (NULL), got %v", result)
	}
}

// A reader is registered by the type it reads, as an operator is by the types
// it applies to, and found by the value beside the string: through a pointer,
// as an operator is. Where none is registered the string is a string.
func TestAReaderIsFoundByTheKindBesideTheString(t *testing.T) {
	reg := NewOperatorRegistry()
	RegisterReader(reg, func(text string) (Money, error) {
		if text == "" {
			return Money{}, errors.New("no money")
		}
		return Money{len(text), "USD"}, nil
	})
	beside := Money{1, "USD"}
	for _, c := range []struct {
		name  string
		other any
		want  any
	}{
		{"the kind", beside, Money{3, "USD"}},
		{"a pointer to it", &beside, Money{3, "USD"}},
		{"no reader", 100, "abc"},
		{"a null", nil, "abc"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := reg.Read("abc", c.other)
			if err != nil || got != c.want {
				t.Errorf("got %v, %v, want %v", got, err, c.want)
			}
		})
	}
	if got, err := reg.Read("", beside); err == nil {
		t.Errorf("got %v, want the reader's error", got)
	}
}
