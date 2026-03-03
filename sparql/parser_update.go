package sparql

import (
	"strings"

	"github.com/tggo/goRDFlib/term"
)

// convertInternalVarsToBnodes replaces internal parser-generated variables (?_reifier*, ?_bnode*, ?_coll*)
// with blank node labels (_:reifier*, etc.) for use in DATA operations where variables aren't allowed.
func convertInternalVarsToBnodes(triples []Triple) {
	mapping := make(map[string]string)
	convert := func(s string) string {
		if !strings.HasPrefix(s, "?_") {
			return s
		}
		name := s[1:] // strip ?
		if strings.HasPrefix(name, "_reifier") || strings.HasPrefix(name, "_bnode") || strings.HasPrefix(name, "_coll") {
			if bn, ok := mapping[s]; ok {
				return bn
			}
			bn := "_:" + name[1:] // strip leading _
			mapping[s] = bn
			return bn
		}
		return s
	}
	for i := range triples {
		triples[i].Subject = convert(triples[i].Subject)
		triples[i].Predicate = convert(triples[i].Predicate)
		triples[i].Object = convert(triples[i].Object)
	}
}

// --- SPARQL Update parsing ---

func (p *sparqlParser) parseUpdate() (*ParsedUpdate, error) {
	// Preprocess codepoint escapes outside strings
	p.input = preprocessCodepointEscapes(p.input)

	u := &ParsedUpdate{
		Prefixes: p.prefixes,
	}

	var allBnodeLabels map[string]int // label → operation index

	for {
		opIndex := len(u.Operations)
		_ = opIndex

		// Prologue: PREFIX, BASE, VERSION (can appear before each operation)
		for {
			p.skipWS()
			if p.matchKeywordCI("VERSION") {
				if err := p.parseVersion(); err != nil {
					return nil, err
				}
				continue
			}
			if p.matchKeywordCI("PREFIX") {
				p.pos += 6
				p.skipWS()
				prefix := p.readUntil(':')
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
				u.BaseURI = p.readIRIRef()
				continue
			}
			break
		}

		p.skipWS()
		if p.pos >= len(p.input) {
			break
		}

		op, err := p.parseUpdateOperation()
		if err != nil {
			return nil, err
		}
		if op != nil {
			// Check for bnode label reuse across DATA operations
			labels := collectDataBnodeLabels(op)
			if len(labels) > 0 {
				if allBnodeLabels == nil {
					allBnodeLabels = make(map[string]int)
				}
				for l := range labels {
					if prevOp, exists := allBnodeLabels[l]; exists && prevOp != opIndex {
						return nil, p.errorf("blank node label _:%s reused across operations", l)
					}
					allBnodeLabels[l] = opIndex
				}
			}
			u.Operations = append(u.Operations, op)
		}

		p.skipWS()
		if p.pos >= len(p.input) {
			break
		}
		if p.input[p.pos] == ';' {
			p.pos++
			// Skip consecutive semicolons (but two in a row is a syntax error per spec - we'll be lenient)
			continue
		}
		// If no semicolon and there's more input, that's an error
		if p.pos < len(p.input) {
			return nil, p.errorf("expected ';' between update operations, got %q", string(p.input[p.pos]))
		}
	}

	return u, nil
}

