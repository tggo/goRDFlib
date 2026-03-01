// Package rdflibgo is an RDF library for Go, ported from Python's rdflib.
//
// This root package re-exports all public types and functions from
// subpackages for backward compatibility. New code may import
// subpackages directly (term, store, namespace, graph, paths, plugin, testutil).
package rdflibgo

import (
	"iter"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/plugin"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/testutil"
)

// ---- term package re-exports ----

// Core interfaces
type Term = term.Term
type Subject = term.Subject
type Predicate = term.Predicate
type NamespaceManager = term.NamespaceManager

// Concrete types
type URIRef = term.URIRef
type BNode = term.BNode
type Literal = term.Literal
type Variable = term.Variable
type LiteralOption = term.LiteralOption
type Triple = term.Triple
type Quad = term.Quad
type TriplePattern = term.TriplePattern
type TermSlice = term.TermSlice

// Constructors & functions
var NewURIRef = term.NewURIRef
var NewURIRefUnsafe = term.NewURIRefUnsafe
var NewURIRefWithBase = term.NewURIRefWithBase
var NewBNode = term.NewBNode
var NewLiteral = term.NewLiteral
var NewVariable = term.NewVariable
var WithLang = term.WithLang
var WithDatatype = term.WithDatatype
var CompareTerm = term.CompareTerm
var SortTerms = term.SortTerms
var GoToLexical = term.GoToLexical

// Errors
var ErrInvalidIRI = term.ErrInvalidIRI
var ErrUnknownFormat = term.ErrUnknownFormat
var ErrTermNotInNamespace = term.ErrTermNotInNamespace
var ErrInvalidCURIE = term.ErrInvalidCURIE
var ErrPrefixNotBound = term.ErrPrefixNotBound

// XSD constants
const XSDNamespace = term.XSDNamespace
const RDFNamespace = term.RDFNamespace

var XSDString = term.XSDString
var XSDInteger = term.XSDInteger
var XSDInt = term.XSDInt
var XSDLong = term.XSDLong
var XSDFloat = term.XSDFloat
var XSDDouble = term.XSDDouble
var XSDDecimal = term.XSDDecimal
var XSDBoolean = term.XSDBoolean
var XSDDateTime = term.XSDDateTime
var XSDDate = term.XSDDate
var XSDTime = term.XSDTime
var XSDAnyURI = term.XSDAnyURI
var RDFLangString = term.RDFLangString

// ---- store package re-exports ----

type Store = store.Store
type MemoryStore = store.MemoryStore

// Iterator types
type TripleIterator = iter.Seq[Triple]
type TermIterator = iter.Seq[Term]
type TermPairIterator = iter.Seq2[Term, Term]
type NamespaceIterator = iter.Seq2[string, URIRef]

var NewMemoryStore = store.NewMemoryStore

// ---- namespace package re-exports ----

type Namespace = namespace.Namespace
type ClosedNamespace = namespace.ClosedNamespace
type NSManager = namespace.NSManager

var NewNamespace = namespace.NewNamespace
var NewClosedNamespace = namespace.NewClosedNamespace
var NewNSManager = namespace.NewNSManager

var RDF = namespace.RDF
var RDFS = namespace.RDFS
var OWL = namespace.OWL
var FOAF = namespace.FOAF
var DC = namespace.DC
var DCTERMS = namespace.DCTERMS
var SKOS = namespace.SKOS
var PROV = namespace.PROV
var SH = namespace.SH
var SOSA = namespace.SOSA
var SSN = namespace.SSN
var DCAT = namespace.DCAT
var VOID = namespace.VOID

const RDFSNamespace = namespace.RDFSNamespace
const OWLNamespace = namespace.OWLNamespace

// ---- graph package re-exports ----

type Graph = graph.Graph
type GraphOption = graph.GraphOption
type Resource = graph.Resource
type Collection = graph.Collection
type ConjunctiveGraph = graph.ConjunctiveGraph
type Dataset = graph.Dataset

var NewGraph = graph.NewGraph
var WithStore = graph.WithStore
var WithIdentifier = graph.WithIdentifier
var WithBase = graph.WithBase
var NewResource = graph.NewResource
var NewCollection = graph.NewCollection
var NewEmptyCollection = graph.NewEmptyCollection
var NewConjunctiveGraph = graph.NewConjunctiveGraph
var NewDataset = graph.NewDataset

// ---- paths package re-exports ----

type Path = paths.Path
type InvPath = paths.InvPath
type SequencePath = paths.SequencePath
type AlternativePath = paths.AlternativePath
type MulPath = paths.MulPath
type NegatedPath = paths.NegatedPath
type URIRefPath = paths.URIRefPath

var Inv = paths.Inv
var Sequence = paths.Sequence
var Alternative = paths.Alternative
var ZeroOrMore = paths.ZeroOrMore
var OneOrMore = paths.OneOrMore
var ZeroOrOne = paths.ZeroOrOne
var Negated = paths.Negated
var AsPath = paths.AsPath

// ---- plugin package re-exports ----

type Parser = plugin.Parser
type Serializer = plugin.Serializer

var RegisterParser = plugin.RegisterParser
var GetParser = plugin.GetParser
var RegisterSerializer = plugin.RegisterSerializer
var GetSerializer = plugin.GetSerializer
var RegisterStore = plugin.RegisterStore
var GetStore = plugin.GetStore
var FormatFromFilename = plugin.FormatFromFilename
var FormatFromMIME = plugin.FormatFromMIME
var FormatFromContent = plugin.FormatFromContent

// ---- testutil package re-exports ----

// AssertGraphEqual checks that two graphs contain the same triples.
func AssertGraphEqual(t *testing.T, expected, actual *Graph) {
	testutil.AssertGraphEqual(t, expected, actual)
}

// AssertGraphContains checks that the graph contains a specific triple.
func AssertGraphContains(t *testing.T, g *Graph, s Subject, p URIRef, o Term) {
	testutil.AssertGraphContains(t, g, s, p, o)
}

// AssertGraphLen checks the number of triples in a graph.
func AssertGraphLen(t *testing.T, g *Graph, expected int) {
	testutil.AssertGraphLen(t, g, expected)
}

