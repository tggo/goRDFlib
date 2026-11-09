package shacl

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors of the SPARQL-RL (SRL) processor. Each is wrapped with the
// position or rule that caused it, so test with errors.Is.
var (
	// ErrSRLSyntax: the document does not match the SPARQL-RL grammar.
	// The message carries the offset of the offending token.
	ErrSRLSyntax = errors.New("shacl: SPARQL-RL syntax error")
	// ErrSRLNotWellFormed: a rule breaks a well-formedness condition (a head
	// variable the body never binds, a FILTER on an unbound variable, an
	// assignment to a variable already in use). Fix the named rule.
	ErrSRLNotWellFormed = errors.New("shacl: SPARQL-RL rule is not well-formed")
	// ErrSRLNotStratifiable: a recursive dependency passes through a negation,
	// a group, a rule that creates blank nodes or triple terms, or an
	// assignment. Break the cycle named in the message, or move the negated
	// pattern to NOT DATA / WHERE DATA if it only needs the input graph.
	ErrSRLNotStratifiable = errors.New("shacl: SPARQL-RL rule set is not stratifiable")
	// ErrSRLImports: the rule set has IMPORTS, which this processor does not
	// resolve (SPARQL-RL makes imports optional and requires rejecting them).
	// Merge the imported rule sets into the document.
	ErrSRLImports = errors.New("shacl: SPARQL-RL IMPORTS are not supported")
)

// ParseSRL parses a SPARQL-RL document into a RuleSet. It checks the grammar
// only; CheckWellformed and Stratify check the conditions on top of it.
func ParseSRL(input string) (*RuleSet, error) {
	p := &srlParser{
		lexer:    newSRLLexer(input),
		prefixes: make(map[string]string),
	}
	rs, err := p.parse()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSRLSyntax, err.Error())
	}
	return rs, nil
}

type srlParser struct {
	lexer    *srlLexer
	prefixes map[string]string
	base     string
	bnodeID  int
	peeked   *srlToken

	// prologue is the SPARQL BASE/PREFIX text of the declarations seen so
	// far, for checking and evaluating expressions; "" when stale.
	prologue string

	// lastDot: the triples block parsed last ended with a '.'.
	lastDot bool
}

func (p *srlParser) peek() (srlToken, error) {
	if p.peeked != nil {
		return *p.peeked, nil
	}
	tok, err := p.lexer.next()
	if err != nil {
		return tok, err
	}
	p.peeked = &tok
	return tok, nil
}

func (p *srlParser) next() (srlToken, error) {
	if p.peeked != nil {
		tok := *p.peeked
		p.peeked = nil
		return tok, nil
	}
	return p.lexer.next()
}

// accept consumes the next token if it has the given kind.
func (p *srlParser) accept(kind srlTokenKind) (bool, error) {
	tok, err := p.peek()
	if err != nil || tok.kind != kind {
		return false, err
	}
	p.peeked = nil
	return true, nil
}

func (p *srlParser) expect(kind srlTokenKind, what string) (srlToken, error) {
	tok, err := p.next()
	if err != nil {
		return tok, err
	}
	if tok.kind != kind {
		return tok, p.unexpected(tok, what)
	}
	return tok, nil
}

func (p *srlParser) unexpected(tok srlToken, expected string) error {
	got := tok.val
	if tok.kind == srlTokEOF {
		got = "end of input"
	} else if got == "" {
		got = srlTokenText(p.lexer.input, tok.pos)
	}
	return fmt.Errorf("expected %s, got %q at position %d", expected, got, tok.pos)
}

// srlTokenText returns a short snippet of the input at pos, for messages about
// tokens that carry no value.
func srlTokenText(in string, pos int) string {
	if pos >= len(in) {
		return ""
	}
	end := pos + 3
	if end > len(in) {
		end = len(in)
	}
	return in[pos:end]
}

func (p *srlParser) newBNode() string {
	p.bnodeID++
	// A label with a colon cannot be written in a document (BLANK_NODE_LABEL
	// has no ':'), so a generated one never collides with a written one.
	return fmt.Sprintf("_anon:%d", p.bnodeID)
}

func (p *srlParser) parse() (*RuleSet, error) {
	rs := &RuleSet{Prefixes: make(map[string]string)}
	for {
		tok, err := p.peek()
		if err != nil {
			return nil, err
		}
		switch tok.kind {
		case srlTokEOF:
			rs.Prefixes = p.prefixes
			rs.Base = p.base
			return rs, nil
		case srlTokPrefix:
			err = p.parsePrefix()
		case srlTokBase:
			err = p.parseBase()
		case srlTokVersion:
			err = p.parseVersion(rs)
		case srlTokImports:
			err = p.parseImports(rs)
		case srlTokData:
			var data []SRLTriple
			if data, err = p.parseDataBlock(); err == nil {
				rs.DataBlocks = append(rs.DataBlocks, data)
			}
		case srlTokRule:
			var rule SRLRule
			if rule, err = p.parseRule(); err == nil {
				rs.Rules = append(rs.Rules, rule)
			}
		default:
			err = p.unexpected(tok, "RULE, DATA, PREFIX, BASE, VERSION or IMPORTS")
		}
		if err != nil {
			return nil, err
		}
	}
}

