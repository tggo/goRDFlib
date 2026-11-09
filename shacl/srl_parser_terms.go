package shacl

import "fmt"

// srlMode is the part of a document being parsed. The grammar has a separate
// production family for each: DATA blocks (no variables, no paths), rule heads
// (variables, no paths) and rule bodies (variables and paths).
type srlMode int

const (
	srlData srlMode = iota
	srlTemplate
	srlPattern
)

func (m srlMode) name() string {
	switch m {
	case srlData:
		return "a DATA block"
	case srlTemplate:
		return "a rule head"
	}
	return "a rule body"
}

var (
	rdfTypeTerm    = SRLTerm{Kind: SRLTermIRI, Value: RDFType}
	rdfFirstTerm   = SRLTerm{Kind: SRLTermIRI, Value: RDFFirst}
	rdfRestTerm    = SRLTerm{Kind: SRLTermIRI, Value: RDFRest}
	rdfNilTerm     = SRLTerm{Kind: SRLTermIRI, Value: RDFNil}
	rdfReifiesTerm = SRLTerm{Kind: SRLTermIRI, Value: RDF + "reifies"}
)

// startsSubject reports whether tok can begin a triples block.
func startsSubject(k srlTokenKind) bool {
	switch k {
	case srlTokIRI, srlTokPName, srlTokVar, srlTokBNode, srlTokLBrack, srlTokLParen,
		srlTokString, srlTokInteger, srlTokDecimal, srlTokDouble, srlTokTrue, srlTokFalse, srlTokLTriple:
		return true
	}
	return false
}

// parseTriplesBlock parses TriplesSameSubject ( '.' TriplesSameSubject? )* and
// stops before whatever cannot start another triple. p.lastDot reports
// whether the block ended with a '.'.
func (p *srlParser) parseTriplesBlock(mode srlMode) ([]SRLTriple, error) {
	var out []SRLTriple
	for {
		ts, err := p.parseTriplesSameSubject(mode)
		if err != nil {
			return nil, err
		}
		out = append(out, ts...)
		p.lastDot = false
		if ok, err := p.accept(srlTokDot); err != nil || !ok {
			return out, err
		}
		p.lastDot = true
		tok, err := p.peek()
		if err != nil {
			return nil, err
		}
		if !startsSubject(tok.kind) {
			return out, nil
		}
	}
}

func (p *srlParser) parseTriplesSameSubject(mode srlMode) ([]SRLTriple, error) {
	tok, err := p.peek()
	if err != nil {
		return nil, err
	}
	subj, aux, needsList, err := p.parseGraphNode(mode)
	if err != nil {
		return nil, err
	}
	out := aux
	nxt, err := p.peek()
	if err != nil {
		return nil, err
	}
	if !p.startsVerb(mode, nxt.kind) {
		if needsList {
			return nil, p.unexpected(nxt, "a predicate after the subject at position "+fmt.Sprint(tok.pos))
		}
		return out, nil
	}
	po, err := p.parsePropertyList(subj, mode)
	if err != nil {
		return nil, err
	}
	return append(out, po...), nil
}

func (p *srlParser) startsVerb(mode srlMode, k srlTokenKind) bool {
	switch k {
	case srlTokIRI, srlTokPName, srlTokA, srlTokVar, srlTokBNode:
		return true
	case srlTokLBrack:
		return true // reported as "blank node as predicate"
	case srlTokCaret, srlTokLParen:
		return mode == srlPattern
	}
	return false
}

// parsePropertyList parses a non-empty PropertyListNotEmpty for subj:
// Verb ObjectList ( ';' ( Verb ObjectList )? )*.
func (p *srlParser) parsePropertyList(subj SRLTerm, mode srlMode) ([]SRLTriple, error) {
	var out []SRLTriple
	for {
		pred, path, err := p.parseVerb(mode)
		if err != nil {
			return nil, err
		}
		for {
			obj, aux, _, err := p.parseGraphNode(mode)
			if err != nil {
				return nil, err
			}
			out = append(out, aux...)
			out = append(out, SRLTriple{Subject: subj, Predicate: pred, Object: obj, Path: path})
			ann, err := p.parseAnnotations(subj, pred, path, obj, mode)
			if err != nil {
				return nil, err
			}
			out = append(out, ann...)
			if ok, err := p.accept(srlTokComma); err != nil {
				return nil, err
			} else if !ok {
				break
			}
		}
		if semi, err := p.accept(srlTokSemicolon); err != nil || !semi {
			return out, err
		}
		for {
			more, err := p.accept(srlTokSemicolon)
			if err != nil {
				return nil, err
			}
			if !more {
				break
			}
		}
		tok, err := p.peek()
		if err != nil {
			return nil, err
		}
		if !p.startsVerb(mode, tok.kind) {
			return out, nil
		}
	}
}

