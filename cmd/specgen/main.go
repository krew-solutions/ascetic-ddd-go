package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// specgen generates AST code from specification predicate functions.
//
// Usage:
//
//	//go:generate go run github.com/krew-solutions/ascetic-ddd-go/cmd/specgen -type=User
//
// This will scan all functions with //spec:sql comment and generate
// corresponding AST builder functions in *_specs_gen.go files.

var (
	typeFlag = flag.String("type", "", "Type name to generate specs for")
)

func main() {
	flag.Parse()

	if *typeFlag == "" {
		log.Fatal("Usage: specgen -type=TypeName")
	}

	// Get the directory from GOFILE env variable (set by go:generate)
	gofile := os.Getenv("GOFILE")
	if gofile == "" {
		// Fallback: use current directory
		gofile = "."
	}

	dir := filepath.Dir(gofile)
	if dir == "" {
		dir = "."
	}

	// Parse Go files in the directory
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		// Skip generated files and test files
		name := fi.Name()
		return !strings.HasSuffix(name, "_test.go") &&
			!strings.HasSuffix(name, "_gen.go") &&
			strings.HasSuffix(name, ".go")
	}, parser.ParseComments)
	if err != nil {
		log.Fatalf("Failed to parse directory: %v", err)
	}

	// Find specification functions
	var specs []SpecFunc
	var pkgName string

	for name, pkg := range pkgs {
		pkgName = name
		for _, file := range pkg.Files {
			specs = append(specs, findSpecFunctions(fset, file, *typeFlag)...)
		}
	}

	if len(specs) == 0 {
		log.Printf("No specification functions found for type %s", *typeFlag)
		return
	}

	// Generate output file
	outputPath := filepath.Join(dir, strings.ToLower(*typeFlag)+"_specs_gen.go")
	err = generateCode(outputPath, pkgName, *typeFlag, specs)
	if err != nil {
		var refused *UnsupportedError
		if errors.As(err, &refused) {
			log.Fatalf("%s: %v", fset.Position(refused.Pos), err)
		}
		log.Fatalf("Failed to generate code: %v", err)
	}

	log.Printf("Generated %s with %d specifications", outputPath, len(specs))
}

// SpecFunc represents a specification function
type SpecFunc struct {
	Name string
	Doc  string
	// Param is the name the predicate gives to its candidate.
	Param string
	// Params are the parameters of the predicate after its candidate: values
	// of the specification, given when its tree is built.
	Params []SpecParam
	// Imports are the imports of the source that the types of Params are of.
	Imports []string
	Body    ast.Expr
	// Refused is why the function, marked as a specification, is not one: it
	// is reported when the code is generated, and nothing is generated.
	Refused error
}

// SpecParam is a parameter of a specification: its name, and its type as the
// source spells it.
type SpecParam struct {
	Name string
	Type string
}

