package sparql

import (
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/term"
)

// Inside GRAPH ?g { P }, P is evaluated before ?g is joined in (SPARQL 1.1
// §18.6), so a FILTER or BIND in P sees ?g only when the row bound it
// outside. The pre-bound GRAPH path once substituted every graph's name into
// P and let its expressions see it.
func TestPreBoundGraphVariableScope(t *testing.T) {
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	ctx := t.Context()
	def := rdflibgo.NewGraph()
	def.Add(ctx, ex("root"), ex("p"), ex("a"))
	def.Add(ctx, ex("root"), ex("in"), ex("g1"))
	g1 := rdflibgo.NewGraph()
	g1.Add(ctx, ex("a"), ex("q"), ex("b"))
	g1.Add(ctx, ex("g1"), ex("q"), ex("b"))
	for _, c := range []struct {
		query string
		rows  int
	}{
		{`SELECT ?x WHERE { ?r ex:p ?x FILTER EXISTS { GRAPH ?g { ?x ex:q ?y FILTER (BOUND(?g)) } } }`, 0},
		{`SELECT ?x WHERE { ?r ex:p ?x FILTER EXISTS { GRAPH ?g { ?x ex:q ?y FILTER (!BOUND(?g)) } } }`, 1},
		{`SELECT ?x ?g WHERE { ?r ex:p ?x GRAPH ?g { ?x ex:q ?y BIND (?g AS ?h) FILTER (!BOUND(?h)) } }`, 1},
		// bound by the row outside: EXISTS substitutes it, so it is visible
		{`SELECT ?x WHERE { ?r ex:p ?x ; ex:in ?g FILTER EXISTS { GRAPH ?g { ?x ex:q ?y FILTER (BOUND(?g)) } } }`, 1},
		// the pattern itself mentions the graph variable
		{`SELECT ?g WHERE { ?r ex:p ?x FILTER EXISTS { GRAPH ?g { ?g ex:q ?y } } BIND (ex:g1 AS ?g) }`, 1},
	} {
		for _, off := range []bool{true, false} {
			offGraphPreBound = off
			pq, err := Parse("PREFIX ex: <http://example.org/>\n" + c.query)
			if err != nil {
				t.Fatal(err)
			}
			pq.NamedGraphs = map[string]*rdflibgo.Graph{"http://example.org/g1": g1}
			res, err := EvalQuery(def, pq, map[string]rdflibgo.Term{"r": ex("root")})
			offGraphPreBound = false
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Bindings) != c.rows {
				t.Errorf("offGraphPreBound=%v: %d rows, want %d\n%s", off, len(res.Bindings), c.rows, c.query)
			}
		}
	}
}
