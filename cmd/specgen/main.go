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
	"sort"
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
	pkgs, err := parser.ParseDir(fset, dir, sourceFiles, parser.ParseComments)
	if err != nil {
		log.Fatalf("Failed to parse directory: %v", err)
	}

	// Find specification functions
	pkgName, specs, err := collect(fset, pkgs, *typeFlag)
	if err != nil {
		log.Fatalf("%v", err)
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

// sourceFiles tells which files of the directory are read: not the generated
// ones, not the tests.
func sourceFiles(fi os.FileInfo) bool {
	name := fi.Name()
	return !strings.HasSuffix(name, "_test.go") &&
		!strings.HasSuffix(name, "_gen.go") &&
		strings.HasSuffix(name, ".go")
}

// collect finds the specifications of the type in the packages of a
// directory, in the order of their files, and names the package the
// generated file is of: the package of the specifications.
//
// The parser does not read build tags, so a directory may hold more than one
// package - a tool under `//go:build ignore` beside the package - and both
// are maps. The generated file used to be of whichever package came last,
// `package main` in a package called shop, and its functions came in the
// order the files did that run.
func collect(fset *token.FileSet, pkgs map[string]*ast.Package, typeName string) (string, []SpecFunc, error) {
	var pkgName string
	var specs []SpecFunc
	for _, name := range sorted(pkgs) {
		pkg := pkgs[name]
		var found []SpecFunc
		for _, path := range sorted(pkg.Files) {
			found = append(found, findSpecFunctions(fset, pkg.Files[path], typeName)...)
		}
		if len(found) == 0 {
			continue
		}
		if pkgName != "" {
			return "", nil, fmt.Errorf("specifications of %s in two packages, %s and %s: one file is generated, of one package", typeName, pkgName, name)
		}
		pkgName = name
		specs = found
	}
	return pkgName, specs, nil
}

// sorted returns the keys of a map in order.
func sorted[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// SpecFunc represents a specification function
type SpecFunc struct {
	Name string
	Doc  string
	// Param is the name the predicate gives to its candidate.
	Param string
	// Receiver is the receiver of a predicate written as a method: the
	// specification, whose fields are its constants. Nil for a function.
	Receiver *Receiver
	// Params are the parameters of the predicate after its candidate: values
	// of the specification, given when its tree is built.
	Params []SpecParam
	// Imports are the imports of the source that the types of Params are of.
	Imports []string
	// OptionPackage is what the source calls the package of Option; see
	// optionPackageOf.
	OptionPackage string
	Body          ast.Expr
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

// Receiver is the receiver of `IsSatisfiedBy`: its name, if it has one, and
// its type as the source spells it, `Dearer` or `*Dearer`.
type Receiver struct {
	Name string
	Type string
}

// generatedName is the name a receiver or a parameter has in the generated
// code. The generated file imports the package as `spec`, so an author's
// `spec` is written as `spec_` there: it used to be written as it was, in
// code that did not compile.
func generatedName(name string) string {
	if name == "spec" {
		return "spec_"
	}
	return name
}

// findSpecFunctions finds all functions with //spec:sql comment
func findSpecFunctions(fset *token.FileSet, file *ast.File, typeName string) []SpecFunc {
	var specs []SpecFunc

	ast.Inspect(file, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		if !marked(funcDecl) {
			return true
		}

		// A method is the predicate of a specification type: IsSatisfiedBy,
		// of the candidate alone; the receiver's fields are the constants.
		// It used to be generated as a function, its receiver dropped.
		var receiver *Receiver
		if funcDecl.Recv != nil {
			if funcDecl.Name.Name != "IsSatisfiedBy" {
				log.Printf("Warning: %s: the method of a specification is IsSatisfiedBy, of which Expression is generated", funcDecl.Name.Name)
				return true
			}
			if funcDecl.Type.Params != nil && len(funcDecl.Type.Params.List) > 1 {
				log.Printf("Warning: IsSatisfiedBy of %s takes the candidate alone: the constants of a specification are its fields", types.ExprString(funcDecl.Recv.List[0].Type))
				return true
			}
			receiver = &Receiver{Type: types.ExprString(funcDecl.Recv.List[0].Type)}
			if names := funcDecl.Recv.List[0].Names; len(names) == 1 {
				receiver.Name = names[0].Name
			}
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
			Name:          funcDecl.Name.Name,
			Doc:           funcDecl.Doc.Text(),
			Param:         param.Names[0].Name,
			Receiver:      receiver,
			Params:        params,
			Imports:       importsOf(file, funcDecl.Type.Params.List[1:]),
			OptionPackage: optionPackageOf(file),
			Body:          returnExpr,
			Refused:       refused,
		})

		return true
	})

	return specs
}

// marked tells whether the function is marked as a specification: a line of
// its doc comment is `//spec:sql`, as a directive is written. A comment that
// mentioned the marker, or showed it in an example, used to mark the
// function as well. The marker written with a space, `// spec:sql`, marks
// nothing and is said so.
func marked(funcDecl *ast.FuncDecl) bool {
	if funcDecl.Doc == nil {
		return false
	}
	for _, comment := range funcDecl.Doc.List {
		text := strings.TrimRight(comment.Text, " \t")
		if text == "//spec:sql" {
			return true
		}
		if strings.TrimSpace(strings.TrimPrefix(text, "//")) == "spec:sql" {
			log.Printf("Warning: %s: the marker is the line //spec:sql, without a space", funcDecl.Name.Name)
		}
	}
	return false
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

// optionPackagePath is how the path of the package of Option ends, whatever
// module it is in.
const optionPackagePath = "asceticddd/option"

// optionPackageOf returns what the file calls the package of Option: the name
// it imports it under, "." if it takes its names for its own, and none if it
// does not import it.
func optionPackageOf(file *ast.File) string {
	for _, spec := range file.Imports {
		path := strings.Trim(spec.Path.Value, `"`)
		if path != optionPackagePath && !strings.HasSuffix(path, "/"+optionPackagePath) {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}
		return "option"
	}
	return ""
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
		visitor := visitorOf(typeName, s)

		// What cannot be a specification is reported where it stands.
		if s.Refused != nil {
			return fmt.Errorf("%s: %w", s.Name, s.Refused)
		}
		body, err := visitor.Visit(s.Body)
		if err != nil {
			return fmt.Errorf("%s: %w", s.Name, err)
		}

		if s.Receiver != nil {
			// The tree of a specification type: with IsSatisfiedBy, the
			// pair a repository takes.
			receiver := strings.TrimSpace(generatedName(s.Receiver.Name) + " " + s.Receiver.Type)
			fmt.Fprintf(f, "// Expression is IsSatisfiedBy as a tree: to compile, or to evaluate.\n")
			fmt.Fprintf(f, "func (%s) Expression() spec.Visitable {\n", receiver)
			fmt.Fprintf(f, "\treturn %s\n", body)
			fmt.Fprintf(f, "}\n\n")
			continue
		}

		// Generate AST function
		fmt.Fprintf(f, "// %sAST returns AST for %s\n", s.Name, s.Name)
		declared := make([]string, 0, len(s.Params))
		for _, param := range s.Params {
			declared = append(declared, generatedName(param.Name)+" "+param.Type)
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
	// optionPackage is what the source calls the package of Option; see
	// optionPackageOf.
	optionPackage string
	// held is what an Option holds, under the name the predicate of IsSomeAnd
	// or of IsNothingOr gives it.
	held map[string]held
	// receiverName is what a method calls its receiver, the specification:
	// a path from it is a value, a constant of the specification. Empty
	// for a function, and for a receiver without a name.
	receiverName string
}

// held is what stands behind the name given to what an Option holds: the
// Option, which is its value or the null.
type held struct {
	// scope and path are of a member of the candidate or of the item: where
	// its path starts, and the names along it.
	scope string
	path  []string
	// value is the code of a value from outside, a parameter.
	value string
}

// code is the code of the node of the Option.
func (h held) code() string {
	if h.value != "" {
		return h.value
	}
	return field(h.scope, h.path)
}

// field is the code of the member at the path from the scope.
func field(scope string, path []string) string {
	for _, name := range path[:len(path)-1] {
		scope = fmt.Sprintf("spec.Object(%s, %q)", scope, name)
	}
	return fmt.Sprintf("spec.Field(%s, %q)", scope, path[len(path)-1])
}

// NewSpecGenVisitor creates a new visitor for the given type.
func NewSpecGenVisitor(typeName string) *SpecGenVisitor {
	return &SpecGenVisitor{
		typeName:   typeName,
		itemName:   "",
		inWildcard: false,
	}
}

// visitorOf returns the visitor of a specification: it knows the candidate by
// its name, and the package of Option by the name the source gives it.
func visitorOf(typeName string, s SpecFunc) *SpecGenVisitor {
	visitor := NewSpecGenVisitor(typeName).withRoot(s.Param)
	visitor.optionPackage = s.OptionPackage
	if s.Receiver != nil {
		visitor.receiverName = s.Receiver.Name
	}
	return visitor
}

// withRoot returns a new visitor that knows the candidate by its name.
func (v *SpecGenVisitor) withRoot(rootName string) *SpecGenVisitor {
	return &SpecGenVisitor{
		typeName:      v.typeName,
		rootName:      rootName,
		itemName:      v.itemName,
		outerItems:    v.outerItems,
		inWildcard:    v.inWildcard,
		optionPackage: v.optionPackage,
		held:          v.held,
		receiverName:  v.receiverName,
	}
}

// withHeld returns a new visitor of a predicate of what an Option holds,
// which it calls name.
func (v *SpecGenVisitor) withHeld(name string, option held) *SpecGenVisitor {
	next := *v
	next.held = map[string]held{name: option}
	for other, option := range v.held {
		if other != name {
			next.held[other] = option
		}
	}
	return &next
}

// withWildcardContext returns a new visitor configured for wildcard context.
// What the item so far holds goes out of reach with it; what the candidate
// holds, or a value from outside, stays.
func (v *SpecGenVisitor) withWildcardContext(itemName string) *SpecGenVisitor {
	outerItems := v.outerItems
	if v.inWildcard && v.itemName != itemName {
		outerItems = append(append([]string{}, v.outerItems...), v.itemName)
	}
	kept := map[string]held{}
	for name, option := range v.held {
		switch {
		case name == itemName:
		case option.scope == "spec.Item()":
			outerItems = append(append([]string{}, outerItems...), name)
		default:
			kept[name] = option
		}
	}
	return &SpecGenVisitor{
		typeName:      v.typeName,
		rootName:      v.rootName,
		itemName:      itemName,
		outerItems:    outerItems,
		inWildcard:    true,
		optionPackage: v.optionPackage,
		held:          kept,
		receiverName:  v.receiverName,
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
// predicate, or a field of the receiver: what it is equal to is known when
// the tree is built, not when it is generated. The name of what a member
// holds is the member's.
func (v *SpecGenVisitor) isOutside(expr ast.Expr) bool {
	if _, ok := v.ofReceiver(expr); ok {
		return true
	}
	ident, ok := expr.(*ast.Ident)
	if !ok || ident.Name == "nil" || ident.Name == "true" || ident.Name == "false" {
		return false
	}
	option, isHeld := v.held[ident.Name]
	return !isHeld || option.value != ""
}

// isReceiver tells whether the name is the receiver's, here: a name is the
// nearest of that name, and an item or what is held may be called the same.
func (v *SpecGenVisitor) isReceiver(name string) bool {
	if v.receiverName == "" || name != v.receiverName {
		return false
	}
	if _, isHeld := v.held[name]; isHeld {
		return false
	}
	return !(v.inWildcard && name == v.itemName)
}

// ofReceiver returns the code of a field of the receiver, reached by fields:
// a value, as the tree function writes it. Not the receiver as a whole, which
// is not a value.
func (v *SpecGenVisitor) ofReceiver(expr ast.Expr) (string, bool) {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	var path []string
	current := sel
	for {
		path = append([]string{current.Sel.Name}, path...)
		switch x := current.X.(type) {
		case *ast.SelectorExpr:
			current = x
			continue
		case *ast.Ident:
			if !v.isReceiver(x.Name) {
				return "", false
			}
			return fmt.Sprintf("spec.Value(%s.%s)", generatedName(x.Name), strings.Join(path, ".")), true
		default:
			return "", false
		}
	}
}

// optionMaker tells which maker of an Option is called, "Some" or "Nothing";
// none if the call is of neither. A maker is told by what the source imports,
// not by its spelling: another package's Some makes another Option, which no
// reader of a tree reads.
func (v *SpecGenVisitor) optionMaker(call *ast.CallExpr) string {
	fun := call.Fun
	if explicit, ok := fun.(*ast.IndexExpr); ok { // Nothing[T]
		fun = explicit.X
	}
	var name *ast.Ident
	switch f := fun.(type) {
	case *ast.Ident:
		if v.optionPackage != "." {
			return ""
		}
		name = f
	case *ast.SelectorExpr:
		// A name declared in the source - a parameter called `option` - is
		// not the package.
		pkg, ok := f.X.(*ast.Ident)
		if !ok || pkg.Obj != nil || v.optionPackage == "." || pkg.Name != v.optionPackage {
			return ""
		}
		name = f.Sel
	default:
		return ""
	}
	if name.Name == "Some" || name.Name == "Nothing" {
		return name.Name
	}
	return ""
}

// heldBySome returns what `option.Some(x)` holds, x, and any other expression as
// it is: an Option is what it holds, or a null, to both readers of a tree.
func (v *SpecGenVisitor) heldBySome(expr ast.Expr) ast.Expr {
	call, ok := expr.(*ast.CallExpr)
	if ok && v.optionMaker(call) == "Some" && len(call.Args) == 1 {
		return v.heldBySome(call.Args[0])
	}
	return expr
}

// isNull tells whether the expression is the null: the nil literal, or an
// Option that holds nothing.
func (v *SpecGenVisitor) isNull(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	return isNil(expr) || ok && v.optionMaker(call) == "Nothing" && len(call.Args) == 0
}

// equality generates `==` or `!=`. With nil it is the null test: written as
// it stands it is `a = NULL`, which is null and true of nothing. So it is with
// an Option that holds nothing. With a name from outside it is decided when
// the tree is built.
func (v *SpecGenVisitor) equality(x, y ast.Expr, operator, comparison, nullTest string) (string, error) {
	x, y = v.heldBySome(x), v.heldBySome(y)
	if v.isNull(x) && v.isNull(y) {
		return "", unsupported(x, "nil is compared with nil")
	}
	if v.isNull(x) {
		x, y = y, x
	}
	left, err := v.Visit(x)
	if err != nil {
		return "", err
	}
	if v.isNull(y) {
		return fmt.Sprintf("%s(%s)", nullTest, left), nil
	}
	right, err := v.Visit(y)
	if err != nil {
		return "", err
	}
	if v.isOutside(x) || v.isOutside(y) {
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
	if value, ok := v.ofReceiver(expr); ok {
		return value, nil
	}
	scope, path, err := v.memberOf(expr)
	if err != nil {
		return "", err
	}
	return field(scope, path), nil
}

// memberOf returns where the path of a member starts, and the names along it.
func (v *SpecGenVisitor) memberOf(expr *ast.SelectorExpr) (string, []string, error) {
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
			return "", nil, unsupported(current.X, "unsupported selector base %T", current.X)
		}
		break
	}

	if v.isReceiver(baseIdent.Name) {
		return "", nil, unsupported(expr, "%q is the specification: its fields are values, not members of the candidate", baseIdent.Name)
	}

	// A member of what an Option holds is a member of the Option's.
	if option, ok := v.held[baseIdent.Name]; ok {
		if option.value != "" {
			return "", nil, unsupported(expr, "a member of what an Option from outside holds is not a value the tree can name")
		}
		return option.scope, append(append([]string{}, option.path...), path...), nil
	}

	// Determine the scope based on context
	scope, err := v.scopeOf(baseIdent)
	if err != nil {
		return "", nil, err
	}
	return scope, path, nil
}

// VisitCallExpr handles function calls (Any, All, IsNull, method calls).
func (v *SpecGenVisitor) VisitCallExpr(expr *ast.CallExpr) (string, error) {
	switch v.optionMaker(expr) {
	case "Some":
		if len(expr.Args) != 1 {
			return "", unsupported(expr, "Some requires exactly 1 argument")
		}
		return v.Visit(expr.Args[0])
	case "Nothing":
		if len(expr.Args) != 0 {
			return "", unsupported(expr, "Nothing takes no arguments")
		}
		return "spec.Value(nil)", nil
	}

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

		// An Option is what it holds, or a null, to both readers of a tree:
		// to ask one whether it holds anything is the null test, and what it
		// holds is the member itself.
		case "IsNothing":
			return v.visitOptionTest(expr, fun, "spec.IsNull")
		case "IsSome":
			return v.visitOptionTest(expr, fun, "spec.IsNotNull")
		case "Unwrap":
			return v.visitUnwrap(expr, fun)
		case "IsSomeAnd":
			return v.visitHeld(expr, fun, "spec.And", "spec.IsNotNull")
		case "IsNothingOr":
			return v.visitHeld(expr, fun, "spec.Or", "spec.IsNull")

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
	if option, ok := v.held[expr.Name]; ok {
		return option.code(), nil
	}
	if expr.Name == v.rootName || (v.inWildcard && expr.Name == v.itemName) || v.isReceiver(expr.Name) {
		return "", unsupported(expr, "%q as a whole is not a value: name a member of it", expr.Name)
	}
	for _, outer := range v.outerItems {
		if expr.Name == outer {
			return "", unsupported(expr, "%q is of the item of an outer collection: only the nearest item can be named", expr.Name)
		}
	}
	return fmt.Sprintf("spec.Value(%s)", generatedName(expr.Name)), nil
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

// visitOptionTest handles option.IsNothing() and option.IsSome() calls.
func (v *SpecGenVisitor) visitOptionTest(expr *ast.CallExpr, sel *ast.SelectorExpr, nullTest string) (string, error) {
	if len(expr.Args) != 0 {
		return "", unsupported(expr, "%s takes no arguments", sel.Sel.Name)
	}
	operand, err := v.Visit(sel.X)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s(%s)", nullTest, operand), nil
}

// visitUnwrap handles option.Unwrap() calls: what an Option holds is the
// Option itself, a member or a value from outside, which is its value or the
// null. Not what a parameter holds when the tree is built: the predicate
// unwraps it behind its guard, and of a Nothing never does.
func (v *SpecGenVisitor) visitUnwrap(expr *ast.CallExpr, sel *ast.SelectorExpr) (string, error) {
	if len(expr.Args) != 0 {
		return "", unsupported(expr, "Unwrap takes no arguments")
	}
	return v.Visit(sel.X)
}

// visitHeld handles option.IsSomeAnd(func(held T) bool { return predicate }):
// the Option is not null and the predicate is true of it; and IsNothingOr: it
// is null, or the predicate is. The name the predicate gives to what is held
// stands for the Option.
//
// The null test beside the predicate makes the whole of two values, as it is
// in Go: of a Nothing the predicate is null, and `false AND null` is false,
// `true OR null` true. So the function and its tree agree under a `!` too,
// and nothing is unwrapped.
func (v *SpecGenVisitor) visitHeld(expr *ast.CallExpr, sel *ast.SelectorExpr, join, nullTest string) (string, error) {
	if len(expr.Args) != 1 {
		return "", unsupported(expr, "%s takes the predicate of what the Option holds", sel.Sel.Name)
	}
	predicate, ok := expr.Args[0].(*ast.FuncLit)
	if !ok {
		return "", unsupported(expr.Args[0], "the predicate of %s is a func literal", sel.Sel.Name)
	}
	params := predicate.Type.Params.List
	if len(params) != 1 || len(params[0].Names) != 1 {
		return "", unsupported(predicate, "the predicate of %s takes what the Option holds", sel.Sel.Name)
	}
	if len(predicate.Body.List) != 1 {
		return "", unsupported(predicate, "the predicate of %s is one return", sel.Sel.Name)
	}
	returned, ok := predicate.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(returned.Results) != 1 {
		return "", unsupported(predicate, "the predicate of %s is one return of one value", sel.Sel.Name)
	}

	var option held
	switch x := sel.X.(type) {
	case *ast.SelectorExpr:
		if value, ok := v.ofReceiver(x); ok {
			option = held{value: value}
			break
		}
		scope, path, err := v.memberOf(x)
		if err != nil {
			return "", err
		}
		option = held{scope: scope, path: path}
	case *ast.Ident:
		if known, ok := v.held[x.Name]; ok {
			option = known
			break
		}
		value, err := v.VisitIdent(x)
		if err != nil {
			return "", err
		}
		option = held{value: value}
	default:
		return "", unsupported(sel.X, "an Option asked for what it holds is a member or a parameter")
	}

	body, err := v.withHeld(params[0].Names[0].Name, option).Visit(returned.Results[0])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s(%s(%s), %s)", join, nullTest, option.code(), body), nil
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
