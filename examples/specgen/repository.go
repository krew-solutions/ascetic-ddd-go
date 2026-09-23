package main

import (
	"fmt"
	"strings"

	spec "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
	infra "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/infrastructure"
)

// The repository's side of the examples: what knows the tables.
//
// A specification is generated as a tree under the names of Go's fields. What
// a field is called in the storage is said here, once for each aggregate, and
// nowhere else; which collections are tables of their own is the schema's to
// say, by the storage's foreign keys. A query cannot be written without
// knowing the table, so it is written by what knows it: the generated code
// has the tree, and no SQL.
//
// A mapping names the members that may be asked about, and refuses any other:
// it is also the list of what a specification may filter by.

// mapping is a Mapping said with a table of names: each member of the
// aggregate by its path from it, a member of an item under its collection,
// and what each is in the storage.
type mapping map[string]string

func (m mapping) AttrNode(path []string) (infra.Mapped, error) {
	name, ok := m[strings.Join(path, ".")]
	if !ok {
		return nil, fmt.Errorf("no such member: %s", strings.Join(path, "."))
	}
	return infra.Scalar(fromCandidate(strings.Split(name, ".")...)), nil
}

func (mapping) ValueNode(val any) (infra.Mapped, error) {
	return infra.Scalar(spec.Value(val)), nil
}

// fromCandidate is a member by its whole path from the candidate: a mapping
// answers so for a member of an item too, and the MappingVisitor puts the
// answer where the member was.
func fromCandidate(names ...string) spec.Visitable {
	var owner spec.EmptiableObject = spec.GlobalScope()
	for _, name := range names[:len(names)-1] {
		owner = spec.Object(owner, name)
	}
	return spec.Field(owner, names[len(names)-1])
}

var users = mapping{"ID": "id", "Age": "age", "Active": "active", "Name": "name", "Email": "email"}

var stores = mapping{
	"ID": "id", "Name": "name", "Active": "active",
	"Items":        "items",
	"Items.ID":     "items.id",
	"Items.Name":   "items.name",
	"Items.Price":  "items.price",
	"Items.Active": "items.active",
	"Items.Stock":  "items.stock",
}

// The items of an organization are under its regions and their categories:
// a member of an item is named by the whole path to it, so the items of one
// collection are told from another's.
var organizations = mapping{
	"ID": "id", "Name": "name", "Active": "active",
	"Regions":                         "regions",
	"Regions.Active":                  "regions.active",
	"Regions.Categories":              "regions.categories",
	"Regions.Categories.Active":       "regions.categories.active",
	"Regions.Categories.Items":        "regions.categories.items",
	"Regions.Categories.Items.Price":  "regions.categories.items.price",
	"Regions.Categories.Items.Active": "regions.categories.items.active",
}

var deals = mapping{"ID": "id", "Discount": "discount"}

// userSQL is the condition of a query for the users a specification is
// satisfied by; storeSQL and organizationSQL are the same of their aggregates.
func userSQL(specification spec.Visitable) (string, []any, error) {
	return infra.Compile(users, specification)
}

func storeSQL(specification spec.Visitable) (string, []any, error) {
	return infra.Compile(stores, specification)
}

func organizationSQL(specification spec.Visitable) (string, []any, error) {
	return infra.Compile(organizations, specification)
}

func dealSQL(specification spec.Visitable) (string, []any, error) {
	return infra.Compile(deals, specification)
}
