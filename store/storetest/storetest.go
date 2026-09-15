package storetest

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// Config describes the backend under test.
type Config struct {
	// New returns a fresh, empty store. It is called once per subtest, so it
	// must not hand out a store that shares state with a previous call —
	// a file-backed backend should use t.TempDir(), a server-backed one a
	// per-test database or graph namespace.
	//
	// New is responsible for cleanup, normally through t.Cleanup.
	New func(t *testing.T) store.Store

	// Reopen closes s and returns a new store over the same backing storage.
	// Leave it nil for a non-persistent backend; the persistence section is
	// then skipped.
	//
	// It exists because persistence is the one capability that cannot be
	// detected from the interface: only the backend knows how to point a second
	// handle at the same bytes.
	Reopen func(t *testing.T, s store.Store) store.Store

	// SkipConcurrency disables the concurrent-access section. Set it only for a
	// backend that documents itself as unsafe for concurrent use — every store
	// in this repository leaves it false.
	SkipConcurrency bool

	// Known declares behaviour the backend cannot provide, mapping a subtest
	// path relative to Run — "TermRoundTrip/BNode subject", "Contexts" for a
	// whole section — to the reason. Matching subtests are skipped with that
	// reason instead of failing.
	//
	// It exists so that a real limitation is written down and visible in the
	// test output rather than worked around by not running the suite. Every
	// entry needs a non-empty reason: an unexplained exemption is how a bug
	// turns into a documented feature.
	//
	// A path matches a subtest and everything under it. Use it only for what
	// the backend genuinely cannot do — a remote protocol with no way to name a
	// blank node, a server that does not implement named graphs — never to
	// silence a failure that is a bug.
	Known map[string]string
}

// Vocabulary shared by the whole suite. Exported so a backend can reuse the
// same terms in its own tests, and so a failure message names something the
// reader has already seen.
var (
	Alice = term.NewURIRefUnsafe("http://example.org/Alice")
	Bob   = term.NewURIRefUnsafe("http://example.org/Bob")
	Carol = term.NewURIRefUnsafe("http://example.org/Carol")
	Dave  = term.NewURIRefUnsafe("http://example.org/Dave")

	Name  = term.NewURIRefUnsafe("http://example.org/name")
	Age   = term.NewURIRefUnsafe("http://example.org/age")
	Knows = term.NewURIRefUnsafe("http://example.org/knows")
	Label = term.NewURIRefUnsafe("http://example.org/label")

	Graph1 = term.NewURIRefUnsafe("http://example.org/graph1")
	Graph2 = term.NewURIRefUnsafe("http://example.org/graph2")
	Graph3 = term.NewURIRefUnsafe("http://example.org/graph3")
)

// Run executes the full conformance suite against the backend described by cfg.
//
// Sections that depend on an optional interface are enabled by detecting it on
// the store cfg.New returns, so a backend gets exactly the coverage it has
// earned and nothing it has to opt out of.
func Run(t *testing.T, cfg Config) {
	t.Helper()
	if cfg.New == nil {
		t.Fatal("storetest: Config.New is required")
	}
	for path, reason := range cfg.Known {
		if reason == "" {
			t.Fatalf("storetest: Config.Known[%q] has no reason", path)
		}
	}
	cfg.section(t)

	cfg.run(t, "Add", func(t *testing.T) { testAdd(t, cfg) })
	cfg.run(t, "AddN", func(t *testing.T) { testAddN(t, cfg) })
	cfg.run(t, "Remove", func(t *testing.T) { testRemove(t, cfg) })
	cfg.run(t, "Set", func(t *testing.T) { testSet(t, cfg) })
	cfg.run(t, "Triples", func(t *testing.T) { testTriples(t, cfg) })
	cfg.run(t, "Contexts", func(t *testing.T) { testContexts(t, cfg) })
	cfg.run(t, "Namespaces", func(t *testing.T) { testNamespaces(t, cfg) })
	cfg.run(t, "ContextConventions", func(t *testing.T) { testContextConventions(t, cfg) })
	cfg.run(t, "TermRoundTrip", func(t *testing.T) { testTermRoundTrip(t, cfg) })
	cfg.run(t, "EmptyStore", func(t *testing.T) { testEmptyStore(t, cfg) })

	probe := cfg.New(t)
	if _, ok := probe.(store.QueryableStore); ok {
		cfg.run(t, "Queryable", func(t *testing.T) { testQueryable(t, cfg) })
	} else {
		t.Log("backend does not implement store.QueryableStore; section skipped")
	}
	if _, ok := probe.(store.SnapshotStore); ok {
		cfg.run(t, "Snapshot", func(t *testing.T) { testSnapshot(t, cfg) })
	} else {
		t.Log("backend does not implement store.SnapshotStore; section skipped")
	}
	if _, ok := probe.(store.CardinalityStore); ok {
		cfg.run(t, "Cardinality", func(t *testing.T) { testCardinality(t, cfg) })
	} else {
		t.Log("backend does not implement store.CardinalityStore; section skipped")
	}
	if _, ok := probe.(store.ReachabilityStore); ok {
		cfg.run(t, "Reachability", func(t *testing.T) { testReachability(t, cfg) })
	} else {
		t.Log("backend does not implement store.ReachabilityStore; section skipped")
	}

	if !cfg.SkipConcurrency {
		cfg.run(t, "Concurrency", func(t *testing.T) { testConcurrency(t, cfg) })
	}
	if cfg.Reopen != nil {
		cfg.run(t, "Persistence", func(t *testing.T) { testPersistence(t, cfg) })
	} else {
		t.Log("Config.Reopen is nil; persistence section skipped")
	}
}

