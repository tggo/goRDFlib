package sparql

// Parse parses a SPARQL query string.
// Ported from: rdflib.plugins.sparql.parser.parseQuery
func Parse(input string) (*ParsedQuery, error) {
	p := &sparqlParser{
		input:    input,
		pos:      0,
		prefixes: make(map[string]string),
	}
	q, err := p.parse()
	if p.undeclaredPrefix != nil { // the likelier cause of any later error
		return nil, p.undeclaredPrefix
	}
	if err == nil && p.badSyntax != nil {
		return nil, p.badSyntax
	}
	if err == nil {
		blankLabelsInQuery(q)
	}
	return q, err
}

// ParseUpdate parses a SPARQL Update request string.
func ParseUpdate(input string) (*ParsedUpdate, error) {
	p := &sparqlParser{
		input:    input,
		pos:      0,
		prefixes: make(map[string]string),
	}
	u, err := p.parseUpdate()
	if p.undeclaredPrefix != nil { // the likelier cause of any later error
		return nil, p.undeclaredPrefix
	}
	if err == nil && p.badSyntax != nil {
		return nil, p.badSyntax
	}
	if err == nil {
		for _, op := range u.Operations {
			if m, ok := op.(*ModifyOp); ok {
				blankLabelsToVars(m.Where)
			}
		}
	}
	return u, err
}

type sparqlParser struct {
	input           string
	pos             int
	prefixes        map[string]string
	bnodeCount      int
	dotAfterTriples bool           // the last triples block ended with '.'
	bgpSeq          int            // last basic graph pattern id (nextBGP)
	bgpLabels       map[string]int // blank-node label -> the BGP that used it
	reifierTriples  []Triple       // pending rdf:reifies triples from reified triple syntax
	tripleTermError error          // deferred error from triple term validation
	// undeclaredPrefix is the first prefixed name whose prefix the prologue
	// did not declare (SPARQL 1.1 §4.1.1: a query error). It is deferred
	// because the term readers return strings, not errors; Parse and
	// ParseUpdate report it once parsing has otherwise succeeded.
	undeclaredPrefix error
	// badSyntax is the first lexical error the term readers cannot return (a
	// ? or $ with no name, a variable position holding something else, a bare
	// word where a term belongs); deferred like undeclaredPrefix, reported
	// when parsing has otherwise succeeded.
	badSyntax error
}

func (p *sparqlParser) parse() (*ParsedQuery, error) {
	// Preprocess codepoint escapes outside strings
	p.input = preprocessCodepointEscapes(p.input)

	q := &ParsedQuery{
		Limit:    -1,
		Prefixes: p.prefixes,
	}

	// Prologue: PREFIX, BASE, VERSION
	for {
		p.skipWS()
		if p.matchKeywordCI("PREFIX") {
			p.pos += 6
			p.skipWS()
			prefix := p.readUntil(':')
			// Validate prefix: must start with letter or be empty
			if prefix != "" && !((prefix[0] >= 'a' && prefix[0] <= 'z') || (prefix[0] >= 'A' && prefix[0] <= 'Z') || (prefix[0] >= 0x80 && validPNRunes(prefix))) {
				return nil, p.errorf("invalid prefix name: %q", prefix)
			}
			p.pos++ // skip ':'
			p.skipWS()
			iri := p.readIRIRef()
			p.prefixes[prefix] = iri
			continue
		}
		if p.matchKeywordCI("BASE") {
			p.pos += 4
			p.skipWS()
			q.BaseURI = p.readIRIRef()
			continue
		}
		if p.matchKeywordCI("VERSION") {
			if err := p.parseVersion(); err != nil {
				return nil, err
			}
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
		// CONSTRUCT WHERE or CONSTRUCT FROM ... WHERE shorthand
		if !p.matchKeywordCI("WHERE") && !p.matchKeywordCI("FROM") {
			if err := p.parseConstruct(q); err != nil {
				return nil, err
			}
		}
	} else if p.matchKeywordCI("DESCRIBE") {
		p.pos += 8
		q.Type = "DESCRIBE"
		if err := p.parseDescribeTargets(q); err != nil {
			return nil, err
		}
	} else {
		return nil, p.errorf("expected SELECT, ASK, CONSTRUCT, or DESCRIBE")
	}

	// FROM / FROM NAMED dataset clauses
	for {
		p.skipWS()
		if !p.matchKeywordCI("FROM") {
			break
		}
		p.pos += 4
		p.skipWS()
		named := false
		if p.matchKeywordCI("NAMED") {
			p.pos += 5
			named = true
			p.skipWS()
		}
		iriTerm := p.readTermOrVar()
		q.DatasetClause = append(q.DatasetClause, DatasetClause{
			IRI:   p.datasetIRI(iriTerm, q.BaseURI),
			Named: named,
		})
	}

	// WHERE clause (optional for DESCRIBE: DescribeQuery ::= ... WhereClause?)
	p.skipWS()
	if q.Type == "DESCRIBE" && !p.matchKeywordCI("WHERE") && (p.pos >= len(p.input) || p.input[p.pos] != '{') {
		q.Where = &BGP{}
		if err := p.parseSolutionModifiers(q); err != nil {
			return nil, err
		}
		return q, nil
	}
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

	// Validate BIND scope
	if err := validateBindScope(q.Where); err != nil {
		return nil, err
	}

	// Validate CONSTRUCT WHERE shorthand: only simple BGPs allowed
	if q.Type == "CONSTRUCT" && len(q.Construct) == 0 {
		if err := validateConstructWhere(q.Where); err != nil {
			return nil, err
		}
	}

	// Semantic validation
	if err := p.validate(q); err != nil {
		return nil, err
	}

	// Post-query VALUES/BINDINGS clause
	p.skipWS()
	if p.matchKeywordCI("BINDINGS") {
		p.pos += 8
		vp, err := p.parseValues()
		if err != nil {
			return nil, err
		}
		if q.Where == nil {
			q.Where = vp
		} else {
			q.Where = &JoinPattern{Left: q.Where, Right: vp}
		}
	} else if p.matchKeywordCI("VALUES") {
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
