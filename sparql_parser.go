package rdflibgo

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// SPARQLQuery is the parsed representation of a SPARQL query.
// Ported from: rdflib.plugins.sparql.parserutils.CompValue
type SPARQLQuery struct {
	Type       string            // "SELECT", "ASK", "CONSTRUCT"
	Distinct   bool
	Variables  []string          // projection vars (nil = *)
	Where      SPARQLPattern
	OrderBy    []SPARQLOrderExpr
	Limit      int               // -1 = no limit
	Offset     int
	Prefixes   map[string]string // prefix → namespace
	Construct  []TripleTemplate  // CONSTRUCT template
	GroupBy    []SPARQLExpr
	Having     SPARQLExpr
}

// TripleTemplate is a triple pattern used in CONSTRUCT.
type TripleTemplate struct {
	Subject, Predicate, Object string // variable names or N3 terms
}

// SPARQLOrderExpr is an ORDER BY expression.
type SPARQLOrderExpr struct {
	Expr SPARQLExpr
	Desc bool
}

// SPARQLPattern represents a WHERE clause pattern.
type SPARQLPattern interface {
	patternType() string
}

// BGP is a Basic Graph Pattern.
type BGP struct {
	Triples []SPARQLTriple
}
func (b *BGP) patternType() string { return "BGP" }

// SPARQLTriple is a triple pattern with possible variables.
type SPARQLTriple struct {
	Subject, Predicate, Object string // "?var" or N3 term
}

// JoinPattern joins two patterns.
type JoinPattern struct {
	Left, Right SPARQLPattern
}
func (j *JoinPattern) patternType() string { return "Join" }

// OptionalPattern is a LEFT JOIN.
type OptionalPattern struct {
	Main, Optional SPARQLPattern
}
func (o *OptionalPattern) patternType() string { return "Optional" }

// UnionPattern is a UNION of two patterns.
type UnionPattern struct {
	Left, Right SPARQLPattern
}
func (u *UnionPattern) patternType() string { return "Union" }

// FilterPattern wraps a pattern with a FILTER expression.
type FilterPattern struct {
	Pattern SPARQLPattern
	Expr    SPARQLExpr
}
func (f *FilterPattern) patternType() string { return "Filter" }

// BindPattern introduces a new variable binding.
type BindPattern struct {
	Pattern SPARQLPattern
	Expr    SPARQLExpr
	Var     string
}
func (b *BindPattern) patternType() string { return "Bind" }

// ValuesPattern provides inline data.
type ValuesPattern struct {
	Vars   []string
	Values [][]Term
}
func (v *ValuesPattern) patternType() string { return "Values" }

// SPARQLExpr is a filter/bind expression.
type SPARQLExpr interface {
	exprType() string
}

type VarExpr struct{ Name string }
func (e *VarExpr) exprType() string { return "Var" }

type LiteralExpr struct{ Value Term }
func (e *LiteralExpr) exprType() string { return "Literal" }

type IRIExpr struct{ Value string }
func (e *IRIExpr) exprType() string { return "IRI" }

type BinaryExpr struct {
	Op    string // "=", "!=", "<", ">", "<=", ">=", "&&", "||", "+", "-", "*", "/"
	Left, Right SPARQLExpr
}
func (e *BinaryExpr) exprType() string { return "Binary" }

type UnaryExpr struct {
	Op   string // "!", "-"
	Arg  SPARQLExpr
}
func (e *UnaryExpr) exprType() string { return "Unary" }

type FuncExpr struct {
	Name string
	Args []SPARQLExpr
}
func (e *FuncExpr) exprType() string { return "Func" }

// --- SPARQL Parser ---

// ParseSPARQL parses a SPARQL query string.
// Ported from: rdflib.plugins.sparql.parser.parseQuery
func ParseSPARQL(input string) (*SPARQLQuery, error) {
	p := &sparqlParser{
		input:    input,
		pos:      0,
		prefixes: make(map[string]string),
	}
	return p.parse()
}

type sparqlParser struct {
	input    string
	pos      int
	prefixes map[string]string
}

