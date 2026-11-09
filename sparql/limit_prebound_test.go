package sparql

import (
	"context"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
)

// A LIMIT over a single triple pattern is answered from the store, which knows
// nothing of the initial bindings. Under them it returned the first triple of
// the predicate for any subject, replacing the pre-bound rows: SHACL's
// `SELECT $this ?v WHERE { $this p ?v } LIMIT 1` sub-SELECT (pre-binding
// substituted into subqueries) found nothing for a focus node that was not the
// first subject of p.
func TestLimitOverSinglePatternHonoursInitialBindings(t *testing.T) {
	ex := func(s string) rdflibgo.URIRef { return rdflibgo.NewURIRefUnsafe("http://example.org/" + s) }
	g := graph.NewGraph()
	ctx := context.Background()
	for _, s := range []string{"a", "b", "c"} {
		g.Add(ctx, ex(s), ex("p"), ex("v"+s))
	}
	pre := map[string]rdflibgo.Term{"s": ex("c")}

	for _, q := range []string{
		`SELECT ?s ?o WHERE { ?s <http://example.org/p> ?o } LIMIT 1`,
		`SELECT ?s ?o WHERE { ?s <http://example.org/p> ?o } LIMIT 1 OFFSET 0`,
	} {
		pq, err := Parse(q)
		if err != nil {
			t.Fatal(err)
		}
		res, err := EvalQueryContext(ctx, g, pq, pre)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Bindings) != 1 || res.Bindings[0]["o"] != ex("vc") {
			t.Fatalf("%s\nwith ?s pre-bound to c: got %v, want ?o = vc", q, res.Bindings)
		}
	}
}