func (p *sparqlParser) parseUpdateOperation() (UpdateOperation, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, nil
	}

	if p.matchKeywordCI("INSERT") {
		p.pos += 6
		p.skipWS()
		if p.matchKeywordCI("DATA") {
			p.pos += 4
			return p.parseInsertData()
		}
		// INSERT { } WHERE { } (no WITH prefix)
		return p.parseModifyOp("", false, true)
	}

	if p.matchKeywordCI("DELETE") {
		p.pos += 6
		p.skipWS()
		if p.matchKeywordCI("DATA") {
			p.pos += 4
			return p.parseDeleteData()
		}
		if p.matchKeywordCI("WHERE") {
			p.pos += 5
			return p.parseDeleteWhere()
		}
		// DELETE { } [INSERT { }] WHERE { }
		return p.parseModifyOp("", true, false)
	}

	if p.matchKeywordCI("WITH") {
		p.pos += 4
		p.skipWS()
		graphIRI := p.readTermOrVar()
		// Resolve to full IRI
		if t := p.resolveTermValue(graphIRI); t != nil {
			if u, ok := t.(term.URIRef); ok {
				graphIRI = u.Value()
			}
		}
		p.skipWS()

		hasDelete := false
		hasInsert := false
		if p.matchKeywordCI("DELETE") {
			p.pos += 6
			hasDelete = true
			p.skipWS()
			if p.matchKeywordCI("WHERE") {
				// WITH <g> DELETE WHERE { }
				p.pos += 5
				return p.parseDeleteWhereWith(graphIRI)
			}
		} else if p.matchKeywordCI("INSERT") {
			p.pos += 6
			hasInsert = true
		}
		return p.parseModifyOp(graphIRI, hasDelete, hasInsert)
	}

	if p.matchKeywordCI("LOAD") {
		p.pos += 4
		return p.parseLoad()
	}
	if p.matchKeywordCI("CLEAR") {
		p.pos += 5
		return p.parseGraphMgmt("CLEAR")
	}
	if p.matchKeywordCI("DROP") {
		p.pos += 4
		return p.parseGraphMgmt("DROP")
	}
	if p.matchKeywordCI("CREATE") {
		p.pos += 6
		return p.parseCreate()
	}
	if p.matchKeywordCI("ADD") {
		p.pos += 3
		return p.parseTransfer("ADD")
	}
	if p.matchKeywordCI("MOVE") {
		p.pos += 4
		return p.parseTransfer("MOVE")
	}
	if p.matchKeywordCI("COPY") {
		p.pos += 4
		return p.parseTransfer("COPY")
	}

	return nil, p.errorf("expected UPDATE operation keyword")
}

func (p *sparqlParser) parseInsertData() (*InsertDataOp, error) {
	quads, err := p.parseQuadData(false)
	if err != nil {
		return nil, err
	}
	return &InsertDataOp{Quads: quads}, nil
}

func (p *sparqlParser) parseDeleteData() (*DeleteDataOp, error) {
	quads, err := p.parseQuadData(true)
	if err != nil {
		return nil, err
	}
	return &DeleteDataOp{Quads: quads}, nil
}

func (p *sparqlParser) parseDeleteWhere() (*DeleteWhereOp, error) {
	quads, err := p.parseQuadPattern()
	if err != nil {
		return nil, err
	}
	// BNodes not allowed in DELETE WHERE
	for _, qp := range quads {
		for _, t := range qp.Triples {
			if strings.HasPrefix(t.Subject, "_:") || strings.HasPrefix(t.Object, "_:") {
				return nil, p.errorf("blank node not allowed in DELETE WHERE")
			}
		}
	}
	return &DeleteWhereOp{Quads: quads}, nil
}

func (p *sparqlParser) parseDeleteWhereWith(graphIRI string) (*DeleteWhereOp, error) {
	quads, err := p.parseQuadPattern()
	if err != nil {
		return nil, err
	}
	// Apply WITH graph to default-graph quads
	for i := range quads {
		if quads[i].Graph == "" {
			quads[i].Graph = graphIRI
		}
	}
	return &DeleteWhereOp{Quads: quads}, nil
}

