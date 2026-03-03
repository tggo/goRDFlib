package trig

import (
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
)

// Parse reads TriG from r and adds all triples (from all graphs) into g.
// Named graph information is discarded. Use ParseDataset for named graph support.
func Parse(g *rdflibgo.Graph, r io.Reader, opts ...Option) error {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	ds := graph.NewDataset()
	g.Namespaces()(func(prefix string, ns rdflibgo.URIRef) bool {
		ds.Bind(prefix, ns)
		return true
	})
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	p := newTrigParser(ds, string(data), cfg.base)
	if err := p.parse(); err != nil {
		return err
	}
	// Copy all triples from all graphs into g, and copy prefixes
	for gr := range ds.Graphs() {
		gr.Triples(nil, nil, nil)(func(t rdflibgo.Triple) bool {
			g.Add(t.Subject, t.Predicate, t.Object)
			return true
		})
	}
	for prefix, ns := range p.prefixes {
		g.Bind(prefix, rdflibgo.NewURIRefUnsafe(ns))
	}
	return nil
}

// ParseDataset reads TriG from r and populates ds with named graphs.
func ParseDataset(ds *graph.Dataset, r io.Reader, opts ...Option) error {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	p := newTrigParser(ds, string(data), cfg.base)
	return p.parse()
}

func newTrigParser(ds *graph.Dataset, input, base string) *trigParser {
	p := &trigParser{
		ds:       ds,
		input:    input,
		base:     base,
		prefixes: make(map[string]string),
	}
	p.currentGraph = ds.DefaultContext()
	ds.Namespaces()(func(prefix string, ns rdflibgo.URIRef) bool {
		p.prefixes[prefix] = ns.Value()
		return true
	})
	return p
}

type trigParser struct {
	ds           *graph.Dataset
	currentGraph *graph.Graph // active graph for Add()
	input        string
	pos          int
	line         int
	col          int
	base         string
	prefixes     map[string]string // prefix -> namespace URI
}

// parse is the main entry point.
func (p *trigParser) parse() error {
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

// statement parses a directive, graph block, or triple statement.
func (p *trigParser) statement() error {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil
	}

	ch := p.input[p.pos]

	// @prefix / @base / @version
	if ch == '@' {
		return p.directive()
	}

	// SPARQL-style PREFIX/BASE/VERSION (case-insensitive)
	if (ch == 'P' || ch == 'p') && p.matchKeywordCI("PREFIX") {
		return p.sparqlPrefix()
	}
	if (ch == 'B' || ch == 'b') && p.matchKeywordCI("BASE") {
		return p.sparqlBase()
	}
	if (ch == 'V' || ch == 'v') && p.matchKeywordCI("VERSION") {
		return p.sparqlVersion()
	}

	// GRAPH keyword
	if (ch == 'G' || ch == 'g') && p.matchKeywordCI("GRAPH") {
		return p.graphBlock()
	}

	// Default graph block: { triples }
	if ch == '{' {
		return p.wrappedGraphBlock(nil)
	}

	// Could be: IRI/bnode/prefixedName followed by either { (graph block) or predicate (triple)
	return p.subjectOrGraph()
}

// graphBlock handles: GRAPH <iri> { ... } or GRAPH _:b { ... } or GRAPH prefix:local { ... }
func (p *trigParser) graphBlock() error {
	p.pos += 5 // skip "GRAPH"
	p.skipWS()

	graphID, err := p.readGraphLabel()
	if err != nil {
		return err
	}
	return p.wrappedGraphBlock(graphID)
}

// wrappedGraphBlock parses { triples } with graphID as the named graph.
// nil graphID means default graph.
func (p *trigParser) wrappedGraphBlock(graphID rdflibgo.Term) error {
	p.skipWS()
	if !p.expect('{') {
		return p.errorf("expected '{' to start graph block")
	}

	prevGraph := p.currentGraph
	p.currentGraph = p.ds.Graph(graphID)

	for {
		p.skipWS()
		if p.pos >= len(p.input) {
			return p.errorf("unterminated graph block")
		}
		if p.input[p.pos] == '}' {
			p.pos++
			break
		}
		if err := p.blockTripleStatement(); err != nil {
			return err
		}
	}

	p.currentGraph = prevGraph
	return nil
}

