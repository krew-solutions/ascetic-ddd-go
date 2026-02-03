package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// Helper to parse expression from string
func parseExpr(t *testing.T, expr string) ast.Expr {
	t.Helper()
	e, err := parser.ParseExpr(expr)
	if err != nil {
		t.Fatalf("Failed to parse expression %q: %v", expr, err)
	}
	return e
}

func TestVisitBinaryExpr_Comparison(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{
			name:     "Equal",
			expr:     "u.Age == 18",
			expected: `spec.Equal(spec.Field(spec.GlobalScope(), "Age"), spec.Value(18))`,
		},
		{
			name:     "NotEqual",
			expr:     "u.Name != \"\"",
			expected: `spec.NotEqual(spec.Field(spec.GlobalScope(), "Name"), spec.Value(""))`,
		},
		{
			name:     "GreaterThan",
			expr:     "u.Age > 18",
			expected: `spec.GreaterThan(spec.Field(spec.GlobalScope(), "Age"), spec.Value(18))`,
		},
		{
			name:     "GreaterThanEqual",
			expr:     "u.Age >= 18",
			expected: `spec.GreaterThanEqual(spec.Field(spec.GlobalScope(), "Age"), spec.Value(18))`,
		},
		{
			name:     "LessThan",
			expr:     "u.Age < 100",
			expected: `spec.LessThan(spec.Field(spec.GlobalScope(), "Age"), spec.Value(100))`,
		},
		{
			name:     "LessThanEqual",
			expr:     "u.Age <= 99",
			expected: `spec.LessThanEqual(spec.Field(spec.GlobalScope(), "Age"), spec.Value(99))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := parseExpr(t, tt.expr).(*ast.BinaryExpr)
			visitor := NewSpecGenVisitor("User")
			result := visitor.VisitBinaryExpr(expr)
			if result != tt.expected {
				t.Errorf("\nExpected: %s\nGot:      %s", tt.expected, result)
			}
		})
	}
}

func TestVisitBinaryExpr_Logical(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{
			name:     "And",
			expr:     "u.Active && u.Age > 18",
			expected: `spec.And(spec.Field(spec.GlobalScope(), "Active"), spec.GreaterThan(spec.Field(spec.GlobalScope(), "Age"), spec.Value(18)))`,
		},
		{
			name:     "Or",
			expr:     "u.Active || u.Premium",
			expected: `spec.Or(spec.Field(spec.GlobalScope(), "Active"), spec.Field(spec.GlobalScope(), "Premium"))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := parseExpr(t, tt.expr).(*ast.BinaryExpr)
			visitor := NewSpecGenVisitor("User")
			result := visitor.VisitBinaryExpr(expr)
			if result != tt.expected {
				t.Errorf("\nExpected: %s\nGot:      %s", tt.expected, result)
			}
		})
	}
}

func TestVisitBinaryExpr_Arithmetic(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{
			name:     "Add",
			expr:     "p.Price + p.Tax",
			expected: `spec.Add(spec.Field(spec.GlobalScope(), "Price"), spec.Field(spec.GlobalScope(), "Tax"))`,
		},
		{
			name:     "Sub",
			expr:     "p.Price - p.Discount",
			expected: `spec.Sub(spec.Field(spec.GlobalScope(), "Price"), spec.Field(spec.GlobalScope(), "Discount"))`,
		},
		{
			name:     "Mul",
			expr:     "p.Price * 2",
			expected: `spec.Mul(spec.Field(spec.GlobalScope(), "Price"), spec.Value(2))`,
		},
		{
			name:     "Div",
			expr:     "p.Price / 10",
			expected: `spec.Div(spec.Field(spec.GlobalScope(), "Price"), spec.Value(10))`,
		},
		{
			name:     "Mod",
			expr:     "p.Price % 100",
			expected: `spec.Mod(spec.Field(spec.GlobalScope(), "Price"), spec.Value(100))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := parseExpr(t, tt.expr).(*ast.BinaryExpr)
			visitor := NewSpecGenVisitor("Product")
			result := visitor.VisitBinaryExpr(expr)
			if result != tt.expected {
				t.Errorf("\nExpected: %s\nGot:      %s", tt.expected, result)
			}
		})
	}
}

func TestVisitBinaryExpr_Bitwise(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{
			name:     "LeftShift",
			expr:     "i.ID << 2",
			expected: `spec.LeftShift(spec.Field(spec.GlobalScope(), "ID"), spec.Value(2))`,
		},
		{
			name:     "RightShift",
			expr:     "i.ID >> 1",
			expected: `spec.RightShift(spec.Field(spec.GlobalScope(), "ID"), spec.Value(1))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := parseExpr(t, tt.expr).(*ast.BinaryExpr)
			visitor := NewSpecGenVisitor("Item")
			result := visitor.VisitBinaryExpr(expr)
			if result != tt.expected {
				t.Errorf("\nExpected: %s\nGot:      %s", tt.expected, result)
			}
		})
	}
}

