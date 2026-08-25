package paths_test

import (
	"errors"
	"sort"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// decliningStore wraps a MemoryStore but refuses every reachability query, so a
// test can run the same evaluation down the client-side fallback path. Any
// difference between it and the plain MemoryStore is a pushdown bug.
type decliningStore struct {
	*store.MemoryStore
	calls int
}

func (d *decliningStore) Reachable(store.ReachabilityQuery) ([]term.Term, error) {
	d.calls++
	return nil, store.ErrReachabilityUnsupported
}

// countingStore records how many reachability queries actually reached the
// backend, so a test can assert that pushdown happened at all rather than
// silently passing on the fallback.
type countingStore struct {
	*store.MemoryStore
	calls int
}

func (c *countingStore) Reachable(q store.ReachabilityQuery) ([]term.Term, error) {
	c.calls++
	return c.MemoryStore.Reachable(q)
}

func uri(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }

// reachGraph builds a graph with a cycle, a branch, a second predicate and a
// literal endpoint — the four things that separate a correct traversal from one
// that merely works on a straight chain.
//
//	a --p--> b --p--> c --p--> a   (cycle)
//	b --p--> d
//	d --q--> e
//	c --p--> "leaf"
func reachGraph(s store.Store) *graph.Graph {
	g := graph.NewGraph(graph.WithStore(s))
	g.Add(uri("a"), uri("p"), uri("b"))
	g.Add(uri("b"), uri("p"), uri("c"))
	g.Add(uri("c"), uri("p"), uri("a"))
	g.Add(uri("b"), uri("p"), uri("d"))
	g.Add(uri("d"), uri("q"), uri("e"))
	g.Add(uri("c"), uri("p"), term.NewLiteral("leaf"))
	return g
}

func pairs(g *graph.Graph, p paths.Path, subj term.Subject, obj term.Term) []string {
	var out []string
	p.Eval(g, subj, obj)(func(s, o term.Term) bool {
		out = append(out, s.N3()+" -> "+o.N3())
		return true
	})
	sort.Strings(out)
	return out
}

// assertSameAsFallback runs the identical query against a pushdown-capable store
// and a declining one and requires byte-identical result sets.
func assertSameAsFallback(t *testing.T, build func(*graph.Graph) (paths.Path, term.Subject, term.Term)) {
	t.Helper()

	pushed := &countingStore{MemoryStore: store.NewMemoryStore()}
	gPush := reachGraph(pushed)
	pPush, sPush, oPush := build(gPush)
	got := pairs(gPush, pPush, sPush, oPush)

	fell := &decliningStore{MemoryStore: store.NewMemoryStore()}
	gFall := reachGraph(fell)
	pFall, sFall, oFall := build(gFall)
	want := pairs(gFall, pFall, sFall, oFall)

	if pushed.calls == 0 {
		t.Fatalf("expected the query to be pushed down, but Reachable was never called")
	}
	if fell.calls == 0 {
		t.Fatalf("declining store was never asked; the two runs took different code paths")
	}
	if len(got) != len(want) {
		t.Fatalf("pushdown returned %d pairs, fallback returned %d\n pushdown: %v\n fallback: %v",
			len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("pushdown != fallback at %d:\n pushdown: %v\n fallback: %v", i, got, want)
		}
	}
}

func TestPushdownMatchesFallback(t *testing.T) {
	p, q := uri("p"), uri("q")

	cases := []struct {
		name  string
		build func(*graph.Graph) (paths.Path, term.Subject, term.Term)
	}{
		{"OneOrMore forward", func(*graph.Graph) (paths.Path, term.Subject, term.Term) {
			return paths.OneOrMore(paths.AsPath(p)), uri("a"), nil
		}},
		{"ZeroOrMore forward", func(*graph.Graph) (paths.Path, term.Subject, term.Term) {
			return paths.ZeroOrMore(paths.AsPath(p)), uri("a"), nil
		}},
		{"OneOrMore forward with bound object", func(*graph.Graph) (paths.Path, term.Subject, term.Term) {
			return paths.OneOrMore(paths.AsPath(p)), uri("a"), uri("c")
		}},
		{"OneOrMore backward", func(*graph.Graph) (paths.Path, term.Subject, term.Term) {
			return paths.OneOrMore(paths.AsPath(p)), nil, uri("c")
		}},
		{"ZeroOrMore backward", func(*graph.Graph) (paths.Path, term.Subject, term.Term) {
			return paths.ZeroOrMore(paths.AsPath(p)), nil, uri("c")
		}},
		{"backward to a literal", func(*graph.Graph) (paths.Path, term.Subject, term.Term) {
			return paths.OneOrMore(paths.AsPath(p)), nil, term.NewLiteral("leaf")
		}},
		{"inverse path", func(*graph.Graph) (paths.Path, term.Subject, term.Term) {
			return paths.OneOrMore(paths.Inv(paths.AsPath(p))), uri("c"), nil
		}},
		{"inverse path backward", func(*graph.Graph) (paths.Path, term.Subject, term.Term) {
			return paths.OneOrMore(paths.Inv(paths.AsPath(p))), nil, uri("a")
		}},
		{"alternation", func(*graph.Graph) (paths.Path, term.Subject, term.Term) {
			return paths.OneOrMore(paths.Alternative(paths.AsPath(p), paths.AsPath(q))), uri("a"), nil
		}},
		{"negated property set", func(*graph.Graph) (paths.Path, term.Subject, term.Term) {
			return paths.OneOrMore(paths.Negated(q)), uri("a"), nil
		}},
		{"negated property set excluding all", func(*graph.Graph) (paths.Path, term.Subject, term.Term) {
			return paths.OneOrMore(paths.Negated(p, q)), uri("a"), nil
		}},
		// The outer repetition cannot be flattened, but the inner one is pushed
		// down on every step of the outer traversal — the two layers must still
		// agree with a fully local evaluation.
		{"nested repetition", func(*graph.Graph) (paths.Path, term.Subject, term.Term) {
			return paths.OneOrMore(paths.OneOrMore(paths.AsPath(p))), uri("a"), nil
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertSameAsFallback(t, tc.build)
		})
	}
}

// TestPushdownDeclinedShapes covers the paths that must never be handed to a
// backend: a sequence, an alternation that mixes directions or polarities, and
// `?`, which is not transitive.
func TestPushdownDeclinedShapes(t *testing.T) {
	p, q := uri("p"), uri("q")

	cases := []struct {
		name string
		path paths.Path
		subj term.Subject
	}{
		{"sequence inside repetition", paths.OneOrMore(paths.Sequence(paths.AsPath(p), paths.AsPath(q))), uri("a")},
		{"alternation mixing directions", paths.OneOrMore(paths.Alternative(paths.AsPath(p), paths.Inv(paths.AsPath(q)))), uri("a")},
		{"alternation mixing polarity", paths.OneOrMore(paths.Alternative(paths.AsPath(p), paths.Negated(q))), uri("a")},
		{"ZeroOrOne is not transitive", paths.ZeroOrOne(paths.AsPath(p)), uri("a")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &countingStore{MemoryStore: store.NewMemoryStore()}
			g := reachGraph(s)
			pairs(g, tc.path, tc.subj, nil)
			if s.calls != 0 {
				t.Errorf("Reachable called %d times for a shape it cannot express", s.calls)
			}
		})
	}
}

