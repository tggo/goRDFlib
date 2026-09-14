package graph_test

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
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

	ds.DefaultContext().Add(s, p, term.NewLiteral("default"))
	ds.Graph(g1).Add(s, p, term.NewLiteral("one"))
	ds.Graph(g1).Add(s, p, term.NewLiteral("shared"))
	ds.Graph(g2).Add(s, p, term.NewLiteral("shared"))

	for _, tc := range []struct {
		name string
		g    *graph.Graph
		want int
	}{
		{"default", ds.DefaultContext(), 1},
		{"g1", ds.Graph(g1), 2},
		{"g2", ds.Graph(g2), 1},
	} {
		if got := tc.g.Len(); got != tc.want {
			t.Errorf("%s: Len = %d, want %d", tc.name, got, tc.want)
		}
	}
	if !ds.Graph(g2).Contains(s, p, term.NewLiteral("shared")) || ds.Graph(g2).Contains(s, p, term.NewLiteral("one")) {
		t.Error("g2 sees a triple that belongs to g1 only")
	}

	if got := ds.Len(); got != 3 {
		t.Errorf("union Len = %d, want 3 (shared triple counted once)", got)
	}
	quads := 0
	ds.Quads(nil, nil, nil)(func(term.Quad) bool { quads++; return true })
	if quads != 4 {
		t.Errorf("Quads yielded %d, want 4", quads)
	}
	ctxs := 0
	ds.Contexts(nil)(func(term.Term) bool { ctxs++; return true })
	if ctxs != 2 {
		t.Errorf("Contexts reported %d graphs, want 2", ctxs)
	}

	shared := term.NewLiteral("shared")
	ds.Remove(nil, nil, shared, nil)
	if ds.Graph(g1).Len() != 1 || ds.Graph(g2).Len() != 0 {
		t.Errorf("Remove with a nil context must remove from every graph: g1=%d g2=%d",
			ds.Graph(g1).Len(), ds.Graph(g2).Len())
	}
	ds.Remove(nil, nil, nil, g1)
	if ds.DefaultContext().Len() != 1 {
		t.Error("Remove in g1 touched the default graph")
	}
}
