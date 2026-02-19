// Package jsonpath provides a native JSONPath parser for Specification Pattern
// without external dependencies.
//
// Parses RFC 9535 compliant JSONPath expressions with C-style placeholders
// (%s, %d, %f, %(name)s) and converts them directly to Specification AST nodes.
//
// RFC 9535 Compliance:
//   - Uses == for equality (double equals)
//   - Uses && for logical AND (double ampersand)
//   - Uses || for logical OR (double pipe)
//   - Uses ! for logical NOT (exclamation mark)
package jsonpath

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	spec "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
	"github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain/operators"
)

// JSONPathError is the base error type for JSONPath parsing and evaluation errors.
type JSONPathError struct {
	Message string
}

func (e *JSONPathError) Error() string {
	return e.Message
}

// JSONPathSyntaxError is raised when JSONPath expression has invalid syntax.
type JSONPathSyntaxError struct {
	Message    string
	Position   int
	Expression string
	Context    string
}

func (e *JSONPathSyntaxError) Error() string {
	parts := []string{e.Message}

	if e.Position >= 0 {
		parts = append(parts, fmt.Sprintf(" at position %d", e.Position))
	}

	if e.Context != "" {
		parts = append(parts, fmt.Sprintf(" (%s)", e.Context))
	}

	if e.Expression != "" && e.Position >= 0 {
		parts = append(parts, fmt.Sprintf("\n  %s", e.Expression))
		if e.Position < len(e.Expression) {
			parts = append(parts, fmt.Sprintf("\n  %s^", strings.Repeat(" ", e.Position)))
		}
	}

	return strings.Join(parts, "")
}

// JSONPathTypeError is raised when data doesn't conform to expected type/protocol.
type JSONPathTypeError struct {
	Message  string
	Expected string
	Got      string
}

func (e *JSONPathTypeError) Error() string {
	parts := []string{e.Message}
	if e.Expected != "" && e.Got != "" {
		parts = append(parts, fmt.Sprintf(": expected %s, got %s", e.Expected, e.Got))
	}
	return strings.Join(parts, "")
}

// TokenType represents the type of a token.
type TokenType string

const (
	TokenLBracket    TokenType = "LBRACKET"
	TokenRBracket    TokenType = "RBRACKET"
	TokenLParen      TokenType = "LPAREN"
	TokenRParen      TokenType = "RPAREN"
	TokenDot         TokenType = "DOT"
	TokenDollar      TokenType = "DOLLAR"
	TokenAt          TokenType = "AT"
	TokenQuestion    TokenType = "QUESTION"
	TokenWildcard    TokenType = "WILDCARD"
	TokenAnd         TokenType = "AND"
	TokenOr          TokenType = "OR"
	TokenEq          TokenType = "EQ"
	TokenNe          TokenType = "NE"
	TokenGte         TokenType = "GTE"
	TokenLte         TokenType = "LTE"
	TokenGt          TokenType = "GT"
	TokenLt          TokenType = "LT"
	TokenNot         TokenType = "NOT"
	TokenNumber      TokenType = "NUMBER"
	TokenString      TokenType = "STRING"
	TokenPlaceholder TokenType = "PLACEHOLDER"
	TokenIdentifier  TokenType = "IDENTIFIER"
	TokenWhitespace  TokenType = "WHITESPACE"
)

// Token represents a token in the JSONPath expression.
type Token struct {
	Type     TokenType
	Value    string
	Position int
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%s, %q)", t.Type, t.Value)
}

// tokenPattern defines a token type and its regex pattern.
type tokenPattern struct {
	Type    TokenType
	Pattern *regexp.Regexp
}