// parseVerb parses a predicate: an IRI or 'a'; a variable (not in DATA); in a
// rule body also a property path (then path is non-nil).
func (p *srlParser) parseVerb(mode srlMode) (SRLTerm, *SRLPath, error) {
	tok, err := p.peek()
	if err != nil {
		return SRLTerm{}, nil, err
	}
	switch tok.kind {
	case srlTokVar:
		if mode == srlData {
			return SRLTerm{}, nil, fmt.Errorf("a variable is not allowed in %s (position %d)", mode.name(), tok.pos)
		}
		p.next()
		return SRLTerm{Kind: SRLTermVariable, Value: tok.val}, nil, nil
	case srlTokBNode, srlTokLBrack:
		return SRLTerm{}, nil, fmt.Errorf("a blank node cannot be a predicate (position %d)", tok.pos)
	case srlTokA, srlTokIRI, srlTokPName, srlTokCaret, srlTokLParen:
		if mode == srlPattern {
			path, err := p.parsePath()
			if err != nil {
				return SRLTerm{}, nil, err
			}
			if path.Kind == SRLPathIRI {
				return SRLTerm{Kind: SRLTermIRI, Value: path.IRI}, nil, nil
			}
			return SRLTerm{}, path, nil
		}
		if tok.kind == srlTokCaret || tok.kind == srlTokLParen {
			return SRLTerm{}, nil, fmt.Errorf("a property path is only allowed in a rule body, not in %s (position %d)", mode.name(), tok.pos)
		}
		return p.parseIRIVerb()
	}
	return SRLTerm{}, nil, p.unexpected(tok, "a predicate")
}

// parseIRIVerb parses VerbData / the IRI-or-'a' alternatives of Verb.
func (p *srlParser) parseIRIVerb() (SRLTerm, *SRLPath, error) {
	tok, err := p.next()
	if err != nil {
		return SRLTerm{}, nil, err
	}
	switch tok.kind {
	case srlTokA:
		return rdfTypeTerm, nil, nil
	case srlTokIRI:
		return SRLTerm{Kind: SRLTermIRI, Value: tok.val}, nil, nil
	case srlTokPName:
		iri, err := p.expandPName(tok)
		if err != nil {
			return SRLTerm{}, nil, err
		}
		return SRLTerm{Kind: SRLTermIRI, Value: iri}, nil, nil
	}
	return SRLTerm{}, nil, p.unexpected(tok, "a predicate")
}

// parsePath parses Path ::= PathEltOrInverse ( '/' PathEltOrInverse )*.
func (p *srlParser) parsePath() (*SRLPath, error) {
	var elems []*SRLPath
	for {
		inv, err := p.accept(srlTokCaret)
		if err != nil {
			return nil, err
		}
		elt, err := p.parsePathElt()
		if err != nil {
			return nil, err
		}
		if inv {
			elt = &SRLPath{Kind: SRLPathInverse, Elems: []*SRLPath{elt}}
		}
		elems = append(elems, elt)
		if ok, err := p.accept(srlTokSlash); err != nil {
			return nil, err
		} else if !ok {
			break
		}
	}
	if len(elems) == 1 {
		return elems[0], nil
	}
	return &SRLPath{Kind: SRLPathSeq, Elems: elems}, nil
}

func (p *srlParser) parsePathElt() (*SRLPath, error) {
	tok, err := p.peek()
	if err != nil {
		return nil, err
	}
	if tok.kind == srlTokLParen {
		p.next()
		inner, err := p.parsePath()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(srlTokRParen, "')' closing the path"); err != nil {
			return nil, err
		}
		return inner, nil
	}
	pred, _, err := p.parseIRIVerb()
	if err != nil {
		return nil, err
	}
	return &SRLPath{Kind: SRLPathIRI, IRI: pred.Value}, nil
}

