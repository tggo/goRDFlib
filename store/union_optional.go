package store

import (
	"context"

	"github.com/tggo/goRDFlib/term"
)

// The optional interfaces a UnionStore can keep. Each one answers for the set
// of triples Triples yields, dedup included: an over triple the base also holds
// is counted once, as it is yielded once. The over store is small (see
// UnionStore), so the part of each answer that comes from it costs one bound
// lookup in the base per over triple matched, whatever the size of the base.

// fresh counts the over store's matches of pattern that the base does not
// hold: the triples the union adds to the base's answer.
func (u *UnionStore) fresh(ctx context.Context, pattern term.TriplePattern, graph term.Term) (int, error) {
	n := 0
	for t, err := range u.over.Triples(ctx, pattern, graph) {
		if err != nil {
			return 0, err
		}
		dup, err := u.held(ctx, t, graph)
		if err != nil {
			return 0, err
		}
		if !dup {
			n++
		}
	}
	return n, nil
}

// TriplesWithLimit implements QueryableStore: Triples, with offset and limit
// applied to the union's own sequence. The base's part is the base's own
// TriplesWithLimit when it has one and the window lies in it.
func (u *UnionStore) TriplesWithLimit(ctx context.Context, pattern term.TriplePattern, graph term.Term, limit, offset int) TripleIterator {
	if err := ctx.Err(); err != nil {
		return ErrorIterator[term.Triple](err)
	}
	if offset < 0 {
		offset = 0
	}
	return func(yield func(term.Triple, error) bool) {
		skipped, yielded := 0, 0
		// take reports whether t is past the offset, and false once the limit
		// is reached or the consumer stops (done).
		done := false
		take := func(t term.Triple) bool {
			if skipped < offset {
				skipped++
				return true
			}
			if limit > 0 && yielded >= limit {
				done = true
				return false
			}
			yielded++
			if !yield(t, nil) {
				done = true
				return false
			}
			return true
		}
		base := u.base.Triples(ctx, pattern, graph)
		if qs, ok := u.base.(QueryableStore); ok && limit > 0 {
			// the base yields at most offset+limit triples of the window
			base = qs.TriplesWithLimit(ctx, pattern, graph, offset+limit, 0)
		}
		for t, err := range base {
			if err != nil {
				yield(term.Triple{}, err)
				return
			}
			if !take(t) {
				return
			}
		}
		if done || (limit > 0 && yielded >= limit) {
			return
		}
		for t, err := range u.over.Triples(ctx, pattern, graph) {
			if err != nil {
				yield(term.Triple{}, err)
				return
			}
			dup, err := u.held(ctx, t, graph)
			if err != nil {
				yield(term.Triple{}, err)
				return
			}
			if dup {
				continue
			}
			if !take(t) {
				return
			}
		}
	}
}

// Count implements QueryableStore. The base's part is the base's own Count
// when it has one.
func (u *UnionStore) Count(ctx context.Context, pattern term.TriplePattern, graph term.Term) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	var n int
	if qs, ok := u.base.(QueryableStore); ok {
		c, err := qs.Count(ctx, pattern, graph)
		if err != nil {
			return 0, err
		}
		n = c
	} else {
		for _, err := range u.base.Triples(ctx, pattern, graph) {
			if err != nil {
				return 0, err
			}
			n++
		}
	}
	f, err := u.fresh(ctx, pattern, graph)
	return n + f, err
}

// Exists implements QueryableStore. Dedup does not change whether a match
// exists, so either layer answers.
func (u *UnionStore) Exists(ctx context.Context, pattern term.TriplePattern, graph term.Term) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	for _, layer := range [2]Store{u.base, u.over} {
		if qs, ok := layer.(QueryableStore); ok {
			found, err := qs.Exists(ctx, pattern, graph)
			if err != nil || found {
				return found, err
			}
			continue
		}
		for _, err := range layer.Triples(ctx, pattern, graph) {
			return err == nil, err
		}
	}
	return false, nil
}

// Reachable implements ReachabilityStore by handing the query to the base when
// the over store holds no edge the traversal could take: the union's edges are
// then the base's, and its answer is the union's. Otherwise, or when the base
// is not a ReachabilityStore, it declines, and the caller traverses the union
// itself.
func (u *UnionStore) Reachable(ctx context.Context, q ReachabilityQuery) ([]term.Term, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rs, ok := u.base.(ReachabilityStore)
	if !ok {
		return nil, ErrReachabilityUnsupported
	}
	edge, err := u.overHasEdge(ctx, q)
	if err != nil {
		return nil, err
	}
	if edge {
		return nil, ErrReachabilityUnsupported
	}
	return rs.Reachable(ctx, q)
}

// overHasEdge reports whether the over store holds a triple that q could
// traverse, in either direction.
func (u *UnionStore) overHasEdge(ctx context.Context, q ReachabilityQuery) (bool, error) {
	if len(q.Predicates) > 0 && !q.Negated {
		for _, p := range q.Predicates {
			for _, err := range u.over.Triples(ctx, term.TriplePattern{Predicate: &p}, q.Context) {
				return err == nil, err
			}
		}
		return false, nil
	}
	excluded := make(map[string]bool, len(q.Predicates))
	for _, p := range q.Predicates {
		excluded[term.TermKey(p)] = true
	}
	for t, err := range u.over.Triples(ctx, term.TriplePattern{}, q.Context) {
		if err != nil {
			return false, err
		}
		if !excluded[term.TermKey(t.Predicate)] {
			return true, nil
		}
	}
	return false, nil
}

var (
	_ QueryableStore    = (*UnionStore)(nil)
	_ ReachabilityStore = (*UnionStore)(nil)
)
