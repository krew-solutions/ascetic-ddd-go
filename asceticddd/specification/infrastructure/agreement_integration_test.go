package specification

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/option"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/utils/testutils"
)

// The evaluator and PostgreSQL, on the same specifications: a specification
// has these two readers, and they must agree.

// errRollback undoes what a test did to the database.
var errRollback = errors.New("rollback")

// failureOf names the failure of the evaluator as PostgreSQL names its own.
func failureOf(err error) string {
	switch {
	case strings.Contains(err.Error(), "out of range"):
		return "22003" // numeric_value_out_of_range
	case strings.Contains(err.Error(), "by zero"):
		return "22012" // division_by_zero
	case strings.Contains(err.Error(), "not supported"):
		return "42883" // undefined_function
	default:
		return err.Error()
	}
}

func constantCases() []s.Visitable {
	v := s.Value
	null := func() s.Visitable { return s.Value(nil) }
	t, f := func() s.Visitable { return s.Value(true) }, func() s.Visitable { return s.Value(false) }
	min, max := math.MinInt64, math.MaxInt64
	nan := math.NaN()
	seven, half, yes, no := 7, 0.5, true, false
	var noNumber *int
	var noFlag *bool
	written := []s.Visitable{
		// Arithmetic, and the parentheses that keep its shape
		s.Sub(v(10), s.Sub(v(4), v(3))),
		s.Sub(s.Sub(v(10), v(4)), v(3)),
		s.Sub(v(10), s.Add(v(4), v(3))),
		s.Div(v(100), s.Div(v(10), v(5))),
		s.Div(s.Mul(v(7), v(3)), v(2)),
		s.Mul(s.Add(v(1), v(2)), v(3)),
		s.Add(v(1), s.Mul(v(2), v(3))),
		s.Div(v(7), v(2)),
		s.Div(v(-7), v(2)),
		s.Mod(v(-7), v(2)),
		s.Mod(v(7), v(-2)),
		s.Mod(v(min), v(-1)),
		s.Neg(s.Neg(v(5))),
		s.Sub(v(5), s.Neg(v(3))),
		s.Neg(s.Add(v(1), v(2))),
		s.Div(v(7.0), v(2)),
		s.Add(v(1), v(0.5)),
		s.Mul(v(2.5), v(4)),
		s.Add(v(int64(1)), v(2)),
		// Pointers: optional values, as the driver takes them
		s.Add(v(&seven), v(2)),
		s.Sub(v(&seven), v(&seven)),
		s.Equal(v(&seven), v(7)),
		s.LessThan(v(&half), v(&seven)),
		s.Equal(v(noNumber), v(1)),
		s.Add(v(1), v(noNumber)),
		s.IsNull(v(noNumber)),
		s.IsNotNull(v(&seven)),
		s.Is(v(noNumber), v(noNumber)),
		s.Is(v(&seven), v(noNumber)),
		s.And(v(noFlag), v(&no)),
		s.Or(v(noFlag), v(&yes)),
		s.Not(v(noFlag)),
		s.And(v(&yes), s.Not(v(&no))),
		s.LeftShift(v(1), v(3)),
		s.LeftShift(v(1), v(64)),
		s.LeftShift(v(1), v(-1)),
		s.LeftShift(v(1), v(63)),
		s.RightShift(v(8), v(65)),
		s.RightShift(v(-8), v(1)),
		// Where it fails
		s.Div(v(1), v(0)),
		s.Mod(v(1), v(0)),
		s.Div(v(1.0), v(0.0)),
		s.Add(v(max), v(1)),
		s.Sub(v(min), v(1)),
		s.Mul(v(max), v(2)),
		s.Div(v(min), v(-1)),
		s.Neg(v(min)),
		s.Mul(v(math.MaxFloat64), v(2.0)),
		// What is not defined here is not defined there
		s.Add(v("a"), v("b")),
		s.Mod(v(5.5), v(2)),
		s.Add(v(true), v(1)),
		s.Neg(v("a")),
		s.Equal(v("a"), v(1)),
		s.LessThan(v(true), v(2)),
		s.Is(v("a"), v(1)),
		// Comparisons
		s.Equal(v(1), v(1.0)),
		s.LessThan(v(1), v(1.5)),
		s.GreaterThanEqual(v(2), v(2)),
		s.LessThanEqual(v(3), v(2)),
		s.NotEqual(v("a"), v("b")),
		s.LessThan(v("a"), v("b")),
		s.GreaterThan(v(true), v(false)),
		s.Equal(v(nan), v(nan)),
		s.GreaterThan(v(nan), v(math.MaxFloat64)),
		s.LessThanEqual(v(1.0), v(nan)),
		s.Equal(v(math.Copysign(0, -1)), v(0.0)),
		s.Equal(s.Equal(v(1), v(1)), v(true)),
		s.Equal(v(true), s.Equal(v(1), v(2))),
		// Nulls
		s.Equal(null(), v(1)),
		s.Equal(null(), null()),
		s.NotEqual(v(1), null()),
		s.Add(v(1), null()),
		s.Neg(null()),
		s.Div(null(), v(0)),
		s.Not(null()),
		s.And(null(), f()),
		s.And(f(), null()),
		s.And(null(), t()),
		s.And(null(), null()),
		s.Or(null(), t()),
		s.Or(t(), null()),
		s.Or(null(), f()),
		s.And(s.Or(t(), f()), f()),
		s.Or(t(), s.And(f(), f())),
		s.And(t(), s.And(t(), f())),
		s.Not(s.And(t(), f())),
		s.Not(s.Not(t())),
		s.IsNull(s.Or(null(), f())),
		s.IsNull(s.IsNull(null())),
		s.Equal(s.IsNull(null()), t()),
		s.IsNull(s.Equal(v(1), null())),
		s.IsNotNull(s.Equal(v(1), null())),
		s.Not(s.IsNull(null())),
		// IS
		s.Is(t(), t()),
		s.Is(t(), f()),
		s.Is(null(), null()),
		s.Is(null(), t()),
		s.Is(v(1), null()),
		s.Is(v(1), v(1)),
		s.Equal(s.Is(t(), null()), f()),
		s.Is(s.Equal(v(1), v(1)), t()),
	}

	// Floats at their edges, every pair under every operator. What the server
	// makes of each - a value, "out of range" for a result too large or too
	// small to be one, "division by zero" - is the server's to say, and the
	// evaluator's to repeat: a zero from operands that are not zero is an
	// underflow, a NaN divided by zero is a NaN, one divided by infinity is a
	// zero and no underflow.
	edges := []float64{
		0, 1, -1, 1e300, 1e-300, math.MaxFloat64, 2.2250738585072014e-308,
		math.Inf(1), math.Inf(-1), nan,
	}
	operators := []func(left, right s.Visitable) s.Visitable{
		func(l, r s.Visitable) s.Visitable { return s.Add(l, r) },
		func(l, r s.Visitable) s.Visitable { return s.Sub(l, r) },
		func(l, r s.Visitable) s.Visitable { return s.Mul(l, r) },
		func(l, r s.Visitable) s.Visitable { return s.Div(l, r) },
	}
	for _, left := range edges {
		for _, right := range edges {
			for _, operator := range operators {
				written = append(written, operator(s.Value(left), s.Value(right)))
			}
		}
	}
	return written
}

