// RDF 1.2 extensions for the TriG parser: triple terms, reified triples,
// annotations, reifiers, and VERSION directives.
package trig

import rdflibgo "github.com/tggo/goRDFlib"

func (p *trigParser) sparqlVersion() error {
	p.pos += 7 // skip "VERSION"
	p.skipWS()
	if _, err := p.readVersionString(); err != nil {
		return err
	}
	return nil
}

func (p *trigParser) readVersionString() (string, error) {
	if p.pos >= len(p.input) {
		return "", p.errorf("expected version string")
	}
	ch := p.input[p.pos]
	if ch != '"' && ch != '\'' {
		return "", p.errorf("expected quoted string for version, got %q", ch)
	}
	if p.pos+2 < len(p.input) && p.input[p.pos+1] == ch && p.input[p.pos+2] == ch {
		return "", p.errorf("triple-quoted strings not allowed for version")
	}
	p.pos++
	start := p.pos
	for p.pos < len(p.input) {
		if p.input[p.pos] == ch {
			val := p.input[start:p.pos]
			p.pos++
			return val, nil
		}
		if p.input[p.pos] == '\n' || p.input[p.pos] == '\r' {
			return "", p.errorf("newline in version string")
		}
		p.pos++
	}
	return "", p.errorf("unterminated version string")
}

// readAnnotationsAndReifiers parses zero or more reifier (~id) and annotation ({| ... |}) blocks.
func (p *trigParser) readAnnotationsAndReifiers(subj rdflibgo.Subject, pred rdflibgo.URIRef, obj rdflibgo.Term) error {
	var tt rdflibgo.TripleTerm
	var ttInit bool
	getTripleTerm := func() rdflibgo.TripleTerm {
		if !ttInit {
			tt = rdflibgo.NewTripleTerm(subj, pred, obj)
			ttInit = true
		}
		return tt
	}
	reifiesPred := rdflibgo.RDFReifies

	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			break
		}

		if p.input[p.pos] == '~' {
			p.pos++
			p.skipWS()

			var reifier rdflibgo.Subject
			if p.pos < len(p.input) && p.input[p.pos] != '{' && p.input[p.pos] != '.' &&
				p.input[p.pos] != ';' && p.input[p.pos] != ',' && p.input[p.pos] != ']' &&
				p.input[p.pos] != '~' && p.input[p.pos] != '|' && p.input[p.pos] != '}' {
				var err error
				reifier, err = p.readReifierID()
				if err != nil {
					return err
				}
			} else {
				reifier = rdflibgo.NewBNode()
			}
			p.currentGraph.Add(reifier, reifiesPred, getTripleTerm())

			p.skipWS()
			if p.pos+1 < len(p.input) && p.input[p.pos] == '{' && p.input[p.pos+1] == '|' {
				if err := p.readAnnotationBlock(reifier); err != nil {
					return err
				}
			}
			continue
		}

		if p.input[p.pos] == '{' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '|' {
			reifier := rdflibgo.NewBNode()
			p.currentGraph.Add(reifier, reifiesPred, getTripleTerm())
			if err := p.readAnnotationBlock(reifier); err != nil {
				return err
			}
			continue
		}

		break
	}
	return nil
}

func (p *trigParser) readEmptyBNodeOnly() (rdflibgo.BNode, error) {
	p.pos++
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == ']' {
		p.pos++
		return rdflibgo.NewBNode(), nil
	}
	return rdflibgo.BNode{}, p.errorf("blank node property list not allowed in reified triple (only [] is allowed)")
}

func (p *trigParser) readReifierID() (rdflibgo.Subject, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("expected reifier identifier")
	}
	ch := p.input[p.pos]
	if ch == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return nil, err
		}
		return rdflibgo.NewURIRefUnsafe(p.resolveIRI(iri)), nil
	}
	if ch == '_' && p.pos+1 < len(p.input) && p.input[p.pos+1] == ':' {
		return p.readBlankNodeLabel()
	}
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

func (p *trigParser) readAnnotationBlock(reifier rdflibgo.Subject) error {
	p.pos += 2 // skip "{|"
	p.skipWS()

	if p.pos+1 < len(p.input) && p.input[p.pos] == '|' && p.input[p.pos+1] == '}' {
		return p.errorf("empty annotation block not allowed")
	}

	if err := p.predicateObjectList(reifier); err != nil {
		return err
	}

	p.skipWS()
	if p.pos+1 >= len(p.input) || p.input[p.pos] != '|' || p.input[p.pos+1] != '}' {
		return p.errorf("expected '|}' to close annotation block")
	}
	p.pos += 2
	return nil
}

func (p *trigParser) readTripleTermOrReified() (rdflibgo.Term, error) {
	p.pos += 2 // skip "<<"
	p.skipWS()

	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		return p.readTripleTermInner()
	}

	return p.readReifiedTripleInner()
}

