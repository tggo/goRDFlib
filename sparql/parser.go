package sparql

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/term"
)

// Parse parses a SPARQL query string.
// Ported from: rdflib.plugins.sparql.parser.parseQuery
func Parse(input string) (*ParsedQuery, error) {
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

func (p *sparqlParser) parse() (*ParsedQuery, error) {
	q := &ParsedQuery{
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
		p.skipWS()
		// CONSTRUCT WHERE { ... } shorthand: no explicit template
		if !p.matchKeywordCI("WHERE") {
			if err := p.parseConstruct(q); err != nil {
				return nil, err
			}
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
	if err := p.parseSolutionModifiers(q); err != nil {
		return nil, err
	}

	// Post-query VALUES clause
	p.skipWS()
	if p.matchKeywordCI("VALUES") {
		p.pos += 6
		vp, err := p.parseValues()
		if err != nil {
			return nil, err
		}
		// Join with existing WHERE pattern
		if q.Where == nil {
			q.Where = vp
		} else {
			q.Where = &JoinPattern{Left: q.Where, Right: vp}
		}
	}

	return q, nil
}

func (p *sparqlParser) parseSolutionModifiers(q *ParsedQuery) error {
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
					p.skipWS()
					if p.pos >= len(p.input) || p.isKeyword() {
						break
					}
					// Check for (expr AS ?var) in GROUP BY
					if p.pos < len(p.input) && p.input[p.pos] == '(' {
						saved := p.pos
						p.pos++
						p.skipWS()
						expr, err := p.parseOrExpr()
						if err != nil {
							p.pos = saved
							break
						}
						p.skipWS()
						if p.matchKeywordCI("AS") {
							p.pos += 2
							p.skipWS()
							v := p.readVar()
							q.GroupBy = append(q.GroupBy, expr)
							q.GroupByAliases = append(q.GroupByAliases, v)
							q.Variables = append(q.Variables, v)
						} else {
							q.GroupBy = append(q.GroupBy, expr)
							q.GroupByAliases = append(q.GroupByAliases, "")
						}
						p.skipWS()
						p.expect(')')
						continue
					}
					expr, err := p.parseExpr()
					if err != nil {
						break
					}
					q.GroupBy = append(q.GroupBy, expr)
					q.GroupByAliases = append(q.GroupByAliases, "")
					p.skipWS()
				}
			}
			continue
		}
		if p.matchKeywordCI("HAVING") {
			p.pos += 6
			p.skipWS()
			// HAVING can have multiple constraint expressions (ANDed)
			var havingExprs []Expr
			for {
				p.skipWS()
				if p.pos >= len(p.input) || p.isKeyword() {
					break
				}
				expr, err := p.parseExpr()
				if err != nil {
					break
				}
				havingExprs = append(havingExprs, expr)
			}
			if len(havingExprs) == 1 {
				q.Having = havingExprs[0]
			} else if len(havingExprs) > 1 {
				combined := havingExprs[0]
				for _, e := range havingExprs[1:] {
					combined = &BinaryExpr{Op: "&&", Left: combined, Right: e}
				}
				q.Having = combined
			}
			continue
		}
		if p.matchKeywordCI("ORDER") {
			p.pos += 5
			p.skipWS()
			if p.matchKeywordCI("BY") {
				p.pos += 2
			}
			if err := p.parseOrderBy(q); err != nil {
				return err
			}
			continue
		}
		if p.matchKeywordCI("LIMIT") {
			p.pos += 5
			p.skipWS()
			n, err := p.readInt()
			if err != nil {
				return err
			}
			q.Limit = n
			continue
		}
		if p.matchKeywordCI("OFFSET") {
			p.pos += 6
			p.skipWS()
			n, err := p.readInt()
			if err != nil {
				return err
			}
			q.Offset = n
			continue
		}
		break
	}
	return nil
}