// findSpecFunctions finds all functions with //spec:sql comment
func findSpecFunctions(fset *token.FileSet, file *ast.File, typeName string) []SpecFunc {
	var specs []SpecFunc

	ast.Inspect(file, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		// Check if function has //spec:sql comment
		if funcDecl.Doc == nil {
			return true
		}

		hasSpecComment := false
		for _, comment := range funcDecl.Doc.List {
			if strings.Contains(comment.Text, "spec:sql") {
				hasSpecComment = true
				break
			}
		}

		if !hasSpecComment {
			return true
		}

		// Validate function signature: func(T, ...) bool. A specification with
		// a parameter is the usual kind - "dearer than this", "created since
		// then" - and one of more than its candidate used to be skipped.
		if funcDecl.Type.Params == nil || len(funcDecl.Type.Params.List) == 0 {
			log.Printf("Warning: %s must have its candidate for the first parameter", funcDecl.Name.Name)
			return true
		}

		param := funcDecl.Type.Params.List[0]
		paramType, ok := param.Type.(*ast.Ident)
		if !ok || paramType.Name != typeName {
			return true
		}
		if len(param.Names) != 1 {
			log.Printf("Warning: %s must have one candidate, with a name", funcDecl.Name.Name)
			return true
		}
		params, ok := specParams(funcDecl.Type.Params.List[1:])
		if !ok {
			log.Printf("Warning: the parameters of %s must have names, and none may be variadic", funcDecl.Name.Name)
			return true
		}

		if funcDecl.Type.Results == nil || len(funcDecl.Type.Results.List) != 1 {
			log.Printf("Warning: %s must return bool", funcDecl.Name.Name)
			return true
		}

		// Extract the return expression
		if funcDecl.Body == nil || len(funcDecl.Body.List) == 0 {
			log.Printf("Warning: %s has empty body", funcDecl.Name.Name)
			return true
		}

		// The body is one return and nothing else. The first return used to
		// be taken and the rest not looked at: `if s.Deleted { return false }`
		// before it made a function that refuses the deleted and a query that
		// selects them.
		returnExpr, refused := theReturnOf(funcDecl.Body)

		specs = append(specs, SpecFunc{
			Name:    funcDecl.Name.Name,
			Doc:     funcDecl.Doc.Text(),
			Param:   param.Names[0].Name,
			Params:  params,
			Imports: importsOf(file, funcDecl.Type.Params.List[1:]),
			Body:    returnExpr,
			Refused: refused,
		})

		return true
	})

	return specs
}

// theReturnOf returns what a body of one return statement returns.
func theReturnOf(body *ast.BlockStmt) (ast.Expr, error) {
	if len(body.List) != 1 {
		return nil, unsupported(body.List[1], "the body of a specification is one return: what else it does is not in its tree")
	}
	retStmt, ok := body.List[0].(*ast.ReturnStmt)
	if !ok || len(retStmt.Results) != 1 {
		return nil, unsupported(body.List[0], "the body of a specification is one return of one value")
	}
	return retStmt.Results[0], nil
}

// specParams reads the parameters of a predicate after its candidate.
func specParams(fields []*ast.Field) ([]SpecParam, bool) {
	var params []SpecParam
	for _, field := range fields {
		if _, variadic := field.Type.(*ast.Ellipsis); variadic || len(field.Names) == 0 {
			return nil, false
		}
		for _, name := range field.Names {
			params = append(params, SpecParam{Name: name.Name, Type: types.ExprString(field.Type)})
		}
	}
	return params, true
}

// importsOf returns the imports of the file that the types of the fields are
// of, as the file spells them: the generated code names the same types.
func importsOf(file *ast.File, fields []*ast.Field) []string {
	used := map[string]bool{}
	for _, field := range fields {
		ast.Inspect(field.Type, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok {
					used[pkg.Name] = true
				}
			}
			return true
		})
	}
	var imports []string
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		name := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if !used[name] {
			continue
		}
		if spec.Name != nil {
			imports = append(imports, spec.Name.Name+" "+spec.Path.Value)
		} else {
			imports = append(imports, spec.Path.Value)
		}
	}
	return imports
}

// generateCode generates the *_spec_gen.go file
func generateCode(outputPath, pkgName, typeName string, specs []SpecFunc) error {
	// Into memory first: a specification that is refused leaves no file
	// written by half.
	var code bytes.Buffer
	if err := renderCode(&code, pkgName, typeName, specs); err != nil {
		return err
	}
	return os.WriteFile(outputPath, code.Bytes(), 0o644)
}