// --- helpers ---------------------------------------------------------------

// section skips the current test if cfg.Known covers it. It is called at the
// top of every subtest through run, and once in Run itself so that a whole
// suite can be exempted.
func (cfg Config) section(t *testing.T) {
	t.Helper()
	if len(cfg.Known) == 0 {
		return
	}
	// t.Name() is "TestX/Section/case"; the paths in Known are relative to Run,
	// so compare against every suffix of the name.
	name := t.Name()
	for i := 0; i < len(name); i++ {
		if i != 0 && name[i-1] != '/' {
			continue
		}
		if reason, ok := cfg.Known[strings.ReplaceAll(name[i:], "_", " ")]; ok {
			t.Skipf("declared limitation: %s", reason)
		}
		if reason, ok := cfg.Known[name[i:]]; ok {
			t.Skipf("declared limitation: %s", reason)
		}
	}
}

// run is t.Run plus the Known check, so a declared limitation is skipped with
// its reason wherever it appears.
func (cfg Config) run(t *testing.T, name string, fn func(*testing.T)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		cfg.section(t)
		fn(t)
	})
}

func triple(s term.Subject, p term.URIRef, o term.Term) term.Triple {
	return term.Triple{Subject: s, Predicate: p, Object: o}
}

func lit(v any, opts ...term.LiteralOption) term.Literal { return term.NewLiteral(v, opts...) }

func pred(p term.URIRef) *term.URIRef { return &p }

// collect materializes an iterator into a sorted list of N3 forms, which makes
// both comparison and failure messages independent of a backend's iteration
// order.
func collect(it store.TripleIterator) []string {
	var out []string
	it(func(t term.Triple) bool {
		out = append(out, fmt.Sprintf("%s %s %s", t.Subject.N3(), t.Predicate.N3(), t.Object.N3()))
		return true
	})
	sort.Strings(out)
	return out
}

func count(it store.TripleIterator) int {
	n := 0
	it(func(term.Triple) bool {
		n++
		return true
	})
	return n
}

func pattern(s term.Subject, p *term.URIRef, o term.Term) term.TriplePattern {
	return term.TriplePattern{Subject: s, Predicate: p, Object: o}
}

func wantLen(t *testing.T, s store.Store, ctx term.Term, want int, what string) {
	t.Helper()
	if got := s.Len(ctx); got != want {
		t.Errorf("%s: Len = %d, want %d", what, got, want)
	}
}

func wantTriples(t *testing.T, s store.Store, pat term.TriplePattern, ctx term.Term, want int, what string) {
	t.Helper()
	if got := count(s.Triples(pat, ctx)); got != want {
		t.Errorf("%s: %d triples, want %d", what, got, want)
	}
}

// --- Add -------------------------------------------------------------------

