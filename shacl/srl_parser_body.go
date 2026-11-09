package shacl

import (
	"fmt"
	"regexp"
	"strings"
)

// bodyCtx says which elements a body block may hold.
type bodyCtx int

const (
	bodyRule  bodyCtx = iota // FILTER, NOT, SET, GROUP
	bodyNot                  // FILTER only (NegationPatternNotTriples)
	bodyGroup                // FILTER, SET (GroupPatternNotTriples)
)

// startsBodyElement reports whether tok begins a BodyNotTriples element.
func startsBodyElement(k srlTokenKind) bool {
	return k == srlTokFilter || k == srlTokNot || k == srlTokSet || k == srlTokBind || k == srlTokGroup
}

// parseBody parses the elements of a rule body, negation or group pattern up
// to (not including) the closing brace:
//
//	BodyTriplesBlock? ( BodyNotTriples '.'? BodyTriplesBlock? )*
func (p *srlParser) parseBody(ctx bodyCtx) ([]SRLBodyElement, error) {
	var elems []SRLBodyElement
	afterTriples := false // a triples block just ended without a '.'
	for {
		tok, err := p.peek()
		if err != nil {
			return nil, err
		}
		switch {
		case tok.kind == srlTokRBrace:
			return elems, nil
		case startsBodyElement(tok.kind):
			elem, err := p.parseBodyElement(ctx, tok)
			if err != nil {
				return nil, err
			}
			elems = append(elems, elem)
			if _, err := p.accept(srlTokDot); err != nil {
				return nil, err
			}
			afterTriples = false
		default:
			if afterTriples {
				return nil, p.unexpected(tok, "'.', a FILTER, NOT, SET or GROUP, or '}'")
			}
			triples, err := p.parseTriplesBlock(srlPattern)
			if err != nil {
				return nil, err
			}
			for _, t := range triples {
				elems = append(elems, SRLBodyElement{Kind: SRLBodyTriple, Triple: t})
			}
			afterTriples = true
			// parseTriplesBlock consumed a trailing '.', which also ends the
			// block; either way the next token decides.
			if p.lastDot {
				afterTriples = false
			}
		}
	}
}

func (p *srlParser) parseBodyElement(ctx bodyCtx, tok srlToken) (SRLBodyElement, error) {
	switch tok.kind {
	case srlTokFilter:
		p.next()
		expr, err := p.lexer.scanConstraint()
		if err != nil {
			return SRLBodyElement{}, err
		}
		if err := checkExpr(p.currentPrologue(), "FILTER %s", expr); err != nil {
			return SRLBodyElement{}, fmt.Errorf("%w (FILTER at position %d)", err, tok.pos)
		}
		return SRLBodyElement{Kind: SRLBodyFilter, FilterExpr: expr}, nil
	case srlTokNot:
		if ctx != bodyRule {
			return SRLBodyElement{}, fmt.Errorf("NOT is not allowed here (position %d): only a rule body can hold a negation", tok.pos)
		}
		return p.parseNot()
	case srlTokSet, srlTokBind:
		if ctx == bodyNot {
			return SRLBodyElement{}, fmt.Errorf("%s is not allowed in a negation (position %d)", tok.val, tok.pos)
		}
		if tok.kind == srlTokBind {
			return p.parseLegacyBind()
		}
		return p.parseSet()
	default: // srlTokGroup
		if ctx != bodyRule {
			return SRLBodyElement{}, fmt.Errorf("GROUP is not allowed here (position %d): groups do not nest", tok.pos)
		}
		return p.parseGroup()
	}
}

// parseNot parses NOT DATA? { NegationPattern }.
func (p *srlParser) parseNot() (SRLBodyElement, error) {
	p.next() // NOT
	elem := SRLBodyElement{Kind: SRLBodyNot}
	var err error
	if elem.NotData, err = p.accept(srlTokData); err != nil {
		return elem, err
	}
	if _, err := p.expect(srlTokLBrace, "'{' after NOT"); err != nil {
		return elem, err
	}
	if elem.NotBody, err = p.parseBody(bodyNot); err != nil {
		return elem, err
	}
	if _, err := p.expect(srlTokRBrace, "'}' ending the negation"); err != nil {
		return elem, err
	}
	return elem, nil
}

// parseSet parses SET ( ?var := Expression ).
func (p *srlParser) parseSet() (SRLBodyElement, error) {
	setTok, _ := p.next() // SET
	if _, err := p.expect(srlTokLParen, "'(' after SET"); err != nil {
		return SRLBodyElement{}, err
	}
	v, err := p.expect(srlTokVar, "a variable in SET")
	if err != nil {
		return SRLBodyElement{}, err
	}
	if _, err := p.expect(srlTokAssign, "':=' after the SET variable"); err != nil {
		return SRLBodyElement{}, err
	}
	expr, err := p.lexer.scanAssignedExpr()
	if err != nil {
		return SRLBodyElement{}, err
	}
	if err := checkExpr(p.currentPrologue(), "BIND ( %s AS ?srlcheck )", expr); err != nil {
		return SRLBodyElement{}, fmt.Errorf("%w (SET at position %d)", err, setTok.pos)
	}
	return SRLBodyElement{Kind: SRLBodyBind, BindVar: v.val, BindExpr: expr}, nil
}

