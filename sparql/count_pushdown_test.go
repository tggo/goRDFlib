package sparql

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// The COUNT pushdown asks the store to count the bare pattern. It ignored
// initial bindings (counting the whole graph) and counted every row for
// COUNT(?z) of a variable the pattern does not bind, which is 0.
func TestCountPushdownRespectsBindingsAndUnboundVars(t *testing.T) {
	g := graph.NewGraph()
	a := term.NewURIRefUnsafe("urn:a")
	p := term.NewURIRefUnsafe("urn:p")
	_ = g.Add(t.Context(), a, p, a)
	_ = g.Add(t.Context(), term.NewURIRefUnsafe("urn:b"), p, a)
	for q, want := range map[string]string{
		`SELECT (COUNT(*) AS ?n) WHERE { ?x <urn:p> ?v }`:  "1",
		`SELECT (COUNT(?z) AS ?n) WHERE { ?x <urn:p> ?v }`: "0",
	} {
		pq, _ := Parse(q)
		pre := map[string]term.Term{"x": a}
		if want == "0" {
			pre = nil
		}
		res, _ := EvalQuery(g, pq, pre)
		if got := res.Bindings[0]["n"]; got == nil || got.(term.Literal).Lexical() != want {
			t.Errorf("%s: n=%v want %s", q, got, want)
		}
	}
}
