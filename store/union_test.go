package store_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/storetest"
	"github.com/tggo/goRDFlib/term"
)

// splitUnion is the test harness that lets the shared conformance suite run
// against a UnionStore. The suite fills a store through Add/AddN/Set/Remove/
// Bind, which a UnionStore refuses by design, so the harness routes those
// writes to the two layers underneath it and leaves every read to the
// UnionStore itself. Successive writes go to the base only, the over store only
// and both (in that rotation), so the suite also exercises dedup between
// layers. Nothing in the harness reads: the reads under test are the union's.
type splitUnion struct {
	*store.UnionStore
	base, over store.Store
	n          atomic.Uint64
}

func newSplitUnion() *splitUnion {
	b, o := store.NewMemoryStore(), store.NewMemoryStore()
	return &splitUnion{UnionStore: store.NewUnion(b, o), base: b, over: o}
}

func (s *splitUnion) layers() []store.Store {
	switch s.n.Add(1) % 3 {
	case 0:
		return []store.Store{s.base}
	case 1:
		return []store.Store{s.over}
	}
	return []store.Store{s.base, s.over}
}

func (s *splitUnion) Add(ctx context.Context, t term.Triple, g term.Term) error {
	for _, l := range s.layers() {
		if err := l.Add(ctx, t, g); err != nil {
			return err
		}
	}
	return nil
}

func (s *splitUnion) AddN(ctx context.Context, qs []term.Quad) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, q := range qs {
		if err := s.Add(ctx, q.Triple, q.Graph); err != nil {
			return err
		}
	}
	return nil
}

func (s *splitUnion) Remove(ctx context.Context, p term.TriplePattern, g term.Term) error {
	for _, l := range []store.Store{s.base, s.over} {
		if err := l.Remove(ctx, p, g); err != nil {
			return err
		}
	}
	return nil
}

func (s *splitUnion) Set(ctx context.Context, t term.Triple, g term.Term) error {
	pred := t.Predicate
	if err := s.Remove(ctx, term.TriplePattern{Subject: t.Subject, Predicate: &pred}, g); err != nil {
		return err
	}
	return s.Add(ctx, t, g)
}

func (s *splitUnion) Bind(ctx context.Context, prefix string, ns term.URIRef) error {
	for _, l := range s.layers() {
		if err := l.Bind(ctx, prefix, ns); err != nil {
			return err
		}
	}
	return nil
}

// A union reads exactly as one store holding both layers. No Known entries:
// the writes the suite performs go through splitUnion, and the refusal of a
// UnionStore's own writes is TestUnionStoreRefusesWrites.
func TestUnionStoreConformance(t *testing.T) {
	storetest.Run(t, storetest.Config{
		New: func(t *testing.T) store.Store { return newSplitUnion() },
	})
}

func TestUnionStoreRefusesWrites(t *testing.T) {
	ctx := t.Context()
	base, over := store.NewMemoryStore(), store.NewMemoryStore()
	tr := term.Triple{Subject: storetest.Alice, Predicate: storetest.Knows, Object: storetest.Bob}
	if err := base.Add(ctx, tr, nil); err != nil {
		t.Fatal(err)
	}
	u := store.NewUnion(base, over)
	pred := storetest.Knows
	for what, err := range map[string]error{
		"Add":    u.Add(ctx, tr, nil),
		"AddN":   u.AddN(ctx, []term.Quad{{Triple: tr}}),
		"Remove": u.Remove(ctx, term.TriplePattern{Predicate: &pred}, nil),
		"Set":    u.Set(ctx, tr, nil),
		"Bind":   u.Bind(ctx, "ex", term.NewURIRefUnsafe("http://example.org/")),
	} {
		if !errors.Is(err, store.ErrReadOnly) {
			t.Errorf("%s: err = %v, want ErrReadOnly", what, err)
		}
	}
	if n, _ := base.Len(ctx, nil); n != 1 {
		t.Errorf("a refused write changed the base: Len = %d", n)
	}
	if n, _ := over.Len(ctx, nil); n != 0 {
		t.Errorf("a refused write changed the over store: Len = %d", n)
	}
}

