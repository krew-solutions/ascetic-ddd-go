# Specgen Features - Complete Guide

## ✅ Fully Supported Features

### 1. **All Operators**

#### Comparison Operators
```go
//spec:sql
func ComparisonSpec(u User) bool {
    return u.Age >= 18 &&    // >=
           u.Age > 17 &&      // >
           u.Age < 100 &&     // <
           u.Age <= 99 &&     // <=
           u.Age == 30 &&     // ==
           u.Age != 0         // !=
}
```

**Generates:**
```go
spec.And(
    spec.And(
        spec.GreaterThanEqual(spec.Field(...), spec.Value(18)),
        spec.GreaterThan(spec.Field(...), spec.Value(17)),
    ),
    // ...
)
```

#### Logical Operators
```go
//spec:sql
func LogicalSpec(u User) bool {
    return u.Active && u.Age > 18 || !u.Deleted
}
```

**Generates:**
```go
spec.Or(
    spec.And(
        spec.Field(spec.GlobalScope(), "Active"),
        spec.GreaterThan(spec.Field(spec.GlobalScope(), "Age"), spec.Value(18)),
    ),
    spec.Not(spec.Field(spec.GlobalScope(), "Deleted")),
)
```

#### Arithmetic Operators
```go
//spec:sql
func ArithmeticSpec(p Product) bool {
    return p.Price + p.Tax > 1000 &&
           p.Price - p.Discount < 500 &&
           p.Price * 2 <= 2000 &&
           p.Price / 10 >= 10 &&
           p.Price % 100 == 0
}
```

**Generates:**
```go
spec.And(
    spec.GreaterThan(
        spec.Add(spec.Field(..., "Price"), spec.Field(..., "Tax")),
        spec.Value(1000),
    ),
    // ...
)
```

#### Bitwise Operators
```go
//spec:sql
func BitwiseSpec(i Item) bool {
    return i.ID << 2 == 8 &&   // Left shift
           i.ID >> 1 == 4      // Right shift
    // Note: &, |, ^ (AND, OR, XOR) are TODO
}
```

**Generates:**
```go
spec.And(
    spec.Equal(
        spec.LeftShift(spec.Field(..., "ID"), spec.Value(2)),
        spec.Value(8),
    ),
    spec.Equal(
        spec.RightShift(spec.Field(..., "ID"), spec.Value(1)),
        spec.Value(4),
    ),
)
```

### 2. **Wildcards (Collections)**

The killer feature! Use `spec.Any()` and `spec.All()` to filter collections.

#### spec.Any() - At least one item matches

```go
//spec:sql
func HasExpensiveItemsSpec(s Store) bool {
    return spec.Any(s.Items, func(item Item) bool {
        return item.Price > 1000
    })
}
```

**Generates:**
```go
spec.Wildcard(
    spec.Object(spec.GlobalScope(), "Items"),
    spec.GreaterThan(
        spec.Field(spec.Item(), "Price"),
        spec.Value(1000),
    ),
)
```

**SQL:** (when PostgreSQL visitor supports it)
```sql
WHERE EXISTS (
    SELECT 1 FROM unnest(Items) AS item
    WHERE item.Price > $1
)
```

#### spec.All() - Every item must match

```go
//spec:sql
func AllItemsActiveSpec(s Store) bool {
    return spec.All(s.Items, func(item Item) bool {
        return item.Active
    })
}
```

**Generates:**
```go
spec.Wildcard(
    spec.Object(spec.GlobalScope(), "Items"),
    spec.Field(spec.Item(), "Active"),
)
```

#### Complex Wildcard Predicates

```go
//spec:sql
func HasPremiumItemsSpec(s Store) bool {
    return spec.Any(s.Items, func(item Item) bool {
        return item.Price > 5000 && item.Active && item.Stock > 0
    })
}
```