// same tells whether the evaluator's value is the one PostgreSQL answered.
func same(evaluated, answered any) bool {
	evaluated = operators.Indirect(evaluated)
	if i, ok := evaluated.(int); ok {
		evaluated = int64(i)
	}
	if e, ok := evaluated.(float64); ok && math.IsNaN(e) {
		a, ok := answered.(float64)
		return ok && math.IsNaN(a) // NaN is what both have
	}
	return reflect.DeepEqual(evaluated, answered)
}

func withConnection(t *testing.T, do func(tx session.Session, conn session.DbConnection) error) {
	t.Helper()
	pool, err := testutils.NewPgSessionPool()
	if err != nil {
		t.Fatalf("Failed to create session pool: %v", err)
	}
	err = pool.Session(context.Background(), func(sess session.Session) error {
		return sess.Atomic(func(tx session.Session) error {
			if err := do(tx, tx.(session.DbSession).Connection()); err != nil {
				return err
			}
			return errRollback
		})
	})
	if err != nil && !errors.Is(err, errRollback) {
		t.Fatal(err)
	}
}

func TestAConstantExpressionHasOneValueForBothReaders(t *testing.T) {
	withConnection(t, func(tx session.Session, _ session.DbConnection) error {
		for _, c := range constantCases() {
			sql, params, err := CompileToSQL(c)
			if err != nil {
				return err
			}
			// Sent as a user sends it, the types the server's to find: this
			// test used to write a type after every parameter itself, and so
			// did not see that where every operand is a constant the server
			// has nothing to find them by.
			query := "SELECT (" + sql + ")"

			evaluated, failure := s.Accept[any](c, s.NewEvaluateVisitor(rowContext{}, operators.NewDefaultRegistry()))

			// A transaction of its own: a failure is one of the answers, and
			// must not take the connection with it.
			var answered any
			refusal := tx.Atomic(func(probe session.Session) error {
				return probe.(session.DbSession).Connection().QueryRow(query, params...).Scan(&answered)
			})

			t.Run(fmt.Sprintf("%s %v", sql, params), func(t *testing.T) {
				var refused *pgconn.PgError
				switch {
				case errors.As(refusal, &refused):
					// The same failure, not just a failure.
					if failure == nil {
						t.Fatalf("PostgreSQL: %v; the evaluator has %v", refused, evaluated)
					}
					if failureOf(failure) != refused.Code {
						t.Errorf("PostgreSQL: %s %v; the evaluator: %v", refused.Code, refused.Message, failure)
					}
				case refusal != nil:
					t.Fatal(refusal)
				case failure != nil:
					t.Errorf("PostgreSQL has %v; the evaluator: %v", answered, failure)
				case !same(evaluated, answered):
					t.Errorf("PostgreSQL has %v (%T); the evaluator has %v (%T)", answered, answered, evaluated, evaluated)
				}
			})
		}
		return nil
	})
}

type storeRow struct {
	id         int
	a, b       any
	flag, name any
	items      [][2]any // price, active
}

func (r storeRow) context() s.Context {
	items := make([]s.Context, 0, len(r.items))
	for _, it := range r.items {
		items = append(items, rowContext{
			"price": it[0], "active": it[1],
			// A Value Object inside the item, which the storage keeps as a
			// composite inside the item's row.
			"maker": rowContext{"name": makerName(it[0])},
			// An object of its own, which the storage keeps in a table of
			// its own and the item refers to by a key.
			"owner": rowContext{"name": ownerOf(it[1]).name},
		})
	}
	return rowContext{
		"id": r.id, "a": r.a, "b": r.b, "flag": r.flag, "name": r.name,
		// The store's owner, kept as the owners of items are.
		"owner": rowContext{"name": ownerOf(r.flag).name},
		// Members named as PostgreSQL names other things, under columns of
		// those very names: `user` is the session's user if it is not quoted,
		// `order` does not parse, `createdAt` folds to `createdat`.
		"user": r.name, "order": r.a, "createdAt": r.b,
		"items": s.NewCollectionContext(items),
	}
}

