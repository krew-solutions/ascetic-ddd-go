package specification

import (
	"reflect"
	"time"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

// The type PostgreSQL is told a constant has, where nothing else tells it.
//
// A constant is a numbered parameter, and the server finds its type from what
// stands beside it: `"age" >= $1` makes `$1` whatever `age` is, and the driver
// writes the value as that. Where every operand of an operator is a constant
// there is nothing beside it: `$1 + $2` is "operator is not unique: unknown +
// unknown", `-$1` likewise, and of `$1 IS NULL` the server "could not
// determine data type".
//
// So there, and only there, the text says the type: `$1::bigint + $2::bigint`.
// Not everywhere. A value adapts to the column it meets - an int is written as
// an int2, an int4 or an int8, a time as a timestamp with or without zone -
// and a type said beside a column takes that away: `"at" = $1::timestamptz`
// of a column without zone is compared in the session's time zone, and selects
// other rows than `"at" = $1` does.

// paramType returns what PostgreSQL calls the type of a value, by its kind:
// "" for a null, which has none, and for a kind this does not know. A pointer
// is what it points at, as it is to the driver.
func paramType(value any) string {
	switch operators.Indirect(value).(type) {
	case nil:
		return ""
	case bool:
		return "boolean"
	case string:
		return "text"
	case time.Time:
		return "timestamptz"
	case time.Duration:
		return "interval"
	}
	switch reflect.ValueOf(operators.Indirect(value)).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "bigint"
	case reflect.Float32, reflect.Float64:
		return "double precision"
	default:
		return ""
	}
}

// constant returns the value of a node that is a constant.
func constant(node s.Visitable) (any, bool) {
	value, ok := node.(s.ValueNode)
	if !ok {
		return nil, false
	}
	return value.Value(), true
}

// typeUnderPrefix returns the type to say of the operand of a prefix operator,
// if it is a constant: it has nothing beside it to take a type from. A null
// has no kind, and takes what the operator is of: a number under `-`; under
// NOT the server finds boolean by itself.
func typeUnderPrefix(op operators.Operator, operand s.Visitable) string {
	value, ok := constant(operand)
	if !ok {
		return ""
	}
	if said := paramType(value); said != "" || !operators.IsNull(value) {
		return said
	}
	if op == operators.OperatorNeg {
		return "bigint"
	}
	return ""
}

// typeUnderPostfix returns the type to say of the operand of a null test, if
// it is a constant. Of what type a null is tested does not matter, and the
// server must be told one: "could not determine data type of parameter".
func typeUnderPostfix(operand s.Visitable) string {
	value, ok := constant(operand)
	if !ok {
		return ""
	}
	if said := paramType(value); said != "" || !operators.IsNull(value) {
		return said
	}
	return "text"
}

var arithmetic = map[operators.Operator]bool{
	operators.OperatorAdd: true, operators.OperatorSub: true, operators.OperatorMul: true,
	operators.OperatorDiv: true, operators.OperatorMod: true,
	operators.OperatorLshift: true, operators.OperatorRshift: true,
}

// typesOfBoth returns the types to say of the operands of an operator, if both
// are constants: neither has anything beside it to take a type from.
// PostgreSQL shifts a bigint by an integer, so the count of a shift is that.
// Two nulls take what the operator is of: numbers under arithmetic; compared,
// the server takes them for texts by itself.
func typesOfBoth(left s.Visitable, op operators.Operator, right s.Visitable) (string, string) {
	leftValue, ok := constant(left)
	if !ok {
		return "", ""
	}
	rightValue, ok := constant(right)
	if !ok {
		return "", ""
	}
	ofLeft, ofRight := paramType(leftValue), paramType(rightValue)
	if operators.IsNull(leftValue) && operators.IsNull(rightValue) && arithmetic[op] {
		ofLeft, ofRight = "bigint", "bigint"
	}
	if ofRight == "bigint" && (op == operators.OperatorLshift || op == operators.OperatorRshift) {
		ofRight = "integer"
	}
	return ofLeft, ofRight
}

// isACountToCast reports whether right is the count of a shift that must be
// said an integer. PostgreSQL shifts by an integer and by nothing else: a
// column as the count, a bigint more often than not, is "operator does not
// exist: bigint << bigint". A constant there is inferred, or was said an
// integer already where nothing stands beside it; a column or an expression
// has a type of its own, which the server will not convert, so it is cast.
// The cast takes nothing away - the operator has the count an integer
// already - and turns a column of any integer type into the one it has.
func isACountToCast(op operators.Operator, right s.Visitable) bool {
	if op != operators.OperatorLshift && op != operators.OperatorRshift {
		return false
	}
	_, isConstant := constant(right)
	return !isConstant
}

// ofType returns the text with its type said, if there is one to say: a cast
// binds tighter than any operator, so what was an atom is one still.
func ofType(sql, said string) string {
	if said == "" {
		return sql
	}
	return sql + "::" + said
}
