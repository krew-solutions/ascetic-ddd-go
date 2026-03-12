package query

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

// IObjectResolver resolves a relation field to foreign object state for evaluation.
type IObjectResolver interface {
	Resolve(s session.Session, field *string, fkValue any) (map[string]any, IObjectResolver, error)
	// Descend returns a resolver scoped to the child provider for the given field.
	// Used when entering nested CompositeQuery fields to ensure
	// the resolver navigates the correct level of the provider tree.
	Descend(field string) IObjectResolver
}

type fieldContext struct {
	field   string
	fkValue any
}

// EvaluateWalker evaluates whether an object state matches query criteria.
type EvaluateWalker struct {
	registry       *operators.OperatorRegistry
	objectResolver IObjectResolver
}

func NewEvaluateWalker(objectResolver IObjectResolver) *EvaluateWalker {
	return &EvaluateWalker{
		registry:       operators.NewDefaultRegistry(),
		objectResolver: objectResolver,
	}
}

// Evaluate checks if state matches query. Supports IObjectResolver for RelOperator.
func (w *EvaluateWalker) Evaluate(
	s session.Session,
	query IQueryOperator,
	state any,
) (bool, error) {
	return w.evaluate(s, query, state, nil)
}

func (w *EvaluateWalker) evaluate(
	s session.Session,
	query IQueryOperator,
	state any,
	fc *fieldContext,
) (bool, error) {
	switch q := query.(type) {
	case EqOperator:
		return reflect.DeepEqual(state, q.Value), nil

	case ComparisonOperator:
		return w.compare(q.Op, state, q.Value)

	case InOperator:
		return w.contains(q.Values, state), nil

	case IsNullOperator:
		return (state == nil) == q.Value, nil

	case AndOperator:
		for _, operand := range q.Operands {
			result, err := w.evaluate(s, operand, state, fc)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil

	case OrOperator:
		for _, operand := range q.Operands {
			result, err := w.evaluate(s, operand, state, fc)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil

	case NotOperator:
		result, err := w.evaluate(s, q.Operand, state, fc)
		if err != nil {
			return false, err
		}
		return !result, nil

	case AnyElementOperator:
		items, ok := toSlice(state)
		if !ok {
			return false, nil
		}
		for _, item := range items {
			result, err := w.evaluate(s, q.Query, item, nil)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil

	case AllElementsOperator:
		items, ok := toSlice(state)
		if !ok {
			return false, nil
		}
		for _, item := range items {
			result, err := w.evaluate(s, q.Query, item, nil)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil

	case LenOperator:
		items, ok := toSlice(state)
		if !ok {
			return false, nil
		}
		return w.evaluate(s, q.Query, len(items), nil)

	case CompositeQuery:
		return w.evaluateComposite(s, q, state)

	case RelOperator:
		if w.objectResolver != nil {
			var field *string
			var fkValue any
			if fc != nil {
				field = &fc.field
				fkValue = fc.fkValue
			} else {
				field = nil
				fkValue = state
			}
			foreignState, nestedResolver, err := w.objectResolver.Resolve(s, field, fkValue)
			if err != nil {
				return false, err
			}
			if foreignState == nil {
				return false, nil
			}
			nested := &EvaluateWalker{registry: w.registry, objectResolver: nestedResolver}
			return nested.evaluate(s, q.Query, foreignState, nil)
		}
		return w.evaluate(s, q.Query, state, nil)
	}

	return false, nil
}

func (w *EvaluateWalker) evaluateComposite(
	s session.Session,
	query CompositeQuery,
	state any,
) (bool, error) {
	if !isStructLike(state) {
		return false, nil
	}
	for field, fieldOp := range query.Fields {
		fieldValue, _ := getFieldValue(state, field)
		result, err := w.evaluateField(s, field, fieldOp, fieldValue)
		if err != nil {
			return false, err
		}
		if !result {
			return false, nil
		}
	}
	return true, nil
}

func (w *EvaluateWalker) evaluateField(
	s session.Session,
	field string,
	fieldOp IQueryOperator,
	fieldValue any,
) (bool, error) {
	if relOp, ok := fieldOp.(RelOperator); ok && w.objectResolver != nil {
		foreignState, nestedResolver, err := w.objectResolver.Resolve(s, &field, fieldValue)
		if err != nil {
			return false, err
		}
		if foreignState == nil {
			return false, nil
		}
		nested := &EvaluateWalker{registry: w.registry, objectResolver: nestedResolver}
		return nested.evaluate(s, relOp.Query, foreignState, nil)
	}
	walker := w
	if w.objectResolver != nil {
		descended := w.objectResolver.Descend(field)
		if descended != nil {
			walker = &EvaluateWalker{registry: w.registry, objectResolver: descended}
		}
	}
	return walker.evaluate(s, fieldOp, fieldValue, &fieldContext{field: field, fkValue: fieldValue})
}

// EvaluateSync checks if state matches query without session or resolver support.
func (w *EvaluateWalker) EvaluateSync(
	query IQueryOperator,
	state any,
) (bool, error) {
	return w.evaluateSync(query, state, nil)
}

func (w *EvaluateWalker) evaluateSync(
	query IQueryOperator,
	state any,
	fc *fieldContext,
) (bool, error) {
	switch q := query.(type) {
	case EqOperator:
		return reflect.DeepEqual(state, q.Value), nil

	case ComparisonOperator:
		return w.compare(q.Op, state, q.Value)

	case InOperator:
		return w.contains(q.Values, state), nil

	case IsNullOperator:
		return (state == nil) == q.Value, nil

	case AndOperator:
		for _, operand := range q.Operands {
			result, err := w.evaluateSync(operand, state, fc)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil

	case OrOperator:
		for _, operand := range q.Operands {
			result, err := w.evaluateSync(operand, state, fc)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil

	case NotOperator:
		result, err := w.evaluateSync(q.Operand, state, fc)
		if err != nil {
			return false, err
		}
		return !result, nil

	case AnyElementOperator:
		items, ok := toSlice(state)
		if !ok {
			return false, nil
		}
		for _, item := range items {
			result, err := w.evaluateSync(q.Query, item, nil)
			if err != nil {
				return false, err
			}
			if result {
				return true, nil
			}
		}
		return false, nil

	case AllElementsOperator:
		items, ok := toSlice(state)
		if !ok {
			return false, nil
		}
		for _, item := range items {
			result, err := w.evaluateSync(q.Query, item, nil)
			if err != nil {
				return false, err
			}
			if !result {
				return false, nil
			}
		}
		return true, nil

	case LenOperator:
		items, ok := toSlice(state)
		if !ok {
			return false, nil
		}
		return w.evaluateSync(q.Query, len(items), nil)

	case CompositeQuery:
		return w.evaluateCompositeSync(q, state)

	case RelOperator:
		return w.evaluateSync(q.Query, state, nil)
	}

	return false, nil
}

func (w *EvaluateWalker) evaluateCompositeSync(
	query CompositeQuery,
	state any,
) (bool, error) {
	if !isStructLike(state) {
		return false, nil
	}
	for field, fieldOp := range query.Fields {
		fieldValue, _ := getFieldValue(state, field)
		result, err := w.evaluateFieldSync(field, fieldOp, fieldValue)
		if err != nil {
			return false, err
		}
		if !result {
			return false, nil
		}
	}
	return true, nil
}

func (w *EvaluateWalker) evaluateFieldSync(
	field string,
	fieldOp IQueryOperator,
	fieldValue any,
) (bool, error) {
	if relOp, ok := fieldOp.(RelOperator); ok {
		return w.evaluateSync(relOp.Query, fieldValue, nil)
	}
	walker := w
	if w.objectResolver != nil {
		descended := w.objectResolver.Descend(field)
		if descended != nil {
			walker = &EvaluateWalker{registry: w.registry, objectResolver: descended}
		}
	}
	return walker.evaluateSync(fieldOp, fieldValue, &fieldContext{field: field, fkValue: fieldValue})
}

func (w *EvaluateWalker) compare(op string, actual, expected any) (bool, error) {
	var regOp operators.Operator
	switch op {
	case "$ne":
		regOp = operators.OperatorNe
	case "$gt":
		regOp = operators.OperatorGt
	case "$gte":
		regOp = operators.OperatorGte
	case "$lt":
		regOp = operators.OperatorLt
	case "$lte":
		regOp = operators.OperatorLte
	default:
		return false, fmt.Errorf("unknown comparison operator: %s", op)
	}
	result, err := w.registry.ExecBinary(actual, regOp, expected)
	if err != nil {
		return false, err
	}
	if result == nil {
		return false, nil
	}
	b, ok := result.(bool)
	if !ok {
		return false, fmt.Errorf("comparison operator %s returned non-bool: %T", op, result)
	}
	return b, nil
}

func (w *EvaluateWalker) contains(values []any, state any) bool {
	for _, v := range values {
		if reflect.DeepEqual(state, v) {
			return true
		}
	}
	return false
}

func toSlice(state any) ([]any, bool) {
	if state == nil {
		return nil, false
	}
	if s, ok := state.([]any); ok {
		return s, true
	}
	v := reflect.ValueOf(state)
	if v.Kind() == reflect.Slice {
		result := make([]any, v.Len())
		for i := 0; i < v.Len(); i++ {
			result[i] = v.Index(i).Interface()
		}
		return result, true
	}
	return nil, false
}

func isStructLike(state any) bool {
	if state == nil {
		return false
	}
	if _, ok := state.(map[string]any); ok {
		return true
	}
	t := reflect.TypeOf(state)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct
}

func getFieldValue(state any, field string) (any, bool) {
	if m, ok := state.(map[string]any); ok {
		v, found := m[field]
		return v, found
	}
	v := reflect.ValueOf(state)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, false
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		tag := sf.Tag.Get("json")
		if tag != "" {
			name, _, _ := strings.Cut(tag, ",")
			if name == field {
				return v.Field(i).Interface(), true
			}
		}
	}
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}
		if sf.Name == field {
			return v.Field(i).Interface(), true
		}
	}
	return nil, false
}

// EvaluateVisitor is a visitor-based evaluator.
// State is carried in the instance; recursion creates new instances.
type EvaluateVisitor struct {
	state          any
	sess           session.Session
	objectResolver IObjectResolver
	fieldCtx       *fieldContext
	registry       *operators.OperatorRegistry
}

func NewEvaluateVisitor(state any, s session.Session, objectResolver IObjectResolver) *EvaluateVisitor {
	return &EvaluateVisitor{
		state:          state,
		sess:           s,
		objectResolver: objectResolver,
		registry:       operators.NewDefaultRegistry(),
	}
}

func (v *EvaluateVisitor) withState(
	state any,
	objectResolver IObjectResolver,
	fc *fieldContext,
) *EvaluateVisitor {
	resolver := objectResolver
	if resolver == nil {
		resolver = v.objectResolver
	}
	return &EvaluateVisitor{
		state:          state,
		sess:           v.sess,
		objectResolver: resolver,
		fieldCtx:       fc,
		registry:       v.registry,
	}
}

func (v *EvaluateVisitor) VisitEq(op EqOperator) (any, error) {
	return reflect.DeepEqual(v.state, op.Value), nil
}

func (v *EvaluateVisitor) VisitComparison(op ComparisonOperator) (any, error) {
	var regOp operators.Operator
	switch op.Op {
	case "$ne":
		regOp = operators.OperatorNe
	case "$gt":
		regOp = operators.OperatorGt
	case "$gte":
		regOp = operators.OperatorGte
	case "$lt":
		regOp = operators.OperatorLt
	case "$lte":
		regOp = operators.OperatorLte
	default:
		return false, nil
	}
	result, err := v.registry.ExecBinary(v.state, regOp, op.Value)
	if err != nil {
		return false, err
	}
	if result == nil {
		return false, nil
	}
	return result.(bool), nil
}

func (v *EvaluateVisitor) VisitIn(op InOperator) (any, error) {
	for _, val := range op.Values {
		if reflect.DeepEqual(v.state, val) {
			return true, nil
		}
	}
	return false, nil
}

func (v *EvaluateVisitor) VisitIsNull(op IsNullOperator) (any, error) {
	return (v.state == nil) == op.Value, nil
}

func (v *EvaluateVisitor) VisitNot(op NotOperator) (any, error) {
	evaluator := v.withState(v.state, nil, v.fieldCtx)
	result, err := op.Operand.Accept(evaluator)
	if err != nil {
		return false, err
	}
	return !result.(bool), nil
}

func (v *EvaluateVisitor) VisitAnyElement(op AnyElementOperator) (any, error) {
	items, ok := toSlice(v.state)
	if !ok {
		return false, nil
	}
	for _, item := range items {
		evaluator := v.withState(item, nil, nil)
		result, err := op.Query.Accept(evaluator)
		if err != nil {
			return false, err
		}
		if result.(bool) {
			return true, nil
		}
	}
	return false, nil
}

func (v *EvaluateVisitor) VisitAllElements(op AllElementsOperator) (any, error) {
	items, ok := toSlice(v.state)
	if !ok {
		return false, nil
	}
	for _, item := range items {
		evaluator := v.withState(item, nil, nil)
		result, err := op.Query.Accept(evaluator)
		if err != nil {
			return false, err
		}
		if !result.(bool) {
			return false, nil
		}
	}
	return true, nil
}

func (v *EvaluateVisitor) VisitLen(op LenOperator) (any, error) {
	items, ok := toSlice(v.state)
	if !ok {
		return false, nil
	}
	evaluator := v.withState(len(items), nil, nil)
	return op.Query.Accept(evaluator)
}

func (v *EvaluateVisitor) VisitAnd(op AndOperator) (any, error) {
	for _, operand := range op.Operands {
		evaluator := v.withState(v.state, nil, v.fieldCtx)
		result, err := operand.Accept(evaluator)
		if err != nil {
			return false, err
		}
		if !result.(bool) {
			return false, nil
		}
	}
	return true, nil
}

func (v *EvaluateVisitor) VisitOr(op OrOperator) (any, error) {
	for _, operand := range op.Operands {
		evaluator := v.withState(v.state, nil, v.fieldCtx)
		result, err := operand.Accept(evaluator)
		if err != nil {
			return false, err
		}
		if result.(bool) {
			return true, nil
		}
	}
	return false, nil
}

func (v *EvaluateVisitor) VisitRel(op RelOperator) (any, error) {
	if v.objectResolver != nil {
		var field *string
		var fkValue any
		if v.fieldCtx != nil {
			field = &v.fieldCtx.field
			fkValue = v.fieldCtx.fkValue
		} else {
			field = nil
			fkValue = v.state
		}
		foreignState, nestedResolver, err := v.objectResolver.Resolve(v.sess, field, fkValue)
		if err != nil {
			return false, err
		}
		if foreignState == nil {
			return false, nil
		}
		nested := v.withState(foreignState, nestedResolver, nil)
		return op.Query.Accept(nested)
	}
	return op.Query.Accept(v)
}

func (v *EvaluateVisitor) VisitComposite(op CompositeQuery) (any, error) {
	if !isStructLike(v.state) {
		return false, nil
	}
	for field, fieldOp := range op.Fields {
		fieldValue, _ := getFieldValue(v.state, field)
		if relOp, isRel := fieldOp.(RelOperator); isRel && v.objectResolver != nil {
			f := field
			foreignState, nestedResolver, err := v.objectResolver.Resolve(v.sess, &f, fieldValue)
			if err != nil {
				return false, err
			}
			if foreignState == nil {
				return false, nil
			}
			nested := v.withState(foreignState, nestedResolver, nil)
			result, err := relOp.Query.Accept(nested)
			if err != nil {
				return false, err
			}
			if !result.(bool) {
				return false, nil
			}
		} else {
			var descended IObjectResolver
			if v.objectResolver != nil {
				descended = v.objectResolver.Descend(field)
			}
			evaluator := v.withState(fieldValue, descended, &fieldContext{field: field, fkValue: fieldValue})
			result, err := fieldOp.Accept(evaluator)
			if err != nil {
				return false, err
			}
			if !result.(bool) {
				return false, nil
			}
		}
	}
	return true, nil
}