// pointer returns a pointer to the value, and a nil pointer of the type the
// column has for a null: a row as a Go struct with nullable fields holds it.
func pointer[T any](value any) any {
	if value == nil {
		return (*T)(nil)
	}
	typed := value.(T)
	return &typed
}

// pointerContext is the row with a pointer for each nullable member.
func (r storeRow) pointerContext() s.Context {
	items := make([]s.Context, 0, len(r.items))
	for _, it := range r.items {
		items = append(items, rowContext{
			"price": pointer[int](it[0]), "active": pointer[bool](it[1]),
			"maker": rowContext{"name": pointer[string](makerName(it[0]))},
			"owner": rowContext{"name": pointer[string](ownerOf(it[1]).name)},
		})
	}
	return rowContext{
		"id": r.id, "a": pointer[int](r.a), "b": pointer[int](r.b),
		"flag": pointer[bool](r.flag), "name": pointer[string](r.name),
		"owner": rowContext{"name": pointer[string](ownerOf(r.flag).name)},
		"user":  pointer[string](r.name), "order": pointer[int](r.a), "createdAt": pointer[int](r.b),
		"items": s.NewCollectionContext(items),
	}
}

// makerName is the name of the maker of an item of this price.
func makerName(price any) any {
	switch p := price.(type) {
	case int:
		if p > 500 {
			return "dear"
		}
		return "cheap"
	default:
		return nil
	}
}

// owner is the key and the name of an owner. The third owner has no name.
type owner struct {
	id   int
	name any
}

func ownerOf(known any) owner {
	switch known {
	case true:
		return owner{1, "ann"}
	case false:
		return owner{2, "bob"}
	default:
		return owner{3, nil}
	}
}

var storeRows = []storeRow{
	{1, 1, 1, true, "one", [][2]any{{900, true}, {10, false}}},
	{2, 1, 2, false, "two", [][2]any{{10, true}}},
	{3, nil, 2, nil, nil, [][2]any{{nil, true}, {10, nil}}},
	{4, nil, nil, true, "four", nil},
	{5, 7, nil, false, "five", [][2]any{{nil, nil}}},
	{6, -3, 0, nil, "", [][2]any{{900, nil}, {901, true}}},
}

func rowSpecifications() []s.Visitable {
	null := func() s.Visitable { return s.Value(nil) }
	dear := func() s.Visitable { return s.GreaterThan(item("price"), s.Value(500)) }
	some := func(predicate s.Visitable) s.Visitable {
		return s.Wildcard(s.Object(s.GlobalScope(), "items"), predicate)
	}
	every := func(predicate s.Visitable) s.Visitable { return s.Not(some(s.Not(predicate))) }
	makerNameOfItem := func() s.Visitable { return s.Field(s.Object(s.Item(), "maker"), "name") }
	ownerNameOfItem := func() s.Visitable { return s.Field(s.Object(s.Item(), "owner"), "name") }
	ownerNameOfStore := func() s.Visitable { return s.Field(s.Object(s.GlobalScope(), "owner"), "name") }
	return []s.Visitable{
		s.Equal(field("a"), field("b")),
		s.Not(s.Equal(field("a"), field("b"))),
		s.NotEqual(field("a"), field("b")),
		s.Is(field("a"), field("b")),
		s.Not(s.Is(field("a"), field("b"))),
		s.IsNull(field("a")),
		s.And(s.IsNotNull(field("a")), s.IsNull(field("b"))),
		s.Or(s.Equal(field("a"), field("b")), field("flag")),
		s.And(s.Not(field("flag")), s.GreaterThan(field("b"), s.Value(1))),
		s.Not(s.Or(field("flag"), s.IsNull(field("name")))),
		s.GreaterThan(s.Sub(field("a"), s.Sub(field("b"), s.Value(1))), s.Value(0)),
		s.LessThan(s.Mul(s.Add(field("a"), s.Value(1)), s.Value(2)), s.Value(5)),
		s.Equal(s.IsNull(field("a")), field("flag")),
		s.Equal(field("name"), s.Value("")),
		s.LessThan(field("name"), s.Value("one")),
		s.Is(field("flag"), null()),
		some(dear()),
		s.Not(some(dear())),
		some(s.Or(dear(), item("active"))),
		some(s.And(dear(), item("active"))),
		some(s.Not(item("active"))),
		some(s.IsNull(item("price"))),
		some(s.GreaterThan(item("price"), field("a"))),
		every(item("active")),
		every(s.GreaterThan(item("price"), s.Value(5))),
		s.Not(every(s.IsNotNull(item("price")))),
		s.And(field("flag"), some(dear())),
		// A member of a Value Object inside the item: a composite inside the
		// item's row, in the array and in the table alike.
		some(s.Equal(makerNameOfItem(), s.Value("dear"))),
		some(s.And(s.IsNull(makerNameOfItem()), item("active"))),
		every(s.NotEqual(makerNameOfItem(), s.Value("cheap"))),
		// A member of an object referred to by a key: the schema says
		// `items.owner`, and `owner`, are kept in a table of their own.
		some(s.Equal(ownerNameOfItem(), s.Value("ann"))),
		some(s.And(s.IsNull(ownerNameOfItem()), dear())),
		every(s.NotEqual(ownerNameOfItem(), s.Value("bob"))),
		some(s.Equal(ownerNameOfItem(), makerNameOfItem())),
		// The same of the candidate itself, and both in one predicate.
		s.Equal(ownerNameOfStore(), s.Value("bob")),
		s.And(s.IsNull(ownerNameOfStore()), s.IsNotNull(field("a"))),
		some(s.Equal(ownerNameOfItem(), ownerNameOfStore())),
		// Constants with nothing but constants beside them: their types are
		// said in the text, for the server has nothing to find them by.
		s.GreaterThan(field("a"), s.Sub(s.Value(4), s.Value(3))),
		some(s.GreaterThan(item("price"), s.Mul(s.Value(100), s.Value(5)))),
		s.LessThan(field("a"), s.Neg(s.Value(-2))),
		s.Or(s.IsNull(null()), field("flag")),
		// A name is the column's, whatever else PostgreSQL knows by it.
		s.Equal(field("user"), s.Value("one")),
		s.GreaterThan(field("order"), s.Value(0)),
		s.Equal(field("createdAt"), s.Value(2)),
	}
}

