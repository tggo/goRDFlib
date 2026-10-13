package endpoint

import (
	"context"
	"fmt"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// backend adapts a dataset to what the handler needs. Every method is called
// with the handler's lock held: view and graph under at least the read lock,
// the others under the write lock. A store failure is the method's error;
// the handler answers it with a 5xx, never with an empty result.
type backend interface {
	// view returns the dataset as the query engine sees it: the default graph
	// and the named graphs by IRI. The map must not be modified.
	view(ctx context.Context) (def *graph.Graph, named map[string]*graph.Graph, err error)
	// graph returns the named graph with this IRI, if it exists.
	graph(ctx context.Context, iri string) (*graph.Graph, bool, error)
	// updateDataset returns the sparql.Dataset an update runs against.
	updateDataset(ctx context.Context) (*sparql.Dataset, error)
	// commit is called after an update ran against ds, whether it failed or
	// not, to carry its effects into the backing dataset.
	commit(ctx context.Context, ds *sparql.Dataset) error
	// ensureGraph returns the named graph with this IRI, creating it.
	ensureGraph(iri string) *graph.Graph
	// dropGraph removes a named graph and its triples.
	dropGraph(ctx context.Context, iri string) error
}

// mapBackend serves a *sparql.Dataset: named graphs are the entries of its map,
// each of which may live in its own store. A graph exists while it has an
// entry, even when it is empty.
type mapBackend struct {
	ds *sparql.Dataset
}

func (b *mapBackend) view(context.Context) (*graph.Graph, map[string]*graph.Graph, error) {
	return b.ds.Default, b.ds.NamedGraphs, nil
}

func (b *mapBackend) graph(_ context.Context, iri string) (*graph.Graph, bool, error) {
	g, ok := b.ds.NamedGraphs[iri]
	return g, ok, nil
}

func (b *mapBackend) updateDataset(context.Context) (*sparql.Dataset, error) { return b.ds, nil }

func (b *mapBackend) commit(context.Context, *sparql.Dataset) error { return nil }

func (b *mapBackend) ensureGraph(iri string) *graph.Graph {
	if g, ok := b.ds.NamedGraphs[iri]; ok {
		return g
	}
	g := graph.NewGraph(graph.WithIdentifier(term.NewURIRefUnsafe(iri)))
	b.ds.NamedGraphs[iri] = g
	return g
}

func (b *mapBackend) dropGraph(ctx context.Context, iri string) error {
	if g, ok := b.ds.NamedGraphs[iri]; ok {
		// The entry stays when the store could not empty the graph: dropping
		// it would hide triples that are still there.
		if err := g.Remove(ctx, nil, nil, nil); err != nil {
			return fmt.Errorf("dropping graph <%s>: %w", iri, err)
		}
		delete(b.ds.NamedGraphs, iri)
	}
	return nil
}

// storeBackend serves a *graph.Dataset: every graph is a context of one store,
// so a named graph exists exactly while the store holds a triple in it. Blank
// node contexts are not listed: no IRI can address them.
type storeBackend struct {
	ds *graph.Dataset
}

func (b *storeBackend) view(ctx context.Context) (*graph.Graph, map[string]*graph.Graph, error) {
	named, err := b.named(ctx)
	if err != nil {
		return nil, nil, err
	}
	return b.ds.DefaultContext(), named, nil
}

func (b *storeBackend) named(ctx context.Context) (map[string]*graph.Graph, error) {
	named := make(map[string]*graph.Graph)
	for c, err := range b.ds.Contexts(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("listing named graphs: %w", err)
		}
		u, ok := c.(term.URIRef)
		if !ok || store.IsDefaultGraph(u) {
			continue
		}
		named[u.Value()] = b.ds.GetContext(u)
	}
	return named, nil
}

func (b *storeBackend) graph(ctx context.Context, iri string) (*graph.Graph, bool, error) {
	g := b.ds.GetContext(term.NewURIRefUnsafe(iri))
	// Some stores keep a context around after its last triple is removed;
	// existence is having a triple.
	for _, err := range g.Triples(ctx, nil, nil, nil) {
		if err != nil {
			return nil, false, fmt.Errorf("looking up graph <%s>: %w", iri, err)
		}
		return g, true, nil
	}
	return nil, false, nil
}

func (b *storeBackend) updateDataset(ctx context.Context) (*sparql.Dataset, error) {
	named, err := b.named(ctx)
	if err != nil {
		return nil, err
	}
	return &sparql.Dataset{Default: b.ds.DefaultContext(), NamedGraphs: named}, nil
}

// commit copies graphs the update created into the store. The engine creates
// a graph it has not seen as a fresh in-memory graph (INSERT DATA { GRAPH <new>
// { … } }, COPY, MOVE, LOAD INTO), so those triples are not yet in the store.
// Graphs it cleared or dropped were already emptied through the store.
func (b *storeBackend) commit(ctx context.Context, ds *sparql.Dataset) error {
	st := b.ds.Store()
	for iri, g := range ds.NamedGraphs {
		if g.Store() == st {
			continue
		}
		target := b.ds.GetContext(term.NewURIRefUnsafe(iri))
		if err := copyTriples(ctx, target, g); err != nil {
			return fmt.Errorf("saving graph <%s>: %w", iri, err)
		}
	}
	return nil
}

func (b *storeBackend) ensureGraph(iri string) *graph.Graph {
	return b.ds.GetContext(term.NewURIRefUnsafe(iri))
}

func (b *storeBackend) dropGraph(ctx context.Context, iri string) error {
	if err := b.ds.GetContext(term.NewURIRefUnsafe(iri)).Remove(ctx, nil, nil, nil); err != nil {
		return fmt.Errorf("dropping graph <%s>: %w", iri, err)
	}
	return nil
}