func testAdd(t *testing.T, cfg Config) {
	cfg.run(t, "counts", func(t *testing.T) {
		s := cfg.New(t)
		s.Add(triple(Alice, Name, lit("Alice")), nil)
		s.Add(triple(Alice, Age, lit(30)), nil)
		wantLen(t, s, nil, 2, "two distinct triples")
	})

	cfg.run(t, "is idempotent", func(t *testing.T) {
		s := cfg.New(t)
		tr := triple(Alice, Name, lit("Alice"))
		s.Add(tr, nil)
		s.Add(tr, nil)
		s.Add(tr, nil)
		wantLen(t, s, nil, 1, "same triple added three times")
	})

	cfg.run(t, "the same triple in two graphs is two triples", func(t *testing.T) {
		s := cfg.New(t)
		if !s.ContextAware() {
			t.Skip("backend is not context aware")
		}
		tr := triple(Alice, Name, lit("Alice"))
		s.Add(tr, Graph1)
		s.Add(tr, Graph2)
		wantLen(t, s, Graph1, 1, "graph1")
		wantLen(t, s, Graph2, 1, "graph2")
		wantLen(t, s, nil, 0, "default graph must be untouched")
	})
}

// --- AddN ------------------------------------------------------------------

func testAddN(t *testing.T, cfg Config) {
	cfg.run(t, "batch", func(t *testing.T) {
		s := cfg.New(t)
		s.AddN([]term.Quad{
			{Triple: triple(Alice, Name, lit("Alice"))},
			{Triple: triple(Bob, Name, lit("Bob"))},
		})
		wantLen(t, s, nil, 2, "two quads")
	})

	cfg.run(t, "empty and nil are no-ops", func(t *testing.T) {
		s := cfg.New(t)
		s.AddN(nil)
		s.AddN([]term.Quad{})
		wantLen(t, s, nil, 0, "no quads")
	})

	cfg.run(t, "deduplicates within a batch", func(t *testing.T) {
		s := cfg.New(t)
		tr := triple(Alice, Name, lit("Alice"))
		s.AddN([]term.Quad{{Triple: tr}, {Triple: tr}, {Triple: tr}})
		wantLen(t, s, nil, 1, "one distinct quad repeated")
	})

	cfg.run(t, "routes quads to their graphs", func(t *testing.T) {
		s := cfg.New(t)
		if !s.ContextAware() {
			t.Skip("backend is not context aware")
		}
		s.AddN([]term.Quad{
			{Triple: triple(Alice, Name, lit("Alice")), Graph: Graph1},
			{Triple: triple(Bob, Name, lit("Bob")), Graph: Graph2},
			{Triple: triple(Carol, Name, lit("Carol"))},
		})
		wantLen(t, s, Graph1, 1, "graph1")
		wantLen(t, s, Graph2, 1, "graph2")
		wantLen(t, s, nil, 1, "default graph")
	})

	// A batch large enough to cross whatever chunking or transaction size the
	// backend uses internally. Backends have shipped bugs on exactly the batch
	// boundary, which a two-element test never reaches.
	cfg.run(t, "large batch", func(t *testing.T) {
		s := cfg.New(t)
		const n = 2500
		quads := make([]term.Quad, 0, n)
		for i := 0; i < n; i++ {
			subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
			quads = append(quads, term.Quad{Triple: triple(subj, Name, lit(i))})
		}
		s.AddN(quads)
		wantLen(t, s, nil, n, "large batch")
	})
}

// --- Remove ----------------------------------------------------------------

func testRemove(t *testing.T, cfg Config) {
	seed := func(t *testing.T) store.Store {
		s := cfg.New(t)
		s.Add(triple(Alice, Name, lit("Alice")), nil)
		s.Add(triple(Alice, Age, lit(30)), nil)
		s.Add(triple(Bob, Name, lit("Bob")), nil)
		s.Add(triple(Bob, Knows, Alice), nil)
		return s
	}

	cases := []struct {
		name string
		pat  term.TriplePattern
		want int
	}{
		{"exact triple", pattern(Alice, pred(Name), lit("Alice")), 3},
		{"by subject", pattern(Alice, nil, nil), 2},
		{"by predicate", pattern(nil, pred(Name), nil), 2},
		{"by object", pattern(nil, nil, lit("Alice")), 3},
		{"by subject and object", pattern(Bob, nil, Alice), 3},
		{"by predicate and object", pattern(nil, pred(Name), lit("Bob")), 3},
		{"everything", pattern(nil, nil, nil), 0},
		{"no match leaves the store alone", pattern(Carol, nil, nil), 4},
	}

	for _, tc := range cases {
		cfg.run(t, tc.name, func(t *testing.T) {
			s := seed(t)
			s.Remove(tc.pat, nil)
			wantLen(t, s, nil, tc.want, tc.name)
		})
	}

	cfg.run(t, "stays inside its graph", func(t *testing.T) {
		s := cfg.New(t)
		if !s.ContextAware() {
			t.Skip("backend is not context aware")
		}
		tr := triple(Alice, Name, lit("Alice"))
		s.Add(tr, Graph1)
		s.Add(tr, Graph2)
		s.Add(tr, nil)

		s.Remove(pattern(nil, nil, nil), Graph1)
		wantLen(t, s, Graph1, 0, "graph1 after Remove")
		wantLen(t, s, Graph2, 1, "graph2 must be untouched")
		wantLen(t, s, nil, 1, "default graph must be untouched")
	})
}