// Pre-compiled token patterns for performance.
var tokenPatterns = []tokenPattern{
	{TokenLBracket, regexp.MustCompile(`^\[`)},
	{TokenRBracket, regexp.MustCompile(`^\]`)},
	{TokenLParen, regexp.MustCompile(`^\(`)},
	{TokenRParen, regexp.MustCompile(`^\)`)},
	{TokenDot, regexp.MustCompile(`^\.`)},
	{TokenDollar, regexp.MustCompile(`^\$`)},
	{TokenAt, regexp.MustCompile(`^@`)},
	{TokenQuestion, regexp.MustCompile(`^\?`)},
	{TokenWildcard, regexp.MustCompile(`^\*`)},
	{TokenAnd, regexp.MustCompile(`^&&`)},
	{TokenOr, regexp.MustCompile(`^\|\|`)},
	{TokenEq, regexp.MustCompile(`^==`)},
	{TokenNe, regexp.MustCompile(`^!=`)},
	{TokenGte, regexp.MustCompile(`^>=`)},
	{TokenLte, regexp.MustCompile(`^<=`)},
	{TokenGt, regexp.MustCompile(`^>`)},
	{TokenLt, regexp.MustCompile(`^<`)},
	{TokenNot, regexp.MustCompile(`^!`)},
	{TokenNumber, regexp.MustCompile(`^-?\d+\.?\d*`)},
	{TokenString, regexp.MustCompile(`^'[^']*'|^"[^"]*"`)},
	{TokenPlaceholder, regexp.MustCompile(`^%\(\w+\)[sdf]|^%[sdf]`)},
	{TokenIdentifier, regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*`)},
	{TokenWhitespace, regexp.MustCompile(`^\s+`)},
}

// Lexer tokenizes JSONPath expressions.
type Lexer struct {
	text     string
	position int
	tokens   []Token
}

// NewLexer creates a new Lexer for the given text.
func NewLexer(text string) *Lexer {
	return &Lexer{
		text:     text,
		position: 0,
		tokens:   nil,
	}
}

// Tokenize tokenizes the input text.
func (l *Lexer) Tokenize() ([]Token, error) {
	for l.position < len(l.text) {
		matched := false
		remaining := l.text[l.position:]

		for _, pattern := range tokenPatterns {
			loc := pattern.Pattern.FindStringIndex(remaining)
			if loc != nil && loc[0] == 0 {
				value := remaining[loc[0]:loc[1]]
				if pattern.Type != TokenWhitespace {
					l.tokens = append(l.tokens, Token{
						Type:     pattern.Type,
						Value:    value,
						Position: l.position,
					})
				}
				l.position += loc[1]
				matched = true
				break
			}
		}

		if !matched {
			return nil, &JSONPathSyntaxError{
				Message:    fmt.Sprintf("Unexpected character '%c'", l.text[l.position]),
				Position:   l.position,
				Expression: l.text,
				Context:    "expected valid token",
			}
		}
	}

	return l.tokens, nil
}

// parseContext is mutable parsing context passed through parser methods.
// Using a context object instead of instance variables makes the parser
// thread-safe and enables concurrent parsing of different templates.
type parseContext struct {
	placeholderBindIndex int
	isWildcardContext    bool
}

// placeholderInfo stores information about a placeholder.
type placeholderInfo struct {
	Name       string
	FormatType string
	Positional bool
}

// placeholderMarker is a special marker for placeholders.
type placeholderMarker struct {
	Index int
}

// NativeParametrizedSpecification is a native JSONPath specification parser
// without external dependencies.
//
// Parses template once, binds different values at execution time.
// Thread-safe: AST is cached at initialization and never modified.
type NativeParametrizedSpecification struct {
	template        string
	placeholderInfo []placeholderInfo
	ast             spec.Visitable // Cached AST, parsed once at initialization
	isWildcard      bool
}

// Parse parses RFC 9535 compliant JSONPath expression with C-style placeholders
// (native implementation).
//
// The AST is parsed once and cached for all subsequent Match() calls.
// This makes the specification thread-safe and efficient for repeated use.
func Parse(template string) (*NativeParametrizedSpecification, error) {
	p := &NativeParametrizedSpecification{
		template:        template,
		placeholderInfo: nil,
	}
	p.extractPlaceholders()

	// Parse AST once at initialization (cached for all match() calls)
	lexer := NewLexer(template)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, err
	}

	ctx := &parseContext{}
	ast, isWildcard, err := p.parsePath(tokens, ctx)
	if err != nil {
		return nil, err
	}

	p.ast = ast
	p.isWildcard = isWildcard

	return p, nil
}

// MustParse is like Parse but panics on error.
func MustParse(template string) *NativeParametrizedSpecification {
	p, err := Parse(template)
	if err != nil {
		panic(err)
	}
	return p
}

// AST returns the cached AST. Useful for testing.
func (p *NativeParametrizedSpecification) AST() spec.Visitable {
	return p.ast
}

// extractPlaceholders extracts placeholder information from template.
func (p *NativeParametrizedSpecification) extractPlaceholders() {
	// Find named placeholders: %(name)s, %(age)d, %(price)f
	namedPattern := regexp.MustCompile(`%\((\w+)\)([sdf])`)
	for _, match := range namedPattern.FindAllStringSubmatch(p.template, -1) {
		p.placeholderInfo = append(p.placeholderInfo, placeholderInfo{
			Name:       match[1],
			FormatType: match[2],
			Positional: false,
		})
	}

	// Find positional placeholders: %s, %d, %f
	temp := namedPattern.ReplaceAllString(p.template, "")
	positionalPattern := regexp.MustCompile(`%([sdf])`)
	position := 0
	for _, match := range positionalPattern.FindAllStringSubmatch(temp, -1) {
		p.placeholderInfo = append(p.placeholderInfo, placeholderInfo{
			Name:       strconv.Itoa(position),
			FormatType: match[1],
			Positional: true,
		})
		position++
	}
}

// parsePrimary parses a primary expression (comparison, NOT, or parenthesized expression).
// Does NOT handle AND/OR operators - those are handled by parseExpression
// to ensure left-associativity.
func (p *NativeParametrizedSpecification) parsePrimary(tokens []Token, ctx *parseContext, start int) (spec.Visitable, int, error) {
	i := start

	// Skip opening bracket if present
	if i < len(tokens) && tokens[i].Type == TokenLBracket {
		i++
	}

	// Skip question mark if present
	if i < len(tokens) && tokens[i].Type == TokenQuestion {
		i++
	}

	// Check for NOT operator (RFC 9535: !)
	hasNot := false
	if i < len(tokens) && tokens[i].Type == TokenNot {
		hasNot = true
		i++
	}

	var node spec.Visitable
	var err error

	// Skip opening parenthesis if present
	if i < len(tokens) && tokens[i].Type == TokenLParen {
		i++
		// Recursively parse FULL expression inside parentheses (can have && and ||)
		node, i, err = p.parseExpression(tokens, ctx, i)
		if err != nil {
			return nil, i, err
		}
		// Skip closing parenthesis
		if i < len(tokens) && tokens[i].Type == TokenRParen {
			i++
		}
	} else {
		// Parse left side (field access or nested wildcard)
		var leftNode spec.Visitable
		leftNode, i, err = p.parseFieldAccess(tokens, ctx, i)
		if err != nil {
			return nil, i, err
		}

		// Check if leftNode is a CollectionNode (nested wildcard case)
		if _, ok := leftNode.(spec.CollectionNode); ok {
			node = leftNode
		} else {
			// Parse operator
			if i >= len(tokens) {
				if hasNot {
					return spec.Not(leftNode), i, nil
				}
				return leftNode, i, nil
			}

			opToken := tokens[i]
			i++

			// Parse right side (value)
			var rightNode spec.Visitable
			rightNode, i, err = p.parseValue(tokens, ctx, i)
			if err != nil {
				return nil, i, err
			}

			// Create comparison node
			switch opToken.Type {
			case TokenEq:
				node = spec.Equal(leftNode, rightNode)
			case TokenNe:
				node = spec.NotEqual(leftNode, rightNode)
			case TokenGt:
				node = spec.GreaterThan(leftNode, rightNode)
			case TokenLt:
				node = spec.LessThan(leftNode, rightNode)
			case TokenGte:
				node = spec.GreaterThanEqual(leftNode, rightNode)
			case TokenLte:
				node = spec.LessThanEqual(leftNode, rightNode)
			default:
				return nil, i, &JSONPathSyntaxError{
					Message:    fmt.Sprintf("Unexpected operator '%s'", opToken.Value),
					Position:   opToken.Position,
					Expression: p.template,
					Context:    "expected comparison operator (==, !=, <, >, <=, >=)",
				}
			}
		}

		// Skip closing parenthesis if present (from earlier opening)
		if i < len(tokens) && tokens[i].Type == TokenRParen {
			i++
		}
	}

	// Apply NOT if present
	if hasNot {
		node = spec.Not(node)
	}

	return node, i, nil
}

// parseAndExpression parses AND expressions with left-associativity.
// AND (&&) has higher precedence than OR (||), so it binds tighter.
// `a && b && c` becomes `And(And(a, b), c)`.
func (p *NativeParametrizedSpecification) parseAndExpression(tokens []Token, ctx *parseContext, start int) (spec.Visitable, int, error) {
	// Parse first primary expression
	node, i, err := p.parsePrimary(tokens, ctx, start)
	if err != nil {
		return nil, i, err
	}

	// Handle && with left associativity
	for i < len(tokens) && tokens[i].Type == TokenAnd {
		i++
		var rightNode spec.Visitable
		rightNode, i, err = p.parsePrimary(tokens, ctx, i)
		if err != nil {
			return nil, i, err
		}
		node = spec.And(node, rightNode)
	}

	return node, i, nil
}

// parseExpression parses OR expressions with left-associativity (lowest precedence).
//
// Operator precedence (highest to lowest):
// 1. Comparisons (==, !=, <, >, <=, >=)
// 2. NOT (!)
// 3. AND (&&)
// 4. OR (||)
//
// This ensures `a || b && c` is parsed as `Or(a, And(b, c))`.
func (p *NativeParametrizedSpecification) parseExpression(tokens []Token, ctx *parseContext, start int) (spec.Visitable, int, error) {
	// Parse first AND expression (higher precedence)
	node, i, err := p.parseAndExpression(tokens, ctx, start)
	if err != nil {
		return nil, i, err
	}

	// Handle || with left associativity
	for i < len(tokens) && tokens[i].Type == TokenOr {
		i++
		var rightNode spec.Visitable
		rightNode, i, err = p.parseAndExpression(tokens, ctx, i)
		if err != nil {
			return nil, i, err
		}
		node = spec.Or(node, rightNode)
	}

	return node, i, nil
}

// parseIdentifierChain parses a chain of dot-separated identifiers.
// Examples: "a", "a.b", "a.b.c"
func (p *NativeParametrizedSpecification) parseIdentifierChain(tokens []Token, start int) ([]string, int) {
	i := start
	var chain []string

	for i < len(tokens) && tokens[i].Type == TokenIdentifier {
		chain = append(chain, tokens[i].Value)
		i++

		// Check for dot followed by identifier
		if i < len(tokens) &&
			tokens[i].Type == TokenDot &&
			i+1 < len(tokens) &&
			tokens[i+1].Type == TokenIdentifier {
			i++ // Skip dot, continue to next identifier
		} else {
			break
		}
	}

	return chain, i
}

// buildObjectChain builds a chain of Object nodes from a list of field names.
// Example: ["a", "b", "c"] with GlobalScope() parent becomes:
//
//	Object(Object(Object(GlobalScope(), "a"), "b"), "c")
func (p *NativeParametrizedSpecification) buildObjectChain(parent spec.EmptiableObject, names []string) spec.EmptiableObject {
	result := parent
	for _, name := range names {
		result = spec.Object(result, name)
	}
	return result
}

// isWildcardPattern checks if tokens at position form a wildcard pattern [*].
func (p *NativeParametrizedSpecification) isWildcardPattern(tokens []Token, start int) bool {
	return start+2 < len(tokens) &&
		tokens[start].Type == TokenLBracket &&
		tokens[start+1].Type == TokenWildcard &&
		tokens[start+2].Type == TokenRBracket
}

// parseFieldAccess parses field access expression (including nested paths and wildcards).
//
// Supports:
//   - Simple: @.field
//   - Nested: @.a.b.c
//   - Nested wildcard: @.items[*][?@.price > 100]
func (p *NativeParametrizedSpecification) parseFieldAccess(tokens []Token, ctx *parseContext, start int) (spec.Visitable, int, error) {
	i := start

	// Check for @ (current item)
	var parent spec.EmptiableObject
	if i < len(tokens) && tokens[i].Type == TokenAt {
		i++
		if ctx.isWildcardContext {
			parent = spec.Item()
		} else {
			parent = spec.GlobalScope()
		}
	} else {
		parent = spec.GlobalScope()
	}

	// Skip dot
	if i < len(tokens) && tokens[i].Type == TokenDot {
		i++
	}

	// Parse field path chain (e.g., a.b.c)
	fieldChain, i := p.parseIdentifierChain(tokens, i)

	if len(fieldChain) == 0 {
		pos := len(p.template)
		if i < len(tokens) {
			pos = tokens[i].Position
		}
		return nil, i, &JSONPathSyntaxError{
			Message:    "Expected field name",
			Position:   pos,
			Expression: p.template,
			Context:    "after '@.' or '.'",
		}
	}

	// Check for nested wildcard on last field: field[*][?...]
	if p.checkNestedWildcard(tokens, i) {
		// Build parent chain for all fields except the last
		parent = p.buildObjectChain(parent, fieldChain[:len(fieldChain)-1])
		collectionName := fieldChain[len(fieldChain)-1]
		return p.parseNestedWildcard(tokens, ctx, i, parent, collectionName)
	}

	// Build nested Field structure: a.b.c -> Field(Object(Object(parent, "a"), "b"), "c")
	parent = p.buildObjectChain(parent, fieldChain[:len(fieldChain)-1])
	return spec.Field(parent, fieldChain[len(fieldChain)-1]), i, nil
}

// checkNestedWildcard checks if tokens starting at position indicate a nested wildcard pattern.
// Pattern: [*][?...]
func (p *NativeParametrizedSpecification) checkNestedWildcard(tokens []Token, start int) bool {
	return p.isWildcardPattern(tokens, start) &&
		start+3 < len(tokens) &&
		tokens[start+3].Type == TokenLBracket
}

// parseNestedWildcard parses nested wildcard pattern: collection[*][?predicate]
func (p *NativeParametrizedSpecification) parseNestedWildcard(tokens []Token, ctx *parseContext, start int, parent spec.EmptiableObject, collectionName string) (spec.Visitable, int, error) {
	i := start

	// Skip [*]
	if p.isWildcardPattern(tokens, i) {
		i += 3
	} else {
		pos := len(p.template)
		if i < len(tokens) {
			pos = tokens[i].Position
		}
		return nil, i, &JSONPathSyntaxError{
			Message:    "Expected wildcard '[*]'",
			Position:   pos,
			Expression: p.template,
			Context:    "in nested wildcard pattern",
		}
	}

	// Parse filter expression [?...]
	if i < len(tokens) && tokens[i].Type == TokenLBracket {
		// Save current wildcard context
		oldContext := ctx.isWildcardContext

		// Set wildcard context to True for nested predicate
		ctx.isWildcardContext = true
		predicate, newI, err := p.parseExpression(tokens, ctx, i)
		if err != nil {
			return nil, newI, err
		}
		i = newI

		// Restore previous context
		ctx.isWildcardContext = oldContext

		// Create Wildcard node
		collectionObj := spec.Object(parent, collectionName)
		return spec.Wildcard(collectionObj, predicate), i, nil
	}

	pos := len(p.template)
	if i < len(tokens) {
		pos = tokens[i].Position
	}
	return nil, i, &JSONPathSyntaxError{
		Message:    "Expected filter expression '[?...]'",
		Position:   pos,
		Expression: p.template,
		Context:    "after wildcard '[*]'",
	}
}

// parseValue parses a value (literal or placeholder).
func (p *NativeParametrizedSpecification) parseValue(tokens []Token, ctx *parseContext, start int) (spec.Visitable, int, error) {
	i := start

	if i >= len(tokens) {
		return nil, i, &JSONPathSyntaxError{
			Message:    "Unexpected end of expression",
			Position:   len(p.template),
			Expression: p.template,
			Context:    "expected value (number, string, boolean, or placeholder)",
		}
	}

	token := tokens[i]

	switch token.Type {
	case TokenNumber:
		var value any
		if strings.Contains(token.Value, ".") {
			value, _ = strconv.ParseFloat(token.Value, 64)
		} else {
			value, _ = strconv.Atoi(token.Value)
		}
		return spec.Value(value), i + 1, nil

	case TokenString:
		value := token.Value[1 : len(token.Value)-1]
		return spec.Value(value), i + 1, nil

	case TokenPlaceholder:
		valueNode := p.createPlaceholderValue(ctx)
		return valueNode, i + 1, nil

	case TokenIdentifier:
		switch strings.ToLower(token.Value) {
		case "true":
			return spec.Value(true), i + 1, nil
		case "false":
			return spec.Value(false), i + 1, nil
		case "null":
			return spec.Value(nil), i + 1, nil
		}
	}

	return nil, i, &JSONPathSyntaxError{
		Message:    fmt.Sprintf("Unexpected token '%s'", token.Value),
		Position:   token.Position,
		Expression: p.template,
		Context:    "expected value (number, string, boolean, or placeholder)",
	}
}

// createPlaceholderValue creates a placeholder value that will be bound later.
func (p *NativeParametrizedSpecification) createPlaceholderValue(ctx *parseContext) spec.ValueNode {
	value := spec.Value(placeholderMarker{Index: ctx.placeholderBindIndex})
	ctx.placeholderBindIndex++
	return value
}

// parsePath parses the full JSONPath expression (supports nested paths).
func (p *NativeParametrizedSpecification) parsePath(tokens []Token, ctx *parseContext) (spec.Visitable, bool, error) {
	i := 0

	// Skip $
	if i < len(tokens) && tokens[i].Type == TokenDollar {
		i++
	}

	// Skip .
	if i < len(tokens) && tokens[i].Type == TokenDot {
		i++
	}

	// Parse path chain (e.g., a.b.c)
	pathChain, i := p.parseIdentifierChain(tokens, i)

	if len(pathChain) == 0 {
		// No path found, check if it's just a filter without path
		if i < len(tokens) && tokens[i].Type == TokenLBracket {
			ctx.isWildcardContext = false
			predicate, _, err := p.parseExpression(tokens, ctx, i)
			if err != nil {
				return nil, false, err
			}
			return predicate, false, nil
		}
		pos := len(p.template)
		if i < len(tokens) {
			pos = tokens[i].Position
		}
		return nil, false, &JSONPathSyntaxError{
			Message:    "Expected path or filter expression",
			Position:   pos,
			Expression: p.template,
			Context:    "after '$'",
		}
	}

	// Build parent chain and get collection name
	var parent spec.EmptiableObject = spec.GlobalScope()
	parent = p.buildObjectChain(parent, pathChain[:len(pathChain)-1])
	collectionName := pathChain[len(pathChain)-1]

	// Check for wildcard [*]
	isWildcard := p.isWildcardPattern(tokens, i)
	if isWildcard {
		i += 3
	}

	// Parse filter expression
	if i < len(tokens) && tokens[i].Type == TokenLBracket {
		if isWildcard {
			ctx.isWildcardContext = true
			predicate, _, err := p.parseExpression(tokens, ctx, i)
			if err != nil {
				return nil, false, err
			}
			ctx.isWildcardContext = false

			collectionObj := spec.Object(parent, collectionName)
			return spec.Wildcard(collectionObj, predicate), true, nil
		}
		ctx.isWildcardContext = false
		predicate, _, err := p.parseExpression(tokens, ctx, i)
		if err != nil {
			return nil, false, err
		}
		return predicate, false, nil
	}

	pos := len(p.template)
	if i < len(tokens) {
		pos = tokens[i].Position
	}
	return nil, false, &JSONPathSyntaxError{
		Message:    "Expected filter expression '[?...]'",
		Position:   pos,
		Expression: p.template,
		Context:    "after path",
	}
}

// bindPlaceholder binds a placeholder to its actual value.
func (p *NativeParametrizedSpecification) bindPlaceholder(value any, params []any, namedParams map[string]any) any {
	marker, ok := value.(placeholderMarker)
	if !ok {
		return value
	}

	if marker.Index < len(p.placeholderInfo) {
		phInfo := p.placeholderInfo[marker.Index]

		if phInfo.Positional {
			paramIdx, _ := strconv.Atoi(phInfo.Name)
			if paramIdx < len(params) {
				return params[paramIdx]
			}
		} else {
			if val, ok := namedParams[phInfo.Name]; ok {
				return val
			}
		}
	}

	return value
}

// bindValuesInAST recursively binds placeholder values in the AST.
func (p *NativeParametrizedSpecification) bindValuesInAST(node spec.Visitable, params []any, namedParams map[string]any) spec.Visitable {
	switch n := node.(type) {
	case spec.ValueNode:
		boundValue := p.bindPlaceholder(n.Value(), params, namedParams)
		return spec.Value(boundValue)

	case spec.InfixNode:
		left := p.bindValuesInAST(n.Left(), params, namedParams)
		right := p.bindValuesInAST(n.Right(), params, namedParams)
		return spec.NewInfixNode(left, n.Operator(), right, n.Associativity())

	case spec.PrefixNode:
		operand := p.bindValuesInAST(n.Operand(), params, namedParams)
		return spec.NewPrefixNode(n.Operator(), operand, n.Associativity())

	case spec.CollectionNode:
		predicate := p.bindValuesInAST(n.Predicate(), params, namedParams)
		return spec.Wildcard(n.Parent(), predicate)

	default:
		return node
	}
}

// Match checks if data matches the specification with given positional parameters.
func (p *NativeParametrizedSpecification) Match(data spec.Context, params ...any) (bool, error) {
	return p.matchInternal(data, params, nil)
}

// MatchNamed checks if data matches the specification with named parameters.
func (p *NativeParametrizedSpecification) MatchNamed(data spec.Context, namedParams map[string]any) (bool, error) {
	return p.matchInternal(data, nil, namedParams)
}

// matchInternal is the internal implementation of Match and MatchNamed.
func (p *NativeParametrizedSpecification) matchInternal(data spec.Context, params []any, namedParams map[string]any) (bool, error) {
	// Bind placeholder values to cached AST
	boundAST := p.bindValuesInAST(p.ast, params, namedParams)

	// Evaluate using EvaluateVisitor
	visitor := spec.NewEvaluateVisitor(data, operators.NewDefaultRegistry())
	err := boundAST.Accept(visitor)
	if err != nil {
		return false, err
	}

	return visitor.Result()
}

// DictContext is a dictionary-based context for testing.
type DictContext struct {
	data map[string]any
}

// NewDictContext creates a new DictContext.
func NewDictContext(data map[string]any) *DictContext {
	return &DictContext{data: data}
}

// Get returns the value for the given key.
func (c *DictContext) Get(key string) (any, error) {
	value, ok := c.data[key]
	if !ok {
		return nil, fmt.Errorf("key '%s' not found", key)
	}
	return value, nil
}

// NestedDictContext is a nested dictionary-based context for testing nested paths.
type NestedDictContext struct {
	data map[string]any
}

// NewNestedDictContext creates a new NestedDictContext.
func NewNestedDictContext(data map[string]any) *NestedDictContext {
	return &NestedDictContext{data: data}
}

// Get returns the value for the given key, supporting nested dict access.
func (c *NestedDictContext) Get(key string) (any, error) {
	value, ok := c.data[key]
	if !ok {
		return nil, fmt.Errorf("key '%s' not found", key)
	}

	if m, ok := value.(map[string]any); ok {
		return NewNestedDictContext(m), nil
	}

	return value, nil
}
