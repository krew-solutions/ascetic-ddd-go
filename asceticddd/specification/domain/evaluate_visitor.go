package specification

import (
	"errors"
	"fmt"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

var ErrKeyNotFound = errors.New("key not found")

// Context retrieves values by key during evaluation.
type Context interface {
	Get(string) (any, error)
}

// EvaluateVisitor evaluates a specification AST against a Context.
//
// Fields are immutable per call; collection iteration creates a sub-visitor
// via withItem() rather than mutating in place.
type EvaluateVisitor struct {
	context Context
	// items is the item under test, last, and before it the items of the
	// enclosing collections: OuterItem(depth) is the one depth steps out.
	items    []Context
	registry *operators.OperatorRegistry
}

func NewEvaluateVisitor(context Context, registry *operators.OperatorRegistry) *EvaluateVisitor {
	return &EvaluateVisitor{
		context:  context,
		registry: registry,
	}
}

// withItem returns a sub-visitor bound to a new current item.
// Used during wildcard iteration to scope @ to the current collection element.
func (v *EvaluateVisitor) withItem(item Context) *EvaluateVisitor {
	return &EvaluateVisitor{
		context:  v.context,
		items:    append(append([]Context{}, v.items...), item),
		registry: v.registry,
	}
}

// Evaluate is the typed entry point. It runs the visitor and asserts the
// final result to bool — top-level specifications must yield a boolean.
func (v *EvaluateVisitor) Evaluate(node Visitable) (bool, error) {
	result, err := Accept[any](node, v)
	if err != nil {
		return false, err
	}
	// A NULL is "not satisfied", as a row with a NULL condition is not selected.
	satisfied, _, ok := operators.Truth(result)
	if !ok {
		return false, errors.New("the result is not a bool")
	}
	return satisfied, nil
}

func (v *EvaluateVisitor) VisitGlobalScope(_ GlobalScopeNode) (any, error) {
	return v.context, nil
}

func (v *EvaluateVisitor) VisitObject(n ObjectNode) (any, error) {
	parentCtx, err := Accept[any](n.Parent(), v)
	if err != nil {
		return nil, err
	}
	obj, err := parentCtx.(Context).Get(n.Name())
	if err != nil {
		return nil, err
	}
	// An Option holding the object is read through, as one holding a field's
	// value is: what it holds. A Nothing holds no object to go into, as the
	// domain's Unwrap() of one has none - an error, which the guard a parser
	// writes beside the path never lets through.
	ctx, ok := operators.ReadOption(obj).(Context)
	if !ok {
		return nil, fmt.Errorf("object %s is not a Context", n.Name())
	}
	return ctx, nil
}

func (v *EvaluateVisitor) VisitItem(n ItemNode) (any, error) {
	if n.Depth() >= len(v.items) {
		return nil, errors.New("no current item in context")
	}
	return v.items[len(v.items)-1-n.Depth()], nil
}

func (v *EvaluateVisitor) VisitField(n FieldNode) (any, error) {
	objCtx, err := Accept[any](n.Object(), v)
	if err != nil {
		return nil, err
	}
	value, err := objCtx.(Context).Get(n.Name())
	// An Option of a value is the value, or a null.
	return operators.ReadOption(value), err
}

func (v *EvaluateVisitor) VisitValue(n ValueNode) (any, error) {
	return operators.ReadOption(n.Value()), nil
}

func (v *EvaluateVisitor) VisitCollection(n CollectionNode) (any, error) {
	parentCtx, err := Accept[any](n.Parent(), v)
	if err != nil {
		return nil, err
	}
	items, err := parentCtx.(Context).Get(n.Name())
	if err != nil {
		return nil, err
	}
	itemsTyped, ok := items.([]Context)
	if !ok {
		return nil, errors.New("value is not a collection of Contexts")
	}
	// EXISTS (... WHERE predicate): an item of which the predicate is NULL is
	// no witness, the result is never NULL, and the first witness decides.
	for i := range itemsTyped {
		value, err := Accept[any](n.Predicate(), v.withItem(itemsTyped[i]))
		if err != nil {
			return nil, err
		}
		witness, _, ok := operators.Truth(value)
		if !ok {
			return nil, errors.New("predicate did not yield a boolean")
		}
		if witness {
			return true, nil
		}
	}
	return false, nil
}

func (v *EvaluateVisitor) VisitPrefix(n PrefixNode) (any, error) {
	operand, err := Accept[any](n.Operand(), v)
	if err != nil {
		return nil, err
	}
	return v.registry.ExecUnary(n.Operator(), operand)
}

func (v *EvaluateVisitor) VisitPostfix(n PostfixNode) (any, error) {
	operand, err := Accept[any](n.Operand(), v)
	if err != nil {
		return nil, err
	}
	return v.registry.ExecUnary(n.Operator(), operand)
}

func (v *EvaluateVisitor) VisitInfix(n InfixNode) (any, error) {
	left, err := Accept[any](n.Left(), v)
	if err != nil {
		return nil, err
	}
	// A connective decided by its left operand does not evaluate its right
	// one, as && and || of the host language do not: `a != 0 AND 10 / a > 1`.
	if decides(n.Operator(), left) {
		return operators.Indirect(left), nil
	}
	right, err := Accept[any](n.Right(), v)
	if err != nil {
		return nil, err
	}
	return v.registry.ExecBinary(left, n.Operator(), right)
}

// decides tells whether a connective has its value in its left operand alone:
// false AND anything is false, true OR anything is true.
func decides(operator operators.Operator, left any) bool {
	if operator != operators.OperatorAnd && operator != operators.OperatorOr {
		return false
	}
	truth, null, ok := operators.Truth(left)
	if !ok || null {
		return false
	}
	return truth == (operator == operators.OperatorOr)
}

func ExtractFieldPath(n FieldNode) []string {
	path := []string{n.Name()}
	var obj EmptiableObject = n.Object()
	for !obj.IsRoot() {
		path = append([]string{obj.Name()}, path...)
		obj = obj.Parent()
	}
	return path
}

// ExtractFieldRoot returns what the path to a field starts at: GlobalScope or
// Item. A path from the item and a path from the candidate can be of the same
// names, and are not the same field.
func ExtractFieldRoot(n FieldNode) EmptiableObject {
	return ExtractObjectRoot(n.Object())
}

// ExtractObjectRoot returns what the path to an object starts at.
func ExtractObjectRoot(obj EmptiableObject) EmptiableObject {
	for !obj.IsRoot() {
		obj = obj.Parent()
	}
	return obj
}

// ExtractObjectPath returns the names from the root to an object.
func ExtractObjectPath(obj EmptiableObject) []string {
	var path []string
	for !obj.IsRoot() {
		path = append([]string{obj.Name()}, path...)
		obj = obj.Parent()
	}
	return path
}

type CollectionContext struct {
	items []Context
}

func NewCollectionContext(items []Context) CollectionContext {
	return CollectionContext{items: items}
}

func (c CollectionContext) Get(slice string) (any, error) {
	if slice == "*" {
		return c.items, nil
	}
	return nil, fmt.Errorf("unsupported slice type \"%s\"", slice)
}

// MapContext is a candidate made of plain data: a map, whose maps are objects
// and whose lists are collections.
//
// For tests, for documents, and for a candidate that arrives as data rather
// than as a domain object; a domain object implements Context itself. A
// member that is not there is ErrKeyNotFound, not a null: a null is a member
// that is there and holds nothing, `MapContext{"discount": nil}`. An Option,
// and a context given ready-made, a MapContext or a CollectionContext, are
// kept as they are.
type MapContext map[string]any

func (c MapContext) Get(key string) (any, error) {
	value, ok := c[key]
	if !ok {
		return nil, ErrKeyNotFound
	}
	return fromData(value), nil
}

// fromData returns a map as an object, a list of contexts - or of maps - as a
// collection, and anything else as it is.
func fromData(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return MapContext(v)
	case []Context:
		return NewCollectionContext(v)
	case []any:
		items := make([]Context, 0, len(v))
		for _, item := range v {
			ctx, ok := fromData(item).(Context)
			if !ok {
				return value
			}
			items = append(items, ctx)
		}
		return NewCollectionContext(items)
	}
	return value
}
