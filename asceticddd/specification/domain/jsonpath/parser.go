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
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf16"

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
	// RFC 9535: a fraction has digits, so `1.` is a number and a dot; an
	// exponent makes a float, `1e3`.
	{TokenNumber, regexp.MustCompile(`^-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?`)},
	// RFC 9535: a backslash takes the character after it along, so a quote of
	// the kind the string is written in can stand inside it. Which escapes
	// there are is readString's to say, with a position.
	{TokenString, regexp.MustCompile(`^(?s:'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*")`)},
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

		if !matched && (l.text[l.position] == '\'' || l.text[l.position] == '"') {
			// A quote starts a string or nothing: the pattern of a string did
			// not match, so its closing quote is not there.
			return nil, &JSONPathSyntaxError{
				Message:    "Unterminated string",
				Position:   l.position,
				Expression: l.text,
				Context:    "expected closing quote",
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

// escapes is what the character after a backslash stands for in a string
// (RFC 9535, 2.3.5.1). The escape of a code point, a "u" and four hexadecimal
// digits, is read apart: it is not one character long.
var escapes = map[byte]rune{
	'\\': '\\',
	'\'': '\'',
	'"':  '"',
	'/':  '/',
	'b':  '\b',
	'f':  '\f',
	'n':  '\n',
	'r':  '\r',
	't':  '\t',
}

// hex4 reads four hexadecimal digits at the given place.
func hex4(text string, at int) (rune, bool) {
	if at+4 > len(text) {
		return 0, false
	}
	code, err := strconv.ParseUint(text[at:at+4], 16, 32)
	if err != nil {
		return 0, false
	}
	return rune(code), true
}

// readEscape reads the escape that starts with the backslash at the given
// place: the character it stands for, and where the text goes on.
func readEscape(text string, at int) (rune, int, bool) {
	if at+1 >= len(text) {
		return 0, 0, false
	}
	letter := text[at+1]
	if character, ok := escapes[letter]; ok {
		return character, at + 2, true
	}
	if letter != 'u' {
		return 0, 0, false
	}
	high, ok := hex4(text, at+2)
	if !ok {
		return 0, 0, false
	}
	if utf16.IsSurrogate(high) {
		// A code point beyond the basic plane is a pair of escapes.
		if at+7 >= len(text) || text[at+6] != '\\' || text[at+7] != 'u' {
			return 0, 0, false
		}
		low, ok := hex4(text, at+8)
		if !ok {
			return 0, 0, false
		}
		// Half a pair, or its halves the wrong way round, decodes to U+FFFD.
		character := utf16.DecodeRune(high, low)
		if character == unicode.ReplacementChar {
			return 0, 0, false
		}
		return character, at + 12, true
	}
	return high, at + 6, true
}

// readString reads the string a STRING token spells: its quotes off, its
// escapes read. The token used to be read as spelling[1:len-1]: a backslash
// was a backslash, so a quote of the kind the string is written in had no
// spelling.
func readString(spelling string, position int, expression string) (string, error) {
	var characters strings.Builder
	end := len(spelling) - 1
	for at := 1; at < end; {
		if spelling[at] != '\\' {
			characters.WriteByte(spelling[at])
			at++
			continue
		}
		character, next, ok := readEscape(spelling[:end], at)
		if !ok {
			return "", &JSONPathSyntaxError{
				Message:    "Invalid escape",
				Position:   position + at,
				Expression: expression,
				Context:    `expected \\, \', \", \/, \b, \f, \n, \r, \t or \uXXXX`,
			}
		}
		characters.WriteRune(character)
		at = next
	}
	return characters.String(), nil
}

// readNumber reads the number a NUMBER token spells: an integer, or - with a
// fraction or an exponent - a float. The evaluator computes an integer as
// PostgreSQL does a bigint and a float as a double precision; a literal that
// is neither is refused, and used to be read as the greatest integer.
func readNumber(spelling string, position int, expression string) (any, error) {
	outOfRange := &JSONPathSyntaxError{
		Message:    "Number out of range",
		Position:   position,
		Expression: expression,
		Context:    "expected a number that fits",
	}
	if strings.ContainsAny(spelling, ".eE") {
		// `1e999` parses, to infinity and an error.
		value, err := strconv.ParseFloat(spelling, 64)
		if err != nil || math.IsInf(value, 0) {
			return nil, outOfRange
		}
		return value, nil
	}
	value, err := strconv.Atoi(spelling)
	if err != nil {
		return nil, outOfRange
	}
	return value, nil
}

// requireParameterOfKind requires a parameter to be what the letter of its
// placeholder asks for: %d takes an integer, %f a number, %s a value of any
// type. A nil fits any. The letter used to be read and never used.
func requireParameterOfKind(formatType, name string, value any) (any, error) {
	if value == nil {
		return value, nil
	}
	isInteger, isFloat := false, false
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		isInteger = true
	case float32, float64:
		isFloat = true
	}
	switch {
	case formatType == "d" && !isInteger:
		return nil, &JSONPathTypeError{
			Message:  fmt.Sprintf("Placeholder %s does not take this parameter", name),
			Expected: "an integer",
			Got:      fmt.Sprintf("%T", value),
		}
	case formatType == "f" && !isInteger && !isFloat:
		return nil, &JSONPathTypeError{
			Message:  fmt.Sprintf("Placeholder %s does not take this parameter", name),
			Expected: "a number",
			Got:      fmt.Sprintf("%T", value),
		}
	}
	return value, nil
}

// parseContext is the parsing context passed through parser methods: what "@"
// is inside the filter being parsed. A value, passed by value: a filter on a
// collection is parsed with a context of its own, so there is nothing to
// restore after it.
type parseContext struct {
	isWildcardContext bool
}

// placeholderInfo stores information about a placeholder.
type placeholderInfo struct {
	Name       string
	FormatType string
	Positional bool
}

// parameters is what a template is bound to: positional or named.
type parameters struct {
	positional []any
	named      map[string]any
}

// builder is what a rule of the parser returns: not a node, but the function
// that builds the node once the parameters are there.
//
// A template is a translation that waits for its parameters. Kept as a tree,
// it needs a word for "a value comes here later", and a specification has no
// such word. That word was a marker inside a Value, which every reader of the
// tree took for a value: a template that was not bound evaluated to false and
// compiled to a query with the marker for a parameter. Kept as a function, a
// tree comes of it with values in it or does not come at all.
type builder func(params parameters) (spec.Visitable, error)

func constant(node spec.Visitable) builder {
	return func(parameters) (spec.Visitable, error) { return node, nil }
}

func negation(operand builder) builder {
	return func(params parameters) (spec.Visitable, error) {
		node, err := operand(params)
		if err != nil {
			return nil, err
		}
		return spec.Not(node), nil
	}
}

// comparison builds the comparison of what left and right build. `@.a == %s`
// bound to nil is the null test, as `@.a == null` is: both operands are values
// by the time the comparison is made.
func comparison(operator operators.Operator, left, right builder) builder {
	return func(params parameters) (spec.Visitable, error) {
		leftNode, err := left(params)
		if err != nil {
			return nil, err
		}
		rightNode, err := right(params)
		if err != nil {
			return nil, err
		}
		return spec.EqualityOrNullTest(operator, leftNode, rightNode), nil
	}
}

func connective(node func(spec.Visitable, ...spec.Visitable) spec.InfixNode, left, right builder) builder {
	return func(params parameters) (spec.Visitable, error) {
		leftNode, err := left(params)
		if err != nil {
			return nil, err
		}
		rightNode, err := right(params)
		if err != nil {
			return nil, err
		}
		return node(leftNode, rightNode), nil
	}
}

func someItem(collection spec.ObjectNode, predicate builder) builder {
	return func(params parameters) (spec.Visitable, error) {
		node, err := predicate(params)
		if err != nil {
			return nil, err
		}
		return spec.Wildcard(collection, node), nil
	}
}

// NativeParametrizedSpecification is a native JSONPath specification parser
// without external dependencies.
//
// Parses template once, binds different values at execution time.
// Thread-safe: what is kept of the template is a function of the parameters,
// never modified. Bind calls it and returns the specification, Match evaluates
// that.
type NativeParametrizedSpecification struct {
	template        string
	placeholderInfo []placeholderInfo
	builder         builder // Parsed once at initialization
	isWildcard      bool
	registry        *operators.OperatorRegistry
}

// Parse parses RFC 9535 compliant JSONPath expression with C-style placeholders
// (native implementation).
//
// The template is parsed once and cached for all subsequent Match() calls.
// This makes the specification thread-safe and efficient for repeated use.
func Parse(template string) (*NativeParametrizedSpecification, error) {
	p := &NativeParametrizedSpecification{
		template:        template,
		placeholderInfo: nil,
		registry:        operators.NewDefaultRegistry(),
	}

	// Parse once at initialization (cached for all match() calls)
	lexer := NewLexer(template)
	tokens, err := lexer.Tokenize()
	if err != nil {
		return nil, err
	}

	// Placeholders are those of the tokens, so one inside a string literal is
	// text, and they are listed in the order they stand, which is the order
	// they are bound in.
	if err := p.extractPlaceholders(tokens); err != nil {
		return nil, err
	}

	build, isWildcard, err := p.parsePath(tokens, parseContext{})
	if err != nil {
		return nil, err
	}

	p.builder = build
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

// extractPlaceholders extracts placeholder information from the tokens of the
// template. Positional and named placeholders in one template are refused:
// Match takes the ones and MatchNamed the others, and neither can bind a
// template that mixes them. They used to be listed named ones first, so a
// mixed template bound the wrong parameters.
func (p *NativeParametrizedSpecification) extractPlaceholders(tokens []Token) error {
	var named, positional []Token
	for _, token := range tokens {
		if token.Type != TokenPlaceholder {
			continue
		}
		if strings.HasPrefix(token.Value, "%(") {
			named = append(named, token)
		} else {
			positional = append(positional, token)
		}
	}
	if len(named) > 0 && len(positional) > 0 {
		return &JSONPathSyntaxError{
			Message:    "Positional and named placeholders in one template",
			Position:   max(named[0].Position, positional[0].Position),
			Expression: p.template,
			Context:    "expected placeholders of one style",
		}
	}

	// Named placeholders: %(name)s, %(age)d, %(price)f
	for _, token := range named {
		p.placeholderInfo = append(p.placeholderInfo, placeholderInfo{
			Name:       token.Value[2 : len(token.Value)-2],
			FormatType: token.Value[len(token.Value)-1:],
			Positional: false,
		})
	}
	// Positional placeholders: %s, %d, %f
	for position, token := range positional {
		p.placeholderInfo = append(p.placeholderInfo, placeholderInfo{
			Name:       strconv.Itoa(position),
			FormatType: token.Value[len(token.Value)-1:],
			Positional: true,
		})
	}
	return nil
}

// position returns the position in the template of the token at i, or of its end.
func (p *NativeParametrizedSpecification) position(tokens []Token, i int) int {
	if i < len(tokens) {
		return tokens[i].Position
	}
	return len(p.template)
}

// expect requires the token at i to be of the given type.
//
// The grammar has no token that "may be present": a bracket the parser stepped
// over where there was one, and did not miss where there was none, let a
// template with a bracket left open, or one too many, be read as if it were
// well formed.
func (p *NativeParametrizedSpecification) expect(tokens []Token, i int, tokenType TokenType, message, context string) (int, error) {
	if i < len(tokens) && tokens[i].Type == tokenType {
		return i + 1, nil
	}
	return i, &JSONPathSyntaxError{
		Message:    message,
		Position:   p.position(tokens, i),
		Expression: p.template,
		Context:    context,
	}
}

// parseFilter parses a filter: "[" "?" expression "]".
func (p *NativeParametrizedSpecification) parseFilter(tokens []Token, ctx parseContext, start int) (builder, int, error) {
	message := "Expected filter expression '[?...]'"
	i, err := p.expect(tokens, start, TokenLBracket, message, "expected '['")
	if err != nil {
		return nil, i, err
	}
	i, err = p.expect(tokens, i, TokenQuestion, message, "expected '?'")
	if err != nil {
		return nil, i, err
	}
	node, i, err := p.parseExpression(tokens, ctx, i)
	if err != nil {
		return nil, i, err
	}
	i, err = p.expect(tokens, i, TokenRBracket, "Expected ']'", "expected end of filter expression")
	if err != nil {
		return nil, i, err
	}
	return node, i, nil
}

var comparisons = map[TokenType]operators.Operator{
	TokenEq:  operators.OperatorEq,
	TokenNe:  operators.OperatorNe,
	TokenGt:  operators.OperatorGt,
	TokenLt:  operators.OperatorLt,
	TokenGte: operators.OperatorGte,
	TokenLte: operators.OperatorLte,
}

// parsePrimary parses a primary expression (comparison or NOT).
// Does NOT handle AND/OR operators - those are handled by parseExpression
// to ensure left-associativity.
//
//	primary    = "!" primary | comparison
//	comparison = operand ( ( "==" | "!=" | "<" | "<=" | ">" | ">=" ) operand )?
//
// A comparison has at most one operator: it does not associate.
func (p *NativeParametrizedSpecification) parsePrimary(tokens []Token, ctx parseContext, start int) (builder, int, error) {
	i := start

	// Check for NOT operator (RFC 9535: !)
	if i < len(tokens) && tokens[i].Type == TokenNot {
		node, i, err := p.parsePrimary(tokens, ctx, i+1)
		if err != nil {
			return nil, i, err
		}
		return negation(node), i, nil
	}

	// Parse left side (field access, nested wildcard, value or parentheses)
	leftNode, i, err := p.parseOperand(tokens, ctx, i)
	if err != nil {
		return nil, i, err
	}

	// Parse operator
	if i >= len(tokens) {
		return leftNode, i, nil
	}
	operator, ok := comparisons[tokens[i].Type]
	if !ok {
		return leftNode, i, nil
	}

	// Parse right side
	rightNode, i, err := p.parseOperand(tokens, ctx, i+1)
	if err != nil {
		return nil, i, err
	}

	// Create comparison node; `@.a == null` is the null test
	return comparison(operator, leftNode, rightNode), i, nil
}

// parseOperand parses an operand: either side of a comparison, or a test by itself.
//
//	operand = "(" expression ")" | value | query
func (p *NativeParametrizedSpecification) parseOperand(tokens []Token, ctx parseContext, start int) (builder, int, error) {
	i := start

	if i < len(tokens) && tokens[i].Type == TokenLParen {
		// Recursively parse FULL expression inside parentheses (can have && and ||)
		node, i, err := p.parseExpression(tokens, ctx, i+1)
		if err != nil {
			return nil, i, err
		}
		// The parenthesis that closes this group: an inner primary used to
		// take it for its own, and `(a || b) && c` was read `a || (b && c)`.
		i, err = p.expect(tokens, i, TokenRParen, "Expected ')'", "expected closing parenthesis")
		if err != nil {
			return nil, i, err
		}
		return node, i, nil
	}

	if i < len(tokens) && (tokens[i].Type == TokenAt || tokens[i].Type == TokenDollar) {
		return p.parseFieldAccess(tokens, ctx, i)
	}

	return p.parseValue(tokens, i)
}

// parseAndExpression parses AND expressions with left-associativity.
// AND (&&) has higher precedence than OR (||), so it binds tighter.
// `a && b && c` becomes `And(And(a, b), c)`.
func (p *NativeParametrizedSpecification) parseAndExpression(tokens []Token, ctx parseContext, start int) (builder, int, error) {
	// Parse first primary expression
	node, i, err := p.parsePrimary(tokens, ctx, start)
	if err != nil {
		return nil, i, err
	}

	// Handle && with left associativity
	for i < len(tokens) && tokens[i].Type == TokenAnd {
		i++
		var rightNode builder
		rightNode, i, err = p.parsePrimary(tokens, ctx, i)
		if err != nil {
			return nil, i, err
		}
		node = connective(spec.And, node, rightNode)
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
func (p *NativeParametrizedSpecification) parseExpression(tokens []Token, ctx parseContext, start int) (builder, int, error) {
	// Parse first AND expression (higher precedence)
	node, i, err := p.parseAndExpression(tokens, ctx, start)
	if err != nil {
		return nil, i, err
	}

	// Handle || with left associativity
	for i < len(tokens) && tokens[i].Type == TokenOr {
		i++
		var rightNode builder
		rightNode, i, err = p.parseAndExpression(tokens, ctx, i)
		if err != nil {
			return nil, i, err
		}
		node = connective(spec.Or, node, rightNode)
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
//	query = ( "@" | "$" ) ( "." name )+ ( "[*]" filter )?
//
// Supports:
//   - Simple: @.field
//   - Nested: @.a.b.c
//   - Nested wildcard: @.items[*][?@.price > 100]
//   - From the candidate, inside a filter as well: $.limit
func (p *NativeParametrizedSpecification) parseFieldAccess(tokens []Token, ctx parseContext, start int) (builder, int, error) {
	i := start

	// Check for @ (current item)
	var parent spec.EmptiableObject
	if i < len(tokens) && tokens[i].Type == TokenAt {
		i++
		// Use Item() only in wildcard context, otherwise GlobalScope()
		if ctx.isWildcardContext {
			parent = spec.Item()
		} else {
			parent = spec.GlobalScope()
		}
	} else {
		// $ (the candidate), whatever filter it stands in
		var err error
		i, err = p.expect(tokens, i, TokenDollar, "Expected '@' or '$'", "expected field access")
		if err != nil {
			return nil, i, err
		}
		parent = spec.GlobalScope()
	}

	// Parse field path chain (e.g., a.b.c)
	var fieldChain []string
	if i < len(tokens) && tokens[i].Type == TokenDot {
		fieldChain, i = p.parseIdentifierChain(tokens, i+1)
	}

	if len(fieldChain) == 0 {
		return nil, i, &JSONPathSyntaxError{
			Message:    "Expected field name",
			Position:   p.position(tokens, i),
			Expression: p.template,
			Context:    "after '@.' or '$.'",
		}
	}

	// Check for nested wildcard on last field: field[*][?...]
	if i < len(tokens) && tokens[i].Type == TokenLBracket {
		// Build parent chain for all fields except the last
		parent = p.buildObjectChain(parent, fieldChain[:len(fieldChain)-1])
		collectionName := fieldChain[len(fieldChain)-1]
		return p.parseNestedWildcard(tokens, i, parent, collectionName)
	}

	// Build nested Field structure: a.b.c -> Field(Object(Object(parent, "a"), "b"), "c")
	parent = p.buildObjectChain(parent, fieldChain[:len(fieldChain)-1])
	return constant(spec.Field(parent, fieldChain[len(fieldChain)-1])), i, nil
}

// parseNestedWildcard parses nested wildcard pattern: collection[*][?predicate]
//
// A filter applies to the items of a collection, so the wildcard is required:
// `collection[?predicate]` used to lose its path and have the predicate
// applied to the candidate.
func (p *NativeParametrizedSpecification) parseNestedWildcard(tokens []Token, start int, parent spec.EmptiableObject, collectionName string) (builder, int, error) {
	i := start

	// Skip [*]
	if p.isWildcardPattern(tokens, i) {
		i += 3
	} else {
		return nil, i, &JSONPathSyntaxError{
			Message:    "Expected wildcard '[*]'",
			Position:   p.position(tokens, i+1),
			Expression: p.template,
			Context:    "a filter applies to the items of a collection",
		}
	}

	// Parse filter expression [?...]
	// "@" is the item inside the predicate
	predicate, i, err := p.parseFilter(tokens, parseContext{isWildcardContext: true}, i)
	if err != nil {
		return nil, i, err
	}

	// Create Wildcard node
	collectionObj := spec.Object(parent, collectionName)
	return someItem(collectionObj, predicate), i, nil
}

// parseValue parses a value (literal or placeholder).
func (p *NativeParametrizedSpecification) parseValue(tokens []Token, start int) (builder, int, error) {
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
		value, err := readNumber(token.Value, token.Position, p.template)
		if err != nil {
			return nil, i, err
		}
		return constant(spec.Value(value)), i + 1, nil

	case TokenString:
		value, err := readString(token.Value, token.Position, p.template)
		if err != nil {
			return nil, i, err
		}
		return constant(spec.Value(value)), i + 1, nil

	case TokenPlaceholder:
		return p.createPlaceholderValue(tokens, i), i + 1, nil

	case TokenIdentifier:
		switch strings.ToLower(token.Value) {
		case "true":
			return constant(spec.Value(true)), i + 1, nil
		case "false":
			return constant(spec.Value(false)), i + 1, nil
		case "null":
			return constant(spec.Value(nil)), i + 1, nil
		}
	}

	return nil, i, &JSONPathSyntaxError{
		Message:    fmt.Sprintf("Unexpected token '%s'", token.Value),
		Position:   token.Position,
		Expression: p.template,
		Context:    "expected value (number, string, boolean, or placeholder)",
	}
}

// createPlaceholderValue creates the builder of a value that will be bound
// later: by what its entry of placeholderInfo says, which is found by the
// place of the token among the placeholders of the template.
func (p *NativeParametrizedSpecification) createPlaceholderValue(tokens []Token, i int) builder {
	index := 0
	for _, token := range tokens[:i] {
		if token.Type == TokenPlaceholder {
			index++
		}
	}
	info := p.placeholderInfo[index]
	return func(params parameters) (spec.Visitable, error) {
		value, err := p.bindPlaceholder(info, params)
		if err != nil {
			return nil, err
		}
		return spec.Value(value), nil
	}
}

// parsePath parses the full JSONPath expression (supports nested paths).
//
//	template = "$" ( filter | ( "." name )+ "[*]" filter )
//
// and nothing after it.
func (p *NativeParametrizedSpecification) parsePath(tokens []Token, ctx parseContext) (builder, bool, error) {
	i, err := p.expect(tokens, 0, TokenDollar, "Expected '$'", "a template starts at the root")
	if err != nil {
		return nil, false, err
	}

	// Parse path chain (e.g., a.b.c)
	var pathChain []string
	if i < len(tokens) && tokens[i].Type == TokenDot {
		pathChain, i = p.parseIdentifierChain(tokens, i+1)
		if len(pathChain) == 0 {
			return nil, false, &JSONPathSyntaxError{
				Message:    "Expected field name",
				Position:   p.position(tokens, i),
				Expression: p.template,
				Context:    "after '$.'",
			}
		}
	}

	var node builder
	isWildcard := false
	if len(pathChain) == 0 {
		// No path found, it's just a filter without path
		if i >= len(tokens) || tokens[i].Type != TokenLBracket {
			return nil, false, &JSONPathSyntaxError{
				Message:    "Expected path or filter expression",
				Position:   p.position(tokens, i),
				Expression: p.template,
				Context:    "after '$'",
			}
		}
		node, i, err = p.parseFilter(tokens, parseContext{isWildcardContext: false}, i)
		if err != nil {
			return nil, false, err
		}
	} else {
		if i >= len(tokens) || tokens[i].Type != TokenLBracket {
			return nil, false, &JSONPathSyntaxError{
				Message:    "Expected filter expression '[?...]'",
				Position:   p.position(tokens, i),
				Expression: p.template,
				Context:    "after path",
			}
		}
		// Build parent chain and get collection name
		parent := p.buildObjectChain(spec.GlobalScope(), pathChain[:len(pathChain)-1])
		collectionName := pathChain[len(pathChain)-1]
		// Wildcard with filter
		node, i, err = p.parseNestedWildcard(tokens, i, parent, collectionName)
		if err != nil {
			return nil, false, err
		}
		isWildcard = true
	}

	if i < len(tokens) {
		return nil, false, &JSONPathSyntaxError{
			Message:    fmt.Sprintf("Unexpected token '%s'", tokens[i].Value),
			Position:   tokens[i].Position,
			Expression: p.template,
			Context:    "expected end of expression",
		}
	}
	return node, isWildcard, nil
}

// bindPlaceholder binds a placeholder to its actual value.
//
// A parameter that is not found is an error: it used to leave the marker in
// the tree, where it compared unequal to everything, and the match was a
// silent false.
func (p *NativeParametrizedSpecification) bindPlaceholder(info placeholderInfo, params parameters) (any, error) {
	if info.Positional {
		paramIdx, _ := strconv.Atoi(info.Name)
		if paramIdx < len(params.positional) {
			return requireParameterOfKind(info.FormatType, info.Name, params.positional[paramIdx])
		}
		return nil, &JSONPathSyntaxError{
			Message:    fmt.Sprintf("Missing positional parameter at index %d", paramIdx),
			Position:   -1,
			Expression: p.template,
			Context:    fmt.Sprintf("expected %d parameters", len(p.placeholderInfo)),
		}
	}
	if value, ok := params.named[info.Name]; ok {
		return requireParameterOfKind(info.FormatType, info.Name, value)
	}
	return nil, &JSONPathSyntaxError{
		Message:    fmt.Sprintf("Missing named parameter: %s", info.Name),
		Position:   -1,
		Expression: p.template,
	}
}

// Bind builds the specification the template is of these positional
// parameters: the tree to evaluate, to transform, to compile to SQL. It has
// values where the template has placeholders; what a parameter is may decide
// what the tree is - a nil makes a null test of an equality - so a query is
// compiled of a bound template, and not once for all.
func (p *NativeParametrizedSpecification) Bind(params ...any) (spec.Visitable, error) {
	return p.builder(parameters{positional: params})
}

// BindNamed builds the specification the template is of these named parameters.
func (p *NativeParametrizedSpecification) BindNamed(namedParams map[string]any) (spec.Visitable, error) {
	return p.builder(parameters{named: namedParams})
}

// Match checks if data matches the specification with given positional parameters.
func (p *NativeParametrizedSpecification) Match(data spec.Context, params ...any) (bool, error) {
	return p.matchInternal(data, parameters{positional: params})
}

// MatchNamed checks if data matches the specification with named parameters.
func (p *NativeParametrizedSpecification) MatchNamed(data spec.Context, namedParams map[string]any) (bool, error) {
	return p.matchInternal(data, parameters{named: namedParams})
}

// matchInternal is the internal implementation of Match and MatchNamed.
func (p *NativeParametrizedSpecification) matchInternal(data spec.Context, params parameters) (bool, error) {
	// Bind placeholder values to the parsed template
	boundAST, err := p.builder(params)
	if err != nil {
		return false, err
	}

	// Evaluate using EvaluateVisitor
	visitor := spec.NewEvaluateVisitor(data, p.registry)
	return visitor.Evaluate(boundAST)
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