// --- Set -------------------------------------------------------------------

func testSet(t *testing.T, cfg Config) {
	cfg.run(t, "replaces the value", func(t *testing.T) {
		s := cfg.New(t)
		s.Add(triple(Alice, Name, lit("Alice")), nil)
		s.Set(triple(Alice, Name, lit("Alicia")), nil)
		wantLen(t, s, nil, 1, "after Set")

		got := collect(s.Triples(pattern(Alice, pred(Name), nil), nil))
		want := []string{`<http://example.org/Alice> <http://example.org/name> "Alicia"`}
		if len(got) != 1 || got[0] != want[0] {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	cfg.run(t, "collapses multiple existing values", func(t *testing.T) {
		s := cfg.New(t)
		s.Add(triple(Alice, Name, lit("A")), nil)
		s.Add(triple(Alice, Name, lit("B")), nil)
		s.Add(triple(Alice, Name, lit("C")), nil)
		s.Set(triple(Alice, Name, lit("D")), nil)
		wantLen(t, s, nil, 1, "after Set over three values")
	})

	cfg.run(t, "leaves other predicates alone", func(t *testing.T) {
		s := cfg.New(t)
		s.Add(triple(Alice, Name, lit("Alice")), nil)
		s.Add(triple(Alice, Age, lit(30)), nil)
		s.Set(triple(Alice, Name, lit("Alicia")), nil)
		wantLen(t, s, nil, 2, "name replaced, age kept")
	})

	cfg.run(t, "works on an empty store", func(t *testing.T) {
		s := cfg.New(t)
		s.Set(triple(Alice, Name, lit("Alice")), nil)
		wantLen(t, s, nil, 1, "Set with nothing to replace")
	})

	cfg.run(t, "stays inside its graph", func(t *testing.T) {
		s := cfg.New(t)
		if !s.ContextAware() {
			t.Skip("backend is not context aware")
		}
		s.Add(triple(Alice, Name, lit("old")), Graph1)
		s.Add(triple(Alice, Name, lit("old")), nil)
		s.Set(triple(Alice, Name, lit("new")), Graph1)
		wantLen(t, s, Graph1, 1, "graph1")
		wantLen(t, s, nil, 1, "default graph")

		got := collect(s.Triples(pattern(nil, nil, nil), nil))
		if len(got) != 1 || got[0] != `<http://example.org/Alice> <http://example.org/name> "old"` {
			t.Errorf("default graph was modified by a Set in graph1: %v", got)
		}
	})
}

// --- Triples ---------------------------------------------------------------

func testTriples(t *testing.T, cfg Config) {
	seed := func(t *testing.T) store.Store {
		s := cfg.New(t)
		s.Add(triple(Alice, Name, lit("Alice")), nil)
		s.Add(triple(Alice, Age, lit(30)), nil)
		s.Add(triple(Bob, Name, lit("Bob")), nil)
		s.Add(triple(Bob, Knows, Alice), nil)
		return s
	}

	// All eight binding combinations. The fully bound pattern is called out
	// because it is the one an implementation is most likely to route through a
	// wildcard branch by accident, which makes every existence check succeed.
	cases := []struct {
		name string
		pat  term.TriplePattern
		want int
	}{
		{"wildcard", pattern(nil, nil, nil), 4},
		{"subject", pattern(Alice, nil, nil), 2},
		{"predicate", pattern(nil, pred(Name), nil), 2},
		{"object", pattern(nil, nil, Alice), 1},
		{"subject+predicate", pattern(Alice, pred(Name), nil), 1},
		{"subject+object", pattern(Bob, nil, Alice), 1},
		{"predicate+object", pattern(nil, pred(Name), lit("Bob")), 1},
		{"fully bound, present", pattern(Alice, pred(Name), lit("Alice")), 1},
		{"fully bound, absent", pattern(Alice, pred(Name), lit("nobody")), 0},
		{"fully bound, absent subject", pattern(Carol, pred(Name), lit("Alice")), 0},
	}

	for _, tc := range cases {
		cfg.run(t, tc.name, func(t *testing.T) {
			s := seed(t)
			wantTriples(t, s, tc.pat, nil, tc.want, tc.name)
		})
	}

	cfg.run(t, "stops when the consumer stops", func(t *testing.T) {
		s := seed(t)
		seen := 0
		s.Triples(pattern(nil, nil, nil), nil)(func(term.Triple) bool {
			seen++
			return false
		})
		if seen != 1 {
			t.Errorf("iterator yielded %d triples after the consumer returned false, want 1", seen)
		}
	})

	cfg.run(t, "is restartable", func(t *testing.T) {
		s := seed(t)
		it := s.Triples(pattern(nil, nil, nil), nil)
		first, second := count(it), count(it)
		if first != second {
			t.Errorf("second pass over the same iterator yielded %d, first yielded %d", second, first)
		}
	})

	cfg.run(t, "is scoped to its graph", func(t *testing.T) {
		s := cfg.New(t)
		if !s.ContextAware() {
			t.Skip("backend is not context aware")
		}
		s.Add(triple(Alice, Name, lit("Alice")), Graph1)
		s.Add(triple(Bob, Name, lit("Bob")), Graph2)
		wantTriples(t, s, pattern(nil, nil, nil), Graph1, 1, "graph1")
		wantTriples(t, s, pattern(nil, nil, nil), Graph2, 1, "graph2")
		wantTriples(t, s, pattern(nil, nil, nil), nil, 0, "default graph")
		wantTriples(t, s, pattern(nil, nil, nil), Graph3, 0, "unused graph")
	})
}

// --- Contexts --------------------------------------------------------------

func testContexts(t *testing.T, cfg Config) {
	names := func(s store.Store, tr *term.Triple) []string {
		var out []string
		s.Contexts(tr)(func(c term.Term) bool {
			out = append(out, c.N3())
			return true
		})
		sort.Strings(out)
		return out
	}

	cfg.run(t, "empty store has none", func(t *testing.T) {
		s := cfg.New(t)
		if got := names(s, nil); len(got) != 0 {
			t.Errorf("got %v, want none", got)
		}
	})

	cfg.run(t, "lists named graphs and excludes the default graph", func(t *testing.T) {
		s := cfg.New(t)
		if !s.ContextAware() {
			t.Skip("backend is not context aware")
		}
		s.Add(triple(Alice, Name, lit("Alice")), Graph1)
		s.Add(triple(Bob, Name, lit("Bob")), Graph2)
		s.Add(triple(Carol, Name, lit("Carol")), nil)

		got := names(s, nil)
		want := []string{Graph1.N3(), Graph2.N3()}
		if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Errorf("got %v, want exactly %v — the default graph is not a context", got, want)
		}
	})

	cfg.run(t, "filters by triple", func(t *testing.T) {
		s := cfg.New(t)
		if !s.ContextAware() {
			t.Skip("backend is not context aware")
		}
		tr := triple(Alice, Name, lit("Alice"))
		s.Add(tr, Graph1)
		s.Add(tr, Graph2)
		s.Add(triple(Bob, Name, lit("Bob")), Graph3)

		if got := names(s, &tr); len(got) != 2 {
			t.Errorf("got %v, want the two graphs holding the triple", got)
		}
		other := triple(Dave, Name, lit("Dave"))
		if got := names(s, &other); len(got) != 0 {
			t.Errorf("got %v for a triple in no graph, want none", got)
		}
	})

	cfg.run(t, "stops when the consumer stops", func(t *testing.T) {
		s := cfg.New(t)
		if !s.ContextAware() {
			t.Skip("backend is not context aware")
		}
		s.Add(triple(Alice, Name, lit("Alice")), Graph1)
		s.Add(triple(Bob, Name, lit("Bob")), Graph2)
		s.Add(triple(Carol, Name, lit("Carol")), Graph3)

		seen := 0
		s.Contexts(nil)(func(term.Term) bool {
			seen++
			return false
		})
		if seen != 1 {
			t.Errorf("iterator yielded %d contexts after the consumer returned false, want 1", seen)
		}
	})
}

// --- Namespaces ------------------------------------------------------------

func testNamespaces(t *testing.T, cfg Config) {
	ex := term.NewURIRefUnsafe("http://example.org/")
	other := term.NewURIRefUnsafe("http://other.example/")

	cfg.run(t, "round-trips both ways", func(t *testing.T) {
		s := cfg.New(t)
		s.Bind("ex", ex)

		got, ok := s.Namespace("ex")
		if !ok || got.Value() != ex.Value() {
			t.Errorf("Namespace(ex) = %v, %v; want %v, true", got, ok, ex)
		}
		prefix, ok := s.Prefix(ex)
		if !ok || prefix != "ex" {
			t.Errorf("Prefix(%v) = %q, %v; want \"ex\", true", ex, prefix, ok)
		}
	})

	cfg.run(t, "reports unknown lookups as missing", func(t *testing.T) {
		s := cfg.New(t)
		if _, ok := s.Namespace("nope"); ok {
			t.Error("Namespace of an unbound prefix reported ok")
		}
		if _, ok := s.Prefix(other); ok {
			t.Error("Prefix of an unbound namespace reported ok")
		}
	})

	cfg.run(t, "rebinding a prefix overwrites it", func(t *testing.T) {
		s := cfg.New(t)
		s.Bind("ex", ex)
		s.Bind("ex", other)
		got, ok := s.Namespace("ex")
		if !ok || got.Value() != other.Value() {
			t.Errorf("Namespace(ex) = %v, %v; want %v, true", got, ok, other)
		}
	})

	cfg.run(t, "iterates every binding", func(t *testing.T) {
		s := cfg.New(t)
		s.Bind("ex", ex)
		s.Bind("other", other)

		got := map[string]string{}
		s.Namespaces()(func(p string, ns term.URIRef) bool {
			got[p] = ns.Value()
			return true
		})
		if got["ex"] != ex.Value() || got["other"] != other.Value() {
			t.Errorf("Namespaces() = %v, want both bindings", got)
		}
	})

	cfg.run(t, "stops when the consumer stops", func(t *testing.T) {
		s := cfg.New(t)
		s.Bind("a", term.NewURIRefUnsafe("http://a.example/"))
		s.Bind("b", term.NewURIRefUnsafe("http://b.example/"))
		s.Bind("c", term.NewURIRefUnsafe("http://c.example/"))

		seen := 0
		s.Namespaces()(func(string, term.URIRef) bool {
			seen++
			return false
		})
		if seen != 1 {
			t.Errorf("iterator yielded %d bindings after the consumer returned false, want 1", seen)
		}
	})
}

// --- context conventions ---------------------------------------------------

// testContextConventions pins the rules that are convention rather than
// interface, and that a new backend therefore gets wrong by default:
//
//   - a nil context means the default graph, not "every graph";
//   - store.DefaultGraph means the default graph too — it is the identifier an
//     unnamed Graph passes through to the store;
//   - a blank node is an ordinary graph name. TriG writes `_:g { … }`, and
//     folding blank-node contexts into the default graph lost those graphs
//     (rdflib #2445).
func testContextConventions(t *testing.T, cfg Config) {
	cfg.run(t, "nil context means the default graph", func(t *testing.T) {
		s := cfg.New(t)
		if !s.ContextAware() {
			t.Skip("backend is not context aware")
		}
		s.Add(triple(Alice, Name, lit("Alice")), nil)
		s.Add(triple(Bob, Name, lit("Bob")), Graph1)
		s.Add(triple(Carol, Name, lit("Carol")), Graph2)

		wantLen(t, s, nil, 1, "Len(nil) counts only the default graph")
		wantTriples(t, s, pattern(nil, nil, nil), nil, 1, "Triples(nil) reads only the default graph")
	})

	cfg.run(t, "the DefaultGraph identifier is the default graph", func(t *testing.T) {
		s := cfg.New(t)
		s.Add(triple(Alice, Name, lit("Alice")), store.DefaultGraph)
		s.Add(triple(Bob, Name, lit("Bob")), nil)

		wantLen(t, s, nil, 2, "Len(nil) after adds under nil and DefaultGraph")
		wantLen(t, s, store.DefaultGraph, 2, "Len(DefaultGraph)")
		wantTriples(t, s, pattern(Alice, nil, nil), nil, 1, "reading a DefaultGraph triple through nil")
		wantTriples(t, s, pattern(Bob, nil, nil), store.DefaultGraph, 1, "reading a nil-context triple through DefaultGraph")

		if s.ContextAware() {
			seen := 0
			s.Contexts(nil)(func(term.Term) bool {
				seen++
				return true
			})
			if seen != 0 {
				t.Errorf("Contexts reported %d contexts; DefaultGraph is not a named graph", seen)
			}
		}

		s.Remove(pattern(Alice, nil, nil), store.DefaultGraph)
		wantLen(t, s, nil, 1, "Remove under DefaultGraph deletes from the default graph")
		s.Set(triple(Bob, Name, lit("Robert")), store.DefaultGraph)
		wantTriples(t, s, pattern(Bob, pred(Name), lit("Robert")), nil, 1, "Set under DefaultGraph")
		wantLen(t, s, nil, 1, "Set under DefaultGraph replaced the value")
	})

	cfg.run(t, "a BNode context is a named graph", func(t *testing.T) {
		s := cfg.New(t)
		if !s.ContextAware() {
			t.Skip("backend is not context aware")
		}
		bn := term.NewBNode()
		other := term.NewBNode()

		s.Add(triple(Alice, Name, lit("Alice")), bn)
		s.Add(triple(Bob, Name, lit("Bob")), nil)
		s.AddN([]term.Quad{{Triple: triple(Carol, Name, lit("Carol")), Graph: other}})

		wantLen(t, s, bn, 1, "the blank-node graph")
		wantLen(t, s, other, 1, "a second blank-node graph")
		wantLen(t, s, nil, 1, "the default graph must not see blank-node graphs")
		wantTriples(t, s, pattern(nil, nil, nil), bn, 1, "reading back through the same BNode")
		wantTriples(t, s, pattern(Alice, nil, nil), other, 0, "a different BNode is a different graph")
		wantTriples(t, s, pattern(Alice, nil, nil), nil, 0, "the default graph")

		var names []string
		s.Contexts(nil)(func(c term.Term) bool {
			names = append(names, c.N3())
			return true
		})
		sort.Strings(names)
		want := []string{bn.N3(), other.N3()}
		sort.Strings(want)
		if len(names) != 2 || names[0] != want[0] || names[1] != want[1] {
			t.Errorf("Contexts = %v, want the two blank-node graphs %v", names, want)
		}

		s.Remove(pattern(nil, nil, nil), bn)
		wantLen(t, s, bn, 0, "the blank-node graph after Remove")
		wantLen(t, s, other, 1, "Remove in one blank-node graph must not touch another")
		wantLen(t, s, nil, 1, "Remove in a blank-node graph must not touch the default graph")
	})
}

// --- term round-trip -------------------------------------------------------

// testTermRoundTrip stores one triple per term shape and reads it back through
// a fully bound pattern. A backend that mangles a datatype, drops a language
// tag, or loses a base direction fails here rather than three layers up in a
// parser conformance run.
func testTermRoundTrip(t *testing.T, cfg Config) {
	xsdInt := term.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#integer")

	cases := []struct {
		name string
		tr   term.Triple
	}{
		{"plain literal", triple(Alice, Label, lit("plain"))},
		{"empty literal", triple(Alice, Label, lit(""))},
		{"literal with language", triple(Alice, Label, lit("bonjour", term.WithLang("fr")))},
		{"literal with language and direction", triple(Alice, Label, lit("שלום", term.WithLang("he"), term.WithDir("rtl")))},
		{"literal with datatype", triple(Alice, Label, lit("42", term.WithDatatype(xsdInt)))},
		{"integer literal", triple(Alice, Age, lit(42))},
		{"negative integer literal", triple(Alice, Age, lit(-7))},
		{"float literal", triple(Alice, Age, lit(3.5))},
		{"boolean literal", triple(Alice, Label, lit(true))},
		{"literal with a newline", triple(Alice, Label, lit("line one\nline two"))},
		{"literal with a quote", triple(Alice, Label, lit(`he said "hi"`))},
		{"literal with a backslash", triple(Alice, Label, lit(`C:\path`))},
		{"unicode literal", triple(Alice, Label, lit("日本語 🎉"))},
		{"URIRef object", triple(Alice, Knows, Bob)},
		{"URIRef with a fragment", triple(Alice, Knows, term.NewURIRefUnsafe("http://example.org/x#frag"))},
		{"BNode subject", triple(term.NewBNode("b1"), Name, lit("anon"))},
		{"BNode object", triple(Alice, Knows, term.NewBNode("b2"))},
		{"triple term object", triple(Alice, Label, term.NewTripleTerm(Bob, Name, lit("Bob")))},
		{"triple term subject", triple(term.NewTripleTerm(Bob, Name, lit("Bob")), Label, lit("a claim"))},
	}

	for _, tc := range cases {
		cfg.run(t, tc.name, func(t *testing.T) {
			s := cfg.New(t)
			s.Add(tc.tr, nil)
			wantLen(t, s, nil, 1, tc.name)

			// Exact lookup: the strongest check, since it requires the stored
			// key to match the key computed from the original term.
			p := tc.tr.Predicate
			exact := pattern(tc.tr.Subject, &p, tc.tr.Object)
			if count(s.Triples(exact, nil)) != 1 {
				t.Fatalf("%s: exact lookup found nothing", tc.name)
			}

			got := collect(s.Triples(pattern(nil, nil, nil), nil))
			want := fmt.Sprintf("%s %s %s", tc.tr.Subject.N3(), tc.tr.Predicate.N3(), tc.tr.Object.N3())
			if len(got) != 1 || got[0] != want {
				t.Errorf("%s: round-tripped as %v, want [%s]", tc.name, got, want)
			}
		})
	}

	cfg.run(t, "terms that differ only by a tag are distinct", func(t *testing.T) {
		s := cfg.New(t)
		s.Add(triple(Alice, Label, lit("x")), nil)
		s.Add(triple(Alice, Label, lit("x", term.WithLang("en"))), nil)
		s.Add(triple(Alice, Label, lit("x", term.WithLang("fr"))), nil)
		s.Add(triple(Alice, Label, lit("x", term.WithDatatype(xsdInt))), nil)
		wantLen(t, s, nil, 4, "four literals with the same lexical form")
	})
}

// --- empty store -----------------------------------------------------------

func testEmptyStore(t *testing.T, cfg Config) {
	s := cfg.New(t)

	wantLen(t, s, nil, 0, "empty store")
	wantTriples(t, s, pattern(nil, nil, nil), nil, 0, "empty store wildcard")
	wantTriples(t, s, pattern(Alice, pred(Name), lit("Alice")), nil, 0, "empty store exact")

	// Removing from an empty store must not panic or corrupt anything.
	s.Remove(pattern(nil, nil, nil), nil)
	s.Remove(pattern(Alice, nil, nil), Graph1)
	wantLen(t, s, nil, 0, "after removing from an empty store")

	if _, ok := s.Namespace("nope"); ok {
		t.Error("empty store returned a namespace binding")
	}
}

// --- concurrency -----------------------------------------------------------

func testConcurrency(t *testing.T, cfg Config) {
	cfg.run(t, "concurrent writes", func(t *testing.T) {
		s := cfg.New(t)
		const writers, each = 8, 50

		var wg sync.WaitGroup
		for w := 0; w < writers; w++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				for i := 0; i < each; i++ {
					subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/w%d-%d", w, i))
					s.Add(triple(subj, Name, lit(i)), nil)
				}
			}(w)
		}
		wg.Wait()

		wantLen(t, s, nil, writers*each, "after concurrent writes")
	})

	cfg.run(t, "concurrent reads and writes", func(t *testing.T) {
		s := cfg.New(t)
		s.Add(triple(Alice, Name, lit("Alice")), nil)

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/r%d", i))
				s.Add(triple(subj, Name, lit(i)), nil)
			}
		}()
		go func() {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				count(s.Triples(pattern(nil, pred(Name), nil), nil))
				s.Len(nil)
			}
		}()
		wg.Wait()

		wantLen(t, s, nil, 201, "after concurrent reads and writes")
	})
}