// parseQuadData parses { triple . GRAPH <g> { triple } } for INSERT DATA / DELETE DATA.
// isDelete=true means variables and bnodes in DELETE DATA are checked.
func (p *sparqlParser) parseQuadData(isDelete bool) ([]QuadPattern, error) {
	p.skipWS()
	if !p.expect('{') {
		return nil, p.errorf("expected '{' in quad data")
	}

	var quads []QuadPattern
	var defaultTriples []Triple

	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			return nil, p.errorf("unexpected end of input in quad data")
		}
		if p.input[p.pos] == '}' {
			p.pos++
			break
		}

		if p.matchKeywordCI("GRAPH") {
			// Flush default triples
			if len(defaultTriples) > 0 {
				quads = append(quads, QuadPattern{Triples: defaultTriples})
				defaultTriples = nil
			}
			p.pos += 5
			p.skipWS()
			graphTerm := p.readTermOrVar()
			if strings.HasPrefix(graphTerm, "?") {
				return nil, p.errorf("variable not allowed in %s DATA GRAPH", map[bool]string{true: "DELETE", false: "INSERT"}[isDelete])
			}
			// Resolve graph IRI
			graphIRI := graphTerm
			if t := p.resolveTermValue(graphTerm); t != nil {
				if u, ok := t.(term.URIRef); ok {
					graphIRI = u.Value()
				}
			}
			p.skipWS()
			if !p.expect('{') {
				return nil, p.errorf("expected '{' after GRAPH <iri>")
			}
			var triples []Triple
			for {
				p.skipWS()
				if p.pos >= len(p.input) || p.input[p.pos] == '}' {
					break
				}
				// Check for nested GRAPH (not allowed)
				if p.matchKeywordCI("GRAPH") {
					return nil, p.errorf("nested GRAPH not allowed in DATA block")
				}
				before := p.pos
				ts, err := p.parseTriplePatterns()
				if err != nil {
					return nil, err
				}
				if p.pos == before {
					return nil, p.errorf("unexpected token in quad data")
				}
				triples = append(triples, ts...)
			}
			p.skipWS()
			if !p.expect('}') {
				return nil, p.errorf("expected '}' after GRAPH triples")
			}
			for _, t := range triples {
				if isDelete {
					if strings.HasPrefix(t.Subject, "_:") || strings.HasPrefix(t.Object, "_:") {
						return nil, p.errorf("blank node not allowed in DELETE DATA")
					}
				}
				if strings.HasPrefix(t.Subject, "?") || strings.HasPrefix(t.Predicate, "?") || strings.HasPrefix(t.Object, "?") {
					return nil, p.errorf("variable not allowed in %s DATA", map[bool]string{true: "DELETE", false: "INSERT"}[isDelete])
				}
			}
			quads = append(quads, QuadPattern{Graph: graphIRI, Triples: triples})
			continue
		}

		// Parse default graph triples
		before := p.pos
		triples, err := p.parseTriplePatterns()
		if err != nil {
			return nil, err
		}
		if p.pos == before {
			return nil, p.errorf("unexpected token in quad data")
		}
		// Convert internal reifier/bnode variables to fresh blank node labels in DATA context
		convertInternalVarsToBnodes(triples)
		for _, t := range triples {
			if isDelete {
				if strings.HasPrefix(t.Subject, "_:") || strings.HasPrefix(t.Object, "_:") {
					return nil, p.errorf("blank node not allowed in DELETE DATA")
				}
			}
			if strings.HasPrefix(t.Subject, "?") || strings.HasPrefix(t.Predicate, "?") || strings.HasPrefix(t.Object, "?") {
				return nil, p.errorf("variable not allowed in %s DATA", map[bool]string{true: "DELETE", false: "INSERT"}[isDelete])
			}
		}
		defaultTriples = append(defaultTriples, triples...)
	}

	if len(defaultTriples) > 0 {
		quads = append(quads, QuadPattern{Triples: defaultTriples})
	}

	return quads, nil
}