// renderCode writes the generated file.
func renderCode(f io.Writer, pkgName, typeName string, specs []SpecFunc) error {
	// Write header
	fmt.Fprintf(f, "// Code generated by specgen. DO NOT EDIT.\n\n")
	fmt.Fprintf(f, "package %s\n\n", pkgName)
	fmt.Fprintf(f, "import (\n")
	// What the types of the parameters are of
	imported := map[string]bool{}
	for _, s := range specs {
		for _, imp := range s.Imports {
			if !imported[imp] {
				imported[imp] = true
				fmt.Fprintf(f, "\t%s\n", imp)
			}
		}
	}
	if len(imported) > 0 {
		fmt.Fprintf(f, "\n")
	}
	fmt.Fprintf(f, "\tspec \"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain\"\n")
	fmt.Fprintf(f, ")\n\n")

	// Generate AST builder for each spec
	for _, s := range specs {
		visitor := NewSpecGenVisitor(typeName).withRoot(s.Param)

		// What cannot be a specification is reported where it stands.
		if s.Refused != nil {
			return fmt.Errorf("%s: %w", s.Name, s.Refused)
		}
		body, err := visitor.Visit(s.Body)
		if err != nil {
			return fmt.Errorf("%s: %w", s.Name, err)
		}

		// Generate AST function
		fmt.Fprintf(f, "// %sAST returns AST for %s\n", s.Name, s.Name)
		declared := make([]string, 0, len(s.Params))
		for _, param := range s.Params {
			declared = append(declared, param.Name+" "+param.Type)
		}
		signature := strings.Join(declared, ", ")

		fmt.Fprintf(f, "func %sAST(%s) spec.Visitable {\n", s.Name, signature)
		fmt.Fprintf(f, "\treturn %s\n", body)
		fmt.Fprintf(f, "}\n\n")

		// The tree is all that is generated. A query cannot be written without
		// knowing the table, and what a field is called there is the
		// repository's to say: it compiles the tree with its own context. A
		// `...SQL()` used to be generated, which compiled the tree under the
		// names of Go's fields and imported the infrastructure into the
		// package of the domain.
	}

	return nil
}

// SpecGenVisitor converts Go AST expressions to Specification AST builder code.
// Implements the Visitor pattern for go/ast nodes.
type SpecGenVisitor struct {
	// typeName is the main type being processed (e.g., "User", "Order")
	typeName string
	// rootName is the name the predicate gives to its candidate (e.g., "u");
	// empty if it is not known, and then any name that is not an item's is
	// taken for it.
	rootName string
	// itemName is the current item variable name in wildcard context (e.g., "item")
	itemName string
	// outerItems are the item names of the enclosing collections.
	outerItems []string
	// inWildcard indicates if we're inside a wildcard predicate
	inWildcard bool
}

// NewSpecGenVisitor creates a new visitor for the given type.
func NewSpecGenVisitor(typeName string) *SpecGenVisitor {
	return &SpecGenVisitor{
		typeName:   typeName,
		itemName:   "",
		inWildcard: false,
	}
}

// withRoot returns a new visitor that knows the candidate by its name.
func (v *SpecGenVisitor) withRoot(rootName string) *SpecGenVisitor {
	return &SpecGenVisitor{
		typeName:   v.typeName,
		rootName:   rootName,
		itemName:   v.itemName,
		outerItems: v.outerItems,
		inWildcard: v.inWildcard,
	}
}

// withWildcardContext returns a new visitor configured for wildcard context.
func (v *SpecGenVisitor) withWildcardContext(itemName string) *SpecGenVisitor {
	outerItems := v.outerItems
	if v.inWildcard && v.itemName != itemName {
		outerItems = append(append([]string{}, v.outerItems...), v.itemName)
	}
	return &SpecGenVisitor{
		typeName:   v.typeName,
		rootName:   v.rootName,
		itemName:   itemName,
		outerItems: outerItems,
		inWildcard: true,
	}
}

// UnsupportedError is a construct that cannot be a specification, where it
// stands. It used to be generated as spec.Value(nil) with a TODO in a comment:
// a specification that compiles, and is null.
type UnsupportedError struct {
	Pos     token.Pos
	Message string
}

func (e *UnsupportedError) Error() string {
	return e.Message
}

func unsupported(node ast.Node, format string, args ...any) error {
	return &UnsupportedError{Pos: node.Pos(), Message: fmt.Sprintf(format, args...)}
}

