package paths_test

import (
	"context"
	"slices"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// A backward walk of (^p)+ to a node reaches the nodes below it, a literal
// child included. The step from the literal handed the inverse path a
// literal as its subject, which became "no subject" and so every p edge of
// the graph: nodes unrelated to the start were reached. (The store
// reachability pushdown was right; only the local traversal was wrong, and
// it is the only traversal a store without Reachable has.)
func TestInverseClosureBackwardThroughALiteral(t *testing.T) {
	p := uri("p")
	// with Reachable (the pushdown answers) and without (local traversal)
	stores := map[string]store.Store{
		"memory (pushdown)":    store.NewMemoryStore(),
		"no Reachable (local)": struct{ store.Store }{store.NewMemoryStore()},
	}
	path := paths.OneOrMore(paths.Inv(paths.AsPath(p)))
	want := []string{
		term.TermKey(uri("b")) + "|" + term.TermKey(uri("a")),
		term.TermKey(term.NewLiteral("leaf")) + "|" + term.TermKey(uri("a")),
	}
	slices.Sort(want)
	for name, st := range stores {
		g := graph.NewGraph(graph.WithStore(st))
		g.Add(t.Context(), uri("a"), p, uri("b"))
		g.Add(t.Context(), uri("b"), p, term.NewLiteral("leaf"))
		g.Add(t.Context(), uri("x"), p, uri("y"))
		g.Add(t.Context(), uri("y"), p, term.NewLiteral("other"))
		if got := invPairKeys(t, path, g, uri("a")); !slices.Equal(got, want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

// invPairKeys returns the sorted keys of the pairs of p ending at o.
func invPairKeys(t *testing.T, p paths.Path, g *graph.Graph, o term.Term) []string {
	t.Helper()
	var out []string
	for pair, err := range p.Eval(context.Background(), g, nil, o) {
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, term.TermKey(pair.First)+"|"+term.TermKey(pair.Second))
	}
	slices.Sort(out)
	return out
}
