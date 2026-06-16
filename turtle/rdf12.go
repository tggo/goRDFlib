// RDF 1.2 extensions for the Turtle parser: triple terms, reified triples,
// annotations, reifiers, and VERSION directives.
package turtle

import rdflibgo "github.com/tggo/goRDFlib"

func (p *turtleParser) sparqlVersion() error {
	p.pos += 7 // skip "VERSION"
	p.skipWS()
	if _, err := p.readVersionString(); err != nil {
		return err
	}
	return nil
}

// readVersionString reads a single-quoted or double-quoted short string (no triple-quoted).
func (p *turtleParser) readVersionString() (string, error) {
	if p.pos >= len(p.input) {
		return "", p.errorf("expected version string")
	}
	ch := p.input[p.pos]
	if ch != '"' && ch != '\'' {
		return "", p.errorf("expected quoted string for version, got %q", ch)
	}
	// Reject triple-quoted strings
	if p.pos+2 < len(p.input) && p.input[p.pos+1] == ch && p.input[p.pos+2] == ch {
		return "", p.errorf("triple-quoted strings not allowed for version")
	}
	p.pos++ // skip opening quote
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

// readAnnotationsAndReifiers parses zero or more reifier (~id) and annotation ({| ... |}) blocks
// after a triple's object. Each ~id and/or {| |} creates a reifier node linked via rdf:reifies.
func (p *turtleParser) readAnnotationsAndReifiers(subj rdflibgo.Subject, pred rdflibgo.URIRef, obj rdflibgo.Term) error {
	// Lazy-init: only create TripleTerm when actually needed (annotation/reifier found).
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

		// Reifier: ~ id or ~ (anonymous)
		if p.input[p.pos] == '~' {
			p.pos++ // skip ~
			p.skipWS()

			var reifier rdflibgo.Subject
			if p.pos < len(p.input) && p.input[p.pos] != '{' && p.input[p.pos] != '.' &&
				p.input[p.pos] != ';' && p.input[p.pos] != ',' && p.input[p.pos] != ']' &&
				p.input[p.pos] != '~' && p.input[p.pos] != '|' {
				// Named reifier (IRI, prefixed name, or blank node)
				var err error
				reifier, err = p.readReifierID()
				if err != nil {
					return err
				}
			} else {
				// Anonymous reifier
				reifier = rdflibgo.NewBNode()
			}
			p.emit(reifier, reifiesPred, getTripleTerm())

			// Check for annotation block after reifier
			p.skipWS()
			if p.pos+1 < len(p.input) && p.input[p.pos] == '{' && p.input[p.pos+1] == '|' {
				if err := p.readAnnotationBlock(reifier, getTripleTerm()); err != nil {
					return err
				}
			}
			continue
		}

		// Annotation block: {| predObjectList |}
		if p.input[p.pos] == '{' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '|' {
			reifier := rdflibgo.NewBNode()
			p.emit(reifier, reifiesPred, getTripleTerm())
			if err := p.readAnnotationBlock(reifier, getTripleTerm()); err != nil {
				return err
			}
			continue
		}

		break
	}
	return nil
}

// readEmptyBNodeOnly reads [] but rejects [pred obj] — used inside reified triples.
func (p *turtleParser) readEmptyBNodeOnly() (rdflibgo.BNode, error) {
	p.pos++ // skip '['
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == ']' {
		p.pos++
		return rdflibgo.NewBNode(), nil
	}
	return rdflibgo.BNode{}, p.errorf("blank node property list not allowed in reified triple (only [] is allowed)")
}

