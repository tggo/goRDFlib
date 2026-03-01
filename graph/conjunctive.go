package graph

import (
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// ConjunctiveGraph manages multiple named graphs over a single store.
// Queries execute against the union of all graphs by default.
// Ported from: rdflib.graph.ConjunctiveGraph
type ConjunctiveGraph struct {
	store          store.Store
	defaultContext *Graph
}

// NewConjunctiveGraph creates a new ConjunctiveGraph.
// Ported from: rdflib.graph.ConjunctiveGraph.__init__
func NewConjunctiveGraph(opts ...GraphOption) *ConjunctiveGraph {
	g := &Graph{}
	for _, opt := range opts {
		opt(g)
	}
	if g.store == nil {
		g.store = store.NewMemoryStore()
	}
	if g.identifier == nil {
		g.identifier = term.NewBNode()
	}
	defaultCtx := &Graph{store: g.store, identifier: g.identifier}
	defaultCtx.Bind("rdf", term.NewURIRefUnsafe(term.RDFNamespace))
	defaultCtx.Bind("rdfs", term.NewURIRefUnsafe(namespace.RDFSNamespace))
	defaultCtx.Bind("xsd", term.NewURIRefUnsafe(term.XSDNamespace))
	defaultCtx.Bind("owl", term.NewURIRefUnsafe(namespace.OWLNamespace))

	return &ConjunctiveGraph{
		store:          g.store,
		defaultContext: defaultCtx,
	}
}

// DefaultContext returns the default graph.
func (cg *ConjunctiveGraph) DefaultContext() *Graph {
	return cg.defaultContext
}

// Store returns the underlying store.
func (cg *ConjunctiveGraph) Store() store.Store {
	return cg.store
}

// GetContext returns a Graph for the given identifier, sharing the same store.
// Ported from: rdflib.graph.ConjunctiveGraph.get_context
func (cg *ConjunctiveGraph) GetContext(id term.Term) *Graph {
	if id == nil {
		return cg.defaultContext
	}
	return &Graph{store: cg.store, identifier: id}
}

// Add adds a triple to the specified context (nil = default graph).
// Ported from: rdflib.graph.ConjunctiveGraph.add
func (cg *ConjunctiveGraph) Add(s term.Subject, p term.URIRef, o term.Term, ctx term.Term) {
	if ctx == nil {
		ctx = cg.defaultContext.identifier
	}
	cg.store.Add(term.Triple{Subject: s, Predicate: p, Object: o}, ctx)
}

// AddQuad adds a quad to the graph.
func (cg *ConjunctiveGraph) AddQuad(q term.Quad) {
	ctx := term.Term(cg.defaultContext.identifier)
	if q.Graph != nil {
		ctx = q.Graph
	}
	cg.store.Add(q.Triple, ctx)
}

// Remove removes matching triples. If ctx is nil, removes from all contexts.
// Ported from: rdflib.graph.ConjunctiveGraph.remove
func (cg *ConjunctiveGraph) Remove(s term.Subject, p *term.URIRef, o term.Term, ctx term.Term) {
	cg.store.Remove(term.TriplePattern{Subject: s, Predicate: p, Object: o}, ctx)
}

// Triples iterates over matching triples across all contexts (union view).
// Ported from: rdflib.graph.ConjunctiveGraph.triples
func (cg *ConjunctiveGraph) Triples(s term.Subject, p *term.URIRef, o term.Term) store.TripleIterator {
	return cg.store.Triples(term.TriplePattern{Subject: s, Predicate: p, Object: o}, nil)
}

// Quads iterates over matching quads across all contexts.
// Ported from: rdflib.graph.ConjunctiveGraph.quads
func (cg *ConjunctiveGraph) Quads(s term.Subject, p *term.URIRef, o term.Term) func(yield func(term.Quad) bool) {
	return func(yield func(term.Quad) bool) {
		graphID, _ := cg.defaultContext.identifier.(term.Subject)
		cg.store.Triples(term.TriplePattern{Subject: s, Predicate: p, Object: o}, nil)(func(t term.Triple) bool {
			return yield(term.Quad{Triple: t, Graph: graphID})
		})
	}
}

// Contexts returns all contexts (named graphs).
// Ported from: rdflib.graph.ConjunctiveGraph.contexts
func (cg *ConjunctiveGraph) Contexts(triple *term.Triple) store.TermIterator {
	return cg.store.Contexts(triple)
}

// Len returns total number of triples across all contexts.
func (cg *ConjunctiveGraph) Len() int {
	return cg.store.Len(nil)
}

// Bind associates a prefix with a namespace.
func (cg *ConjunctiveGraph) Bind(prefix string, ns term.URIRef) {
	cg.store.Bind(prefix, ns)
}

// Namespaces returns all namespace bindings.
func (cg *ConjunctiveGraph) Namespaces() store.NamespaceIterator {
	return cg.store.Namespaces()
}