func (p *srlParser) parsePrefix() error {
	p.next() // consume PREFIX
	tok, err := p.next()
	if err != nil {
		return err
	}
	if tok.kind != srlTokPName || !strings.HasSuffix(tok.val, ":") {
		return p.unexpected(tok, "a prefix name ending in ':'")
	}
	iriTok, err := p.expect(srlTokIRI, "an IRI after the prefix name")
	if err != nil {
		return err
	}
	p.prefixes[strings.TrimSuffix(tok.val, ":")] = iriTok.val
	p.prologue = ""
	return nil
}

func (p *srlParser) parseBase() error {
	p.next() // consume BASE
	iriTok, err := p.expect(srlTokIRI, "an IRI after BASE")
	if err != nil {
		return err
	}
	p.lexer.base = iriTok.val
	p.base = iriTok.val
	p.prologue = ""
	return nil
}

func (p *srlParser) parseVersion(rs *RuleSet) error {
	p.next() // consume VERSION
	tok, err := p.expect(srlTokString, "a version string after VERSION")
	if err != nil {
		return err
	}
	rs.Version = tok.val
	return nil
}

func (p *srlParser) parseImports(rs *RuleSet) error {
	p.next() // consume IMPORTS
	iri, err := p.parseIRI("an IRI after IMPORTS")
	if err != nil {
		return err
	}
	rs.Imports = append(rs.Imports, iri)
	return nil
}

// parseIRI reads an IRIREF or prefixed name and returns the absolute IRI.
func (p *srlParser) parseIRI(what string) (string, error) {
	tok, err := p.next()
	if err != nil {
		return "", err
	}
	switch tok.kind {
	case srlTokIRI:
		return tok.val, nil
	case srlTokPName:
		return p.expandPName(tok)
	}
	return "", p.unexpected(tok, what)
}

func (p *srlParser) expandPName(tok srlToken) (string, error) {
	idx := strings.Index(tok.val, ":")
	if idx < 0 {
		return "", fmt.Errorf("invalid prefixed name %q at position %d", tok.val, tok.pos)
	}
	ns, ok := p.prefixes[tok.val[:idx]]
	if !ok {
		return "", fmt.Errorf("undefined prefix %q in %q at position %d: declare it with PREFIX", tok.val[:idx], tok.val, tok.pos)
	}
	return ns + tok.val[idx+1:], nil
}

// currentPrologue is the SPARQL prologue equivalent to the declarations in
// force, used to parse and later evaluate expressions.
func (p *srlParser) currentPrologue() string {
	if p.prologue != "" {
		return p.prologue
	}
	var b strings.Builder
	if p.base != "" {
		fmt.Fprintf(&b, "BASE <%s>\n", p.base)
	}
	for _, k := range sortedKeys(p.prefixes) {
		fmt.Fprintf(&b, "PREFIX %s: <%s>\n", k, p.prefixes[k])
	}
	p.prologue = b.String() + "\n"
	return p.prologue
}

func (p *srlParser) parseDataBlock() ([]SRLTriple, error) {
	p.next() // consume DATA
	if _, err := p.expect(srlTokLBrace, "'{' after DATA"); err != nil {
		return nil, err
	}
	triples, err := p.parseTemplateBlock(srlData)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect(srlTokRBrace, "'}' ending the DATA block"); err != nil {
		return nil, err
	}
	return triples, nil
}

// parseTemplateBlock parses the triples of a DATA block or a rule head up to
// (not including) the closing brace.
func (p *srlParser) parseTemplateBlock(mode srlMode) ([]SRLTriple, error) {
	tok, err := p.peek()
	if err != nil {
		return nil, err
	}
	if tok.kind == srlTokRBrace {
		return nil, nil
	}
	return p.parseTriplesBlock(mode)
}

func (p *srlParser) parseRule() (SRLRule, error) {
	p.next() // consume RULE
	var rule SRLRule
	tok, err := p.peek()
	if err != nil {
		return rule, err
	}
	if tok.kind == srlTokIRI || tok.kind == srlTokPName {
		if rule.ID, err = p.parseIRI("a rule identifier"); err != nil {
			return rule, err
		}
	}
	if _, err := p.expect(srlTokLBrace, "'{' starting the rule head"); err != nil {
		return rule, err
	}
	if rule.Head, err = p.parseTemplateBlock(srlTemplate); err != nil {
		return rule, err
	}
	if _, err := p.expect(srlTokRBrace, "'}' ending the rule head"); err != nil {
		return rule, err
	}
	if _, err := p.expect(srlTokWhere, "WHERE"); err != nil {
		return rule, err
	}
	if rule.BaseGraphOnly, err = p.accept(srlTokData); err != nil {
		return rule, err
	}
	if _, err := p.expect(srlTokLBrace, "'{' starting the rule body"); err != nil {
		return rule, err
	}
	if rule.Body, err = p.parseBody(bodyRule); err != nil {
		return rule, err
	}
	if _, err := p.expect(srlTokRBrace, "'}' ending the rule body"); err != nil {
		return rule, err
	}
	rule.prologue = p.currentPrologue()
	return rule, nil
}