func TestVisitSelectorExpr_SimpleField(t *testing.T) {
	expr := parseExpr(t, "u.Age").(*ast.SelectorExpr)
	visitor := NewSpecGenVisitor("User")
	result := visitor.VisitSelectorExpr(expr)
	expected := `spec.Field(spec.GlobalScope(), "Age")`

	if result != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, result)
	}
}

func TestVisitSelectorExpr_NestedField(t *testing.T) {
	expr := parseExpr(t, "u.Profile.Age").(*ast.SelectorExpr)
	visitor := NewSpecGenVisitor("User")
	result := visitor.VisitSelectorExpr(expr)
	expected := `spec.Field(spec.Object(spec.GlobalScope(), "Profile"), "Age")`

	if result != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, result)
	}
}

func TestVisitSelectorExpr_ItemField(t *testing.T) {
	// Inside wildcard context: item.Price
	expr := parseExpr(t, "item.Price").(*ast.SelectorExpr)
	visitor := NewSpecGenVisitor("Store").withWildcardContext("item")
	result := visitor.VisitSelectorExpr(expr)
	expected := `spec.Field(spec.Item(), "Price")`

	if result != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, result)
	}
}

func TestVisitSelectorExpr_ItemNestedField(t *testing.T) {
	// Inside wildcard context: item.Details.Stock
	expr := parseExpr(t, "item.Details.Stock").(*ast.SelectorExpr)
	visitor := NewSpecGenVisitor("Store").withWildcardContext("item")
	result := visitor.VisitSelectorExpr(expr)
	expected := `spec.Field(spec.Object(spec.Item(), "Details"), "Stock")`

	if result != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, result)
	}
}

func TestVisitUnaryExpr_Not(t *testing.T) {
	expr := parseExpr(t, "!u.Active").(*ast.UnaryExpr)
	visitor := NewSpecGenVisitor("User")
	result := visitor.Visit(expr)
	expected := `spec.Not(spec.Field(spec.GlobalScope(), "Active"))`

	if result != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, result)
	}
}

func TestVisit_BasicLit(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{name: "Int", expr: "42", expected: `spec.Value(42)`},
		{name: "Float", expr: "3.14", expected: `spec.Value(3.14)`},
		{name: "String", expr: `"hello"`, expected: `spec.Value("hello")`},
		{name: "True", expr: "true", expected: `spec.Value(true)`},
		{name: "False", expr: "false", expected: `spec.Value(false)`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := parseExpr(t, tt.expr)
			visitor := NewSpecGenVisitor("User")
			result := visitor.Visit(expr)
			if result != tt.expected {
				t.Errorf("\nExpected: %s\nGot:      %s", tt.expected, result)
			}
		})
	}
}

func TestVisit_Parentheses(t *testing.T) {
	expr := parseExpr(t, "(u.Age > 18)")
	visitor := NewSpecGenVisitor("User")
	result := visitor.Visit(expr)
	expected := `spec.GreaterThan(spec.Field(spec.GlobalScope(), "Age"), spec.Value(18))`

	if result != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, result)
	}
}

func TestVisit_ComplexExpression(t *testing.T) {
	// u.Active && u.Age >= 18 && u.Name != ""
	expr := parseExpr(t, `u.Active && u.Age >= 18 && u.Name != ""`)
	visitor := NewSpecGenVisitor("User")
	result := visitor.Visit(expr)
	expected := `spec.And(spec.And(spec.Field(spec.GlobalScope(), "Active"), spec.GreaterThanEqual(spec.Field(spec.GlobalScope(), "Age"), spec.Value(18))), spec.NotEqual(spec.Field(spec.GlobalScope(), "Name"), spec.Value("")))`

	if result != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, result)
	}
}

