package sparql

import "strings"

// Joined basic graph patterns.
//
// The parser closes a basic graph pattern at a FILTER and at a nested group, so
// a group such as
//
//	{ ?this ex:part+ ?a , ?b . ?a ex:type ?c . FILTER (?c IN (…)) ?b ex:type ex:ClassB }
//
// becomes Filter(Join(BGP1, BGP2)) and the evaluator runs BGP2 once per
// solution of BGP1. BGP1 alone cannot tell that ?b only matters when it is an
// ex:ClassB: its solutions are every (?a, ?c) row times every ?b reachable from
// ?this, and BGP2 is matched for each of them. A basic graph pattern is a set
// of triple patterns (SPARQL 1.1 §18.1) and the join of two BGPs is the BGP of
// all their patterns, so evaluating the leaves of such a join as one BGP gives
// the same solutions, multiplicities included, and lets the planner order all
// the patterns together. The FILTERs of the group are not affected: they wrap
// the join and apply to its solutions either way.

// mergeJoinedBGPs returns p unchanged unless p is a join whose leaves are all
// basic graph patterns, in which case it returns one BGP with the leaves'
// triple patterns in order. A blank node label is scoped to its basic graph
// pattern, so leaves that share one are not merged (the query is not valid
// SPARQL anyway); neither is a leaf with a triple term holding a blank node.
// The query's own patterns are never modified.
func mergeJoinedBGPs(p Pattern) Pattern {
	j, ok := p.(*JoinPattern)
	if !ok {
		return p
	}
	n, ok := countJoinedTriples(j)
	if !ok {
		return p
	}
	out := &BGP{Triples: make([]Triple, 0, n)}
	var labels map[string]*BGP // blank node label → the leaf it appears in
	var collect func(Pattern) bool
	collect = func(p Pattern) bool {
		if j, ok := p.(*JoinPattern); ok {
			return collect(j.Left) && collect(j.Right)
		}
		leaf := p.(*BGP)
		for _, t := range leaf.Triples {
			for _, s := range [2]string{t.Subject, t.Object} {
				switch {
				case strings.HasPrefix(s, "_:"):
					if labels == nil {
						labels = map[string]*BGP{}
					}
					if other, seen := labels[s]; seen && other != leaf {
						return false
					}
					labels[s] = leaf
				case strings.HasPrefix(s, "<<") && strings.Contains(s, "_:"):
					return false
				}
			}
		}
		out.Triples = append(out.Triples, leaf.Triples...)
		return true
	}
	if !collect(j) {
		return p
	}
	return out
}

// countJoinedTriples reports whether every leaf of the join tree p is a BGP,
// and how many triple patterns they hold.
func countJoinedTriples(p Pattern) (int, bool) {
	switch p := p.(type) {
	case *BGP:
		return len(p.Triples), true
	case *JoinPattern:
		l, ok := countJoinedTriples(p.Left)
		if !ok {
			return 0, false
		}
		r, ok := countJoinedTriples(p.Right)
		return l + r, ok
	}
	return 0, false
}
