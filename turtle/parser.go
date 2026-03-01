package turtle

import (
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Parse reads Turtle from r and adds triples to g.
func Parse(g *rdflibgo.Graph, r io.Reader, opts ...Option) error {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	parser := &turtleParser{
		g:        g,
		input:    string(data),
		base:     cfg.base,
		prefixes: make(map[string]string),
	}
	// Copy graph namespace bindings as initial prefixes
	g.Namespaces()(func(prefix string, ns rdflibgo.URIRef) bool {
		parser.prefixes[prefix] = ns.Value()
		return true
	})
	return parser.parse()
}

type turtleParser struct {
	g        *rdflibgo.Graph
	input    string
	pos      int
	line     int
	col      int
	base     string
	prefixes map[string]string // prefix -> namespace URI
}

// parse is the main entry point.
func (p *turtleParser) parse() error {
	p.line = 1
	p.col = 1
	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			break
		}
		if err := p.statement(); err != nil {
			return err
		}
	}
	return nil
}

// statement parses a directive or triple statement.
func (p *turtleParser) statement() error {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil
	}

	ch := p.input[p.pos]

	// @prefix
	if ch == '@' {
		return p.directive()
	}

	// SPARQL-style PREFIX/BASE (case-insensitive)
	if (ch == 'P' || ch == 'p') && p.matchKeywordCI("PREFIX") {
		return p.sparqlPrefix()
	}
	if (ch == 'B' || ch == 'b') && p.matchKeywordCI("BASE") {
		return p.sparqlBase()
	}

	// Triple
	return p.tripleStatement()
}

// directive handles @prefix and @base.
func (p *turtleParser) directive() error {
	p.pos++ // skip '@'
	if p.startsWith("prefix") {
		p.pos += 6
		p.skipWS()
		prefix := p.readPrefixName()
		if !p.expect(':') {
			return p.errorf("expected ':' after prefix name")
		}
		p.skipWS()
		iri, err := p.readIRI()
		if err != nil {
			return err
		}
		iri = p.resolveIRI(iri)
		p.prefixes[prefix] = iri
		p.g.Bind(prefix, rdflibgo.NewURIRefUnsafe(iri))
		p.skipWS()
		if !p.expect('.') {
			return p.errorf("expected '.' after @prefix")
		}
		return nil
	}
	if p.startsWith("base") {
		p.pos += 4
		p.skipWS()
		iri, err := p.readIRI()
		if err != nil {
			return err
		}
		p.base = p.resolveIRI(iri)
		p.skipWS()
		if !p.expect('.') {
			return p.errorf("expected '.' after @base")
		}
		return nil
	}
	return p.errorf("unknown directive")
}

func (p *turtleParser) sparqlPrefix() error {
	p.pos += 6
	p.skipWS()
	prefix := p.readPrefixName()
	if !p.expect(':') {
		return p.errorf("expected ':' after PREFIX name")
	}
	p.skipWS()
	iri, err := p.readIRI()
	if err != nil {
		return err
	}
	iri = p.resolveIRI(iri)
	p.prefixes[prefix] = iri
	p.g.Bind(prefix, rdflibgo.NewURIRefUnsafe(iri))
	return nil
}

func (p *turtleParser) sparqlBase() error {
	p.pos += 4
	p.skipWS()
	iri, err := p.readIRI()
	if err != nil {
		return err
	}
	p.base = p.resolveIRI(iri)
	return nil
}

// tripleStatement parses: subject predicateObjectList '.'
func (p *turtleParser) tripleStatement() error {
	subj, err := p.readSubject()
	if err != nil {
		return err
	}

	p.skipWS()
	if err := p.predicateObjectList(subj); err != nil {
		return err
	}

	p.skipWS()
	if !p.expect('.') {
		return p.errorf("expected '.' at end of triple")
	}
	return nil
}

