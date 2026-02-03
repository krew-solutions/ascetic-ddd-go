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
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	spec "github.com/krew-solutions/ascetic-ddd-go/asceticddd/specification/domain"
)

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

// Lexer tokenizes JSONPath expressions.
type Lexer struct {
	text     string
	position int
	tokens   []Token
	patterns []tokenPattern
}

// NewLexer creates a new Lexer for the given text.
func NewLexer(text string) *Lexer {
	return &Lexer{
		text:     text,
		position: 0,
		tokens:   nil,
		patterns: []tokenPattern{
			{TokenLBracket, regexp.MustCompile(`^\[`)},
			{TokenRBracket, regexp.MustCompile(`^\]`)},
			{TokenLParen, regexp.MustCompile(`^\(`)},
			{TokenRParen, regexp.MustCompile(`^\)`)},
			{TokenDot, regexp.MustCompile(`^\.`)},
			{TokenDollar, regexp.MustCompile(`^\$`)},
			{TokenAt, regexp.MustCompile(`^@`)},
			{TokenQuestion, regexp.MustCompile(`^\?`)},
			{TokenWildcard, regexp.MustCompile(`^\*`)},
			{TokenAnd, regexp.MustCompile(`^&&`)},                       // RFC 9535: double ampersand
			{TokenOr, regexp.MustCompile(`^\|\|`)},                      // RFC 9535: double pipe
			{TokenEq, regexp.MustCompile(`^==`)},                        // RFC 9535: double equals (must be before single =)
			{TokenNe, regexp.MustCompile(`^!=`)},                        // Must be before NOT to match != as one token
			{TokenGte, regexp.MustCompile(`^>=`)},                       // Must be before GT
			{TokenLte, regexp.MustCompile(`^<=`)},                       // Must be before LT
			{TokenGt, regexp.MustCompile(`^>`)},                         // After GTE
			{TokenLt, regexp.MustCompile(`^<`)},                         // After LTE
			{TokenNot, regexp.MustCompile(`^!`)},                        // RFC 9535: exclamation mark (after !=)
			{TokenNumber, regexp.MustCompile(`^-?\d+\.?\d*`)},           // Numbers (int and float)
			{TokenString, regexp.MustCompile(`^'[^']*'|^"[^"]*"`)},      // Strings
			{TokenPlaceholder, regexp.MustCompile(`^%\(\w+\)[sdf]|^%[sdf]`)}, // Placeholders
			{TokenIdentifier, regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*`)}, // Identifiers
			{TokenWhitespace, regexp.MustCompile(`^\s+`)},               // Whitespace
		},
	}
}

// Tokenize tokenizes the input text.
func (l *Lexer) Tokenize() ([]Token, error) {
	for l.position < len(l.text) {
		matched := false
		remaining := l.text[l.position:]

		for _, pattern := range l.patterns {
			loc := pattern.Pattern.FindStringIndex(remaining)
			if loc != nil && loc[0] == 0 {
				value := remaining[loc[0]:loc[1]]
				if pattern.Type != TokenWhitespace { // Skip whitespace
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
			return nil, fmt.Errorf("unexpected character at position %d: %c", l.position, l.text[l.position])
		}
	}

	return l.tokens, nil
}

// placeholderInfo stores information about a placeholder.
type placeholderInfo struct {
	Name       string
	FormatType string
	Positional bool
}

// NativeParametrizedSpecification is a native JSONPath specification parser
// without external dependencies.
//
// Parses template once, binds different values at execution time.
type NativeParametrizedSpecification struct {
	template             string
	placeholderInfo      []placeholderInfo
	placeholderBindIndex int
	isWildcardContext    bool // Track if we're in wildcard context
}

// Parse parses RFC 9535 compliant JSONPath expression with C-style placeholders
// (native implementation).
//
// Args:
//
//	template: JSONPath with %s, %d, %f or %(name)s placeholders
//
// Returns:
//
//	NativeParametrizedSpecification that can be executed with different parameter values
//
// Examples:
//
//	spec := Parse("$[?@.age > %d]")
//	user := NewDictContext(map[string]any{"age": 30})
//	result, _ := spec.Match(user, 25)  // true
//
//	spec := Parse("$[?@.name == %(name)s]")
//	user := NewDictContext(map[string]any{"name": "Alice"})
//	result, _ := spec.MatchNamed(user, map[string]any{"name": "Alice"})  // true
func Parse(template string) *NativeParametrizedSpecification {
	p := &NativeParametrizedSpecification{
		template:             template,
		placeholderInfo:      nil,
		placeholderBindIndex: 0,
		isWildcardContext:    false,
	}
	p.extractPlaceholders()
	return p
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
	// Create a temp string without named placeholders
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

// parseExpression parses a filter expression from tokens.
func (p *NativeParametrizedSpecification) parseExpression(tokens []Token, start int) (spec.Visitable, int, error) {
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
		// Recursively parse expression inside parentheses
		node, i, err = p.parseExpression(tokens, i)
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
		leftNode, i, err = p.parseFieldAccess(tokens, i)
		if err != nil {
			return nil, i, err
		}

		// Check if leftNode is a CollectionNode (nested wildcard case)
		if _, ok := leftNode.(spec.CollectionNode); ok {
			// This is a nested wildcard - return it directly
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
			rightNode, i, err = p.parseValue(tokens, i)
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
				return nil, i, fmt.Errorf("unexpected operator: %s", opToken.Type)
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

	// Check for AND/OR (RFC 9535: && and ||)
	if i < len(tokens) && (tokens[i].Type == TokenAnd || tokens[i].Type == TokenOr) {
		op := tokens[i].Type
		i++
		var rightExpr spec.Visitable
		rightExpr, i, err = p.parseExpression(tokens, i)
		if err != nil {
			return nil, i, err
		}
		if op == TokenAnd {
			node = spec.And(node, rightExpr)
		} else {
			node = spec.Or(node, rightExpr)
		}
	}

	return node, i, nil
}

// parseFieldAccess parses field access expression (including nested paths and wildcards).
//
// Supports:
//   - Simple: @.field
//   - Nested: @.a.b.c
//   - Nested wildcard: @.items[*][?@.price > 100]
func (p *NativeParametrizedSpecification) parseFieldAccess(tokens []Token, start int) (spec.Visitable, int, error) {
	i := start

	// Check for @ (current item)
	var parent spec.EmptiableObject
	if i < len(tokens) && tokens[i].Type == TokenAt {
		i++
		// Use Item() only in wildcard context, otherwise GlobalScope()
		if p.isWildcardContext {
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
	var fieldChain []string
	for i < len(tokens) && tokens[i].Type == TokenIdentifier {
		fieldChain = append(fieldChain, tokens[i].Value)
		i++

		// Check for dot (continues path)
		if i < len(tokens) && tokens[i].Type == TokenDot {
			// Check if next token is also an identifier
			if i+1 < len(tokens) && tokens[i+1].Type == TokenIdentifier {
				i++ // Skip dot
				continue
			}
			// Dot but no identifier after - break
			break
		}
		// No dot, break
		break
	}

	if len(fieldChain) == 0 {
		return nil, i, fmt.Errorf("expected field name at position %d", i)
	}

	// Check for nested wildcard on last field: field[*][?...]
	if len(fieldChain) > 0 && p.checkNestedWildcard(tokens, i) {
		// Build parent chain for all fields except the last
		for _, field := range fieldChain[:len(fieldChain)-1] {
			parent = spec.Object(parent, field)
		}

		// Last field is the collection for the wildcard
		collectionName := fieldChain[len(fieldChain)-1]
		return p.parseNestedWildcard(tokens, i, parent, collectionName)
	}

	// Build nested Field structure
	// e.g., ["a", "b", "c"] -> Field(Object(Object(parent, "a"), "b"), "c")
	for _, field := range fieldChain[:len(fieldChain)-1] {
		parent = spec.Object(parent, field)
	}

	// Last field
	fieldName := fieldChain[len(fieldChain)-1]
	return spec.Field(parent, fieldName), i, nil
}

// checkNestedWildcard checks if tokens starting at position indicate a nested wildcard pattern.
//
// Pattern: [*][?...]
func (p *NativeParametrizedSpecification) checkNestedWildcard(tokens []Token, start int) bool {
	i := start

	// Check for [*]
	if i+2 < len(tokens) &&
		tokens[i].Type == TokenLBracket &&
		tokens[i+1].Type == TokenWildcard &&
		tokens[i+2].Type == TokenRBracket {
		// Check if followed by [?...]
		if i+3 < len(tokens) && tokens[i+3].Type == TokenLBracket {
			return true
		}
	}

	return false
}

// parseNestedWildcard parses nested wildcard pattern: collection[*][?predicate]
func (p *NativeParametrizedSpecification) parseNestedWildcard(tokens []Token, start int, parent spec.EmptiableObject, collectionName string) (spec.Visitable, int, error) {
	i := start

	// Skip [*]
	if i+2 < len(tokens) &&
		tokens[i].Type == TokenLBracket &&
		tokens[i+1].Type == TokenWildcard &&
		tokens[i+2].Type == TokenRBracket {
		i += 3
	} else {
		return nil, i, fmt.Errorf("expected [*] at position %d", i)
	}

	// Parse filter expression [?...]
	if i < len(tokens) && tokens[i].Type == TokenLBracket {
		// Save current wildcard context
		oldContext := p.isWildcardContext

		// Set wildcard context to True for nested predicate
		p.isWildcardContext = true
		predicate, newI, err := p.parseExpression(tokens, i)
		if err != nil {
			return nil, newI, err
		}
		i = newI

		// Restore previous context
		p.isWildcardContext = oldContext

		// Create Wildcard node
		collectionObj := spec.Object(parent, collectionName)
		return spec.Wildcard(collectionObj, predicate), i, nil
	}

	return nil, i, fmt.Errorf("expected filter expression at position %d", i)
}

// parseValue parses a value (literal or placeholder).
func (p *NativeParametrizedSpecification) parseValue(tokens []Token, start int) (spec.Visitable, int, error) {
	i := start

	if i >= len(tokens) {
		return nil, i, errors.New("expected value but reached end of tokens")
	}

	token := tokens[i]

	switch token.Type {
	case TokenNumber:
		// Parse number
		var value any
		if strings.Contains(token.Value, ".") {
			value, _ = strconv.ParseFloat(token.Value, 64)
		} else {
			value, _ = strconv.Atoi(token.Value)
		}
		return spec.Value(value), i + 1, nil

	case TokenString:
		// Parse string (remove quotes)
		value := token.Value[1 : len(token.Value)-1]
		return spec.Value(value), i + 1, nil

	case TokenPlaceholder:
		// This is a placeholder - will be bound later
		valueNode := p.createPlaceholderValue(token.Value)
		return valueNode, i + 1, nil

	case TokenIdentifier:
		// Could be a boolean literal
		switch strings.ToLower(token.Value) {
		case "true":
			return spec.Value(true), i + 1, nil
		case "false":
			return spec.Value(false), i + 1, nil
		case "null":
			return spec.Value(nil), i + 1, nil
		}
	}

	return nil, i, fmt.Errorf("unexpected token in value position: %s", token)
}

// placeholderMarker is a special marker for placeholders.
type placeholderMarker struct {
	Index int
}

// createPlaceholderValue creates a placeholder value that will be bound later.
func (p *NativeParametrizedSpecification) createPlaceholderValue(placeholderStr string) spec.ValueNode {
	// We'll store a special marker that we'll replace during Match()
	value := spec.Value(placeholderMarker{Index: p.placeholderBindIndex})
	p.placeholderBindIndex++
	return value
}

// parsePath parses the full JSONPath expression (supports nested paths).
//
// Supports:
//   - Simple: $.items[?@.price > 100]
//   - Nested: $.store.items[?@.price > 100]
//   - Deep nested: $.a.b.c.items[?@.x > 1]
func (p *NativeParametrizedSpecification) parsePath(tokens []Token) (spec.Visitable, bool, error) {
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
	var pathChain []string
	for i < len(tokens) && tokens[i].Type == TokenIdentifier {
		pathChain = append(pathChain, tokens[i].Value)
		i++

		// Check for dot (continues path)
		if i < len(tokens) && tokens[i].Type == TokenDot {
			i++
			// Continue to next identifier
		} else {
			// No more dots, break
			break
		}
	}

	if len(pathChain) == 0 {
		// No path found, check if it's just a filter without path
		// e.g., $[?@.age > 25]
		if i < len(tokens) && tokens[i].Type == TokenLBracket {
			// Simple filter without path
			p.isWildcardContext = false
			predicate, _, err := p.parseExpression(tokens, i)
			if err != nil {
				return nil, false, err
			}
			return predicate, false, nil
		}
		return nil, false, errors.New("expected path or filter expression")
	}

	// Build nested Object structure from path chain
	// e.g., ["a", "b", "c"] -> Object(Object(Object(GlobalScope(), "a"), "b"), "c")
	var parent spec.EmptiableObject = spec.GlobalScope()
	for _, pathElement := range pathChain[:len(pathChain)-1] {
		parent = spec.Object(parent, pathElement)
	}

	// Last element in path is the collection name
	collectionName := pathChain[len(pathChain)-1]

	// Check for wildcard [*]
	isWildcard := false
	if i+2 < len(tokens) &&
		tokens[i].Type == TokenLBracket &&
		tokens[i+1].Type == TokenWildcard &&
		tokens[i+2].Type == TokenRBracket {
		isWildcard = true
		i += 3
	}

	// Parse filter expression
	if i < len(tokens) && tokens[i].Type == TokenLBracket {
		if isWildcard {
			// Wildcard with filter
			p.isWildcardContext = true
			predicate, _, err := p.parseExpression(tokens, i)
			if err != nil {
				return nil, false, err
			}
			p.isWildcardContext = false

			// Create Wildcard node
			collectionObj := spec.Object(parent, collectionName)
			return spec.Wildcard(collectionObj, predicate), true, nil
		}
		// Simple filter without wildcard
		p.isWildcardContext = false
		predicate, _, err := p.parseExpression(tokens, i)
		if err != nil {
			return nil, false, err
		}
		return predicate, false, nil
	}

	return nil, false, errors.New("expected filter expression")
}

// bindPlaceholder binds a placeholder to its actual value.
func (p *NativeParametrizedSpecification) bindPlaceholder(value any, params []any, namedParams map[string]any) any {
	marker, ok := value.(placeholderMarker)
	if !ok {
		return value
	}

	if marker.Index < len(p.placeholderInfo) {
		phInfo := p.placeholderInfo[marker.Index]

		// Get actual value from params
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

	// If not found, return marker as-is
	return value
}

// bindValuesInAST recursively binds placeholder values in the AST.
func (p *NativeParametrizedSpecification) bindValuesInAST(node spec.Visitable, params []any, namedParams map[string]any) spec.Visitable {
	switch n := node.(type) {
	case spec.ValueNode:
		// Bind the value if it's a placeholder
		boundValue := p.bindPlaceholder(n.Value(), params, namedParams)
		return spec.Value(boundValue)

	case spec.InfixNode:
		// Recursively bind left and right
		left := p.bindValuesInAST(n.Left(), params, namedParams)
		right := p.bindValuesInAST(n.Right(), params, namedParams)
		return spec.NewInfixNode(left, n.Operator(), right, n.Associativity())

	case spec.PrefixNode:
		operand := p.bindValuesInAST(n.Operand(), params, namedParams)
		return spec.NewPrefixNode(n.Operator(), operand, n.Associativity())

	case spec.CollectionNode:
		// Recursively bind predicate
		predicate := p.bindValuesInAST(n.Predicate(), params, namedParams)
		return spec.Wildcard(n.Parent(), predicate)

	default:
		// For other nodes (Field, Item, GlobalScope, Object), return as-is
		return node
	}
}

// Match checks if data matches the specification with given positional parameters.
//
// Args:
//
//	data: The data object to check (must implement Context interface)
//	params: Parameter values (positional)
//
// Returns:
//
//	True if data matches the specification, False otherwise
//
// Examples:
//
//	spec := Parse("$[?(@.age > %d)]")
//	user := NewDictContext(map[string]any{"age": 30})
//	result, _ := spec.Match(user, 25)  // true
func (p *NativeParametrizedSpecification) Match(data spec.Context, params ...any) (bool, error) {
	return p.matchInternal(data, params, nil)
}

// MatchNamed checks if data matches the specification with named parameters.
//
// Args:
//
//	data: The data object to check (must implement Context interface)
//	params: Parameter values (named)
//
// Returns:
//
//	True if data matches the specification, False otherwise
//
// Examples:
//
//	spec := Parse("$[?(@.age > %(min_age)d)]")
//	user := NewDictContext(map[string]any{"age": 30})
//	result, _ := spec.MatchNamed(user, map[string]any{"min_age": 25})  // true
func (p *NativeParametrizedSpecification) MatchNamed(data spec.Context, namedParams map[string]any) (bool, error) {
	return p.matchInternal(data, nil, namedParams)
}

// matchInternal is the internal implementation of Match and MatchNamed.
func (p *NativeParametrizedSpecification) matchInternal(data spec.Context, params []any, namedParams map[string]any) (bool, error) {
	// Reset placeholder binding index
	p.placeholderBindIndex = 0

	// Tokenize
	lexer := NewLexer(p.template)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return false, err
	}

	// Parse to AST
	specAST, _, err := p.parsePath(tokens)
	if err != nil {
		return false, err
	}

	// Bind placeholder values
	boundAST := p.bindValuesInAST(specAST, params, namedParams)

	// Evaluate using EvaluateVisitor
	visitor := spec.NewEvaluateVisitor(data)
	err = boundAST.Accept(visitor)
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

	// If value is a map, wrap it in NestedDictContext
	if m, ok := value.(map[string]any); ok {
		return NewNestedDictContext(m), nil
	}

	return value, nil
}
