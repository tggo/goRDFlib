package sparql


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

// ParseUpdate parses a SPARQL Update request string.
func ParseUpdate(input string) (*ParsedUpdate, error) {
	p := &sparqlParser{
		input:    input,
		pos:      0,
		prefixes: make(map[string]string),
	}
	return p.parseUpdate()
}

type sparqlParser struct {
	input          string
	pos            int
	prefixes       map[string]string
	bnodeCount     int
	reifierTriples []Triple // pending rdf:reifies triples from reified triple syntax
	tripleTermError error   // deferred error from triple term validation
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
			if prefix != "" && !((prefix[0] >= 'a' && prefix[0] <= 'z') || (prefix[0] >= 'A' && prefix[0] <= 'Z')) {
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
	} else {
		return nil, p.errorf("expected SELECT, ASK, or CONSTRUCT")
	}

	// FROM / FROM NAMED clauses (skip dataset declarations)
	for {
		p.skipWS()
		if p.matchKeywordCI("FROM") {
			p.pos += 4
			p.skipWS()
			if p.matchKeywordCI("NAMED") {
				p.pos += 5
				p.skipWS()
			}
			p.readTermOrVar() // skip the IRI
			continue
		}
		break
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

