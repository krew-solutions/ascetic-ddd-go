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
// a field is called in the storage, and where a collection is kept, is said
// here, once for each aggregate, and nowhere else. A query cannot be written
// without knowing the table, so it is written by what knows it: the generated
// code has the tree, and no SQL.
//
// A mapping names the members that may be asked about, and refuses any other:
// it is also the list of what a specification may filter by.

// columns are the names of a struct's fields, and what each is in the storage.
type columns map[string]string

func (c columns) name(path []string) (string, error) {
	if len(path) == 1 {
		if name, ok := c[path[0]]; ok {
			return name, nil
		}
	}
	return "", fmt.Errorf("no such member: %s", strings.Join(path, "."))
}

// mapping is a Context said with tables of names: the members of the
// aggregate, the members of the items of its collections, its collections.
type mapping struct {
	members     columns
	itemMembers columns
	collections columns
}

func (m mapping) AttrNode(path []string) (infra.Mapped, error) {
	name, err := m.members.name(path)
	if err != nil {
		return nil, err
	}
	return infra.Scalar(spec.Field(spec.GlobalScope(), name)), nil
}

func (m mapping) ItemAttrNode(path []string) (infra.Mapped, error) {
	name, err := m.itemMembers.name(path)
	if err != nil {
		return nil, err
	}
	return infra.Scalar(spec.Field(spec.Item(), name)), nil
}

func (m mapping) CollectionNode(path []string) (spec.EmptiableObject, error) {
	name, err := m.collections.name(path)
	if err != nil {
		return nil, err
	}
	return spec.Object(spec.GlobalScope(), name), nil
}

func (m mapping) ItemCollectionNode(path []string) (spec.EmptiableObject, error) {
	name, err := m.collections.name(path)
	if err != nil {
		return nil, err
	}
	return spec.Object(spec.Item(), name), nil
}

func (m mapping) ValueNode(val any) (infra.Mapped, error) {
	return infra.Scalar(spec.Value(val)), nil
}

var users = mapping{
	members: columns{"ID": "id", "Age": "age", "Active": "active", "Name": "name", "Email": "email"},
}

var stores = mapping{
	members:     columns{"ID": "id", "Name": "name", "Active": "active"},
	itemMembers: columns{"ID": "id", "Name": "name", "Price": "price", "Active": "active", "Stock": "stock"},
	collections: columns{"Items": "items"},
}

// The items of three collections - regions, categories, items - are told
// apart by nothing: a context is asked for a member of "the item", not of
// which collection. Here the three have their names in common.
var organizations = mapping{
	members:     columns{"ID": "id", "Name": "name", "Active": "active"},
	itemMembers: columns{"ID": "id", "Name": "name", "Price": "price", "Active": "active", "Stock": "stock"},
	collections: columns{"Regions": "regions", "Categories": "categories", "Items": "items"},
}

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