// parseAnnotations parses ( Reifier | AnnotationBlock )* after an object and
// returns the rdf:reifies triples and the annotation triples.
func (p *srlParser) parseAnnotations(subj, pred SRLTerm, path *SRLPath, obj SRLTerm, mode srlMode) ([]SRLTriple, error) {
	var out []SRLTriple
	var cur *SRLTerm
	reify := func(r SRLTerm) {
		s, pr, o := subj, pred, obj
		out = append(out, SRLTriple{
			Subject:   r,
			Predicate: rdfReifiesTerm,
			Object:    SRLTerm{Kind: SRLTermTripleTerm, TTSubject: &s, TTPredicate: &pr, TTObject: &o},
		})
	}
	for {
		tok, err := p.peek()
		if err != nil {
			return nil, err
		}
		if tok.kind != srlTokTilde && tok.kind != srlTokLAnnot {
			return out, nil
		}
		if path != nil {
			return nil, fmt.Errorf("an annotation needs a single predicate, not a property path (position %d)", tok.pos)
		}
		p.next()
		if tok.kind == srlTokTilde {
			r, err := p.parseReifierID(mode)
			if err != nil {
				return nil, err
			}
			reify(r)
			cur = &r
			continue
		}
		var r SRLTerm
		if cur != nil {
			r = *cur
		} else {
			r = SRLTerm{Kind: SRLTermBlankNode, Value: p.newBNode()}
			reify(r)
		}
		cur = nil
		nxt, err := p.peek()
		if err != nil {
			return nil, err
		}
		if nxt.kind == srlTokRAnnot {
			return nil, p.unexpected(nxt, "a predicate in the annotation block")
		}
		po, err := p.parsePropertyList(r, mode)
		if err != nil {
			return nil, err
		}
		out = append(out, po...)
		if _, err := p.expect(srlTokRAnnot, "'|}' ending the annotation block"); err != nil {
			return nil, err
		}
	}
}

// parseReifierID parses the optional id after '~' (the '~' is consumed): a
// variable (not in DATA), IRI or blank node; absent, a fresh blank node.
func (p *srlParser) parseReifierID(mode srlMode) (SRLTerm, error) {
	tok, err := p.peek()
	if err != nil {
		return SRLTerm{}, err
	}
	switch tok.kind {
	case srlTokIRI, srlTokPName:
		iri, err := p.parseIRI("a reifier")
		return SRLTerm{Kind: SRLTermIRI, Value: iri}, err
	case srlTokBNode:
		p.next()
		return SRLTerm{Kind: SRLTermBlankNode, Value: tok.val}, nil
	case srlTokLBrack:
		p.next()
		if _, err := p.expect(srlTokRBrack, "']' (a reifier is [] or a blank node label)"); err != nil {
			return SRLTerm{}, err
		}
		return SRLTerm{Kind: SRLTermBlankNode, Value: p.newBNode()}, nil
	case srlTokVar:
		if mode == srlData {
			return SRLTerm{}, fmt.Errorf("a variable is not allowed in %s (position %d)", mode.name(), tok.pos)
		}
		p.next()
		return SRLTerm{Kind: SRLTermVariable, Value: tok.val}, nil
	}
	return SRLTerm{Kind: SRLTermBlankNode, Value: p.newBNode()}, nil
}

// parseGraphNode parses a term, a blank node property list, a collection or a
// reified triple. needsList is true for a node that, as a subject, must be
// followed by a property list (everything except [ ... ], a non-empty
// collection and a reified triple).
func (p *srlParser) parseGraphNode(mode srlMode) (t SRLTerm, aux []SRLTriple, needsList bool, err error) {
	tok, err := p.peek()
	if err != nil {
		return t, nil, false, err
	}
	switch tok.kind {
	case srlTokLBrack:
		p.next()
		bn := SRLTerm{Kind: SRLTermBlankNode, Value: p.newBNode()}
		if closed, err := p.accept(srlTokRBrack); err != nil {
			return t, nil, false, err
		} else if closed {
			return bn, nil, true, nil
		}
		po, err := p.parsePropertyList(bn, mode)
		if err != nil {
			return t, nil, false, err
		}
		if _, err := p.expect(srlTokRBrack, "']' ending the blank node property list"); err != nil {
			return t, nil, false, err
		}
		return bn, po, false, nil
	case srlTokLParen:
		return p.parseCollection(mode)
	case srlTokLTriple:
		if tok.val == "<<" {
			r, aux, err := p.parseReifiedTriple(mode)
			return r, aux, false, err
		}
	}
	t, err = p.parseSimpleTerm(mode)
	return t, nil, true, err
}

