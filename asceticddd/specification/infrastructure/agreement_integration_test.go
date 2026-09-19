package specification

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/session"
	s "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/utils/testutils"
)

// The evaluator and PostgreSQL, on the same specifications: a specification
// has these two readers, and they must agree.

// errRollback undoes what a test did to the database.
var errRollback = errors.New("rollback")

// constantCase is an expression of constants, with what the server is told of
// its parameters: a constant expression gives it nothing to infer a type from.
type constantCase struct {
	node s.Visitable
	// nullType is the type of the nulls of the case.
	nullType string
	// types are the types of the parameters, where they are not what the
	// values say: PostgreSQL shifts a bigint by an integer.
	types []string
}

func number(node s.Visitable) constantCase  { return constantCase{node: node, nullType: "bigint"} }
func boolean(node s.Visitable) constantCase { return constantCase{node: node, nullType: "bool"} }
func shift(node s.Visitable) constantCase {
	return constantCase{node: node, types: []string{"bigint", "integer"}}
}

func typeOf(value any, nullType string) string {
	switch value.(type) {
	case nil:
		return nullType
	case bool:
		return "bool"
	case int, int64:
		return "bigint"
	case float64:
		return "float8"
	case string:
		return "text"
	default:
		panic(fmt.Sprintf("no type for %T", value))
	}
}

var parameterPattern = regexp.MustCompile(`\$\d+`)

// typed writes the type of each parameter after it.
func typed(sql string, types []string) string {
	i := 0
	return parameterPattern.ReplaceAllStringFunc(sql, func(parameter string) string {
		i++
		return parameter + "::" + types[i-1]
	})
}

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

func constantCases() []constantCase {
	v := s.Value
	null := func() s.Visitable { return s.Value(nil) }
	t, f := func() s.Visitable { return s.Value(true) }, func() s.Visitable { return s.Value(false) }
	min, max := math.MinInt64, math.MaxInt64
	nan := math.NaN()
	return []constantCase{
		// Arithmetic, and the parentheses that keep its shape
		number(s.Sub(v(10), s.Sub(v(4), v(3)))),
		number(s.Sub(s.Sub(v(10), v(4)), v(3))),
		number(s.Sub(v(10), s.Add(v(4), v(3)))),
		number(s.Div(v(100), s.Div(v(10), v(5)))),
		number(s.Div(s.Mul(v(7), v(3)), v(2))),
		number(s.Mul(s.Add(v(1), v(2)), v(3))),
		number(s.Add(v(1), s.Mul(v(2), v(3)))),
		number(s.Div(v(7), v(2))),
		number(s.Div(v(-7), v(2))),
		number(s.Mod(v(-7), v(2))),
		number(s.Mod(v(7), v(-2))),
		number(s.Mod(v(min), v(-1))),
		number(s.Neg(s.Neg(v(5)))),
		number(s.Sub(v(5), s.Neg(v(3)))),
		number(s.Neg(s.Add(v(1), v(2)))),
		number(s.Div(v(7.0), v(2))),
		number(s.Add(v(1), v(0.5))),
		number(s.Mul(v(2.5), v(4))),
		number(s.Add(v(int64(1)), v(2))),
		shift(s.LeftShift(v(1), v(3))),
		shift(s.LeftShift(v(1), v(64))),
		shift(s.LeftShift(v(1), v(-1))),
		shift(s.LeftShift(v(1), v(63))),
		shift(s.RightShift(v(8), v(65))),
		shift(s.RightShift(v(-8), v(1))),
		// Where it fails
		number(s.Div(v(1), v(0))),
		number(s.Mod(v(1), v(0))),
		number(s.Div(v(1.0), v(0.0))),
		number(s.Add(v(max), v(1))),
		number(s.Sub(v(min), v(1))),
		number(s.Mul(v(max), v(2))),
		number(s.Div(v(min), v(-1))),
		number(s.Neg(v(min))),
		number(s.Mul(v(math.MaxFloat64), v(2.0))),
		// What is not defined here is not defined there
		number(s.Add(v("a"), v("b"))),
		number(s.Mod(v(5.5), v(2))),
		number(s.Add(v(true), v(1))),
		number(s.Neg(v("a"))),
		number(s.Equal(v("a"), v(1))),
		number(s.LessThan(v(true), v(2))),
		number(s.Is(v("a"), v(1))),
		// Comparisons
		number(s.Equal(v(1), v(1.0))),
		number(s.LessThan(v(1), v(1.5))),
		number(s.GreaterThanEqual(v(2), v(2))),
		number(s.LessThanEqual(v(3), v(2))),
		number(s.NotEqual(v("a"), v("b"))),
		number(s.LessThan(v("a"), v("b"))),
		number(s.GreaterThan(v(true), v(false))),
		number(s.Equal(v(nan), v(nan))),
		number(s.GreaterThan(v(nan), v(math.MaxFloat64))),
		number(s.LessThanEqual(v(1.0), v(nan))),
		number(s.Equal(v(math.Copysign(0, -1)), v(0.0))),
		number(s.Equal(s.Equal(v(1), v(1)), v(true))),
		number(s.Equal(v(true), s.Equal(v(1), v(2)))),
		// Nulls
		number(s.Equal(null(), v(1))),
		number(s.Equal(null(), null())),
		number(s.NotEqual(v(1), null())),
		number(s.Add(v(1), null())),
		number(s.Neg(null())),
		number(s.Div(null(), v(0))),
		boolean(s.Not(null())),
		boolean(s.And(null(), f())),
		boolean(s.And(f(), null())),
		boolean(s.And(null(), t())),
		boolean(s.And(null(), null())),
		boolean(s.Or(null(), t())),
		boolean(s.Or(t(), null())),
		boolean(s.Or(null(), f())),
		boolean(s.And(s.Or(t(), f()), f())),
		boolean(s.Or(t(), s.And(f(), f()))),
		boolean(s.And(t(), s.And(t(), f()))),
		boolean(s.Not(s.And(t(), f()))),
		boolean(s.Not(s.Not(t()))),
		boolean(s.IsNull(s.Or(null(), f()))),
		boolean(s.IsNull(s.IsNull(null()))),
		boolean(s.Equal(s.IsNull(null()), t())),
		number(s.IsNull(s.Equal(v(1), null()))),
		number(s.IsNotNull(s.Equal(v(1), null()))),
		boolean(s.Not(s.IsNull(null()))),
		// IS
		boolean(s.Is(t(), t())),
		boolean(s.Is(t(), f())),
		boolean(s.Is(null(), null())),
		boolean(s.Is(null(), t())),
		number(s.Is(v(1), null())),
		number(s.Is(v(1), v(1))),
		boolean(s.Equal(s.Is(t(), null()), f())),
		boolean(s.Is(s.Equal(v(1), v(1)), t())),
	}
}