func (p *sparqlParser) parse() (*SPARQLQuery, error) {
	q := &SPARQLQuery{
		Limit:    -1,
		Prefixes: p.prefixes,
	}

	// Prologue: PREFIX and BASE
	for {
		p.skipWS()
		if p.matchKeywordCI("PREFIX") {
			p.pos += 6
			p.skipWS()
			prefix := p.readUntil(':')
			p.pos++ // skip ':'
			p.skipWS()
			iri := p.readIRIRef()
			p.prefixes[prefix] = iri
			continue
		}
		if p.matchKeywordCI("BASE") {
			p.pos += 4
			p.skipWS()
			p.readIRIRef() // stored but not used in simple impl
			continue
		}
		break
	}

	p.skipWS()

	// Query form
	if p.matchKeywordCI("SELECT") {
		p.pos += 6
		q.Type = "SELECT"
		if err := p.parseSelect(q); err != nil {
			return nil, err
		}
	} else if p.matchKeywordCI("ASK") {
		p.pos += 3
		q.Type = "ASK"
	} else if p.matchKeywordCI("CONSTRUCT") {
		p.pos += 9
		q.Type = "CONSTRUCT"
		if err := p.parseConstruct(q); err != nil {
			return nil, err
		}
	} else {
		return nil, p.errorf("expected SELECT, ASK, or CONSTRUCT")
	}

	// WHERE clause
	p.skipWS()
	if p.matchKeywordCI("WHERE") {
		p.pos += 5
	}
	p.skipWS()

	pattern, err := p.parseGroupGraphPattern()
	if err != nil {
		return nil, err
	}
	q.Where = pattern

	// Solution modifiers
	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			break
		}
		if p.matchKeywordCI("GROUP") {
			p.pos += 5
			p.skipWS()
			if p.matchKeywordCI("BY") {
				p.pos += 2
				p.skipWS()
				for {
					expr, err := p.parseExpr()
					if err != nil {
						break
					}
					q.GroupBy = append(q.GroupBy, expr)
					p.skipWS()
					if p.pos >= len(p.input) || p.isKeyword() {
						break
					}
				}
			}
			continue
		}
		if p.matchKeywordCI("HAVING") {
			p.pos += 6
			p.skipWS()
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			q.Having = expr
			continue
		}
		if p.matchKeywordCI("ORDER") {
			p.pos += 5
			p.skipWS()
			if p.matchKeywordCI("BY") {
				p.pos += 2
			}
			if err := p.parseOrderBy(q); err != nil {
				return nil, err
			}
			continue
		}
		if p.matchKeywordCI("LIMIT") {
			p.pos += 5
			p.skipWS()
			n, err := p.readInt()
			if err != nil {
				return nil, err
			}
			q.Limit = n
			continue
		}
		if p.matchKeywordCI("OFFSET") {
			p.pos += 6
			p.skipWS()
			n, err := p.readInt()
			if err != nil {
				return nil, err
			}
			q.Offset = n
			continue
		}
		break
	}

	return q, nil
}

func (p *sparqlParser) parseSelect(q *SPARQLQuery) error {
	p.skipWS()
	if p.matchKeywordCI("DISTINCT") {
		p.pos += 8
		q.Distinct = true
		p.skipWS()
	}
	if p.matchKeywordCI("REDUCED") {
		p.pos += 7
		p.skipWS()
	}

	// Variables or *
	if p.pos < len(p.input) && p.input[p.pos] == '*' {
		p.pos++
		q.Variables = nil // wildcard
	} else {
		for p.pos < len(p.input) {
			p.skipWS()
			if p.input[p.pos] == '?' || p.input[p.pos] == '$' {
				v := p.readVar()
				q.Variables = append(q.Variables, v)
			} else if p.input[p.pos] == '(' {
				// Expression AS ?var
				p.pos++
				p.skipWS()
				// skip expression for now — read until AS
				for p.pos < len(p.input) && !p.matchKeywordCI("AS") {
					p.pos++
				}
				if p.matchKeywordCI("AS") {
					p.pos += 2
					p.skipWS()
					v := p.readVar()
					q.Variables = append(q.Variables, v)
				}
				p.skipWS()
				if p.pos < len(p.input) && p.input[p.pos] == ')' {
					p.pos++
				}
			} else {
				break
			}
		}
	}
	return nil
}

