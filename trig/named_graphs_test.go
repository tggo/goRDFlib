package trig

import (
	"bytes"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store/badgerstore"
	"github.com/tggo/goRDFlib/store/sqlitestore"
)

// TestDatasetKeepsGraphsApart is rdflib #2393/#1842: on the default in-memory
// store a named graph used to see every triple in the dataset, because
// MemoryStore ignored the context it was given.
func TestDatasetKeepsGraphsApart(t *testing.T) {
	ds := graph.NewDataset()
	src := `@prefix : <http://e/> . :a :b :c . :g { :x :y :z . }`
	if err := ParseDataset(ds, strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}
	g := rdflibgo.NewURIRefUnsafe("http://e/g")
	if n := ds.Graph(g).Len(); n != 1 {
		t.Errorf("named graph :g has %d triples, want 1", n)
	}
	if n := ds.DefaultContext().Len(); n != 1 {
		t.Errorf("default graph has %d triples, want 1", n)
	}
	if n := ds.Len(); n != 2 {
		t.Errorf("dataset union has %d triples, want 2", n)
	}

	var b bytes.Buffer
	if err := SerializeDataset(ds, &b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if strings.Count(out, "<http://e/a>")+strings.Count(out, ":a ") != 1 {
		t.Errorf("default-graph triple written more than once:\n%s", out)
	}
}

// TestBlankNodeGraphNameStaysNamed is rdflib #2445: every store folded a
// blank-node context into the default graph, so `_:g { … }` lost its graph.
func TestBlankNodeGraphNameStaysNamed(t *testing.T) {
	backends := map[string]func(t *testing.T) *graph.Dataset{
		"memory": func(*testing.T) *graph.Dataset { return graph.NewDataset() },
		"badger": func(t *testing.T) *graph.Dataset {
			s, err := badgerstore.New(badgerstore.WithInMemory())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { s.Close() })
			return graph.NewDataset(graph.WithStore(s))
		},
		"sqlite": func(t *testing.T) *graph.Dataset {
			s, err := sqlitestore.New(sqlitestore.WithInMemory())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { s.Close() })
			return graph.NewDataset(graph.WithStore(s))
		},
	}
	for name, newDS := range backends {
		t.Run(name, func(t *testing.T) {
			ds := newDS(t)
			src := `@prefix : <http://e/> . :d :d :d . _:g { :a :b :c . } [] { :x :y :z . }`
			if err := ParseDataset(ds, strings.NewReader(src)); err != nil {
				t.Fatal(err)
			}
			if n := ds.DefaultContext().Len(); n != 1 {
				t.Errorf("default graph has %d triples, want 1", n)
			}
			var named []rdflibgo.Term
			ds.Contexts(nil)(func(c rdflibgo.Term) bool {
				named = append(named, c)
				return true
			})
			if len(named) != 2 {
				t.Fatalf("Contexts reported %d graphs, want the 2 blank-node graphs: %v", len(named), named)
			}
			for _, c := range named {
				if _, ok := c.(rdflibgo.BNode); !ok {
					t.Errorf("graph name %v is not a blank node", c)
				}
				if n := ds.Graph(c).Len(); n != 1 {
					t.Errorf("graph %v has %d triples, want 1", c, n)
				}
			}
		})
	}
}