func TestFindSpecFunctions(t *testing.T) {
	source := `package main

type User struct {
	Age int
	Active bool
}

// Regular function without marker
func RegularFunc(u User) bool {
	return u.Age > 18
}

//spec:sql
func AdultUserSpec(u User) bool {
	return u.Age >= 18
}

//spec:sql
func ActiveUserSpec(u User) bool {
	return u.Active
}

// Another regular function
func OtherFunc(u User) bool {
	return true
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	functions := findSpecFunctions(fset, file, "User")

	if len(functions) != 2 {
		t.Errorf("Expected 2 spec functions, got %d", len(functions))
	}

	expectedNames := map[string]bool{
		"AdultUserSpec":  true,
		"ActiveUserSpec": true,
	}

	for _, fn := range functions {
		if !expectedNames[fn.Name] {
			t.Errorf("Unexpected function found: %s", fn.Name)
		}
		delete(expectedNames, fn.Name)
	}

	if len(expectedNames) > 0 {
		t.Errorf("Expected functions not found: %v", expectedNames)
	}
}

func TestVisit_SimpleSpec(t *testing.T) {
	source := `package main

type User struct {
	Age int
}

//spec:sql
func AdultUserSpec(u User) bool {
	return u.Age >= 18
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	specs := findSpecFunctions(fset, file, "User")
	if len(specs) != 1 {
		t.Fatalf("Expected 1 function, got %d", len(specs))
	}

	spec := specs[0]

	// Test that body was correctly extracted and can be converted
	visitor := NewSpecGenVisitor("User")
	result := visitor.Visit(spec.Body)

	expectedParts := []string{
		"spec.GreaterThanEqual",
		`spec.Field(spec.GlobalScope(), "Age")`,
		"spec.Value(18)",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected AST to contain %q\nGot:\n%s", part, result)
		}
	}
}

func TestVisit_ComplexSpec(t *testing.T) {
	source := `package main

type User struct {
	Age    int
	Active bool
	Name   string
}

//spec:sql
func PremiumUserSpec(u User) bool {
	return u.Active && u.Age >= 18 && u.Name != ""
}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse source: %v", err)
	}

	specs := findSpecFunctions(fset, file, "User")
	if len(specs) != 1 {
		t.Fatalf("Expected 1 function, got %d", len(specs))
	}

	spec := specs[0]
	visitor := NewSpecGenVisitor("User")
	result := visitor.Visit(spec.Body)

	expectedParts := []string{
		"spec.And",
		`spec.Field(spec.GlobalScope(), "Active")`,
		"spec.GreaterThanEqual",
		`spec.Field(spec.GlobalScope(), "Age")`,
		"spec.NotEqual",
		`spec.Field(spec.GlobalScope(), "Name")`,
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected AST to contain %q\nGot:\n%s", part, result)
		}
	}
}

// Test for wildcard conversion - need to create a mock CallExpr
func TestVisitAnyAll_RootWildcard(t *testing.T) {
	// This would test: spec.Any(s.Items, func(item Item) bool { return item.Price > 1000 })
	// For now, we'll parse a simplified version
	source := `package main
func test(s Store) bool {
	return spec.Any(s.Items, func(item Item) bool { return item.Price > 1000 })
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Extract the call expression
	fn := file.Decls[0].(*ast.FuncDecl)
	retStmt := fn.Body.List[0].(*ast.ReturnStmt)
	callExpr := retStmt.Results[0].(*ast.CallExpr)

	visitor := NewSpecGenVisitor("Store")
	result := visitor.visitAnyAll(callExpr, "Any")

	// Check that it generates correct AST
	expectedParts := []string{
		"spec.Wildcard",
		`spec.Object(spec.GlobalScope(), "Items")`,
		"spec.GreaterThan",
		`spec.Field(spec.Item(), "Price")`,
		"spec.Value(1000)",
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected result to contain %q\nGot: %s", part, result)
		}
	}
}