// blockTripleStatement parses a triple inside a graph block.
// The trailing '.' is optional before '}'.
func (p *trigParser) blockTripleStatement() error {
	p.skipWS()
	if p.pos >= len(p.input) {
		return p.errorf("unexpected end of input in graph block")
	}

	// Collections require a predicate-object list
	isCollection := p.input[p.pos] == '('

	subj, err := p.readSubject()
	if err != nil {
		return err
	}

	p.skipWS()
	// Blank node property list may stand alone (but not collections)
	if p.pos < len(p.input) && (p.input[p.pos] == '.' || p.input[p.pos] == '}') {
		if _, isBNode := subj.(rdflibgo.BNode); isBNode && !isCollection {
			if p.pos < len(p.input) && p.input[p.pos] == '.' {
				p.pos++
			}
			return nil
		}
		if isCollection {
			return p.errorf("standalone collection requires predicate-object list")
		}
	}

	if err := p.predicateObjectList(subj); err != nil {
		return err
	}

	p.skipWS()
	// '.' is optional before '}'
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		p.pos++
	}
	return nil
}

// subjectOrGraph reads a term and then decides:
// - If followed by '{', it's a named graph block (only IRI, _:label, or [] allowed as graph label)
// - Otherwise, it's a triple statement
func (p *trigParser) subjectOrGraph() error {
	p.skipWS()
	if p.pos >= len(p.input) {
		return p.errorf("unexpected end of input")
	}

	ch := p.input[p.pos]

	// Check if this could be a graph label: IRI, _:label, or []
	// If it's '[' — peek ahead: [] followed by { is graph block, [...] is bnode property list subject
	if ch == '[' {
		return p.bracketSubjectOrGraph()
	}

	// Collections cannot be graph labels in TriG
	if ch == '(' {
		return p.collectionTripleStatement()
	}

	// Reified triple subject
	if ch == '<' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '<' {
		return p.reifiedTripleStatement()
	}

	// Read IRI, _:label, or prefixed name as subject
	subj, err := p.readSubject()
	if err != nil {
		return err
	}

	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '{' {
		// Named graph block
		var graphID rdflibgo.Term
		switch v := subj.(type) {
		case rdflibgo.URIRef:
			graphID = v
		case rdflibgo.BNode:
			graphID = v
		default:
			return p.errorf("invalid graph label type")
		}
		return p.wrappedGraphBlock(graphID)
	}

	// Regular triple statement
	if err := p.predicateObjectList(subj); err != nil {
		return err
	}

	p.skipWS()
	if !p.expect('.') {
		return p.errorf("expected '.' at end of triple")
	}
	return nil
}

// bracketSubjectOrGraph handles '[' at top level:
// - [] followed by { → graph block with anonymous bnode graph label
// - [] or [...] → bnode subject for triple statement
func (p *trigParser) bracketSubjectOrGraph() error {
	savedPos := p.pos
	savedLine := p.line
	savedCol := p.col
	p.pos++ // skip '['
	p.skipWS()

	if p.pos < len(p.input) && p.input[p.pos] == ']' {
		p.pos++
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == '{' {
			// [] { ... } — anonymous graph block
			b := rdflibgo.NewBNode()
			return p.wrappedGraphBlock(b)
		}
		// [] as subject
		b := rdflibgo.NewBNode()
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == '.' {
			p.pos++
			return nil
		}
		if err := p.predicateObjectList(b); err != nil {
			return err
		}
		p.skipWS()
		if !p.expect('.') {
			return p.errorf("expected '.' at end of triple")
		}
		return nil
	}

	// [...] — blank node property list as subject (cannot be graph label)
	p.pos = savedPos
	p.line = savedLine
	p.col = savedCol

	subj, err := p.readBlankNodePropertyList()
	if err != nil {
		return err
	}

	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '{' {
		return p.errorf("blank node property list [...] not allowed as graph label")
	}

	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		p.pos++
		return nil
	}

	if err := p.predicateObjectList(subj); err != nil {
		return err
	}
	p.skipWS()
	if !p.expect('.') {
		return p.errorf("expected '.' at end of triple")
	}
	return nil
}

