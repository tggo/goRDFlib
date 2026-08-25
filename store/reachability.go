package store

import (
	"errors"

	"github.com/tggo/goRDFlib/term"
)

// ErrReachabilityUnsupported is returned by ReachabilityStore.Reachable when the
// backend cannot answer this particular query — either because the query shape is
// outside what it can express, or because answering it would exceed a backend
// limit (MongoDB's $graphLookup, for example, is capped at 100 MiB per stage and
// ignores allowDiskUse).
//
// Callers must treat it as "compute this yourself", never as a failure: property
// path evaluation falls back to a client-side traversal, so a backend is always
// free to decline. Declining is cheap and correct; a wrong answer is not.
var ErrReachabilityUnsupported = errors.New("store: reachability query not supported by this backend")

// ReachabilityQuery describes a transitive traversal over a flat set of
// predicates — the shape that SPARQL property paths of the form `<p>*`, `<p>+`,
// `^<p>+`, `(<p1>|<p2>)*` and `!(<p1>|<p2>)+` reduce to.
//
// It deliberately cannot express nested paths (`(<p1>/<p2>)*`): those are the
// caller's job to decompose, and a backend must never see a query it would have
// to interpret.
type ReachabilityQuery struct {
	// Start is the node the traversal begins at. Required.
	//
	// Start is not itself part of the result unless a cycle leads back to it.
	Start term.Term

	// Predicates restricts which edges may be traversed. An empty slice means
	// "any predicate", regardless of Negated.
	Predicates []term.URIRef

	// Negated inverts Predicates: traverse every edge whose predicate is NOT in
	// the set. This is SPARQL's negated property set, `!(<p1>|<p2>)`.
	Negated bool

	// Inverse traverses edges backwards, object → subject, instead of
	// subject → object. This is SPARQL's `^<p>`.
	Inverse bool

	// MaxDepth bounds the traversal to at most this many steps. Zero or negative
	// means unbounded — which is safe, because a node is expanded at most once.
	MaxDepth int

	// Context is the named graph to traverse, following the same convention as
	// the rest of Store: nil means the default graph. Backends that report
	// ContextAware() == false ignore it.
	Context term.Term
}

// ReachabilityStore is an optional interface a Store may implement to answer
// transitive-closure queries in the backend instead of shipping every candidate
// edge to the client. Implementing it is what lets MongoDB's $graphLookup,
// SQL recursive CTEs, or a native graph engine be used for property paths.
//
// Implementing it is never required: paths.MulPath falls back to a client-side
// traversal whenever the interface is absent or Reachable declines.
type ReachabilityStore interface {
	// Reachable returns every node reachable from q.Start in one or more steps.
	//
	// The contract, in full:
	//
	//   - The result is a set: no duplicates, order unspecified.
	//   - Nodes reachable in one step are included; q.Start itself is included
	//     only if some path of length >= 1 leads back to it.
	//   - Cycles must terminate. A node is expanded at most once.
	//   - Terminal nodes count. When traversing forward, a literal object is a
	//     reachable node even though nothing can be traversed from it.
	//   - The result is fully materialized. A backend that cannot produce the
	//     complete set — because it hit a size limit, lost its connection, or
	//     does not support the query shape — must return an error and no
	//     partial result. Callers rely on this to fall back safely: a partially
	//     yielded stream cannot be un-yielded.
	//   - Any error means "compute this yourself". Wrap or return
	//     ErrReachabilityUnsupported for the expected case of declining.
	Reachable(q ReachabilityQuery) ([]term.Term, error)
}

// Reachable implements ReachabilityStore with a breadth-first traversal over the
// SPO and OSP indexes. MemoryStore has no network to save, so this is not a
// performance win over the client-side fallback — it exists so that every
// property path in the test suite exercises the pushdown path, and so that
// storetest has a reference implementation to check other backends against.
//
// Safe for concurrent use.
func (m *MemoryStore) Reachable(q ReachabilityQuery) ([]term.Term, error) {
	if q.Start == nil {
		return nil, ErrReachabilityUnsupported
	}

	allowed := make(map[string]bool, len(q.Predicates))
	for _, p := range q.Predicates {
		allowed[term.TermKey(p)] = true
	}
	// An empty predicate set means "any predicate" in both polarities: an empty
	// exclusion list excludes nothing.
	matches := func(pk string) bool {
		switch {
		case len(allowed) == 0:
			return true
		case q.Negated:
			return !allowed[pk]
		default:
			return allowed[pk]
		}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// expanded guards against re-visiting a node (and so against cycles);
	// emitted deduplicates the result set. They are separate because Start is
	// expanded without being emitted — it only enters the result if an edge
	// leads back to it.
	expanded := map[string]bool{term.TermKey(q.Start): true}
	emitted := make(map[string]bool)
	var result []term.Term

	frontier := []term.Term{q.Start}
	for depth := 0; len(frontier) > 0; depth++ {
		if q.MaxDepth > 0 && depth >= q.MaxDepth {
			break
		}
		var next []term.Term
		for _, node := range frontier {
			nk := term.TermKey(node)
			if q.Inverse {
				// osp: object → subject → predicate → triple
				for sk, byPred := range m.osp[nk] {
					for pk, t := range byPred {
						if !matches(pk) {
							continue
						}
						if !emitted[sk] {
							emitted[sk] = true
							result = append(result, t.Subject)
						}
						if !expanded[sk] {
							expanded[sk] = true
							next = append(next, t.Subject)
						}
						break // one traversable edge to sk is enough
					}
				}
				continue
			}
			// spo: subject → predicate → object → triple
			for pk, byObj := range m.spo[nk] {
				if !matches(pk) {
					continue
				}
				for ok, t := range byObj {
					if !emitted[ok] {
						emitted[ok] = true
						result = append(result, t.Object)
					}
					if !expanded[ok] {
						expanded[ok] = true
						next = append(next, t.Object)
					}
				}
			}
		}
		frontier = next
	}

	return result, nil
}

// Compile-time check: MemoryStore must implement ReachabilityStore.
var _ ReachabilityStore = (*MemoryStore)(nil)
