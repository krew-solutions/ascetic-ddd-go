package specification

import (
	"errors"
	"fmt"
	"strings"

	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

var (
	ErrCompositeExpressionsDifferentLength   = errors.New("composite expressions have different length")
	ErrCompositeExpressionIsEmpty            = errors.New("a composite expression has no parts")
	ErrCompositeExpressionWhereANodeIsNeeded = errors.New("a composite expression where a single one is needed: only = and != compare composites")
)

// Mapping is what a domain's members and values are in the storage: Fowler's
// Metadata Mapping for a Query Object.
//
// A mapping is of the aggregate's members and knows nothing of any query: it
// is asked about a member by its whole path from the candidate,
// ["categories", "products", "price"] for the price of a product of a
// category, and answers what that is in the storage as a path from the
// candidate's row, categories.products.price_cents. A collection is a member
// like any other, ["categories", "products"]. Where a query stands when it
// asks - inside which collection's predicate, how far out an item is - is the
// tree's, and the MappingVisitor puts the answer there: the member of an item
// is the answer less the collection's, from the item.
//
// It used to be asked about the members of "the item" by their names alone,
// ItemAttrNode, so a mapping could not tell the items of one collection from
// another's; and about where a collection is kept, which is the schema's to
// say.
//
// A mapping is a guard as well. A specification may arrive as data - a
// template bound from a request, a tree from another service - and the
// mapping names every member such a specification may filter by, and refuses
// any other: a member the mapping does not know is its error, not a column
// that happens to exist. So what reaches the database is a query over the
// columns the repository chose to expose, through the relations it declared
// in the SchemaRegistry, and not whatever a caller composed to read another
// table or to scan a column without an index. The transformer is the one
// place for that: the compiler writes the names it is given. The size of a
// tree is bounded by the parsers - its height and its nesting - and its
// members by the mapping.
type Mapping interface {
	// AttrNode maps the member at path, a path from the candidate, to what
	// it is in the storage: a node, or a composite of them.
	AttrNode(path []string) (Mapped, error)
	// ValueNode maps a value of the domain to what it is in the storage: a
	// node, or a composite of them.
	ValueNode(val any) (Mapped, error)
}

func objectChain(root s.EmptiableObject, names []string) s.EmptiableObject {
	result := root
	for _, name := range names {
		result = s.Object(result, name)
	}
	return result
}

// collection is a collection whose predicate is being mapped: its whole path
// from the candidate, in the domain's names and in the storage's.
type collection struct {
	domain  []string
	storage []string
}

// placed returns the storage's path of a member, put where the member was.
//
// Of a member of an item the mapping's answer is a path from the candidate,
// which starts with the collection's; the rest is the member from the item,
// depth collections out. A part of a composite is put so as a whole, and a
// value is what it is. An answer that does not start with the collection's
// is an error: the mapping put the member outside its collection.
func placed(mapped Mapped, c collection, depth int) (Mapped, error) {
	switch m := mapped.(type) {
	case CompositeExpressionNode:
		parts := make([]Mapped, 0, len(m.nodes))
		for _, part := range m.nodes {
			placedPart, err := placed(part, c, depth)
			if err != nil {
				return nil, err
			}
			parts = append(parts, placedPart)
		}
		return CompositeExpression(parts...), nil
	case ScalarExpression:
		node, err := placedNode(m.node, c, depth)
		if err != nil {
			return nil, err
		}
		return Scalar(node), nil
	}
	return mapped, nil
}

// placedNode puts every path from the candidate in node at the item depth
// collections out, less the collection's storage path.
func placedNode(node s.Visitable, c collection, depth int) (s.Visitable, error) {
	switch n := node.(type) {
	case s.FieldNode:
		if _, ok := s.ExtractFieldRoot(n).(s.GlobalScopeNode); !ok {
			return n, nil
		}
		names := s.ExtractFieldPath(n)
		prefix := c.storage
		if len(names) <= len(prefix) || !equalNames(names[:len(prefix)], prefix) {
			return nil, fmt.Errorf(
				"the mapping put a member of an item outside its collection: %s is not under %s",
				strings.Join(names, "."), strings.Join(prefix, "."),
			)
		}
		rest := names[len(prefix):]
		return s.Field(objectChain(s.OuterItem(depth), rest[:len(rest)-1]), rest[len(rest)-1]), nil
	case s.PrefixNode:
		operand, err := placedNode(n.Operand(), c, depth)
		if err != nil {
			return nil, err
		}
		return s.NewPrefixNode(n.Operator(), operand, n.Associativity()), nil
	case s.PostfixNode:
		operand, err := placedNode(n.Operand(), c, depth)
		if err != nil {
			return nil, err
		}
		return s.NewPostfixNode(operand, n.Operator(), n.Associativity()), nil
	case s.InfixNode:
		left, err := placedNode(n.Left(), c, depth)
		if err != nil {
			return nil, err
		}
		right, err := placedNode(n.Right(), c, depth)
		if err != nil {
			return nil, err
		}
		return s.NewInfixNode(left, n.Operator(), right, n.Associativity()), nil
	}
	return node, nil
}

func equalNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// node returns a node, which a composite is not. Where one node is needed -
// under any operator but = and !=, as a predicate, as the whole
// specification - a composite is an error.
func node(mapped Mapped) (s.Visitable, error) {
	scalar, ok := mapped.(ScalarExpression)
	if !ok {
		return nil, ErrCompositeExpressionWhereANodeIsNeeded
	}
	return scalar.Node(), nil
}

// MappingVisitor applies a mapping: the domain's specification tree becomes
// the storage's, its members and values as the mapping has them. Each visit
// method returns what its node is mapped to - a node, or, for a field or a
// value that the mapping maps so, a composite, which the VisitInfix above it
// turns into nodes.
type MappingVisitor struct {
	mapping Mapping
	// inside is the collections the expression is inside of, the nearest
	// last: the item depth collections out is of inside[len-1-depth].
	inside []collection
}

func NewMappingVisitor(mapping Mapping) *MappingVisitor {
	return &MappingVisitor{mapping: mapping}
}

// collectionOf returns the collection whose item is item.
func (v *MappingVisitor) collectionOf(item s.ItemNode) (collection, error) {
	if item.Depth() >= len(v.inside) {
		return collection{}, fmt.Errorf("no current item in context: the item %d collections out", item.Depth())
	}
	return v.inside[len(v.inside)-1-item.Depth()], nil
}

// whole returns the whole path from the candidate of the member at names
// from root.
func (v *MappingVisitor) whole(root s.EmptiableObject, names []string) ([]string, error) {
	item, ok := root.(s.ItemNode)
	if !ok {
		return names, nil
	}
	c, err := v.collectionOf(item)
	if err != nil {
		return nil, err
	}
	return append(append([]string{}, c.domain...), names...), nil
}

// Transform is the typed entry point. Of a part of the tree the result may be
// a composite; of the whole tree it may not.
func (v *MappingVisitor) Transform(exp s.Visitable) (s.Visitable, error) {
	return v.node(exp)
}

// node transforms an operand that must be a single node.
func (v *MappingVisitor) node(exp s.Visitable) (s.Visitable, error) {
	mapped, err := s.Accept[Mapped](exp, v)
	if err != nil {
		return nil, err
	}
	return node(mapped)
}

func (v *MappingVisitor) VisitGlobalScope(n s.GlobalScopeNode) (Mapped, error) {
	return Scalar(n), nil
}

func (v *MappingVisitor) VisitObject(n s.ObjectNode) (Mapped, error) {
	return Scalar(n), nil
}

// VisitCollection maps the collection and its predicate. A collection is a
// member: the mapping says where it is, by a path from the candidate, and the
// predicate is mapped inside it. It used to be a passthrough.
func (v *MappingVisitor) VisitCollection(n s.CollectionNode) (Mapped, error) {
	root := s.ExtractObjectRoot(n.Parent())
	domain, err := v.whole(root, s.ExtractObjectPath(n.Parent()))
	if err != nil {
		return nil, err
	}
	mapped, err := v.mapping.AttrNode(domain)
	if err != nil {
		return nil, err
	}
	place, ok := mapped.(ScalarExpression)
	var field s.FieldNode
	if ok {
		field, ok = place.node.(s.FieldNode)
	}
	if !ok || !isGlobal(s.ExtractFieldRoot(field)) {
		return nil, fmt.Errorf(
			"the mapping answered for a collection with what is not a place: %s", strings.Join(domain, "."),
		)
	}
	storage := s.ExtractFieldPath(field)
	c := collection{domain: domain, storage: storage}
	var parent s.EmptiableObject
	if item, isItem := root.(s.ItemNode); isItem {
		enclosing, err := v.collectionOf(item)
		if err != nil {
			return nil, err
		}
		placedField, err := placedNode(field, enclosing, item.Depth())
		if err != nil {
			return nil, err
		}
		fieldNode := placedField.(s.FieldNode)
		parent = s.Object(fieldNode.Object(), fieldNode.Name())
	} else {
		parent = objectChain(s.GlobalScope(), storage)
	}
	inside := &MappingVisitor{mapping: v.mapping, inside: append(append([]collection{}, v.inside...), c)}
	predicate, err := inside.node(n.Predicate())
	if err != nil {
		return nil, err
	}
	return Scalar(s.Wildcard(parent, predicate)), nil
}

func isGlobal(root s.EmptiableObject) bool {
	_, ok := root.(s.GlobalScopeNode)
	return ok
}

func (v *MappingVisitor) VisitItem(n s.ItemNode) (Mapped, error) {
	return Scalar(n), nil
}

// VisitField maps a member by its whole path from the candidate, and puts
// the answer where the member was.
func (v *MappingVisitor) VisitField(n s.FieldNode) (Mapped, error) {
	root := s.ExtractFieldRoot(n)
	path, err := v.whole(root, s.ExtractFieldPath(n))
	if err != nil {
		return nil, err
	}
	mapped, err := v.mapping.AttrNode(path)
	if err != nil {
		return nil, err
	}
	if item, ok := root.(s.ItemNode); ok {
		c, err := v.collectionOf(item)
		if err != nil {
			return nil, err
		}
		return placed(mapped, c, item.Depth())
	}
	return mapped, nil
}

func (v *MappingVisitor) VisitValue(n s.ValueNode) (Mapped, error) {
	// An Option of a value is the value, which the mapping maps, or the null
	// it is in any storage: a mapping is asked of the domain's values.
	value := operators.ReadOption(n.Value())
	if value == nil && n.Value() != nil {
		return Scalar(s.Value(nil)), nil
	}
	return v.mapping.ValueNode(value)
}

func (v *MappingVisitor) VisitPrefix(n s.PrefixNode) (Mapped, error) {
	operand, err := v.node(n.Operand())
	if err != nil {
		return nil, err
	}
	return Scalar(s.NewPrefixNode(n.Operator(), operand, n.Associativity())), nil
}

func (v *MappingVisitor) VisitPostfix(n s.PostfixNode) (Mapped, error) {
	operand, err := v.node(n.Operand())
	if err != nil {
		return nil, err
	}
	return Scalar(s.NewPostfixNode(operand, n.Operator(), n.Associativity())), nil
}

// nullTest returns the null test an equality stands for, if it stands for one.
//
// A value of the domain that the storage keeps as a null - a special case that
// answers for itself, `discount == NoDiscount` - is equal to itself in the
// domain, and `discount = $1` with a null is true of nothing. It is tested
// for: IS NULL, and IS NOT NULL of `!=`.
//
// Only a null the mapping made: a value that was null in the domain already
// stays compared, as it is in the tree. The mapping maps operands and knows
// nothing of operators; it is here, where both operands are mapped and the
// node is built, that the operator is seen.
func nullTest(n s.InfixNode, left, right Mapped) (s.Visitable, bool) {
	test := s.IsNull
	switch n.Operator() {
	case operators.OperatorEq:
	case operators.OperatorNe:
		test = s.IsNotNull
	default:
		return nil, false
	}
	for _, side := range [][3]any{{n.Right(), right, left}, {n.Left(), left, right}} {
		if !madeNull(side[0].(s.Visitable), side[1].(Mapped)) {
			continue
		}
		other, ok := side[2].(ScalarExpression)
		if !ok {
			continue
		}
		return test(other.Node()), true
	}
	return nil, false
}

// madeNull tells whether the mapping made a null of an operand that was a
// value, and not a null, in the domain.
func madeNull(operand s.Visitable, mapped Mapped) bool {
	value, ok := operand.(s.ValueNode)
	if !ok || operators.IsNull(operators.ReadOption(value.Value())) {
		return false
	}
	scalar, ok := mapped.(ScalarExpression)
	if !ok {
		return false
	}
	made, ok := scalar.Node().(s.ValueNode)
	return ok && operators.IsNull(made.Value())
}

func (v *MappingVisitor) VisitInfix(n s.InfixNode) (Mapped, error) {
	left, err := s.Accept[Mapped](n.Left(), v)
	if err != nil {
		return nil, err
	}
	right, err := s.Accept[Mapped](n.Right(), v)
	if err != nil {
		return nil, err
	}

	// Equality with a value the mapping made the storage's null is the null
	// test of the other operand.
	if tested, ok := nullTest(n, left, right); ok {
		return Scalar(tested), nil
	}

	// A composite on either side: one on the right alone used to go into the
	// tree as it was.
	leftComposite, leftIs := left.(CompositeExpressionNode)
	rightComposite, rightIs := right.(CompositeExpressionNode)
	if leftIs || rightIs {
		if !leftIs || !rightIs {
			return nil, errors.New("not enough composite expressions")
		}
		var result s.Visitable
		switch n.Operator() {
		case operators.OperatorEq:
			result, err = leftComposite.Equal(rightComposite)
		case operators.OperatorNe:
			result, err = leftComposite.NotEqual(rightComposite)
		default:
			return nil, fmt.Errorf("operator %q is not supported for composite expressions", n.Operator())
		}
		if err != nil {
			return nil, err
		}
		return Scalar(result), nil
	}

	leftNode, err := node(left)
	if err != nil {
		return nil, err
	}
	rightNode, err := node(right)
	if err != nil {
		return nil, err
	}
	return Scalar(s.NewInfixNode(leftNode, n.Operator(), rightNode, n.Associativity())), nil
}
