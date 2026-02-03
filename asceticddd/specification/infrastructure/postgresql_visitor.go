package specification

import (
	"fmt"
	"strings"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

func Compile(context Context, exp s.Visitable) (sql string, params []any, err error) {
	tv := NewTransformVisitor(context)
	err = exp.Accept(tv)
	if err != nil {
		return "", nil, err
	}
	exp, err = tv.Result()
	if err != nil {
		return "", nil, err
	}
	v := NewPostgresqlVisitor()
	err = exp.Accept(v)
	if err != nil {
		return "", nil, err
	}
	return v.Result()
}

// CompileToSQL compiles AST directly to SQL without context transformation
// Useful for generated code where AST is already in the right form
func CompileToSQL(exp s.Visitable) (sql string, params []any, err error) {
	v := NewPostgresqlVisitor()
	err = exp.Accept(v)
	if err != nil {
		return "", nil, err
	}
	return v.Result()
}

type PostgresqlVisitorOption func(*PostgresqlVisitor)

func PlaceholderIndex(index uint8) PostgresqlVisitorOption {
	return func(v *PostgresqlVisitor) {
		v.placeholderIndex = index
	}
}

func NewPostgresqlVisitor(opts ...PostgresqlVisitorOption) *PostgresqlVisitor {
	v := &PostgresqlVisitor{
		precedenceMapping: make(map[string]int),
	}
	// https://www.postgresql.org/docs/14/sql-syntax-lexical.html#SQL-PRECEDENCE-TABLE
	v.setPrecedence(160, ". LEFT")
	v.setPrecedence(160, ":: LEFT")
	v.setPrecedence(150, "[ LEFT")
	v.setPrecedence(140, "+ RIGHT", "- RIGHT")
	v.setPrecedence(130, "^ LEFT")
	v.setPrecedence(120, "* LEFT", "/ LEFT", "% LEFT")
	v.setPrecedence(110, "+ LEFT", "- LEFT")
	// all other native and user-defined operators 👇️
	v.setPrecedence(100, "(any other operator) LEFT")
	v.setPrecedence(90, "BETWEEN NON", "IN NON", "LIKE NON", "ILIKE NON", "SIMILAR NON")
	v.setPrecedence(80, "< NON", "> NON", "= NON", "<= NON", ">= NON", "!= NON")
	v.setPrecedence(70, "IS NON", "ISNULL NON", "NOTNULL NON")
	v.setPrecedence(60, "NOT RIGHT")
	v.setPrecedence(50, "AND LEFT")
	v.setPrecedence(40, "OR LEFT")
	for i := range opts {
		opts[i](v)
	}
	return v
}

type PostgresqlVisitor struct {
	sql               string
	placeholderIndex  uint8
	parameters        []any
	precedence        int
	precedenceMapping map[string]int
	// Wildcard context tracking
	inWildcard      bool   // Are we inside a wildcard predicate?
	wildcardAlias   string // Current wildcard item alias (e.g., "item")
	wildcardCounter int    // Counter for unique aliases
}

func (v PostgresqlVisitor) getNodePrecedenceKey(n s.Operable) string {
	operator := n.Operator()
	return fmt.Sprintf("%s %s", operator, n.Associativity())
}
func (v PostgresqlVisitor) setPrecedence(precedence int, operators ...string) {
	for _, op := range operators {
		v.precedenceMapping[op] = precedence
	}
}

func (v *PostgresqlVisitor) visit(precedenceKey string, callable func() error) error {
	outerPrecedence := v.precedence
	innerPrecedence, ok := v.precedenceMapping[precedenceKey]
	if !ok {
		innerPrecedence, ok = v.precedenceMapping["(any other operator) LEFT"]
		if !ok {
			innerPrecedence = outerPrecedence
		}
	}
	v.precedence = innerPrecedence
	if innerPrecedence < outerPrecedence {
		v.sql += "("
	}
	err := callable()
	if err != nil {
		return err
	}
	if innerPrecedence < outerPrecedence {
		v.sql += ")"
	}
	v.precedence = outerPrecedence
	return nil
}

func (v *PostgresqlVisitor) VisitGlobalScope(_ s.GlobalScopeNode) error {
	return nil
}

func (v *PostgresqlVisitor) VisitObject(_ s.ObjectNode) error {
	return nil
}

func (v *PostgresqlVisitor) VisitCollection(n s.CollectionNode) error {
	// Collection node represents a wildcard: spec.Any/All over a collection
	// Generate: EXISTS (SELECT 1 FROM unnest(collection_path) AS item WHERE predicate)

	// Extract collection path (e.g., "Items" from Object(GlobalScope(), "Items"))
	collectionPath := v.extractCollectionPath(n)

	// Generate unique alias for this wildcard
	v.wildcardCounter++
	alias := fmt.Sprintf("item_%d", v.wildcardCounter)

	// Save context
	outerInWildcard := v.inWildcard
	outerWildcardAlias := v.wildcardAlias

	// Enter wildcard context
	v.inWildcard = true
	v.wildcardAlias = alias

	// Generate EXISTS subquery
	v.sql += "EXISTS (SELECT 1 FROM unnest("
	v.sql += collectionPath
	v.sql += ") AS "
	v.sql += alias
	v.sql += " WHERE "

	// Visit predicate
	err := n.Predicate().Accept(v)
	if err != nil {
		return err
	}

	v.sql += ")"

	// Restore context
	v.inWildcard = outerInWildcard
	v.wildcardAlias = outerWildcardAlias

	return nil
}

// extractCollectionPath extracts the SQL path to a collection from a CollectionNode
func (v *PostgresqlVisitor) extractCollectionPath(n s.CollectionNode) string {
	var parts []string

	// Walk up the parent chain to collect path components
	parent := n.Parent()
	for !parent.IsRoot() {
		parts = append([]string{parent.Name()}, parts...) // prepend
		parent = parent.Parent()
	}

	// Note: We don't add n.Name() because it's "*" (wildcard marker)
	// The actual collection name is the last component in the parent chain

	return strings.Join(parts, ".")
}

func (v *PostgresqlVisitor) VisitItem(n s.ItemNode) error {
	// Item() in wildcard context refers to the current item alias
	// This is handled in VisitField when we detect Item() as parent
	return nil
}

func (v *PostgresqlVisitor) VisitField(n s.FieldNode) error {
	// Check if this field references an item in a wildcard context
	if v.inWildcard && v.isItemReference(n.Object()) {
		// This is a field of the current item: item.Price, item.Active, etc.
		v.sql += v.wildcardAlias
		v.sql += "."
		v.sql += n.Name()
	} else {
		// Normal field access
		path := s.ExtractFieldPath(n)
		name := strings.Join(path, ".")
		v.sql += name
	}
	return nil
}

// isItemReference checks if the object is Item() (current item in wildcard)
func (v *PostgresqlVisitor) isItemReference(obj s.EmptiableObject) bool {
	// Item() has IsRoot() == true and Name() == "@"
	_, isItem := obj.(s.ItemNode)
	return isItem
}

func (v *PostgresqlVisitor) VisitValue(n s.ValueNode) error {
	value := n.Value()
	v.parameters = append(v.parameters, value)
	v.sql += fmt.Sprintf("$%d", len(v.parameters))
	return nil
}

func (v *PostgresqlVisitor) VisitPrefix(node s.PrefixNode) error {
	precedenceKey := v.getNodePrecedenceKey(node)
	return v.visit(precedenceKey, func() error {
		operator := node.Operator()
		if operator == s.OperatorPos || operator == s.OperatorNeg {
			v.sql += string(operator)
		} else {
			v.sql += fmt.Sprintf("%s ", operator)
		}
		return node.Operand().Accept(v)
	})
}

func (v *PostgresqlVisitor) VisitInfix(n s.InfixNode) error {
	precedenceKey := v.getNodePrecedenceKey(n)
	return v.visit(precedenceKey, func() error {
		err := n.Left().Accept(v)
		if err != nil {
			return err
		}
		v.sql += fmt.Sprintf(" %s ", n.Operator())
		err = n.Right().Accept(v)
		if err != nil {
			return err
		}
		return nil
	})
}

func (v *PostgresqlVisitor) VisitPostfix(node s.PostfixNode) error {
	precedenceKey := v.getNodePrecedenceKey(node)
	return v.visit(precedenceKey, func() error {
		err := node.Operand().Accept(v)
		if err != nil {
			return err
		}
		operator := node.Operator()
		v.sql += fmt.Sprintf(" %s", operator)
		return nil
	})
}

func (v PostgresqlVisitor) Result() (sql string, params []any, err error) {
	return v.sql, v.parameters, nil
}
