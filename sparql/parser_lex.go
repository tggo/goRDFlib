package sparql

import (
	"fmt"
	"strconv"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

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
		s := p.readStringLiteral()
		if err := validateStringEscapes(s); err != nil {
			return "" // will cause a parse error downstream
		}
		return s
	}

	// Blank node with property list: [ pred obj ; ... ]
	if ch == '[' {
		return p.readBlankNodePropertyList()
	}

	// Collection syntax: ( term1 term2 ... )
	if ch == '(' {
		return p.readCollectionAsTerm()
	}

	// Blank node: _:label
	if ch == '_' && p.pos+1 < len(p.input) && p.input[p.pos+1] == ':' {
		start := p.pos
		p.pos += 2 // skip "_:"
		for p.pos < len(p.input) && (isNameChar(rune(p.input[p.pos])) || p.input[p.pos] == '.' || p.input[p.pos] == '-') {
			p.pos++
		}
		// Trim trailing dots
		for p.pos > start+2 && p.input[p.pos-1] == '.' {
			p.pos--
		}
		return p.input[start:p.pos]
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

	// Prefixed name (may start with digit for local part like :123)
	start := p.pos
	for p.pos < len(p.input) && isNameChar(rune(p.input[p.pos])) {
		p.pos++
	}
	if p.pos < len(p.input) && p.input[p.pos] == ':' {
		p.pos++
		p.readPNLocal()
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
// Returns a fresh blank node variable name. The caller should use
// parseBnodePropertyList to get the additional triples.
func (p *sparqlParser) readBlankNodePropertyList() string {
	p.bnodeCount++
	bnode := fmt.Sprintf("?_bnode%d", p.bnodeCount)
	p.pos++ // skip [
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == ']' {
		p.pos++
		return bnode
	}
	// Skip to matching ] — triples will be handled by parseBnodeTriples
	depth := 1
	for p.pos < len(p.input) && depth > 0 {
		if p.input[p.pos] == '[' {
			depth++
		} else if p.input[p.pos] == ']' {
			depth--
		}
		p.pos++
	}
	return bnode
}

// parseBnodePropertyListTriples parses [ pred obj ; ... ] and returns the bnode var and extra triples.
func (p *sparqlParser) parseBnodePropertyListTriples() (string, []Triple, error) {
	p.bnodeCount++
	bnode := fmt.Sprintf("?_bnode%d", p.bnodeCount)
	p.pos++ // skip [
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == ']' {
		p.pos++
		return bnode, nil, nil
	}

	var triples []Triple
	for {
		p.skipWS()
		if p.pos >= len(p.input) || p.input[p.pos] == ']' {
			if p.pos < len(p.input) {
				p.pos++
			}
			break
		}
		pred, predPath, err := p.parsePredicateOrPath()
		if err != nil {
			return bnode, nil, err
		}
		if pred == "" && predPath == nil {
			break
		}
		p.skipWS()
		for {
			obj := p.readTermOrVar()
			if obj == "" {
				break
			}
			t := Triple{Subject: bnode, Predicate: pred, Object: obj}
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
			continue
		}
		if p.pos < len(p.input) && p.input[p.pos] == ']' {
			p.pos++
			break
		}
	}
	return bnode, triples, nil
}

// readPNLocal reads the local part of a prefixed name (after the colon).
// Supports SPARQL 1.1 PN_LOCAL: name chars, dots, dashes, colons, % escapes, \ escapes.
func (p *sparqlParser) readPNLocal() string {
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if isNameChar(rune(ch)) || ch == '.' || ch == '-' || ch == ':' {
			p.pos++
		} else if ch == '%' && p.pos+2 < len(p.input) {
			// Percent-encoded char: %HH
			p.pos += 3
		} else if ch == '\\' && p.pos+1 < len(p.input) && isPNLocalEscChar(p.input[p.pos+1]) {
			// PN_LOCAL_ESC: backslash-escaped char
			p.pos += 2
		} else {
			break
		}
	}
	// Trim trailing dots (not part of the name), but not if escaped (\.)
	for p.pos > start && p.input[p.pos-1] == '.' {
		if p.pos >= start+2 && p.input[p.pos-2] == '\\' {
			break // escaped dot, keep it
		}
		p.pos--
	}
	return p.input[start:p.pos]
}

// readCollectionAsTerm skips a collection ( ... ) and returns a bnode placeholder.
// The actual triples are generated by parseCollectionTriples.
func (p *sparqlParser) readCollectionAsTerm() string {
	p.bnodeCount++
	bnode := fmt.Sprintf("?_coll%d", p.bnodeCount)
	// Skip to matching )
	p.pos++ // skip (
	depth := 1
	for p.pos < len(p.input) && depth > 0 {
		if p.input[p.pos] == '(' {
			depth++
		} else if p.input[p.pos] == ')' {
			depth--
		}
		p.pos++
	}
	return bnode
}

// parseCollectionTriples parses ( term1 term2 ... ) and returns the head bnode and rdf:first/rest triples.
func (p *sparqlParser) parseCollectionTriples() (string, []Triple, error) {
	p.pos++ // skip (
	rdfFirst := "<" + rdflibgo.RDF.First.Value() + ">"
	rdfRest := "<" + rdflibgo.RDF.Rest.Value() + ">"
	rdfNil := "<" + rdflibgo.RDF.Nil.Value() + ">"

	var triples []Triple
	var head string
	var prevBnode string

	for {
		p.skipWS()
		if p.pos >= len(p.input) || p.input[p.pos] == ')' {
			p.pos++
			break
		}

		p.bnodeCount++
		bnode := fmt.Sprintf("?_coll%d", p.bnodeCount)

		if head == "" {
			head = bnode
		}

		// Link previous node to this one via rdf:rest
		if prevBnode != "" {
			triples = append(triples, Triple{Subject: prevBnode, Predicate: rdfRest, Object: bnode})
		}

		// Read the element (may be a blank node property list, nested collection, or regular term)
		var elem string
		if p.pos < len(p.input) && p.input[p.pos] == '[' {
			bn, extraTriples, err := p.parseBnodePropertyListTriples()
			if err != nil {
				return "", nil, err
			}
			elem = bn
			triples = append(triples, extraTriples...)
		} else {
			elem = p.readTermOrVar()
		}

		triples = append(triples, Triple{Subject: bnode, Predicate: rdfFirst, Object: elem})
		prevBnode = bnode
	}

	// Terminate with rdf:nil
	if prevBnode != "" {
		triples = append(triples, Triple{Subject: prevBnode, Predicate: rdfRest, Object: rdfNil})
	}
	if head == "" {
		head = rdfNil
	}

	return head, triples, nil
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
