# specgen - Specification Code Generator

`specgen` is a code generation tool that converts Go predicate functions into Specification Pattern AST nodes.

## Why?

**Goal**: Maximize performance by using native Go functions for in-memory checks while still being able to compile the same specification to SQL.

**Problem**: Python can parse lambda functions at runtime using `inspect` and `ast`, but Go cannot inspect function bodies at runtime.

**Solution**: Parse function AST at **compile time** using `go/parser` and generate corresponding Specification AST code.

## How It Works

```
Go Function (predicate)  →  specgen  →  Generated AST Code
     ↓                                         ↓
In-Memory Check                     Repository: Compile(context, AST)
(FASTEST)                                      ↓
                                           SQL Query
```

The tree is all that is generated. It has the names of the struct's fields;
what a field is called in the storage, and where a collection is kept, is the
repository's to say, with a `Context` - one place for each aggregate. A query
cannot be written without knowing the table, so it is written by what knows
it, and the package of the domain does not import the infrastructure.

## Usage

### 1. Mark your specification functions

```go
//go:generate specgen -type=User

type User struct {
    Age    int
    Active bool
}

// AdultUserSpec checks if user is adult
//spec:sql
func AdultUserSpec(u User) bool {
    return u.Age >= 18
}
```

### 2. Generate code

```bash
go generate
```

### 3. Use both representations

```go
// In-memory: Direct function call (FASTEST)
if AdultUserSpec(user) {
    fmt.Println("Adult")
}

// SQL: in the repository, from the generated AST and the repository's context
sql, params, _ := infra.Compile(userContext{}, AdultUserSpecAST())
db.Query("SELECT * FROM users WHERE " + sql, params...)
```

```go
// userContext says what the members of a User are in the table, and refuses
// any other: it is also the list of what a specification may filter by.
type userContext struct{ infra.ContextDefaults }

func (userContext) AttrNode(path []string) (infra.Mapped, error) {
    switch strings.Join(path, ".") {
    case "Age":
        return infra.Scalar(spec.Field(spec.GlobalScope(), "age")), nil
    case "Active":
        return infra.Scalar(spec.Field(spec.GlobalScope(), "is_active")), nil
    default:
        return nil, fmt.Errorf("no such member of a user: %s", strings.Join(path, "."))
    }
}

func (userContext) ValueNode(val any) (infra.Mapped, error) {
    return infra.Scalar(spec.Value(val)), nil
}
```

`infra.Compile(context, tree, infra.WithSchema(schema))` takes the schema of
the collections kept in tables of their own as well.

A name is written into the query between quotes, as it is: `"Age"` is not the
column `age`. A `...SQL()` used to be generated, which compiled the tree under
the names of Go's fields; it found a column only while PostgreSQL folded an
unquoted word, and only a word of one part - `CreatedAt` was `createdat`, not
`created_at`.

## Performance

| Aspect | Python lambda | Go specgen |
|--------|--------------|-----------|
| In-memory | Bytecode interpretation | **Native function call** |
| Tree for SQL | Runtime AST parse | **Pre-generated AST** |
| Parse overhead | Every time | **Once at compile time** |
| Type safety | Runtime | **Compile time** |

## Features

### Supported Operators

- **Comparison**: `==`, `!=`, `>`, `<`, `>=`, `<=`
- **Logical**: `&&`, `||`, `!`
- **Arithmetic**: `+`, `-`, `*`, `/`, `%`

### Example

```go
//spec:sql
func PremiumUserSpec(u User) bool {
    return u.Age >= 18 && u.Active && u.Name != ""
}
```

Generates:

```go
func PremiumUserSpecAST() spec.Visitable {
    return spec.And(
        spec.And(
            spec.GreaterThanEqual(
                spec.Field(spec.GlobalScope(), "Age"),
                spec.Value(18),
            ),
            spec.Field(spec.GlobalScope(), "Active"),
        ),
        spec.NotEqual(
            spec.Field(spec.GlobalScope(), "Name"),
            spec.Value(""),
        ),
    )
}
```

## Installation

```bash
go install github.com/krew-solutions/ascetic-ddd-go/cmd/specgen@latest
```

Or build from source:

```bash
go build -o $GOPATH/bin/specgen ./cmd/specgen
```

## Command Line Options

```bash
specgen -type=TypeName
```

- `-type`: The type name to generate specifications for (required)

## Requirements

- Functions must have signature: `func(T) bool`, or `func(T, ...) bool`: what
  follows the candidate are the parameters of the specification, and the
  generated `...AST` takes the same. A parameter must have a name and may not
  be variadic.

  ```go
  //spec:sql
  func DearSinceSpec(s Store, since time.Time, min int) bool { ... }

  // generated:
  func DearSinceSpecAST(since time.Time, min int) spec.Visitable
  ```