func (p *sparqlParser) parseConstruct(q *SPARQLQuery) error {
	p.skipWS()
	if p.pos >= len(p.input) || p.input[p.pos] != '{' {
		return p.errorf("expected '{' in CONSTRUCT template")
	}
	p.pos++

	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			return p.errorf("unterminated CONSTRUCT template")
		}
		if p.input[p.pos] == '}' {
			p.pos++
			break
		}
		s := p.readTermOrVar()
		p.skipWS()
		pred := p.readTermOrVar()
		p.skipWS()
		obj := p.readTermOrVar()
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == '.' {
			p.pos++
		}
		q.Construct = append(q.Construct, TripleTemplate{Subject: s, Predicate: pred, Object: obj})
	}
	return nil
}

func (p *sparqlParser) parseGroupGraphPattern() (SPARQLPattern, error) {
	p.skipWS()
	if p.pos >= len(p.input) || p.input[p.pos] != '{' {
		return nil, p.errorf("expected '{'")
	}
	p.pos++

	var result SPARQLPattern
	var currentTriples []SPARQLTriple

	flushTriples := func() {
		if len(currentTriples) > 0 {
			bgp := &BGP{Triples: currentTriples}
			currentTriples = nil
			if result == nil {
				result = bgp
			} else {
				result = &JoinPattern{Left: result, Right: bgp}
			}
		}
	}

	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			return nil, p.errorf("unterminated group graph pattern")
		}
		if p.input[p.pos] == '}' {
			p.pos++
			flushTriples()
			if result == nil {
				result = &BGP{}
			}
			return result, nil
		}

		// OPTIONAL
		if p.matchKeywordCI("OPTIONAL") {
			p.pos += 8
			flushTriples()
			opt, err := p.parseGroupGraphPattern()
			if err != nil {
				return nil, err
			}
			if result == nil {
				result = &BGP{}
			}
			result = &OptionalPattern{Main: result, Optional: opt}
			continue
		}

		// UNION — look ahead after block
		if p.input[p.pos] == '{' {
			flushTriples()
			left, err := p.parseGroupGraphPattern()
			if err != nil {
				return nil, err
			}
			p.skipWS()
			if p.matchKeywordCI("UNION") {
				p.pos += 5
				p.skipWS()
				right, err := p.parseGroupGraphPattern()
				if err != nil {
					return nil, err
				}
				union := &UnionPattern{Left: left, Right: right}
				if result == nil {
					result = union
				} else {
					result = &JoinPattern{Left: result, Right: union}
				}
			} else {
				if result == nil {
					result = left
				} else {
					result = &JoinPattern{Left: result, Right: left}
				}
			}
			continue
		}

		// FILTER
		if p.matchKeywordCI("FILTER") {
			p.pos += 6
			flushTriples()
			p.skipWS()
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if result == nil {
				result = &BGP{}
			}
			result = &FilterPattern{Pattern: result, Expr: expr}
			continue
		}

		// BIND
		if p.matchKeywordCI("BIND") {
			p.pos += 4
			flushTriples()
			p.skipWS()
			if !p.expect('(') {
				return nil, p.errorf("expected '(' after BIND")
			}
			p.skipWS()
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			p.skipWS()
			if !p.matchKeywordCI("AS") {
				return nil, p.errorf("expected AS in BIND")
			}
			p.pos += 2
			p.skipWS()
			v := p.readVar()
			p.skipWS()
			p.expect(')')
			if result == nil {
				result = &BGP{}
			}
			result = &BindPattern{Pattern: result, Expr: expr, Var: v}
			continue
		}

		// VALUES
		if p.matchKeywordCI("VALUES") {
			p.pos += 6
			flushTriples()
			vp, err := p.parseValues()
			if err != nil {
				return nil, err
			}
			if result == nil {
				result = vp
			} else {
				result = &JoinPattern{Left: result, Right: vp}
			}
			continue
		}

		// Triple pattern
		triples, err := p.parseTriplePatterns()
		if err != nil {
			return nil, err
		}
		currentTriples = append(currentTriples, triples...)
	}
}

func (p *sparqlParser) parseTriplePatterns() ([]SPARQLTriple, error) {
	var triples []SPARQLTriple
	subj := p.readTermOrVar()
	p.skipWS()

	for {
		pred := p.readTermOrVar()
		if pred == "" {
			break
		}
		p.skipWS()

		// Object list
		for {
			obj := p.readTermOrVar()
			if obj == "" {
				return nil, p.errorf("expected object")
			}
			triples = append(triples, SPARQLTriple{Subject: subj, Predicate: pred, Object: obj})
			p.skipWS()
			if p.pos < len(p.input) && p.input[p.pos] == ',' {
				p.pos++
				p.skipWS()
				continue
			}
			break
		}

		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == ';' {
			p.pos++
			p.skipWS()
			if p.pos < len(p.input) && (p.input[p.pos] == '.' || p.input[p.pos] == '}') {
				break
			}
			continue
		}
		break
	}

	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		p.pos++
	}
	return triples, nil
}

