package graph

import (
	"context"
	"sync"

	"github.com/tggo/goRDFlib/term"
)

// Dataset extends ConjunctiveGraph with explicit named graph management.
// Safe for concurrent use.
// Ported from: rdflib.graph.Dataset
type Dataset struct {
	*ConjunctiveGraph
	mu     sync.RWMutex
	graphs map[string]*Graph // identifier key → Graph
}

// NewDataset creates a new Dataset.
// Ported from: rdflib.graph.Dataset.__init__
func NewDataset(opts ...GraphOption) *Dataset {
	cg := NewConjunctiveGraph(opts...)
	ds := &Dataset{
		ConjunctiveGraph: cg,
		graphs:           make(map[string]*Graph),
	}
	ds.graphs[term.TermKey(cg.defaultContext.identifier)] = cg.defaultContext
	return ds
}

// Graph returns or creates a named graph with the given identifier.
// Ported from: rdflib.graph.Dataset.graph
func (ds *Dataset) Graph(id term.Term) *Graph {
	if id == nil {
		return ds.DefaultContext()
	}
	k := term.TermKey(id)
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if g, ok := ds.graphs[k]; ok {
		return g
	}
	g := &Graph{store: ds.store, identifier: id}
	ds.graphs[k] = g
	return g
}

// AddGraph registers a graph in the dataset.
// Ported from: rdflib.graph.Dataset.add_graph
func (ds *Dataset) AddGraph(g *Graph) {
	ds.mu.Lock()
	defer ds.mu.Unlock()
	ds.graphs[term.TermKey(g.identifier)] = g
}

// RemoveGraph removes a named graph and its triples from the dataset. The
// graph stays registered when removing its triples fails.
// Ported from: rdflib.graph.Dataset.remove_graph
func (ds *Dataset) RemoveGraph(ctx context.Context, id term.Term) error {
	if id == nil {
		return nil
	}
	k := term.TermKey(id)
	if k == term.TermKey(ds.defaultContext.identifier) {
		return nil
	}
	ds.mu.Lock()
	defer ds.mu.Unlock()
	if g, ok := ds.graphs[k]; ok {
		if err := ds.store.Remove(ctx, term.TriplePattern{}, g.identifier); err != nil {
			return err
		}
		delete(ds.graphs, k)
	}
	return nil
}

// Graphs returns all named graphs in the dataset.
// Iteration order is non-deterministic.
// Ported from: rdflib.graph.Dataset.graphs
func (ds *Dataset) Graphs() func(yield func(*Graph) bool) {
	return func(yield func(*Graph) bool) {
		ds.mu.RLock()
		defer ds.mu.RUnlock()
		for _, g := range ds.graphs {
			if !yield(g) {
				return
			}
		}
	}
}
