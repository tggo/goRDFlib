package sparql

import (
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/turtle"
)

// Issue #50: with no named graphs configured, GRAPH used to be answered from
// the default graph, which is not a named graph (SPARQL 1.1 §13.3).
func TestGraphWithoutNamedGraphsMatchesNothing(t *testing.T) {
	g := graph.NewGraph()
	if err := turtle.Parse(t.Context(), g, strings.NewReader(`@prefix ex: <http://example.org/> . ex:a a ex:Tool . ex:b a ex:Person .`)); err != nil {
		t.Fatal(err)
	}
	cases := map[string]int{
		`SELECT ?e WHERE { GRAPH <http://example.org/nosuchgraph> { ?e a ?t } }`: 0,
		`SELECT ?g ?e WHERE { GRAPH ?g { ?e a ?t } }`:                            0,
		`ASK { GRAPH ?g { ?s ?p ?o } }`:                                          0,
		// The default graph is still there outside GRAPH.
		`SELECT ?e WHERE { ?e a ?t OPTIONAL { GRAPH ?g { ?e ?p ?o } } }`: 2,
	}
	for q, want := range cases {
		r, err := Query(g, q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		got := len(r.Bindings)
		if r.Type == "ASK" {
			got = 0
			if r.AskResult {
				got = 1
			}
		}
		if got != want {
			t.Errorf("%s: %d rows, want %d: %v", q, got, want, r.Bindings)
		}
		for _, b := range r.Bindings {
			if _, ok := b["g"]; ok {
				t.Errorf("%s: ?g bound to %v", q, b["g"])
			}
		}
	}
}