// Visit dispatches to the appropriate visit method based on node type.
func (v *SpecGenVisitor) Visit(expr ast.Expr) (string, error) {
	switch e := expr.(type) {
	case *ast.BinaryExpr:
		return v.VisitBinaryExpr(e)
	case *ast.UnaryExpr:
		return v.VisitUnaryExpr(e)
	case *ast.SelectorExpr:
		return v.VisitSelectorExpr(e)
	case *ast.CallExpr:
		return v.VisitCallExpr(e)
	case *ast.BasicLit:
		return v.VisitBasicLit(e)
	case *ast.Ident:
		return v.VisitIdent(e)
	case *ast.ParenExpr:
		return v.VisitParenExpr(e)
	default:
		return "", unsupported(expr, "unsupported expression %T", expr)
	}
}

// isNil tells whether the expression is the nil literal.
func isNil(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "nil"
}

// isOutside tells whether the expression is a name from outside the
// predicate: what it is equal to is known when the tree is built, not when it
// is generated.
func isOutside(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name != "nil" && ident.Name != "true" && ident.Name != "false"
}

// equality generates `==` or `!=`. With nil it is the null test: written as
// it stands it is `a = NULL`, which is null and true of nothing. With a name
// from outside it is decided when the tree is built.
func (v *SpecGenVisitor) equality(x, y ast.Expr, operator, comparison, nullTest string) (string, error) {
	if isNil(x) && isNil(y) {
		return "", unsupported(x, "nil is compared with nil")
	}
	if isNil(x) {
		x, y = y, x
	}
	left, err := v.Visit(x)
	if err != nil {
		return "", err
	}
	if isNil(y) {
		return fmt.Sprintf("%s(%s)", nullTest, left), nil
	}
	right, err := v.Visit(y)
	if err != nil {
		return "", err
	}
	if isOutside(x) || isOutside(y) {
		return fmt.Sprintf("spec.EqualityOrNullTest(%q, %s, %s)", operator, left, right), nil
	}
	return fmt.Sprintf("%s(%s, %s)", comparison, left, right), nil
}

// VisitBinaryExpr handles binary expressions (comparisons, logical, arithmetic).
func (v *SpecGenVisitor) VisitBinaryExpr(expr *ast.BinaryExpr) (string, error) {
	switch expr.Op {
	case token.EQL: // ==
		return v.equality(expr.X, expr.Y, "=", "spec.Equal", "spec.IsNull")
	case token.NEQ: // !=
		return v.equality(expr.X, expr.Y, "!=", "spec.NotEqual", "spec.IsNotNull")
	}

	left, err := v.Visit(expr.X)
	if err != nil {
		return "", err
	}
	right, err := v.Visit(expr.Y)
	if err != nil {
		return "", err
	}

	switch expr.Op {
	// Comparison
	case token.LSS: // <
		return fmt.Sprintf("spec.LessThan(%s, %s)", left, right), nil
	case token.LEQ: // <=
		return fmt.Sprintf("spec.LessThanEqual(%s, %s)", left, right), nil
	case token.GTR: // >
		return fmt.Sprintf("spec.GreaterThan(%s, %s)", left, right), nil
	case token.GEQ: // >=
		return fmt.Sprintf("spec.GreaterThanEqual(%s, %s)", left, right), nil

	// Logical
	case token.LAND: // &&
		return fmt.Sprintf("spec.And(%s, %s)", left, right), nil
	case token.LOR: // ||
		return fmt.Sprintf("spec.Or(%s, %s)", left, right), nil

	// Arithmetic
	case token.ADD: // +
		return fmt.Sprintf("spec.Add(%s, %s)", left, right), nil
	case token.SUB: // -
		return fmt.Sprintf("spec.Sub(%s, %s)", left, right), nil
	case token.MUL: // *
		return fmt.Sprintf("spec.Mul(%s, %s)", left, right), nil
	case token.QUO: // /
		return fmt.Sprintf("spec.Div(%s, %s)", left, right), nil
	case token.REM: // %
		return fmt.Sprintf("spec.Mod(%s, %s)", left, right), nil

	// Bitwise
	case token.SHL: // <<
		return fmt.Sprintf("spec.LeftShift(%s, %s)", left, right), nil
	case token.SHR: // >>
		return fmt.Sprintf("spec.RightShift(%s, %s)", left, right), nil

	default:
		// & (bitwise AND), | (bitwise OR), ^ (bitwise XOR) are not yet
		// implemented in spec
		return "", unsupported(expr, "unsupported operator %v", expr.Op)
	}
}

