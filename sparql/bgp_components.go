package sparql

import (
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// BGP components.
//
// Nested loops evaluate every triple pattern once per solution of the patterns
// before it. When a BGP falls into groups that share no variable (variables
// already bound by the caller, such as a pre-bound ?this, are constants and
// join nothing), a later group does not depend on the earlier ones at all, yet
// it is evaluated again for each of their solutions:
//
//	?this ex:part+ ?a . ?a a ex:ClassA .  ?this ex:part+ ?b . ?b a ex:ClassB .
//
// walks from ?this again for every ?a. The join of groups that share no
// variable is their cross product (SPARQL 1.1 §18.5: two such solutions are
// always compatible), so evalBGPComponents evaluates each group once, in the
// planned order within the group, and combines their solutions. The solution
// multiset is unchanged.

// bgpComponents returns the triple patterns (in the order given) split into
// groups that share no variable unbound by bindings, or nil when there is only
// one group. A blank node label is treated like a variable: it keeps the
// patterns it appears in together. Variables and labels inside a triple term
// count too, nested ones included: matchTripleTermPattern binds the variables
// of a term pattern like any position does, so two patterns that share one
// are joined on it, not multiplied.
func bgpComponents(triples []Triple, bindings map[string]rdflibgo.Term) [][]Triple {
	if len(triples) < 2 || offBGPComponents {
		return nil
	}
	for _, tp := range triples {
		if strings.HasPrefix(tp.Subject, "<<") || strings.HasPrefix(tp.Object, "<<") {
			return bgpComponentsNested(triples, bindings)
		}
	}
	// Called for every BGP evaluation, often once per row of an enclosing
	// pattern, so a small BGP is grouped without allocating: most are
	// connected and return nil before any group is built. A large one looks
	// names up in a map rather than by a linear scan. (The arrays below stay
	// on the stack only because nothing here is reachable from a method that
	// recurses; the triple term case, which recurses, is bgpComponentsNested.)
	var parentBuf [16]int
	var namesBuf [48]componentName
	parent := parentBuf[:0]
	names := namesBuf[:0]
	var owner map[string]int
	if len(triples) > len(parentBuf) {
		owner = make(map[string]int, 2*len(triples))
	}
	for i := range triples {
		parent = append(parent, i)
	}
	for i, tp := range triples {
		pos := [3]string{tp.Subject, tp.Object, ""}
		if tp.PredicatePath == nil {
			pos[2] = tp.Predicate
		}
		for _, s := range pos {
			switch {
			case strings.HasPrefix(s, "?"):
				if _, bound := bindings[s[1:]]; bound {
					continue
				}
			case strings.HasPrefix(s, "_:"):
			default:
				continue
			}
			j, seen := -1, false
			if owner != nil {
				j, seen = owner[s]
			} else {
				for _, n := range names {
					if n.name == s {
						j, seen = n.owner, true
						break
					}
				}
			}
			if !seen {
				if owner != nil {
					owner[s] = i
				} else {
					names = append(names, componentName{s, i})
				}
				continue
			}
			parent[componentRoot(parent, i)] = componentRoot(parent, j)
		}
	}
	return splitComponents(parent, triples)
}

// bgpComponentsNested is bgpComponents for patterns with a triple term in the
// subject or object position, whose variables and labels are found by a
// recursive walk. It allocates.
func bgpComponentsNested(triples []Triple, bindings map[string]rdflibgo.Term) [][]Triple {
	cb := componentBuilder{bindings: bindings, parent: make([]int, 0, len(triples)), owner: make(map[string]int, 2*len(triples))}
	for i := range triples {
		cb.parent = append(cb.parent, i)
	}
	for i, tp := range triples {
		cb.visit(tp.Subject, i)
		cb.visit(tp.Object, i)
		if tp.PredicatePath == nil {
			cb.visit(tp.Predicate, i)
		}
	}
	return splitComponents(cb.parent, triples)
}

// splitComponents returns the patterns grouped by the roots of parent, or nil
// when they all share one.
func splitComponents(parent []int, triples []Triple) [][]Triple {
	root := componentRoot(parent, 0)
	split := false
	for i := 1; i < len(parent); i++ {
		if componentRoot(parent, i) != root {
			split = true
			break
		}
	}
	if !split {
		return nil
	}
	var comps [][]Triple
	index := make(map[int]int, len(triples))
	for i, tp := range triples {
		r := componentRoot(parent, i)
		k, ok := index[r]
		if !ok {
			k = len(comps)
			index[r] = k
			comps = append(comps, nil)
		}
		comps[k] = append(comps[k], tp)
	}
	return comps
}

// componentBuilder is the union-find state of bgpComponents.
type componentBuilder struct {
	bindings map[string]rdflibgo.Term
	parent   []int
	owner    map[string]int
}

// visit joins pattern i with every earlier pattern that holds one of the
// unbound variables or blank node labels of s, a position of pattern i.
func (cb *componentBuilder) visit(s string, i int) {
	switch {
	case strings.HasPrefix(s, "?"):
		if _, bound := cb.bindings[s[1:]]; bound {
			return
		}
	case strings.HasPrefix(s, "_:"):
	case strings.HasPrefix(s, "<<( ") && strings.HasSuffix(s, " )>>"):
		if len(s) < 8 {
			return
		}
		for _, part := range splitTripleTermParts(s[4 : len(s)-4]) {
			cb.visit(part, i)
		}
		return
	default:
		return
	}
	j, seen := cb.owner[s]
	if !seen {
		cb.owner[s] = i
		return
	}
	cb.parent[componentRoot(cb.parent, i)] = componentRoot(cb.parent, j)
}

// componentName records the first pattern a variable or blank node label
// appears in.
type componentName struct {
	name  string
	owner int
}

// componentRoot returns the representative of i's group, halving the path.
func componentRoot(parent []int, i int) int {
	for parent[i] != i {
		parent[i] = parent[parent[i]]
		i = parent[i]
	}
	return i
}

// evalBGPComponents evaluates each group once and returns the cross product of
// their solutions, at most limit of them (limit <= 0: all). The product is
// enumerated with the first group varying slowest, and only its rows are
// built: each one is counted against the solution budget (WithMaxSolutions),
// as nested loops count each solution, and the groups' own solutions are
// counted when they are built, so the budget still bounds every row held in
// memory.
func evalBGPComponents(ec *evalCtx, g *rdflibgo.Graph, comps [][]Triple, bindings map[string]rdflibgo.Term, prefixes map[string]string, limit int) []map[string]rdflibgo.Term {
	// With a limit, row k of the product uses solution number <= k of every
	// group (the odometer below moves one step per row), so the first limit
	// rows never read past a group's first limit solutions. A group cut there
	// is a prefix of the full one, in the same order; an empty group is still
	// empty, so the answer stays empty.
	parts := make([][]map[string]rdflibgo.Term, len(comps))
	for i, c := range comps {
		parts[i] = collectBGPSolutions(ec, g, c, bindings, prefixes, limit)
		if len(parts[i]) == 0 || ec.stop() {
			return nil
		}
	}
	var results []map[string]rdflibgo.Term
	idx := make([]int, len(parts))
	for {
		if ec.stop() {
			return nil
		}
		// Every group's solution carries bindings, and the groups share no
		// other variable, so the row is the union of one solution of each.
		row := copyBindings(parts[0][idx[0]])
		for i := 1; i < len(parts); i++ {
			for k, v := range parts[i][idx[i]] {
				row[k] = v
			}
		}
		results = append(results, row)
		if ec.produced(1) {
			return nil
		}
		if limit > 0 && len(results) >= limit {
			return results
		}
		// Advance the last group fastest, like the innermost loop.
		i := len(idx) - 1
		for ; i >= 0; i-- {
			idx[i]++
			if idx[i] < len(parts[i]) {
				break
			}
			idx[i] = 0
		}
		if i < 0 {
			return results
		}
	}
}
