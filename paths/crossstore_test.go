package paths_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/badgerstore"
	"github.com/tggo/goRDFlib/store/sqlitestore"
	"github.com/tggo/goRDFlib/term"
)

// The same closure queries over every kind of store must give the same
// pairs: MemoryStore answers through the Reachable pushdown, a wrapper that
// hides Reachable forces the local traversal, and Badger and SQLite have no
// Reachable and always traverse locally. A bug in one traversal (the inverse
// path through a literal reached unrelated nodes) shows as a difference here.
func TestClosureQueriesAgreeAcrossStores(t *testing.T) {
	newStores := func(t *testing.T) map[string]store.Store {
		bs, err := badgerstore.New(badgerstore.WithInMemory())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { bs.Close() })
		ss, err := sqlitestore.New(sqlitestore.WithFile(filepath.Join(t.TempDir(), "p.db")))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ss.Close() })
		return map[string]store.Store{
			"memory":       store.NewMemoryStore(),
			"memory-local": struct{ store.Store }{store.NewMemoryStore()},
			"badger":       bs,
			"sqlite":       ss,
		}
	}
	p, q := uri("p"), uri("q")
	shapes := map[string]paths.Path{
		"p+":       paths.OneOrMore(paths.AsPath(p)),
		"p*":       paths.ZeroOrMore(paths.AsPath(p)),
		"^p+":      paths.OneOrMore(paths.Inv(paths.AsPath(p))),
		"^p*":      paths.ZeroOrMore(paths.Inv(paths.AsPath(p))),
		"(p|q)+":   paths.OneOrMore(paths.Alternative(paths.AsPath(p), paths.AsPath(q))),
		"(^p|^q)*": paths.ZeroOrMore(paths.Alternative(paths.Inv(paths.AsPath(p)), paths.Inv(paths.AsPath(q)))),
		"!(p)+":    paths.OneOrMore(paths.Negated(p)),
		"(p/q)+":   paths.OneOrMore(paths.Sequence(paths.AsPath(p), paths.AsPath(q))),
		"p/q*":     paths.Sequence(paths.AsPath(p), paths.ZeroOrMore(paths.AsPath(q))),
	}
	keys := func(t *testing.T, path paths.Path, g *graph.Graph, s term.Subject, o term.Term) []string {
		t.Helper()
		var out []string
		for pair, err := range path.Eval(context.Background(), g, s, o) {
			if err != nil {
				t.Fatal(err)
			}
			out = append(out, term.TermKey(pair.First)+"|"+term.TermKey(pair.Second))
		}
		slices.Sort(out)
		return out
	}
	for seed := range uint64(4) {
		r := rand.New(rand.NewPCG(seed, 7))
		type edge struct {
			s, p, o term.Term
		}
		var edges []edge
		node := func(i int) term.URIRef { return uri(fmt.Sprintf("n%d", i)) }
		for range 70 {
			var o term.Term = node(r.IntN(18))
			if r.IntN(7) == 0 {
				o = term.NewLiteral(r.IntN(4)) // terminal nodes
			}
			edges = append(edges, edge{node(r.IntN(18)), []term.URIRef{p, q, uri("r")}[r.IntN(3)], o})
		}
		graphs := map[string]*graph.Graph{}
		for name, st := range newStores(t) {
			g := graph.NewGraph(graph.WithStore(st))
			for _, e := range edges {
				if err := g.Add(t.Context(), e.s.(term.Subject), e.p.(term.URIRef), e.o); err != nil {
					t.Fatal(err)
				}
			}
			graphs[name] = g
		}
		for name, path := range shapes {
			for start := range 5 {
				s := node(start)
				for _, forward := range []bool{true, false} {
					var sub term.Subject
					var obj term.Term
					if forward {
						sub = s
					} else {
						obj = s
					}
					want := keys(t, path, graphs["memory-local"], sub, obj)
					for sname, g := range graphs {
						if got := keys(t, path, g, sub, obj); !slices.Equal(got, want) {
							t.Errorf("seed %d %s from %s forward=%v: %s gave %d pairs, memory-local %d", seed, name, s, forward, sname, len(got), len(want))
						}
					}
				}
			}
			// no endpoint bound
			want := keys(t, path, graphs["memory-local"], nil, nil)
			for sname, g := range graphs {
				if got := keys(t, path, g, nil, nil); !slices.Equal(got, want) {
					t.Errorf("seed %d %s unbound: %s gave %d pairs, memory-local %d", seed, name, sname, len(got), len(want))
				}
			}
		}
	}
}