func literal(value any) string {
	switch v := value.(type) {
	case nil:
		return "NULL"
	case string:
		return "'" + v + "'"
	default:
		return fmt.Sprint(v)
	}
}

func makeTables(conn session.DbConnection) error {
	statements := []string{
		"CREATE TABLE spec_owners (id bigint PRIMARY KEY, name text)",
		"INSERT INTO spec_owners VALUES (1, 'ann'), (2, 'bob'), (3, NULL)",
		`CREATE TABLE spec_stores (id bigint PRIMARY KEY, a bigint, b bigint, flag boolean, name text, "user" text, "order" bigint, "createdAt" bigint, owner_id bigint)`,
		"CREATE TYPE spec_maker AS (name text)",
		"CREATE TABLE spec_items (store_id bigint, price bigint, active boolean, maker spec_maker, owner_id bigint REFERENCES spec_owners)",
		"CREATE TYPE spec_item AS (price bigint, active boolean, maker spec_maker, owner_id bigint)",
		`CREATE TABLE spec_stores_embedded (id bigint PRIMARY KEY, a bigint, b bigint, flag boolean, name text, "user" text, "order" bigint, "createdAt" bigint, owner_id bigint, items spec_item[])`,
	}
	for _, row := range storeRows {
		columns := fmt.Sprintf(
			"%d, %s, %s, %s, %s, %s, %s, %s, %d",
			row.id, literal(row.a), literal(row.b), literal(row.flag), literal(row.name),
			literal(row.name), literal(row.a), literal(row.b), ownerOf(row.flag).id,
		)
		embedded := make([]string, 0, len(row.items))
		for _, it := range row.items {
			statements = append(statements, fmt.Sprintf(
				"INSERT INTO spec_items VALUES (%d, %s, %s, ROW(%s)::spec_maker, %d)",
				row.id, literal(it[0]), literal(it[1]), literal(makerName(it[0])), ownerOf(it[1]).id,
			))
			embedded = append(embedded, fmt.Sprintf(
				"ROW(%s, %s, ROW(%s)::spec_maker, %d)::spec_item",
				literal(it[0]), literal(it[1]), literal(makerName(it[0])), ownerOf(it[1]).id,
			))
		}
		statements = append(statements,
			"INSERT INTO spec_stores VALUES ("+columns+")",
			"INSERT INTO spec_stores_embedded VALUES ("+columns+", ARRAY["+strings.Join(embedded, ", ")+"]::spec_item[])",
		)
	}
	for _, statement := range statements {
		if _, err := conn.Exec(statement); err != nil {
			return fmt.Errorf("%s: %w", statement, err)
		}
	}
	return nil
}