// VisitUnaryExpr handles unary expressions (!, -, +).
func (v *SpecGenVisitor) VisitUnaryExpr(expr *ast.UnaryExpr) (string, error) {
	// `-5` is the constant it looks like, not a negation of `5`.
	if lit, ok := expr.X.(*ast.BasicLit); ok && expr.Op == token.SUB && (lit.Kind == token.INT || lit.Kind == token.FLOAT) {
		return fmt.Sprintf("spec.Value(-%s)", lit.Value), nil
	}

	operand, err := v.Visit(expr.X)
	if err != nil {
		return "", err
	}

	switch expr.Op {
	case token.NOT: // !
		return fmt.Sprintf("spec.Not(%s)", operand), nil
	case token.SUB: // - (negation)
		return fmt.Sprintf("spec.Neg(%s)", operand), nil
	case token.ADD: // + (positive, no-op)
		return operand, nil
	default:
		return "", unsupported(expr, "unsupported unary operator %v", expr.Op)
	}
}

// scopeOf returns the scope a base identifier stands for: the item of the
// nearest collection, or the candidate.
//
// A name that is neither used to be read as the candidate: the item of an
// outer collection named from the predicate of an inner one, which the tree
// cannot name - it has one "@", the nearest - became a field of the candidate.
func (v *SpecGenVisitor) scopeOf(base *ast.Ident) (string, error) {
	if v.inWildcard && base.Name == v.itemName {
		// Inside wildcard, referring to item
		return "spec.Item()", nil
	}
	for _, outer := range v.outerItems {
		if base.Name == outer {
			return "", unsupported(base, "%q is the item of an outer collection: only the nearest item can be named", base.Name)
		}
	}
	if v.rootName != "" && base.Name != v.rootName {
		return "", unsupported(base, "%q is neither the candidate %q nor the item of a collection", base.Name, v.rootName)
	}
	// Normal context, referring to root object
	return "spec.GlobalScope()", nil
}

// VisitSelectorExpr handles field access (e.g., u.Age, item.Price, u.Profile.Age).
func (v *SpecGenVisitor) VisitSelectorExpr(expr *ast.SelectorExpr) (string, error) {
	// Build the chain of field accesses
	var path []string
	var baseIdent *ast.Ident

	// Walk up the chain to collect all field names
	current := expr
	for {
		path = append([]string{current.Sel.Name}, path...) // prepend

		switch x := current.X.(type) {
		case *ast.SelectorExpr:
			// Nested selector (e.g., u.Profile.Age)
			current = x
			continue
		case *ast.Ident:
			// Base identifier (u, item, etc.)
			baseIdent = x
		default:
			// Unknown base
			return "", unsupported(current.X, "unsupported selector base %T", current.X)
		}
		break
	}

	// Determine the scope based on context
	scope, err := v.scopeOf(baseIdent)
	if err != nil {
		return "", err
	}

	// Build nested Object chain for all but the last field
	for i := 0; i < len(path)-1; i++ {
		scope = fmt.Sprintf("spec.Object(%s, %q)", scope, path[i])
	}

	// Last element is the field
	return fmt.Sprintf("spec.Field(%s, %q)", scope, path[len(path)-1]), nil
}

