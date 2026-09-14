package sparql

import (
	"slices"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/store"
)

// BGP join ordering.
//
// A basic graph pattern is a conjunction, so its triple patterns can be
// evaluated in any order without changing the solution multiset (the SPARQL
// algebra's Join is commutative and associative). evalBGP evaluates them as nested
// loops, which makes the order decide the cost: a query whose first pattern
// matches every label in the graph runs the rest of the BGP once per label,
// even when a later pattern would have narrowed the result to a handful of
// rows. orderBGP picks the order instead of trusting the order the author
// typed.
//
// The planner is greedy and statistics-free. A store that implements
// store.CardinalityStore gives exact match counts from its indexes and is not
// probed. Any other store has each pattern probed with its constants and
// pre-bound variables filled in, and the probe stops early. Probing is the planner's whole cost, and on MemoryStore a
// counted match costs about as much as an evaluated one, so the probes have to
// stay far smaller than a selective query:
//
//   - The first round stops each probe at bgpProbeStart matches, and at the
//     smallest exact count seen so far once one is known: a pattern that
//     reaches that count can no longer be cheaper. Patterns are probed
//     most-constrained first so that bound comes early.
//   - Only if every probe hit its limit, which says nothing about which
//     pattern is smaller, does a second round re-probe with bgpProbeLimit.
//
// A pattern whose probe hit the limit is "capped": its true count is unknown
// beyond being at least the limit it was probed with. Exact counts rank before
// capped ones, since a capped pattern is at least as large as the limit that
// stopped it and possibly far larger. After probing every capped count is set
// to the same final limit, so capped patterns compare by structure and written
// order rather than by the accident of which was probed first.
//
// Variables that an earlier pattern will bind are not known values at planning
// time; they are credited with a fixed selectivity divisor instead. Patterns
// that share no variable with what is already bound are postponed, because
// running them early produces a cross product.
//
// Result order changes with evaluation order. SPARQL leaves the order of
// solutions unspecified without ORDER BY, so this is not an observable
// difference in the standard's terms, but callers that relied on the old
// incidental order will see a different one.

const (
	// bgpProbeStart is the first-round probe limit.
	bgpProbeStart = 64
	// bgpProbeLimit is the second-round probe limit, used only when every
	// first-round probe was capped.
	bgpProbeLimit = 1000
)

// Selectivity credited to a position whose variable is bound by a pattern
// chosen earlier. A bound subject narrows a pattern the most (a subject has
// few values per predicate), a bound predicate the least.
const (
	boundSubjectDivisor   = 100
	boundObjectDivisor    = 10
	boundPredicateDivisor = 4
)

var bgpPositionDivisor = [3]float64{boundSubjectDivisor, boundPredicateDivisor, boundObjectDivisor}

type bgpCandidate struct {
	tp Triple
	// vars holds the variable name in subject, predicate and object position,
	// or "" when that position is not a variable left unbound by the caller.
	vars [3]string
	// fixed counts the positions known before evaluation (constants and
	// pre-bound variables); it decides probe order.
	fixed int
	// s, p, o are the probe pattern; nil is a wildcard.
	s rdflibgo.Subject
	p *rdflibgo.URIRef
	o rdflibgo.Term

	// probed is false until count holds a result.
	probed bool
	// count is the number of matches, exact unless capped.
	count int
	// capped means the probe stopped at its limit, so count is a lower bound.
	capped bool
	// path marks a property-path pattern. Paths are never probed, since
	// counting a path costs as much as evaluating it; they rank below every
	// probed pattern.
	path bool
}

// orderBGP returns the triple patterns in the order evalBGP should run them.
// It returns triples unchanged when there is nothing to choose, and when a
// pattern contains a triple term with variables: those bind variables through
// matchTripleTermPattern rather than by position, which the cost model does
// not track.
func orderBGP(g *rdflibgo.Graph, triples []Triple, bindings map[string]rdflibgo.Term, prefixes map[string]string) []Triple {
	if len(triples) < 2 {
		return triples
	}

	cands := make([]bgpCandidate, len(triples))
	for i, tp := range triples {
		if isVarTripleTerm(tp.Subject) || isVarTripleTerm(tp.Object) {
			return triples
		}
		cands[i] = newBGPCandidate(tp, bindings, prefixes)
	}
	probeCandidates(g, cands)

	ordered := make([]Triple, 0, len(triples))
	bound := make(map[string]struct{}, 2*len(triples))
	used := make([]bool, len(cands))
	for range cands {
		best := -1
		var bestKey bgpCostKey
		for i := range cands {
			if used[i] {
				continue
			}
			key := cands[i].cost(bound, len(ordered) > 0)
			if best < 0 || key.less(bestKey) {
				best, bestKey = i, key
			}
		}
		used[best] = true
		ordered = append(ordered, cands[best].tp)
		for _, v := range cands[best].vars {
			if v != "" {
				bound[v] = struct{}{}
			}
		}
	}
	return ordered
}