func (p *sparqlParser) parseValues() (*ValuesPattern, error) {
	p.skipWS()
	var vars []string

	// Single var or ( var1 var2 )
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		p.pos++
		for {
			p.skipWS()
			if p.pos < len(p.input) && p.input[p.pos] == ')' {
				p.pos++
				break
			}
			vars = append(vars, p.readVar())
		}
	} else {
		vars = append(vars, p.readVar())
	}

	p.skipWS()
	if !p.expect('{') {
		return nil, p.errorf("expected '{' in VALUES")
	}

	var values [][]Term
	for {
		p.skipWS()
		if p.pos >= len(p.input) || p.input[p.pos] == '}' {
			p.pos++
			break
		}
		if p.input[p.pos] == '(' {
			p.pos++
			var row []Term
			for {
				p.skipWS()
				if p.input[p.pos] == ')' {
					p.pos++
					break
				}
				row = append(row, p.readTermValue())
			}
			values = append(values, row)
		} else {
			values = append(values, []Term{p.readTermValue()})
		}
	}

	return &ValuesPattern{Vars: vars, Values: values}, nil
}

func (p *sparqlParser) parseOrderBy(q *SPARQLQuery) error {
	for {
		p.skipWS()
		if p.pos >= len(p.input) || p.isKeyword() {
			break
		}

		desc := false
		if p.matchKeywordCI("DESC") {
			p.pos += 4
			desc = true
			p.skipWS()
		} else if p.matchKeywordCI("ASC") {
			p.pos += 3
			p.skipWS()
		}

		expr, err := p.parseExpr()
		if err != nil {
			break
		}
		q.OrderBy = append(q.OrderBy, SPARQLOrderExpr{Expr: expr, Desc: desc})
	}
	return nil
}

// parseExpr parses a SPARQL expression (simplified: handles comparisons, booleans, function calls).
func (p *sparqlParser) parseExpr() (SPARQLExpr, error) {
	p.skipWS()

	// Handle parenthesized expressions
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		p.pos++
		expr, err := p.parseOrExpr()
		if err != nil {
			return nil, err
		}
		p.skipWS()
		p.expect(')')
		return expr, nil
	}

	return p.parseOrExpr()
}