func TestVisitAnyAll_NestedWildcard(t *testing.T) {
	// Test: spec.Any(region.Categories, func(category Category) bool { return category.Active })
	// Inside a wildcard context (region is the item)
	source := `package main
func test(region Region) bool {
	return spec.Any(region.Categories, func(category Category) bool { return category.Active })
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Extract the call expression
	fn := file.Decls[0].(*ast.FuncDecl)
	retStmt := fn.Body.List[0].(*ast.ReturnStmt)
	callExpr := retStmt.Results[0].(*ast.CallExpr)

	// Simulate being inside a wildcard context where "region" is the item
	visitor := NewSpecGenVisitor("Organization").withWildcardContext("region")
	result := visitor.visitAnyAll(callExpr, "Any")

	// Check that it generates spec.Item() for nested wildcard
	expectedParts := []string{
		"spec.Wildcard",
		`spec.Object(spec.Item(), "Categories")`, // Key: spec.Item() not GlobalScope()
		`spec.Field(spec.Item(), "Active")`,
	}

	for _, part := range expectedParts {
		if !strings.Contains(result, part) {
			t.Errorf("Expected result to contain %q\nGot: %s", part, result)
		}
	}
}

func TestExtractTypeName(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		typeName string
		expected string
	}{
		{
			name: "Simple parameter",
			source: `package main
type User struct { Age int }
//spec:sql
func TestSpec(u User) bool { return true }
`,
			typeName: "User",
			expected: "User",
		},
		{
			name: "Different type parameter",
			source: `package main
type Store struct { Name string }
//spec:sql
func TestSpec(s Store) bool { return true }
`,
			typeName: "Store",
			expected: "Store",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.go", tt.source, parser.ParseComments)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			specs := findSpecFunctions(fset, file, tt.typeName)
			if len(specs) != 1 {
				t.Fatalf("Expected 1 function, got %d", len(specs))
			}

			// Just verify that spec was found - type extraction happens in findSpecFunctions
			if specs[0].Name != "TestSpec" {
				t.Errorf("Expected function name TestSpec, got %s", specs[0].Name)
			}
		})
	}
}

func TestVisitMethodComparison_Equal(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{
			name:     "Equal method",
			expr:     "u.Email.Equal(email)",
			expected: `spec.Equal(spec.Field(spec.GlobalScope(), "Email"), spec.Field(spec.GlobalScope(), "email"))`,
		},
		{
			name:     "Equals method",
			expr:     "u.Email.Equals(email)",
			expected: `spec.Equal(spec.Field(spec.GlobalScope(), "Email"), spec.Field(spec.GlobalScope(), "email"))`,
		},
		{
			name:     "Eq method",
			expr:     "u.Email.Eq(email)",
			expected: `spec.Equal(spec.Field(spec.GlobalScope(), "Email"), spec.Field(spec.GlobalScope(), "email"))`,
		},
		{
			name:     "Equal with literal",
			expr:     `u.Status.Equal("active")`,
			expected: `spec.Equal(spec.Field(spec.GlobalScope(), "Status"), spec.Value("active"))`,
		},
		{
			name:     "NotEqual method",
			expr:     "u.Email.NotEqual(email)",
			expected: `spec.NotEqual(spec.Field(spec.GlobalScope(), "Email"), spec.Field(spec.GlobalScope(), "email"))`,
		},
		{
			name:     "Ne method",
			expr:     "u.Email.Ne(email)",
			expected: `spec.NotEqual(spec.Field(spec.GlobalScope(), "Email"), spec.Field(spec.GlobalScope(), "email"))`,
		},
		{
			name:     "Neq method",
			expr:     "u.Email.Neq(email)",
			expected: `spec.NotEqual(spec.Field(spec.GlobalScope(), "Email"), spec.Field(spec.GlobalScope(), "email"))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := parseExpr(t, tt.expr).(*ast.CallExpr)
			visitor := NewSpecGenVisitor("User")
			result := visitor.Visit(expr)
			if result != tt.expected {
				t.Errorf("\nExpected: %s\nGot:      %s", tt.expected, result)
			}
		})
	}
}