func newBGPCandidate(tp Triple, bindings map[string]rdflibgo.Term, prefixes map[string]string) bgpCandidate {
	c := bgpCandidate{tp: tp}
	for pos, s := range [3]string{tp.Subject, tp.Predicate, tp.Object} {
		if pos == 1 && tp.PredicatePath != nil {
			continue
		}
		if v, ok := strings.CutPrefix(s, "?"); ok {
			if _, pre := bindings[v]; !pre {
				c.vars[pos] = v
			}
		}
	}

	if tp.PredicatePath != nil {
		c.path, c.probed = true, true
		// Bound endpoints still make a path cheaper than an unbound one.
		if resolvePatternTerm(tp.Subject, bindings, prefixes) != nil {
			c.fixed++
		}
		if resolvePatternTerm(tp.Object, bindings, prefixes) != nil {
			c.fixed++
		}
		return c
	}

	if v := resolvePatternTerm(tp.Subject, bindings, prefixes); v != nil {
		s, ok := v.(rdflibgo.Subject)
		if !ok {
			c.probed = true // unmatchable, see evalBGP: count 0
			return c
		}
		c.s = s
		c.fixed++
	}
	if v := resolvePatternTerm(tp.Predicate, bindings, prefixes); v != nil {
		u, ok := v.(rdflibgo.URIRef)
		if !ok {
			c.probed = true
			return c
		}
		c.p = &u
		c.fixed++
	}
	if v := resolvePatternTerm(tp.Object, bindings, prefixes); v != nil {
		c.o = v
		c.fixed++
	}
	return c
}

// probeCandidates fills in count and capped for every candidate that needs
// one: exactly from a CardinalityStore, otherwise by probing in the rounds
// described at the top of the file.
func probeCandidates(g *rdflibgo.Graph, cands []bgpCandidate) {
	if cs, ok := g.Store().(store.CardinalityStore); ok {
		for i := range cands {
			if c := &cands[i]; !c.probed {
				c.count = cs.Cardinality(rdflibgo.TriplePattern{Subject: c.s, Predicate: c.p, Object: c.o}, g.Identifier())
				c.probed = true
			}
		}
		return
	}

	order := make([]int, 0, len(cands))
	for i := range cands {
		if !cands[i].probed {
			order = append(order, i)
		}
	}
	if len(order) == 0 {
		return
	}
	slices.SortStableFunc(order, func(a, b int) int {
		return cands[b].fixed - cands[a].fixed
	})

	limit := bgpProbeStart
	for {
		exact := false
		for _, i := range order {
			c := &cands[i]
			if c.probed && !c.capped {
				continue // exact from the first round
			}
			c.count = probeCount(g, c.s, c.p, c.o, limit)
			c.probed = true
			c.capped = c.count >= limit
			if !c.capped {
				exact = true
				limit = max(1, min(limit, c.count))
			}
		}
		if exact || limit >= bgpProbeLimit {
			break
		}
		limit = bgpProbeLimit
	}

	// A pattern capped early was stopped by a looser limit than one capped
	// late; neither says more than "large", so give them the same count.
	for _, i := range order {
		if c := &cands[i]; c.capped {
			c.count = limit
		}
	}
}

// bgpCostKey orders candidates: connected before disconnected, exact before
// capped, then by estimated cost. Ties keep the written order, because
// orderBGP only replaces its pick on a strictly smaller key.
type bgpCostKey struct {
	disconnected bool
	capped       bool
	cost         float64
}

func (k bgpCostKey) less(o bgpCostKey) bool {
	if k.disconnected != o.disconnected {
		return !k.disconnected
	}
	if k.capped != o.capped {
		return !k.capped
	}
	return k.cost < o.cost
}

// cost scores the candidate given the variables bound by patterns already
// chosen. A disconnected candidate shares no variable with them and still has
// variables of its own to bind.
func (c *bgpCandidate) cost(bound map[string]struct{}, anyChosen bool) bgpCostKey {
	if !c.path && c.count == 0 {
		return bgpCostKey{} // an empty pattern empties the BGP; run it first
	}
	cost := float64(c.count)
	if c.path {
		// Paths are unprobed; bgpProbeLimit is above every probed count.
		cost = float64(bgpProbeLimit) * float64(1+c.pathPenalty())
	}
	connected, free := false, false
	for pos, v := range c.vars {
		if v == "" {
			continue
		}
		if _, ok := bound[v]; ok {
			connected = true
			cost /= bgpPositionDivisor[pos]
		} else {
			free = true
		}
	}
	return bgpCostKey{
		disconnected: anyChosen && !connected && free,
		capped:       c.capped || c.path,
		cost:         cost,
	}
}

// pathPenalty is 2 for a path with no endpoint known before evaluation, 1 for
// one known endpoint and 0 for both.
func (c *bgpCandidate) pathPenalty() int { return 2 - c.fixed }

// probeCount counts matches up to limit. A QueryableStore gets the limit
// pushed down, which matters for SQL and remote backends where an abandoned
// iteration may still have fetched the whole result.
func probeCount(g *rdflibgo.Graph, s rdflibgo.Subject, p *rdflibgo.URIRef, o rdflibgo.Term, limit int) int {
	n := 0
	count := func(rdflibgo.Triple) bool {
		n++
		return n < limit
	}
	if qs, ok := g.Store().(store.QueryableStore); ok {
		pat := rdflibgo.TriplePattern{Subject: s, Predicate: p, Object: o}
		qs.TriplesWithLimit(pat, g.Identifier(), limit, 0)(count)
		return n
	}
	g.Triples(s, p, o)(count)
	return n
}

func isVarTripleTerm(s string) bool {
	return strings.HasPrefix(s, "<<( ") && tripleTermHasVariables(s)
}