func (p *sparqlParser) parseOrExpr() (SPARQLExpr, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWS()
		if p.pos+1 < len(p.input) && p.input[p.pos:p.pos+2] == "||" {
			p.pos += 2
			right, err := p.parseAndExpr()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: "||", Left: left, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *sparqlParser) parseAndExpr() (SPARQLExpr, error) {
	left, err := p.parseCompareExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWS()
		if p.pos+1 < len(p.input) && p.input[p.pos:p.pos+2] == "&&" {
			p.pos += 2
			right, err := p.parseCompareExpr()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: "&&", Left: left, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *sparqlParser) parseCompareExpr() (SPARQLExpr, error) {
	left, err := p.parseAddExpr()
	if err != nil {
		return nil, err
	}
	p.skipWS()
	for _, op := range []string{"!=", "<=", ">=", "=", "<", ">"} {
		if p.startsWith(op) {
			p.pos += len(op)
			right, err := p.parseAddExpr()
			if err != nil {
				return nil, err
			}
			return &BinaryExpr{Op: op, Left: left, Right: right}, nil
		}
	}
	return left, nil
}

func (p *sparqlParser) parseAddExpr() (SPARQLExpr, error) {
	left, err := p.parseMulExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWS()
		if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
			op := string(p.input[p.pos])
			p.pos++
			right, err := p.parseMulExpr()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: op, Left: left, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *sparqlParser) parseMulExpr() (SPARQLExpr, error) {
	left, err := p.parseUnaryExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWS()
		if p.pos < len(p.input) && (p.input[p.pos] == '*' || p.input[p.pos] == '/') {
			op := string(p.input[p.pos])
			p.pos++
			right, err := p.parseUnaryExpr()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: op, Left: left, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *sparqlParser) parseUnaryExpr() (SPARQLExpr, error) {
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '!' {
		p.pos++
		arg, err := p.parsePrimaryExpr()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "!", Arg: arg}, nil
	}
	if p.pos < len(p.input) && p.input[p.pos] == '-' {
		p.pos++
		arg, err := p.parsePrimaryExpr()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "-", Arg: arg}, nil
	}
	return p.parsePrimaryExpr()
}

func (p *sparqlParser) parsePrimaryExpr() (SPARQLExpr, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of expression")
	}

	ch := p.input[p.pos]

	// Parenthesized
	if ch == '(' {
		p.pos++
		expr, err := p.parseOrExpr()
		if err != nil {
			return nil, err
		}
		p.skipWS()
		p.expect(')')
		return expr, nil
	}

	// Variable
	if ch == '?' || ch == '$' {
		v := p.readVar()
		return &VarExpr{Name: v}, nil
	}

	// String literal
	if ch == '"' || ch == '\'' {
		t := p.readTermOrVar()
		return &LiteralExpr{Value: p.resolveTermValue(t)}, nil
	}

	// Numeric
	if ch >= '0' && ch <= '9' || ch == '.' {
		t := p.readTermOrVar()
		return &LiteralExpr{Value: p.resolveTermValue(t)}, nil
	}

	// IRI
	if ch == '<' {
		iri := p.readIRIRef()
		return &IRIExpr{Value: iri}, nil
	}

	// Boolean or function call
	if p.matchKeywordCI("true") && (p.pos+4 >= len(p.input) || !isNameChar(rune(p.input[p.pos+4]))) {
		p.pos += 4
		return &LiteralExpr{Value: NewLiteral(true)}, nil
	}
	if p.matchKeywordCI("false") && (p.pos+5 >= len(p.input) || !isNameChar(rune(p.input[p.pos+5]))) {
		p.pos += 5
		return &LiteralExpr{Value: NewLiteral(false)}, nil
	}

	// Built-in function or prefixed name
	name := p.readFuncName()
	if name == "" {
		return nil, p.errorf("unexpected character in expression: %c", ch)
	}

	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		p.pos++
		var args []SPARQLExpr
		for {
			p.skipWS()
			if p.pos < len(p.input) && p.input[p.pos] == ')' {
				p.pos++
				break
			}
			if len(args) > 0 {
				if !p.expect(',') {
					p.skipWS()
				}
			}
			arg, err := p.parseOrExpr()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
		return &FuncExpr{Name: strings.ToUpper(name), Args: args}, nil
	}

	// It's a prefixed name used as a value
	resolved := p.resolveTermValue(name)
	if u, ok := resolved.(URIRef); ok {
		return &IRIExpr{Value: u.Value()}, nil
	}
	return &LiteralExpr{Value: resolved}, nil
}

// --- Helper methods ---

func (p *sparqlParser) readVar() string {
	if p.pos >= len(p.input) {
		return ""
	}
	p.pos++ // skip ? or $
	start := p.pos
	for p.pos < len(p.input) && isNameChar(rune(p.input[p.pos])) {
		p.pos++
	}
	return p.input[start:p.pos]
}

