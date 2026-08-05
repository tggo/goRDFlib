// Package shacl implements a SHACL Core validator.
//
// The types and helpers in this file provide the RDF abstraction layer used by
// the validator. They wrap rdflibgo's graph and term packages behind a simpler
// API modelled after the original shacl-validator's rdf package.
package shacl

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

// Well-known IRI namespace prefixes.
const (
	RDF  = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
	RDFS = "http://www.w3.org/2000/01/rdf-schema#"
	XSD  = "http://www.w3.org/2001/XMLSchema#"
	SH   = "http://www.w3.org/ns/shacl#"
	OWL  = "http://www.w3.org/2002/07/owl#"
	MF   = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#"
	SHT  = "http://www.w3.org/ns/shacl-test#"
)

// Well-known IRIs used throughout the codebase.
const (
	RDFType        = RDF + "type"
	RDFFirst       = RDF + "first"
	RDFRest        = RDF + "rest"
	RDFNil         = RDF + "nil"
	RDFSClass      = RDFS + "Class"
	RDFSSubClassOf = RDFS + "subClassOf"
)

// TermKind distinguishes the three kinds of RDF terms.
type TermKind int

const (
	TermNone TermKind = iota
	TermIRI
	TermLiteral
	TermBlankNode
)

func (k TermKind) String() string {
	switch k {
	case TermNone:
		return "None"
	case TermIRI:
		return "IRI"
	case TermLiteral:
		return "Literal"
	case TermBlankNode:
		return "BlankNode"
	}
	return "Unknown"
}

// Term represents an RDF term: IRI, literal, or blank node.
type Term struct {
	kind     TermKind
	value    string
	datatype string
	language string
}

func (t Term) Kind() TermKind   { return t.kind }
func (t Term) Value() string    { return t.value }
func (t Term) Datatype() string { return t.datatype }
func (t Term) Language() string { return t.language }
func (t Term) IsNone() bool     { return t.kind == TermNone }
func (t Term) IsIRI() bool      { return t.kind == TermIRI }
func (t Term) IsLiteral() bool  { return t.kind == TermLiteral }
func (t Term) IsBlank() bool    { return t.kind == TermBlankNode }

func (t Term) String() string {
	switch t.kind {
	case TermIRI:
		return "<" + t.value + ">"
	case TermLiteral:
		s := `"` + t.value + `"`
		if t.language != "" {
			s += "@" + t.language
		} else if t.datatype != "" && t.datatype != XSD+"string" {
			s += "^^<" + t.datatype + ">"
		}
		return s
	case TermBlankNode:
		return "_:" + t.value
	}
	return ""
}

func (t Term) TermKey() string {
	switch t.kind {
	case TermIRI:
		return "I:" + t.value
	case TermLiteral:
		if t.language != "" {
			return "L:" + t.value + "@" + t.language
		}
		return "L:" + t.value + "^^" + t.datatype
	case TermBlankNode:
		return "B:" + t.value
	}
	return ""
}

func (t Term) Equal(other Term) bool {
	if t.kind != other.kind {
		return false
	}
	switch t.kind {
	case TermIRI:
		return t.value == other.value
	case TermLiteral:
		return t.value == other.value && t.datatype == other.datatype && t.language == other.language
	case TermBlankNode:
		return t.value == other.value
	}
	return false
}

// IRI constructs a new IRI term.
func IRI(uri string) Term {
	return Term{kind: TermIRI, value: uri}
}

// Literal constructs a new literal term with optional datatype and language.
func Literal(value, datatype, language string) Term {
	if datatype == "" && language == "" {
		datatype = XSD + "string"
	}
	return Term{kind: TermLiteral, value: value, datatype: datatype, language: language}
}

// BlankNode constructs a new blank node term.
func BlankNode(id string) Term {
	return Term{kind: TermBlankNode, value: id}
}

// Triple represents an RDF triple (subject-predicate-object).
type Triple struct {
	Subject   Term
	Predicate Term
	Object    Term
}

// fromRDFLib converts an rdflibgo term to a shacl Term.
func fromRDFLib(t term.Term) Term {
	if t == nil {
		return Term{}
	}
	switch v := t.(type) {
	case term.URIRef:
		return Term{kind: TermIRI, value: v.Value()}
	case term.Literal:
		dt := v.Datatype().Value()
		lang := v.Language()
		dir := v.Dir()
		val := v.Lexical()
		if lang != "" && dt == "" {
			if dir != "" {
				dt = RDF + "dirLangString"
			} else {
				dt = RDF + "langString"
			}
		} else if dt == "" && lang == "" {
			dt = XSD + "string"
		}
		// Combine language and direction into a single tag (e.g. "ar--ltr")
		fullLang := lang
		if dir != "" {
			fullLang = lang + "--" + dir
		}
		return Term{kind: TermLiteral, value: val, datatype: dt, language: fullLang}
	case term.BNode:
		return Term{kind: TermBlankNode, value: v.Value()}
	}
	return Term{}
}

