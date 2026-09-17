package storetest

import (
	"context"
	"testing"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// testContextBinder checks store.ContextBinder: a bound view is the same data
// with a context attached, and it keeps the optional interfaces the engines
// look for.
func testContextBinder(t *testing.T, cfg Config) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "storetest")
	bind := func(s store.Store) store.Store { return s.(store.ContextBinder).BindContext(ctx) }

	cfg.run(t, "view reads the store's data", func(t *testing.T) {
		s := cfg.New(t)
		s.Add(triple(Alice, Knows, Bob), nil)
		s.Add(triple(Bob, Knows, Carol), Graph1)
		v := bind(s)
		for _, p := range []term.TriplePattern{
			pattern(nil, nil, nil),
			pattern(Alice, pred(Knows), nil),
			pattern(Dave, nil, nil),
		} {
			for _, g := range []term.Term{nil, Graph1} {
				if got, want := count(v.Triples(p, g)), count(s.Triples(p, g)); got != want {
					t.Errorf("pattern %v in %v: view has %d, store has %d", p, g, got, want)
				}
			}
		}
		if got, want := v.Len(Graph1), s.Len(Graph1); got != want {
			t.Errorf("Len(Graph1): view %d, store %d", got, want)
		}
	})

	cfg.run(t, "writes through the view reach the store", func(t *testing.T) {
		s := cfg.New(t)
		v := bind(s)
		v.Add(triple(Alice, Knows, Bob), nil)
		v.AddN([]term.Quad{{Triple: triple(Bob, Knows, Carol), Graph: Graph1}})
		if got := count(s.Triples(pattern(nil, nil, nil), nil)); got != 1 {
			t.Errorf("default graph after Add through the view: %d triples, want 1", got)
		}
		if got := count(s.Triples(pattern(nil, nil, nil), Graph1)); got != 1 {
			t.Errorf("Graph1 after AddN through the view: %d triples, want 1", got)
		}
		v.Remove(pattern(Alice, nil, nil), nil)
		if got := count(s.Triples(pattern(Alice, nil, nil), nil)); got != 0 {
			t.Errorf("after Remove through the view the store still has %d triples", got)
		}
		v.Bind("ex", term.NewURIRefUnsafe("http://example.org/ns#"))
		if ns, ok := s.Namespace("ex"); !ok || ns.Value() != "http://example.org/ns#" {
			t.Errorf("a prefix bound through the view is not seen by the store: %v, %v", ns, ok)
		}
	})

	cfg.run(t, "view keeps the optional interfaces", func(t *testing.T) {
		s := cfg.New(t)
		v := bind(s)
		check := func(name string, onStore, onView bool) {
			if onStore && !onView {
				t.Errorf("the store implements %s and its bound view does not; engines lose that path", name)
			}
		}
		_, a := s.(store.QueryableStore)
		_, b := v.(store.QueryableStore)
		check("QueryableStore", a, b)
		_, a = s.(store.SnapshotStore)
		_, b = v.(store.SnapshotStore)
		check("SnapshotStore", a, b)
		_, a = s.(store.CardinalityStore)
		_, b = v.(store.CardinalityStore)
		check("CardinalityStore", a, b)
		_, a = s.(store.ReachabilityStore)
		_, b = v.(store.ReachabilityStore)
		check("ReachabilityStore", a, b)
		_, a = s.(store.ContextBinder)
		_, b = v.(store.ContextBinder)
		check("ContextBinder", a, b)
	})
}
