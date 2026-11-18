package sparql

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// existsGraphData has items in the default graph, each naming the one of
// graphs named graphs that holds its label, and marks every tenth graph as
// special. Every named graph also holds labels of other things, so matching
// a GRAPH pattern in full in a graph pulls far more triples than looking an
// item up.
func existsGraphData(t testing.TB, items, graphs int) (*graph.Graph, map[string]*graph.Graph, []*probingStore) {
	ctx := t.Context()
	ex := func(f string, a ...any) term.URIRef {
		return term.NewURIRefUnsafe("http://example.org/" + fmt.Sprintf(f, a...))
	}
	def := graph.NewGraph()
	named := make(map[string]*graph.Graph, graphs)
	stores := make([]*probingStore, graphs)
	for k := range graphs {
		stores[k] = &probingStore{Store: store.NewMemoryStore()}
		g := graph.NewGraph(graph.WithStore(stores[k]))
		for j := range 20 {
			g.Add(ctx, ex("other%d_%d", k, j), ex("label"), term.NewLiteral("other"))
		}
		named[ex("g%d", k).Value()] = g
		if k%10 == 0 {
			def.Add(ctx, ex("g%d", k), ex("kind"), ex("special"))
		}
	}
	for i := range items {
		k := i % graphs
		def.Add(ctx, ex("item%d", i), ex("storedIn"), ex("g%d", k))
		named[ex("g%d", k).Value()].Add(ctx, ex("item%d", i), ex("label"), term.NewLiteral(fmt.Sprintf("label %d", i)))
	}
	return def, named, stores
}

// A GRAPH pattern in an EXISTS body is evaluated with the outer row's values:
// in the one graph the row names, or else as a lookup in each graph. Matching
// it in full in every named graph for every row pulls about rows × graphs ×
// 21 triples.
func TestExistsGraphWork(t *testing.T) {
	def, named, stores := existsGraphData(t, 200, 100)
	for _, c := range []struct{ name, query string }{
		{"graph bound by the row", `SELECT ?item WHERE { ?item ex:storedIn ?g . FILTER EXISTS { GRAPH ?g { ?item ex:label ?label } } }`},
		{"graph unbound", `SELECT ?item WHERE { ?item ex:storedIn ?g . FILTER EXISTS { GRAPH ?h { ?item ex:label ?label } } }`},
		{"graph named", `SELECT ?item WHERE { ?item ex:storedIn ?g . FILTER EXISTS { GRAPH ex:g7 { ?item ex:label ?label } } }`},
	} {
		t.Run(c.name, func(t *testing.T) {
			for _, s := range stores {
				s.pulled = 0
			}
			pq, err := Parse("PREFIX ex: <http://example.org/>\n" + c.query)
			if err != nil {
				t.Fatal(err)
			}
			pq.NamedGraphs = named
			res, err := EvalQuery(def, pq, nil)
			if err != nil {
				t.Fatal(err)
			}
			want := 200
			if c.name == "graph named" {
				want = 2
			}
			if len(res.Bindings) != want {
				t.Fatalf("%d rows, want %d", len(res.Bindings), want)
			}
			pulled := 0
			for _, s := range stores {
				pulled += s.pulled
			}
			// one label per item, and the planner's probes
			if pulled > 1_000 {
				t.Errorf("pulled %d triples from the named graphs: the GRAPH pattern was matched in full per row", pulled)
			}
		})
	}
}
