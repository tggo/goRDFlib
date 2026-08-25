package paths

import (
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// flatSpec is a property path reduced to the shape a store.ReachabilityQuery can
// express: a set of predicates, a direction, and a polarity.
type flatSpec struct {
	preds   []term.URIRef
	inverse bool
	negated bool
}

// flatten reduces p to a flat predicate set, reporting false for any shape a
// backend cannot answer directly — sequences, nested repetitions, or an
// alternation that mixes directions or polarities.
//
// A negated set is never merged with anything: `!(<a>) | <b>` is not a flat
// include-list nor a flat exclude-list, so it stays on the fallback path.
func flatten(p Path) (flatSpec, bool) {
	switch v := p.(type) {
	case URIRefPath:
		return flatSpec{preds: []term.URIRef{v.URI}}, true

	case *URIRefPath:
		return flatSpec{preds: []term.URIRef{v.URI}}, true

	case *InvPath:
		inner, ok := flatten(v.Arg)
		if !ok {
			return flatSpec{}, false
		}
		inner.inverse = !inner.inverse
		return inner, true

	case *NegatedPath:
		// NegatedPath has no inverse form in this package: `!(^<p>)` parses as
		// InvPath{NegatedPath}, which the *InvPath case above handles.
		return flatSpec{preds: v.Excluded, negated: true}, true

	case *AlternativePath:
		if len(v.Args) == 0 {
			return flatSpec{}, false
		}
		var out flatSpec
		for i, arg := range v.Args {
			alt, ok := flatten(arg)
			if !ok || alt.negated {
				return flatSpec{}, false
			}
			if i == 0 {
				out.inverse = alt.inverse
			} else if alt.inverse != out.inverse {
				return flatSpec{}, false
			}
			out.preds = append(out.preds, alt.preds...)
		}
		return out, true

	default:
		// SequencePath, MulPath, and anything a caller has added themselves.
		return flatSpec{}, false
	}
}

// evalViaStore attempts to answer this repetition with a single backend
// reachability query instead of a client-side traversal. It reports whether it
// handled the evaluation; false means the caller must fall back, and in that
// case nothing has been emitted.
//
// Only the transitive forms (`*` and `+`) are pushed down. `?` is a single
// optional step, which the backend cannot answer more cheaply than a plain
// triple pattern already does.
//
// The zero-length pair is the caller's responsibility in both paths — this
// function only ever emits pairs at distance >= 1.
func (p *MulPath) evalViaStore(g *graph.Graph, subj term.Subject, obj term.Term, emit func(term.Term, term.Term) bool) bool {
	if !p.More {
		return false
	}
	rs, ok := g.Store().(store.ReachabilityStore)
	if !ok {
		return false
	}
	spec, ok := flatten(p.Path)
	if !ok {
		return false
	}

	// With neither endpoint bound the traversal has to start from every node in
	// the graph, which would be one round trip per node — worse than the local
	// traversal it is meant to replace.
	var start term.Term
	backward := false
	switch {
	case subj != nil:
		start = subj
	case obj != nil:
		start = obj
		backward = true
		spec.inverse = !spec.inverse
	default:
		return false
	}

	reached, err := rs.Reachable(store.ReachabilityQuery{
		Start:      start,
		Predicates: spec.preds,
		Negated:    spec.negated,
		Inverse:    spec.inverse,
		Context:    g.Identifier(),
	})
	if err != nil {
		// Documented contract: an error means "compute this yourself", and no
		// partial result has been emitted, so falling back is safe.
		return false
	}

	for _, n := range reached {
		if backward {
			if !emit(n, obj) {
				return true
			}
			continue
		}
		if obj != nil && term.TermKey(n) != term.TermKey(obj) {
			continue
		}
		if !emit(subj, n) {
			return true
		}
	}
	return true
}
