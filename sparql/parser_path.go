package sparql

import (
	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/term"
)

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
		if p.pos < len(p.input) && (isPathModifier(p.input[p.pos]) || (p.input[p.pos] == '?' && (p.pos+1 >= len(p.input) || !isNameChar(rune(p.input[p.pos+1]))))) {
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

	// Check if followed by path operators (/, |, *, +)
	// Also check ? when not followed by a name char (variable)
	if p.pos < len(p.input) {
		ch2 := p.input[p.pos]
		if isPathModifier(ch2) || (ch2 == '?' && (p.pos+1 >= len(p.input) || !isNameChar(rune(p.input[p.pos+1])))) {
			p.pos = savedPos
			path, err := p.parsePathExpr()
			if err != nil {
				return "", nil, err
			}
			return "", path, nil
		}
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

	// Negated property set: !uri, !^uri, !(uri1|^uri2|uri3)
	if ch == '!' {
		p.pos++
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == '(' {
			p.pos++
			var fwdExcluded, invExcluded []term.URIRef
			for {
				p.skipWS()
				if p.pos < len(p.input) && p.input[p.pos] == ')' {
					p.pos++
					break
				}
				if len(fwdExcluded)+len(invExcluded) > 0 {
					p.skipWS()
					if p.pos < len(p.input) && p.input[p.pos] == '|' {
						p.pos++
					}
				}
				p.skipWS()
				inverse := false
				if p.pos < len(p.input) && p.input[p.pos] == '^' {
					p.pos++
					inverse = true
				}
				uri := p.resolvePathURI()
				if uri != "" {
					if inverse {
						invExcluded = append(invExcluded, term.NewURIRefUnsafe(uri))
					} else {
						fwdExcluded = append(fwdExcluded, term.NewURIRefUnsafe(uri))
					}
				}
			}
			return p.buildNegatedPath(fwdExcluded, invExcluded), nil
		}
		// Single negated URI or ^URI
		inverse := false
		if p.pos < len(p.input) && p.input[p.pos] == '^' {
			p.pos++
			inverse = true
		}
		uri := p.resolvePathURI()
		if uri != "" {
			u := term.NewURIRefUnsafe(uri)
			if inverse {
				return paths.Inv(paths.Negated(u)), nil
			}
			return paths.Negated(u), nil
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

// buildNegatedPath creates a path for !(fwd1|fwd2|^inv1|^inv2).
// Forward exclusions use NegatedPath; inverse exclusions use Inv(NegatedPath).
// Combined with Alternative if both exist.
func (p *sparqlParser) buildNegatedPath(fwd, inv []term.URIRef) paths.Path {
	if len(inv) == 0 {
		return paths.Negated(fwd...)
	}
	if len(fwd) == 0 {
		return paths.Inv(paths.Negated(inv...))
	}
	// Both forward and inverse: Alternative of (not-fwd) and ^(not-inv)
	return paths.Alternative(paths.Negated(fwd...), paths.Inv(paths.Negated(inv...)))
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
		local := unescapePNLocal(p.readPNLocal())
		if ns, ok := p.prefixes[prefix]; ok {
			return ns + local
		}
	}
	p.pos = start
	return ""
}
