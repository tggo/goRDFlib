package rdflibgo

// ConjunctiveGraph manages multiple named graphs over a single store.
// Queries execute against the union of all graphs by default.
// Ported from: rdflib.graph.ConjunctiveGraph
type ConjunctiveGraph struct {
	store          Store
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
		g.store = NewMemoryStore()
	}
	if g.identifier == nil {
		g.identifier = NewBNode()
	}
	defaultCtx := &Graph{store: g.store, identifier: g.identifier}
	defaultCtx.Bind("rdf", NewURIRefUnsafe(RDFNamespace))
	defaultCtx.Bind("rdfs", NewURIRefUnsafe(RDFSNamespace))
	defaultCtx.Bind("xsd", NewURIRefUnsafe(XSDNamespace))
	defaultCtx.Bind("owl", NewURIRefUnsafe(OWLNamespace))

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
func (cg *ConjunctiveGraph) Store() Store {
	return cg.store
}

// GetContext returns a Graph for the given identifier, sharing the same store.
// Ported from: rdflib.graph.ConjunctiveGraph.get_context
func (cg *ConjunctiveGraph) GetContext(id Term) *Graph {
	if id == nil {
		return cg.defaultContext
	}
	return &Graph{store: cg.store, identifier: id}
}

// Add adds a triple to the specified context (nil = default graph).
// Ported from: rdflib.graph.ConjunctiveGraph.add
func (cg *ConjunctiveGraph) Add(s Subject, p URIRef, o Term, ctx Term) {
	if ctx == nil {
		ctx = cg.defaultContext.identifier
	}
	cg.store.Add(Triple{Subject: s, Predicate: p, Object: o}, ctx)
}

// AddQuad adds a quad to the graph.
func (cg *ConjunctiveGraph) AddQuad(q Quad) {
	ctx := Term(cg.defaultContext.identifier)
	if q.Graph != nil {
		ctx = q.Graph
	}
	cg.store.Add(q.Triple, ctx)
}

// Remove removes matching triples. If ctx is nil, removes from all contexts.
// Ported from: rdflib.graph.ConjunctiveGraph.remove
func (cg *ConjunctiveGraph) Remove(s Subject, p *URIRef, o Term, ctx Term) {
	cg.store.Remove(TriplePattern{Subject: s, Predicate: p, Object: o}, ctx)
}

// Triples iterates over matching triples across all contexts (union view).
// Ported from: rdflib.graph.ConjunctiveGraph.triples
func (cg *ConjunctiveGraph) Triples(s Subject, p *URIRef, o Term) TripleIterator {
	return cg.store.Triples(TriplePattern{Subject: s, Predicate: p, Object: o}, nil)
}

// Quads iterates over matching quads across all contexts.
// Ported from: rdflib.graph.ConjunctiveGraph.quads
func (cg *ConjunctiveGraph) Quads(s Subject, p *URIRef, o Term) func(yield func(Quad) bool) {
	return func(yield func(Quad) bool) {
		// For SimpleMemory store (non-context-aware), just wrap triples
		cg.store.Triples(TriplePattern{Subject: s, Predicate: p, Object: o}, nil)(func(t Triple) bool {
			return yield(Quad{Triple: t, Graph: nil})
		})
	}
}

// Contexts returns all contexts (named graphs).
// Ported from: rdflib.graph.ConjunctiveGraph.contexts
func (cg *ConjunctiveGraph) Contexts(triple *Triple) TermIterator {
	return cg.store.Contexts(triple)
}

// Len returns total number of triples across all contexts.
func (cg *ConjunctiveGraph) Len() int {
	return cg.store.Len(nil)
}

// Bind associates a prefix with a namespace.
func (cg *ConjunctiveGraph) Bind(prefix string, namespace URIRef) {
	cg.store.Bind(prefix, namespace)
}

// Namespaces returns all namespace bindings.
func (cg *ConjunctiveGraph) Namespaces() NamespaceIterator {
	return cg.store.Namespaces()
}