// collectionTripleStatement handles '(' at top level — collections cannot be graph labels.
func (p *trigParser) collectionTripleStatement() error {
	term, err := p.readCollection()
	if err != nil {
		return err
	}
	subj, ok := term.(rdflibgo.Subject)
	if !ok {
		return p.errorf("collection as subject must be a node")
	}

	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '{' {
		return p.errorf("collection not allowed as graph label")
	}

	// A standalone collection without predicates is invalid in TriG
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		return p.errorf("standalone collection requires predicate-object list")
	}

	if err := p.predicateObjectList(subj); err != nil {
		return err
	}
	p.skipWS()
	if !p.expect('.') {
		return p.errorf("expected '.' at end of triple")
	}
	return nil
}

// reifiedTripleStatement handles '<< ... >>' at top level as a subject.
func (p *trigParser) reifiedTripleStatement() error {
	subj, err := p.readReifiedTriple()
	if err != nil {
		return err
	}

	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		p.pos++
		return nil
	}

	if err := p.predicateObjectList(subj); err != nil {
		return err
	}
	p.skipWS()
	if !p.expect('.') {
		return p.errorf("expected '.' at end of triple")
	}
	return nil
}

// readGraphLabel reads a graph label (IRI, prefixed name, blank node label, or []).
func (p *trigParser) readGraphLabel() (rdflibgo.Term, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("expected graph label")
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
		p.pos++
		p.skipWS()
		if p.pos < len(p.input) && p.input[p.pos] == ']' {
			p.pos++
			return rdflibgo.NewBNode(), nil
		}
		return nil, p.errorf("only [] allowed as blank node graph label")
	}
	// Prefixed name
	uri, err := p.readPrefixedName()
	if err != nil {
		return nil, err
	}
	return rdflibgo.NewURIRefUnsafe(uri), nil
}

