package graph

import (
	"context"
	"iter"

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

// NewConjunctiveGraph creates a new ConjunctiveGraph. Like NewGraph, it binds
// the default prefixes only over a MemoryStore; see Graph.BindDefaults.
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
		g.identifier = store.DefaultGraph
	}
	defaultCtx := &Graph{store: g.store, identifier: g.identifier}
	if _, ok := g.store.(*store.MemoryStore); ok {
		_ = defaultCtx.BindDefaults(context.Background())
	}
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
func (cg *ConjunctiveGraph) GetContext(id term.Term) *Graph {
	if id == nil {
		return cg.defaultContext
	}
	return &Graph{store: cg.store, identifier: id}
}

// Add adds a triple to the graph named graph (nil = default graph).
func (cg *ConjunctiveGraph) Add(ctx context.Context, s term.Subject, p term.URIRef, o term.Term, graph term.Term) error {
	if graph == nil {
		graph = cg.defaultContext.identifier
	}
	return cg.store.Add(ctx, term.Triple{Subject: s, Predicate: p, Object: o}, graph)
}

// AddQuad adds a quad to the graph.
func (cg *ConjunctiveGraph) AddQuad(ctx context.Context, q term.Quad) error {
	graph := term.Term(cg.defaultContext.identifier)
	if q.Graph != nil {
		graph = q.Graph
	}
	return cg.store.Add(ctx, q.Triple, graph)
}

// Remove removes matching triples from graph, or from the default graph and
// every named graph when graph is nil.
func (cg *ConjunctiveGraph) Remove(ctx context.Context, s term.Subject, p *term.URIRef, o term.Term, graph term.Term) error {
	pat := term.TriplePattern{Subject: s, Predicate: p, Object: o}
	if graph != nil {
		return cg.store.Remove(ctx, pat, graph)
	}
	graphs, err := cg.allContexts(ctx)
	if err != nil {
		return err
	}
	for _, c := range graphs {
		if err := cg.store.Remove(ctx, pat, c); err != nil {
			return err
		}
	}
	return nil
}

// allContexts lists the default graph (as nil) followed by every named graph.
// The list is materialized first so that no store read lock is held while the
// caller goes on to read or write each graph.
func (cg *ConjunctiveGraph) allContexts(ctx context.Context) ([]term.Term, error) {
	graphs := []term.Term{nil}
	for c, err := range cg.store.Contexts(ctx, nil) {
		if err != nil {
			return nil, err
		}
		graphs = append(graphs, c)
	}
	return graphs, nil
}

// Triples iterates over matching triples across all contexts (union view). A
// triple present in several graphs is yielded once.
func (cg *ConjunctiveGraph) Triples(ctx context.Context, s term.Subject, p *term.URIRef, o term.Term) store.TripleIterator {
	pat := term.TriplePattern{Subject: s, Predicate: p, Object: o}
	return func(yield func(term.Triple, error) bool) {
		graphs, err := cg.allContexts(ctx)
		if err != nil {
			yield(term.Triple{}, err)
			return
		}
		if len(graphs) == 1 {
			cg.store.Triples(ctx, pat, nil)(yield)
			return
		}
		seen := make(map[string]struct{})
		for _, c := range graphs {
			for t, err := range cg.store.Triples(ctx, pat, c) {
				if err != nil {
					yield(term.Triple{}, err)
					return
				}
				k := term.TermKey(t.Subject) + "\x00" + term.TermKey(t.Predicate) + "\x00" + term.TermKey(t.Object)
				if _, dup := seen[k]; dup {
					continue
				}
				seen[k] = struct{}{}
				if !yield(t, nil) {
					return
				}
			}
		}
	}
}

// Quads iterates over matching quads across all contexts. A triple in the
// default graph carries the default context's identifier.
func (cg *ConjunctiveGraph) Quads(ctx context.Context, s term.Subject, p *term.URIRef, o term.Term) iter.Seq2[term.Quad, error] {
	pat := term.TriplePattern{Subject: s, Predicate: p, Object: o}
	return func(yield func(term.Quad, error) bool) {
		graphs, err := cg.allContexts(ctx)
		if err != nil {
			yield(term.Quad{}, err)
			return
		}
		defaultID, _ := cg.defaultContext.identifier.(term.Subject)
		for _, c := range graphs {
			graphID := defaultID
			if c != nil {
				graphID, _ = c.(term.Subject)
			}
			for t, err := range cg.store.Triples(ctx, pat, c) {
				if err != nil {
					yield(term.Quad{}, err)
					return
				}
				if !yield(term.Quad{Triple: t, Graph: graphID}, nil) {
					return
				}
			}
		}
	}
}

// Contexts returns all contexts (named graphs).
func (cg *ConjunctiveGraph) Contexts(ctx context.Context, triple *term.Triple) store.TermIterator {
	return cg.store.Contexts(ctx, triple)
}

// Len returns the number of distinct triples across all contexts.
func (cg *ConjunctiveGraph) Len(ctx context.Context) (int, error) {
	graphs, err := cg.allContexts(ctx)
	if err != nil {
		return 0, err
	}
	if len(graphs) == 1 {
		return cg.store.Len(ctx, nil)
	}
	n := 0
	for _, err := range cg.Triples(ctx, nil, nil, nil) {
		if err != nil {
			return 0, err
		}
		n++
	}
	return n, nil
}

// Bind associates a prefix with a namespace.
func (cg *ConjunctiveGraph) Bind(ctx context.Context, prefix string, ns term.URIRef) error {
	return cg.store.Bind(ctx, prefix, ns)
}

// Namespaces returns all namespace bindings.
func (cg *ConjunctiveGraph) Namespaces(ctx context.Context) store.NamespaceIterator {
	return cg.store.Namespaces(ctx)
}