// same tells whether the evaluator's value is the one PostgreSQL answered.
func same(evaluated, answered any) bool {
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
			sql, params, err := CompileToSQL(c.node)
			if err != nil {
				return err
			}
			types := c.types
			if types == nil {
				for _, param := range params {
					types = append(types, typeOf(param, c.nullType))
				}
			}
			query := "SELECT (" + typed(sql, types) + ")"

			evaluated, failure := s.Accept[any](c.node, s.NewEvaluateVisitor(rowContext{}, operators.NewDefaultRegistry()))

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
		items = append(items, rowContext{"price": it[0], "active": it[1]})
	}
	return rowContext{
		"id": r.id, "a": r.a, "b": r.b, "flag": r.flag, "name": r.name,
		"items": s.NewCollectionContext(items),
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
		"CREATE TABLE spec_stores (id bigint PRIMARY KEY, a bigint, b bigint, flag boolean, name text)",
		"CREATE TABLE spec_items (store_id bigint, price bigint, active boolean)",
		"CREATE TYPE spec_item AS (price bigint, active boolean)",
		"CREATE TABLE spec_stores_embedded (id bigint PRIMARY KEY, a bigint, b bigint, flag boolean, name text, items spec_item[])",
	}
	for _, row := range storeRows {
		columns := fmt.Sprintf("%d, %s, %s, %s, %s", row.id, literal(row.a), literal(row.b), literal(row.flag), literal(row.name))
		embedded := make([]string, 0, len(row.items))
		for _, it := range row.items {
			statements = append(statements, fmt.Sprintf(
				"INSERT INTO spec_items VALUES (%d, %s, %s)", row.id, literal(it[0]), literal(it[1]),
			))
			embedded = append(embedded, fmt.Sprintf("ROW(%s, %s)::spec_item", literal(it[0]), literal(it[1])))
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

func TestASpecificationSelectsTheRowsItIsSatisfiedBy(t *testing.T) {
	relational := NewSchemaRegistry("spec_stores").RegisterRelational("items", "spec_items", "store_id", "id")
	storages := []struct {
		name  string
		table string
		opts  []PostgresqlVisitorOption
	}{
		{"relational", "spec_stores", []PostgresqlVisitorOption{WithSchema(relational)}},
		{"embedded", "spec_stores_embedded", nil},
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
			}
			for _, storage := range storages {
				sql, params, err := CompileToSQL(specification, storage.opts...)
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
