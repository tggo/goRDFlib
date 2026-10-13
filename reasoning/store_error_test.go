package reasoning

import (
	"errors"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/storetest"
	"github.com/tggo/goRDFlib/term"
)

// failingGraph is a small graph where both regimes have work to do, on a store
// that can be told to fail.
func failingGraph(t *testing.T) (*graph.Graph, *storetest.FailingStore) {
	t.Helper()
	fs := storetest.NewFailingStore(store.NewMemoryStore())
	g := graph.NewGraph(graph.WithStore(fs))
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	mustV(0, g.Add(t.Context(), ex("Dog"), namespace.RDFS.SubClassOf, ex("Animal")))
	mustV(0, g.Add(t.Context(), ex("rex"), namespace.RDF.Type, ex("Dog")))
	mustV(0, g.Add(t.Context(), ex("owns"), namespace.OWL.InverseOf, ex("ownedBy")))
	mustV(0, g.Add(t.Context(), ex("ann"), ex("owns"), ex("rex")))
	return g, fs
}

func TestStoreErrorReachesCaller(t *testing.T) {
	run := map[string]func(*graph.Graph) error{
		"Expand":             func(g *graph.Graph) error { _, err := Expand(g, RDFS|OWLRL); return err },
		"ExpandCheck":        func(g *graph.Graph) error { _, _, err := ExpandCheck(g, RDFS|OWLRL); return err },
		"ExpandContext":      func(g *graph.Graph) error { _, err := ExpandContext(t.Context(), g, RDFS|OWLRL); return err },
		"ExpandCheckContext": func(g *graph.Graph) error { _, _, err := ExpandCheckContext(t.Context(), g, OWLRL); return err },
		"RDFSClosure":        func(g *graph.Graph) error { _, err := RDFSClosure(t.Context(), g); return err },
		"OWLRLClosure":       func(g *graph.Graph) error { _, err := OWLRLClosure(t.Context(), g); return err },
		"OWLRLClosureCheck":  func(g *graph.Graph) error { _, _, err := OWLRLClosureCheck(t.Context(), g); return err },
	}
	for name, f := range run {
		t.Run(name+"/reads", func(t *testing.T) {
			g, fs := failingGraph(t)
			fs.Fail(nil)
			if err := f(g); !errors.Is(err, storetest.ErrInjected) {
				t.Fatalf("err = %v, want the store's error", err)
			}
		})
		t.Run(name+"/after some reads", func(t *testing.T) {
			g, fs := failingGraph(t)
			fs.FailReadsAfter(3, nil)
			if err := f(g); !errors.Is(err, storetest.ErrInjected) {
				t.Fatalf("err = %v, want the store's error", err)
			}
		})
	}
}
