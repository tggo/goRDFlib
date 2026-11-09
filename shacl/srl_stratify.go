package shacl

import "fmt"

// srlLayer is a stratification layer: the run-once rules are evaluated once
// at the start of the layer, then the general rules until nothing new is
// inferred.
type srlLayer struct {
	once    []int
	general []int
}

// Stratify partitions the rules of rs into stratification layers following
// SPARQL-RL "Stratification": a rule that negates, groups over or is
// run-once relative to another rule is placed above it. Each element of the
// result is a layer holding rule indices (run-once rules first, in the order
// they must run). It fails with ErrSRLNotStratifiable if a cycle in the rule
// dependency graph contains a closed dependency.
func Stratify(rs *RuleSet) ([][]int, error) {
	layers, err := stratify(rs)
	if err != nil {
		return nil, err
	}
	out := make([][]int, len(layers))
	for i, l := range layers {
		out[i] = append(append([]int(nil), l.once...), l.general...)
	}
	return out, nil
}

// srlIsRunOnce: a rule is run-once if its body has an assignment or a group,
// or its head has a blank node or a triple term in which a variable occurs.
func srlIsRunOnce(r SRLRule) bool {
	for _, e := range r.Body {
		if e.Kind == SRLBodyBind || e.Kind == SRLBodyGroup {
			return true
		}
	}
	for _, t := range r.Head {
		for _, term := range []SRLTerm{t.Subject, t.Predicate, t.Object} {
			if srlTemplateCreates(term, false) {
				return true
			}
		}
	}
	return false
}

// srlTemplateCreates reports whether a template term mints a blank node, or
// is a triple term with a variable (inTriple says we are inside one).
func srlTemplateCreates(t SRLTerm, inTriple bool) bool {
	switch t.Kind {
	case SRLTermBlankNode:
		return true
	case SRLTermVariable:
		return inTriple
	case SRLTermTripleTerm:
		for _, c := range []*SRLTerm{t.TTSubject, t.TTPredicate, t.TTObject} {
			if c != nil && srlTemplateCreates(*c, true) {
				return true
			}
		}
	}
	return false
}

type srlEdge struct{ from, to int }

func stratify(rs *RuleSet) ([]srlLayer, error) {
	n := len(rs.Rules)
	if n == 0 {
		return nil, nil
	}
	closed := buildDependencies(rs)

	// The stratification condition: no cycle contains a closed dependency.
	adj := make([][]int, n)
	for e := range closed {
		adj[e.from] = append(adj[e.from], e.to)
	}
	for e, isClosed := range closed {
		if isClosed && srlReaches(adj, e.to, e.from) {
			return nil, srlCycleError(rs, e)
		}
	}

	// The stratification algorithm of the specification.
	stratum := make([]int, n)
	for changed := true; changed; {
		changed = false
		for e, isClosed := range closed {
			need := stratum[e.to]
			if isClosed {
				need++
			}
			if stratum[e.from] < need {
				stratum[e.from] = need
				changed = true
			}
		}
	}
	maxS := 0
	for _, s := range stratum {
		if s > maxS {
			maxS = s
		}
	}
	layers := make([]srlLayer, maxS+1)
	for i, s := range stratum {
		if srlIsRunOnce(rs.Rules[i]) {
			layers[s].once = append(layers[s].once, i)
		} else {
			layers[s].general = append(layers[s].general, i)
		}
	}
	return layers, nil
}

// srlReaches reports whether to is reachable from from (from == to counts
// only through a cycle, so a self-loop is found by the first step).
func srlReaches(adj [][]int, from, to int) bool {
	seen := make([]bool, len(adj))
	stack := []int{from}
	for len(stack) > 0 {
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if v == to {
			return true
		}
		if seen[v] {
			continue
		}
		seen[v] = true
		stack = append(stack, adj[v]...)
	}
	return false
}