// predicateObjectList parses: verb objectList (';' verb objectList)*
func (p *turtleParser) predicateObjectList(subj rdflibgo.Subject) error {
	pred, err := p.readVerb()
	if err != nil {
		return err
	}

	if err := p.objectList(subj, pred); err != nil {
		return err
	}

	for {
		p.skipWS()
		if p.pos >= len(p.input) || p.input[p.pos] != ';' {
			break
		}
		p.pos++ // skip ';'
		p.skipWS()
		// Allow trailing ';' before '.'
		if p.pos >= len(p.input) || p.input[p.pos] == '.' || p.input[p.pos] == ']' {
			break
		}
		pred, err = p.readVerb()
		if err != nil {
			return err
		}
		if err := p.objectList(subj, pred); err != nil {
			return err
		}
	}
	return nil
}

// objectList parses: object (',' object)*
func (p *turtleParser) objectList(subj rdflibgo.Subject, pred rdflibgo.URIRef) error {
	obj, err := p.readObject()
	if err != nil {
		return err
	}
	p.g.Add(subj, pred, obj)

	for {
		p.skipWS()
		if p.pos >= len(p.input) || p.input[p.pos] != ',' {
			break
		}
		p.pos++ // skip ','
		p.skipWS()
		obj, err = p.readObject()
		if err != nil {
			return err
		}
		p.g.Add(subj, pred, obj)
	}
	return nil
}

// readSubject parses a subject: IRI, prefixed name, blank node, or collection.
func (p *turtleParser) readSubject() (rdflibgo.Subject, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input, expected subject")
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
	if ch == '[' {
		return p.readBlankNodePropertyList()
	}
	if ch == '(' {
		term, err := p.readCollection()
		if err != nil {
			return nil, err
		}
		if subj, ok := term.(rdflibgo.Subject); ok {
			return subj, nil
		}
		return nil, p.errorf("collection as subject must be a node")
	}

	// Prefixed name
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

// readVerb parses a predicate: 'a' | IRI | prefixed name.
func (p *turtleParser) readVerb() (rdflibgo.URIRef, error) {
	p.skipWS()
	// Check for 'a' keyword
	if p.pos < len(p.input) && p.input[p.pos] == 'a' {
		// Make sure it's not part of a longer name
		next := p.pos + 1
		if next >= len(p.input) || isDelimiter(p.input[next]) {
			p.pos++
			return rdflibgo.RDF.Type, nil
		}
	}

	return p.readPredicate()
}

func (p *turtleParser) readPredicate() (rdflibgo.URIRef, error) {
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return rdflibgo.URIRef{}, err
		}
		return rdflibgo.NewURIRefUnsafe(p.resolveIRI(iri)), nil
	}
	uri, err := p.readPrefixedName()
	if err != nil {
		return rdflibgo.URIRef{}, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

// readObject parses an object term.
func (p *turtleParser) readObject() (rdflibgo.Term, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input")
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
	if ch == '[' {
		return p.readBlankNodePropertyList()
	}
	if ch == '(' {
		return p.readCollection()
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

	// Prefixed name
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

// readIRI reads <...> and returns the IRI string (without angle brackets).
func (p *turtleParser) readIRI() (string, error) {
	if !p.expect('<') {
		return "", p.errorf("expected '<'")
	}
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '>' {
			iri := p.input[start:p.pos]
			p.pos++
			unescaped, err := p.unescapeIRI(iri)
			if err != nil {
				return "", err
			}
			return unescaped, nil
		}
		if ch == '\\' {
			p.pos += 2 // skip escape (handled in unescape)
			continue
		}
		p.pos++
	}
	return "", p.errorf("unterminated IRI")
}

// readPrefixedName reads prefix:local and returns the full URI.
func (p *turtleParser) readPrefixedName() (string, error) {
	prefix := p.readPrefixName()
	if !p.expect(':') {
		return "", p.errorf("expected ':' in prefixed name")
	}
	local := p.readLocalName()
	ns, ok := p.prefixes[prefix]
	if !ok {
		return "", p.errorf("undefined prefix %q", prefix)
	}
	return ns + local, nil
}

// readBlankNodeLabel reads _:label.
func (p *turtleParser) readBlankNodeLabel() (rdflibgo.BNode, error) {
	p.pos += 2 // skip "_:"
	start := p.pos
	for p.pos < len(p.input) && !isDelimiter(p.input[p.pos]) {
		p.pos++
	}
	// Strip trailing '.' if present (not part of blank node label)
	label := p.input[start:p.pos]
	label = strings.TrimRight(label, ".")
	p.pos = start + len(label)
	if label == "" {
		return rdflibgo.BNode{}, p.errorf("empty blank node label after _:")
	}
	return rdflibgo.NewBNode(label), nil
}

// readBlankNodePropertyList reads [...].
func (p *turtleParser) readBlankNodePropertyList() (rdflibgo.BNode, error) {
	p.pos++ // skip '['
	p.skipWS()

	b := rdflibgo.NewBNode()

	// Empty blank node []
	if p.pos < len(p.input) && p.input[p.pos] == ']' {
		p.pos++
		return b, nil
	}

	if err := p.predicateObjectList(b); err != nil {
		return rdflibgo.BNode{}, err
	}
	p.skipWS()
	if !p.expect(']') {
		return rdflibgo.BNode{}, p.errorf("expected ']'")
	}
	return b, nil
}

// readCollection reads (...) and builds rdf:List triples.
func (p *turtleParser) readCollection() (rdflibgo.Term, error) {
	p.pos++ // skip '('
	p.skipWS()

	// Empty collection
	if p.pos < len(p.input) && p.input[p.pos] == ')' {
		p.pos++
		return rdflibgo.RDF.Nil, nil
	}

	var items []rdflibgo.Term
	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			return nil, p.errorf("unterminated collection")
		}
		if p.input[p.pos] == ')' {
			p.pos++
			break
		}
		obj, err := p.readObject()
		if err != nil {
			return nil, err
		}
		items = append(items, obj)
	}

	// Build rdf:List chain
	if len(items) == 0 {
		return rdflibgo.RDF.Nil, nil
	}

	head := rdflibgo.NewBNode()
	current := head
	for i, item := range items {
		p.g.Add(current, rdflibgo.RDF.First, item)
		if i < len(items)-1 {
			next := rdflibgo.NewBNode()
			p.g.Add(current, rdflibgo.RDF.Rest, next)
			current = next
		} else {
			p.g.Add(current, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)
		}
	}
	return head, nil
}

