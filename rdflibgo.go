// Package rdflibgo is an RDF library for Go, ported from Python's rdflib.
//
// This root package re-exports all public types and functions from
// subpackages for backward compatibility. New code may import
// subpackages directly: term, store, namespace, graph, paths, plugin, testutil.
package rdflibgo

import (
	"iter"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/plugin"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// --- Core interfaces and types (from term/) ---

type (
	Term             = term.Term
	Subject          = term.Subject
	Predicate        = term.Predicate
	NamespaceManager = term.NamespaceManager
	URIRef           = term.URIRef
	BNode            = term.BNode
	Literal          = term.Literal
	Variable         = term.Variable
	LiteralOption    = term.LiteralOption
	Triple           = term.Triple
	Quad             = term.Quad
	TriplePattern    = term.TriplePattern
	TermSlice        = term.TermSlice
)

// --- Store types (from store/) ---

type (
	Store             = store.Store
	MemoryStore       = store.MemoryStore
	TripleIterator    = iter.Seq[Triple]
	TermIterator      = iter.Seq[Term]
	TermPairIterator  = iter.Seq2[Term, Term]
	NamespaceIterator = iter.Seq2[string, URIRef]
)

// --- Namespace types (from namespace/) ---

type (
	Namespace       = namespace.Namespace
	ClosedNamespace = namespace.ClosedNamespace
	NSManager       = namespace.NSManager
)

// --- Graph types (from graph/) ---

type (
	Graph            = graph.Graph
	GraphOption      = graph.GraphOption
	Resource         = graph.Resource
	Collection       = graph.Collection
	ConjunctiveGraph = graph.ConjunctiveGraph
	Dataset          = graph.Dataset
)

// --- Path types (from paths/) ---

type (
	Path            = paths.Path
	InvPath         = paths.InvPath
	SequencePath    = paths.SequencePath
	AlternativePath = paths.AlternativePath
	MulPath         = paths.MulPath
	NegatedPath     = paths.NegatedPath
	URIRefPath      = paths.URIRefPath
)

// --- Plugin types (from plugin/) ---

type (
	Parser     = plugin.Parser
	Serializer = plugin.Serializer
)

// --- Namespace constants ---

const (
	XSDNamespace  = term.XSDNamespace
	RDFNamespace  = term.RDFNamespace
	RDFSNamespace = namespace.RDFSNamespace
	OWLNamespace  = namespace.OWLNamespace
)

// --- Sentinel errors ---

var (
	ErrInvalidIRI         = term.ErrInvalidIRI
	ErrUnknownFormat      = term.ErrUnknownFormat
	ErrTermNotInNamespace = term.ErrTermNotInNamespace
	ErrInvalidCURIE       = term.ErrInvalidCURIE
	ErrPrefixNotBound     = term.ErrPrefixNotBound
)

// --- XSD datatype URIs ---

var (
	XSDString     = term.XSDString
	XSDInteger    = term.XSDInteger
	XSDInt        = term.XSDInt
	XSDLong       = term.XSDLong
	XSDFloat      = term.XSDFloat
	XSDDouble     = term.XSDDouble
	XSDDecimal    = term.XSDDecimal
	XSDBoolean    = term.XSDBoolean
	XSDDateTime   = term.XSDDateTime
	XSDDate       = term.XSDDate
	XSDTime       = term.XSDTime
	XSDAnyURI     = term.XSDAnyURI
	RDFLangString = term.RDFLangString
)

// --- Built-in namespace instances ---

var (
	RDF     = namespace.RDF
	RDFS    = namespace.RDFS
	OWL     = namespace.OWL
	FOAF    = namespace.FOAF
	DC      = namespace.DC
	DCTERMS = namespace.DCTERMS
	SKOS    = namespace.SKOS
	PROV    = namespace.PROV
	SH      = namespace.SH
	SOSA    = namespace.SOSA
	SSN     = namespace.SSN
	DCAT    = namespace.DCAT
	VOID    = namespace.VOID
)

// --- Term constructors ---

func NewURIRef(value string) (URIRef, error) { return term.NewURIRef(value) }
func NewURIRefUnsafe(value string) URIRef    { return term.NewURIRefUnsafe(value) }
func NewURIRefWithBase(value, base string) (URIRef, error) {
	return term.NewURIRefWithBase(value, base)
}
func NewBNode(id ...string) BNode                         { return term.NewBNode(id...) }
func NewLiteral(value any, opts ...LiteralOption) Literal { return term.NewLiteral(value, opts...) }
func NewVariable(name string) Variable                    { return term.NewVariable(name) }
func WithLang(lang string) LiteralOption                  { return term.WithLang(lang) }
func WithDatatype(dt URIRef) LiteralOption                { return term.WithDatatype(dt) }
func GoToLexical(value any) (string, URIRef)              { return term.GoToLexical(value) }
func CompareTerm(a, b Term) int                           { return term.CompareTerm(a, b) }
func SortTerms(terms []Term)                              { term.SortTerms(terms) }

// --- Store constructors ---

func NewMemoryStore() *MemoryStore { return store.NewMemoryStore() }

// --- Namespace constructors ---

func NewNamespace(base string) Namespace { return namespace.NewNamespace(base) }
func NewClosedNamespace(base string, terms []string) ClosedNamespace {
	return namespace.NewClosedNamespace(base, terms)
}
func NewNSManager(s Store) *NSManager { return namespace.NewNSManager(s) }

// --- Graph constructors ---

func NewGraph(opts ...GraphOption) *Graph              { return graph.NewGraph(opts...) }
func WithStore(s Store) GraphOption                    { return graph.WithStore(s) }
func WithIdentifier(id Term) GraphOption               { return graph.WithIdentifier(id) }
func WithBase(base string) GraphOption                 { return graph.WithBase(base) }
func NewResource(g *Graph, id Subject) *Resource       { return graph.NewResource(g, id) }
func NewCollection(g *Graph, head Subject) *Collection { return graph.NewCollection(g, head) }
func NewEmptyCollection(g *Graph) *Collection          { return graph.NewEmptyCollection(g) }
func NewConjunctiveGraph(opts ...GraphOption) *ConjunctiveGraph {
	return graph.NewConjunctiveGraph(opts...)
}
func NewDataset(opts ...GraphOption) *Dataset { return graph.NewDataset(opts...) }

// --- Path constructors ---

func AsPath(u URIRef) URIRefPath              { return paths.AsPath(u) }
func Inv(p Path) *InvPath                     { return paths.Inv(p) }
func Sequence(ps ...Path) *SequencePath       { return paths.Sequence(ps...) }
func Alternative(ps ...Path) *AlternativePath { return paths.Alternative(ps...) }
func ZeroOrMore(p Path) *MulPath              { return paths.ZeroOrMore(p) }
func OneOrMore(p Path) *MulPath               { return paths.OneOrMore(p) }
func ZeroOrOne(p Path) *MulPath               { return paths.ZeroOrOne(p) }
func Negated(excluded ...URIRef) *NegatedPath { return paths.Negated(excluded...) }

// --- Plugin registry ---

func RegisterParser(name string, factory func() Parser) { plugin.RegisterParser(name, factory) }
func GetParser(name string) (Parser, bool)              { return plugin.GetParser(name) }
func RegisterSerializer(name string, factory func() Serializer) {
	plugin.RegisterSerializer(name, factory)
}
func GetSerializer(name string) (Serializer, bool)    { return plugin.GetSerializer(name) }
func RegisterStore(name string, factory func() Store) { plugin.RegisterStore(name, factory) }
func GetStore(name string) (Store, bool)              { return plugin.GetStore(name) }

// --- Format detection ---

func FormatFromFilename(filename string) (string, bool) { return plugin.FormatFromFilename(filename) }
func FormatFromMIME(contentType string) (string, bool)  { return plugin.FormatFromMIME(contentType) }
func FormatFromContent(data []byte) (string, bool)      { return plugin.FormatFromContent(data) }
