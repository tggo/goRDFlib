package reasoning

import (
	"fmt"

	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

// checkConsistency runs all consistency checks after closure is complete.
func (e *owlrlEngine) checkConsistency() {
	rdfType := namespace.RDF.Type

	// Build indexes for consistency checks
	// type index: subject key → set of class keys
	typeIndex := make(map[string]map[string]struct{})
	// property triples: (subject key, property key) → object keys
	type spKey struct{ subj, prop string }
	propObjs := make(map[spKey][]term.Term)

	e.stop.triples(e.g, nil, nil, nil, func(t term.Triple) bool {
		sk := term.TermKey(t.Subject)
		pk := term.TermKey(t.Predicate)
		if pk == term.TermKey(rdfType) {
			if typeIndex[sk] == nil {
				typeIndex[sk] = make(map[string]struct{})
			}
			typeIndex[sk][term.TermKey(t.Object)] = struct{}{}
		}
		propObjs[spKey{sk, pk}] = append(propObjs[spKey{sk, pk}], t.Object)
		return true
	})

	// --- cls-nothing2: x rdf:type owl:Nothing → inconsistency ---
	nothingKey := term.TermKey(namespace.OWL.Nothing)
	for sk, types := range typeIndex {
		if _, ok := types[nothingKey]; ok {
			e.inconsistencies = append(e.inconsistencies, Inconsistency{
				Rule:    "cls-nothing2",
				Message: fmt.Sprintf("individual %s is typed as owl:Nothing", sk),
			})
		}
	}

	// --- prp-irp: p a owl:IrreflexiveProperty, x p x → inconsistency ---
	e.stop.triples(e.g, nil, nil, nil, func(t term.Triple) bool {
		pk := term.TermKey(t.Predicate)
		if _, ok := e.irreflexiveProps[pk]; ok {
			if term.TermKey(t.Subject) == term.TermKey(t.Object) {
				e.inconsistencies = append(e.inconsistencies, Inconsistency{
					Rule:    "prp-irp",
					Message: fmt.Sprintf("%s %s %s violates irreflexivity", t.Subject.N3(), t.Predicate.N3(), t.Object.N3()),
				})
			}
		}
		return true
	})

	// --- prp-asyp: p a owl:AsymmetricProperty, x p y, y p x → inconsistency ---
	e.stop.triples(e.g, nil, nil, nil, func(t term.Triple) bool {
		pk := term.TermKey(t.Predicate)
		if _, ok := e.asymmetricProps[pk]; !ok {
			return true
		}
		if oSubj, ok := t.Object.(term.Subject); ok {
			if e.stop.contains(e.g, oSubj, t.Predicate, t.Subject) {
				// Only report once (for the pair where s < o lexicographically)
				if term.TermKey(t.Subject) < term.TermKey(t.Object) {
					e.inconsistencies = append(e.inconsistencies, Inconsistency{
						Rule:    "prp-asyp",
						Message: fmt.Sprintf("%s %s %s and reverse violates asymmetry", t.Subject.N3(), t.Predicate.N3(), t.Object.N3()),
					})
				}
			}
		}
		return true
	})

	// --- prp-pdw: p1 owl:propertyDisjointWith p2, x p1 y, x p2 y → inconsistency ---
	e.stop.triples(e.g, nil, nil, nil, func(t term.Triple) bool {
		pk := term.TermKey(t.Predicate)
		disjoints, ok := e.propDisjointWith[pk]
		if !ok {
			return true
		}
		for _, p2 := range disjoints {
			if e.stop.contains(e.g, t.Subject, p2, t.Object) {
				// Only report once (for the pair where p1 < p2 lexicographically)
				if pk < term.TermKey(p2) {
					e.inconsistencies = append(e.inconsistencies, Inconsistency{
						Rule:    "prp-pdw",
						Message: fmt.Sprintf("%s has disjoint properties %s and %s with same value %s", t.Subject.N3(), t.Predicate.N3(), p2.N3(), t.Object.N3()),
					})
				}
			}
		}
		return true
	})

	// --- prp-adp: AllDisjointProperties members (p1...pN), x pi y, x pj y → inconsistency ---
	for _, group := range e.allDisjointProps {
		for i := 0; i < len(group); i++ {
			for j := i + 1; j < len(group); j++ {
				pi := group[i]
				pj := group[j]
				pik := term.TermKey(pi)
				// Check if any subject has both pi and pj to the same object
				for key, objs := range propObjs {
					if key.prop != pik {
						continue
					}
					for _, obj := range objs {
						pjk := term.TermKey(pj)
						for _, t := range propObjs[spKey{key.subj, pjk}] {
							if term.TermKey(t) == term.TermKey(obj) {
								e.inconsistencies = append(e.inconsistencies, Inconsistency{
									Rule:    "prp-adp",
									Message: fmt.Sprintf("all-disjoint properties %s and %s share value for same subject", pi.N3(), pj.N3()),
								})
							}
						}
					}
				}
			}
		}
	}

	// --- cax-dw: C1 owl:disjointWith C2, x rdf:type C1, x rdf:type C2 → inconsistency ---
	for sk, types := range typeIndex {
		for ck := range types {
			disjoints, ok := e.disjointWith[ck]
			if !ok {
				continue
			}
			for _, c2 := range disjoints {
				c2k := term.TermKey(c2)
				if _, ok := types[c2k]; ok {
					if ck < c2k { // report once per pair
						e.inconsistencies = append(e.inconsistencies, Inconsistency{
							Rule:    "cax-dw",
							Message: fmt.Sprintf("individual %s is member of disjoint classes %s and %s", sk, ck, c2k),
						})
					}
				}
			}
		}
	}

	// --- cax-adc: AllDisjointClasses members (C1...CN), x rdf:type Ci, x rdf:type Cj → inconsistency ---
	for _, group := range e.allDisjointClass {
		for sk, types := range typeIndex {
			var memberOf []term.URIRef
			for _, c := range group {
				if _, ok := types[term.TermKey(c)]; ok {
					memberOf = append(memberOf, c)
				}
			}
			if len(memberOf) >= 2 {
				e.inconsistencies = append(e.inconsistencies, Inconsistency{
					Rule:    "cax-adc",
					Message: fmt.Sprintf("individual %s is member of multiple all-disjoint classes: %s, %s", sk, memberOf[0].N3(), memberOf[1].N3()),
				})
			}
		}
	}

	// --- eq-diff1: x owl:sameAs y, x owl:differentFrom y → inconsistency ---
	for sk, diffs := range e.differentFrom {
		for _, d := range diffs {
			dk := term.TermKey(d)
			// Check if they're in the same sameAs equivalence class
			if e.uf.size() > 0 {
				rootS := e.uf.findKey(sk)
				rootD := e.uf.findKey(dk)
				if rootS == rootD && rootS != "" {
					if sk < dk {
						e.inconsistencies = append(e.inconsistencies, Inconsistency{
							Rule:    "eq-diff1",
							Message: fmt.Sprintf("%s is both owl:sameAs and owl:differentFrom %s", sk, dk),
						})
					}
				}
			}
		}
	}

	// --- cls-com: C1 owl:complementOf C2, x rdf:type C1, x rdf:type C2 → inconsistency ---
	for c1k, c2 := range e.complementOf {
		c2k := term.TermKey(c2)
		for sk, types := range typeIndex {
			_, hasC1 := types[c1k]
			_, hasC2 := types[c2k]
			if hasC1 && hasC2 {
				e.inconsistencies = append(e.inconsistencies, Inconsistency{
					Rule:    "cls-com",
					Message: fmt.Sprintf("individual %s is member of complementary classes %s and %s", sk, c1k, c2k),
				})
			}
		}
	}

	// --- cls-maxc1: restriction maxCard 0, onProperty p, u rdf:type restriction, u p y → inconsistency ---
	// --- cls-maxqc1/2: restriction maxQualCard 0, ... → inconsistency ---
	for _, r := range e.restrictions {
		rk := term.TermKey(r.node)
		pk := term.TermKey(r.onProperty)

		if r.maxCard == 0 {
			for sk, types := range typeIndex {
				if _, ok := types[rk]; !ok {
					continue
				}
				objs := propObjs[spKey{sk, pk}]
				if len(objs) > 0 {
					e.inconsistencies = append(e.inconsistencies, Inconsistency{
						Rule:    "cls-maxc1",
						Message: fmt.Sprintf("individual %s has property %s but maxCardinality is 0", sk, r.onProperty.N3()),
					})
				}
			}
		}

		if r.maxQualCard == 0 && r.onClass != nil {
			onClassKey := term.TermKey(r.onClass)
			isThing := onClassKey == term.TermKey(namespace.OWL.Thing)
			for sk, types := range typeIndex {
				if _, ok := types[rk]; !ok {
					continue
				}
				objs := propObjs[spKey{sk, pk}]
				for _, obj := range objs {
					if isThing {
						e.inconsistencies = append(e.inconsistencies, Inconsistency{
							Rule:    "cls-maxqc2",
							Message: fmt.Sprintf("individual %s has property %s but maxQualifiedCardinality is 0 on owl:Thing", sk, r.onProperty.N3()),
						})
						break
					}
					objTypes := typeIndex[term.TermKey(obj)]
					if _, ok := objTypes[onClassKey]; ok {
						e.inconsistencies = append(e.inconsistencies, Inconsistency{
							Rule:    "cls-maxqc1",
							Message: fmt.Sprintf("individual %s has property %s to qualified individual but maxQualifiedCardinality is 0", sk, r.onProperty.N3()),
						})
						break
					}
				}
			}
		}
	}

	// --- prp-npa1/2: NegativePropertyAssertion(source, property, target) and source property target → inconsistency ---
	for _, npa := range e.negPropAsserts {
		sk := term.TermKey(npa.source)
		pk := term.TermKey(npa.property)
		objs := propObjs[spKey{sk, pk}]
		for _, obj := range objs {
			if npa.targetI != nil && term.TermKey(obj) == term.TermKey(npa.targetI) {
				e.inconsistencies = append(e.inconsistencies, Inconsistency{
					Rule:    "prp-npa1",
					Message: fmt.Sprintf("negative property assertion violated: %s %s %s", npa.source.N3(), npa.property.N3(), npa.targetI.N3()),
				})
			}
			if npa.targetV != nil && term.TermKey(obj) == term.TermKey(npa.targetV) {
				e.inconsistencies = append(e.inconsistencies, Inconsistency{
					Rule:    "prp-npa2",
					Message: fmt.Sprintf("negative property assertion violated: %s %s %s", npa.source.N3(), npa.property.N3(), npa.targetV.N3()),
				})
			}
		}
	}
}