- Functions must have `//spec:sql` comment: that line, as a directive is
  written. A comment that mentions the marker does not mark the function, nor
  does an example in its doc; written with a space, `// spec:sql`, the marker
  marks nothing and specgen says so.
- Function body must contain a single return statement
- Type `T` must be in the same package. Its fields need not be exported: the
  generator reads the source, and the generated file is of the same package,
  so a predicate inside the package of an aggregate may name private fields.

## Example Project

See [examples/specgen](../../examples/specgen) for a complete working example.

## Advantages over Python Lambda Parser

1. ✅ **Zero Runtime Overhead**: Parsing happens at compile time
2. ✅ **Maximum Performance**: In-memory checks use native Go functions
3. ✅ **Type Safety**: Compiler checks both representations
4. ✅ **Clear Intent**: `//spec:sql` makes it explicit
5. ✅ **IDE Support**: Full autocomplete, refactoring, debugging
6. ✅ **Error Detection**: Compile-time errors instead of runtime

## Limitations

- Cannot parse complex control flow (if/else, loops)
- Cannot access external variables (closures): a value comes as a parameter
  of the function, or as a constant or a variable of the package
- Cannot call methods (only field access)
- Single return statement only

These limitations are intentional - specifications should be pure boolean expressions.

What cannot be a specification is an error where it stands, and nothing is
generated:

```
advanced_example.go:116:10: HasItemWithFlagSpec: unsupported operator &
```

It used to be generated as `spec.Value(nil)` with a TODO in a comment: code
that compiles, and a query that compares NULL and selects nothing.

## What Is Generated

- `spec.All(s.Items, p)` is "no item fails `p`":
  `spec.Not(spec.Wildcard(items, spec.Not(p)))`. It used to be generated as
  `spec.Any`.
- `x == nil` is `spec.IsNull(x)` and `x != nil` is `spec.IsNotNull(x)`: written
  as it stands it is `x = NULL`, which is null and true of nothing.
- A name that is not the candidate's nor an item's - a constant or a variable
  of the package - is a value, `spec.Value(MinBalance)`. Equality with one is
  decided when the tree is built: `spec.EqualityOrNullTest("=", x, v)` is the
  null test if the value is nil. Such a name used to be read as a field of the
  candidate: `u.Email.Equal(email)` compiled to `Email = email`.
- `-x` is `spec.Neg(x)`, `-5` is `spec.Value(-5)`, `+x` is `x`.
- A `time.Time` is compared by its methods: `a.After(b)` is
  `spec.GreaterThan(a, b)`, `a.Before(b)` is `spec.LessThan(a, b)`, `a.Equal(b)`
  is the equality. The table goes by the name of the method - the generator
  does not know the types - as it does for `Gt`, `Lt` and the rest. `time.Now()`
  is refused: the clock comes as a parameter. A nullable time is a pointer, and
  its guard is generated with it: `s.DeletedAt != nil && s.DeletedAt.After(since)`
  is `"DeletedAt" IS NOT NULL AND "DeletedAt" > $1`, under the names a context
  gives.
- A member that is an `option.Option` is what it holds, or a null, to both
  readers of a tree. What it holds is asked under a name:
  `d.Discount.IsSomeAnd(func(v int) bool { return v > 10 })` is
  `spec.And(spec.IsNotNull(d.Discount), spec.GreaterThan(d.Discount, 10))`, and
  `IsNothingOr` is `spec.Or(spec.IsNull(..), ..)`; `v.Percent` is
  `d.Discount.Percent`. The null test beside the predicate makes the whole of
  two values, as it is in Go, so the function and its tree agree under a `!`
  too, and nothing is unwrapped. A parameter that is an Option is asked the
  same way. `d.Discount.IsNothing()` is `spec.IsNull(d.Discount)` and
  `IsSome()` is `spec.IsNotNull`; `d.Discount.Unwrap()` is the member itself,
  and `limit.Unwrap()` of a parameter is `spec.Value(limit)`, the Option, not
  what it holds when the tree is built: the predicate unwraps it behind its
  guard, and of a Nothing never does. `option.Some(x)` is `x`, and
  `option.Nothing[T]()` is the null, so equality with it is the null test. The
  two makers are told by what the file imports, under whatever name: another
  package's `Some` makes another Option, which no reader of a tree reads, and
  is refused. `UnwrapOr` and the rest have no node and are refused. See
  `examples/specgen/deal.go`.
- From the predicate of an inner collection the item of an outer one cannot be
  named: the tree has one item, the nearest. It used to become a field of the
  candidate.

## Future Enhancements

- [ ] Support for nested field access (`u.Profile.Age`)
- [ ] Support for wildcards (`any(item in collection where ...)`)
- [ ] Support for method calls on fields
- [ ] Custom operator mappings
- [ ] A specification as an object: `Expression()` generated from the
  `IsSatisfiedBy` method of a specification's type, its fields as the values
