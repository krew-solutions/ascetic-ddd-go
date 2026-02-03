package specification

// StorageType defines how a collection is stored
type StorageType int

const (
	// StorageEmbedded means collection is stored as JSONB/array in parent table
	StorageEmbedded StorageType = iota
	// StorageRelational means collection is stored in a separate table
	StorageRelational
)

// ForeignKeyPair represents a single FK column mapping
type ForeignKeyPair struct {
	// ChildColumn is the column in the child table (e.g., "store_id", "tenant_id")
	ChildColumn string
	// ParentColumn is the column in the parent table (e.g., "id", "tenant_id")
	ParentColumn string
}

// CollectionMapping defines how a collection field maps to storage
type CollectionMapping struct {
	// Storage defines whether collection is embedded or in separate table
	Storage StorageType

	// Table is the name of the child table (only for StorageRelational)
	Table string

	// ForeignKeys defines the FK relationship (supports composite keys)
	// For simple FK: []ForeignKeyPair{{ChildColumn: "store_id", ParentColumn: "id"}}
	// For composite: []ForeignKeyPair{
	//     {ChildColumn: "tenant_id", ParentColumn: "tenant_id"},
	//     {ChildColumn: "store_id", ParentColumn: "id"},
	// }
	ForeignKeys []ForeignKeyPair

	// Alias is optional custom alias for the subquery (defaults to singularized table name)
	Alias string
}

// SchemaRegistry holds collection mappings for a specific aggregate/repository
type SchemaRegistry struct {
	// ParentTable is the main table name (e.g., "stores")
	ParentTable string

	// ParentAlias is the alias used for parent table in queries (e.g., "s" for "stores s")
	ParentAlias string

	// collections maps collection field name to its mapping
	collections map[string]CollectionMapping
}

// NewSchemaRegistry creates a new SchemaRegistry for a parent table
func NewSchemaRegistry(parentTable string) *SchemaRegistry {
	return &SchemaRegistry{
		ParentTable: parentTable,
		ParentAlias: "",
		collections: make(map[string]CollectionMapping),
	}
}

// WithParentAlias sets the parent table alias
func (r *SchemaRegistry) WithParentAlias(alias string) *SchemaRegistry {
	r.ParentAlias = alias
	return r
}

// RegisterEmbedded registers a collection stored as embedded JSONB/array
func (r *SchemaRegistry) RegisterEmbedded(fieldName string) *SchemaRegistry {
	r.collections[fieldName] = CollectionMapping{
		Storage: StorageEmbedded,
	}
	return r
}

// RegisterRelational registers a collection stored in a separate table with simple FK
func (r *SchemaRegistry) RegisterRelational(fieldName, table, childColumn, parentColumn string) *SchemaRegistry {
	r.collections[fieldName] = CollectionMapping{
		Storage: StorageRelational,
		Table:   table,
		ForeignKeys: []ForeignKeyPair{
			{ChildColumn: childColumn, ParentColumn: parentColumn},
		},
	}
	return r
}

// RegisterRelationalComposite registers a collection with composite FK
func (r *SchemaRegistry) RegisterRelationalComposite(fieldName, table string, foreignKeys []ForeignKeyPair) *SchemaRegistry {
	r.collections[fieldName] = CollectionMapping{
		Storage:     StorageRelational,
		Table:       table,
		ForeignKeys: foreignKeys,
	}
	return r
}

// Register registers a collection with full mapping configuration
func (r *SchemaRegistry) Register(fieldName string, mapping CollectionMapping) *SchemaRegistry {
	r.collections[fieldName] = mapping
	return r
}

// Get returns the collection mapping for a field name
func (r *SchemaRegistry) Get(fieldName string) (CollectionMapping, bool) {
	mapping, ok := r.collections[fieldName]
	return mapping, ok
}

// IsEmbedded returns true if collection is stored as embedded JSONB/array
func (r *SchemaRegistry) IsEmbedded(fieldName string) bool {
	mapping, ok := r.collections[fieldName]
	if !ok {
		// Default to embedded if not registered
		return true
	}
	return mapping.Storage == StorageEmbedded
}

// IsRelational returns true if collection is stored in a separate table
func (r *SchemaRegistry) IsRelational(fieldName string) bool {
	mapping, ok := r.collections[fieldName]
	if !ok {
		return false
	}
	return mapping.Storage == StorageRelational
}

// GetParentRef returns the reference to parent table (alias or table name)
func (r *SchemaRegistry) GetParentRef() string {
	if r.ParentAlias != "" {
		return r.ParentAlias
	}
	return r.ParentTable
}