// toSubject converts a shacl Term to an rdflibgo Subject.
func toSubject(t Term) term.Subject {
	switch t.kind {
	case TermIRI:
		return term.NewURIRefUnsafe(t.value)
	case TermBlankNode:
		return term.NewBNode(t.value)
	}
	return nil
}

// toURIRef converts a shacl Term to an rdflibgo URIRef.
func toURIRef(t Term) term.URIRef {
	return term.NewURIRefUnsafe(t.value)
}

// toTerm converts a shacl Term to an rdflibgo term.Term.
func toTerm(t Term) term.Term {
	switch t.kind {
	case TermIRI:
		return term.NewURIRefUnsafe(t.value)
	case TermLiteral:
		if t.language != "" {
			return term.NewLiteral(t.value, term.WithLang(t.language))
		}
		if t.datatype != "" && t.datatype != XSD+"string" {
			return term.NewLiteral(t.value, term.WithDatatype(term.NewURIRefUnsafe(t.datatype)))
		}
		return term.NewLiteral(t.value)
	case TermBlankNode:
		return term.NewBNode(t.value)
	}
	return nil
}

// Graph wraps an rdflibgo graph with lazy SPO/POS indexes.
//
// Concurrency: safe for concurrent reading. Several goroutines may validate
// against the same shapes graph at once, and the indexes each read builds are
// constructed once and published atomically. It is not safe to read a graph
// while another goroutine is calling Add or Merge — those invalidate the
// indexes, and the underlying graph is not itself synchronised.
type Graph struct {
	g       *graph.Graph
	baseURI string

	// idx holds the lazily built lookup indexes. It is replaced wholesale
	// rather than mutated in place, so a reader always sees a complete set.
	idx   atomic.Pointer[graphIndexes]
	idxMu sync.Mutex // serialises index construction
}

// NewGraph creates an empty graph with no base URI.
func NewGraph() *Graph {
	return &Graph{g: graph.NewGraph()}
}

// LoadTurtleFile loads a Turtle file from disk.
func LoadTurtleFile(path string, opts ...turtle.Option) (*Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	base := "file://" + path
	g := graph.NewGraph(graph.WithBase(base))
	parseOpts := append([]turtle.Option{}, opts...)
	parseOpts = append(parseOpts, turtle.WithBase(base))
	if err := turtle.Parse(g, f, parseOpts...); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &Graph{g: g, baseURI: base}, nil
}

// LoadTurtle parses Turtle data from a reader.
func LoadTurtle(r io.Reader, base string, opts ...turtle.Option) (*Graph, error) {
	g := graph.NewGraph(graph.WithBase(base))
	parseOpts := append([]turtle.Option{}, opts...)
	parseOpts = append(parseOpts, turtle.WithBase(base))
	if err := turtle.Parse(g, r, parseOpts...); err != nil {
		return nil, err
	}
	return &Graph{g: g, baseURI: base}, nil
}

// LoadTurtleString parses Turtle data from a string.
func LoadTurtleString(data, base string, opts ...turtle.Option) (*Graph, error) {
	return LoadTurtle(strings.NewReader(data), base, opts...)
}

// LoadJsonLDFile loads a JSON-LD file from disk.
func LoadJsonLDFile(path string, opts ...jsonld.Option) (*Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	base := "file://" + path
	g := graph.NewGraph(graph.WithBase(base))
	parseOpts := append([]jsonld.Option{}, opts...)
	parseOpts = append(parseOpts, jsonld.WithBase(base))
	if err := jsonld.Parse(g, f, parseOpts...); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &Graph{g: g, baseURI: base}, nil
}

// LoadJsonLD parses JSON-LD data from a reader.
func LoadJsonLD(r io.Reader, base string, opts ...jsonld.Option) (*Graph, error) {
	g := graph.NewGraph(graph.WithBase(base))
	parseOpts := append([]jsonld.Option{}, opts...)
	parseOpts = append(parseOpts, jsonld.WithBase(base))
	if err := jsonld.Parse(g, r, parseOpts...); err != nil {
		return nil, err
	}
	return &Graph{g: g, baseURI: base}, nil
}

// LoadJsonLDString parses JSON-LD data from a string.
func LoadJsonLDString(data, base string, opts ...jsonld.Option) (*Graph, error) {
	return LoadJsonLD(strings.NewReader(data), base, opts...)
}