// VisitCallExpr handles function calls (Any, All, IsNull, method calls).
func (v *SpecGenVisitor) VisitCallExpr(expr *ast.CallExpr) (string, error) {
	switch fun := expr.Fun.(type) {
	case *ast.Ident:
		switch fun.Name {
		case "Any", "All":
			return v.visitAnyAll(expr, fun.Name)
		}
	case *ast.SelectorExpr:
		switch fun.Sel.Name {
		case "Any", "All":
			return v.visitAnyAll(expr, fun.Sel.Name)
		case "IsNull":
			return v.visitIsNull(expr)
		case "IsNotNull":
			return v.visitIsNotNull(expr)

		// Value Object comparison methods
		case "Equal", "Equals", "Eq":
			return v.visitMethodEquality(expr, fun, "=", "spec.Equal", "spec.IsNull")
		case "NotEqual", "NotEquals", "Ne", "Neq":
			return v.visitMethodEquality(expr, fun, "!=", "spec.NotEqual", "spec.IsNotNull")
		// Before and After are how a time.Time is compared, which has no `<`.
		// The table goes by the name of the method: the generator reads the
		// source and does not know the types.
		case "LessThan", "Lt", "Before":
			return v.visitMethodComparison(expr, fun, "spec.LessThan")
		case "LessThanOrEqual", "LessThanEqual", "Lte", "Le":
			return v.visitMethodComparison(expr, fun, "spec.LessThanEqual")
		case "GreaterThan", "Gt", "After":
			return v.visitMethodComparison(expr, fun, "spec.GreaterThan")
		case "GreaterThanOrEqual", "GreaterThanEqual", "Gte", "Ge":
			return v.visitMethodComparison(expr, fun, "spec.GreaterThanEqual")
		}
	}

	return "", unsupported(expr, "unsupported call %T", expr.Fun)
}

// VisitBasicLit handles literal values (numbers, strings).
func (v *SpecGenVisitor) VisitBasicLit(expr *ast.BasicLit) (string, error) {
	return fmt.Sprintf("spec.Value(%s)", expr.Value), nil
}

// VisitIdent handles identifiers (true, false, nil, names from outside).
func (v *SpecGenVisitor) VisitIdent(expr *ast.Ident) (string, error) {
	// Boolean constants, nil, or a name from outside the predicate - a
	// constant or a variable of the package - which is a value. It used to be
	// read as a field of the candidate of that name: `u.Email.Equal(email)`
	// compiled to `Email = email`.
	if expr.Name == v.rootName || (v.inWildcard && expr.Name == v.itemName) {
		return "", unsupported(expr, "%q as a whole is not a value: name a member of it", expr.Name)
	}
	return fmt.Sprintf("spec.Value(%s)", expr.Name), nil
}

// VisitParenExpr handles parenthesized expressions.
func (v *SpecGenVisitor) VisitParenExpr(expr *ast.ParenExpr) (string, error) {
	return v.Visit(expr.X)
}

