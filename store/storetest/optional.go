package storetest

// Optional-interface sections of the conformance suite. A backend gets these
// only if it implements the interface; Run detects that rather than asking.

import (
	"fmt"
	"sort"
	"testing"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// --- QueryableStore --------------------------------------------------------

func testQueryable(t *testing.T, cfg Config) {
	seed := func(t *testing.T) (store.Store, store.QueryableStore) {
		s := cfg.New(t)
		for i := 0; i < 10; i++ {
			subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
			s.Add(triple(subj, Name, lit(i)), nil)
		}
		s.Add(triple(Alice, Knows, Bob), nil)
		return s, s.(store.QueryableStore)
	}

	cfg.run(t, "Count agrees with Triples", func(t *testing.T) {
		s, q := seed(t)
		pats := []struct {
			name string
			pat  term.TriplePattern
		}{
			{"wildcard", pattern(nil, nil, nil)},
			{"predicate", pattern(nil, pred(Name), nil)},
			{"subject", pattern(Alice, nil, nil)},
			{"object", pattern(nil, nil, Bob)},
			{"fully bound, present", pattern(Alice, pred(Knows), Bob)},
			{"fully bound, absent", pattern(Alice, pred(Knows), Carol)},
			{"no match", pattern(Dave, nil, nil)},
		}
		for _, p := range pats {
			want := count(s.Triples(p.pat, nil))
			if got := q.Count(p.pat, nil); got != want {
				t.Errorf("%s: Count = %d, Triples yielded %d", p.name, got, want)
			}
		}
	})

	cfg.run(t, "Exists agrees with Count", func(t *testing.T) {
		s, q := seed(t)
		pats := []term.TriplePattern{
			pattern(nil, nil, nil),
			pattern(Alice, pred(Knows), Bob),
			pattern(Alice, pred(Knows), Carol),
			pattern(Dave, nil, nil),
		}
		for _, p := range pats {
			want := count(s.Triples(p, nil)) > 0
			if got := q.Exists(p, nil); got != want {
				t.Errorf("Exists = %v, want %v", got, want)
			}
		}
	})

	cfg.run(t, "Exists on an empty store is false", func(t *testing.T) {
		s := cfg.New(t)
		q := s.(store.QueryableStore)
		if q.Exists(pattern(nil, nil, nil), nil) {
			t.Error("Exists reported true on an empty store")
		}
		if got := q.Count(pattern(nil, nil, nil), nil); got != 0 {
			t.Errorf("Count = %d on an empty store, want 0", got)
		}
	})

	cfg.run(t, "TriplesWithLimit windows the result", func(t *testing.T) {
		s, q := seed(t)
		all := collect(s.Triples(pattern(nil, nil, nil), nil))
		inAll := map[string]bool{}
		for _, tr := range all {
			inAll[tr] = true
		}

		// Only the window arithmetic is checked, not which triples land in
		// which window. Store does not promise a stable iteration order, and
		// MemoryStore genuinely does not have one — Go randomizes map order per
		// range — so a backend must not be required to page consistently across
		// separate calls. What every backend must get right is how many triples
		// come back for a given limit and offset, and that they are real
		// triples from the matching set.
		for _, tc := range []struct{ limit, offset int }{
			{3, 0}, {3, 3}, {3, 9}, {3, len(all)}, {3, len(all) + 5},
			{len(all), 0}, {len(all) + 10, 0}, {1, 1},
		} {
			want := len(all) - tc.offset
			if want < 0 {
				want = 0
			}
			if tc.limit > 0 && want > tc.limit {
				want = tc.limit
			}
			page := collect(q.TriplesWithLimit(pattern(nil, nil, nil), nil, tc.limit, tc.offset))
			if len(page) != want {
				t.Errorf("limit %d offset %d: got %d triples, want %d (store holds %d)",
					tc.limit, tc.offset, len(page), want, len(all))
			}
			for _, tr := range page {
				if !inAll[tr] {
					t.Errorf("limit %d offset %d: %s is not in the unpaged result",
						tc.limit, tc.offset, tr)
				}
			}
		}
	})

	cfg.run(t, "TriplesWithLimit with no limit returns everything after the offset", func(t *testing.T) {
		s, q := seed(t)
		all := count(s.Triples(pattern(nil, nil, nil), nil))
		if got := count(q.TriplesWithLimit(pattern(nil, nil, nil), nil, 0, 0)); got != all {
			t.Errorf("limit 0 returned %d triples, want all %d", got, all)
		}
		if got := count(q.TriplesWithLimit(pattern(nil, nil, nil), nil, -1, 2)); got != all-2 {
			t.Errorf("limit -1 offset 2 returned %d triples, want %d", got, all-2)
		}
	})

	cfg.run(t, "TriplesWithLimit honours the pattern", func(t *testing.T) {
		_, q := seed(t)
		got := count(q.TriplesWithLimit(pattern(nil, pred(Name), nil), nil, 4, 0))
		if got != 4 {
			t.Errorf("got %d triples, want 4", got)
		}
	})

	cfg.run(t, "TriplesWithLimit past the end is empty", func(t *testing.T) {
		_, q := seed(t)
		if got := count(q.TriplesWithLimit(pattern(nil, nil, nil), nil, 5, 1000)); got != 0 {
			t.Errorf("got %d triples past the end, want 0", got)
		}
	})

	cfg.run(t, "TriplesWithLimit stops when the consumer stops", func(t *testing.T) {
		_, q := seed(t)
		seen := 0
		q.TriplesWithLimit(pattern(nil, nil, nil), nil, 5, 0)(func(term.Triple) bool {
			seen++
			return false
		})
		if seen != 1 {
			t.Errorf("iterator yielded %d triples after the consumer returned false, want 1", seen)
		}
	})

	cfg.run(t, "is scoped to its graph", func(t *testing.T) {
		s := cfg.New(t)
		if !s.ContextAware() {
			t.Skip("backend is not context aware")
		}
		q := s.(store.QueryableStore)
		s.Add(triple(Alice, Name, lit("Alice")), Graph1)
		s.Add(triple(Bob, Name, lit("Bob")), nil)

		if got := q.Count(pattern(nil, nil, nil), Graph1); got != 1 {
			t.Errorf("Count(graph1) = %d, want 1", got)
		}
		if got := q.Count(pattern(nil, nil, nil), nil); got != 1 {
			t.Errorf("Count(default) = %d, want 1", got)
		}
		if q.Exists(pattern(Alice, nil, nil), nil) {
			t.Error("Exists found a graph1 triple in the default graph")
		}
	})
}

// --- CardinalityStore ------------------------------------------------------

// testCardinality pins Cardinality to what Triples yields for every pattern
// shape, including after removals: an index-size answer that drifts from the
// data is worse than no answer, because the planner trusts it silently.
func testCardinality(t *testing.T, cfg Config) {
	shapes := func() []struct {
		name string
		pat  term.TriplePattern
	} {
		return []struct {
			name string
			pat  term.TriplePattern
		}{
			{"wildcard", pattern(nil, nil, nil)},
			{"subject", pattern(Alice, nil, nil)},
			{"predicate", pattern(nil, pred(Knows), nil)},
			{"object", pattern(nil, nil, Bob)},
			{"subject+predicate", pattern(Alice, pred(Knows), nil)},
			{"predicate+object", pattern(nil, pred(Knows), Bob)},
			{"subject+object", pattern(Alice, nil, Bob)},
			{"fully bound, present", pattern(Alice, pred(Knows), Bob)},
			{"fully bound, absent", pattern(Alice, pred(Knows), Dave)},
			{"no match", pattern(Dave, nil, nil)},
		}
	}
	check := func(t *testing.T, s store.Store) {
		t.Helper()
		c := s.(store.CardinalityStore)
		for _, sh := range shapes() {
			want := count(s.Triples(sh.pat, nil))
			if got := c.Cardinality(sh.pat, nil); got != want {
				t.Errorf("%s: Cardinality = %d, Triples yielded %d", sh.name, got, want)
			}
		}
	}
	seed := func(t *testing.T) store.Store {
		s := cfg.New(t)
		s.Add(triple(Alice, Knows, Bob), nil)
		s.Add(triple(Alice, Knows, Carol), nil)
		s.Add(triple(Alice, Name, lit("Alice")), nil)
		s.Add(triple(Bob, Knows, Carol), nil)
		s.Add(triple(Carol, Knows, Bob), nil)
		s.Add(triple(Carol, Name, Bob), nil)
		return s
	}

	cfg.run(t, "agrees with Triples for every pattern shape", func(t *testing.T) {
		check(t, seed(t))
	})
	cfg.run(t, "stays in step with duplicates, Remove and Set", func(t *testing.T) {
		s := seed(t)
		s.Add(triple(Alice, Knows, Bob), nil) // duplicate: must not count twice
		check(t, s)
		s.Remove(pattern(nil, pred(Knows), Carol), nil)
		check(t, s)
		s.Set(triple(Alice, Knows, Dave), nil)
		check(t, s)
		s.Remove(pattern(nil, nil, nil), nil)
		check(t, s)
	})

	// The planner passes the graph it is evaluating as the context, so a count
	// that leaks triples from another graph — a per-predicate counter shared by
	// every graph, say — silently reorders joins on a wrong estimate.
	cfg.run(t, "is scoped to its graph", func(t *testing.T) {
		s := cfg.New(t)
		if !s.ContextAware() {
			t.Skip("backend is not context aware")
		}
		c := s.(store.CardinalityStore)
		// The default graph and graph1 share one triple and differ in every
		// other count, so no pattern shape can pass by reading the wrong graph.
		s.Add(triple(Alice, Knows, Bob), nil)
		s.Add(triple(Alice, Knows, Carol), nil)
		s.Add(triple(Bob, Knows, Carol), nil)
		s.Add(triple(Alice, Knows, Bob), Graph1)
		s.Add(triple(Carol, Knows, Bob), Graph1)
		s.Add(triple(Carol, Name, Bob), Graph1)
		s.Add(triple(Dave, Knows, Bob), Graph1)
		s.Add(triple(Dave, Knows, Alice), Graph2)

		checkIn := func(t *testing.T, what string) {
			t.Helper()
			for _, ctx := range []term.Term{nil, Graph1, Graph2, Graph3} {
				for _, sh := range shapes() {
					want := count(s.Triples(sh.pat, ctx))
					if got := c.Cardinality(sh.pat, ctx); got != want {
						t.Errorf("%s, context %v, %s: Cardinality = %d, Triples yielded %d",
							what, ctx, sh.name, got, want)
					}
				}
			}
		}
		checkIn(t, "seeded")

		for _, tc := range []struct {
			ctx  term.Term
			pat  term.TriplePattern
			want int
		}{
			{nil, pattern(nil, pred(Knows), nil), 3},
			{Graph1, pattern(nil, pred(Knows), nil), 3},
			{Graph1, pattern(nil, pred(Name), nil), 1},
			{nil, pattern(nil, pred(Name), nil), 0},
			{Graph1, pattern(nil, nil, Bob), 4},
			{Graph2, pattern(nil, nil, nil), 1},
			{Graph3, pattern(nil, nil, nil), 0},
		} {
			if got := c.Cardinality(tc.pat, tc.ctx); got != tc.want {
				t.Errorf("context %v: Cardinality = %d, want %d", tc.ctx, got, tc.want)
			}
		}

		s.Remove(pattern(nil, pred(Knows), nil), Graph1)
		checkIn(t, "after Remove in graph1")
		if got := c.Cardinality(pattern(nil, pred(Knows), nil), nil); got != 3 {
			t.Errorf("Remove in graph1 changed the default graph's count to %d, want 3", got)
		}
		s.Set(triple(Alice, Knows, Dave), Graph2)
		checkIn(t, "after Set in graph2")
		s.Remove(pattern(nil, nil, nil), Graph1)
		checkIn(t, "after emptying graph1")
	})
}

// --- ReachabilityStore -----------------------------------------------------

// testReachability checks the contract in store.ReachabilityStore against a
// graph built to break a naive implementation: a cycle that must terminate, a
// branch, a second predicate that must be excluded, and a literal endpoint that
// must still be reported.
//
//	a --knows--> b --knows--> c --knows--> a
//	b --knows--> d
//	d --name--> "Dave"
func testReachability(t *testing.T, cfg Config) {
	a := term.NewURIRefUnsafe("http://example.org/a")
	b := term.NewURIRefUnsafe("http://example.org/b")
	c := term.NewURIRefUnsafe("http://example.org/c")
	d := term.NewURIRefUnsafe("http://example.org/d")
	dave := lit("Dave")

	seed := func(t *testing.T) (store.Store, store.ReachabilityStore) {
		s := cfg.New(t)
		s.Add(triple(a, Knows, b), nil)
		s.Add(triple(b, Knows, c), nil)
		s.Add(triple(c, Knows, a), nil)
		s.Add(triple(b, Knows, d), nil)
		s.Add(triple(d, Name, dave), nil)
		return s, s.(store.ReachabilityStore)
	}

	// keys renders a result set as a sorted list, skipping the comparison
	// entirely when the backend declined — declining is always allowed.
	run := func(t *testing.T, r store.ReachabilityStore, q store.ReachabilityQuery) ([]string, bool) {
		t.Helper()
		got, err := r.Reachable(q)
		if err != nil {
			t.Logf("backend declined %v: %v", q, err)
			return nil, false
		}
		out := make([]string, 0, len(got))
		for _, n := range got {
			out = append(out, n.N3())
		}
		sort.Strings(out)
		return out, true
	}

	same := func(t *testing.T, name string, got, want []string) {
		t.Helper()
		sort.Strings(want)
		if len(got) != len(want) {
			t.Errorf("%s: got %v, want %v", name, got, want)
			return
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%s: got %v, want %v", name, got, want)
				return
			}
		}
	}

	cases := []struct {
		name  string
		query store.ReachabilityQuery
		want  []string
	}{
		{
			// The cycle brings a back into its own reachable set.
			"forward over one predicate",
			store.ReachabilityQuery{Start: a, Predicates: []term.URIRef{Knows}},
			[]string{a.N3(), b.N3(), c.N3(), d.N3()},
		},
		{
			"forward from a mid-cycle node",
			store.ReachabilityQuery{Start: b, Predicates: []term.URIRef{Knows}},
			[]string{a.N3(), b.N3(), c.N3(), d.N3()},
		},
		{
			"forward from a leaf",
			store.ReachabilityQuery{Start: d, Predicates: []term.URIRef{Knows}},
			nil,
		},
		{
			"forward from a node that is not in the graph",
			store.ReachabilityQuery{Start: Dave, Predicates: []term.URIRef{Knows}},
			nil,
		},
		{
			"depth of one is a single hop",
			store.ReachabilityQuery{Start: a, Predicates: []term.URIRef{Knows}, MaxDepth: 1},
			[]string{b.N3()},
		},
		{
			"depth of two",
			store.ReachabilityQuery{Start: a, Predicates: []term.URIRef{Knows}, MaxDepth: 2},
			[]string{b.N3(), c.N3(), d.N3()},
		},
		{
			"inverse traversal",
			store.ReachabilityQuery{Start: c, Predicates: []term.URIRef{Knows}, Inverse: true},
			[]string{a.N3(), b.N3(), c.N3()},
		},
		{
			"inverse traversal from a literal",
			store.ReachabilityQuery{Start: dave, Predicates: []term.URIRef{Name}, Inverse: true},
			[]string{d.N3()},
		},
		{
			// A literal is a reachable node even though nothing leads out of it.
			"a literal endpoint is reported",
			store.ReachabilityQuery{Start: b, Predicates: []term.URIRef{Knows, Name}},
			[]string{a.N3(), b.N3(), c.N3(), d.N3(), dave.N3()},
		},
		{
			"no predicates means any predicate",
			store.ReachabilityQuery{Start: b},
			[]string{a.N3(), b.N3(), c.N3(), d.N3(), dave.N3()},
		},
		{
			"negated set excludes its predicates",
			store.ReachabilityQuery{Start: b, Predicates: []term.URIRef{Name}, Negated: true},
			[]string{a.N3(), b.N3(), c.N3(), d.N3()},
		},
		{
			"an empty negated set excludes nothing",
			store.ReachabilityQuery{Start: b, Negated: true},
			[]string{a.N3(), b.N3(), c.N3(), d.N3(), dave.N3()},
		},
		{
			"a predicate that appears in no triple reaches nothing",
			store.ReachabilityQuery{Start: a, Predicates: []term.URIRef{Label}},
			nil,
		},
	}

	for _, tc := range cases {
		cfg.run(t, tc.name, func(t *testing.T) {
			_, r := seed(t)
			got, ok := run(t, r, tc.query)
			if !ok {
				t.Skip("backend declined this query shape")
			}
			same(t, tc.name, got, tc.want)
		})
	}

	cfg.run(t, "a nil start is refused", func(t *testing.T) {
		_, r := seed(t)
		if _, err := r.Reachable(store.ReachabilityQuery{Predicates: []term.URIRef{Knows}}); err == nil {
			t.Error("Reachable accepted a query with no Start")
		}
	})

	cfg.run(t, "is scoped to its graph", func(t *testing.T) {
		s := cfg.New(t)
		if !s.ContextAware() {
			t.Skip("backend is not context aware")
		}
		r := s.(store.ReachabilityStore)
		s.Add(triple(a, Knows, b), Graph1)
		s.Add(triple(b, Knows, c), nil)

		got, ok := run(t, r, store.ReachabilityQuery{Start: a, Predicates: []term.URIRef{Knows}, Context: Graph1})
		if !ok {
			t.Skip("backend declined this query shape")
		}
		same(t, "graph1 only", got, []string{b.N3()})
	})
}