func selected(conn session.DbConnection, query string, params []any) ([]int, error) {
	rows, err := conn.Query(query, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int{}
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// discount is a Value Object, which a specification compares as a whole. Its
// special case answers for itself, as Fowler's Special Case does: it is equal
// to itself and to no discount, and less than any. Nothing of it is null to
// the evaluator.
type discount struct {
	percent int
	none    bool
}

func registerDiscounts(reg *operators.OperatorRegistry) {
	rank := func(d discount) int {
		if d.none {
			return -1
		}
		return d.percent
	}
	operators.RegisterBinary[discount, discount](reg, operators.OperatorEq, func(a, b discount) (any, error) { return a == b, nil })
	operators.RegisterBinary[discount, discount](reg, operators.OperatorNe, func(a, b discount) (any, error) { return a != b, nil })
	operators.RegisterBinary[discount, discount](reg, operators.OperatorGt, func(a, b discount) (any, error) { return rank(a) > rank(b), nil })
}

// discountsContext is what the storage has for them: a discount is its percent
// in a column, and the special case is that column's null.
type discountsContext struct{}

func (discountsContext) AttrNode(path []string) (Mapped, error) {
	// By the whole path from the candidate: the collection, and a member of
	// its item under it. Where the item is, is the tree's.
	switch strings.Join(path, ".") {
	case "items":
		return Scalar(field("items")), nil
	case "items.discount":
		return Scalar(fromCandidate("items", "discount_percent")), nil
	}
	return nil, fmt.Errorf("no such member: %s", strings.Join(path, "."))
}

// renamed is a mapping that renames each name of a path by a table, and
// leaves the values: the storage's name of a member, whatever leads to it.
type renamed map[string]string

func (r renamed) AttrNode(path []string) (Mapped, error) {
	names := make([]string, 0, len(path))
	for _, name := range path {
		if stored, ok := r[name]; ok {
			name = stored
		}
		names = append(names, name)
	}
	return Scalar(fromCandidate(names...)), nil
}

func (renamed) ValueNode(val any) (Mapped, error) {
	return Scalar(s.Value(val)), nil
}

func (discountsContext) ValueNode(val any) (Mapped, error) {
	if d, ok := val.(discount); ok {
		if d.none {
			return Scalar(s.Value(nil)), nil
		}
		return Scalar(s.Value(d.percent)), nil
	}
	return Scalar(s.Value(val)), nil
}

// A special case that answers for itself is equal to itself, and the storage
// has a null for it: `discount = $1` with a null is true of nothing, so the
// server found no shop where the evaluator found all three. Equality with a
// value the mapping made the storage's null is the null test.
//
// What stays the server's own: a null compared with a value is unknown to it,
// and so is the negation of that, where the special case answers false and
// true. A special case kept as a value, and not as a null, has none of this.
func TestEqualityWithASpecialCaseKeptAsANullIsTheNullTest(t *testing.T) {
	fifteen, none := discount{percent: 15}, discount{none: true}
	shop := func(discounts ...discount) s.Context {
		items := make([]s.Context, 0, len(discounts))
		for _, d := range discounts {
			items = append(items, rowContext{"discount": d})
		}
		return rowContext{"items": s.NewCollectionContext(items)}
	}
	shops := map[int]s.Context{1: shop(fifteen, none), 2: shop(none, fifteen), 3: shop(none)}
	member := s.Field(s.Item(), "discount")
	some := func(predicate s.Visitable) s.Visitable {
		return s.Wildcard(s.Object(s.GlobalScope(), "items"), predicate)
	}
	over := func(percent int) s.Visitable { return s.GreaterThan(member, s.Value(discount{percent: percent})) }
	cases := []struct {
		specification s.Visitable
		inMemory      []int
		onTheServer   []int
	}{
		{some(s.Equal(member, s.Value(none))), []int{1, 2, 3}, []int{1, 2, 3}},
		{some(s.Equal(s.Value(none), member)), []int{1, 2, 3}, []int{1, 2, 3}},
		{some(s.NotEqual(member, s.Value(none))), []int{1, 2}, []int{1, 2}},
		{some(over(10)), []int{1, 2}, []int{1, 2}},
		// The server's own logic of a null, which the null test does not reach.
		{some(s.Not(over(10))), []int{1, 2, 3}, []int{}},
		{some(s.NotEqual(member, s.Value(fifteen))), []int{1, 2, 3}, []int{}},
	}
	reg := operators.NewDefaultRegistry()
	registerDiscounts(reg)

	withConnection(t, func(_ session.Session, conn session.DbConnection) error {
		for _, statement := range []string{
			"CREATE TYPE spec_answering AS (price bigint, discount_percent bigint)",
			"CREATE TABLE spec_answering_shops (id bigint, items spec_answering[])",
			`INSERT INTO spec_answering_shops VALUES
				(1, ARRAY[ROW(900, 15), ROW(100, NULL)]::spec_answering[]),
				(2, ARRAY[ROW(100, NULL), ROW(900, 15)]::spec_answering[]),
				(3, ARRAY[ROW(100, NULL)]::spec_answering[])`,
		} {
			if _, err := conn.Exec(statement); err != nil {
				return fmt.Errorf("%s: %w", statement, err)
			}
		}
		for _, c := range cases {
			satisfied := []int{}
			for id := 1; id <= 3; id++ {
				ok, err := s.NewEvaluateVisitor(shops[id], reg).Evaluate(c.specification)
				if err != nil {
					return fmt.Errorf("the evaluator, on shop %d: %w", id, err)
				}
				if ok {
					satisfied = append(satisfied, id)
				}
			}
			sql, params, err := Compile(discountsContext{}, c.specification)
			if err != nil {
				return err
			}
			selected, err := selected(conn, "SELECT id FROM spec_answering_shops WHERE "+sql+" ORDER BY id", params)
			if err != nil {
				return fmt.Errorf("%s: %w", sql, err)
			}
			if !reflect.DeepEqual(satisfied, c.inMemory) || !reflect.DeepEqual(selected, c.onTheServer) {
				t.Errorf("%s %v: the evaluator %v, want %v; PostgreSQL %v, want %v", sql, params, satisfied, c.inMemory, selected, c.onTheServer)
			}
		}
		return nil
	})
}

// A member of an aggregate may be an Option of a Value Object,
// option.Some(discount{15}) or option.Nothing[discount](). The evaluator took
// the wrapper for the value: there is no operator of an Option, and IS NULL
// was false of a Nothing. It is read as what it holds, or as a null - where a
// value comes to a reader: from the candidate, and from a constant of the
// specification.
//
// A null on both sides, so the two readers agree on every specification, the
// negation of a comparison included: where a special case kept as a null
// differs from the server, an Option does not.
func TestAnOptionIsWhatItHoldsOrANull(t *testing.T) {
	fifteen, nothing := option.Some(discount{percent: 15}), option.Nothing[discount]()
	shop := func(discounts ...option.Option[discount]) s.Context {
		items := make([]s.Context, 0, len(discounts))
		for _, d := range discounts {
			items = append(items, rowContext{"discount": d})
		}
		return rowContext{"items": s.NewCollectionContext(items)}
	}
	shops := map[int]s.Context{1: shop(fifteen, nothing), 2: shop(nothing, fifteen), 3: shop(nothing)}
	member := s.Field(s.Item(), "discount")
	some := func(predicate s.Visitable) s.Visitable {
		return s.Wildcard(s.Object(s.GlobalScope(), "items"), predicate)
	}
	over := func(percent int) s.Visitable { return s.GreaterThan(member, s.Value(discount{percent: percent})) }
	bothOver := func(limit option.Option[discount]) s.Visitable {
		return s.And(s.IsNotNull(s.Value(limit)), s.And(s.IsNotNull(member), s.GreaterThan(member, s.Value(limit))))
	}
	cases := []struct {
		specification s.Visitable
		want          []int
	}{
		{some(over(10)), []int{1, 2}},
		{some(s.Equal(member, s.Value(discount{percent: 15}))), []int{1, 2}},
		{some(s.IsNull(member)), []int{1, 2, 3}},
		{s.Not(some(over(10))), []int{3}},
		// A null to both readers: unknown, and so is its negation.
		{some(s.Not(over(10))), []int{}},
		{some(s.NotEqual(member, s.Value(discount{percent: 15}))), []int{}},
		// A constant of the specification may be an Option as well.
		{some(s.Equal(member, s.Value(fifteen))), []int{1, 2}},
		{some(s.Is(member, s.Value(nothing))), []int{1, 2, 3}},
		{some(s.Equal(member, s.Value(nothing))), []int{}},
		// What two Options hold, asked under names - the tree IsSomeAnd is
		// generated as, one of the two from outside: of two values, so its
		// negation is true where a none makes it false.
		{some(bothOver(option.Some(discount{percent: 10}))), []int{1, 2}},
		{some(bothOver(nothing)), []int{}},
		{s.Not(some(bothOver(nothing))), []int{1, 2, 3}},
		{some(s.Not(bothOver(option.Some(discount{percent: 10})))), []int{1, 2, 3}},
	}
	reg := operators.NewDefaultRegistry()
	registerDiscounts(reg)

	withConnection(t, func(_ session.Session, conn session.DbConnection) error {
		for _, statement := range []string{
			"CREATE TYPE spec_optional AS (price bigint, discount_percent bigint)",
			"CREATE TABLE spec_optional_shops (id bigint, items spec_optional[])",
			`INSERT INTO spec_optional_shops VALUES
				(1, ARRAY[ROW(900, 15), ROW(100, NULL)]::spec_optional[]),
				(2, ARRAY[ROW(100, NULL), ROW(900, 15)]::spec_optional[]),
				(3, ARRAY[ROW(100, NULL)]::spec_optional[])`,
		} {
			if _, err := conn.Exec(statement); err != nil {
				return fmt.Errorf("%s: %w", statement, err)
			}
		}
		for _, c := range cases {
			satisfied := []any{}
			for id := 1; id <= 3; id++ {
				ok, err := s.NewEvaluateVisitor(shops[id], reg).Evaluate(c.specification)
				if err != nil {
					satisfied = append(satisfied, err.Error())
				} else if ok {
					satisfied = append(satisfied, id)
				}
			}
			sql, params, err := Compile(discountsContext{}, c.specification)
			if err != nil {
				t.Errorf("%v: the transformer: %v", c.want, err)
				continue
			}
			selected, err := selected(conn, "SELECT id FROM spec_optional_shops WHERE "+sql+" ORDER BY id", params)
			if err != nil {
				return fmt.Errorf("%s %v: %w", sql, params, err)
			}
			if fmt.Sprint(satisfied) != fmt.Sprint(c.want) || !reflect.DeepEqual(selected, c.want) {
				t.Errorf("%s %v: the evaluator %v, PostgreSQL %v, want %v", sql, params, satisfied, selected, c.want)
			}
		}
		return nil
	})
}

// Why a type is said only where nothing stands beside the constant. A time is
// written as a timestamp with zone or without, whichever the column is. Said
// to be timestamptz beside a column without zone, it would be compared in the
// session's time zone, and the row would not be found.
func TestAConstantBesideAColumnTakesTheColumnsType(t *testing.T) {
	withConnection(t, func(_ session.Session, conn session.DbConnection) error {
		for _, statement := range []string{
			"SET LOCAL TIME ZONE 'Asia/Tokyo'",
			"CREATE TABLE spec_moments (id bigint, at timestamp, zoned timestamptz, small smallint)",
			"INSERT INTO spec_moments VALUES (1, '2023-11-14 22:13:20', '2023-11-14 22:13:20+00', 7)",
		} {
			if _, err := conn.Exec(statement); err != nil {
				return fmt.Errorf("%s: %w", statement, err)
			}
		}
		noon := time.Unix(1_700_000_000, 0).UTC()
		for _, specification := range []s.Visitable{
			s.Equal(field("at"), s.Value(noon)),
			s.Equal(field("zoned"), s.Value(noon)),
			s.Equal(field("small"), s.Value(7)),
			// And where nothing stands beside them, the constants say their own.
			s.Equal(field("small"), s.Add(s.Value(3), s.Value(4))),
		} {
			sql, params, err := CompileToSQL(specification)
			if err != nil {
				return err
			}
			ids, err := selected(conn, "SELECT id FROM spec_moments WHERE "+sql, params)
			if err != nil {
				return fmt.Errorf("%s: %w", sql, err)
			}
			if len(ids) != 1 {
				t.Errorf("%s %v: selected %v", sql, params, ids)
			}
		}
		return nil
	})
}

// PostgreSQL shifts by an integer and by nothing else: a bigint column as the
// count was "operator does not exist: bigint << bigint". Both readers take
// the count modulo 64, a negative one included.
func TestTheCountOfAShiftIsAnIntegerWhateverItsColumnIs(t *testing.T) {
	rows := map[int]rowContext{
		1: {"n": 1, "count": 3, "small": 3},
		2: {"n": 1, "count": 64, "small": 64},
		3: {"n": 1, "count": -1, "small": -1},
		4: {"n": 1, "count": nil, "small": nil},
		5: {"n": 8, "count": 62, "small": nil},
	}
	n, count, small := field("n"), field("count"), field("small")
	cases := []struct {
		specification s.Visitable
		want          []int
	}{
		{s.Equal(s.LeftShift(n, count), s.Value(8)), []int{1}},
		// 64 is no shift at all, and -1 is one by 63.
		{s.Equal(s.LeftShift(n, count), s.Value(1)), []int{2}},
		{s.Equal(s.LeftShift(n, count), s.Value(math.MinInt64)), []int{3}},
		{s.IsNull(s.LeftShift(n, count)), []int{4}},
		{s.IsNull(s.LeftShift(n, small)), []int{4, 5}},
		{s.Equal(s.LeftShift(n, small), s.Value(8)), []int{1}},
		// An expression as the count, and a constant shifted by a column.
		{s.Equal(s.LeftShift(n, s.Add(count, s.Value(1))), s.Value(16)), []int{1}},
		{s.Equal(s.RightShift(s.Value(64), count), s.Value(8)), []int{1}},
	}
	reg := operators.NewDefaultRegistry()
	withConnection(t, func(_ session.Session, conn session.DbConnection) error {
		if _, err := conn.Exec("CREATE TABLE spec_shifts (id bigint, n bigint, count bigint, small smallint)"); err != nil {
			return err
		}
		for id, row := range rows {
			if _, err := conn.Exec("INSERT INTO spec_shifts VALUES ($1, $2, $3, $4)", id, row["n"], row["count"], row["small"]); err != nil {
				return err
			}
		}
		for _, c := range cases {
			satisfied := []int{}
			for id := 1; id <= len(rows); id++ {
				ok, err := s.NewEvaluateVisitor(rows[id], reg).Evaluate(c.specification)
				if err != nil {
					return fmt.Errorf("the evaluator, on row %d: %w", id, err)
				}
				if ok {
					satisfied = append(satisfied, id)
				}
			}
			if !reflect.DeepEqual(satisfied, c.want) {
				t.Errorf("%v: the evaluator %v", c.want, satisfied)
			}
			sql, params, err := CompileToSQL(c.specification)
			if err != nil {
				return err
			}
			ids, err := selected(conn, "SELECT id FROM spec_shifts WHERE "+sql+" ORDER BY id", params)
			if err != nil {
				return fmt.Errorf("%s: %w", sql, err)
			}
			if !reflect.DeepEqual(ids, c.want) {
				t.Errorf("%s %v: PostgreSQL %v, want %v", sql, params, ids, c.want)
			}
		}
		return nil
	})
}

func TestASpecificationSelectsTheRowsItIsSatisfiedBy(t *testing.T) {
	// The schema is the storage's keys; the tree reaches the compiler in the
	// storage's names, which a mapping gives it: the items are a table of
	// their own in one storage and an array in the other, and the owner is
	// named by the key's column in both. A row of the items array has no
	// table: it is named by the array's column.
	relational := NewSchemaRegistry("spec_stores").
		ForeignKey("spec_items", "store_id", "spec_stores", "id").
		ForeignKey("spec_items", "owner_id", "spec_owners", "id").
		ForeignKey("spec_stores", "owner_id", "spec_owners", "id")
	embedded := NewSchemaRegistry("spec_stores_embedded").
		ForeignKey("spec_stores_embedded.items", "owner_id", "spec_owners", "id").
		ForeignKey("spec_stores_embedded", "owner_id", "spec_owners", "id")
	storages := []struct {
		name    string
		table   string
		schema  *SchemaRegistry
		mapping Mapping
	}{
		{"relational", "spec_stores", relational, renamed{"items": "spec_items", "owner": "owner_id"}},
		{"embedded", "spec_stores_embedded", embedded, renamed{"owner": "owner_id"}},
	}

	withConnection(t, func(_ session.Session, conn session.DbConnection) error {
		// Rolled back whatever happens: the tables are of this test alone.
		if err := makeTables(conn); err != nil {
			return err
		}
		for _, specification := range rowSpecifications() {
			satisfied := []int{}
			for _, row := range storeRows {
				ok, err := s.NewEvaluateVisitor(row.context(), operators.NewDefaultRegistry()).Evaluate(specification)
				if err != nil {
					return fmt.Errorf("the evaluator, on store %d: %w", row.id, err)
				}
				if ok {
					satisfied = append(satisfied, row.id)
				}
				// The same row with pointers for its nullable members is
				// the same row.
				pointed, err := s.NewEvaluateVisitor(row.pointerContext(), operators.NewDefaultRegistry()).Evaluate(specification)
				if err != nil {
					return fmt.Errorf("the evaluator, on store %d with pointers: %w", row.id, err)
				}
				if pointed != ok {
					return fmt.Errorf("store %d: %v with values, %v with pointers", row.id, ok, pointed)
				}
			}
			for _, storage := range storages {
				sql, params, err := Compile(storage.mapping, specification, WithSchema(storage.schema))
				if err != nil {
					return err
				}
				ids, err := selected(conn, "SELECT id FROM "+storage.table+" WHERE "+sql+" ORDER BY id", params)
				if err != nil {
					return fmt.Errorf("%s: %w", sql, err)
				}
				t.Run(storage.name+": "+sql, func(t *testing.T) {
					if !reflect.DeepEqual(ids, satisfied) {
						t.Errorf("PostgreSQL selects %v; the evaluator is satisfied by %v", ids, satisfied)
					}
				})
			}
		}
		return nil
	})
}

// The item of an enclosing collection, named from an inner predicate by how
// far out it is: the category's limit beside the price of its product,
// OuterItem(1). In either storage - arrays nested in a composite, or tables
// that point at one another - the enclosing item's row is in scope of the
// inner query. And the shop's own limit beside them, which used to be written
// unqualified and read by PostgreSQL from the innermost row that has a column
// of that name: the category's, not the shop's.
func TestTheItemOfAnEnclosingCollectionIsNamedFromAnInnerPredicate(t *testing.T) {
	type category struct {
		limit  any
		prices []any
	}
	shop := func(limit any, categories ...category) s.Context {
		contexts := make([]s.Context, 0, len(categories))
		for _, c := range categories {
			products := make([]s.Context, 0, len(c.prices))
			for _, price := range c.prices {
				products = append(products, rowContext{"price": price})
			}
			contexts = append(contexts, rowContext{"limit": c.limit, "products": s.NewCollectionContext(products)})
		}
		return rowContext{"limit": limit, "categories": s.NewCollectionContext(contexts)}
	}
	shops := map[int]s.Context{
		1: shop(50, category{10, []any{5, 20}}, category{100, []any{30}}),
		2: shop(50, category{100, []any{30, nil}}),
		3: shop(5, category{nil, []any{30}}),
		4: shop(50),
	}
	price := s.Field(s.Item(), "price")
	categoryLimit := s.Field(s.OuterItem(1), "limit")
	shopLimit := s.Field(s.GlobalScope(), "limit")
	overItsCategory := func(predicate s.Visitable) s.Visitable {
		return s.Wildcard(s.Object(s.GlobalScope(), "categories"), s.Wildcard(s.Object(s.Item(), "products"), predicate))
	}
	cases := []struct {
		specification s.Visitable
		want          []int
	}{
		{overItsCategory(s.GreaterThan(price, categoryLimit)), []int{1}},
		{overItsCategory(s.LessThan(price, categoryLimit)), []int{1, 2}},
		{overItsCategory(s.GreaterThan(price, shopLimit)), []int{3}},
		{overItsCategory(s.And(s.GreaterThan(price, categoryLimit), s.LessThan(categoryLimit, shopLimit))), []int{1}},
		{overItsCategory(s.IsNull(categoryLimit)), []int{3}},
		// Of a null limit the comparison is null, and so is its negation: no
		// product of shop 3 is a witness, and the shop is selected.
		{s.Not(overItsCategory(s.Not(s.GreaterThan(price, categoryLimit)))), []int{3, 4}},
	}
	storages := map[string]struct {
		schema  *SchemaRegistry
		mapping Mapping
	}{
		"embedded": {NewSchemaRegistry("spec_shops"), renamed{}},
		"relational": {
			NewSchemaRegistry("spec_shops").
				ForeignKey("spec_categories", "shop_id", "spec_shops", "id").
				ForeignKey("spec_products", "category_id", "spec_categories", "id"),
			renamed{"categories": "spec_categories", "products": "spec_products"},
		},
	}
	reg := operators.NewDefaultRegistry()

	withConnection(t, func(_ session.Session, conn session.DbConnection) error {
		for _, statement := range []string{
			"CREATE TYPE spec_product AS (price bigint)",
			`CREATE TYPE spec_category AS ("limit" bigint, products spec_product[])`,
			`CREATE TABLE spec_shops (id bigint, "limit" bigint, categories spec_category[])`,
			`CREATE TABLE spec_categories (id bigint, shop_id bigint, "limit" bigint)`,
			"CREATE TABLE spec_products (category_id bigint, price bigint)",
			`INSERT INTO spec_shops VALUES
				(1, 50, ARRAY[ROW(10, ARRAY[ROW(5), ROW(20)]::spec_product[]),
				              ROW(100, ARRAY[ROW(30)]::spec_product[])]::spec_category[]),
				(2, 50, ARRAY[ROW(100, ARRAY[ROW(30), ROW(NULL)]::spec_product[])]::spec_category[]),
				(3, 5, ARRAY[ROW(NULL, ARRAY[ROW(30)]::spec_product[])]::spec_category[]),
				(4, 50, '{}')`,
			"INSERT INTO spec_categories VALUES (11, 1, 10), (12, 1, 100), (21, 2, 100), (31, 3, NULL)",
			"INSERT INTO spec_products VALUES (11, 5), (11, 20), (12, 30), (21, 30), (21, NULL), (31, 30)",
		} {
			if _, err := conn.Exec(statement); err != nil {
				return fmt.Errorf("%s: %w", statement, err)
			}
		}
		for _, c := range cases {
			satisfied := []int{}
			for id := 1; id <= 4; id++ {
				ok, err := s.NewEvaluateVisitor(shops[id], reg).Evaluate(c.specification)
				if err != nil {
					return err
				}
				if ok {
					satisfied = append(satisfied, id)
				}
			}
			if !reflect.DeepEqual(satisfied, c.want) {
				t.Errorf("%v: the evaluator %v", c.want, satisfied)
			}
			for storage, st := range storages {
				sql, params, err := Compile(st.mapping, c.specification, WithSchema(st.schema))
				if err != nil {
					return err
				}
				rows, err := selected(conn, "SELECT id FROM spec_shops WHERE "+sql+" ORDER BY id", params)
				if err != nil {
					return fmt.Errorf("%s: %s: %w", storage, sql, err)
				}
				if !reflect.DeepEqual(rows, c.want) {
					t.Errorf("%s: %s %v: PostgreSQL %v, want %v", storage, sql, params, rows, c.want)
				}
			}
		}
		return nil
	})
}
