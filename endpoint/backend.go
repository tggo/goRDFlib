package endpoint

import (
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// backend adapts a dataset to what the handler needs. Every method is called
// with the handler's lock held: view and graph under at least the read lock,
// the others under the write lock.
type backend interface {
	// view returns the dataset as the query engine sees it: the default graph
	// and the named graphs by IRI. The map must not be modified.
	view() (def *graph.Graph, named map[string]*graph.Graph)
	// graph returns the named graph with this IRI, if it exists.
	graph(iri string) (*graph.Graph, bool)
	// updateDataset returns the sparql.Dataset an update runs against.
	updateDataset() *sparql.Dataset
	// commit is called after an update ran against ds, whether it failed or
	// not, to carry its effects into the backing dataset.
	commit(ds *sparql.Dataset)
	// ensureGraph returns the named graph with this IRI, creating it.
	ensureGraph(iri string) *graph.Graph
	// dropGraph removes a named graph and its triples.
	dropGraph(iri string)
}

// mapBackend serves a *sparql.Dataset: named graphs are the entries of its map,
// each of which may live in its own store. A graph exists while it has an
// entry, even when it is empty.
type mapBackend struct {
	ds *sparql.Dataset
}

func (b *mapBackend) view() (*graph.Graph, map[string]*graph.Graph) {
	return b.ds.Default, b.ds.NamedGraphs
}

func (b *mapBackend) graph(iri string) (*graph.Graph, bool) {
	g, ok := b.ds.NamedGraphs[iri]
	return g, ok
}

func (b *mapBackend) updateDataset() *sparql.Dataset { return b.ds }

func (b *mapBackend) commit(*sparql.Dataset) {}

func (b *mapBackend) ensureGraph(iri string) *graph.Graph {
	if g, ok := b.ds.NamedGraphs[iri]; ok {
		return g
	}
	g := graph.NewGraph(graph.WithIdentifier(term.NewURIRefUnsafe(iri)))
	b.ds.NamedGraphs[iri] = g
	return g
}

func (b *mapBackend) dropGraph(iri string) {
	if g, ok := b.ds.NamedGraphs[iri]; ok {
		g.Remove(nil, nil, nil)
		delete(b.ds.NamedGraphs, iri)
	}
}

// storeBackend serves a *graph.Dataset: every graph is a context of one store,
// so a named graph exists exactly while the store holds a triple in it. Blank
// node contexts are not listed: no IRI can address them.
type storeBackend struct {
	ds *graph.Dataset
}

func (b *storeBackend) view() (*graph.Graph, map[string]*graph.Graph) {
	return b.ds.DefaultContext(), b.named()
}

func (b *storeBackend) named() map[string]*graph.Graph {
	named := make(map[string]*graph.Graph)
	for c := range b.ds.Contexts(nil) {
		u, ok := c.(term.URIRef)
		if !ok || store.IsDefaultGraph(u) {
			continue
		}
		named[u.Value()] = b.ds.GetContext(u)
	}
	return named
}

func (b *storeBackend) graph(iri string) (*graph.Graph, bool) {
	g := b.ds.GetContext(term.NewURIRefUnsafe(iri))
	// Some stores keep a context around after its last triple is removed;
	// existence is having a triple.
	for range g.Triples(nil, nil, nil) {
		return g, true
	}
	return nil, false
}

func (b *storeBackend) updateDataset() *sparql.Dataset {
	return &sparql.Dataset{Default: b.ds.DefaultContext(), NamedGraphs: b.named()}
}

// commit copies graphs the update created into the store. The engine creates
// a graph it has not seen as a fresh in-memory graph (INSERT DATA { GRAPH <new>
// { … } }, COPY, MOVE, LOAD INTO), so those triples are not yet in the store.
// Graphs it cleared or dropped were already emptied through the store.
func (b *storeBackend) commit(ds *sparql.Dataset) {
	st := b.ds.Store()
	for iri, g := range ds.NamedGraphs {
		if g.Store() == st {
			continue
		}
		target := b.ds.GetContext(term.NewURIRefUnsafe(iri))
		for t := range g.Triples(nil, nil, nil) {
			target.Add(t.Subject, t.Predicate, t.Object)
		}
	}
}

func (b *storeBackend) ensureGraph(iri string) *graph.Graph {
	return b.ds.GetContext(term.NewURIRefUnsafe(iri))
}

func (b *storeBackend) dropGraph(iri string) {
	b.ds.GetContext(term.NewURIRefUnsafe(iri)).Remove(nil, nil, nil)
}