**Generates:**
```go
spec.Wildcard(
    spec.Object(spec.GlobalScope(), "Items"),
    spec.And(
        spec.And(
            spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Value(5000)),
            spec.Field(spec.Item(), "Active"),
        ),
        spec.GreaterThan(spec.Field(spec.Item(), "Stock"), spec.Value(0)),
    ),
)
```

### 3. **Nested Fields**

Access nested object fields:

```go
type User struct {
    Profile Profile
}

type Profile struct {
    Age int
}

//spec:sql
func AdultProfileSpec(u User) bool {
    return u.Profile.Age >= 18
}
```

**Generates:**
```go
spec.Field(
    spec.Object(spec.GlobalScope(), "Profile"),
    "Age",
)
```

### 4. **Combined Specifications**

Combine wildcards with root-level conditions:

```go
//spec:sql
func PremiumActiveStoreSpec(s Store) bool {
    return s.Active &&
           s.Name != "" &&
           spec.Any(s.Items, func(item Item) bool {
               return item.Price > 1000
           })
}
```

**Generates:**
```go
spec.And(
    spec.And(
        spec.Field(spec.GlobalScope(), "Active"),
        spec.NotEqual(spec.Field(spec.GlobalScope(), "Name"), spec.Value("")),
    ),
    spec.Wildcard(
        spec.Object(spec.GlobalScope(), "Items"),
        spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Value(1000)),
    ),
)
```

### 5. **Negation with Wildcards**

```go
//spec:sql
func NoExpensiveItemsSpec(s Store) bool {
    return !spec.Any(s.Items, func(item Item) bool {
        return item.Price > 5000
    })
}
```

**Generates:**
```go
spec.Not(
    spec.Wildcard(
        spec.Object(spec.GlobalScope(), "Items"),
        spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Value(5000)),
    ),
)
```

### 6. **Value Object Comparison Methods**

Since Go doesn't support operator overloading, Value Objects typically use methods for comparison. Specgen recognizes these method patterns:

```go
type Email struct {
    value string
}

func (e Email) Equal(other Email) bool {
    return e.value == other.value
}

type User struct {
    Email  Email
    Status Status
}

//spec:sql
func UserByEmailSpec(u User) bool {
    return u.Email.Equal(targetEmail)
}
```

**Generates:**
```go
spec.Equal(
    spec.Field(spec.GlobalScope(), "Email"),
    spec.Field(spec.GlobalScope(), "targetEmail"),
)
```

#### Supported Method Names

| Category | Methods | Generated Spec |
|----------|---------|----------------|
| Equality | `Equal`, `Equals`, `Eq` | `spec.Equal` |
| Inequality | `NotEqual`, `NotEquals`, `Ne`, `Neq` | `spec.NotEqual` |
| Less than | `LessThan`, `Lt` | `spec.LessThan` |
| Less or equal | `LessThanOrEqual`, `LessThanEqual`, `Lte`, `Le` | `spec.LessThanEqual` |
| Greater than | `GreaterThan`, `Gt` | `spec.GreaterThan` |
| Greater or equal | `GreaterThanOrEqual`, `GreaterThanEqual`, `Gte`, `Ge` | `spec.GreaterThanEqual` |

#### Examples

```go
// Equality with literal
//spec:sql
func ActiveUserSpec(u User) bool {
    return u.Status.Equal("active")
}

// Comparison with nested field
//spec:sql
func UserWithHighAgeSpec(u User) bool {
    return u.Profile.Age.GreaterThan(minAge)
}

// Combined with logical operators
//spec:sql
func PremiumUserSpec(u User) bool {
    return u.Email.Equal(email) && u.Status.NotEqual("banned")
}

// Inside wildcards
//spec:sql
func HasExpensiveItemSpec(s Store) bool {
    return spec.Any(s.Items, func(item Item) bool {
        return item.Price.Gt(minPrice) && item.Status.Equal("available")
    })
}
```

### 7. **Nested Wildcards (Collections of Collections)**

The most powerful feature! Nest wildcards to filter multi-level collection hierarchies.

#### Simple 3-Level Nesting