// TestPushdownUnboundEndpoints checks that a path with neither endpoint bound
// stays local: pushing it down would be one round trip per node in the graph.
func TestPushdownUnboundEndpoints(t *testing.T) {
	s := &countingStore{MemoryStore: store.NewMemoryStore()}
	g := reachGraph(s)
	pairs(g, paths.OneOrMore(paths.AsPath(uri("p"))), nil, nil)
	if s.calls != 0 {
		t.Errorf("Reachable called %d times with both endpoints unbound", s.calls)
	}
}

// TestBackwardPathToLiteral pins the behaviour directly rather than only
// relative to the fallback: a path may end at a literal.
func TestBackwardPathToLiteral(t *testing.T) {
	g := reachGraph(store.NewMemoryStore())
	got := pairs(g, paths.OneOrMore(paths.AsPath(uri("p"))), nil, term.NewLiteral("leaf"))

	// a, b and c all reach "leaf" through p+ (c directly, b and a through c).
	// d does not: its only outgoing edge uses q.
	want := []string{
		`<http://example.org/a> -> "leaf"`,
		`<http://example.org/b> -> "leaf"`,
		`<http://example.org/c> -> "leaf"`,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// TestPushdownErrorFallsBackSilently checks that a backend failure that is not
// ErrReachabilityUnsupported is still treated as "compute this yourself" — the
// query must produce the right answer, not an empty one.
func TestPushdownErrorFallsBackSilently(t *testing.T) {
	broken := &brokenStore{MemoryStore: store.NewMemoryStore()}
	g := reachGraph(broken)
	got := pairs(g, paths.OneOrMore(paths.AsPath(uri("p"))), uri("a"), nil)
	if len(got) == 0 {
		t.Fatal("a backend error swallowed the result instead of falling back")
	}
}

type brokenStore struct {
	*store.MemoryStore
}

func (b *brokenStore) Reachable(store.ReachabilityQuery) ([]term.Term, error) {
	return nil, errors.New("connection reset")
}
