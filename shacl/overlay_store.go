package shacl

import (
	"context"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// disjointUnion is the SPARQL store of an overlay whose base store reports
// cardinalities. rdfOverlay fills its over store with only the added triples
// the base does not hold, and neither layer changes while the overlay is in use
// (see Overlay), so no triple is in both: the union's answers are the sum of
// the layers', with no dedup to pay per read. That is what lets it promise
// CardinalityStore, which store.UnionStore cannot in general. The SPARQL
// planner orders a BGP by those counts; without them it probes each pattern.
type disjointUnion struct {
	*store.UnionStore
	base, over store.Store
	baseCard   store.CardinalityStore
	overCard   store.CardinalityStore
	// empty: the over store holds nothing (every added triple was in the
	// base), so every read is the base's own.
	empty bool
}

// newOverlayStore returns the read-only store an overlay's SPARQL constraints
// query: base plus over, where over holds no triple of base. empty says that
// over holds nothing at all.
func newOverlayStore(base, over store.Store, empty bool) store.Store {
	u := store.NewUnion(base, over)
	bc, ok := base.(store.CardinalityStore)
	if !ok {
		return u
	}
	oc, ok := over.(store.CardinalityStore)
	if !ok {
		return u
	}
	return &disjointUnion{UnionStore: u, base: base, over: over, baseCard: bc, overCard: oc, empty: empty}
}

// Triples yields the base's matches, then the over store's.
func (d *disjointUnion) Triples(ctx context.Context, pattern term.TriplePattern, graph term.Term) store.TripleIterator {
	if err := ctx.Err(); err != nil {
		return store.ErrorIterator[term.Triple](err)
	}
	if d.empty {
		return d.base.Triples(ctx, pattern, graph)
	}
	return func(yield func(term.Triple, error) bool) {
		for _, layer := range [2]store.Store{d.base, d.over} {
			for t, err := range layer.Triples(ctx, pattern, graph) {
				if !yield(t, err) || err != nil {
					return
				}
			}
		}
	}
}

// Len is the sum of the layers' sizes.
func (d *disjointUnion) Len(ctx context.Context, graph term.Term) (int, error) {
	b, err := d.base.Len(ctx, graph)
	if err != nil {
		return 0, err
	}
	o, err := d.over.Len(ctx, graph)
	return b + o, err
}

// Cardinality implements store.CardinalityStore: the sum of the layers'.
func (d *disjointUnion) Cardinality(ctx context.Context, pattern term.TriplePattern, graph term.Term) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	b, err := d.baseCard.Cardinality(ctx, pattern, graph)
	if err != nil || d.empty {
		return b, err
	}
	o, err := d.overCard.Cardinality(ctx, pattern, graph)
	return b + o, err
}

// Count is Cardinality: both layers count from their indexes.
func (d *disjointUnion) Count(ctx context.Context, pattern term.TriplePattern, graph term.Term) (int, error) {
	return d.Cardinality(ctx, pattern, graph)
}

// TriplesWithLimit applies offset and limit to Triples.
func (d *disjointUnion) TriplesWithLimit(ctx context.Context, pattern term.TriplePattern, graph term.Term, limit, offset int) store.TripleIterator {
	if err := ctx.Err(); err != nil {
		return store.ErrorIterator[term.Triple](err)
	}
	if qs, ok := d.base.(store.QueryableStore); ok && d.empty {
		return qs.TriplesWithLimit(ctx, pattern, graph, limit, offset)
	}
	return func(yield func(term.Triple, error) bool) {
		skipped, yielded := 0, 0
		for t, err := range d.Triples(ctx, pattern, graph) {
			if err != nil {
				yield(term.Triple{}, err)
				return
			}
			if skipped < offset {
				skipped++
				continue
			}
			if limit > 0 && yielded >= limit {
				return
			}
			yielded++
			if !yield(t, nil) {
				return
			}
		}
	}
}

// Exists is the base's own when the over store is empty.
func (d *disjointUnion) Exists(ctx context.Context, pattern term.TriplePattern, graph term.Term) (bool, error) {
	if qs, ok := d.base.(store.QueryableStore); ok && d.empty {
		return qs.Exists(ctx, pattern, graph)
	}
	return d.UnionStore.Exists(ctx, pattern, graph)
}

// Reachable is the base's own when the over store is empty: no edge of the
// over store can change the answer.
func (d *disjointUnion) Reachable(ctx context.Context, q store.ReachabilityQuery) ([]term.Term, error) {
	if rs, ok := d.base.(store.ReachabilityStore); ok && d.empty {
		return rs.Reachable(ctx, q)
	}
	return d.UnionStore.Reachable(ctx, q)
}

var (
	_ store.CardinalityStore  = (*disjointUnion)(nil)
	_ store.QueryableStore    = (*disjointUnion)(nil)
	_ store.ReachabilityStore = (*disjointUnion)(nil)
)
