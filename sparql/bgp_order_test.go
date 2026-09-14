package sparql

import (
	"fmt"
	"slices"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

const ob = "http://example.org/"

func obIRI(local string) string { return "<" + ob + local + ">" }

// orderTestGraph has deliberately lopsided predicate counts so that the
// written order and the selective order differ: 2000 labels, 1200 performance
// actors, 1200 starring edges, 400 typed films, and a single director with 4
// films.
func orderTestGraph(t testing.TB, opts ...graph.GraphOption) *rdflibgo.Graph {
	t.Helper()
	g := rdflibgo.NewGraph(opts...)
	u := func(format string, a ...any) rdflibgo.URIRef {
		return rdflibgo.NewURIRefUnsafe(ob + fmt.Sprintf(format, a...))
	}
	label, actorP, starring, directedBy, genre, rating, filmType :=
		u("label"), u("actor"), u("starring"), u("directedBy"), u("genre"), u("rating"), u("type")
	for i := range 2000 {
		g.Add(u("thing/%d", i), label, rdflibgo.NewLiteral(fmt.Sprintf("thing %d", i)))
	}
	for f := range 400 {
		film := u("film/%d", f)
		if f < 4 {
			g.Add(film, directedBy, u("director/0"))
		} else {
			g.Add(film, directedBy, u("director/%d", 1+f%50))
		}
		g.Add(film, genre, u("genre/%d", f%7))
		g.Add(film, filmType, u("Film"))
		if f%3 == 0 {
			g.Add(film, rating, rdflibgo.NewLiteral(f%10))
		}
		for j := range 3 {
			perf := u("perf/%d/%d", f, j)
			g.Add(film, starring, perf)
			g.Add(perf, actorP, u("thing/%d", (f*7+j)%2000))
		}
	}
	return g
}

// probingStore hides every optional store interface, so the planner has to
// probe through Triples, and counts the triples those probes pull.
type probingStore struct {
	store.Store
	pulled int
}

func (s *probingStore) Triples(p term.TriplePattern, ctx term.Term) store.TripleIterator {
	return func(yield func(term.Triple) bool) {
		s.Store.Triples(p, ctx)(func(t term.Triple) bool {
			s.pulled++
			return yield(t)
		})
	}
}

// plannerStores runs a planner test on both planning paths: exact counts from
// a CardinalityStore, and early-stopping probes.
var plannerStores = []struct {
	name  string
	graph func(t testing.TB) *rdflibgo.Graph
}{
	{"cardinality", func(t testing.TB) *rdflibgo.Graph { return orderTestGraph(t) }},
	{"probing", func(t testing.TB) *rdflibgo.Graph {
		return orderTestGraph(t, graph.WithStore(&probingStore{Store: store.NewMemoryStore()}))
	}},
}

func forEachPlannerStore(t *testing.T, test func(t *testing.T, g *rdflibgo.Graph)) {
	for _, ps := range plannerStores {
		t.Run(ps.name, func(t *testing.T) { test(t, ps.graph(t)) })
	}
}

func probingGraph(t testing.TB) *rdflibgo.Graph { return plannerStores[1].graph(t) }

var actorsByDirectorReversed = []Triple{
	{Subject: "?actor", Predicate: obIRI("label"), Object: "?name"},
	{Subject: "?perf", Predicate: obIRI("actor"), Object: "?actor"},
	{Subject: "?film", Predicate: obIRI("starring"), Object: "?perf"},
	{Subject: "?film", Predicate: obIRI("directedBy"), Object: obIRI("director/0")},
}

func TestOrderBGP_SelectiveChainFirst(t *testing.T) {
	forEachPlannerStore(t, func(t *testing.T, g *rdflibgo.Graph) {
		got := orderBGP(g, actorsByDirectorReversed, map[string]rdflibgo.Term{}, nil)
		want := []Triple{
			actorsByDirectorReversed[3],
			actorsByDirectorReversed[2],
			actorsByDirectorReversed[1],
			actorsByDirectorReversed[0],
		}
		if !slices.Equal(got, want) {
			t.Fatalf("order:\n got  %v\n want %v", got, want)
		}
	})
}

// A disconnected pattern must wait until the patterns connected to what is
// already bound are used up, even when it looks cheaper: running it in between
// multiplies every partial solution by its matches. Here the unrelated pattern
// is exactly counted (57) while the connected one is an unprobed property
// path, so every other rule prefers the unrelated pattern.
func TestOrderBGP_PostponesCrossProduct(t *testing.T) {
	forEachPlannerStore(t, testPostponesCrossProduct)
}

func testPostponesCrossProduct(t *testing.T, g *rdflibgo.Graph) {
	unrelated := Triple{Subject: "?x", Predicate: obIRI("genre"), Object: obIRI("genre/3")}
	chain := Triple{Subject: "?film", Predicate: obIRI("directedBy"), Object: obIRI("director/0")}
	connected := Triple{
		Subject:       "?film",
		PredicatePath: paths.OneOrMore(paths.AsPath(rdflibgo.NewURIRefUnsafe(ob + "starring"))),
		Object:        "?perf",
	}
	got := orderBGP(g, []Triple{unrelated, connected, chain}, map[string]rdflibgo.Term{}, nil)
	want := []Triple{chain, connected, unrelated}
	if !slices.Equal(got, want) {
		t.Fatalf("order:\n got  %v\n want %v", got, want)
	}
}

// Capped counts carry no ordering information between themselves. Without
// evening them out, the typed-films pattern (probed first, stopped at 64)
// would rank below starring (probed after the seed, stopped at 4) purely
// because of probe order; with it they tie and keep the written order.
func TestOrderBGP_CappedPatternsKeepWrittenOrder(t *testing.T) {
	g := probingGraph(t)
	typed := Triple{Subject: "?film", Predicate: obIRI("type"), Object: obIRI("Film")}
	starring := Triple{Subject: "?film", Predicate: obIRI("starring"), Object: "?perf"}
	seed := Triple{Subject: "?film", Predicate: obIRI("directedBy"), Object: obIRI("director/0")}
	got := orderBGP(g, []Triple{typed, starring, seed}, map[string]rdflibgo.Term{}, nil)
	if want := []Triple{seed, typed, starring}; !slices.Equal(got, want) {
		t.Fatalf("order:\n got  %v\n want %v", got, want)
	}
}

// When every pattern is too broad for the first-round probe limit, a second
// round with the full limit has to tell them apart: 400 genre edges must run
// before 2000 labels even though both exceed bgpProbeStart.
func TestOrderBGP_SecondRoundSeparatesBroadPatterns(t *testing.T) {
	g := probingGraph(t)
	labels := Triple{Subject: "?x", Predicate: obIRI("label"), Object: "?l"}
	genres := Triple{Subject: "?x", Predicate: obIRI("genre"), Object: "?g"}
	got := orderBGP(g, []Triple{labels, genres}, map[string]rdflibgo.Term{}, nil)
	if want := []Triple{genres, labels}; !slices.Equal(got, want) {
		t.Fatalf("order:\n got  %v\n want %v", got, want)
	}
}

// Probing is the planner's whole cost, so once the seed's 4 matches are known
// the other probes must stop at 4 too, not at the first-round limit of 64.
func TestOrderBGP_ProbesStopAtSmallestCount(t *testing.T) {
	st := &probingStore{Store: store.NewMemoryStore()}
	g := orderTestGraph(t, graph.WithStore(st))
	st.pulled = 0
	orderBGP(g, actorsByDirectorReversed, map[string]rdflibgo.Term{}, nil)
	if want := 4 * 4; st.pulled > want {
		t.Fatalf("probes pulled %d triples, want at most %d", st.pulled, want)
	}
}

// cardinalityCountingStore is a MemoryStore, CardinalityStore included, that
// counts every triple read through it.
type cardinalityCountingStore struct {
	*store.MemoryStore
	pulled int
}

func (s *cardinalityCountingStore) Triples(p term.TriplePattern, ctx term.Term) store.TripleIterator {
	return s.counting(s.MemoryStore.Triples(p, ctx))
}

func (s *cardinalityCountingStore) TriplesWithLimit(p term.TriplePattern, ctx term.Term, limit, offset int) store.TripleIterator {
	return s.counting(s.MemoryStore.TriplesWithLimit(p, ctx, limit, offset))
}

func (s *cardinalityCountingStore) counting(it store.TripleIterator) store.TripleIterator {
	return func(yield func(term.Triple) bool) {
		it(func(t term.Triple) bool {
			s.pulled++
			return yield(t)
		})
	}
}

// A store that can count from its indexes must not be probed at all: that is
// the whole point of CardinalityStore.
func TestOrderBGP_CardinalityStoreIsNotProbed(t *testing.T) {
	st := &cardinalityCountingStore{MemoryStore: store.NewMemoryStore()}
	g := orderTestGraph(t, graph.WithStore(st))
	st.pulled = 0
	got := orderBGP(g, actorsByDirectorReversed, map[string]rdflibgo.Term{}, nil)
	if st.pulled != 0 {
		t.Fatalf("planning pulled %d triples from a CardinalityStore, want 0", st.pulled)
	}
	if got[0] != actorsByDirectorReversed[3] {
		t.Fatalf("first pattern = %v, want the director pattern", got[0])
	}
}

// An unmatchable pattern empties the BGP, so it runs first even when it is not
// connected to anything.
func TestOrderBGP_EmptyPatternFirst(t *testing.T) {
	forEachPlannerStore(t, func(t *testing.T, g *rdflibgo.Graph) {
		empty := Triple{Subject: "?a", Predicate: obIRI("noSuchPredicate"), Object: "?b"}
		got := orderBGP(g, []Triple{actorsByDirectorReversed[0], actorsByDirectorReversed[1], empty}, map[string]rdflibgo.Term{}, nil)
		if got[0] != empty {
			t.Fatalf("first pattern = %v, want the empty one", got[0])
		}
	})
}

func TestOrderBGP_KeepsOrderWithVariableTripleTerm(t *testing.T) {
	g := orderTestGraph(t)
	in := []Triple{
		actorsByDirectorReversed[0],
		{Subject: "?r", Predicate: obIRI("reifies"), Object: "<<( ?s " + obIRI("p") + " ?o )>>"},
	}
	got := orderBGP(g, in, map[string]rdflibgo.Term{}, nil)
	if &got[0] != &in[0] {
		t.Fatal("BGP with a variable triple term was reordered")
	}
}

// Reordering must never change the solution multiset. Every permutation of
// each BGP, with and without pre-bound variables, is compared against nested
// loops in the written order.
func TestOrderBGP_SameSolutionsForEveryPermutation(t *testing.T) {
	forEachPlannerStore(t, testSameSolutionsForEveryPermutation)
}

func testSameSolutionsForEveryPermutation(t *testing.T, g *rdflibgo.Graph) {
	bgps := map[string][]Triple{
		"chain": actorsByDirectorReversed,
		"star": {
			{Subject: "?film", Predicate: obIRI("genre"), Object: obIRI("genre/3")},
			{Subject: "?film", Predicate: obIRI("rating"), Object: "?r"},
			{Subject: "?film", Predicate: obIRI("directedBy"), Object: "?d"},
			{Subject: "?film", Predicate: obIRI("starring"), Object: "?perf"},
		},
		"crossProduct": {
			{Subject: "?film", Predicate: obIRI("directedBy"), Object: obIRI("director/0")},
			{Subject: "?g", Predicate: obIRI("genre"), Object: obIRI("genre/5")},
			{Subject: "?film", Predicate: obIRI("genre"), Object: "?gg"},
		},
		"repeatedVariable": {
			{Subject: "?film", Predicate: obIRI("starring"), Object: "?perf"},
			{Subject: "?perf", Predicate: obIRI("actor"), Object: "?a"},
			{Subject: "?a", Predicate: obIRI("label"), Object: "?l"},
			{Subject: "?film", Predicate: "?p", Object: obIRI("director/0")},
		},
		"literalObject": {
			{Subject: "?film", Predicate: obIRI("rating"), Object: `"3"^^<http://www.w3.org/2001/XMLSchema#integer>`},
			{Subject: "?film", Predicate: obIRI("starring"), Object: "?perf"},
		},
	}
	film2 := rdflibgo.NewURIRefUnsafe(ob + "film/2")
	for name, bgp := range bgps {
		for _, pre := range []map[string]rdflibgo.Term{{}, {"film": film2}} {
			want := canonicalSolutions(evalBGPInOrder(g, bgp, pre, nil))
			if name != "literalObject" && len(pre) == 0 && len(want) == 0 {
				t.Fatalf("%s: fixture produces no solutions, the test would prove nothing", name)
			}
			for perm := range permutations(bgp) {
				got := canonicalSolutions(evalBGP(g, perm, pre, nil))
				if !slices.Equal(got, want) {
					t.Fatalf("%s (pre-bound %v), written %v:\n got %d solutions, want %d",
						name, pre, perm, len(got), len(want))
				}
			}
		}
	}
}

func canonicalSolutions(sols []map[string]rdflibgo.Term) []string {
	keys := make([]string, len(sols))
	for i, s := range sols {
		keys[i] = solutionKey(s, nil)
	}
	slices.Sort(keys)
	return keys
}

func permutations(in []Triple) func(yield func([]Triple) bool) {
	return func(yield func([]Triple) bool) {
		p := slices.Clone(in)
		var rec func(k int) bool
		rec = func(k int) bool {
			if k == len(p) {
				return yield(slices.Clone(p))
			}
			for i := k; i < len(p); i++ {
				p[k], p[i] = p[i], p[k]
				if !rec(k + 1) {
					return false
				}
				p[k], p[i] = p[i], p[k]
			}
			return true
		}
		rec(0)
	}
}