// parseCollection parses ( GraphNode+ ) into rdf:first/rest triples, or () into
// rdf:nil.
func (p *srlParser) parseCollection(mode srlMode) (SRLTerm, []SRLTriple, bool, error) {
	p.next() // (
	if closed, err := p.accept(srlTokRParen); err != nil {
		return SRLTerm{}, nil, false, err
	} else if closed {
		return rdfNilTerm, nil, true, nil
	}
	var items []SRLTerm
	var aux []SRLTriple
	for {
		item, a, _, err := p.parseGraphNode(mode)
		if err != nil {
			return SRLTerm{}, nil, false, err
		}
		items = append(items, item)
		aux = append(aux, a...)
		if closed, err := p.accept(srlTokRParen); err != nil {
			return SRLTerm{}, nil, false, err
		} else if closed {
			break
		}
	}
	head := SRLTerm{Kind: SRLTermBlankNode, Value: p.newBNode()}
	cur := head
	for i, item := range items {
		aux = append(aux, SRLTriple{Subject: cur, Predicate: rdfFirstTerm, Object: item})
		rest := rdfNilTerm
		if i < len(items)-1 {
			rest = SRLTerm{Kind: SRLTermBlankNode, Value: p.newBNode()}
		}
		aux = append(aux, SRLTriple{Subject: cur, Predicate: rdfRestTerm, Object: rest})
		cur = rest
	}
	return head, aux, false, nil
}

// parseReifiedTriple parses << s p o ~id? >>. The result is the reifier and
// the triple rdf:reifies <<( s p o )>> (plus those of nested reified
// triples). The reified triple itself is not asserted (RDF 1.2).
func (p *srlParser) parseReifiedTriple(mode srlMode) (SRLTerm, []SRLTriple, error) {
	p.next() // <<
	s, aux, err := p.parseReifiedElement(mode)
	if err != nil {
		return SRLTerm{}, nil, err
	}
	pred, _, err := p.parseTripleVerb(mode)
	if err != nil {
		return SRLTerm{}, nil, err
	}
	o, oaux, err := p.parseReifiedElement(mode)
	if err != nil {
		return SRLTerm{}, nil, err
	}
	aux = append(aux, oaux...)
	reifier := SRLTerm{Kind: SRLTermBlankNode, Value: p.newBNode()}
	if tilde, err := p.accept(srlTokTilde); err != nil {
		return SRLTerm{}, nil, err
	} else if tilde {
		if reifier, err = p.parseReifierID(mode); err != nil {
			return SRLTerm{}, nil, err
		}
	}
	end, err := p.next()
	if err != nil {
		return SRLTerm{}, nil, err
	}
	if end.kind != srlTokRTriple || end.val != ">>" {
		return SRLTerm{}, nil, p.unexpected(end, "'>>' ending the reified triple")
	}
	aux = append(aux, SRLTriple{
		Subject:   reifier,
		Predicate: rdfReifiesTerm,
		Object:    SRLTerm{Kind: SRLTermTripleTerm, TTSubject: &s, TTPredicate: &pred, TTObject: &o},
	})
	return reifier, aux, nil
}

// parseTripleVerb parses the Verb inside << >> and <<( )>>.
func (p *srlParser) parseTripleVerb(mode srlMode) (SRLTerm, *SRLPath, error) {
	tok, err := p.peek()
	if err != nil {
		return SRLTerm{}, nil, err
	}
	switch tok.kind {
	case srlTokVar:
		if mode == srlData {
			return SRLTerm{}, nil, fmt.Errorf("a variable is not allowed in %s (position %d)", mode.name(), tok.pos)
		}
		p.next()
		return SRLTerm{Kind: SRLTermVariable, Value: tok.val}, nil, nil
	case srlTokA, srlTokIRI, srlTokPName:
		return p.parseIRIVerb()
	}
	return SRLTerm{}, nil, p.unexpected(tok, "a predicate")
}

// parseReifiedElement parses a subject or object of a reified triple: a
// variable, IRI, literal, blank node (label or []), reified triple or triple
// term. A collection or a [ ... ] property list is not allowed there.
func (p *srlParser) parseReifiedElement(mode srlMode) (SRLTerm, []SRLTriple, error) {
	tok, err := p.peek()
	if err != nil {
		return SRLTerm{}, nil, err
	}
	if tok.kind == srlTokLTriple && tok.val == "<<" {
		return p.parseReifiedTriple(mode)
	}
	t, err := p.parseSimpleTerm(mode)
	return t, nil, err
}

