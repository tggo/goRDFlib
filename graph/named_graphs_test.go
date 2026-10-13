package graph_test

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// TestDatasetNamedGraphsOnMemoryStore pins that a named graph on the default
// store holds only its own triples, while the ConjunctiveGraph view is the
// union of all of them with each triple counted once.
func TestDatasetNamedGraphsOnMemoryStore(t *testing.T) {
	ds := graph.NewDataset()
	s := term.NewURIRefUnsafe("http://e/s")
	p := term.NewURIRefUnsafe("http://e/p")
	g1 := term.NewURIRefUnsafe("http://e/g1")
	g2 := term.NewURIRefUnsafe("http://e/g2")

	ds.DefaultContext().Add(t.Context(), s, p, term.NewLiteral("default"))
	ds.Graph(g1).Add(t.Context(), s, p, term.NewLiteral("one"))
	ds.Graph(g1).Add(t.Context(), s, p, term.NewLiteral("shared"))
	ds.Graph(g2).Add(t.Context(), s, p, term.NewLiteral("shared"))

	for _, tc := range []struct {
		name string
		g    *graph.Graph
		want int
	}{
		{"default", ds.DefaultContext(), 1},
		{"g1", ds.Graph(g1), 2},
		{"g2", ds.Graph(g2), 1},
	} {
		if got, _ := tc.g.Len(t.Context()); got != tc.want {
			t.Errorf("%s: Len = %d, want %d", tc.name, got, tc.want)
		}
	}
	if !mustV(ds.Graph(g2).Contains(t.Context(), s, p, term.NewLiteral("shared"))) || mustV(ds.Graph(g2).Contains(t.Context(), s, p, term.NewLiteral("one"))) {
		t.Error("g2 sees a triple that belongs to g1 only")
	}

	if got, _ := ds.Len(t.Context()); got != 3 {
		t.Errorf("union Len = %d, want 3 (shared triple counted once)", got)
	}
	quads := 0
	ds.Quads(t.Context(), nil, nil, nil)(func(term.Quad, error) bool { quads++; return true })
	if quads != 4 {
		t.Errorf("Quads yielded %d, want 4", quads)
	}
	ctxs := 0
	ds.Contexts(t.Context(), nil)(func(term.Term, error) bool { ctxs++; return true })
	if ctxs != 2 {
		t.Errorf("Contexts reported %d graphs, want 2", ctxs)
	}

	shared := term.NewLiteral("shared")
	ds.Remove(t.Context(), nil, nil, shared, nil)
	if mustV(ds.Graph(g1).Len(t.Context())) != 1 || mustV(ds.Graph(g2).Len(t.Context())) != 0 {
		t.Errorf("Remove with a nil context must remove from every graph: g1=%d g2=%d",
			mustV(ds.Graph(g1).Len(t.Context())), mustV(ds.Graph(g2).Len(t.Context())))
	}
	ds.Remove(t.Context(), nil, nil, nil, g1)
	if mustV(ds.DefaultContext().Len(t.Context())) != 1 {
		t.Error("Remove in g1 touched the default graph")
	}
}

// TestUnnamedGraphUsesDefaultGraphIdentifier pins the identifier contract that
// replaced "a blank node is the default graph": an unnamed graph carries
// store.DefaultGraph, and a graph named by a blank node is a named graph even on
// a shared store (rdflib #2445).
func TestUnnamedGraphUsesDefaultGraphIdentifier(t *testing.T) {
	if id := graph.NewGraph().Identifier(); !store.IsDefaultGraph(id) {
		t.Errorf("NewGraph().Identifier() = %v, want store.DefaultGraph", id)
	}
	if id := graph.NewDataset().DefaultContext().Identifier(); !store.IsDefaultGraph(id) {
		t.Errorf("Dataset default context identifier = %v, want store.DefaultGraph", id)
	}

	s := store.NewMemoryStore()
	p := term.NewURIRefUnsafe("http://e/p")
	unnamed := graph.NewGraph(graph.WithStore(s))
	bn := term.NewBNode()
	blank := graph.NewGraph(graph.WithStore(s), graph.WithIdentifier(bn))

	unnamed.Add(t.Context(), term.NewURIRefUnsafe("http://e/a"), p, term.NewLiteral("default"))
	blank.Add(t.Context(), term.NewURIRefUnsafe("http://e/b"), p, term.NewLiteral("named"))

	if mustV(unnamed.Len(t.Context())) != 1 || mustV(blank.Len(t.Context())) != 1 {
		t.Errorf("unnamed=%d blank=%d, want 1 each", mustV(unnamed.Len(t.Context())), mustV(blank.Len(t.Context())))
	}
	if mustV(s.Len(t.Context(), nil)) != 1 {
		t.Errorf("default graph holds %d triples, want 1", mustV(s.Len(t.Context(), nil)))
	}
	found := false
	s.Contexts(t.Context(), nil)(func(c term.Term, _ error) bool {
		found = found || c.Equal(bn)
		return true
	})
	if !found {
		t.Error("the blank-node graph is not reported by Contexts")
	}
}
