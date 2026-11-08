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
