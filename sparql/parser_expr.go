package sparql

import (
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// parseExpr parses a SPARQL expression (simplified: handles comparisons, booleans, function calls).
func (p *sparqlParser) parseExpr() (Expr, error) {
	p.skipWS()

	// Handle parenthesized expressions
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		p.pos++
		expr, err := p.parseOrExpr()
		if err != nil {
			return nil, err
		}
		p.skipWS()
		p.expect(')')
		return expr, nil
	}

	return p.parseOrExpr()
}

func (p *sparqlParser) parseOrExpr() (Expr, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWS()
		if p.pos+1 < len(p.input) && p.input[p.pos:p.pos+2] == "||" {
			p.pos += 2
			right, err := p.parseAndExpr()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: "||", Left: left, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *sparqlParser) parseAndExpr() (Expr, error) {
	left, err := p.parseCompareExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWS()
		if p.pos+1 < len(p.input) && p.input[p.pos:p.pos+2] == "&&" {
			p.pos += 2
			right, err := p.parseCompareExpr()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: "&&", Left: left, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *sparqlParser) parseCompareExpr() (Expr, error) {
	left, err := p.parseAddExpr()
	if err != nil {
		return nil, err
	}
	p.skipWS()
	for _, op := range []string{"!=", "<=", ">=", "=", "<", ">"} {
		if p.startsWith(op) {
			p.pos += len(op)
			right, err := p.parseAddExpr()
			if err != nil {
				return nil, err
			}
			return &BinaryExpr{Op: op, Left: left, Right: right}, nil
		}
	}

	// IN / NOT IN
	if p.matchKeywordCI("IN") {
		p.pos += 2
		p.skipWS()
		list, err := p.parseExprList()
		if err != nil {
			return nil, err
		}
		// Convert to OR chain of equality
		if len(list) == 0 {
			return &LiteralExpr{Value: rdflibgo.NewLiteral(false)}, nil
		}
		var result Expr = &BinaryExpr{Op: "=", Left: left, Right: list[0]}
		for _, item := range list[1:] {
			result = &BinaryExpr{Op: "||", Left: result, Right: &BinaryExpr{Op: "=", Left: left, Right: item}}
		}
		return result, nil
	}
	if p.matchKeywordCI("NOT") {
		saved := p.pos
		p.pos += 3
		p.skipWS()
		if p.matchKeywordCI("IN") {
			p.pos += 2
			p.skipWS()
			list, err := p.parseExprList()
			if err != nil {
				return nil, err
			}
			if len(list) == 0 {
				return &LiteralExpr{Value: rdflibgo.NewLiteral(true)}, nil
			}
			var result Expr = &BinaryExpr{Op: "!=", Left: left, Right: list[0]}
			for _, item := range list[1:] {
				result = &BinaryExpr{Op: "&&", Left: result, Right: &BinaryExpr{Op: "!=", Left: left, Right: item}}
			}
			return result, nil
		}
		p.pos = saved
	}

	return left, nil
}

func (p *sparqlParser) parseExprList() ([]Expr, error) {
	if !p.expect('(') {
		return nil, p.errorf("expected '('")
	}
	var list []Expr
	for {
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == ')' {
			p.pos++
			return list, nil
		}
		if len(list) > 0 {
			p.expect(',')
			p.skipWS()
		}
		expr, err := p.parseOrExpr()
		if err != nil {
			return nil, err
		}
		list = append(list, expr)
	}
}

func (p *sparqlParser) parseAddExpr() (Expr, error) {
	left, err := p.parseMulExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWS()
		if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
			op := string(p.input[p.pos])
			p.pos++
			right, err := p.parseMulExpr()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: op, Left: left, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *sparqlParser) parseMulExpr() (Expr, error) {
	left, err := p.parseUnaryExpr()
	if err != nil {
		return nil, err
	}
	for {
		p.skipWS()
		if p.pos < len(p.input) && (p.input[p.pos] == '*' || p.input[p.pos] == '/') {
			op := string(p.input[p.pos])
			p.pos++
			right, err := p.parseUnaryExpr()
			if err != nil {
				return nil, err
			}
			left = &BinaryExpr{Op: op, Left: left, Right: right}
		} else {
			break
		}
	}
	return left, nil
}

func (p *sparqlParser) parseUnaryExpr() (Expr, error) {
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '!' {
		// Check for ! EXISTS (same as NOT EXISTS)
		saved := p.pos
		p.pos++
		p.skipWS()
		if p.matchKeywordCI("EXISTS") {
			p.pos += 6
			p.skipWS()
			pat, err := p.parseGroupGraphPattern()
			if err != nil {
				return nil, err
			}
			return &ExistsExpr{Pattern: pat, Not: true}, nil
		}
		p.pos = saved + 1
		arg, err := p.parseUnaryExpr()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "!", Arg: arg}, nil
	}
	if p.pos < len(p.input) && p.input[p.pos] == '-' {
		p.pos++
		arg, err := p.parseUnaryExpr()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{Op: "-", Arg: arg}, nil
	}
	return p.parsePrimaryExpr()
}

func (p *sparqlParser) parsePrimaryExpr() (Expr, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of expression")
	}

	ch := p.input[p.pos]

	// Parenthesized
	if ch == '(' {
		p.pos++
		expr, err := p.parseOrExpr()
		if err != nil {
			return nil, err
		}
		p.skipWS()
		p.expect(')')
		return expr, nil
	}

	// Variable
	if ch == '?' || ch == '$' {
		v := p.readVar()
		return &VarExpr{Name: v}, nil
	}

	// String literal
	if ch == '"' || ch == '\'' {
		t := p.readTermOrVar()
		if p.tripleTermError != nil {
			return nil, p.tripleTermError
		}
		if err := validateStringEscapes(t); err != nil {
			return nil, p.errorf("%s", err)
		}
		return &LiteralExpr{Value: p.resolveTermValue(t)}, nil
	}

	// Numeric
	if ch >= '0' && ch <= '9' || ch == '.' {
		t := p.readTermOrVar()
		return &LiteralExpr{Value: p.resolveTermValue(t)}, nil
	}

	// IRI or triple term
	if ch == '<' {
		if p.pos+1 < len(p.input) && p.input[p.pos+1] == '<' {
			saved := p.pos
			p.pos += 2
			p.skipWS()
			// Check if it's a triple term <<( or a reified triple <<
			if p.pos < len(p.input) && p.input[p.pos] == '(' {
				p.pos++ // skip (
				p.skipWS()
				// Parse inner S, P, O as expressions
				sExpr, err := p.parseExprOrTermInTriple()
				if err != nil {
					return nil, err
				}
				// Validate: subject must not be a literal constant or nested triple term
				if le, ok := sExpr.(*LiteralExpr); ok && le.Value != nil {
					if _, isLit := le.Value.(rdflibgo.Literal); isLit {
						return nil, p.errorf("literal in subject position of triple term")
					}
				}
				if fe, ok := sExpr.(*FuncExpr); ok && fe.Name == "TRIPLE" {
					return nil, p.errorf("triple term in subject position of triple term")
				}
				p.skipWS()
				pExpr, err := p.parseExprOrTermInTriple()
				if err != nil {
					return nil, err
				}
				p.skipWS()
				oExpr, err := p.parseExprOrTermInTriple()
				if err != nil {
					return nil, err
				}
				p.skipWS()
				if p.pos < len(p.input) && p.input[p.pos] == ')' {
					p.pos++
				}
				p.skipWS()
				if p.pos+1 < len(p.input) && p.input[p.pos] == '>' && p.input[p.pos+1] == '>' {
					p.pos += 2
				}
				return &FuncExpr{Name: "TRIPLE", Args: []Expr{sExpr, pExpr, oExpr}}, nil
			}
			// Reified triple << s p o >> in expression context — not allowed
			p.pos = saved
			return nil, p.errorf("reified triple syntax << ... >> not allowed in expressions, use <<( ... )>> for triple terms")
		}
		iri := p.readIRIRef()
		return &IRIExpr{Value: iri}, nil
	}

	// Boolean or function call
	if p.matchKeywordCI("true") && (p.pos+4 >= len(p.input) || !isNameChar(rune(p.input[p.pos+4]))) {
		p.pos += 4
		return &LiteralExpr{Value: rdflibgo.NewLiteral(true)}, nil
	}
	if p.matchKeywordCI("false") && (p.pos+5 >= len(p.input) || !isNameChar(rune(p.input[p.pos+5]))) {
		p.pos += 5
		return &LiteralExpr{Value: rdflibgo.NewLiteral(false)}, nil
	}

	// EXISTS / NOT EXISTS
	if p.matchKeywordCI("EXISTS") {
		p.pos += 6
		p.skipWS()
		pat, err := p.parseGroupGraphPattern()
		if err != nil {
			return nil, err
		}
		return &ExistsExpr{Pattern: pat, Not: false}, nil
	}
	if p.matchKeywordCI("NOT") {
		saved := p.pos
		p.pos += 3
		p.skipWS()
		if p.matchKeywordCI("EXISTS") {
			p.pos += 6
			p.skipWS()
			pat, err := p.parseGroupGraphPattern()
			if err != nil {
				return nil, err
			}
			return &ExistsExpr{Pattern: pat, Not: true}, nil
		}
		p.pos = saved
	}

	// Built-in function or prefixed name
	name := p.readFuncName()
	if name == "" {
		return nil, p.errorf("unexpected character in expression: %c", ch)
	}

	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '(' {
		upperName := strings.ToUpper(name)
		// Resolve prefixed function names (e.g. xsd:integer -> full IRI)
		if idx := strings.Index(name, ":"); idx >= 0 {
			prefix := name[:idx]
			local := name[idx+1:]
			if ns, ok := p.prefixes[prefix]; ok {
				upperName = strings.ToUpper(ns + local)
			}
		}
		if isAggregateName(upperName) {
			return p.parseAggregateCall(upperName)
		}
		p.pos++
		var args []Expr
		for {
			p.skipWS()
			if p.pos < len(p.input) && p.input[p.pos] == ')' {
				p.pos++
				break
			}
			if len(args) > 0 {
				if !p.expect(',') {
					p.skipWS()
				}
			}
			arg, err := p.parseOrExpr()
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
		}
		return &FuncExpr{Name: upperName, Args: args}, nil
	}

	// It's a prefixed name used as a value
	resolved := p.resolveTermValue(name)
	if u, ok := resolved.(rdflibgo.URIRef); ok {
		return &IRIExpr{Value: u.Value()}, nil
	}
	return &LiteralExpr{Value: resolved}, nil
}

