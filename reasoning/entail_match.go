package reasoning

import (
	"context"

	"github.com/tggo/goRDFlib/term"
)

// pat is one position of a conclusion triple: a ground node, a variable (a
// blank node of the conclusion) or a triple term that contains variables.
type pat struct {
	id  int // ground node id, -1 otherwise
	v   int // variable index, -1 otherwise
	tt  *[3]pat
	vcs []int // variables below this pattern
}

// query is the conclusion graph as a conjunctive pattern over the model.
type query struct {
	m     *model
	vars  map[string]int
	trips [][3]pat
}

func newQuery(m *model) *query {
	return &query{m: m, vars: make(map[string]int, 8)}
}

func (q *query) addTriple(t term.Triple) {
	q.trips = append(q.trips, [3]pat{q.pattern(t.Subject), q.pattern(t.Predicate), q.pattern(t.Object)})
}

func (q *query) pattern(t term.Term) pat {
	switch x := t.(type) {
	case term.BNode:
		i, ok := q.vars[x.Value()]
		if !ok {
			i = len(q.vars)
			q.vars[x.Value()] = i
		}
		return pat{id: -1, v: i, vcs: []int{i}}
	case term.TripleTerm:
		parts := [3]pat{q.pattern(x.Subject()), q.pattern(x.Predicate()), q.pattern(x.Object())}
		var vcs []int
		for _, p := range parts {
			vcs = append(vcs, p.vcs...)
		}
		if len(vcs) == 0 {
			return pat{id: q.m.internTerm(x, false), v: -1}
		}
		return pat{id: -1, v: -1, tt: &parts, vcs: vcs}
	}
	return pat{id: q.m.internTerm(t, false), v: -1}
}

// solve reports whether some assignment of nodes to the conclusion's blank
// nodes makes every conclusion triple a triple of the saturated model.
func (q *query) solve(ctx context.Context) (bool, error) {
	bind := make([]int, len(q.vars))
	for i := range bind {
		bind[i] = -1
	}
	order := q.order()
	steps := 0
	var rec func(k int) (bool, error)
	rec = func(k int) (bool, error) {
		if k == len(order) {
			return true, nil
		}
		if steps++; steps&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return false, err
			}
		}
		t := q.trips[order[k]]
		for _, cand := range q.candidates(t, bind) {
			trail, ok := q.unifyTriple(t, q.m.triples[cand], bind)
			if ok {
				if found, err := rec(k + 1); found || err != nil {
					return found, err
				}
			}
			for _, v := range trail {
				bind[v] = -1
			}
		}
		return false, nil
	}
	return rec(0)
}

// order sorts the conclusion triples so that ground ones come first and each
// next one shares as many variables as possible with those before it.
func (q *query) order() []int {
	n := len(q.trips)
	done := make([]bool, n)
	seen := make(map[int]bool, len(q.vars))
	out := make([]int, 0, n)
	for len(out) < n {
		best, bestScore := -1, -1<<30
		for i, t := range q.trips {
			if done[i] {
				continue
			}
			score := 0
			for _, p := range t {
				switch {
				case p.v < 0 && p.tt == nil:
					score += 4
				default:
					for _, v := range p.vcs {
						if seen[v] {
							score += 3
						} else {
							score--
						}
					}
				}
			}
			if score > bestScore {
				best, bestScore = i, score
			}
		}
		done[best] = true
		out = append(out, best)
		for _, p := range q.trips[best] {
			for _, v := range p.vcs {
				seen[v] = true
			}
		}
	}
	return out
}

// known resolves a position to a node id when it is ground or a bound
// variable, and -1 otherwise.
func known(p pat, bind []int) int {
	if p.id >= 0 {
		return p.id
	}
	if p.v >= 0 {
		return bind[p.v]
	}
	return -1
}

func (q *query) candidates(t [3]pat, bind []int) []int {
	m := q.m
	s, p, o := known(t[0], bind), known(t[1], bind), known(t[2], bind)
	switch {
	case p < 0:
		all := make([]int, len(m.triples))
		for i := range all {
			all[i] = i
		}
		return all
	case s >= 0 && o >= 0:
		if _, ok := m.set[[3]int{s, p, o}]; ok {
			return []int{indexOf(m, [3]int{s, p, o})}
		}
		return nil
	case s >= 0:
		return m.bySP[[2]int{s, p}]
	case o >= 0:
		return m.byPO[[2]int{p, o}]
	}
	return m.byP[p]
}

func indexOf(m *model, k [3]int) int {
	for _, i := range m.bySP[[2]int{k[0], k[1]}] {
		if m.triples[i] == k {
			return i
		}
	}
	return -1
}

func (q *query) unifyTriple(t [3]pat, tr [3]int, bind []int) ([]int, bool) {
	var trail []int
	for i := 0; i < 3; i++ {
		if !q.unify(t[i], tr[i], bind, &trail) {
			for _, v := range trail {
				bind[v] = -1
			}
			return nil, false
		}
	}
	return trail, true
}

func (q *query) unify(p pat, id int, bind []int, trail *[]int) bool {
	switch {
	case p.id >= 0:
		return p.id == id
	case p.v >= 0:
		if bind[p.v] >= 0 {
			return bind[p.v] == id
		}
		bind[p.v] = id
		*trail = append(*trail, p.v)
		return true
	}
	n := q.m.nodes[id]
	if n.kind != nodeTripleTerm {
		return false
	}
	for i := 0; i < 3; i++ {
		if !q.unify(p.tt[i], n.parts[i], bind, trail) {
			return false
		}
	}
	return true
}
