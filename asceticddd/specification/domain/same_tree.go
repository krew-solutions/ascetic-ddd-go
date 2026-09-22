package specification

import (
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

// Description is a tree as plain data: what Describe folds a tree into. Two
// descriptions compare with reflect.DeepEqual, and one prints readably.
type Description []any

// SameTree tells whether two trees are the same specification: the same
// nodes, with the same values.
//
// The nodes are comparable structs, so `==` compiles, and is not this. A value
// is held in an interface: two pointers to one content compare by their
// addresses and differ, and a slice or a map in a value panics, "comparing
// uncomparable type". The evaluator and the driver read a pointer through;
// so does this.
//
// A value is the same as another of the same type and content. An integer is
// not a float of the same number, nor an int an int64: the query is not the
// same. A null is a null whatever kind of nil it is, a NaN is itself, and an
// instant is the same in any zone.
func SameTree(a, b Visitable) bool {
	return reflect.DeepEqual(Describe(a), Describe(b))
}

// Describe folds a tree into its description.
//
// It is a Visitor, as every reader of a tree is: a kind of node added to
// Visitor does not compile until it is described here, and there is no second
// list of the kinds of node to be kept in step with Accept's.
func Describe(node Visitable) Description {
	if node == nil {
		return nil
	}
	description, err := Accept[Description](node, describeVisitor{})
	if err != nil {
		// A node Accept does not know: described by what it is.
		return Description{"unknown", node}
	}
	return description
}

type describeVisitor struct{}

func (describeVisitor) VisitGlobalScope(GlobalScopeNode) (Description, error) {
	return Description{"$"}, nil
}

func (describeVisitor) VisitItem(n ItemNode) (Description, error) {
	if n.Depth() == 0 {
		return Description{"@"}, nil
	}
	return Description{fmt.Sprintf("@%d", n.Depth())}, nil
}

func (describeVisitor) VisitObject(n ObjectNode) (Description, error) {
	return Description{"object", Describe(n.Parent()), n.Name()}, nil
}

func (describeVisitor) VisitField(n FieldNode) (Description, error) {
	return Description{"field", Describe(n.Object()), n.Name()}, nil
}

func (describeVisitor) VisitCollection(n CollectionNode) (Description, error) {
	return Description{"any", Describe(n.Parent()), n.Name(), Describe(n.Predicate())}, nil
}

func (describeVisitor) VisitValue(n ValueNode) (Description, error) {
	return Description{"value", describeValue(n.Value())}, nil
}

func (describeVisitor) VisitPrefix(n PrefixNode) (Description, error) {
	return Description{"prefix", n.Operator(), n.Associativity(), Describe(n.Operand())}, nil
}

func (describeVisitor) VisitInfix(n InfixNode) (Description, error) {
	return Description{"infix", n.Operator(), n.Associativity(), Describe(n.Left()), Describe(n.Right())}, nil
}

func (describeVisitor) VisitPostfix(n PostfixNode) (Description, error) {
	return Description{"postfix", n.Operator(), n.Associativity(), Describe(n.Operand())}, nil
}

// notANumber stands for a NaN, which is not equal to itself.
type notANumber struct{}

// instant stands for a time.Time, two values of which differ by their zone
// and their monotonic clock reading where the instant is one.
type instant struct {
	seconds     int64
	nanoseconds int
}

// describeValue returns a value as it is compared: a pointer read through, a
// null whatever kind of nil it is.
func describeValue(value any) any {
	if operators.IsNull(value) {
		return nil
	}
	switch v := operators.Indirect(value).(type) {
	case float64:
		if math.IsNaN(v) {
			return notANumber{}
		}
		return v
	case time.Time:
		return instant{v.Unix(), v.Nanosecond()}
	default:
		return v
	}
}

// String writes the description as an expression in parentheses:
//
//	(infix = NON (field ($) name) (value "ann"))
func (d Description) String() string {
	if d == nil {
		return "<no tree>"
	}
	parts := make([]string, 0, len(d))
	for i, part := range d {
		// What follows the word "value" is the value, a string included.
		if i == 1 && d[0] == "value" {
			parts = append(parts, formatValue(part))
			continue
		}
		switch p := part.(type) {
		case Description:
			parts = append(parts, p.String())
		default:
			parts = append(parts, fmt.Sprint(p))
		}
	}
	return "(" + strings.Join(parts, " ") + ")"
}

// formatValue writes a value with its type where the type does not show: an
// int64 prints as an int does, and they are not the same value.
func formatValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case bool, int, float64:
		return fmt.Sprintf("%v", v)
	case string:
		return fmt.Sprintf("%q", v)
	default:
		return fmt.Sprintf("%T(%v)", v, v)
	}
}
