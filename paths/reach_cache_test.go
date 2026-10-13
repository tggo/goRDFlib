package paths_test

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// classGraph types nItems items with 20 classes in a subClassOf chain:
// C1 -> C0, C2 -> C1, ...
func classGraph(t *testing.T, s store.Store, nItems int) *graph.Graph {
	t.Helper()
	g := graph.NewGraph(graph.WithStore(s))
	for i := 1; i < 20; i++ {
		if err := g.Add(t.Context(), uri(fmt.Sprintf("C%d", i)), uri("sub"), uri(fmt.Sprintf("C%d", i-1))); err != nil {
			t.Fatal(err)
		}
	}
	for i := range nItems {
		if err := g.Add(t.Context(), uri(fmt.Sprintf("item%d", i)), uri("type"), uri(fmt.Sprintf("C%d", i%20))); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

// tggo/rdflibgo-mongostore#3: `type/sub*` reaches the repetition once per
// item, but there are only 20 distinct classes to start from, so the store is
// asked 20 times, not 340.
func TestReachableAskedOncePerStartNode(t *testing.T) {
	cs := &countingStore{MemoryStore: store.NewMemoryStore()}
	g := classGraph(t, cs, 340)
	p := paths.Sequence(paths.AsPath(uri("type")), paths.ZeroOrMore(paths.AsPath(uri("sub"))))

	rows := 0
	for _, err := range p.Eval(t.Context(), g, nil, nil) {
		if err != nil {
			t.Fatal(err)
		}
		rows++
	}
	// item i of class C(i%20) reaches C(i%20) .. C0: sum over 340 items.
	want := 0
	for i := range 340 {
		want += i%20 + 1
	}
	if rows != want {
		t.Fatalf("got %d rows, want %d", rows, want)
	}
	if cs.calls != 20 {
		t.Errorf("Reachable called %d times, want 20 (one per class)", cs.calls)
	}
}

// Under WithReachabilityCache separate Eval calls share the answers; without
// it each Eval asks again, so a write between two evaluations is seen.
func TestReachabilityCacheScope(t *testing.T) {
	cs := &countingStore{MemoryStore: store.NewMemoryStore()}
	g := classGraph(t, cs, 0)
	p := paths.ZeroOrMore(paths.AsPath(uri("sub")))
	ctx := paths.WithReachabilityCache(t.Context())
	for range 3 {
		for _, err := range p.Eval(ctx, g, uri("C5"), nil) {
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	if cs.calls != 1 {
		t.Errorf("shared cache: Reachable called %d times, want 1", cs.calls)
	}

	cs.calls = 0
	if err := g.Add(t.Context(), uri("C0"), uri("sub"), uri("Top")); err != nil {
		t.Fatal(err)
	}
	found := false
	for pair, err := range p.Eval(t.Context(), g, uri("C5"), nil) {
		if err != nil {
			t.Fatal(err)
		}
		found = found || term.TermKey(pair.Second) == term.TermKey(uri("Top"))
	}
	if cs.calls != 1 || !found {
		t.Errorf("without a shared cache: %d calls, Top reached %v; want 1 and true", cs.calls, found)
	}
}