// LoadNQuadsFile loads an N-Quads file from disk.
func LoadNQuadsFile(path string, opts ...nq.Option) (*Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	base := "file://" + path
	g := graph.NewGraph(graph.WithBase(base))
	parseOpts := append([]nq.Option{}, opts...)
	parseOpts = append(parseOpts, nq.WithBase(base))
	if err := nq.Parse(g, f, parseOpts...); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &Graph{g: g, baseURI: base}, nil
}

// LoadNQuads parses N-Quads data from a reader.
func LoadNQuads(r io.Reader, base string, opts ...nq.Option) (*Graph, error) {
	g := graph.NewGraph(graph.WithBase(base))
	parseOpts := append([]nq.Option{}, opts...)
	parseOpts = append(parseOpts, nq.WithBase(base))
	if err := nq.Parse(g, r, parseOpts...); err != nil {
		return nil, err
	}
	return &Graph{g: g, baseURI: base}, nil
}

// LoadNQuadsString parses N-Quads data from a string.
func LoadNQuadsString(data, base string, opts ...nq.Option) (*Graph, error) {
	return LoadNQuads(strings.NewReader(data), base, opts...)
}

// graphIndexes are the lookup tables All, Objects and Subjects read.
type graphIndexes struct {
	spo map[string]map[string][]Term // subject → predicate → []object
	pos map[string]map[string][]Term // predicate → object → []subject
	p   map[string][]Triple          // predicate → []Triple
}

// ensureIndexes returns the indexes, building them on first use.
//
// Validation reads a shapes graph from every goroutine that validates against
// it, so building the indexes lazily has to be safe to race on. Construction is
// serialised and the finished set published atomically; a second caller that
// arrives mid-build waits rather than seeing a half-filled map.
func (g *Graph) ensureIndexes() *graphIndexes {
	if idx := g.idx.Load(); idx != nil {
		return idx
	}
	g.idxMu.Lock()
	defer g.idxMu.Unlock()
	if idx := g.idx.Load(); idx != nil {
		return idx
	}

	idx := &graphIndexes{
		spo: make(map[string]map[string][]Term),
		pos: make(map[string]map[string][]Term),
		p:   make(map[string][]Triple),
	}
	g.g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		s := fromRDFLib(t.Subject)
		p := fromRDFLib(t.Predicate)
		o := fromRDFLib(t.Object)
		sk, pk, ok := s.TermKey(), p.TermKey(), o.TermKey()

		sp := idx.spo[sk]
		if sp == nil {
			sp = make(map[string][]Term)
			idx.spo[sk] = sp
		}
		sp[pk] = append(sp[pk], o)

		po := idx.pos[pk]
		if po == nil {
			po = make(map[string][]Term)
			idx.pos[pk] = po
		}
		po[ok] = append(po[ok], s)

		idx.p[pk] = append(idx.p[pk], Triple{Subject: s, Predicate: p, Object: o})
		return true
	})

	g.idx.Store(idx)
	return idx
}

func (g *Graph) invalidateIndexes() {
	g.idx.Store(nil)
}

// Triples returns all triples in the graph.
func (g *Graph) Triples() []Triple {
	var result []Triple
	g.g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		result = append(result, Triple{
			Subject:   fromRDFLib(t.Subject),
			Predicate: fromRDFLib(t.Predicate),
			Object:    fromRDFLib(t.Object),
		})
		return true
	})
	return result
}