```go
type Organization struct {
    Regions []Region
}

type Region struct {
    Categories []Category
}

type Category struct {
    Items []Item
}

//spec:sql
func HasRegionWithExpensiveItemsSpec(o Organization) bool {
    return spec.Any(o.Regions, func(region Region) bool {
        return spec.Any(region.Categories, func(category Category) bool {
            return spec.Any(category.Items, func(item Item) bool {
                return item.Price > 5000
            })
        })
    })
}
```

**Generates:**
```go
spec.Wildcard(
    spec.Object(spec.GlobalScope(), "Regions"),
    spec.Wildcard(
        spec.Object(spec.Item(), "Categories"),  // Note: Item() for nested
        spec.Wildcard(
            spec.Object(spec.Item(), "Items"),
            spec.GreaterThan(spec.Field(spec.Item(), "Price"), spec.Value(5000)),
        ),
    ),
)
```

**SQL:**
```sql
WHERE EXISTS (
    SELECT 1 FROM unnest(Regions) AS region_1
    WHERE EXISTS (
        SELECT 1 FROM unnest(region_1.Categories) AS category_2
        WHERE EXISTS (
            SELECT 1 FROM unnest(category_2.Items) AS item_3
            WHERE item_3.Price > $1
        )
    )
)
```

#### Nested with Conditions at Each Level

```go
//spec:sql
func HasActiveRegionWithPremiumItemsSpec(o Organization) bool {
    return spec.Any(o.Regions, func(region Region) bool {
        return region.Active && spec.Any(region.Categories, func(category Category) bool {
            return category.Active && spec.Any(category.Items, func(item Item) bool {
                return item.Price > 5000 && item.Active
            })
        })
    })
}
```

**SQL:**
```sql
WHERE EXISTS (
    SELECT 1 FROM unnest(Regions) AS region_1
    WHERE region_1.Active AND EXISTS (
        SELECT 1 FROM unnest(region_1.Categories) AS category_2
        WHERE category_2.Active AND EXISTS (
            SELECT 1 FROM unnest(category_2.Items) AS item_3
            WHERE item_3.Price > $1 AND item_3.Active
        )
    )
)
```

#### Key Features of Nested Wildcards

✅ **Unique aliases per level**: `region_1`, `category_2`, `item_3`
✅ **Proper path resolution**: `region_1.Categories`, `category_2.Items`
✅ **Conditions at any level**: Filter at each nesting depth
✅ **Unlimited nesting depth**: As many levels as needed
✅ **Optimal SQL**: Generates efficient nested EXISTS subqueries

## 📊 Performance

### Benchmark Results

```
BenchmarkAdultUserSpec_InMemory-12        1000000000    0.13 ns/op    0 B/op    0 allocs/op
BenchmarkAdultUserSpec_ManualAST-12         8523465    142.2 ns/op  176 B/op    4 allocs/op
BenchmarkHasExpensiveItems_InMemory-12    100000000    10.5 ns/op     0 B/op    0 allocs/op
```

**In-memory checks are 1000x faster than AST evaluation!**

### Why so fast?

1. **Direct function call**: Native Go code, JIT compiled
2. **Zero overhead**: No reflection, no AST traversal
3. **Inline-friendly**: Go compiler can inline simple checks

### SQL Generation

```
BenchmarkAdultUserSpec_SQL-12               335817   3271 ns/op  3818 B/op   24 allocs/op
```

SQL generation has overhead, but:
- It's only called once per query
- AST is pre-built at compile time (no parsing!)
- Result can be cached

## 📝 How to Use

### Step 1: Write Specifications

```go
//go:generate go run github.com/krew-solutions/ascetic-ddd-go/cmd/specgen -type=Store

type Store struct {
    Name   string
    Active bool
    Items  []Item
}

type Item struct {
    Price  int
    Active bool
}

// Simple specification
//spec:sql
func ActiveStoreSpec(s Store) bool {
    return s.Active
}

// Wildcard specification
//spec:sql
func HasCheapItemsSpec(s Store) bool {
    return spec.Any(s.Items, func(item Item) bool {
        return item.Price < 100
    })
}
```

