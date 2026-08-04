package reasoning

import (
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

// applyPropertyRules implements prp-symp, prp-trp, prp-inv, prp-eqp, prp-fp, prp-ifp,
// prp-spo1, prp-key, cax-eqc.
func (e *owlrlEngine) applyPropertyRules(allTriples []ruleTriple, emit emitFunc) {
	rdfType := namespace.RDF.Type
	sameAs := namespace.OWL.SameAs

	// Per-triple rules
	for _, t := range allTriples {
		pk := term.TermKey(t.p)

		// prp-symp: ?p a owl:SymmetricProperty, ?x ?p ?y → ?y ?p ?x
		if _, ok := e.symmetricProps[pk]; ok {
			if oSubj, ok := t.o.(term.Subject); ok {
				emit(oSubj, t.p, t.s)
			}
		}

		// prp-inv1/2: ?p1 owl:inverseOf ?p2, ?x ?p1 ?y → ?y ?p2 ?x
		if inverses, ok := e.inverseOf[pk]; ok {
			if oSubj, ok := t.o.(term.Subject); ok {
				for _, inv := range inverses {
					emit(oSubj, inv, t.s)
				}
			}
		}

		// prp-eqp1/2: ?p1 owl:equivalentProperty ?p2, ?x ?p1 ?y → ?x ?p2 ?y
		if equivs, ok := e.equivProp[pk]; ok {
			for _, eq := range equivs {
				emit(t.s, eq, t.o)
			}
		}

		// cax-eqc1/2: ?C1 owl:equivalentClass ?C2, ?x rdf:type ?C1 → ?x rdf:type ?C2
		// C1 is keyed directly so that an anonymous class expression matches.
		if pk == term.TermKey(rdfType) {
			c1k := term.TermKey(t.o)
			if equivs, ok := e.equivClass[c1k]; ok {
				for _, c2 := range equivs {
					emit(t.s, rdfType, c2)
				}
			}
		}
	}

	// prp-trp: transitive properties
	e.applyTransitiveRules(allTriples, emit)

	// prp-fp: functional properties
	e.applyFunctionalRules(allTriples, emit, sameAs)

	// prp-ifp: inverse functional properties
	e.applyInverseFunctionalRules(allTriples, emit, sameAs)

	// prp-spo1: property chain axioms
	e.applyPropertyChainRules(allTriples, emit)

	// prp-key: hasKey
	e.applyHasKeyRules(allTriples, emit, sameAs)
}

func (e *owlrlEngine) applyTransitiveRules(allTriples []ruleTriple, emit emitFunc) {
	if len(e.transitiveProps) == 0 {
		return
	}
	type propEdges struct {
		pred term.URIRef
		fwd  map[string][]term.Term
	}
	propMap := make(map[string]*propEdges, len(e.transitiveProps))
	for _, t := range allTriples {
		pk := term.TermKey(t.p)
		if _, ok := e.transitiveProps[pk]; !ok {
			continue
		}
		pe := propMap[pk]
		if pe == nil {
			pe = &propEdges{pred: t.p, fwd: make(map[string][]term.Term)}
			propMap[pk] = pe
		}
		sk := term.TermKey(t.s)
		pe.fwd[sk] = append(pe.fwd[sk], t.o)
	}

	for _, pe := range propMap {
		for _, t := range allTriples {
			if term.TermKey(t.p) != term.TermKey(pe.pred) {
				continue
			}
			visited := make(map[string]struct{})
			visited[term.TermKey(t.o)] = struct{}{}
			var queue []term.Term
			if nexts, ok := pe.fwd[term.TermKey(t.o)]; ok {
				queue = append(queue, nexts...)
			}
			for len(queue) > 0 {
				cur := queue[0]
				queue = queue[1:]
				ck := term.TermKey(cur)
				if _, seen := visited[ck]; seen {
					continue
				}
				visited[ck] = struct{}{}
				emit(t.s, pe.pred, cur)
				if nexts, ok := pe.fwd[ck]; ok {
					queue = append(queue, nexts...)
				}
			}
		}
	}
}

func (e *owlrlEngine) applyFunctionalRules(allTriples []ruleTriple, emit emitFunc, sameAs term.URIRef) {
	if len(e.functionalProps) == 0 {
		return
	}
	type psKey struct{ prop, subj string }
	groups := make(map[psKey][]term.Term)
	for _, t := range allTriples {
		pk := term.TermKey(t.p)
		if _, ok := e.functionalProps[pk]; !ok {
			continue
		}
		k := psKey{pk, term.TermKey(t.s)}
		groups[k] = append(groups[k], t.o)
	}
	for _, objs := range groups {
		if len(objs) < 2 {
			continue
		}
		for i := 1; i < len(objs); i++ {
			if s0, ok := objs[0].(term.Subject); ok {
				if si, ok := objs[i].(term.Subject); ok {
					emit(s0, sameAs, objs[i])
					emit(si, sameAs, objs[0])
				}
			}
		}
	}
}

func (e *owlrlEngine) applyInverseFunctionalRules(allTriples []ruleTriple, emit emitFunc, sameAs term.URIRef) {
	if len(e.invFuncProps) == 0 {
		return
	}
	type poKey struct{ prop, obj string }
	groups := make(map[poKey][]term.Subject)
	for _, t := range allTriples {
		pk := term.TermKey(t.p)
		if _, ok := e.invFuncProps[pk]; !ok {
			continue
		}
		k := poKey{pk, term.TermKey(t.o)}
		groups[k] = append(groups[k], t.s)
	}
	for _, subjs := range groups {
		if len(subjs) < 2 {
			continue
		}
		for i := 1; i < len(subjs); i++ {
			emit(subjs[0], sameAs, subjs[i])
			emit(subjs[i], sameAs, subjs[0])
		}
	}
}

// applyPropertyChainRules implements prp-spo1:
// p owl:propertyChainAxiom (p1 p2 ... pN), x p1 y1, y1 p2 y2, ..., y(N-1) pN yN → x p yN
func (e *owlrlEngine) applyPropertyChainRules(allTriples []ruleTriple, emit emitFunc) {
	if len(e.propChainAxiom) == 0 {
		return
	}
	// Build property → triples index
	byPred := make(map[string][]ruleTriple)
	for _, t := range allTriples {
		pk := term.TermKey(t.p)
		byPred[pk] = append(byPred[pk], t)
	}

	for pk, chains := range e.propChainAxiom {
		prop, ok := e.termForKey(pk)
		if !ok {
			continue
		}
		for _, chain := range chains {
			e.evalChain(chain, 0, byPred, nil, prop, emit)
		}
	}
}

// termForKey finds a URIRef for a property key by scanning propertyChainAxiom keys.
func (e *owlrlEngine) termForKey(key string) (term.URIRef, bool) {
	// Scan allTriples looking for a predicate with this key.
	// This is called only for propChainAxiom keys, which we stored.
	chainAxiom := namespace.OWL.PropertyChainAxiom
	var result term.URIRef
	found := false
	e.g.Triples(nil, &chainAxiom, nil)(func(t term.Triple) bool {
		if u, ok := t.Subject.(term.URIRef); ok && term.TermKey(u) == key {
			result = u
			found = true
			return false
		}
		return true
	})
	return result, found
}

// evalChain recursively evaluates a property chain starting from step idx.
// subjects is the set of current starting nodes (nil = use all subjects of chain[0]).
func (e *owlrlEngine) evalChain(chain []term.URIRef, idx int, byPred map[string][]ruleTriple, subjects []term.Subject, resultProp term.URIRef, emit emitFunc) {
	if idx >= len(chain) {
		return
	}
	pk := term.TermKey(chain[idx])
	triples := byPred[pk]

	if idx == 0 {
		// First step: collect all (subject, object) pairs
		for _, t := range triples {
			if idx == len(chain)-1 {
				// Single-step chain (shouldn't happen, chains have >= 2 elements)
				emit(t.s, resultProp, t.o)
			} else {
				if oSubj, ok := t.o.(term.Subject); ok {
					e.evalChainStep(chain, idx+1, byPred, t.s, oSubj, resultProp, emit)
				}
			}
		}
	} else {
		// Shouldn't be called directly at idx > 0 from here
		return
	}
}

func (e *owlrlEngine) evalChainStep(chain []term.URIRef, idx int, byPred map[string][]ruleTriple, startSubj term.Subject, curNode term.Subject, resultProp term.URIRef, emit emitFunc) {
	if idx >= len(chain) {
		return
	}
	pk := term.TermKey(chain[idx])
	triples := byPred[pk]

	curKey := term.TermKey(curNode)
	for _, t := range triples {
		if term.TermKey(t.s) != curKey {
			continue
		}
		if idx == len(chain)-1 {
			// Last step: emit startSubj resultProp t.o
			emit(startSubj, resultProp, t.o)
		} else {
			if oSubj, ok := t.o.(term.Subject); ok {
				e.evalChainStep(chain, idx+1, byPred, startSubj, oSubj, resultProp, emit)
			}
		}
	}
}

// applyHasKeyRules implements prp-key:
// C owl:hasKey (p1 ... pN), x rdf:type C, y rdf:type C,
// x p1 z1, y p1 z1, ..., x pN zN, y pN zN → x owl:sameAs y
func (e *owlrlEngine) applyHasKeyRules(allTriples []ruleTriple, emit emitFunc, sameAs term.URIRef) {
	if len(e.hasKeyIndex) == 0 {
		return
	}
	rdfType := namespace.RDF.Type

	// Build type index: class key → instances
	typeIndex := make(map[string][]term.Subject)
	// Build property value index: (subject key, property key) → values
	type spKey struct{ subj, prop string }
	propVals := make(map[spKey][]term.Term)

	for _, t := range allTriples {
		pk := term.TermKey(t.p)
		if pk == term.TermKey(rdfType) {
			ck := term.TermKey(t.o)
			if _, ok := e.hasKeyIndex[ck]; ok {
				typeIndex[ck] = append(typeIndex[ck], t.s)
			}
		}
		// Index all property values (we don't know which are key properties in advance)
		sk := term.TermKey(t.s)
		k := spKey{sk, pk}
		propVals[k] = append(propVals[k], t.o)
	}

	for ck, keyProps := range e.hasKeyIndex {
		instances := typeIndex[ck]
		if len(instances) < 2 {
			continue
		}
		// For each pair of instances, check if they agree on all key properties
		for i := 0; i < len(instances); i++ {
			for j := i + 1; j < len(instances); j++ {
				ik := term.TermKey(instances[i])
				jk := term.TermKey(instances[j])
				if ik == jk {
					continue
				}
				allMatch := true
				for _, kp := range keyProps {
					kpk := term.TermKey(kp)
					iVals := propVals[spKey{ik, kpk}]
					jVals := propVals[spKey{jk, kpk}]
					if !hasCommonValue(iVals, jVals) {
						allMatch = false
						break
					}
				}
				if allMatch {
					emit(instances[i], sameAs, instances[j])
					emit(instances[j], sameAs, instances[i])
				}
			}
		}
	}
}

func hasCommonValue(a, b []term.Term) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(a))
	for _, v := range a {
		set[term.TermKey(v)] = struct{}{}
	}
	for _, v := range b {
		if _, ok := set[term.TermKey(v)]; ok {
			return true
		}
	}
	return false
}
