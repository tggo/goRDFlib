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
type Graph struct {
	g       *graph.Graph
	baseURI string

	spoIdx map[string]map[string][]Term // subject → predicate → []object
	posIdx map[string]map[string][]Term // predicate → object → []subject
	pIdx   map[string][]Triple          // predicate → []Triple
}

// NewGraph creates an empty graph with no base URI.
func NewGraph() *Graph {
	return &Graph{g: graph.NewGraph()}
}

// LoadTurtleFile loads a Turtle file from disk.
func LoadTurtleFile(path string) (*Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	base := "file://" + path
	g := graph.NewGraph(graph.WithBase(base))
	if err := turtle.Parse(g, f, turtle.WithBase(base)); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &Graph{g: g, baseURI: base}, nil
}

// LoadTurtle parses Turtle data from a reader.
func LoadTurtle(r io.Reader, base string) (*Graph, error) {
	g := graph.NewGraph(graph.WithBase(base))
	if err := turtle.Parse(g, r, turtle.WithBase(base)); err != nil {
		return nil, err
	}
	return &Graph{g: g, baseURI: base}, nil
}

// LoadTurtleString parses Turtle data from a string.
func LoadTurtleString(data, base string) (*Graph, error) {
	return LoadTurtle(strings.NewReader(data), base)
}

// LoadJsonLDFile loads a JSON-LD file from disk.
func LoadJsonLDFile(path string) (*Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	base := "file://" + path
	g := graph.NewGraph(graph.WithBase(base))
	if err := jsonld.Parse(g, f, jsonld.WithBase(base)); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &Graph{g: g, baseURI: base}, nil
}

// LoadJsonLD parses JSON-LD data from a reader.
func LoadJsonLD(r io.Reader, base string) (*Graph, error) {
	g := graph.NewGraph(graph.WithBase(base))
	if err := jsonld.Parse(g, r, jsonld.WithBase(base)); err != nil {
		return nil, err
	}
	return &Graph{g: g, baseURI: base}, nil
}

// LoadJsonLDString parses JSON-LD data from a string.
func LoadJsonLDString(data, base string) (*Graph, error) {
	return LoadJsonLD(strings.NewReader(data), base)
}

// LoadNQuadsFile loads an N-Quads file from disk.
func LoadNQuadsFile(path string) (*Graph, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	base := "file://" + path
	g := graph.NewGraph(graph.WithBase(base))
	if err := nq.Parse(g, f, nq.WithBase(base)); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &Graph{g: g, baseURI: base}, nil
}

// LoadNQuads parses N-Quads data from a reader.
func LoadNQuads(r io.Reader, base string) (*Graph, error) {
	g := graph.NewGraph(graph.WithBase(base))
	if err := nq.Parse(g, r, nq.WithBase(base)); err != nil {
		return nil, err
	}
	return &Graph{g: g, baseURI: base}, nil
}

// LoadNQuadsString parses N-Quads data from a string.
func LoadNQuadsString(data, base string) (*Graph, error) {
	return LoadNQuads(strings.NewReader(data), base)
}

func (g *Graph) ensureIndexes() {
	if g.spoIdx != nil {
		return
	}
	g.spoIdx = make(map[string]map[string][]Term)
	g.posIdx = make(map[string]map[string][]Term)
	g.pIdx = make(map[string][]Triple)
	g.g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		s := fromRDFLib(t.Subject)
		p := fromRDFLib(t.Predicate)
		o := fromRDFLib(t.Object)
		sk, pk, ok := s.TermKey(), p.TermKey(), o.TermKey()

		sp := g.spoIdx[sk]
		if sp == nil {
			sp = make(map[string][]Term)
			g.spoIdx[sk] = sp
		}
		sp[pk] = append(sp[pk], o)

		po := g.posIdx[pk]
		if po == nil {
			po = make(map[string][]Term)
			g.posIdx[pk] = po
		}
		po[ok] = append(po[ok], s)

		g.pIdx[pk] = append(g.pIdx[pk], Triple{Subject: s, Predicate: p, Object: o})
		return true
	})
}

func (g *Graph) invalidateIndexes() {
	g.spoIdx = nil
	g.posIdx = nil
	g.pIdx = nil
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
func (g *Graph) All(s, p, o *Term) []Triple {
	g.ensureIndexes()

	if s != nil && p != nil && o == nil {
		objs := g.spoIdx[s.TermKey()][p.TermKey()]
		result := make([]Triple, len(objs))
		for i, obj := range objs {
			result[i] = Triple{Subject: *s, Predicate: *p, Object: obj}
		}
		return result
	}

	if s == nil && p != nil && o != nil {
		subs := g.posIdx[p.TermKey()][o.TermKey()]
		result := make([]Triple, len(subs))
		for i, sub := range subs {
			result[i] = Triple{Subject: sub, Predicate: *p, Object: *o}
		}
		return result
	}

	if s == nil && p != nil && o == nil {
		return g.pIdx[p.TermKey()]
	}

	// Fallback: subject-only or all wildcards
	if s != nil && p == nil && o == nil {
		sk := s.TermKey()
		sp, ok := g.spoIdx[sk]
		if !ok {
			return nil
		}
		var result []Triple
		for _, objs := range sp {
			for _, obj := range objs {
				result = append(result, Triple{Subject: *s, Predicate: obj, Object: obj})
			}
		}
		// Wrong — need to iterate properly. Use rdflibgo.
		result = nil
		sub := toSubject(*s)
		g.g.Triples(sub, nil, nil)(func(t term.Triple) bool {
			result = append(result, Triple{
				Subject:   fromRDFLib(t.Subject),
				Predicate: fromRDFLib(t.Predicate),
				Object:    fromRDFLib(t.Object),
			})
			return true
		})
		return result
	}

	// All wildcards
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
	g.ensureIndexes()
	sp := g.spoIdx[s.TermKey()]
	if sp == nil {
		return nil
	}
	return sp[p.TermKey()]
}

// Subjects returns all subjects of triples matching (?, p, o).
func (g *Graph) Subjects(p, o Term) []Term {
	g.ensureIndexes()
	po := g.posIdx[p.TermKey()]
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