func (p *sparqlParser) parseSelect(q *ParsedQuery) error {
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
			if p.pos >= len(p.input) {
				break
			}
			if p.input[p.pos] == '?' || p.input[p.pos] == '$' {
				v := p.readVar()
				q.Variables = append(q.Variables, v)
			} else if p.input[p.pos] == '(' {
				// (expression AS ?var)
				p.pos++ // skip '('
				p.skipWS()
				expr, err := p.parseOrExpr()
				if err != nil {
					return err
				}
				p.skipWS()
				if p.matchKeywordCI("AS") {
					p.pos += 2
					p.skipWS()
					v := p.readVar()
					q.Variables = append(q.Variables, v)
					q.ProjectExprs = append(q.ProjectExprs, ProjectExpr{Expr: expr, Var: v})
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

func (p *sparqlParser) parseConstruct(q *ParsedQuery) error {
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
		// Skip list syntax (not supported)
		if p.input[p.pos] == '(' {
			return p.errorf("list syntax in CONSTRUCT template not supported")
		}
		s := p.readTermOrVar()
		if s == "" {
			return p.errorf("unexpected token in CONSTRUCT template")
		}
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

func (p *sparqlParser) parseGroupGraphPattern() (Pattern, error) {
	p.skipWS()
	if p.pos >= len(p.input) || p.input[p.pos] != '{' {
		return nil, p.errorf("expected '{'")
	}
	p.pos++

	var result Pattern
	var currentTriples []Triple

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

		// Subquery: SELECT inside group graph pattern
		if p.matchKeywordCI("SELECT") {
			flushTriples()
			subQ, err := p.parseSubQuery()
			if err != nil {
				return nil, err
			}
			pat := &SubqueryPattern{Query: subQ}
			if result == nil {
				result = pat
			} else {
				result = &JoinPattern{Left: result, Right: pat}
			}
			continue
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

		// MINUS
		if p.matchKeywordCI("MINUS") {
			p.pos += 5
			flushTriples()
			right, err := p.parseGroupGraphPattern()
			if err != nil {
				return nil, err
			}
			if result == nil {
				result = &BGP{}
			}
			result = &MinusPattern{Left: result, Right: right}
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

		// GRAPH (not fully supported - skip the block)
		if p.matchKeywordCI("GRAPH") {
			p.pos += 5
			flushTriples()
			p.skipWS()
			p.readTermOrVar() // skip graph name
			p.skipWS()
			sub, err := p.parseGroupGraphPattern()
			if err != nil {
				return nil, err
			}
			if result == nil {
				result = sub
			} else {
				result = &JoinPattern{Left: result, Right: sub}
			}
			continue
		}

		// SERVICE (not supported - skip)
		if p.matchKeywordCI("SERVICE") {
			return nil, p.errorf("SERVICE not supported")
		}

		// Triple pattern (may include property paths in predicate)
		triples, err := p.parseTriplePatterns()
		if err != nil {
			return nil, err
		}
		if len(triples) == 0 {
			return nil, p.errorf("unexpected token in group graph pattern")
		}
		currentTriples = append(currentTriples, triples...)
	}
}

func (p *sparqlParser) parseSubQuery() (*ParsedQuery, error) {
	q := &ParsedQuery{
		Limit:    -1,
		Prefixes: p.prefixes,
	}
	p.pos += 6 // skip "SELECT"
	q.Type = "SELECT"
	if err := p.parseSelect(q); err != nil {
		return nil, err
	}
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
	if err := p.parseSolutionModifiers(q); err != nil {
		return nil, err
	}
	return q, nil
}

func (p *sparqlParser) parseTriplePatterns() ([]Triple, error) {
	var triples []Triple
	subj := p.readTermOrVar()
	p.skipWS()

	for {
		// Try to parse predicate as property path
		pred, predPath, err := p.parsePredicateOrPath()
		if err != nil {
			return nil, err
		}
		if pred == "" && predPath == nil {
			break
		}
		p.skipWS()

		// Object list
		for {
			obj := p.readTermOrVar()
			if obj == "" {
				return nil, p.errorf("expected object")
			}
			t := Triple{Subject: subj, Predicate: pred, Object: obj}
			if predPath != nil {
				t.PredicatePath = predPath
			}
			triples = append(triples, t)
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

// parsePredicateOrPath tries to parse a predicate which may be a property path.
// Returns (predString, path, error). If it's a simple URI/var, path is nil.
func (p *sparqlParser) parsePredicateOrPath() (string, paths.Path, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return "", nil, nil
	}

	ch := p.input[p.pos]

	// Can't start a predicate with these chars
	if ch == '}' || ch == '.' || ch == '{' || ch == ';' || ch == ',' {
		return "", nil, nil
	}

	// These chars always indicate a property path
	if ch == '^' || ch == '(' {
		path, err := p.parsePathExpr()
		if err != nil {
			return "", nil, err
		}
		return "", path, nil
	}

	// ! could be negated path if followed by a URI, (, or ^
	if ch == '!' {
		savedPos := p.pos
		p.pos++
		p.skipWS()
		next := byte(0)
		if p.pos < len(p.input) {
			next = p.input[p.pos]
		}
		p.pos = savedPos
		if next == '<' || next == '(' || next == '^' || (next != 0 && isNameChar(rune(next))) {
			path, err := p.parsePathExpr()
			if err != nil {
				return "", nil, err
			}
			return "", path, nil
		}
	}

	savedPos := p.pos

	// Check for 'a' shorthand
	if ch == 'a' && (p.pos+1 >= len(p.input) || !isNameChar(rune(p.input[p.pos+1]))) {
		p.pos++
		pred := "<" + rdflibgo.RDF.Type.Value() + ">"
		p.skipWS()
		if p.pos < len(p.input) && isPathModifier(p.input[p.pos]) {
			p.pos = savedPos
			path, err := p.parsePathExpr()
			if err != nil {
				return "", nil, err
			}
			return "", path, nil
		}
		return pred, nil, nil
	}

	// Try reading a simple term/var
	pred := p.readTermOrVar()
	if pred == "" {
		return "", nil, nil
	}

	// Check if followed by path operators (/, |, *, +, ?)
	p.skipWS()
	if p.pos < len(p.input) && isPathModifier(p.input[p.pos]) {
		p.pos = savedPos
		path, err := p.parsePathExpr()
		if err != nil {
			return "", nil, err
		}
		return "", path, nil
	}

	return pred, nil, nil
}

func isPathModifier(ch byte) bool {
	return ch == '/' || ch == '|' || ch == '*' || ch == '+'
	// Note: '?' is intentionally excluded because it's ambiguous with variable names.
	// ex:p? is handled within parsePathEltOrInverse after already being in path context.
}

// --- Property path parsing ---

func (p *sparqlParser) parsePathExpr() (paths.Path, error) {
	return p.parsePathAlternative()
}

func (p *sparqlParser) parsePathAlternative() (paths.Path, error) {
	left, err := p.parsePathSequence()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == '|' {
			p.pos++
			right, err := p.parsePathSequence()
			if err != nil {
				return nil, err
			}
			left = paths.Alternative(left, right)
		} else {
			break
		}
	}
	return left, nil
}

func (p *sparqlParser) parsePathSequence() (paths.Path, error) {
	left, err := p.parsePathEltOrInverse()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == '/' {
			p.pos++
			right, err := p.parsePathEltOrInverse()
			if err != nil {
				return nil, err
			}
			left = paths.Sequence(left, right)
		} else {
			break
		}
	}
	return left, nil
}

func (p *sparqlParser) parsePathEltOrInverse() (paths.Path, error) {
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '^' {
		p.pos++
		inner, err := p.parsePathPrimary()
		if err != nil {
			return nil, err
		}
		return paths.Inv(inner), nil
	}
	elt, err := p.parsePathPrimary()
	if err != nil {
		return nil, err
	}
	// Check for modifier: *, +, ?
	// Note: ? is only a path modifier if NOT followed by a name char (otherwise it's a variable)
	if p.pos < len(p.input) {
		switch p.input[p.pos] {
		case '*':
			p.pos++
			return paths.ZeroOrMore(elt), nil
		case '+':
			p.pos++
			return paths.OneOrMore(elt), nil
		case '?':
			if p.pos+1 >= len(p.input) || !isNameChar(rune(p.input[p.pos+1])) {
				p.pos++
				return paths.ZeroOrOne(elt), nil
			}
		}
	}
	return elt, nil
}

func (p *sparqlParser) parsePathPrimary() (paths.Path, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("expected path element")
	}

	ch := p.input[p.pos]

	// Parenthesized path
	if ch == '(' {
		p.pos++
		inner, err := p.parsePathExpr()
		if err != nil {
			return nil, err
		}
		p.skipWS()
		p.expect(')')
		return inner, nil
	}

	// Negated property set: !uri or !(uri1|uri2|^uri3)
	if ch == '!' {
		p.pos++
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == '(' {
			p.pos++
			var excluded []term.URIRef
			for {
				p.skipWS()
				if p.pos < len(p.input) && p.input[p.pos] == ')' {
					p.pos++
					break
				}
				if len(excluded) > 0 {
					p.skipWS()
					if p.pos < len(p.input) && p.input[p.pos] == '|' {
						p.pos++
					}
				}
				p.skipWS()
				// skip ^ for inverse in negated set (we just collect the URIs)
				if p.pos < len(p.input) && p.input[p.pos] == '^' {
					p.pos++
				}
				uri := p.resolvePathURI()
				if uri != "" {
					excluded = append(excluded, term.NewURIRefUnsafe(uri))
				}
			}
			return paths.Negated(excluded...), nil
		}
		// Single negated URI
		// skip ^ for inverse
		if p.pos < len(p.input) && p.input[p.pos] == '^' {
			p.pos++
		}
		uri := p.resolvePathURI()
		if uri != "" {
			return paths.Negated(term.NewURIRefUnsafe(uri)), nil
		}
		return nil, p.errorf("expected URI in negated path")
	}

	// 'a' shorthand
	if ch == 'a' && (p.pos+1 >= len(p.input) || !isNameChar(rune(p.input[p.pos+1]))) {
		p.pos++
		return paths.AsPath(term.NewURIRefUnsafe(rdflibgo.RDF.Type.Value())), nil
	}

	// IRI or prefixed name
	uri := p.resolvePathURI()
	if uri != "" {
		return paths.AsPath(term.NewURIRefUnsafe(uri)), nil
	}

	return nil, p.errorf("expected path element, got %c", ch)
}

func (p *sparqlParser) resolvePathURI() string {
	p.skipWS()
	if p.pos >= len(p.input) {
		return ""
	}
	// 'a' shorthand for rdf:type
	if p.input[p.pos] == 'a' && (p.pos+1 >= len(p.input) || !isNameChar(rune(p.input[p.pos+1]))) {
		p.pos++
		return rdflibgo.RDF.Type.Value()
	}
	if p.input[p.pos] == '<' {
		return p.readIRIRef()
	}
	// Prefixed name
	start := p.pos
	for p.pos < len(p.input) && isNameChar(rune(p.input[p.pos])) {
		p.pos++
	}
	if p.pos < len(p.input) && p.input[p.pos] == ':' {
		prefix := p.input[start:p.pos]
		p.pos++
		localStart := p.pos
		for p.pos < len(p.input) && (isNameChar(rune(p.input[p.pos])) || p.input[p.pos] == '.' || p.input[p.pos] == '-') {
			p.pos++
		}
		for p.pos > localStart && p.input[p.pos-1] == '.' {
			p.pos--
		}
		local := p.input[localStart:p.pos]
		if ns, ok := p.prefixes[prefix]; ok {
			return ns + local
		}
	}
	p.pos = start
	return ""
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

	var values [][]rdflibgo.Term
	for {
		p.skipWS()
		if p.pos >= len(p.input) || p.input[p.pos] == '}' {
			p.pos++
			break
		}
		if p.input[p.pos] == '(' {
			p.pos++
			var row []rdflibgo.Term
			for {
				p.skipWS()
				if p.pos >= len(p.input) || p.input[p.pos] == ')' {
					p.pos++
					break
				}
				if p.matchKeywordCI("UNDEF") {
					p.pos += 5
					row = append(row, nil)
				} else {
					row = append(row, p.readTermValue())
				}
			}
			values = append(values, row)
		} else {
			if p.matchKeywordCI("UNDEF") {
				p.pos += 5
				values = append(values, []rdflibgo.Term{nil})
			} else {
				values = append(values, []rdflibgo.Term{p.readTermValue()})
			}
		}
	}

	return &ValuesPattern{Vars: vars, Values: values}, nil
}

func (p *sparqlParser) parseOrderBy(q *ParsedQuery) error {
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
		q.OrderBy = append(q.OrderBy, OrderExpr{Expr: expr, Desc: desc})
	}
	return nil
}

// parseExpr parses a SPARQL expression (simplified: handles comparisons, booleans, function calls).
func (p *sparqlParser) parseExpr() (Expr, error) {
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

func (p *sparqlParser) parseOrExpr() (Expr, error) {
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

func (p *sparqlParser) parseAndExpr() (Expr, error) {
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

func (p *sparqlParser) parseCompareExpr() (Expr, error) {
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

	// IN / NOT IN
	if p.matchKeywordCI("IN") {
		p.pos += 2
		p.skipWS()
		list, err := p.parseExprList()
		if err != nil {
			return nil, err
		}
		// Convert to OR chain of equality
		if len(list) == 0 {
			return &LiteralExpr{Value: rdflibgo.NewLiteral(false)}, nil
		}
		var result Expr = &BinaryExpr{Op: "=", Left: left, Right: list[0]}
		for _, item := range list[1:] {
			result = &BinaryExpr{Op: "||", Left: result, Right: &BinaryExpr{Op: "=", Left: left, Right: item}}
		}
		return result, nil
	}
	if p.matchKeywordCI("NOT") {
		saved := p.pos
		p.pos += 3
		p.skipWS()
		if p.matchKeywordCI("IN") {
			p.pos += 2
			p.skipWS()
			list, err := p.parseExprList()
			if err != nil {
				return nil, err
			}
			if len(list) == 0 {
				return &LiteralExpr{Value: rdflibgo.NewLiteral(true)}, nil
			}
			var result Expr = &BinaryExpr{Op: "!=", Left: left, Right: list[0]}
			for _, item := range list[1:] {
				result = &BinaryExpr{Op: "&&", Left: result, Right: &BinaryExpr{Op: "!=", Left: left, Right: item}}
			}
			return result, nil
		}
		p.pos = saved
	}

	return left, nil
}

func (p *sparqlParser) parseExprList() ([]Expr, error) {
	if !p.expect('(') {
		return nil, p.errorf("expected '('")
	}
	var list []Expr
	for {
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == ')' {
			p.pos++
			return list, nil
		}
		if len(list) > 0 {
			p.expect(',')
			p.skipWS()
		}
		expr, err := p.parseOrExpr()
		if err != nil {
			return nil, err
		}
		list = append(list, expr)
	}
}

func (p *sparqlParser) parseAddExpr() (Expr, error) {
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

func (p *sparqlParser) parseMulExpr() (Expr, error) {
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

func (p *sparqlParser) parseUnaryExpr() (Expr, error) {
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '!' {
		// Check for NOT EXISTS
		saved := p.pos
		p.pos++
		p.skipWS()
		if p.matchKeywordCI("EXISTS") {
			p.pos = saved
			return p.parsePrimaryExpr()
		}
		p.pos = saved + 1
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

func (p *sparqlParser) parsePrimaryExpr() (Expr, error) {
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
		return &LiteralExpr{Value: rdflibgo.NewLiteral(true)}, nil
	}
	if p.matchKeywordCI("false") && (p.pos+5 >= len(p.input) || !isNameChar(rune(p.input[p.pos+5]))) {
		p.pos += 5
		return &LiteralExpr{Value: rdflibgo.NewLiteral(false)}, nil
	}

	// EXISTS / NOT EXISTS
	if p.matchKeywordCI("EXISTS") {
		p.pos += 6
		p.skipWS()
		pat, err := p.parseGroupGraphPattern()
		if err != nil {
			return nil, err
		}
		return &ExistsExpr{Pattern: pat, Not: false}, nil
	}
	if p.matchKeywordCI("NOT") {
		saved := p.pos
		p.pos += 3
		p.skipWS()
		if p.matchKeywordCI("EXISTS") {
			p.pos += 6
			p.skipWS()
			pat, err := p.parseGroupGraphPattern()
			if err != nil {
				return nil, err
			}
			return &ExistsExpr{Pattern: pat, Not: true}, nil
		}
		p.pos = saved
	}

	// Built-in function or prefixed name
	name := p.readFuncName()
	if name == "" {
		return nil, p.errorf("unexpected character in expression: %c", ch)
	}

	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		upperName := strings.ToUpper(name)
		// Resolve prefixed function names (e.g. xsd:integer -> full IRI)
		if idx := strings.Index(name, ":"); idx >= 0 {
			prefix := name[:idx]
			local := name[idx+1:]
			if ns, ok := p.prefixes[prefix]; ok {
				upperName = strings.ToUpper(ns + local)
			}
		}
		if isAggregateName(upperName) {
			return p.parseAggregateCall(upperName)
		}
		p.pos++
		var args []Expr
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
		return &FuncExpr{Name: upperName, Args: args}, nil
	}

	// It's a prefixed name used as a value
	resolved := p.resolveTermValue(name)
	if u, ok := resolved.(rdflibgo.URIRef); ok {
		return &IRIExpr{Value: u.Value()}, nil
	}
	return &LiteralExpr{Value: resolved}, nil
}

func isAggregateName(name string) bool {
	switch name {
	case "COUNT", "SUM", "AVG", "MIN", "MAX", "GROUP_CONCAT", "SAMPLE":
		return true
	}
	return false
}

func (p *sparqlParser) parseAggregateCall(name string) (Expr, error) {
	p.pos++ // skip '('
	p.skipWS()

	fe := &FuncExpr{Name: name}

	// COUNT(*)
	if name == "COUNT" && p.pos < len(p.input) && p.input[p.pos] == '*' {
		p.pos++
		fe.Star = true
		p.skipWS()
		p.expect(')')
		return fe, nil
	}

	// DISTINCT
	if p.matchKeywordCI("DISTINCT") {
		p.pos += 8
		p.skipWS()
		fe.Distinct = true
	}

	// COUNT(DISTINCT *)
	if name == "COUNT" && p.pos < len(p.input) && p.input[p.pos] == '*' {
		p.pos++
		fe.Star = true
		p.skipWS()
		p.expect(')')
		return fe, nil
	}

	// Parse argument
	if p.pos < len(p.input) && p.input[p.pos] != ')' {
		arg, err := p.parseOrExpr()
		if err != nil {
			return nil, err
		}
		fe.Args = append(fe.Args, arg)
	}

	// GROUP_CONCAT separator
	if name == "GROUP_CONCAT" {
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == ';' {
			p.pos++
			p.skipWS()
			if p.matchKeywordCI("SEPARATOR") {
				p.pos += 9
				p.skipWS()
				p.expect('=')
				p.skipWS()
				sep := p.readTermOrVar()
				fe.Separator = string(p.resolveTermValue(sep).(rdflibgo.Literal).Lexical())
			}
		} else {
			fe.Separator = " " // default separator
		}
	}

	p.skipWS()
	p.expect(')')
	return fe, nil
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
		v := p.readVar()
		return "?" + v
	}

	if ch == '<' {
		return "<" + p.readIRIRef() + ">"
	}

	if ch == '"' || ch == '\'' {
		return p.readStringLiteral()
	}

	// Blank node with property list: [ pred obj ; ... ]
	if ch == '[' {
		return p.readBlankNodePropertyList()
	}

	// 'a' as rdf:type shorthand
	if ch == 'a' && (p.pos+1 >= len(p.input) || !isNameChar(rune(p.input[p.pos+1]))) {
		p.pos++
		return "<" + rdflibgo.RDF.Type.Value() + ">"
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

// readBlankNodePropertyList handles [ pred obj ; ... ] syntax.
// Returns a blank node variable name and queues the triples.
func (p *sparqlParser) readBlankNodePropertyList() string {
	// For now, generate a unique blank node and skip the content
	p.pos++ // skip [
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == ']' {
		p.pos++
		return "?_bnode" // anonymous blank node
	}
	// Skip everything until matching ]
	depth := 1
	for p.pos < len(p.input) && depth > 0 {
		if p.input[p.pos] == '[' {
			depth++
		} else if p.input[p.pos] == ']' {
			depth--
		}
		p.pos++
	}
	return "?_bnode" // simplified - not fully correct
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

func (p *sparqlParser) readTermValue() rdflibgo.Term {
	tv := p.readTermOrVar()
	return p.resolveTermValue(tv)
}

func (p *sparqlParser) resolveTermValue(s string) rdflibgo.Term {
	if s == "" {
		return rdflibgo.NewLiteral("")
	}
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		return rdflibgo.NewURIRefUnsafe(s[1 : len(s)-1])
	}
	if strings.HasPrefix(s, "\"") || strings.HasPrefix(s, "'") {
		return parseLiteralString(s)
	}
	if s == "true" {
		return rdflibgo.NewLiteral(true)
	}
	if s == "false" {
		return rdflibgo.NewLiteral(false)
	}
	// Numeric
	if len(s) > 0 && (s[0] >= '0' && s[0] <= '9' || s[0] == '+' || s[0] == '-') {
		if strings.ContainsAny(s, "eE") {
			return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(rdflibgo.XSDDouble))
		}
		if strings.Contains(s, ".") {
			return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
		}
		return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
	}
	// Prefixed name
	if idx := strings.Index(s, ":"); idx >= 0 {
		prefix := s[:idx]
		local := s[idx+1:]
		if ns, ok := p.prefixes[prefix]; ok {
			return rdflibgo.NewURIRefUnsafe(ns + local)
		}
	}
	return rdflibgo.NewLiteral(s)
}

func parseLiteralString(s string) rdflibgo.Literal {
	// Simplified literal parsing from N3 form
	quote := s[0]
	long := len(s) >= 6 && s[1] == quote && s[2] == quote

	var lexEnd int
	if long {
		q3 := string([]byte{quote, quote, quote})
		lexEnd = strings.Index(s[3:], q3)
		if lexEnd < 0 {
			return rdflibgo.NewLiteral(s)
		}
		lexEnd += 3
	} else {
		lexEnd = strings.Index(s[1:], string(quote))
		if lexEnd < 0 {
			return rdflibgo.NewLiteral(s)
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

	var opts []rdflibgo.LiteralOption
	if strings.HasPrefix(rest, "@") {
		opts = append(opts, rdflibgo.WithLang(rest[1:]))
	} else if strings.HasPrefix(rest, "^^") {
		dt := rest[2:]
		if strings.HasPrefix(dt, "<") && strings.HasSuffix(dt, ">") {
			opts = append(opts, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(dt[1:len(dt)-1])))
		}
	}
	return rdflibgo.NewLiteral(lexical, opts...)
}

// sparqlStringUnescaper is a package-level replacer for SPARQL string escape sequences.
var sparqlStringUnescaper = strings.NewReplacer(`\"`, `"`, `\\`, `\`, `\n`, "\n", `\r`, "\r", `\t`, "\t")

func unescapeSPARQLString(s string) string {
	return sparqlStringUnescaper.Replace(s)
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
