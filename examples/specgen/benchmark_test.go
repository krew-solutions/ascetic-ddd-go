package main

import (
	"testing"

	spec "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

var testUser = User{
	ID:     1,
	Age:    30,
	Active: true,
	Name:   "Alice",
	Email:  "alice@example.com",
}

// Benchmark: In-memory check using native Go function
func BenchmarkAdultUserSpec_InMemory(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = AdultUserSpec(testUser)
	}
}

// Benchmark: Manual AST construction + evaluation (for comparison)
func BenchmarkAdultUserSpec_ManualAST(b *testing.B) {
	ast := spec.GreaterThanEqual(
		spec.Field(spec.GlobalScope(), "Age"),
		spec.Value(18),
	)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		visitor := spec.NewEvaluateVisitor(&userContext{testUser})
		_ = ast.Accept(visitor)
		_, _ = visitor.Result()
	}
}

// Benchmark: Generated AST (to show it's pre-built)
func BenchmarkAdultUserSpec_GeneratedAST(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = AdultUserSpecAST()
	}
}

// Benchmark: SQL generation (only when needed)
func BenchmarkAdultUserSpec_SQL(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = AdultUserSpecSQL()
	}
}

// Benchmark: Complex specification in-memory
func BenchmarkPremiumUserSpec_InMemory(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = PremiumUserSpec(testUser)
	}
}

// Benchmark: Complex specification SQL generation
func BenchmarkPremiumUserSpec_SQL(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _, _ = PremiumUserSpecSQL()
	}
}

// userContext implements Context interface for testing
type userContext struct {
	user User
}

func (c *userContext) Get(field string) (any, error) {
	switch field {
	case "Age":
		return c.user.Age, nil
	case "Active":
		return c.user.Active, nil
	case "Name":
		return c.user.Name, nil
	case "Email":
		return c.user.Email, nil
	default:
		return nil, nil
	}
}