// baseUnion is a union whose writes all go to its base, so the over store
// stays empty. A traversal can then take no edge of the over store, and the
// union hands every reachability query to its base: the reachability section
// of the suite runs in full instead of being declined.
type baseUnion struct {
	*store.UnionStore
	*store.MemoryStore
}

func newBaseUnion() *baseUnion {
	b := store.NewMemoryStore()
	return &baseUnion{UnionStore: store.NewUnion(b, store.NewMemoryStore()), MemoryStore: b}
}

func (s *baseUnion) Add(ctx context.Context, t term.Triple, g term.Term) error {
	return s.MemoryStore.Add(ctx, t, g)
}
func (s *baseUnion) AddN(ctx context.Context, qs []term.Quad) error {
	return s.MemoryStore.AddN(ctx, qs)
}
func (s *baseUnion) Remove(ctx context.Context, p term.TriplePattern, g term.Term) error {
	return s.MemoryStore.Remove(ctx, p, g)
}
func (s *baseUnion) Set(ctx context.Context, t term.Triple, g term.Term) error {
	return s.MemoryStore.Set(ctx, t, g)
}
func (s *baseUnion) Bind(ctx context.Context, prefix string, ns term.URIRef) error {
	return s.MemoryStore.Bind(ctx, prefix, ns)
}

// Every read goes to the union, not to the base underneath.
func (s *baseUnion) ContextAware() bool     { return s.UnionStore.ContextAware() }
func (s *baseUnion) TransactionAware() bool { return s.UnionStore.TransactionAware() }
func (s *baseUnion) Triples(ctx context.Context, p term.TriplePattern, g term.Term) store.TripleIterator {
	return s.UnionStore.Triples(ctx, p, g)
}
func (s *baseUnion) Len(ctx context.Context, g term.Term) (int, error) {
	return s.UnionStore.Len(ctx, g)
}
func (s *baseUnion) Contexts(ctx context.Context, t *term.Triple) store.TermIterator {
	return s.UnionStore.Contexts(ctx, t)
}
func (s *baseUnion) Namespace(ctx context.Context, prefix string) (term.URIRef, bool, error) {
	return s.UnionStore.Namespace(ctx, prefix)
}
func (s *baseUnion) Prefix(ctx context.Context, ns term.URIRef) (string, bool, error) {
	return s.UnionStore.Prefix(ctx, ns)
}
func (s *baseUnion) Namespaces(ctx context.Context) store.NamespaceIterator {
	return s.UnionStore.Namespaces(ctx)
}
func (s *baseUnion) TriplesWithLimit(ctx context.Context, p term.TriplePattern, g term.Term, limit, offset int) store.TripleIterator {
	return s.UnionStore.TriplesWithLimit(ctx, p, g, limit, offset)
}
func (s *baseUnion) Count(ctx context.Context, p term.TriplePattern, g term.Term) (int, error) {
	return s.UnionStore.Count(ctx, p, g)
}
func (s *baseUnion) Exists(ctx context.Context, p term.TriplePattern, g term.Term) (bool, error) {
	return s.UnionStore.Exists(ctx, p, g)
}
func (s *baseUnion) Reachable(ctx context.Context, q store.ReachabilityQuery) ([]term.Term, error) {
	return s.UnionStore.Reachable(ctx, q)
}

func TestUnionStoreConformanceEmptyOver(t *testing.T) {
	storetest.Run(t, storetest.Config{
		New: func(t *testing.T) store.Store { return newBaseUnion() },
	})
}