// parseSimpleTerm parses a variable, IRI, literal, blank node label, [] or
// triple term. () is not accepted here: parseGraphNode handles it.
func (p *srlParser) parseSimpleTerm(mode srlMode) (SRLTerm, error) {
	tok, err := p.next()
	if err != nil {
		return SRLTerm{}, err
	}
	switch tok.kind {
	case srlTokIRI:
		return SRLTerm{Kind: SRLTermIRI, Value: tok.val}, nil
	case srlTokPName:
		iri, err := p.expandPName(tok)
		return SRLTerm{Kind: SRLTermIRI, Value: iri}, err
	case srlTokVar:
		if mode == srlData {
			return SRLTerm{}, fmt.Errorf("a variable is not allowed in %s (position %d)", mode.name(), tok.pos)
		}
		return SRLTerm{Kind: SRLTermVariable, Value: tok.val}, nil
	case srlTokBNode:
		return SRLTerm{Kind: SRLTermBlankNode, Value: tok.val}, nil
	case srlTokLBrack:
		if _, err := p.expect(srlTokRBrack, "']' (in this position a blank node is [] or a label)"); err != nil {
			return SRLTerm{}, err
		}
		return SRLTerm{Kind: SRLTermBlankNode, Value: p.newBNode()}, nil
	case srlTokString:
		return p.finishLiteral(tok)
	case srlTokInteger:
		return SRLTerm{Kind: SRLTermLiteral, Value: tok.val, Datatype: XSD + "integer"}, nil
	case srlTokDecimal:
		return SRLTerm{Kind: SRLTermLiteral, Value: tok.val, Datatype: XSD + "decimal"}, nil
	case srlTokDouble:
		return SRLTerm{Kind: SRLTermLiteral, Value: tok.val, Datatype: XSD + "double"}, nil
	case srlTokTrue:
		return SRLTerm{Kind: SRLTermLiteral, Value: "true", Datatype: XSD + "boolean"}, nil
	case srlTokFalse:
		return SRLTerm{Kind: SRLTermLiteral, Value: "false", Datatype: XSD + "boolean"}, nil
	case srlTokLTriple:
		if tok.val == "<<(" {
			return p.parseTripleTerm(mode)
		}
	}
	return SRLTerm{}, p.unexpected(tok, "an RDF term")
}

// finishLiteral reads the optional @lang or ^^datatype after a string.
func (p *srlParser) finishLiteral(str srlToken) (SRLTerm, error) {
	t := SRLTerm{Kind: SRLTermLiteral, Value: str.val, Datatype: XSD + "string"}
	tok, err := p.peek()
	if err != nil {
		return t, err
	}
	switch tok.kind {
	case srlTokLang:
		p.next()
		t.Language = tok.val
		t.Datatype = RDF + "langString"
		if containsDirSep(tok.val) {
			t.Datatype = RDF + "dirLangString"
		}
	case srlTokHat:
		p.next()
		dt, err := p.parseIRI("a datatype IRI after ^^")
		if err != nil {
			return t, err
		}
		t.Datatype = dt
	}
	return t, nil
}

func containsDirSep(lang string) bool {
	for i := 0; i+1 < len(lang); i++ {
		if lang[i] == '-' && lang[i+1] == '-' {
			return true
		}
	}
	return false
}

// parseTripleTerm parses <<( s p o )>> (the "<<(" is consumed).
func (p *srlParser) parseTripleTerm(mode srlMode) (SRLTerm, error) {
	s, err := p.parseSimpleTerm(mode)
	if err != nil {
		return SRLTerm{}, err
	}
	pred, _, err := p.parseTripleVerb(mode)
	if err != nil {
		return SRLTerm{}, err
	}
	o, err := p.parseSimpleTerm(mode)
	if err != nil {
		return SRLTerm{}, err
	}
	end, err := p.next()
	if err != nil {
		return SRLTerm{}, err
	}
	if end.kind != srlTokRTriple || end.val != ")>>" {
		return SRLTerm{}, p.unexpected(end, "')>>' ending the triple term")
	}
	return SRLTerm{Kind: SRLTermTripleTerm, TTSubject: &s, TTPredicate: &pred, TTObject: &o}, nil
}
