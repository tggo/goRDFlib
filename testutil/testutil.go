package testutil

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// AssertGraphEqual checks that two graphs are isomorphic — they contain the
// same triples up to blank-node relabelling.
func AssertGraphEqual(t *testing.T, expected, actual *graph.Graph) {
	t.Helper()

	if expected.Len() != actual.Len() {
		t.Errorf("graph triple count: expected %d, got %d", expected.Len(), actual.Len())
		reportDiff(t, expected, actual)
		return
	}

	// Fast path: if no blank nodes, exact string comparison suffices.
	expTriples := collectTriples(expected)
	actTriples := collectTriples(actual)

	if !hasBNodes(expTriples) && !hasBNodes(actTriples) {
		expSet := tripleSet(expTriples)
		actSet := tripleSet(actTriples)
		if !mapsEqual(expSet, actSet) {
			reportDiff(t, expected, actual)
		}
		return
	}

	// Blank-node isomorphism check.
	if !isomorphic(expTriples, actTriples) {
		reportDiff(t, expected, actual)
	}
}

type triple struct {
	s, p, o term.Term
}

func collectTriples(g *graph.Graph) []triple {
	var ts []triple
	g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		ts = append(ts, triple{t.Subject, t.Predicate, t.Object})
		return true
	})
	return ts
}

func hasBNodes(ts []triple) bool {
	for _, t := range ts {
		if _, ok := t.s.(term.BNode); ok {
			return true
		}
		if _, ok := t.o.(term.BNode); ok {
			return true
		}
	}
	return false
}

func tripleSet(ts []triple) map[string]bool {
	m := make(map[string]bool, len(ts))
	for _, t := range ts {
		m[tripleKey(t)] = true
	}
	return m
}

func tripleKey(t triple) string {
	return fmt.Sprintf("%s %s %s", t.s.(term.Subject).N3(), t.p.(term.URIRef).N3(), termN3(t.o))
}

func termN3(t term.Term) string {
	switch v := t.(type) {
	case term.URIRef:
		return v.N3()
	case term.BNode:
		return v.N3()
	case term.Literal:
		return v.N3()
	default:
		return fmt.Sprintf("%v", t)
	}
}

// isomorphic checks whether two sets of triples are isomorphic under blank-node
// relabelling using a backtracking search with degree-based heuristics.
func isomorphic(exp, act []triple) bool {
	// Collect blank nodes from each graph.
	expBNodes := bnodeSet(exp)
	actBNodes := bnodeSet(act)
	if len(expBNodes) != len(actBNodes) {
		return false
	}
	if len(expBNodes) == 0 {
		// No bnodes — just compare sets.
		return mapsEqual(tripleSet(exp), tripleSet(act))
	}

	// Build "signature" for each bnode: multiset of (pred, isSubj, otherTermN3-or-bnode-marker).
	// BNodes with the same signature are candidates for mapping to each other.
	expSigs := bnodeSigs(exp)
	actSigs := bnodeSigs(act)

	// Build candidate map: for each exp bnode, which act bnodes have the same signature.
	candidates := make(map[string][]string, len(expBNodes))
	for eb, esig := range expSigs {
		for ab, asig := range actSigs {
			if esig == asig {
				candidates[eb] = append(candidates[eb], ab)
			}
		}
	}

	// Check that every exp bnode has at least one candidate.
	for _, eb := range expBNodes {
		if len(candidates[eb]) == 0 {
			return false
		}
	}

	// Backtracking search.
	mapping := make(map[string]string, len(expBNodes))   // exp -> act
	usedAct := make(map[string]bool, len(actBNodes))

	// Sort expBNodes by candidate count (most constrained first).
	sortBNodesByConstraint(expBNodes, candidates)

	// Index actual triples for fast lookup.
	actSet := tripleSet(act)

	var search func(idx int) bool
	search = func(idx int) bool {
		if idx == len(expBNodes) {
			// Verify all exp triples map to act triples.
			return verifyMapping(exp, actSet, mapping)
		}
		eb := expBNodes[idx]
		for _, ab := range candidates[eb] {
			if usedAct[ab] {
				continue
			}
			mapping[eb] = ab
			usedAct[ab] = true
			if search(idx + 1) {
				return true
			}
			delete(mapping, eb)
			delete(usedAct, ab)
		}
		return false
	}

	return search(0)
}

func bnodeSet(ts []triple) []string {
	seen := map[string]bool{}
	var result []string
	add := func(t term.Term) {
		if b, ok := t.(term.BNode); ok {
			if !seen[b.Value()] {
				seen[b.Value()] = true
				result = append(result, b.Value())
			}
		}
	}
	for _, tr := range ts {
		add(tr.s)
		add(tr.o)
	}
	return result
}