// All returns all triples matching the pattern. Nil arguments are wildcards.
// All returns every triple matching the pattern. A nil argument is a wildcard.
//
// The indexes cover the patterns the validator asks for most; anything else
// falls through to the underlying graph, which can match any combination.
func (g *Graph) All(s, p, o *Term) []Triple {
	idx := g.ensureIndexes()

	switch {
	case s != nil && p != nil && o != nil:
		// A fully bound pattern is an existence check. It has to compare the
		// object rather than fall through to a wildcard scan, or Has reports
		// every triple as present in any non-empty graph.
		ok := o.TermKey()
		for _, obj := range idx.spo[s.TermKey()][p.TermKey()] {
			if obj.TermKey() == ok {
				return []Triple{{Subject: *s, Predicate: *p, Object: *o}}
			}
		}
		return nil

	case s != nil && p != nil && o == nil:
		objs := idx.spo[s.TermKey()][p.TermKey()]
		result := make([]Triple, len(objs))
		for i, obj := range objs {
			result[i] = Triple{Subject: *s, Predicate: *p, Object: obj}
		}
		return result

	case s == nil && p != nil && o != nil:
		subs := idx.pos[p.TermKey()][o.TermKey()]
		result := make([]Triple, len(subs))
		for i, sub := range subs {
			result[i] = Triple{Subject: sub, Predicate: *p, Object: *o}
		}
		return result

	case s == nil && p != nil && o == nil:
		return idx.p[p.TermKey()]

	case s != nil && p == nil:
		// Subject-bound, with the object possibly bound too.
		var result []Triple
		sub := toSubject(*s)
		g.g.Triples(sub, nil, nil)(func(t term.Triple) bool {
			triple := Triple{
				Subject:   fromRDFLib(t.Subject),
				Predicate: fromRDFLib(t.Predicate),
				Object:    fromRDFLib(t.Object),
			}
			if o == nil || triple.Object.TermKey() == o.TermKey() {
				result = append(result, triple)
			}
			return true
		})
		return result

	case s == nil && p == nil && o != nil:
		ok := o.TermKey()
		var result []Triple
		for _, t := range g.Triples() {
			if t.Object.TermKey() == ok {
				result = append(result, t)
			}
		}
		return result
	}

	return g.Triples()
}

// One returns the first triple matching the pattern.
func (g *Graph) One(s, p, o *Term) (Triple, bool) {
	triples := g.All(s, p, o)
	if len(triples) == 0 {
		return Triple{}, false
	}
	return triples[0], true
}

// Has returns true if at least one triple matches the pattern.
func (g *Graph) Has(s, p, o *Term) bool {
	return len(g.All(s, p, o)) > 0
}

// Objects returns all objects of triples matching (s, p, ?).
func (g *Graph) Objects(s, p Term) []Term {
	sp := g.ensureIndexes().spo[s.TermKey()]
	if sp == nil {
		return nil
	}
	return sp[p.TermKey()]
}

// Subjects returns all subjects of triples matching (?, p, o).
func (g *Graph) Subjects(p, o Term) []Term {
	po := g.ensureIndexes().pos[p.TermKey()]
	if po == nil {
		return nil
	}
	return po[o.TermKey()]
}

// RDFList follows an RDF list starting at head and returns all elements.
func (g *Graph) RDFList(head Term) []Term {
	var result []Term
	current := head
	nilTerm := IRI(RDFNil)
	firstPred := IRI(RDFFirst)
	restPred := IRI(RDFRest)
	visited := make(map[string]bool)
	for {
		if current.Equal(nilTerm) {
			break
		}
		key := current.TermKey()
		if visited[key] {
			break // cycle detected
		}
		visited[key] = true
		firsts := g.Objects(current, firstPred)
		if len(firsts) == 0 {
			break
		}
		result = append(result, firsts[0])
		rests := g.Objects(current, restPred)
		if len(rests) == 0 {
			break
		}
		current = rests[0]
	}
	return result
}

// Len returns the number of triples in the graph.
func (g *Graph) Len() int {
	return g.g.Len()
}

// Add inserts a triple into the graph.
func (g *Graph) Add(s, p, o Term) {
	g.invalidateIndexes()
	sub := toSubject(s)
	pred := toURIRef(p)
	obj := toTerm(o)
	g.g.Add(sub, pred, obj)
}

// Merge adds all triples from other into this graph.
func (g *Graph) Merge(other *Graph) {
	g.invalidateIndexes()
	other.g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		g.g.Add(t.Subject.(term.Subject), t.Predicate, t.Object)
		return true
	})
}

// BaseURI returns the base URI of the graph.
func (g *Graph) BaseURI() string {
	return g.baseURI
}

// HasType checks if a node has rdf:type of the given class (including subclass reasoning).
func (g *Graph) HasType(node, class Term) bool {
	typePred := IRI(RDFType)
	types := g.Objects(node, typePred)
	for _, t := range types {
		if t.Equal(class) || g.IsSubClassOf(t, class) {
			return true
		}
	}
	return false
}

// IsSubClassOf checks if sub is a subclass of super (transitive).
func (g *Graph) IsSubClassOf(sub, super Term) bool {
	return g.isSubClassOfVisited(sub, super, make(map[string]bool))
}

func (g *Graph) isSubClassOfVisited(sub, super Term, visited map[string]bool) bool {
	if sub.Equal(super) {
		return true
	}
	key := sub.TermKey()
	if visited[key] {
		return false
	}
	visited[key] = true
	subclassPred := IRI(RDFSSubClassOf)
	parents := g.Objects(sub, subclassPred)
	for _, parent := range parents {
		if g.isSubClassOfVisited(parent, super, visited) {
			return true
		}
	}
	return false
}