// parseQuadPattern parses { triples . GRAPH <g> { triples } } for DELETE WHERE / templates.
func (p *sparqlParser) parseQuadPattern() ([]QuadPattern, error) {
	p.skipWS()
	if !p.expect('{') {
		return nil, p.errorf("expected '{'")
	}

	var quads []QuadPattern
	var defaultTriples []Triple

	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			return nil, p.errorf("unexpected end of input")
		}
		if p.input[p.pos] == '}' {
			p.pos++
			break
		}

		if p.matchKeywordCI("GRAPH") {
			if len(defaultTriples) > 0 {
				quads = append(quads, QuadPattern{Triples: defaultTriples})
				defaultTriples = nil
			}
			p.pos += 5
			p.skipWS()
			graphTerm := p.readTermOrVar()
			graphIRI := graphTerm
			if !strings.HasPrefix(graphTerm, "?") {
				if t := p.resolveTermValue(graphTerm); t != nil {
					if u, ok := t.(term.URIRef); ok {
						graphIRI = u.Value()
					}
				}
			}
			p.skipWS()
			if !p.expect('{') {
				return nil, p.errorf("expected '{' after GRAPH")
			}
			var triples []Triple
			for {
				p.skipWS()
				if p.pos >= len(p.input) || p.input[p.pos] == '}' {
					break
				}
				before := p.pos
				ts, err := p.parseTriplePatterns()
				if err != nil {
					return nil, err
				}
				if p.pos == before {
					return nil, p.errorf("unexpected token in quad template")
				}
				triples = append(triples, ts...)
			}
			p.skipWS()
			if !p.expect('}') {
				return nil, p.errorf("expected '}' after GRAPH triples")
			}
			quads = append(quads, QuadPattern{Graph: graphIRI, Triples: triples})
			continue
		}

		before := p.pos
		triples, err := p.parseTriplePatterns()
		if err != nil {
			return nil, err
		}
		if p.pos == before {
			return nil, p.errorf("unexpected token in quad template")
		}
		defaultTriples = append(defaultTriples, triples...)
	}

	if len(defaultTriples) > 0 {
		quads = append(quads, QuadPattern{Triples: defaultTriples})
	}

	return quads, nil
}

// parseModifyOp parses [DELETE { }] [INSERT { }] [USING ...] WHERE { }
// Called after WITH/DELETE/INSERT keyword has already been consumed.
func (p *sparqlParser) parseModifyOp(with string, hasDelete, hasInsert bool) (*ModifyOp, error) {
	op := &ModifyOp{With: with}

	if hasDelete {
		quads, err := p.parseQuadPattern()
		if err != nil {
			return nil, err
		}
		// Check for bnodes in DELETE template
		for _, qp := range quads {
			for _, t := range qp.Triples {
				if strings.HasPrefix(t.Subject, "_:") || strings.HasPrefix(t.Object, "_:") ||
					strings.HasPrefix(t.Subject, "?_bnode") || strings.HasPrefix(t.Object, "?_bnode") {
					return nil, p.errorf("blank node not allowed in DELETE template")
				}
			}
		}
		op.Delete = quads
		p.skipWS()
		if p.matchKeywordCI("INSERT") {
			p.pos += 6
			hasInsert = true
		}
	}

	if hasInsert {
		quads, err := p.parseQuadPattern()
		if err != nil {
			return nil, err
		}
		op.Insert = quads
	}

	// USING clauses
	for {
		p.skipWS()
		if !p.matchKeywordCI("USING") {
			break
		}
		p.pos += 5
		p.skipWS()
		named := false
		if p.matchKeywordCI("NAMED") {
			p.pos += 5
			named = true
			p.skipWS()
		}
		iriTerm := p.readTermOrVar()
		iri := iriTerm
		if t := p.resolveTermValue(iriTerm); t != nil {
			if u, ok := t.(term.URIRef); ok {
				iri = u.Value()
			}
		}
		op.Using = append(op.Using, UsingClause{IRI: iri, Named: named})
	}

	// WHERE clause
	p.skipWS()
	if !p.matchKeywordCI("WHERE") {
		return nil, p.errorf("expected WHERE in update operation")
	}
	p.pos += 5
	p.skipWS()

	pattern, err := p.parseGroupGraphPattern()
	if err != nil {
		return nil, err
	}
	op.Where = pattern

	return op, nil
}