// bnodeSigs builds a canonical signature string for each bnode based on its
// immediate neighbourhood (predicates, directions, and non-bnode neighbours).
func bnodeSigs(ts []triple) map[string]string {
	// Collect (direction, predicate, other) tuples per bnode.
	type entry struct{ dir, pred, other string }
	m := map[string][]entry{}

	for _, tr := range ts {
		pred := tr.p.(term.URIRef).Value()
		sb, sbOk := tr.s.(term.BNode)
		ob, obOk := tr.o.(term.BNode)

		if sbOk {
			other := "_"
			if !obOk {
				other = termN3(tr.o)
			}
			m[sb.Value()] = append(m[sb.Value()], entry{"S", pred, other})
		}
		if obOk {
			other := "_"
			if !sbOk {
				other = termN3(tr.s)
			}
			m[ob.Value()] = append(m[ob.Value()], entry{"O", pred, other})
		}
	}

	// Sort entries and join to form signature.
	sigs := make(map[string]string, len(m))
	for bn, entries := range m {
		strs := make([]string, len(entries))
		for i, e := range entries {
			strs[i] = e.dir + "|" + e.pred + "|" + e.other
		}
		// Sort for canonical form.
		sortStrings(strs)
		sigs[bn] = strings.Join(strs, "\n")
	}
	return sigs
}

func sortStrings(ss []string) {
	// Simple insertion sort (lists are small).
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}

func sortBNodesByConstraint(bns []string, candidates map[string][]string) {
	for i := 1; i < len(bns); i++ {
		for j := i; j > 0 && len(candidates[bns[j]]) < len(candidates[bns[j-1]]); j-- {
			bns[j], bns[j-1] = bns[j-1], bns[j]
		}
	}
}

func verifyMapping(exp []triple, actSet map[string]bool, mapping map[string]string) bool {
	for _, tr := range exp {
		mapped := applyMapping(tr, mapping)
		if !actSet[mapped] {
			return false
		}
	}
	return true
}

func applyMapping(tr triple, mapping map[string]string) string {
	s := mapTerm(tr.s, mapping)
	p := tr.p.(term.URIRef).N3()
	o := mapTerm(tr.o, mapping)
	return fmt.Sprintf("%s %s %s", s, p, o)
}

func mapTerm(t term.Term, mapping map[string]string) string {
	if b, ok := t.(term.BNode); ok {
		if mapped, exists := mapping[b.Value()]; exists {
			return "_:" + mapped
		}
		return b.N3()
	}
	return termN3(t)
}

func mapsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func reportDiff(t *testing.T, expected, actual *graph.Graph) {
	t.Helper()
	expTriples := graphTripleStrings(expected)
	actTriples := graphTripleStrings(actual)

	expSet := make(map[string]bool, len(expTriples))
	for _, s := range expTriples {
		expSet[s] = true
	}
	actSet := make(map[string]bool, len(actTriples))
	for _, s := range actTriples {
		actSet[s] = true
	}

	var missing, extra []string
	for s := range expSet {
		if !actSet[s] {
			missing = append(missing, s)
		}
	}
	for s := range actSet {
		if !expSet[s] {
			extra = append(extra, s)
		}
	}

	var sb strings.Builder
	if len(missing) > 0 {
		sb.WriteString("Missing triples:\n")
		for _, s := range missing {
			sb.WriteString("  - " + s + "\n")
		}
	}
	if len(extra) > 0 {
		sb.WriteString("Extra triples:\n")
		for _, s := range extra {
			sb.WriteString("  + " + s + "\n")
		}
	}
	if sb.Len() > 0 {
		t.Error(sb.String())
	}
}

func graphTripleStrings(g *graph.Graph) []string {
	var result []string
	g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		s := fmt.Sprintf("%s %s %s", t.Subject.N3(), t.Predicate.N3(), t.Object.N3())
		result = append(result, s)
		return true
	})
	return result
}

// AssertGraphContains checks that the graph contains a specific triple.
func AssertGraphContains(t *testing.T, g *graph.Graph, s term.Subject, p term.URIRef, o term.Term) {
	t.Helper()
	if !g.Contains(s, p, o) {
		t.Errorf("graph does not contain: %s %s %s", s.N3(), p.N3(), o.N3())
	}
}

// AssertGraphLen checks the number of triples in a graph.
func AssertGraphLen(t *testing.T, g *graph.Graph, expected int) {
	t.Helper()
	if g.Len() != expected {
		t.Errorf("expected %d triples, got %d", expected, g.Len())
	}
}
