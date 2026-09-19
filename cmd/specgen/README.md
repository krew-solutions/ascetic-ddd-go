# specgen - Specification Code Generator

`specgen` is a code generation tool that converts Go predicate functions into Specification Pattern AST nodes.

## Why?

**Goal**: Maximize performance by using native Go functions for in-memory checks while still being able to generate SQL from the same specification.

**Problem**: Python can parse lambda functions at runtime using `inspect` and `ast`, but Go cannot inspect function bodies at runtime.

**Solution**: Parse function AST at **compile time** using `go/parser` and generate corresponding Specification AST code.

## How It Works

```
Go Function (predicate)  →  specgen  →  Generated AST Code
     ↓                                         ↓
In-Memory Check                            SQL Query
(FASTEST)                              (from pre-built AST)
```

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

// SQL: From generated AST (ZERO OVERHEAD)
sql, params, _ := AdultUserSpecSQL()
db.Query("SELECT * FROM users WHERE " + sql, params...)
```

## Performance

| Aspect | Python lambda | Go specgen |
|--------|--------------|-----------|
| In-memory | Bytecode interpretation | **Native function call** |
| SQL generation | Runtime AST parse | **Pre-generated AST** |
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

func PremiumUserSpecSQL() (string, []any, error) {
    ast := PremiumUserSpecAST()
    return infra.CompileToSQL(ast)
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
  generated `...AST` and `...SQL` take the same. A parameter must have a name
  and may not be variadic.

  ```go
  //spec:sql
  func DearSinceSpec(s Store, since time.Time, min int) bool { ... }

  // generated:
  func DearSinceSpecAST(since time.Time, min int) spec.Visitable
  func DearSinceSpecSQL(since time.Time, min int) (string, []any, error)
  ```
- Functions must have `//spec:sql` comment
- Function body must contain a single return statement
- Type `T` must be in the same package

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
  is `DeletedAt IS NOT NULL AND DeletedAt > $1`.
- From the predicate of an inner collection the item of an outer one cannot be
  named: the tree has one item, the nearest. It used to become a field of the
  candidate.

## Future Enhancements

- [ ] Support for nested field access (`u.Profile.Age`)
- [ ] Support for wildcards (`any(item in collection where ...)`)
- [ ] Support for method calls on fields
- [ ] Custom operator mappings
- [ ] Multiple SQL dialect targets
- [ ] Validation of generated SQL at compile time