func (p *sparqlParser) parseLoad() (*GraphMgmtOp, error) {
	p.skipWS()
	op := &GraphMgmtOp{Op: "LOAD"}
	if p.matchKeywordCI("SILENT") {
		p.pos += 6
		op.Silent = true
		p.skipWS()
	}
	// Source IRI
	src := p.readTermOrVar()
	if src == "" {
		return nil, p.errorf("expected IRI after LOAD")
	}
	if t := p.resolveTermValue(src); t != nil {
		if u, ok := t.(term.URIRef); ok {
			src = u.Value()
		}
	}
	op.Source = src

	p.skipWS()
	if p.matchKeywordCI("INTO") {
		p.pos += 4
		p.skipWS()
		if p.matchKeywordCI("GRAPH") {
			p.pos += 5
			p.skipWS()
		}
		dst := p.readTermOrVar()
		if t := p.resolveTermValue(dst); t != nil {
			if u, ok := t.(term.URIRef); ok {
				dst = u.Value()
			}
		}
		op.Into = dst
	}

	return op, nil
}

func (p *sparqlParser) parseGraphMgmt(opName string) (*GraphMgmtOp, error) {
	p.skipWS()
	op := &GraphMgmtOp{Op: opName}
	if p.matchKeywordCI("SILENT") {
		p.pos += 6
		op.Silent = true
		p.skipWS()
	}
	op.Target = p.parseGraphRef()
	return op, nil
}

func (p *sparqlParser) parseCreate() (*GraphMgmtOp, error) {
	p.skipWS()
	op := &GraphMgmtOp{Op: "CREATE"}
	if p.matchKeywordCI("SILENT") {
		p.pos += 6
		op.Silent = true
		p.skipWS()
	}
	if !p.matchKeywordCI("GRAPH") {
		return nil, p.errorf("expected GRAPH after CREATE")
	}
	p.pos += 5
	p.skipWS()
	iri := p.readTermOrVar()
	if t := p.resolveTermValue(iri); t != nil {
		if u, ok := t.(term.URIRef); ok {
			iri = u.Value()
		}
	}
	op.Target = iri
	return op, nil
}

func (p *sparqlParser) parseTransfer(opName string) (*GraphMgmtOp, error) {
	p.skipWS()
	op := &GraphMgmtOp{Op: opName}
	if p.matchKeywordCI("SILENT") {
		p.pos += 6
		op.Silent = true
		p.skipWS()
	}
	op.Source = p.parseGraphRefAll()
	p.skipWS()
	if p.matchKeywordCI("TO") {
		p.pos += 2
		p.skipWS()
	}
	op.Target = p.parseGraphRefAll()
	return op, nil
}

// parseGraphRef parses DEFAULT | NAMED | ALL | GRAPH <iri> | <iri>
func (p *sparqlParser) parseGraphRef() string {
	if p.matchKeywordCI("DEFAULT") {
		p.pos += 7
		return "DEFAULT"
	}
	if p.matchKeywordCI("NAMED") {
		p.pos += 5
		return "NAMED"
	}
	if p.matchKeywordCI("ALL") {
		p.pos += 3
		return "ALL"
	}
	if p.matchKeywordCI("GRAPH") {
		p.pos += 5
		p.skipWS()
	}
	iri := p.readTermOrVar()
	if t := p.resolveTermValue(iri); t != nil {
		if u, ok := t.(term.URIRef); ok {
			return u.Value()
		}
	}
	return iri
}

// parseGraphRefAll parses DEFAULT | NAMED | ALL | GRAPH <iri> | <iri>
func (p *sparqlParser) parseGraphRefAll() string {
	if p.matchKeywordCI("DEFAULT") {
		p.pos += 7
		return "DEFAULT"
	}
	if p.matchKeywordCI("NAMED") {
		p.pos += 5
		return "NAMED"
	}
	if p.matchKeywordCI("ALL") {
		p.pos += 3
		return "ALL"
	}
	if p.matchKeywordCI("GRAPH") {
		p.pos += 5
		p.skipWS()
	}
	iri := p.readTermOrVar()
	if t := p.resolveTermValue(iri); t != nil {
		if u, ok := t.(term.URIRef); ok {
			return u.Value()
		}
	}
	return iri
}