func isAggregateName(name string) bool {
	switch name {
	case "COUNT", "SUM", "AVG", "MIN", "MAX", "GROUP_CONCAT", "SAMPLE":
		return true
	}
	return false
}

func (p *sparqlParser) parseAggregateCall(name string) (Expr, error) {
	p.pos++ // skip '('
	p.skipWS()

	fe := &FuncExpr{Name: name}

	// COUNT(*)
	if name == "COUNT" && p.pos < len(p.input) && p.input[p.pos] == '*' {
		p.pos++
		fe.Star = true
		p.skipWS()
		p.expect(')')
		return fe, nil
	}

	// DISTINCT
	if p.matchKeywordCI("DISTINCT") {
		p.pos += 8
		p.skipWS()
		fe.Distinct = true
	}

	// COUNT(DISTINCT *)
	if name == "COUNT" && p.pos < len(p.input) && p.input[p.pos] == '*' {
		p.pos++
		fe.Star = true
		p.skipWS()
		p.expect(')')
		return fe, nil
	}

	// Parse argument
	if p.pos < len(p.input) && p.input[p.pos] != ')' {
		arg, err := p.parseOrExpr()
		if err != nil {
			return nil, err
		}
		fe.Args = append(fe.Args, arg)
	}

	// GROUP_CONCAT separator
	if name == "GROUP_CONCAT" {
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == ';' {
			p.pos++
			p.skipWS()
			if p.matchKeywordCI("SEPARATOR") {
				p.pos += 9
				p.skipWS()
				p.expect('=')
				p.skipWS()
				sep := p.readTermOrVar()
				fe.Separator = string(p.resolveTermValue(sep).(rdflibgo.Literal).Lexical())
			}
		} else {
			fe.Separator = " " // default separator
		}
	}

	p.skipWS()
	p.expect(')')
	return fe, nil
}