// parseGroup parses GROUP ( BY Var+ )? { GroupPattern } ( ANON | '[' aggregations ']' ).
func (p *srlParser) parseGroup() (SRLBodyElement, error) {
	p.next() // GROUP
	elem := SRLBodyElement{Kind: SRLBodyGroup}
	by, err := p.accept(srlTokBy)
	if err != nil {
		return elem, err
	}
	if by {
		for {
			tok, err := p.peek()
			if err != nil {
				return elem, err
			}
			if tok.kind != srlTokVar {
				break
			}
			p.next()
			elem.GroupKeys = append(elem.GroupKeys, tok.val)
		}
		if len(elem.GroupKeys) == 0 {
			tok, _ := p.peek()
			return elem, p.unexpected(tok, "at least one variable after GROUP BY")
		}
	}
	if _, err := p.expect(srlTokLBrace, "'{' starting the group pattern"); err != nil {
		return elem, err
	}
	if elem.GroupBody, err = p.parseBody(bodyGroup); err != nil {
		return elem, err
	}
	if _, err := p.expect(srlTokRBrace, "'}' ending the group pattern"); err != nil {
		return elem, err
	}
	if _, err := p.expect(srlTokLBrack, "'[' (or '[]') after the group pattern"); err != nil {
		return elem, err
	}
	if elem.GroupAggs, err = p.parseAggregations(); err != nil {
		return elem, err
	}
	return elem, nil
}

// parseAggregations parses the inside of [ ... ] after a group pattern and
// the closing bracket.
func (p *srlParser) parseAggregations() ([]SRLAggregation, error) {
	var aggs []SRLAggregation
	closed, err := p.accept(srlTokRBrack)
	if err != nil || closed {
		return nil, err
	}
	for {
		v, err := p.expect(srlTokVar, "an aggregation variable")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(srlTokAssign, "':=' after the aggregation variable"); err != nil {
			return nil, err
		}
		fn, err := p.expect(srlTokAgg, "COUNT, MIN, MAX or SUM")
		if err != nil {
			return nil, err
		}
		agg := SRLAggregation{Var: v.val, Func: fn.val}
		if _, err := p.expect(srlTokLParen, "'(' after "+fn.val); err != nil {
			return nil, err
		}
		if fn.val == "COUNT" {
			if agg.Distinct, err = p.accept(srlTokDistinct); err != nil {
				return nil, err
			}
		}
		arg, err := p.next()
		if err != nil {
			return nil, err
		}
		switch {
		case arg.kind == srlTokVar:
			agg.Arg = arg.val
		case arg.kind == srlTokStar && fn.val == "COUNT":
			agg.Arg = "*"
		default:
			return nil, p.unexpected(arg, "a variable"+map[bool]string{true: " or '*'", false: ""}[fn.val == "COUNT"]+" as the aggregation argument")
		}
		if _, err := p.expect(srlTokRParen, "')' closing "+fn.val); err != nil {
			return nil, err
		}
		aggs = append(aggs, agg)
		tok, err := p.next()
		if err != nil {
			return nil, err
		}
		switch tok.kind {
		case srlTokComma:
		case srlTokRBrack:
			return aggs, nil
		default:
			return nil, p.unexpected(tok, "',' or ']' in the aggregations")
		}
	}
}

// parseLegacyBind parses BIND ( expr AS ?var ), the spelling of the assignment
// element before SPARQL-RL's SET ( ?var := expr ). It is accepted so rule
// files written for the earlier SHACL Rules syntax keep working, and means the
// same as SET.
func (p *srlParser) parseLegacyBind() (SRLBodyElement, error) {
	bindTok, _ := p.next() // the keyword
	if _, err := p.expect(srlTokLParen, "'(' after "+bindTok.val); err != nil {
		return SRLBodyElement{}, err
	}
	inner, err := p.lexer.scanAssignedExpr()
	if err != nil {
		return SRLBodyElement{}, err
	}
	m := legacyBindRe.FindStringSubmatchIndex(blankLiterals(inner))
	if m == nil {
		return SRLBodyElement{}, fmt.Errorf("%s at position %d must end with AS ?variable", bindTok.val, bindTok.pos)
	}
	expr := strings.TrimSpace(inner[:m[0]])
	v := inner[m[2]:m[3]]
	if expr == "" {
		return SRLBodyElement{}, fmt.Errorf("empty expression in %s at position %d", bindTok.val, bindTok.pos)
	}
	if err := checkExpr(p.currentPrologue(), "BIND ( %s AS ?srlcheck )", expr); err != nil {
		return SRLBodyElement{}, fmt.Errorf("%w (%s at position %d)", err, bindTok.val, bindTok.pos)
	}
	return SRLBodyElement{Kind: SRLBodyBind, BindVar: v, BindExpr: expr}, nil
}

var legacyBindRe = regexp.MustCompile(`(?i)\s+AS\s+[?$]([\p{L}\p{N}_]+)\s*$`)
