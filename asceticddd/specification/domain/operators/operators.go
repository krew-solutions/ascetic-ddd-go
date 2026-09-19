package operators

type Operator string

const (
	// Comparison

	OperatorEq  Operator = "="
	OperatorGt  Operator = ">"
	OperatorLt  Operator = "<"
	OperatorGte Operator = ">="
	OperatorLte Operator = "<="
	OperatorNe  Operator = "!="
	OperatorIs  Operator = "IS"

	// Logical operators

	OperatorAnd Operator = "AND"
	OperatorOr  Operator = "OR"
	OperatorNot Operator = "NOT"

	// Mathematical

	OperatorAdd Operator = "+"
	OperatorSub Operator = "-"
	OperatorMul Operator = "*"
	OperatorDiv Operator = "/"
	OperatorMod Operator = "%"

	// OperatorNeg is named, not spelled: "-" is OperatorSub. How an operator
	// is spelled is for the notation that writes it to say. There is no unary
	// plus: nothing made a node of it.
	OperatorNeg Operator = "-neg"

	// Bitwise

	OperatorLshift Operator = "<<"
	OperatorRshift Operator = ">>"

	// Postfix

	OperatorIsNull    Operator = "IS NULL"
	OperatorIsNotNull Operator = "IS NOT NULL"
)