// parseExprOrTermInTriple parses a single term inside a triple term in expression context.
// Handles variables, IRIs, literals, prefixed names, and nested triple terms.
func (p *sparqlParser) parseExprOrTermInTriple() (Expr, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end in triple term expression")
	}
	ch := p.input[p.pos]
	if ch == '?' || ch == '$' {
		v := p.readVar()
		return &VarExpr{Name: v}, nil
	}
	if ch == '<' {
		if p.pos+1 < len(p.input) && p.input[p.pos+1] == '<' {
			// Nested triple term
			return p.parsePrimaryExpr()
		}
		iri := p.readIRIRef()
		return &IRIExpr{Value: iri}, nil
	}
	if ch == '"' || ch == '\'' {
		t := p.readTermOrVar()
		return &LiteralExpr{Value: p.resolveTermValue(t)}, nil
	}
	// Bnode property lists and collections not allowed in triple term expressions
	if ch == '[' || ch == '(' {
		return nil, p.errorf("bnode property list or collection not allowed in triple term expression")
	}
	// Blank node _:label
	if ch == '_' && p.pos+1 < len(p.input) && p.input[p.pos+1] == ':' {
		t := p.readTermOrVar()
		return &LiteralExpr{Value: p.resolveTermValue(t)}, nil
	}
	// Numeric, boolean, prefixed name
	t := p.readTermOrVar()
	if t == "" {
		return nil, p.errorf("unexpected token in triple term")
	}
	val := p.resolveTermValue(t)
	if val == nil {
		return &LiteralExpr{Value: rdflibgo.NewLiteral(t)}, nil
	}
	if u, ok := val.(rdflibgo.URIRef); ok {
		return &IRIExpr{Value: u.Value()}, nil
	}
	return &LiteralExpr{Value: val}, nil
}
