package specification

import (
	"fmt"
	"strings"
)

// A schema is the foreign keys of a storage, as `\d` shows them, and nothing
// of any aggregate or query: a key is on a table, of columns, and references
// a table's columns. What the compiler calls a row in a query - an alias - is
// the compiler's own, made as it goes. The one thing of the query in a schema
// is the table the query is of, `FROM stores s`: the row the compiler starts
// from, and what it qualifies that row's columns with.
//
// A tree names a collection by the table its rows are in, `store_items`, or,
// where two keys of that table reference the same row, by the key's name; and
// an object kept in a table of its own by the key's column, `owner_id`. A key
// has a name as it has in PostgreSQL: the one it is given, or
// `<table>_<columns>_fkey`. A Value Object kept in the query's row as a column
// of a composite type is declared as one, `Composite("stores", "address")`,
// and a path through it from the candidate is a member of it,
// `("s"."address")."city"`: from the candidate an undeclared name is a
// table's alias, `"s"."price"`.

// ForeignKey is `Table (Columns) REFERENCES ReferencedTable (ReferencedColumns)`.
//
// A key has at least one column: without any, every row of the table would
// belong to every row it references; and as many referenced columns as
// columns. Key refuses any other where it is declared. Table is a table; or,
// for a key on a row of an array in a composite, which has no table, the
// array's column by its table, `stores.items`.
type ForeignKey struct {
	// ConstraintName is the key's name where the storage gives it one:
	// `CONSTRAINT name`. Empty, the key is named as PostgreSQL names it.
	ConstraintName    string
	Table             string
	Columns           []string
	ReferencedTable   string
	ReferencedColumns []string
}

// Name is the key's name: the one it was given, or the one PostgreSQL gives
// a key that was not, `<table>_<columns>_fkey`.
func (k ForeignKey) Name() string {
	if k.ConstraintName != "" {
		return k.ConstraintName
	}
	table := k.Table[strings.LastIndex(k.Table, ".")+1:]
	return table + "_" + strings.Join(k.Columns, "_") + "_fkey"
}

// SchemaRegistry is the foreign keys of a storage, for the queries of one
// table.
//
//	schema := NewSchemaRegistry("accounts").WithAlias("a").
//		ForeignKey("transfers", "from_account_id", "accounts", "id").
//		ForeignKey("transfers", "to_account_id", "accounts", "id").
//		ForeignKey("accounts", "owner_id", "owners", "id")
//
// A tree names a collection by its table, `Any(transfers, ...)`, and where
// two keys of that table reference the row it is named from, by the key's
// name, `transfers_from_account_id_fkey`; an object by the key's column,
// `owner_id.name`.
type SchemaRegistry struct {
	// Table is the table the query is of: the row the compiler starts from.
	Table string
	// Alias is the alias the query gives its table: `s` of `FROM stores s`.
	Alias      string
	keys       []ForeignKey
	composites [][2]string
}

// NewSchemaRegistry creates a SchemaRegistry for the queries of table.
func NewSchemaRegistry(table string) *SchemaRegistry {
	return &SchemaRegistry{Table: table}
}

// WithAlias sets the alias the query gives its table.
func (r *SchemaRegistry) WithAlias(alias string) *SchemaRegistry {
	r.Alias = alias
	return r
}

// ForeignKey registers a key of one column: `table (column) REFERENCES
// referencedTable (referencedColumn)`.
func (r *SchemaRegistry) ForeignKey(table, column, referencedTable, referencedColumn string) *SchemaRegistry {
	return r.Key(ForeignKey{
		Table:             table,
		Columns:           []string{column},
		ReferencedTable:   referencedTable,
		ReferencedColumns: []string{referencedColumn},
	})
}

// Key registers a key as built: composite, or named.
//
// A key of no columns, or whose columns and referenced columns disagree in
// number, is a mistake in the program's constants - a schema is declared,
// not read - and is refused where it is declared, in PostgreSQL's words,
// rather than at the first query that joins by it.
func (r *SchemaRegistry) Key(key ForeignKey) *SchemaRegistry {
	if len(key.Columns) == 0 {
		panic(key.ddl() + ": a foreign key has at least one column")
	}
	if len(key.Columns) != len(key.ReferencedColumns) {
		panic(key.ddl() + ": number of referencing and referenced columns for foreign key disagree")
	}
	r.keys = append(r.keys, key)
	return r
}

// ddl is the key as `\d` shows it.
func (k ForeignKey) ddl() string {
	return fmt.Sprintf(
		"%s (%s) REFERENCES %s (%s)",
		k.Table, strings.Join(k.Columns, ", "), k.ReferencedTable, strings.Join(k.ReferencedColumns, ", "),
	)
}

// KeyNamed returns the key called name, if there is one.
func (r *SchemaRegistry) KeyNamed(name string) (ForeignKey, bool) {
	for _, key := range r.keys {
		if key.Name() == name {
			return key, true
		}
	}
	return ForeignKey{}, false
}

// KeysReferencing returns the keys on table that reference referencedTable.
func (r *SchemaRegistry) KeysReferencing(table, referencedTable string) []ForeignKey {
	var keys []ForeignKey
	for _, key := range r.keys {
		if key.Table == table && key.ReferencedTable == referencedTable {
			keys = append(keys, key)
		}
	}
	return keys
}

// KeysOn returns the keys on table that column is a column of.
func (r *SchemaRegistry) KeysOn(table, column string) []ForeignKey {
	var keys []ForeignKey
	for _, key := range r.keys {
		if key.Table != table {
			continue
		}
		for _, c := range key.Columns {
			if c == column {
				keys = append(keys, key)
				break
			}
		}
	}
	return keys
}

// Composite declares the column of table to be of a composite type: a Value
// Object kept in the row. From the candidate a path through it is a member
// of the composite, `("s"."address")."city"`, where a name not declared is a
// table's alias, `"s"."price"`.
func (r *SchemaRegistry) Composite(table, column string) *SchemaRegistry {
	r.composites = append(r.composites, [2]string{table, column})
	return r
}

// IsComposite reports whether column of table is declared a composite.
func (r *SchemaRegistry) IsComposite(table, column string) bool {
	for _, composite := range r.composites {
		if composite == [2]string{table, column} {
			return true
		}
	}
	return false
}

// Row returns what the query calls its table's row: the alias, or the table.
func (r *SchemaRegistry) Row() string {
	if r.Alias != "" {
		return r.Alias
	}
	return r.Table
}