// readReifierID reads a reifier identifier: IRI, prefixed name, or blank node.
func (p *turtleParser) readReifierID() (rdflibgo.Subject, error) {
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
	// Prefixed name
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

// readAnnotationBlock reads {| predicateObjectList |} and asserts triples on the reifier.
func (p *turtleParser) readAnnotationBlock(reifier rdflibgo.Subject, _ rdflibgo.TripleTerm) error {
	// Consume "{|"
	p.pos += 2
	p.skipWS()

	// Check for empty annotation block — that's an error
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

// readTripleTermOrReified reads either <<( s p o )>> (triple term) or << s p o >> (reified triple in object position).
func (p *turtleParser) readTripleTermOrReified() (rdflibgo.Term, error) {
	p.pos += 2 // skip "<<"
	p.skipWS()

	// Triple term: <<( s p o )>>
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		return p.readTripleTermInner()
	}

	// Reified triple: << s p o >> or << s p o ~ id >>
	return p.readReifiedTripleInner()
}

// readTripleTermInner parses the inner part of <<( s p o )>> after "<<" has been consumed.
func (p *turtleParser) readTripleTermInner() (rdflibgo.TripleTerm, error) {
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

// readTripleTermSubject reads a subject for a triple term (IRI or blank node, not a reified triple).
func (p *turtleParser) readTripleTermSubject() (rdflibgo.Subject, error) {
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
	// Prefixed name
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

// readReifiedTriple reads << s p o >> or << s p o ~ id >> as a subject.
// The reified triple creates a node (bnode or named) that gets rdf:reifies <<(s p o)>>.
func (p *turtleParser) readReifiedTriple() (rdflibgo.Subject, error) {
	p.pos += 2 // skip "<<"
	p.skipWS()

	// Check for "(" — that would be a triple term, which is not valid as subject
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		return nil, p.errorf("triple term <<( ... )>> cannot be used as subject")
	}

	return p.readReifiedTripleInner()
}

// readReifiedTripleInner parses the inside of << s p o [~ id] >> after "<<" has been consumed.
// Returns the reifier node (bnode or IRI).
func (p *turtleParser) readReifiedTripleInner() (rdflibgo.Subject, error) {
	// Read inner subject — cannot be a literal or collection
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

	// Optional reifier: ~ id
	var reifier rdflibgo.Subject
	if p.pos < len(p.input) && p.input[p.pos] == '~' {
		p.pos++ // skip ~
		p.skipWS()
		// Check if there's an identifier or just >>
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

	// Emit the rdf:reifies triple
	tt := rdflibgo.NewTripleTerm(subj, pred, obj)
	p.emit(reifier, rdflibgo.RDFReifies, tt)

	return reifier, nil
}

// readReifiedInnerSubject reads the subject inside a reified triple.
// IRI, prefixed name, blank node label, empty [], or nested reified triple.
// Blank node property lists with content (e.g. [:p :o]) are NOT allowed.
func (p *turtleParser) readReifiedInnerSubject() (rdflibgo.Subject, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input in reified triple subject")
	}
	ch := p.input[p.pos]
	// Nested reified triple
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
	// Prefixed name
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

// readReifiedInnerObject reads the object inside a reified triple.
// IRI, prefixed name, blank node, literal, or nested reified triple.
// Collections and blank node property lists with content are NOT allowed.
func (p *turtleParser) readReifiedInnerObject() (rdflibgo.Term, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input in reified triple object")
	}
	ch := p.input[p.pos]

	// Nested reified triple or triple term
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

	// Try numeric literal
	if ch == '+' || ch == '-' || (ch >= '0' && ch <= '9') || ch == '.' {
		if lit, ok := p.tryNumeric(); ok {
			return lit, nil
		}
	}

	// Boolean keywords
	if p.startsWith("true") && (p.pos+4 >= len(p.input) || isDelimiter(p.input[p.pos+4])) {
		p.pos += 4
		return rdflibgo.NewLiteral(true), nil
	}
	if p.startsWith("false") && (p.pos+5 >= len(p.input) || isDelimiter(p.input[p.pos+5])) {
		p.pos += 5
		return rdflibgo.NewLiteral(false), nil
	}

	// Collection not allowed in reified triple
	if ch == '(' {
		return nil, p.errorf("collection not allowed in reified triple")
	}
	// Only empty blank node [] allowed in reified triple; [pred obj] is not.
	if ch == '[' {
		return p.readEmptyBNodeOnly()
	}

	// Prefixed name
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}