### Step 2: Generate Code

```bash
go generate
```

Creates `store_specs_gen.go`:

```go
func ActiveStoreSpecAST() spec.Visitable {
    return spec.Field(spec.GlobalScope(), "Active")
}

func ActiveStoreSpecSQL() (string, []any, error) {
    ast := ActiveStoreSpecAST()
    return infra.CompileToSQL(ast)
}

func HasCheapItemsSpecAST() spec.Visitable {
    return spec.Wildcard(
        spec.Object(spec.GlobalScope(), "Items"),
        spec.LessThan(spec.Field(spec.Item(), "Price"), spec.Value(100)),
    )
}

func HasCheapItemsSpecSQL() (string, []any, error) {
    ast := HasCheapItemsSpecAST()
    return infra.CompileToSQL(ast)
}
```

### Step 3: Use in Code

```go
// In-memory filtering (FAST!)
stores := []Store{...}
for _, store := range stores {
    if ActiveStoreSpec(store) {
        fmt.Println(store.Name)
    }
}

// SQL generation (when needed)
sql, params, _ := ActiveStoreSpecSQL()
db.Query("SELECT * FROM stores WHERE " + sql, params...)
```

## 🎯 Summary

### ✅ Supported

| Feature | Status | Example |
|---------|--------|---------|
| Comparison (`==`, `!=`, `>`, `<`, `>=`, `<=`) | ✅ Full | `u.Age >= 18` |
| Logical (`&&`, `\|\|`, `!`) | ✅ Full | `u.Active && !u.Deleted` |
| Arithmetic (`+`, `-`, `*`, `/`, `%`) | ✅ Full | `p.Price - p.Discount > 100` |
| Bitwise (`<<`, `>>`) | ✅ Full | `i.ID << 2 == 8` |
| Bitwise (`&`, `\|`, `^`) | ⚠️ TODO | Will be added |
| Wildcards (`Any`, `All`) | ✅ Full | `spec.Any(s.Items, ...)` |
| Nested wildcards | ✅ Full | `spec.Any(region.Categories, ...)` |
| Nested fields | ✅ Full | `u.Profile.Age` |
| Value Object methods | ✅ Full | `u.Email.Equal(email)` |
| Complex expressions | ✅ Full | Unlimited nesting |
| In-memory checks | ✅ Fast | 0.13 ns, 0 allocs |
| SQL generation | ✅ Works | Pre-built AST |

### ⚠️ Limitations

1. **Single return statement**: Function body must have exactly one `return` statement
2. **No control flow**: Cannot use `if/else`, `for`, `switch`
3. **No closures**: Cannot access variables from outer scope
4. **Bitwise AND/OR/XOR**: Not yet implemented in Specification nodes

These limitations are intentional - specifications should be pure boolean expressions.

## 🚀 Next Steps

### TODO for PostgreSQL Visitor

- [x] ~~Implement Collection/Wildcard compilation to SQL~~ ✅ Done
- [x] ~~Support unnest() for array columns~~ ✅ Done
- [ ] Support JSONPath for nested collections (if needed for JSON columns)
- [ ] Support ANY/ALL SQL operators (alternative to EXISTS)

### TODO for Specgen

- [ ] Add support for `&`, `|`, `^` bitwise operators
- [x] ~~Add support for method calls (`.IsNull()`, `.IsNotNull()`)~~ ✅ Done
- [x] ~~Add support for Value Object comparison methods (`.Equal()`, `.GreaterThan()`, etc.)~~ ✅ Done
- [x] ~~Add support for nested wildcards (collections of collections)~~ ✅ Done
- [ ] Better error messages for unsupported expressions

## 📚 Examples

See:
- [user.go](user.go) - Basic specifications
- [advanced_example.go](advanced_example.go) - All features
- [main.go](main.go) - Running demo
- [benchmark_test.go](benchmark_test.go) - Performance tests
