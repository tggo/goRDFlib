package sparql

import "fmt"

// parseSelect parses SELECT DISTINCT/REDUCED and variable list.
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
				if !p.matchKeywordCI("AS") {
					return p.errorf("expected AS in SELECT expression")
				}
				p.pos += 2
				p.skipWS()
				v := p.readVar()
				q.Variables = append(q.Variables, v)
				q.ProjectExprs = append(q.ProjectExprs, ProjectExpr{Expr: expr, Var: v})
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

	rdfReifies := "<http://www.w3.org/1999/02/22-rdf-syntax-ns#reifies>"

	// Helper to flush reifier triples into construct template
	flushReifiers := func() {
		for _, rt := range p.reifierTriples {
			q.Construct = append(q.Construct, TripleTemplate{Subject: rt.Subject, Predicate: rt.Predicate, Object: rt.Object})
		}
		p.reifierTriples = nil
	}

	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			return p.errorf("unterminated CONSTRUCT template")
		}
		if p.input[p.pos] == '}' {
			p.pos++
			break
		}
		// Collection/list syntax in CONSTRUCT template
		if p.input[p.pos] == '(' {
			head, collTriples, err := p.parseCollectionTriples()
			if err != nil {
				return err
			}
			for _, ct := range collTriples {
				q.Construct = append(q.Construct, TripleTemplate{Subject: ct.Subject, Predicate: ct.Predicate, Object: ct.Object})
			}
			// Now read predicate and object for the collection head
			p.skipWS()
			pred := p.readTermOrVar()
			p.skipWS()
			obj := p.readTermOrVar()
			flushReifiers()
			q.Construct = append(q.Construct, TripleTemplate{Subject: head, Predicate: pred, Object: obj})
			p.skipWS()
			if p.pos < len(p.input) && p.input[p.pos] == '.' {
				p.pos++
			}
			continue
		}
		s := p.readTermOrVar()
		flushReifiers()
		if s == "" {
			return p.errorf("unexpected token in CONSTRUCT template")
		}
		p.skipWS()
		pred := p.readTermOrVar()
		p.skipWS()
		// Object list (,) and predicate-object list (;)
		for {
			obj := p.readTermOrVar()
			flushReifiers()
			q.Construct = append(q.Construct, TripleTemplate{Subject: s, Predicate: pred, Object: obj})
			p.skipWS()
			// Check for annotation/reifier syntax after object in CONSTRUCT
			p.parseConstructAnnotations(q, s, pred, obj, rdfReifies)
			p.skipWS()
			if p.pos < len(p.input) && p.input[p.pos] == ',' {
				p.pos++
				p.skipWS()
				continue
			}
			break
		}
		p.skipWS()
		for p.pos < len(p.input) && p.input[p.pos] == ';' {
			p.pos++
			p.skipWS()
			if p.pos >= len(p.input) || p.input[p.pos] == '.' || p.input[p.pos] == '}' {
				break
			}
			pred = p.readTermOrVar()
			p.skipWS()
			for {
				obj := p.readTermOrVar()
				flushReifiers()
				q.Construct = append(q.Construct, TripleTemplate{Subject: s, Predicate: pred, Object: obj})
				p.skipWS()
				p.parseConstructAnnotations(q, s, pred, obj, rdfReifies)
				p.skipWS()
				if p.pos < len(p.input) && p.input[p.pos] == ',' {
					p.pos++
					p.skipWS()
					continue
				}
				break
			}
			p.skipWS()
		}
		if p.pos < len(p.input) && p.input[p.pos] == '.' {
			p.pos++
		}
	}
	return nil
}

// parseConstructAnnotations handles ~ reifier and {| annotation |} in CONSTRUCT templates.
func (p *sparqlParser) parseConstructAnnotations(q *ParsedQuery, s, pred, obj, rdfReifies string) {
	tripleTermStr := "<<( " + s + " " + pred + " " + obj + " )>>"

	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			break
		}

		// ~ reifier
		if p.input[p.pos] == '~' {
			p.pos++
			p.skipWS()
			reifierID := p.readTermOrVar()
			// Flush any nested reifier triples
			for _, rt := range p.reifierTriples {
				q.Construct = append(q.Construct, TripleTemplate{Subject: rt.Subject, Predicate: rt.Predicate, Object: rt.Object})
			}
			p.reifierTriples = nil
			q.Construct = append(q.Construct, TripleTemplate{
				Subject:   reifierID,
				Predicate: rdfReifies,
				Object:    tripleTermStr,
			})

			p.skipWS()
			if p.pos+1 < len(p.input) && p.input[p.pos] == '{' && p.input[p.pos+1] == '|' {
				p.pos += 2
				p.parseConstructAnnotationBlock(q, reifierID)
			}
			continue
		}

		// {| annotation |}
		if p.pos+1 < len(p.input) && p.input[p.pos] == '{' && p.input[p.pos+1] == '|' {
			p.pos += 2
			p.bnodeCount++
			reifierID := fmt.Sprintf("?_reifier%d", p.bnodeCount)
			q.Construct = append(q.Construct, TripleTemplate{
				Subject:   reifierID,
				Predicate: rdfReifies,
				Object:    tripleTermStr,
			})
			p.parseConstructAnnotationBlock(q, reifierID)
			continue
		}

		break
	}
}

// parseConstructAnnotationBlock parses {| pred obj [; pred obj]* |} inside CONSTRUCT.
func (p *sparqlParser) parseConstructAnnotationBlock(q *ParsedQuery, reifierID string) {
	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			break
		}
		if p.pos+1 < len(p.input) && p.input[p.pos] == '|' && p.input[p.pos+1] == '}' {
			p.pos += 2
			break
		}

		pred := p.readTermOrVar()
		if pred == "" {
			break
		}
		p.skipWS()

		for {
			obj := p.readTermOrVar()
			for _, rt := range p.reifierTriples {
				q.Construct = append(q.Construct, TripleTemplate{Subject: rt.Subject, Predicate: rt.Predicate, Object: rt.Object})
			}
			p.reifierTriples = nil
			if obj == "" {
				break
			}
			q.Construct = append(q.Construct, TripleTemplate{Subject: reifierID, Predicate: pred, Object: obj})
			p.skipWS()
			if p.pos < len(p.input) && p.input[p.pos] == ',' {
				p.pos++
				continue
			}
			break
		}
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == ';' {
			p.pos++
			continue
		}
	}
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
					before := p.pos
					expr, err := p.parseExpr()
					if err != nil {
						break
					}
					if p.pos == before {
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
				before := p.pos
				expr, err := p.parseExpr()
				if err != nil {
					break
				}
				if p.pos == before {
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

		before := p.pos
		expr, err := p.parseExpr()
		if err != nil {
			break
		}
		if p.pos == before {
			break
		}
		q.OrderBy = append(q.OrderBy, OrderExpr{Expr: expr, Desc: desc})
	}
	return nil
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