// readLiteral reads a string literal with optional language tag or datatype.
func (p *turtleParser) readLiteral() (rdflibgo.Literal, error) {
	quote := p.input[p.pos]
	p.pos++

	// Check for triple-quoted string
	longString := false
	if p.pos+1 < len(p.input) && p.input[p.pos] == quote && p.input[p.pos+1] == quote {
		p.pos += 2
		longString = true
	}

	var sb strings.Builder
	for p.pos < len(p.input) {
		ch := p.input[p.pos]

		if ch == '\\' {
			p.pos++
			if p.pos >= len(p.input) {
				return rdflibgo.Literal{}, p.errorf("unterminated escape")
			}
			escaped, err := p.readEscape()
			if err != nil {
				return rdflibgo.Literal{}, err
			}
			sb.WriteString(escaped)
			continue
		}

		if longString {
			if ch == quote && p.pos+2 < len(p.input) && p.input[p.pos+1] == quote && p.input[p.pos+2] == quote {
				p.pos += 3
				goto done
			}
			r, size := utf8.DecodeRuneInString(p.input[p.pos:])
			sb.WriteRune(r)
			if ch == '\n' {
				p.line++
				p.col = 1
			}
			p.pos += size
		} else {
			if ch == quote {
				p.pos++
				goto done
			}
			if ch == '\n' || ch == '\r' {
				return rdflibgo.Literal{}, p.errorf("newline in short string")
			}
			r, size := utf8.DecodeRuneInString(p.input[p.pos:])
			sb.WriteRune(r)
			p.pos += size
		}
	}
	return rdflibgo.Literal{}, p.errorf("unterminated string literal")

done:
	value := sb.String()
	var lopts []rdflibgo.LiteralOption

	// Language tag or datatype
	if p.pos < len(p.input) && p.input[p.pos] == '@' {
		p.pos++
		lang := p.readLangTag()
		lopts = append(lopts, rdflibgo.WithLang(lang))
	} else if p.pos+1 < len(p.input) && p.input[p.pos] == '^' && p.input[p.pos+1] == '^' {
		p.pos += 2
		dt, err := p.readDatatypeIRI()
		if err != nil {
			return rdflibgo.Literal{}, err
		}
		lopts = append(lopts, rdflibgo.WithDatatype(rdflibgo.NewURIRefUnsafe(dt)))
	}

	return rdflibgo.NewLiteral(value, lopts...), nil
}