func srlCycleError(rs *RuleSet, e srlEdge) error {
	name := func(i int) string {
		if rs.Rules[i].ID != "" {
			return fmt.Sprintf("rule %d <%s>", i+1, rs.Rules[i].ID)
		}
		return fmt.Sprintf("rule %d", i+1)
	}
	why := "a negation, a group, an assignment, or a head that creates blank nodes or triple terms"
	if e.from == e.to {
		return fmt.Errorf("%w: %s depends on its own output through %s; "+
			"a rule cannot read what it creates there", ErrSRLNotStratifiable, name(e.from), why)
	}
	return fmt.Errorf("%w: %s depends through %s on %s, which depends on it in turn; "+
		"break the cycle or match the base graph with NOT DATA / WHERE DATA",
		ErrSRLNotStratifiable, name(e.from), why, name(e.to))
}

// buildDependencies computes the rule dependency graph: an edge R1 -> R2 when
// a triple pattern of R1's body can match a triple R2's head generates; the
// value says whether the dependency is closed.
func buildDependencies(rs *RuleSet) map[srlEdge]bool {
	type dep struct {
		tp     SRLTriple
		closed bool
	}
	edges := map[srlEdge]bool{}
	for i, r1 := range rs.Rules {
		if r1.BaseGraphOnly {
			continue // matched against the base graph: no dependencies
		}
		var deps []dep
		once := srlIsRunOnce(r1)
		for _, e := range r1.Body {
			switch e.Kind {
			case SRLBodyTriple:
				deps = append(deps, dep{e.Triple, once})
			case SRLBodyNot:
				if e.NotData {
					continue
				}
				for _, ne := range e.NotBody {
					if ne.Kind == SRLBodyTriple {
						deps = append(deps, dep{ne.Triple, true})
					}
				}
			case SRLBodyGroup:
				for _, ge := range e.GroupBody {
					if ge.Kind == SRLBodyTriple {
						deps = append(deps, dep{ge.Triple, true})
					}
				}
			}
		}
		for _, d := range deps {
			for j, r2 := range rs.Rules {
				for _, tt := range r2.Head {
					if srlPatternMatchesTemplate(d.tp, tt) {
						k := srlEdge{i, j}
						// A closed dependency overrides an open one.
						edges[k] = edges[k] || d.closed
					}
				}
			}
		}
	}
	return edges
}

// srlPatternMatchesTemplate: the template can generate a triple the pattern
// matches. Variables and (in patterns) blank nodes match anything.
func srlPatternMatchesTemplate(tp, tt SRLTriple) bool {
	if tp.Path != nil {
		// A path's subject and object are not the triple's: compare the
		// predicates only, and conservatively.
		if tt.Predicate.Kind == SRLTermVariable {
			return true
		}
		return srlPathMentions(tp.Path, tt.Predicate.Value)
	}
	return srlTermsUnify(tp.Subject, tt.Subject) &&
		srlTermsUnify(tp.Predicate, tt.Predicate) &&
		srlTermsUnify(tp.Object, tt.Object)
}

func srlPathMentions(p *SRLPath, iri string) bool {
	if p.Kind == SRLPathIRI {
		return p.IRI == iri
	}
	for _, e := range p.Elems {
		if srlPathMentions(e, iri) {
			return true
		}
	}
	return false
}

// srlTermsUnify reports whether two terms can denote the same RDF term.
func srlTermsUnify(a, b SRLTerm) bool {
	if a.Kind == SRLTermVariable || a.Kind == SRLTermBlankNode ||
		b.Kind == SRLTermVariable || b.Kind == SRLTermBlankNode {
		return true
	}
	if a.Kind != b.Kind {
		return false
	}
	switch a.Kind {
	case SRLTermIRI:
		return a.Value == b.Value
	case SRLTermLiteral:
		return a.Value == b.Value && a.Datatype == b.Datatype && a.Language == b.Language
	case SRLTermTripleTerm:
		return srlTermsUnify(*a.TTSubject, *b.TTSubject) &&
			srlTermsUnify(*a.TTPredicate, *b.TTPredicate) &&
			srlTermsUnify(*a.TTObject, *b.TTObject)
	}
	return false
}
