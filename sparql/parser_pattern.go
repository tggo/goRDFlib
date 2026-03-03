package sparql

import (
	rdflibgo "github.com/tggo/goRDFlib"
)

func (p *sparqlParser) parseGroupGraphPattern() (Pattern, error) {
	p.skipWS()
	if p.pos >= len(p.input) || p.input[p.pos] != '{' {
		return nil, p.errorf("expected '{'")
	}
	p.pos++

	var result Pattern
	var currentTriples []Triple
	var deferredFilters []Expr // FILTER has whole-group scope, applied at end

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
			// Apply deferred FILTERs (whole-group scope)
			for _, fexpr := range deferredFilters {
				result = &FilterPattern{Pattern: result, Expr: fexpr}
			}
			return result, nil
		}

		// Subquery: SELECT inside group graph pattern
		if p.matchKeywordCI("SELECT") {
			flushTriples()
			// Check for invalid empty group before subquery: { {} SELECT ... }
			if result != nil {
				if bgp, ok := result.(*BGP); ok && len(bgp.Triples) == 0 {
					return nil, p.errorf("empty group pattern before subquery is not allowed")
				}
			}
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
			// Chain multiple UNIONs: { A } UNION { B } UNION { C } → UNION(UNION(A, B), C)
			for p.matchKeywordCI("UNION") {
				p.pos += 5
				p.skipWS()
				right, err := p.parseGroupGraphPattern()
				if err != nil {
					return nil, err
				}
				left = &UnionPattern{Left: left, Right: right}
				p.skipWS()
			}
			if result == nil {
				result = left
			} else {
				result = &JoinPattern{Left: result, Right: left}
			}
			continue
		}

		// FILTER (deferred — SPARQL FILTER has whole-group scope)
		if p.matchKeywordCI("FILTER") {
			p.pos += 6
			flushTriples()
			p.skipWS()
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			deferredFilters = append(deferredFilters, expr)
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

		// GRAPH clause
		if p.matchKeywordCI("GRAPH") {
			p.pos += 5
			flushTriples()
			p.skipWS()
			graphName := p.readTermOrVar()
			p.skipWS()
			sub, err := p.parseGroupGraphPattern()
			if err != nil {
				return nil, err
			}
			gp := &GraphPattern{Name: graphName, Pattern: sub}
			if result == nil {
				result = gp
			} else {
				result = &JoinPattern{Left: result, Right: gp}
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

func (p *sparqlParser) parseTriplePatterns() ([]Triple, error) {
	var triples []Triple
	p.skipWS()
	// Handle [ pred obj ; ... ] or ( term1 term2 ... ) as subject
	var subj string
	if p.pos < len(p.input) && p.input[p.pos] == '[' {
		bnode, extraTriples, err := p.parseBnodePropertyListTriples()
		if err != nil {
			return nil, err
		}
		subj = bnode
		triples = append(triples, extraTriples...)
	} else if p.pos < len(p.input) && p.input[p.pos] == '(' {
		head, extraTriples, err := p.parseCollectionTriples()
		if err != nil {
			return nil, err
		}
		subj = head
		triples = append(triples, extraTriples...)
	} else {
		subj = p.readTermOrVar()
	}
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
			p.skipWS()
			var obj string
			if p.pos < len(p.input) && p.input[p.pos] == '[' {
				bnode, extraTriples, err := p.parseBnodePropertyListTriples()
				if err != nil {
					return nil, err
				}
				obj = bnode
				triples = append(triples, extraTriples...)
			} else if p.pos < len(p.input) && p.input[p.pos] == '(' {
				head, extraTriples, err := p.parseCollectionTriples()
				if err != nil {
					return nil, err
				}
				obj = head
				triples = append(triples, extraTriples...)
			} else {
				obj = p.readTermOrVar()
			}
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

	// Validate: each row must have the same number of values as variables
	for _, row := range values {
		if len(row) != len(vars) {
			return nil, p.errorf("wrong number of values in VALUES clause: expected %d, got %d", len(vars), len(row))
		}
	}

	return &ValuesPattern{Vars: vars, Values: values}, nil
}