// readEscape handles escape sequences.
func (p *turtleParser) readEscape() (string, error) {
	ch := p.input[p.pos]
	p.pos++
	switch ch {
	case 'n':
		return "\n", nil
	case 'r':
		return "\r", nil
	case 't':
		return "\t", nil
	case '\\':
		return "\\", nil
	case '"':
		return "\"", nil
	case '\'':
		return "'", nil
	case 'u':
		return p.readUnicodeEscape(4)
	case 'U':
		return p.readUnicodeEscape(8)
	default:
		return "", p.errorf("unknown escape \\%c", ch)
	}
}

func (p *turtleParser) readUnicodeEscape(n int) (string, error) {
	if p.pos+n > len(p.input) {
		return "", p.errorf("truncated unicode escape")
	}
	hex := p.input[p.pos : p.pos+n]
	p.pos += n
	code, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return "", p.errorf("invalid unicode escape: %s", hex)
	}
	return string(rune(code)), nil
}

// tryNumeric attempts to parse a numeric literal.
func (p *turtleParser) tryNumeric() (rdflibgo.Literal, bool) {
	start := p.pos

	// Optional sign
	if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
		p.pos++
	}

	hasDigitsBefore := false
	for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
		hasDigitsBefore = true
		p.pos++
	}

	hasDot := false
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		// Peek: if next char is not a digit, this dot is a statement terminator
		if p.pos+1 < len(p.input) && p.input[p.pos+1] >= '0' && p.input[p.pos+1] <= '9' {
			hasDot = true
			p.pos++ // skip '.'
			for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
				p.pos++
			}
		} else if !hasDigitsBefore {
			// Just a dot, not a number
			p.pos = start
			return rdflibgo.Literal{}, false
		}
	}

	hasExp := false
	if p.pos < len(p.input) && (p.input[p.pos] == 'e' || p.input[p.pos] == 'E') {
		hasExp = true
		p.pos++
		if p.pos < len(p.input) && (p.input[p.pos] == '+' || p.input[p.pos] == '-') {
			p.pos++
		}
		if p.pos >= len(p.input) || p.input[p.pos] < '0' || p.input[p.pos] > '9' {
			p.pos = start
			return rdflibgo.Literal{}, false
		}
		for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
			p.pos++
		}
	}

	if !hasDigitsBefore && !hasDot {
		p.pos = start
		return rdflibgo.Literal{}, false
	}

	lexical := p.input[start:p.pos]

	var dt rdflibgo.URIRef
	switch {
	case hasExp:
		dt = rdflibgo.XSDDouble
	case hasDot:
		dt = rdflibgo.XSDDecimal
	default:
		dt = rdflibgo.XSDInteger
	}

	return rdflibgo.NewLiteral(lexical, rdflibgo.WithDatatype(dt)), true
}

func (p *turtleParser) readLangTag() string {
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '-' || (ch >= '0' && ch <= '9') {
			p.pos++
		} else {
			break
		}
	}
	return p.input[start:p.pos]
}

func (p *turtleParser) readDatatypeIRI() (string, error) {
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return "", err
		}
		return p.resolveIRI(iri), nil
	}
	// Prefixed name
	return p.readPrefixedName()
}

// --- Helper methods ---