// visitAnyAll handles Any/All collection predicates.
func (v *SpecGenVisitor) visitAnyAll(expr *ast.CallExpr, funcName string) (string, error) {
	// Any/All(collection, func(item Type) bool { return predicate })
	if len(expr.Args) != 2 {
		return "", unsupported(expr, "%s requires 2 arguments", funcName)
	}

	// First arg is the collection selector (e.g., store.Items or region.Categories)
	collectionExpr := expr.Args[0]
	collectionSelector, ok := collectionExpr.(*ast.SelectorExpr)
	if !ok {
		return "", unsupported(collectionExpr, "%s first arg must be selector", funcName)
	}

	collectionField := collectionSelector.Sel.Name

	// Build parent scope for collection
	var parentScope string
	switch x := collectionSelector.X.(type) {
	case *ast.Ident:
		// item.Collection (nested wildcard: region.Categories, category.Items)
		// or root.Collection (store.Items, o.Regions)
		scope, err := v.scopeOf(x)
		if err != nil {
			return "", err
		}
		parentScope = scope
	case *ast.SelectorExpr:
		// Nested case: store.Nested.Items
		field, err := v.VisitSelectorExpr(x)
		if err != nil {
			return "", err
		}
		// Convert Field to Object
		parentScope = fmt.Sprintf("spec.Object(%s.Object(), %s.Name())", field, field)
	default:
		return "", unsupported(collectionSelector.X, "unsupported collection parent %T", collectionSelector.X)
	}

	// Second arg is the lambda function
	lambdaExpr := expr.Args[1]
	funcLit, ok := lambdaExpr.(*ast.FuncLit)
	if !ok {
		return "", unsupported(lambdaExpr, "%s second arg must be func literal", funcName)
	}

	// Extract lambda parameter name
	if len(funcLit.Type.Params.List) != 1 || len(funcLit.Type.Params.List[0].Names) != 1 {
		return "", unsupported(funcLit, "%s lambda must have exactly one param", funcName)
	}
	lambdaItemName := funcLit.Type.Params.List[0].Names[0].Name

	// Extract lambda body (should be a return statement)
	if len(funcLit.Body.List) != 1 {
		return "", unsupported(funcLit, "%s lambda must have exactly one statement", funcName)
	}
	retStmt, ok := funcLit.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(retStmt.Results) != 1 {
		return "", unsupported(funcLit, "%s lambda must have return statement", funcName)
	}

	// Convert predicate in wildcard context using a new visitor
	wildcardVisitor := v.withWildcardContext(lambdaItemName)
	predicate, err := wildcardVisitor.Visit(retStmt.Results[0])
	if err != nil {
		return "", err
	}

	// Generate Wildcard node
	collection := fmt.Sprintf("spec.Object(%s, %q)", parentScope, collectionField)
	if funcName == "All" {
		// "All satisfy" is "none fails": All used to be generated as Any.
		return fmt.Sprintf("spec.Not(spec.Wildcard(%s, spec.Not(%s)))", collection, predicate), nil
	}
	return fmt.Sprintf("spec.Wildcard(%s, %s)", collection, predicate), nil
}

// visitIsNull handles value.IsNull() calls.
func (v *SpecGenVisitor) visitIsNull(expr *ast.CallExpr) (string, error) {
	sel, ok := expr.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", unsupported(expr, "IsNull: invalid selector")
	}

	operand, err := v.Visit(sel.X)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("spec.IsNull(%s)", operand), nil
}

// visitIsNotNull handles value.IsNotNull() calls.
func (v *SpecGenVisitor) visitIsNotNull(expr *ast.CallExpr) (string, error) {
	sel, ok := expr.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", unsupported(expr, "IsNotNull: invalid selector")
	}

	operand, err := v.Visit(sel.X)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("spec.IsNotNull(%s)", operand), nil
}

// visitMethodEquality handles Value Object method calls like receiver.Equal(arg).
func (v *SpecGenVisitor) visitMethodEquality(expr *ast.CallExpr, sel *ast.SelectorExpr, operator, comparison, nullTest string) (string, error) {
	if len(expr.Args) != 1 {
		return "", unsupported(expr, "%s requires exactly 1 argument", sel.Sel.Name)
	}
	// receiver becomes left operand, method argument becomes right operand
	return v.equality(sel.X, expr.Args[0], operator, comparison, nullTest)
}

// visitMethodComparison handles Value Object method calls like receiver.LessThan(arg).
func (v *SpecGenVisitor) visitMethodComparison(expr *ast.CallExpr, sel *ast.SelectorExpr, specFunc string) (string, error) {
	if len(expr.Args) != 1 {
		return "", unsupported(expr, "%s requires exactly 1 argument", sel.Sel.Name)
	}

	// receiver becomes left operand
	left, err := v.Visit(sel.X)
	if err != nil {
		return "", err
	}
	// method argument becomes right operand
	right, err := v.Visit(expr.Args[0])
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s(%s, %s)", specFunc, left, right), nil
}