func (p *sparqlParser) readTermOrVar() string {
	p.skipWS()
	if p.pos >= len(p.input) {
		return ""
	}
	ch := p.input[p.pos]

	if ch == '?' || ch == '$' {
		return "?" + p.readVar()[0:] // readVar already skipped ? so re-read
	}
	// Re-do: readVar skips the ? prefix
	if ch == '?' || ch == '$' {
		v := p.readVar()
		return "?" + v
	}

	if ch == '<' {
		return "<" + p.readIRIRef() + ">"
	}

	if ch == '"' || ch == '\'' {
		return p.readStringLiteral()
	}

	// 'a' as rdf:type shorthand
	if ch == 'a' && (p.pos+1 >= len(p.input) || !isNameChar(rune(p.input[p.pos+1]))) {
		p.pos++
		return "<" + RDF.Type.Value() + ">"
	}

	// true/false
	if p.matchKeywordCI("true") && (p.pos+4 >= len(p.input) || !isNameChar(rune(p.input[p.pos+4]))) {
		p.pos += 4
		return "true"
	}
	if p.matchKeywordCI("false") && (p.pos+5 >= len(p.input) || !isNameChar(rune(p.input[p.pos+5]))) {
		p.pos += 5
		return "false"
	}

	// Numeric
	if (ch >= '0' && ch <= '9') || ch == '+' || ch == '-' {
		start := p.pos
		if ch == '+' || ch == '-' {
			p.pos++
		}
		for p.pos < len(p.input) && (p.input[p.pos] >= '0' && p.input[p.pos] <= '9') {
			p.pos++
		}
		if p.pos < len(p.input) && p.input[p.pos] == '.' {
			p.pos++
			for p.pos < len(p.input) && (p.input[p.pos] >= '0' && p.input[p.pos] <= '9') {
				p.pos++
			}
		}
		if p.pos < len(p.input) && (p.input[p.pos] == 'e' || p.input[p.pos] == 'E') {
			p.pos++
			if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
				p.pos++
			}
			for p.pos < len(p.input) && (p.input[p.pos] >= '0' && p.input[p.pos] <= '9') {
				p.pos++
			}
		}
		return p.input[start:p.pos]
	}

	// Prefixed name
	start := p.pos
	for p.pos < len(p.input) && isNameChar(rune(p.input[p.pos])) {
		p.pos++
	}
	if p.pos < len(p.input) && p.input[p.pos] == ':' {
		p.pos++
		for p.pos < len(p.input) && (isNameChar(rune(p.input[p.pos])) || p.input[p.pos] == '.' || p.input[p.pos] == '-') {
			p.pos++
		}
		// Trim trailing dot
		for p.pos > start && p.input[p.pos-1] == '.' {
			p.pos--
		}
	}
	return p.input[start:p.pos]
}

func (p *sparqlParser) readStringLiteral() string {
	start := p.pos
	quote := p.input[p.pos]
	p.pos++

	// Triple-quoted?
	long := false
	if p.pos+1 < len(p.input) && p.input[p.pos] == quote && p.input[p.pos+1] == quote {
		p.pos += 2
		long = true
	}

	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '\\' {
			p.pos += 2
			continue
		}
		if long {
			if ch == quote && p.pos+2 < len(p.input) && p.input[p.pos+1] == quote && p.input[p.pos+2] == quote {
				p.pos += 3
				goto afterString
			}
		} else if ch == quote {
			p.pos++
			goto afterString
		}
		p.pos++
	}
afterString:
	// Check for @lang or ^^type
	if p.pos < len(p.input) && p.input[p.pos] == '@' {
		p.pos++
		for p.pos < len(p.input) && (p.input[p.pos] >= 'a' && p.input[p.pos] <= 'z' || p.input[p.pos] >= 'A' && p.input[p.pos] <= 'Z' || p.input[p.pos] == '-') {
			p.pos++
		}
	} else if p.pos+1 < len(p.input) && p.input[p.pos] == '^' && p.input[p.pos+1] == '^' {
		p.pos += 2
		if p.pos < len(p.input) && p.input[p.pos] == '<' {
			p.pos++
			for p.pos < len(p.input) && p.input[p.pos] != '>' {
				p.pos++
			}
			if p.pos < len(p.input) {
				p.pos++
			}
		} else {
			for p.pos < len(p.input) && (isNameChar(rune(p.input[p.pos])) || p.input[p.pos] == ':') {
				p.pos++
			}
		}
	}
	return p.input[start:p.pos]
}

func (p *sparqlParser) readIRIRef() string {
	if p.pos >= len(p.input) || p.input[p.pos] != '<' {
		return ""
	}
	p.pos++
	start := p.pos
	for p.pos < len(p.input) && p.input[p.pos] != '>' {
		p.pos++
	}
	iri := p.input[start:p.pos]
	if p.pos < len(p.input) {
		p.pos++ // skip '>'
	}
	return iri
}

func (p *sparqlParser) readFuncName() string {
	start := p.pos
	for p.pos < len(p.input) && (isNameChar(rune(p.input[p.pos])) || p.input[p.pos] == ':') {
		p.pos++
	}
	return p.input[start:p.pos]
}

func (p *sparqlParser) readUntil(ch byte) string {
	start := p.pos
	for p.pos < len(p.input) && p.input[p.pos] != ch {
		p.pos++
	}
	return strings.TrimSpace(p.input[start:p.pos])
}

func (p *sparqlParser) readInt() (int, error) {
	start := p.pos
	for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
		p.pos++
	}
	return strconv.Atoi(p.input[start:p.pos])
}