func (p *trigParser) readTripleTermInner() (rdflibgo.TripleTerm, error) {
	p.pos++ // skip '('
	p.skipWS()

	subj, err := p.readTripleTermSubject()
	if err != nil {
		return rdflibgo.TripleTerm{}, err
	}

	pred, err := p.readPredicate()
	if err != nil {
		return rdflibgo.TripleTerm{}, err
	}

	obj, err := p.readObject()
	if err != nil {
		return rdflibgo.TripleTerm{}, err
	}

	p.skipWS()
	if !p.expect(')') {
		return rdflibgo.TripleTerm{}, p.errorf("expected ')' in triple term")
	}
	p.skipWS()
	if !p.startsWith(">>") {
		return rdflibgo.TripleTerm{}, p.errorf("expected '>>' to close triple term")
	}
	p.pos += 2

	return rdflibgo.NewTripleTerm(subj, pred, obj), nil
}

func (p *trigParser) readTripleTermSubject() (rdflibgo.Subject, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input, expected triple term subject")
	}
	ch := p.input[p.pos]
	if ch == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return nil, err
		}
		return rdflibgo.NewURIRefUnsafe(p.resolveIRI(iri)), nil
	}
	if ch == '_' && p.pos+1 < len(p.input) && p.input[p.pos+1] == ':' {
		return p.readBlankNodeLabel()
	}
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

func (p *trigParser) readReifiedTriple() (rdflibgo.Subject, error) {
	p.pos += 2 // skip "<<"
	p.skipWS()

	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		return nil, p.errorf("triple term <<( ... )>> cannot be used as subject")
	}

	return p.readReifiedTripleInner()
}

func (p *trigParser) readReifiedTripleInner() (rdflibgo.Subject, error) {
	subj, err := p.readReifiedInnerSubject()
	if err != nil {
		return nil, err
	}

	pred, err := p.readPredicate()
	if err != nil {
		return nil, err
	}

	obj, err := p.readReifiedInnerObject()
	if err != nil {
		return nil, err
	}

	p.skipWS()

	var reifier rdflibgo.Subject
	if p.pos < len(p.input) && p.input[p.pos] == '~' {
		p.pos++
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] != '>' {
			reifier, err = p.readReifierID()
			if err != nil {
				return nil, err
			}
		} else {
			reifier = rdflibgo.NewBNode()
		}
	} else {
		reifier = rdflibgo.NewBNode()
	}

	p.skipWS()
	if !p.startsWith(">>") {
		return nil, p.errorf("expected '>>' to close reified triple")
	}
	p.pos += 2

	tt := rdflibgo.NewTripleTerm(subj, pred, obj)
	p.currentGraph.Add(reifier, rdflibgo.RDFReifies, tt)

	return reifier, nil
}

func (p *trigParser) readReifiedInnerSubject() (rdflibgo.Subject, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input in reified triple subject")
	}
	ch := p.input[p.pos]
	if ch == '<' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '<' {
		return p.readReifiedTriple()
	}
	if ch == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return nil, err
		}
		return rdflibgo.NewURIRefUnsafe(p.resolveIRI(iri)), nil
	}
	if ch == '_' && p.pos+1 < len(p.input) && p.input[p.pos+1] == ':' {
		return p.readBlankNodeLabel()
	}
	if ch == '[' {
		return p.readEmptyBNodeOnly()
	}
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

func (p *trigParser) readReifiedInnerObject() (rdflibgo.Term, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input in reified triple object")
	}
	ch := p.input[p.pos]

	if ch == '<' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '<' {
		return p.readTripleTermOrReified()
	}
	if ch == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return nil, err
		}
		return rdflibgo.NewURIRefUnsafe(p.resolveIRI(iri)), nil
	}
	if ch == '_' && p.pos+1 < len(p.input) && p.input[p.pos+1] == ':' {
		return p.readBlankNodeLabel()
	}
	if ch == '"' || ch == '\'' {
		return p.readLiteral()
	}

	if ch == '+' || ch == '-' || (ch >= '0' && ch <= '9') || ch == '.' {
		if lit, ok := p.tryNumeric(); ok {
			return lit, nil
		}
	}

	if p.startsWith("true") && (p.pos+4 >= len(p.input) || isDelimiter(p.input[p.pos+4])) {
		p.pos += 4
		return rdflibgo.NewLiteral(true), nil
	}
	if p.startsWith("false") && (p.pos+5 >= len(p.input) || isDelimiter(p.input[p.pos+5])) {
		p.pos += 5
		return rdflibgo.NewLiteral(false), nil
	}

	if ch == '(' {
		return nil, p.errorf("collection not allowed in reified triple")
	}
	if ch == '[' {
		return p.readEmptyBNodeOnly()
	}

	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}
