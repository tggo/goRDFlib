package sparql

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// pathProbeGraph links a root to 1,500 nodes by urn:part; every 75th node is
// typed urn:Class1 (20 nodes), and the first three nodes also hang from
// urn:small.
func pathProbeGraph(t *testing.T, opts ...graph.GraphOption) *graph.Graph {
	t.Helper()
	ctx := t.Context()
	g := graph.NewGraph(opts...)
	u := func(s string) term.URIRef { return term.NewURIRefUnsafe("urn:" + s) }
	for i := range 1500 {
		p := u(fmt.Sprintf("p%d", i))
		g.Add(ctx, u("root"), u("part"), p)
		if i%75 == 0 {
			g.Add(ctx, p, u("type"), u("Class1"))
		}
		if i < 3 {
			g.Add(ctx, u("small"), u("part"), p)
		}
	}
	return g
}

// A path with a known endpoint and few pairs is probed and runs first: the 20
// instances of a class before the 1,500 nodes below the root, whichever order
// the query is written in and whether the root is a constant or pre-bound.
func TestOrderBGP_ProbesPathsWithAKnownEndpoint(t *testing.T) {
	g := pathProbeGraph(t)
	root := term.NewURIRefUnsafe("urn:root")
	for _, tc := range []struct {
		name, query string
		pre         map[string]term.Term
	}{
		{"pre-bound, written last", `SELECT ?x WHERE { ?this <urn:part>+ ?x . ?x <urn:type>/<urn:sub>* <urn:Class1> }`, map[string]term.Term{"this": root}},
		{"constant, written last", `SELECT ?x WHERE { <urn:root> <urn:part>+ ?x . ?x <urn:type>/<urn:sub>* <urn:Class1> }`, nil},
		{"written first", `SELECT ?x WHERE { ?x <urn:type>/<urn:sub>* <urn:Class1> . <urn:root> <urn:part>+ ?x }`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pq, err := Parse(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			bgp := pq.Where.(*BGP)
			ordered := orderBGP(nil, g, bgp.Triples, tc.pre, pq.Prefixes)
			if ordered[0].Object != "<urn:Class1>" {
				t.Errorf("first pattern %s %s, want the type path", ordered[0].Subject, ordered[0].Object)
			}
			res, err := EvalQuery(g, pq, tc.pre)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Bindings) != 20 {
				t.Errorf("got %d rows, want 20", len(res.Bindings))
			}
		})
	}
}

// A path is probed only up to the smallest exact count of the other
// patterns: the 3 nodes below urn:small keep running first, and a path with
// no pair empties the BGP before anything else runs.
func TestOrderBGP_PathProbeLimit(t *testing.T) {
	g := pathProbeGraph(t)
	for _, tc := range []struct {
		name, query string
		firstObj    string
		rows        int
	}{
		{"smaller pattern first", `SELECT ?x WHERE { ?x <urn:type>/<urn:sub>* <urn:Class1> . <urn:small> <urn:part> ?x }`, "?x", 1},
		{"empty path first", `SELECT ?x WHERE { <urn:root> <urn:part> ?x . ?x <urn:type>/<urn:sub>* <urn:Missing> }`, "<urn:Missing>", 0},
		{"large path", `SELECT ?x WHERE { <urn:root> <urn:part>+ ?x . <urn:small> <urn:part> ?x }`, "?x", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pq, err := Parse(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			bgp := pq.Where.(*BGP)
			ordered := orderBGP(nil, g, bgp.Triples, nil, pq.Prefixes)
			if ordered[0].Object != tc.firstObj {
				t.Errorf("first pattern %s %s, want object %s", ordered[0].Subject, ordered[0].Object, tc.firstObj)
			}
			res, err := EvalQuery(g, pq, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Bindings) != tc.rows {
				t.Errorf("got %d rows, want %d", len(res.Bindings), tc.rows)
			}
		})
	}
}

// The probe of a large path stops at the smallest exact count: planning
// `<root> part+ ?x . <small> part ?x` pulls a handful of triples, not the
// 1,500 below the root.
func TestOrderBGP_PathProbeStopsAtSmallestCount(t *testing.T) {
	ps := &probingStore{Store: store.NewMemoryStore()}
	g := pathProbeGraph(t, graph.WithStore(ps))
	pq, err := Parse(`SELECT ?x WHERE { <urn:root> <urn:part>+ ?x . <urn:small> <urn:part> ?x }`)
	if err != nil {
		t.Fatal(err)
	}
	ps.pulled = 0
	ordered := orderBGP(nil, g, pq.Where.(*BGP).Triples, nil, pq.Prefixes)
	if ordered[0].Subject != "<urn:small>" {
		t.Errorf("first pattern %s %s, want the urn:small pattern", ordered[0].Subject, ordered[0].Object)
	}
	if ps.pulled > 100 {
		t.Errorf("planning pulled %d triples, want at most 100", ps.pulled)
	}
}