// --- persistence -----------------------------------------------------------

func testPersistence(t *testing.T, cfg Config) {
	s := cfg.New(t)

	s.Add(triple(Alice, Name, lit("Alice")), nil)
	s.Add(triple(Alice, Age, lit(30)), nil)
	s.Add(triple(Bob, Knows, Alice), Graph1)
	bn := term.NewBNode("persistedgraph")
	if s.ContextAware() {
		s.Add(triple(Carol, Knows, Alice), bn)
	}
	s.Bind("ex", term.NewURIRefUnsafe("http://example.org/"))

	s2 := cfg.Reopen(t, s)

	wantLen(t, s2, nil, 2, "default graph after reopen")
	wantTriples(t, s2, pattern(Alice, pred(Name), lit("Alice")), nil, 1, "exact lookup after reopen")

	if s2.ContextAware() {
		wantLen(t, s2, Graph1, 1, "named graph after reopen")
		wantLen(t, s2, bn, 1, "blank-node graph after reopen")
		found := false
		s2.Contexts(nil)(func(c term.Term) bool {
			found = found || c.N3() == bn.N3()
			return true
		})
		if !found {
			t.Error("Contexts after reopen does not report the blank-node graph")
		}
	}

	ns, ok := s2.Namespace("ex")
	if !ok || ns.Value() != "http://example.org/" {
		t.Errorf("namespace binding did not survive reopen: %v, %v", ns, ok)
	}
}
