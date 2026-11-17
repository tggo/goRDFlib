package sparql

import (
	rdflibgo "github.com/tggo/goRDFlib"
)

// joinSide is the right side of a join evaluated without initial bindings
// (evalPattern and eachSolution), asked for its solutions one left row at a
// time.
//
// A BGP or a SERVICE clause takes each row as bindings, through
// evalPatternWithBindings. Any other right side is a group whose solutions do
// not depend on the left row (SPARQL 1.1 §18.2.1), so matching it in full once
// per row repeats the same work for every row. One without expressions
// (expressionFree) is instead evaluated with the row's values substituted, an
// index nested-loop join that only touches the triples that can match. Any
// other is evaluated once, when the first row asks, and its solutions are
// joined with each row. A right side that calls a function giving each call
// its own value, such as RAND or STRUUID, is still evaluated once per row, as
// is every right side while offJoinRightSide is set.
//
// A joinSide is not safe for concurrent use.
type joinSide struct {
	ec          *evalCtx
	g           *rdflibgo.Graph
	pattern     Pattern
	prefixes    map[string]string
	namedGraphs map[string]*rdflibgo.Graph

	// perRow is whether the right side is evaluated for every row through
	// evalPatternWithBindings.
	perRow bool
	// free describes an expression-free right side, which takes a row's
	// values by substitution; nil otherwise.
	free *rightSide
	// once holds the right side's solutions once evaluated is set.
	once      []map[string]rdflibgo.Term
	evaluated bool
}

func newJoinSide(ec *evalCtx, g *rdflibgo.Graph, pattern Pattern, prefixes map[string]string, namedGraphs map[string]*rdflibgo.Graph) *joinSide {
	pattern = ec.mergeJoined(pattern) // a join of BGPs is one BGP (bgp_merge.go)
	j := &joinSide{ec: ec, g: g, pattern: pattern, prefixes: prefixes, namedGraphs: namedGraphs}
	switch pattern.(type) {
	case *BGP, *ServicePattern:
		j.perRow = true
		return j
	}
	if offJoinRightSide || callsVolatileFunction(pattern) {
		j.perRow = true
		return j
	}
	if joinRightSideHook != nil {
		joinRightSideHook()
	}
	if rs := ec.rightSideOf(pattern); rs.free && len(rs.vars) > 0 {
		j.free = rs
	}
	return j
}

// rows returns the right side's solutions for the left row lb. When filter is
// true, only those compatible with lb join it; otherwise all of them do.
func (j *joinSide) rows(lb map[string]rdflibgo.Term) (rows []map[string]rdflibgo.Term, filter bool) {
	if j.perRow {
		return evalPatternWithBindings(j.ec, j.g, j.pattern, lb, j.prefixes, j.namedGraphs), false
	}
	if j.free != nil && j.free.bindsAVariableIn(lb, nil) {
		return evalPatternPreBound(j.ec, j.g, j.pattern, lb, j.prefixes, j.namedGraphs), false
	}
	if !j.evaluated {
		j.once = evalPattern(j.ec, j.g, j.pattern, j.prefixes, j.namedGraphs)
		j.evaluated = true
	}
	return j.once, true
}