func (p *turtleParser) readPrefixName() string {
	start := p.pos
	for p.pos < len(p.input) {
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if r == ':' || (r < 128 && isDelimiter(byte(r))) {
			break
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '.' {
			break
		}
		p.pos += size
	}
	return p.input[start:p.pos]
}

func (p *turtleParser) readLocalName() string {
	start := p.pos
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '\\' && p.pos+1 < len(p.input) {
			p.pos += 2 // escaped char in local name
			continue
		}
		if ch == '%' && p.pos+2 < len(p.input) {
			p.pos += 3 // percent-encoded
			continue
		}
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if (r < 128 && isDelimiter(byte(r))) || r == ';' || r == ',' || r == '.' || r == '[' || r == ']' || r == '(' || r == ')' {
			break
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-' && r != '.' && r != '\u00B7' {
			break
		}
		p.pos += size
	}
	// Trim trailing dots (not part of local name)
	for p.pos > start && p.input[p.pos-1] == '.' {
		p.pos--
	}
	return p.input[start:p.pos]
}

func (p *turtleParser) skipWS() {
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == ' ' || ch == '\t' || ch == '\r' {
			p.pos++
			p.col++
		} else if ch == '\n' {
			p.pos++
			p.line++
			p.col = 1
		} else if ch == '#' {
			// Comment: skip to end of line
			for p.pos < len(p.input) && p.input[p.pos] != '\n' {
				p.pos++
			}
		} else {
			break
		}
	}
}

func (p *turtleParser) expect(ch byte) bool {
	if p.pos < len(p.input) && p.input[p.pos] == ch {
		p.pos++
		return true
	}
	return false
}

func (p *turtleParser) startsWith(s string) bool {
	return strings.HasPrefix(p.input[p.pos:], s)
}

func (p *turtleParser) matchKeywordCI(kw string) bool {
	if p.pos+len(kw) > len(p.input) {
		return false
	}
	candidate := p.input[p.pos : p.pos+len(kw)]
	if !strings.EqualFold(candidate, kw) {
		return false
	}
	// Must be followed by whitespace or EOF
	after := p.pos + len(kw)
	if after < len(p.input) && !isWhitespace(p.input[after]) {
		return false
	}
	return true
}

func (p *turtleParser) resolveIRI(iri string) string {
	if p.base == "" || isAbsoluteIRI(iri) {
		return iri
	}
	b, err := url.Parse(p.base)
	if err != nil {
		return iri
	}
	ref, err := url.Parse(iri)
	if err != nil {
		return iri
	}
	return b.ResolveReference(ref).String()
}

func (p *turtleParser) unescapeIRI(s string) (string, error) {
	if !strings.ContainsRune(s, '\\') {
		return s, nil
	}
	var sb strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case 'u':
				if i+5 > len(s) {
					return "", p.errorf("truncated \\u escape in IRI")
				}
				code, err := strconv.ParseUint(s[i+1:i+5], 16, 32)
				if err != nil {
					return "", p.errorf("invalid \\u escape in IRI: %s", s[i+1:i+5])
				}
				sb.WriteRune(rune(code))
				i += 5
			case 'U':
				if i+9 > len(s) {
					return "", p.errorf("truncated \\U escape in IRI")
				}
				code, err := strconv.ParseUint(s[i+1:i+9], 16, 32)
				if err != nil {
					return "", p.errorf("invalid \\U escape in IRI: %s", s[i+1:i+9])
				}
				sb.WriteRune(rune(code))
				i += 9
			default:
				return "", p.errorf("unknown escape \\%c in IRI", s[i])
			}
		} else {
			sb.WriteByte(s[i])
			i++
		}
	}
	return sb.String(), nil
}

func (p *turtleParser) errorf(format string, args ...any) error {
	return fmt.Errorf("turtle parse error at line %d: "+format, append([]any{p.line}, args...)...)
}

func isDelimiter(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '<' || ch == '>' || ch == '"' || ch == '\'' || ch == '{' || ch == '}' || ch == '|' || ch == '^' || ch == '`'
}

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func isAbsoluteIRI(s string) bool {
	// Has scheme: starts with letter followed by letters/digits/+/-./:
	colon := strings.Index(s, ":")
	if colon <= 0 {
		return false
	}
	for i := 0; i < colon; i++ {
		ch := s[i]
		if i == 0 {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')) {
				return false
			}
		} else {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '+' || ch == '-' || ch == '.') {
				return false
			}
		}
	}
	return true
}