func (p *sparqlParser) readTermValue() Term {
	tv := p.readTermOrVar()
	return p.resolveTermValue(tv)
}

func (p *sparqlParser) resolveTermValue(s string) Term {
	if s == "" {
		return NewLiteral("")
	}
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		return NewURIRefUnsafe(s[1 : len(s)-1])
	}
	if strings.HasPrefix(s, "\"") || strings.HasPrefix(s, "'") {
		return parseLiteralString(s)
	}
	if s == "true" {
		return NewLiteral(true)
	}
	if s == "false" {
		return NewLiteral(false)
	}
	// Numeric
	if len(s) > 0 && (s[0] >= '0' && s[0] <= '9' || s[0] == '+' || s[0] == '-') {
		if strings.ContainsAny(s, "eE") {
			return NewLiteral(s, WithDatatype(XSDDouble))
		}
		if strings.Contains(s, ".") {
			return NewLiteral(s, WithDatatype(XSDDecimal))
		}
		return NewLiteral(s, WithDatatype(XSDInteger))
	}
	// Prefixed name
	if idx := strings.Index(s, ":"); idx >= 0 {
		prefix := s[:idx]
		local := s[idx+1:]
		if ns, ok := p.prefixes[prefix]; ok {
			return NewURIRefUnsafe(ns + local)
		}
	}
	return NewLiteral(s)
}

func parseLiteralString(s string) Literal {
	// Simplified literal parsing from N3 form
	quote := s[0]
	long := len(s) >= 6 && s[1] == quote && s[2] == quote

	var lexEnd int
	if long {
		q3 := string([]byte{quote, quote, quote})
		lexEnd = strings.Index(s[3:], q3)
		if lexEnd < 0 {
			return NewLiteral(s)
		}
		lexEnd += 3
	} else {
		lexEnd = strings.Index(s[1:], string(quote))
		if lexEnd < 0 {
			return NewLiteral(s)
		}
		lexEnd += 1
	}

	var lexical string
	if long {
		lexical = s[3:lexEnd]
	} else {
		lexical = s[1:lexEnd]
	}
	lexical = unescapeSPARQLString(lexical)

	rest := s[lexEnd+1:]
	if long {
		rest = s[lexEnd+3:]
	}

	var opts []LiteralOption
	if strings.HasPrefix(rest, "@") {
		opts = append(opts, WithLang(rest[1:]))
	} else if strings.HasPrefix(rest, "^^") {
		dt := rest[2:]
		if strings.HasPrefix(dt, "<") && strings.HasSuffix(dt, ">") {
			opts = append(opts, WithDatatype(NewURIRefUnsafe(dt[1:len(dt)-1])))
		}
	}
	return NewLiteral(lexical, opts...)
}

func unescapeSPARQLString(s string) string {
	r := strings.NewReplacer(`\"`, `"`, `\\`, `\`, `\n`, "\n", `\r`, "\r", `\t`, "\t")
	return r.Replace(s)
}

func (p *sparqlParser) skipWS() {
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			p.pos++
		} else if ch == '#' {
			for p.pos < len(p.input) && p.input[p.pos] != '\n' {
				p.pos++
			}
		} else {
			break
		}
	}
}

func (p *sparqlParser) expect(ch byte) bool {
	if p.pos < len(p.input) && p.input[p.pos] == ch {
		p.pos++
		return true
	}
	return false
}

func (p *sparqlParser) startsWith(s string) bool {
	return strings.HasPrefix(p.input[p.pos:], s)
}

func (p *sparqlParser) matchKeywordCI(kw string) bool {
	if p.pos+len(kw) > len(p.input) {
		return false
	}
	if !strings.EqualFold(p.input[p.pos:p.pos+len(kw)], kw) {
		return false
	}
	after := p.pos + len(kw)
	if after < len(p.input) && isNameChar(rune(p.input[after])) {
		return false
	}
	return true
}

func (p *sparqlParser) isKeyword() bool {
	for _, kw := range []string{"ORDER", "LIMIT", "OFFSET", "GROUP", "HAVING", "VALUES"} {
		if p.matchKeywordCI(kw) {
			return true
		}
	}
	return false
}

func (p *sparqlParser) errorf(format string, args ...any) error {
	return fmt.Errorf("sparql parse error at pos %d: %s", p.pos, fmt.Sprintf(format, args...))
}

func isNameChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
