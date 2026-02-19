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

	OperatorPos Operator = "+pos"
	OperatorNeg Operator = "-neg"

	// Bitwise

	OperatorLshift Operator = "<<"
	OperatorRshift Operator = ">>"

	// Postfix

	OperatorIsNull    Operator = "IS NULL"
	OperatorIsNotNull Operator = "IS NOT NULL"
)
