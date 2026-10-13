package sparql

import (
	"context"
	"errors"
	"fmt"

	rdflibgo "github.com/tggo/goRDFlib"
)

// ErrTooManySolutions is returned when a query or update builds more
// intermediate solutions than the budget set with WithMaxSolutions. Raise the
// budget, or make the query more selective; a context deadline bounds time
// but not memory, so a service that runs untrusted queries needs both.
var ErrTooManySolutions = errors.New("sparql: solution budget exceeded")

type maxSolutionsKey struct{}

// WithMaxSolutions returns a context that bounds every query and update run
// with it to n intermediate solutions. A solution is counted each time one is
// built: per BGP match, per row a join or OPTIONAL produces, per named graph a
// GRAPH ?g matches. When the count passes n, evaluation stops and the entry
// point returns an error matching ErrTooManySolutions, never a partial result.
//
// The count is cumulative over the evaluation, so it bounds the work and,
// with it, the memory a query can take; it is not a count of live rows. n <= 0
// removes the bound.
func WithMaxSolutions(ctx context.Context, n int) context.Context {
	return context.WithValue(ctx, maxSolutionsKey{}, n)
}

func maxSolutionsFrom(ctx context.Context) int {
	n, _ := ctx.Value(maxSolutionsKey{}).(int)
	return n
}

// produced counts n built solutions and reports whether evaluation must end,
// either because the budget is spent or because it was stopped before.
func (e *evalCtx) produced(n int) bool {
	if e == nil {
		return false
	}
	if e.err != nil {
		return true
	}
	if e.budget <= 0 {
		return false
	}
	e.built += n
	if e.built > e.budget {
		e.err = fmt.Errorf("%w: more than %d solutions", ErrTooManySolutions, e.budget)
		return true
	}
	return false
}

// pushdownLimit is how many solutions of the WHERE clause the query can use,
// or 0 when it needs all of them. Without ORDER BY, DISTINCT, grouping or
// aggregates the solution sequence is cut to OFFSET+LIMIT unchanged, so the
// pattern evaluation can stop there (issue #51); ASK needs one.
func pushdownLimit(q *ParsedQuery) int {
	switch q.Type {
	case "ASK":
		return 1
	case "SELECT", "CONSTRUCT":
		if q.Limit <= 0 || len(q.OrderBy) > 0 || q.Distinct ||
			len(q.GroupBy) > 0 || hasAggregates(q) || q.Having != nil {
			return 0
		}
		return q.Offset + q.Limit
	}
	return 0
}

// evalPatternUpTo is evalPattern for a caller that uses only the first limit
// solutions (limit <= 0: all of them). Which solutions those are is
// unspecified, as is the order of evalPattern's. It collects from
// eachSolution, so `LIMIT 10` over a cross product stops after ten rows
// instead of building the cross product first (issue #51).
func evalPatternUpTo(ec *evalCtx, g *rdflibgo.Graph, pattern Pattern, prefixes map[string]string, namedGraphs map[string]*rdflibgo.Graph, limit int) []map[string]rdflibgo.Term {
	if limit <= 0 {
		return evalPattern(ec, g, pattern, prefixes, namedGraphs)
	}
	var result []map[string]rdflibgo.Term
	eachSolution(ec, g, pattern, prefixes, namedGraphs, func(b map[string]rdflibgo.Term) bool {
		result = append(result, b)
		return len(result) < limit
	})
	return result
}

// eachSolution passes the solutions of pattern to yield, and reports false
// once yield or ec has stopped it. BGP, join, OPTIONAL, UNION and FILTER are
// streamed: the left side of a join is not built before the right side runs,
// so a consumer that stops early stops the whole nest. The right side of a
// join or OPTIONAL is evaluated per left row, as evalPattern does, and every
// other pattern is evaluated in full and then passed on. The solutions are the
// ones evalPattern returns.
func eachSolution(ec *evalCtx, g *rdflibgo.Graph, pattern Pattern, prefixes map[string]string, namedGraphs map[string]*rdflibgo.Graph, yield func(map[string]rdflibgo.Term) bool) bool {
	switch p := pattern.(type) {
	case *BGP:
		empty := map[string]rdflibgo.Term{}
		return eachBGPSolution(ec, g, orderBGP(g, p.Triples, empty, prefixes), empty, prefixes, yield)

	case *JoinPattern:
		return eachSolution(ec, g, p.Left, prefixes, namedGraphs, func(lb map[string]rdflibgo.Term) bool {
			if ec.stop() {
				return false
			}
			if r, ok := p.Right.(*BGP); ok {
				// What evalPatternWithBindings does for a BGP, streamed: its
				// solutions already contain lb.
				return eachBGPSolution(ec, g, orderBGP(g, r.Triples, lb, prefixes), lb, prefixes, yield)
			}
			for _, rb := range evalPatternWithBindings(ec, g, p.Right, lb, prefixes, namedGraphs) {
				if ec.produced(1) || !yield(mergeBindings(lb, rb)) {
					return false
				}
			}
			return !ec.stop()
		})

	case *OptionalPattern:
		return eachSolution(ec, g, p.Main, prefixes, namedGraphs, func(mb map[string]rdflibgo.Term) bool {
			if ec.stop() {
				return false
			}
			if o, ok := p.Optional.(*BGP); ok {
				matched := false
				if !eachBGPSolution(ec, g, orderBGP(g, o.Triples, mb, prefixes), mb, prefixes, func(b map[string]rdflibgo.Term) bool {
					matched = true
					return yield(b)
				}) {
					return false
				}
				return matched || (!ec.produced(1) && yield(mb))
			}
			opt := evalPatternWithBindings(ec, g, p.Optional, mb, prefixes, namedGraphs)
			if len(opt) == 0 {
				return !ec.produced(1) && yield(mb)
			}
			for _, ob := range opt {
				if ec.produced(1) || !yield(mergeBindings(mb, ob)) {
					return false
				}
			}
			return true
		})

	case *UnionPattern:
		return eachSolution(ec, g, p.Left, prefixes, namedGraphs, yield) &&
			!ec.poll() &&
			eachSolution(ec, g, p.Right, prefixes, namedGraphs, yield)

	case *FilterPattern:
		return eachSolution(ec, g, p.Pattern, prefixes, namedGraphs, func(b map[string]rdflibgo.Term) bool {
			if ec.stop() {
				return false
			}
			if effectiveBooleanValue(evalExprWithGraph(ec, p.Expr, b, prefixes, g, namedGraphs)) {
				return yield(b)
			}
			return true
		})
	}
	for _, b := range evalPattern(ec, g, pattern, prefixes, namedGraphs) {
		if ec.stop() || !yield(b) {
			return false
		}
	}
	return !ec.stop()
}