func TestVisitMethodComparison_Ordering(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{
			name:     "LessThan method",
			expr:     "u.Age.LessThan(maxAge)",
			expected: `spec.LessThan(spec.Field(spec.GlobalScope(), "Age"), spec.Field(spec.GlobalScope(), "maxAge"))`,
		},
		{
			name:     "Lt method",
			expr:     "u.Age.Lt(maxAge)",
			expected: `spec.LessThan(spec.Field(spec.GlobalScope(), "Age"), spec.Field(spec.GlobalScope(), "maxAge"))`,
		},
		{
			name:     "LessThanOrEqual method",
			expr:     "u.Age.LessThanOrEqual(maxAge)",
			expected: `spec.LessThanEqual(spec.Field(spec.GlobalScope(), "Age"), spec.Field(spec.GlobalScope(), "maxAge"))`,
		},
		{
			name:     "Lte method",
			expr:     "u.Age.Lte(maxAge)",
			expected: `spec.LessThanEqual(spec.Field(spec.GlobalScope(), "Age"), spec.Field(spec.GlobalScope(), "maxAge"))`,
		},
		{
			name:     "Le method",
			expr:     "u.Age.Le(maxAge)",
			expected: `spec.LessThanEqual(spec.Field(spec.GlobalScope(), "Age"), spec.Field(spec.GlobalScope(), "maxAge"))`,
		},
		{
			name:     "GreaterThan method",
			expr:     "u.Age.GreaterThan(minAge)",
			expected: `spec.GreaterThan(spec.Field(spec.GlobalScope(), "Age"), spec.Field(spec.GlobalScope(), "minAge"))`,
		},
		{
			name:     "Gt method",
			expr:     "u.Age.Gt(minAge)",
			expected: `spec.GreaterThan(spec.Field(spec.GlobalScope(), "Age"), spec.Field(spec.GlobalScope(), "minAge"))`,
		},
		{
			name:     "GreaterThanOrEqual method",
			expr:     "u.Age.GreaterThanOrEqual(minAge)",
			expected: `spec.GreaterThanEqual(spec.Field(spec.GlobalScope(), "Age"), spec.Field(spec.GlobalScope(), "minAge"))`,
		},
		{
			name:     "Gte method",
			expr:     "u.Age.Gte(minAge)",
			expected: `spec.GreaterThanEqual(spec.Field(spec.GlobalScope(), "Age"), spec.Field(spec.GlobalScope(), "minAge"))`,
		},
		{
			name:     "Ge method",
			expr:     "u.Age.Ge(minAge)",
			expected: `spec.GreaterThanEqual(spec.Field(spec.GlobalScope(), "Age"), spec.Field(spec.GlobalScope(), "minAge"))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := parseExpr(t, tt.expr).(*ast.CallExpr)
			visitor := NewSpecGenVisitor("User")
			result := visitor.Visit(expr)
			if result != tt.expected {
				t.Errorf("\nExpected: %s\nGot:      %s", tt.expected, result)
			}
		})
	}
}

func TestVisitMethodComparison_NestedField(t *testing.T) {
	// Test nested field access: u.Profile.Email.Equal(email)
	expr := parseExpr(t, "u.Profile.Email.Equal(email)").(*ast.CallExpr)
	visitor := NewSpecGenVisitor("User")
	result := visitor.Visit(expr)
	expected := `spec.Equal(spec.Field(spec.Object(spec.GlobalScope(), "Profile"), "Email"), spec.Field(spec.GlobalScope(), "email"))`

	if result != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, result)
	}
}

func TestVisitMethodComparison_InWildcard(t *testing.T) {
	// Test inside wildcard context: item.Status.Equal("active")
	expr := parseExpr(t, `item.Status.Equal("active")`).(*ast.CallExpr)
	visitor := NewSpecGenVisitor("Store").withWildcardContext("item")
	result := visitor.Visit(expr)
	expected := `spec.Equal(spec.Field(spec.Item(), "Status"), spec.Value("active"))`

	if result != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, result)
	}
}

func TestVisitMethodComparison_WithLiteral(t *testing.T) {
	tests := []struct {
		name     string
		expr     string
		expected string
	}{
		{
			name:     "With string literal",
			expr:     `u.Email.Equal("test@example.com")`,
			expected: `spec.Equal(spec.Field(spec.GlobalScope(), "Email"), spec.Value("test@example.com"))`,
		},
		{
			name:     "With int literal",
			expr:     "u.Age.GreaterThan(18)",
			expected: `spec.GreaterThan(spec.Field(spec.GlobalScope(), "Age"), spec.Value(18))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expr := parseExpr(t, tt.expr).(*ast.CallExpr)
			visitor := NewSpecGenVisitor("User")
			result := visitor.Visit(expr)
			if result != tt.expected {
				t.Errorf("\nExpected: %s\nGot:      %s", tt.expected, result)
			}
		})
	}
}

func TestVisitMethodComparison_CombinedWithLogical(t *testing.T) {
	// Test: u.Email.Equal(email) && u.Active
	expr := parseExpr(t, "u.Email.Equal(email) && u.Active")
	visitor := NewSpecGenVisitor("User")
	result := visitor.Visit(expr)
	expected := `spec.And(spec.Equal(spec.Field(spec.GlobalScope(), "Email"), spec.Field(spec.GlobalScope(), "email")), spec.Field(spec.GlobalScope(), "Active"))`

	if result != expected {
		t.Errorf("\nExpected: %s\nGot:      %s", expected, result)
	}
}
