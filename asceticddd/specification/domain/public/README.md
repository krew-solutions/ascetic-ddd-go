# Public API for Specification Pattern

This package provides a convenient high-level API for building specifications using a fluent interface.

## Overview

The public API wraps the core specification nodes with typed wrappers that provide:

- **Type-safe field and value creation**
- **Fluent method chaining**
- **Comparison operations** (Eq, Ne, Gt, Lt, Gte, Lte)
- **Logical operations** (And, Or, Is)
- **Mathematical operations** (Add, Sub, Mul, Div, Mod)
- **NULL checks** (IsNull, IsNotNull)
- **Bitwise operations** (Lshift, Rshift)

## Usage Examples

### Basic Comparison

```go
import (
    pub "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/public"
)

// Create fields
age := pub.MakeNumberField("age")
isActive := pub.MakeBooleanField("is_active")

// Build specification: age >= 18
spec := age.Gte(pub.MakeNumberValue(18))
```

### Logical Operations

```go
// Combine conditions: (age >= 18) AND is_active
age := pub.MakeNumberField("age")
isActive := pub.MakeBooleanField("is_active")

spec := age.Gte(pub.MakeNumberValue(18)).And(isActive)
```

### Mathematical Operations

```go
// Calculate: (price * quantity) - discount
price := pub.MakeNumberField("price")
quantity := pub.MakeNumberField("quantity")
discount := pub.MakeNumberField("discount")

total := price.Mul(quantity).Sub(discount)

// Use in comparison: total > 1000
expensiveSpec := total.(pub.IComparison).Gt(pub.MakeNumberValue(1000))
```

### Nullable Fields

```go
// Check for NULL: deleted_at IS NULL
deletedAt := pub.MakeNullDatetimeField("deleted_at")
notDeletedSpec := deletedAt.IsNull()

// Check for NOT NULL: email IS NOT NULL
email := pub.MakeNullTextField("email")
hasEmailSpec := email.IsNotNull()
```

### Complex Expressions

```go
// Build: (age > 18) AND (name == "Alice") AND is_active
age := pub.MakeNumberField("age")
name := pub.MakeTextField("name")
isActive := pub.MakeBooleanField("is_active")

spec := age.Gt(pub.MakeNumberValue(18)).
    And(name.Eq(pub.MakeTextValue("Alice"))).
    And(isActive)
```

### Nested Fields

```go
// Access nested fields: user.profile.age >= 18
userAge := pub.MakeNumberField("user.profile.age")
spec := userAge.Gte(pub.MakeNumberValue(18))
```

### Compiling to SQL

```go
import (
    spec "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/infrastructure"
)

// Create specification
age := pub.MakeNumberField("age")
isActive := pub.MakeBooleanField("is_active")
condition := age.Gte(pub.MakeNumberValue(18)).And(isActive)

// Compile to SQL
sql, params, err := spec.CompileToSQL(condition.Delegate())
// Result: "age >= $1 AND is_active", [18]
```

## Available Types

### Boolean Types
- `Boolean` - Boolean field with logical operations
- `NullBoolean` - Nullable boolean field

### Number Types
- `Number` - Numeric field with comparison and mathematical operations
- `NullNumber` - Nullable numeric field

### Text Types
- `Text` - Text field with comparison operations
- `NullText` - Nullable text field

### Datetime Types
- `Datetime` - Datetime field with comparison and mathematical operations
- `NullDatetime` - Nullable datetime field

## Factory Functions

Each type has factory functions for creating fields and values:

```go
// Fields
age := pub.MakeNumberField("age")
name := pub.MakeTextField("name")
isActive := pub.MakeBooleanField("is_active")
createdAt := pub.MakeDatetimeField("created_at")

// Values
ageValue := pub.MakeNumberValue(18)
nameValue := pub.MakeTextValue("Alice")
activeValue := pub.MakeBooleanValue(true)
dateValue := pub.MakeDatetimeValue(time.Now())
```

## Helper Functions

### Object_()
Create nested object paths:

```go
// Create: user.profile
obj := pub.Object_("user.profile")
```

### Field()
Create field references with dotted paths:

```go
// Create: user.profile.age
field := pub.Field("user.profile.age")
```

## Differences from Python Version

Since Go doesn't support operator overloading like Python, we use methods instead:

| Python | Go |
|--------|-----|
| `age >= 18` | `age.Gte(MakeNumberValue(18))` |
| `a & b` | `a.And(b)` |
| `a \| b` | `a.Or(b)` |
| `a == b` | `a.Eq(b)` |
| `a != b` | `a.Ne(b)` |
| `a + b` | `a.Add(b)` |
| `a - b` | `a.Sub(b)` |

## Type Safety

The public API provides compile-time type safety:

```go
// This works - Number has Gte method
age := pub.MakeNumberField("age")
spec := age.Gte(pub.MakeNumberValue(18))

// This works - Boolean has And method
isActive := pub.MakeBooleanField("is_active")
isPremium := pub.MakeBooleanField("is_premium")
spec2 := isActive.And(isPremium)

// This won't compile - can't compare boolean with number
// spec3 := isActive.Gte(pub.MakeNumberValue(18)) // ERROR!
```

## See Also

- [Specification Pattern](../../README.md) - Core specification implementation
- [Test Examples](public_test.go) - Comprehensive test suite with usage examples
