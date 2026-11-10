package shacl

import (
	"context"
	"sync"
	"testing"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/storetest"
	"github.com/tggo/goRDFlib/term"
)

// disjointHarness runs the store conformance suite against disjointUnion. The
// suite writes through Add, AddN, Remove, Set and Bind, which the union
// refuses, so the harness routes them to the two layers, alternating between
// them, and never puts a triple in both: that is the invariant rdfOverlay
// guarantees. Every read is the union's.
type disjointHarness struct {
	*disjointUnion
	// mu makes each write's check-then-add atomic, so that concurrent writers
	// of one triple cannot put it in both layers.
	mu sync.Mutex
	n  int
}

func newDisjointHarness() *disjointHarness {
	u := newOverlayStore(store.NewMemoryStore(), store.NewMemoryStore(), false).(*disjointUnion)
	return &disjointHarness{disjointUnion: u}
}

func (h *disjointHarness) Add(ctx context.Context, t term.Triple, g term.Term) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.add(ctx, t, g)
}

func (h *disjointHarness) add(ctx context.Context, t term.Triple, g term.Term) error {
	for _, l := range [2]store.Store{h.base, h.over} {
		held, err := holds(ctx, l, t, g)
		if err != nil || held {
			return err
		}
	}
	h.n++
	if h.n%2 == 0 {
		return h.base.Add(ctx, t, g)
	}
	return h.over.Add(ctx, t, g)
}

func (h *disjointHarness) AddN(ctx context.Context, qs []term.Quad) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, q := range qs {
		if err := h.add(ctx, q.Triple, q.Graph); err != nil {
			return err
		}
	}
	return nil
}

func (h *disjointHarness) Remove(ctx context.Context, p term.TriplePattern, g term.Term) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.remove(ctx, p, g)
}

func (h *disjointHarness) remove(ctx context.Context, p term.TriplePattern, g term.Term) error {
	for _, l := range [2]store.Store{h.base, h.over} {
		if err := l.Remove(ctx, p, g); err != nil {
			return err
		}
	}
	return nil
}

func (h *disjointHarness) Set(ctx context.Context, t term.Triple, g term.Term) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	pred := t.Predicate
	if err := h.remove(ctx, term.TriplePattern{Subject: t.Subject, Predicate: &pred}, g); err != nil {
		return err
	}
	return h.add(ctx, t, g)
}

func (h *disjointHarness) Bind(ctx context.Context, prefix string, ns term.URIRef) error {
	return h.base.Bind(ctx, prefix, ns)
}

func TestOverlayStoreConformance(t *testing.T) {
	storetest.Run(t, storetest.Config{
		New: func(t *testing.T) store.Store { return newDisjointHarness() },
	})
}

// With nothing new in the over store, the overlay store reads as its base,
// and still refuses writes.
func TestOverlayStoreEmptyOver(t *testing.T) {
	storetest.Run(t, storetest.Config{
		New: func(t *testing.T) store.Store { return newEmptyOverHarness() },
	})
}

// emptyOverHarness writes to the base only, and reads through an overlay
// store built with empty set, as rdfOverlay builds it when every added triple
// is already in the base.
type emptyOverHarness struct {
	*disjointUnion
}

func newEmptyOverHarness() *emptyOverHarness {
	u := newOverlayStore(store.NewMemoryStore(), store.NewMemoryStore(), true).(*disjointUnion)
	return &emptyOverHarness{disjointUnion: u}
}

func (h *emptyOverHarness) Add(ctx context.Context, t term.Triple, g term.Term) error {
	return h.base.Add(ctx, t, g)
}
func (h *emptyOverHarness) AddN(ctx context.Context, qs []term.Quad) error {
	return h.base.AddN(ctx, qs)
}
func (h *emptyOverHarness) Remove(ctx context.Context, p term.TriplePattern, g term.Term) error {
	return h.base.Remove(ctx, p, g)
}
func (h *emptyOverHarness) Set(ctx context.Context, t term.Triple, g term.Term) error {
	return h.base.Set(ctx, t, g)
}
func (h *emptyOverHarness) Bind(ctx context.Context, prefix string, ns term.URIRef) error {
	return h.base.Bind(ctx, prefix, ns)
}