// tripleStatement parses: subject predicateObjectList '.'
func (p *trigParser) tripleStatement() error {
	subj, err := p.readSubject()
	if err != nil {
		return err
	}

	p.skipWS()
	// When the subject is a blank node property list [...] or a reified triple,
	// the predicateObjectList is optional.
	if p.pos < len(p.input) && p.input[p.pos] == '.' {
		if _, isBNode := subj.(rdflibgo.BNode); isBNode {
			p.pos++
			return nil
		}
	}

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
func (p *trigParser) predicateObjectList(subj rdflibgo.Subject) error {
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
		for p.pos < len(p.input) && p.input[p.pos] == ';' {
			p.pos++
			p.skipWS()
		}
		// Allow trailing ';' before '.', ']', '}', or '|}'
		if p.pos >= len(p.input) || p.input[p.pos] == '.' || p.input[p.pos] == ']' || p.input[p.pos] == '|' || p.input[p.pos] == '}' {
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
func (p *trigParser) objectList(subj rdflibgo.Subject, pred rdflibgo.URIRef) error {
	obj, err := p.readObject()
	if err != nil {
		return err
	}
	p.currentGraph.Add(subj, pred, obj)
	if err := p.readAnnotationsAndReifiers(subj, pred, obj); err != nil {
		return err
	}

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
		p.currentGraph.Add(subj, pred, obj)
		if err := p.readAnnotationsAndReifiers(subj, pred, obj); err != nil {
			return err
		}
	}
	return nil
}

// readSubject parses a subject: IRI, prefixed name, blank node, or collection.
func (p *trigParser) readSubject() (rdflibgo.Subject, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input, expected subject")
	}
	ch := p.input[p.pos]

	// Reified triple as subject: << s p o >> or << s p o ~ id >>
	if ch == '<' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '<' {
		return p.readReifiedTriple()
	}
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
func (p *trigParser) readVerb() (rdflibgo.URIRef, error) {
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == 'a' {
		next := p.pos + 1
		if next >= len(p.input) || isDelimiter(p.input[next]) {
			p.pos++
			return rdflibgo.RDF.Type, nil
		}
	}
	return p.readPredicate()
}

func (p *trigParser) readPredicate() (rdflibgo.URIRef, error) {
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
func (p *trigParser) readObject() (rdflibgo.Term, error) {
	p.skipWS()
	if p.pos >= len(p.input) {
		return nil, p.errorf("unexpected end of input")
	}
	ch := p.input[p.pos]

	// Triple term <<( s p o )>> or reified triple << s p o >>
	if ch == '<' && p.pos+1 < len(p.input) && p.input[p.pos+1] == '<' {
		return p.readTripleTermOrReified()
	}
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
func (p *trigParser) readIRI() (string, error) {
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
			if err := validateIRI(unescaped); err != nil {
				return "", p.errorf("%s", err)
			}
			return unescaped, nil
		}
		if ch == '\\' {
			p.pos += 2
			continue
		}
		if ch <= 0x20 || ch == '{' || ch == '}' || ch == '|' || ch == '^' || ch == '`' {
			return "", p.errorf("invalid character %q in IRI", ch)
		}
		p.pos++
	}
	return "", p.errorf("unterminated IRI")
}

// readPrefixedName reads prefix:local and returns the full URI.
func (p *trigParser) readPrefixedName() (string, error) {
	prefix := p.readPrefixName()
	if !p.expect(':') {
		return "", p.errorf("expected ':' in prefixed name")
	}
	local, err := p.readLocalName()
	if err != nil {
		return "", err
	}
	ns, ok := p.prefixes[prefix]
	if !ok {
		return "", p.errorf("undefined prefix %q", prefix)
	}
	return ns + unescapeLocalName(local), nil
}

func unescapeLocalName(s string) string {
	if !strings.ContainsAny(s, "\\") {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			sb.WriteByte(s[i])
		} else {
			sb.WriteByte(s[i])
		}
	}
	return sb.String()
}

// readBlankNodeLabel reads _:label.
func (p *trigParser) readBlankNodeLabel() (rdflibgo.BNode, error) {
	p.pos += 2 // skip "_:"
	start := p.pos
	if p.pos >= len(p.input) {
		return rdflibgo.BNode{}, p.errorf("empty blank node label after _:")
	}
	r, size := utf8.DecodeRuneInString(p.input[p.pos:])
	if !isPNCharsU(r) && !(r >= '0' && r <= '9') {
		return rdflibgo.BNode{}, p.errorf("invalid blank node label start: %c", r)
	}
	p.pos += size
	for p.pos < len(p.input) {
		r, size = utf8.DecodeRuneInString(p.input[p.pos:])
		if isPNChar(r) || r == '.' {
			p.pos += size
		} else {
			break
		}
	}
	for p.pos > start && p.input[p.pos-1] == '.' {
		p.pos--
	}
	label := p.input[start:p.pos]
	if label == "" {
		return rdflibgo.BNode{}, p.errorf("empty blank node label after _:")
	}
	return rdflibgo.NewBNode(label), nil
}

// readBlankNodePropertyList reads [...].
func (p *trigParser) readBlankNodePropertyList() (rdflibgo.BNode, error) {
	p.pos++ // skip '['
	p.skipWS()

	b := rdflibgo.NewBNode()

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
func (p *trigParser) readCollection() (rdflibgo.Term, error) {
	p.pos++ // skip '('
	p.skipWS()

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

	if len(items) == 0 {
		return rdflibgo.RDF.Nil, nil
	}

	head := rdflibgo.NewBNode()
	current := head
	for i, item := range items {
		p.currentGraph.Add(current, rdflibgo.RDF.First, item)
		if i < len(items)-1 {
			next := rdflibgo.NewBNode()
			p.currentGraph.Add(current, rdflibgo.RDF.Rest, next)
			current = next
		} else {
			p.currentGraph.Add(current, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)
		}
	}
	return head, nil
}

// readLiteral reads a string literal with optional language tag or datatype.
func (p *trigParser) readLiteral() (rdflibgo.Literal, error) {
	quote := p.input[p.pos]
	p.pos++

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

	if p.pos < len(p.input) && p.input[p.pos] == '@' {
		p.pos++
		lang, err := p.readLangTag()
		if err != nil {
			return rdflibgo.Literal{}, err
		}
		if idx := strings.Index(lang, "--"); idx >= 0 {
			dir := lang[idx+2:]
			lang = lang[:idx]
			if dir != "ltr" && dir != "rtl" {
				return rdflibgo.Literal{}, p.errorf("invalid base direction %q (must be ltr or rtl)", dir)
			}
			lopts = append(lopts, rdflibgo.WithLang(lang), rdflibgo.WithDir(dir))
		} else {
			lopts = append(lopts, rdflibgo.WithLang(lang))
		}
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

func (p *trigParser) readEscape() (string, error) {
	ch := p.input[p.pos]
	p.pos++
	switch ch {
	case 'n':
		return "\n", nil
	case 'r':
		return "\r", nil
	case 't':
		return "\t", nil
	case 'b':
		return "\b", nil
	case 'f':
		return "\f", nil
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

func (p *trigParser) readUnicodeEscape(n int) (string, error) {
	if p.pos+n > len(p.input) {
		return "", p.errorf("truncated unicode escape")
	}
	hex := p.input[p.pos : p.pos+n]
	p.pos += n
	code, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return "", p.errorf("invalid unicode escape: %s", hex)
	}
	if code >= 0xD800 && code <= 0xDFFF {
		return "", p.errorf("invalid surrogate in unicode escape: %s", hex)
	}
	return string(rune(code)), nil
}

func (p *trigParser) tryNumeric() (rdflibgo.Literal, bool) {
	start := p.pos
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
		next := byte(0)
		if p.pos+1 < len(p.input) {
			next = p.input[p.pos+1]
		}
		if next >= '0' && next <= '9' {
			hasDot = true
			p.pos++
			for p.pos < len(p.input) && p.input[p.pos] >= '0' && p.input[p.pos] <= '9' {
				p.pos++
			}
		} else if hasDigitsBefore && (next == 'e' || next == 'E') {
			hasDot = true
			p.pos++
		} else if !hasDigitsBefore {
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

func (p *trigParser) readLangTag() (string, error) {
	start := p.pos
	if p.pos >= len(p.input) || !((p.input[p.pos] >= 'a' && p.input[p.pos] <= 'z') || (p.input[p.pos] >= 'A' && p.input[p.pos] <= 'Z')) {
		return "", p.errorf("invalid language tag: must start with a letter")
	}
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '-' || (ch >= '0' && ch <= '9') {
			p.pos++
		} else {
			break
		}
	}
	return p.input[start:p.pos], nil
}

func (p *trigParser) readDatatypeIRI() (string, error) {
	p.skipWS()
	if p.pos < len(p.input) && p.input[p.pos] == '<' {
		iri, err := p.readIRI()
		if err != nil {
			return "", err
		}
		return p.resolveIRI(iri), nil
	}
	return p.readPrefixedName()
}

// --- Directives ---

func (p *trigParser) directive() error {
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
		p.ds.Bind(prefix, rdflibgo.NewURIRefUnsafe(iri))
		p.skipWS()
		if !p.expect('.') {
			return p.errorf("expected '.' after @prefix")
		}
		return nil
	}
	if p.startsWith("version") {
		p.pos += 7
		p.skipWS()
		if _, err := p.readVersionString(); err != nil {
			return err
		}
		p.skipWS()
		if !p.expect('.') {
			return p.errorf("expected '.' after @version")
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

func (p *trigParser) sparqlPrefix() error {
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
	p.ds.Bind(prefix, rdflibgo.NewURIRefUnsafe(iri))
	return nil
}

func (p *trigParser) sparqlBase() error {
	p.pos += 4
	p.skipWS()
	iri, err := p.readIRI()
	if err != nil {
		return err
	}
	p.base = p.resolveIRI(iri)
	return nil
}

// --- Helper methods ---

func (p *trigParser) readPrefixName() string {
	start := p.pos
	for p.pos < len(p.input) {
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if r == ':' || (r < 128 && isDelimiter(byte(r))) {
			break
		}
		if p.pos == start {
			if !isPNCharsBase(r) {
				break
			}
		} else {
			if !isPNChar(r) && r != '.' {
				break
			}
		}
		p.pos += size
	}
	for p.pos > start && p.input[p.pos-1] == '.' {
		p.pos--
	}
	return p.input[start:p.pos]
}

func (p *trigParser) readLocalName() (string, error) {
	start := p.pos
	first := true
	for p.pos < len(p.input) {
		ch := p.input[p.pos]
		if ch == '\\' && p.pos+1 < len(p.input) {
			next := p.input[p.pos+1]
			if next == 'u' || next == 'U' {
				return "", p.errorf("\\%c escape not allowed in local name", next)
			}
			p.pos += 2
			first = false
			continue
		}
		if ch == '%' {
			if p.pos+2 >= len(p.input) || !isHexDigit(p.input[p.pos+1]) || !isHexDigit(p.input[p.pos+2]) {
				return "", p.errorf("invalid percent encoding in local name")
			}
			p.pos += 3
			first = false
			continue
		}
		r, size := utf8.DecodeRuneInString(p.input[p.pos:])
		if first {
			if !isPNCharsU(r) && r != ':' && !(r >= '0' && r <= '9') {
				break
			}
		} else {
			if r == ':' || r == '.' {
				p.pos += size
				continue
			}
			if r == ';' || r == ',' || r == '[' || r == ']' || r == '(' || r == ')' || r == '#' {
				break
			}
			if r < 128 && isDelimiter(byte(r)) {
				break
			}
			if !isPNChar(r) {
				break
			}
		}
		p.pos += size
		first = false
	}
	for p.pos > start && p.input[p.pos-1] == '.' {
		p.pos--
	}
	return p.input[start:p.pos], nil
}

func (p *trigParser) skipWS() {
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
			for p.pos < len(p.input) && p.input[p.pos] != '\n' {
				p.pos++
			}
		} else {
			break
		}
	}
}

func (p *trigParser) expect(ch byte) bool {
	if p.pos < len(p.input) && p.input[p.pos] == ch {
		p.pos++
		return true
	}
	return false
}

func (p *trigParser) startsWith(s string) bool {
	return strings.HasPrefix(p.input[p.pos:], s)
}

func (p *trigParser) matchKeywordCI(kw string) bool {
	if p.pos+len(kw) > len(p.input) {
		return false
	}
	candidate := p.input[p.pos : p.pos+len(kw)]
	if !strings.EqualFold(candidate, kw) {
		return false
	}
	after := p.pos + len(kw)
	if after >= len(p.input) {
		return true
	}
	ch := p.input[after]
	// Keyword can be followed by whitespace, '<' (for BASE<url>), or ':' (for PREFIX:)
	return isWhitespace(ch) || ch == '<' || ch == ':'
}

func (p *trigParser) resolveIRI(iri string) string {
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
	resolved := b.ResolveReference(ref).String()
	if strings.Contains(iri, "#") && !strings.Contains(resolved, "#") {
		resolved += "#"
	}
	return resolved
}

func (p *trigParser) unescapeIRI(s string) (string, error) {
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
				if code >= 0xD800 && code <= 0xDFFF {
					return "", p.errorf("invalid surrogate in IRI escape: %s", s[i+1:i+5])
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
				if code >= 0xD800 && code <= 0xDFFF {
					return "", p.errorf("invalid surrogate in IRI escape: %s", s[i+1:i+9])
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

func (p *trigParser) errorf(format string, args ...any) error {
	return fmt.Errorf("trig parse error at line %d: "+format, append([]any{p.line}, args...)...)
}

func isDelimiter(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' || ch == '<' || ch == '>' || ch == '"' || ch == '\'' || ch == '{' || ch == '}' || ch == '|' || ch == '^' || ch == '`' || ch == ')' || ch == '~'
}

func isWhitespace(ch byte) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func isPNChar(r rune) bool {
	return isPNCharsU(r) ||
		r == '-' ||
		(r >= '0' && r <= '9') ||
		r == 0x00B7 ||
		(r >= 0x0300 && r <= 0x036F) ||
		(r >= 0x203F && r <= 0x2040)
}

func isPNCharsU(r rune) bool {
	return r == '_' || isPNCharsBase(r)
}

func isPNCharsBase(r rune) bool {
	return (r >= 'A' && r <= 'Z') ||
		(r >= 'a' && r <= 'z') ||
		(r >= 0x00C0 && r <= 0x00D6) ||
		(r >= 0x00D8 && r <= 0x00F6) ||
		(r >= 0x00F8 && r <= 0x02FF) ||
		(r >= 0x0370 && r <= 0x037D) ||
		(r >= 0x037F && r <= 0x1FFF) ||
		(r >= 0x200C && r <= 0x200D) ||
		(r >= 0x2070 && r <= 0x218F) ||
		(r >= 0x2C00 && r <= 0x2FEF) ||
		(r >= 0x3001 && r <= 0xD7FF) ||
		(r >= 0xF900 && r <= 0xFDCF) ||
		(r >= 0xFDF0 && r <= 0xFFFD) ||
		(r >= 0x10000 && r <= 0xEFFFF)
}

func isHexDigit(ch byte) bool {
	return (ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

func validateIRI(s string) error {
	for _, r := range s {
		if r <= 0x20 || r == '<' || r == '>' || r == '{' || r == '}' || r == '|' || r == '^' || r == '`' {
			return fmt.Errorf("invalid character U+%04X in IRI", r)
		}
	}
	return nil
}

func isAbsoluteIRI(s string) bool {
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