// A union hands a traversal to its base only when the over store holds no
// edge the traversal could take; otherwise it declines, and the caller walks
// the union itself.
func TestUnionStoreReachable(t *testing.T) {
	ctx := t.Context()
	knows, name := storetest.Knows, storetest.Name
	base := store.NewMemoryStore()
	for _, tr := range []term.Triple{
		{Subject: storetest.Alice, Predicate: knows, Object: storetest.Bob},
		{Subject: storetest.Bob, Predicate: knows, Object: storetest.Carol},
	} {
		if err := base.Add(ctx, tr, nil); err != nil {
			t.Fatal(err)
		}
	}
	unionWith := func(t *testing.T, over ...term.Triple) *store.UnionStore {
		o := store.NewMemoryStore()
		for _, tr := range over {
			if err := o.Add(ctx, tr, nil); err != nil {
				t.Fatal(err)
			}
		}
		return store.NewUnion(base, o)
	}
	daveName := term.Triple{Subject: storetest.Dave, Predicate: name, Object: term.NewLiteral("Dave")}
	carolKnowsDave := term.Triple{Subject: storetest.Carol, Predicate: knows, Object: storetest.Dave}
	for _, tc := range []struct {
		name string
		over []term.Triple
		q    store.ReachabilityQuery
		want int // nodes reached; -1: declined
	}{
		{"no edge of the predicate in the over store", []term.Triple{daveName},
			store.ReachabilityQuery{Start: storetest.Alice, Predicates: []term.URIRef{knows}}, 2},
		{"an edge of the predicate in the over store", []term.Triple{carolKnowsDave},
			store.ReachabilityQuery{Start: storetest.Alice, Predicates: []term.URIRef{knows}}, -1},
		{"an inverse traversal sees the same edges", []term.Triple{carolKnowsDave},
			store.ReachabilityQuery{Start: storetest.Dave, Predicates: []term.URIRef{knows}, Inverse: true}, -1},
		{"any predicate: every over triple is an edge", []term.Triple{daveName},
			store.ReachabilityQuery{Start: storetest.Alice}, -1},
		{"a negated set that excludes every over predicate", []term.Triple{daveName},
			store.ReachabilityQuery{Start: storetest.Alice, Predicates: []term.URIRef{name}, Negated: true}, 2},
		{"a negated set that lets an over predicate through", []term.Triple{daveName},
			store.ReachabilityQuery{Start: storetest.Alice, Predicates: []term.URIRef{knows}, Negated: true}, -1},
		{"an empty over store", nil,
			store.ReachabilityQuery{Start: storetest.Alice}, 2},
		{"an over edge the base also holds still declines", []term.Triple{{Subject: storetest.Alice, Predicate: knows, Object: storetest.Bob}},
			store.ReachabilityQuery{Start: storetest.Alice, Predicates: []term.URIRef{knows}}, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := unionWith(t, tc.over...).Reachable(ctx, tc.q)
			if tc.want < 0 {
				if !errors.Is(err, store.ErrReachabilityUnsupported) {
					t.Fatalf("err = %v, want ErrReachabilityUnsupported", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != tc.want {
				t.Fatalf("reached %d nodes (%v), want %d", len(got), got, tc.want)
			}
		})
	}

	t.Run("only the over edges of the queried graph count", func(t *testing.T) {
		g1, g2 := term.NewURIRefUnsafe("http://example.org/g1"), term.NewURIRefUnsafe("http://example.org/g2")
		b, o := store.NewMemoryStore(), store.NewMemoryStore()
		if err := b.Add(ctx, term.Triple{Subject: storetest.Alice, Predicate: knows, Object: storetest.Bob}, g1); err != nil {
			t.Fatal(err)
		}
		if err := o.Add(ctx, term.Triple{Subject: storetest.Bob, Predicate: knows, Object: storetest.Carol}, g2); err != nil {
			t.Fatal(err)
		}
		u := store.NewUnion(b, o)
		got, err := u.Reachable(ctx, store.ReachabilityQuery{Start: storetest.Alice, Predicates: []term.URIRef{knows}, Context: g1})
		if err != nil || len(got) != 1 {
			t.Fatalf("in g1: reached %v, err %v; want Bob alone, delegated", got, err)
		}
		if _, err := u.Reachable(ctx, store.ReachabilityQuery{Start: storetest.Alice, Predicates: []term.URIRef{knows}, Context: g2}); !errors.Is(err, store.ErrReachabilityUnsupported) {
			t.Fatalf("in g2: err = %v, want ErrReachabilityUnsupported", err)
		}
	})

	t.Run("a base that is not a ReachabilityStore", func(t *testing.T) {
		u := store.NewUnion(storetest.NewFailingStore(base), store.NewMemoryStore())
		if _, err := u.Reachable(ctx, store.ReachabilityQuery{Start: storetest.Alice}); !errors.Is(err, store.ErrReachabilityUnsupported) {
			t.Fatalf("err = %v, want ErrReachabilityUnsupported", err)
		}
	})
}
