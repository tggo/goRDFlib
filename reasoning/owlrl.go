package reasoning

import (
	"strconv"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

// Inconsistency represents a detected OWL 2 RL inconsistency.
type Inconsistency struct {
	Rule    string // OWL 2 RL rule identifier (e.g., "prp-irp", "cax-dw")
	Message string // Human-readable description
}

// OWLRLClosure applies OWL 2 RL closure rules to the graph, adding inferred triples.
// Returns the total number of triples added.
//
// Not safe for concurrent use.
func OWLRLClosure(g *graph.Graph) int {
	n, _ := OWLRLClosureCheck(g)
	return n
}

// OWLRLClosureCheck applies OWL 2 RL closure rules and checks consistency.
// Returns the total number of triples added and any detected inconsistencies.
//
// Not safe for concurrent use.
func OWLRLClosureCheck(g *graph.Graph) (int, []Inconsistency) {
	e := newOWLRLEngine(g)
	return e.run()
}

// ruleTriple is a snapshot triple used during rule application.
type ruleTriple struct {
	s term.Subject
	p term.URIRef
	o term.Term
}

// owlRestriction holds parsed OWL restriction data.
type owlRestriction struct {
	node           term.Subject // the restriction bnode/URIRef
	onProperty     term.URIRef  // owl:onProperty
	hasOnProperty  bool
	someValuesFrom term.Term // owl:someValuesFrom
	allValuesFrom  term.Term // owl:allValuesFrom
	hasValue       term.Term // owl:hasValue
	hasSelf        bool      // owl:hasSelf "true"
	maxCard        int       // owl:maxCardinality (-1 = not set)
	maxQualCard    int       // owl:maxQualifiedCardinality (-1 = not set)
	onClass        term.Term // owl:onClass (for qualified cardinality)
	complementOf   term.Term // owl:complementOf
}

// negPropAssertion holds a parsed owl:NegativePropertyAssertion.
type negPropAssertion struct {
	source   term.Subject
	property term.URIRef
	targetI  term.Subject // targetIndividual (if set)
	targetV  term.Term    // targetValue (if set)
}

// owlrlEngine holds schema indexes and dedup state for OWL 2 RL closure.
type owlrlEngine struct {
	g   *graph.Graph
	ded *dedupSet

	// Phase 1: Core property indexes
	symmetricProps  map[string]struct{}       // owl:SymmetricProperty
	transitiveProps map[string]struct{}       // owl:TransitiveProperty
	functionalProps map[string]struct{}       // owl:FunctionalProperty
	invFuncProps    map[string]struct{}       // owl:InverseFunctionalProperty
	inverseOf       map[string][]term.URIRef  // property key → inverse properties
	equivProp       map[string][]term.URIRef  // property key → equivalent properties
	equivClass      map[string][]term.Subject // class key → equivalent classes (may be anonymous)

	// Phase 2: Extended property indexes
	irreflexiveProps map[string]struct{}        // owl:IrreflexiveProperty
	asymmetricProps  map[string]struct{}        // owl:AsymmetricProperty
	propDisjointWith map[string][]term.URIRef   // property key → disjoint properties
	allDisjointProps [][]term.URIRef            // groups of mutually disjoint properties
	propChainAxiom   map[string][][]term.URIRef // property key → chains of properties
	hasKeyIndex      map[string][]term.URIRef   // class key → key properties

	// Phase 3: Class restriction indexes
	restrictions   []*owlRestriction            // all parsed restrictions
	restrByProp    map[string][]*owlRestriction // property key → restrictions
	intersectionOf map[string][]term.Term       // class key → intersection member classes
	unionOf        map[string][]term.Term       // class key → union member classes
	complementOf   map[string]term.Term         // class key → complement class
	oneOfMembers   map[string][]term.Term       // class key → enumerated members

	// Phase 4: Class axiom indexes
	disjointWith     map[string][]term.URIRef // class key → disjoint classes
	allDisjointClass [][]term.URIRef          // groups of mutually disjoint classes

	// Phase 5: Consistency indexes
	differentFrom  map[string][]term.Term // term key → differentFrom terms
	negPropAsserts []*negPropAssertion

	// Union-find for owl:sameAs
	uf *unionFind

	// Inconsistencies collected during rule application
	inconsistencies []Inconsistency
}

func newOWLRLEngine(g *graph.Graph) *owlrlEngine {
	return &owlrlEngine{
		g:   g,
		ded: newDedupSet(g.Len()),
	}
}

func (e *owlrlEngine) run() (int, []Inconsistency) {
	// Populate dedup set
	e.g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		e.ded.addNew(t.Subject, t.Predicate, t.Object)
		return true
	})

	e.buildSchemaIndexes()
	totalAdded := 0

	for {
		newTriples := e.applyRules()
		if len(newTriples) == 0 {
			break
		}

		hasSchemaTriples := false
		for _, t := range newTriples {
			e.g.Add(t.Subject, t.Predicate, t.Object)
			if e.isSchemaTriple(t) {
				hasSchemaTriples = true
			}
		}
		totalAdded += len(newTriples)

		if hasSchemaTriples {
			e.buildSchemaIndexes()
		}
	}

	// Run final consistency checks
	e.checkConsistency()

	return totalAdded, e.inconsistencies
}

// isSchemaTriple returns true if the triple could affect schema indexes.
func (e *owlrlEngine) isSchemaTriple(t term.Triple) bool {
	pk := term.TermKey(t.Predicate)
	switch pk {
	case term.TermKey(namespace.RDF.Type):
		ok := term.TermKey(t.Object)
		switch ok {
		case term.TermKey(namespace.OWL.SymmetricProperty),
			term.TermKey(namespace.OWL.TransitiveProperty),
			term.TermKey(namespace.OWL.FunctionalProperty),
			term.TermKey(namespace.OWL.InverseFunctionalProperty),
			term.TermKey(namespace.OWL.IrreflexiveProperty),
			term.TermKey(namespace.OWL.AsymmetricProperty),
			term.TermKey(namespace.OWL.Restriction),
			term.TermKey(namespace.OWL.AllDisjointProperties),
			term.TermKey(namespace.OWL.AllDisjointClasses),
			term.TermKey(namespace.OWL.NegativePropertyAssertion):
			return true
		}
	case term.TermKey(namespace.OWL.InverseOf),
		term.TermKey(namespace.OWL.EquivalentProperty),
		term.TermKey(namespace.OWL.EquivalentClass),
		term.TermKey(namespace.OWL.SameAs),
		term.TermKey(namespace.OWL.PropertyDisjointWith),
		term.TermKey(namespace.OWL.PropertyChainAxiom),
		term.TermKey(namespace.OWL.HasKey),
		term.TermKey(namespace.OWL.OnProperty),
		term.TermKey(namespace.OWL.SomeValuesFrom),
		term.TermKey(namespace.OWL.AllValuesFrom),
		term.TermKey(namespace.OWL.HasValue),
		term.TermKey(namespace.OWL.HasSelf),
		term.TermKey(namespace.OWL.MaxCardinality),
		term.TermKey(namespace.OWL.MaxQualifiedCardinality),
		term.TermKey(namespace.OWL.IntersectionOf),
		term.TermKey(namespace.OWL.UnionOf),
		term.TermKey(namespace.OWL.ComplementOf),
		term.TermKey(namespace.OWL.OneOf),
		term.TermKey(namespace.OWL.DisjointWith),
		term.TermKey(namespace.OWL.DifferentFrom):
		return true
	}
	return false
}

func (e *owlrlEngine) buildSchemaIndexes() {
	// Phase 1
	e.symmetricProps = make(map[string]struct{})
	e.transitiveProps = make(map[string]struct{})
	e.functionalProps = make(map[string]struct{})
	e.invFuncProps = make(map[string]struct{})
	e.inverseOf = make(map[string][]term.URIRef)
	e.equivProp = make(map[string][]term.URIRef)
	e.equivClass = make(map[string][]term.Subject)
	e.uf = newUnionFind()

	// Phase 2
	e.irreflexiveProps = make(map[string]struct{})
	e.asymmetricProps = make(map[string]struct{})
	e.propDisjointWith = make(map[string][]term.URIRef)
	e.allDisjointProps = nil
	e.propChainAxiom = make(map[string][][]term.URIRef)
	e.hasKeyIndex = make(map[string][]term.URIRef)

	// Phase 3
	e.restrictions = nil
	e.restrByProp = make(map[string][]*owlRestriction)
	e.intersectionOf = make(map[string][]term.Term)
	e.unionOf = make(map[string][]term.Term)
	e.complementOf = make(map[string]term.Term)
	e.oneOfMembers = make(map[string][]term.Term)

	// Phase 4
	e.disjointWith = make(map[string][]term.URIRef)
	e.allDisjointClass = nil

	// Phase 5
	e.differentFrom = make(map[string][]term.Term)
	e.negPropAsserts = nil

	rdfType := namespace.RDF.Type

	// === rdf:type scan ===
	restrictionNodes := make(map[string]term.Subject) // key → restriction bnode
	var allDisjPropNodes []term.Subject
	var allDisjClassNodes []term.Subject
	var negPANodes []term.Subject

	e.g.Triples(nil, &rdfType, nil)(func(t term.Triple) bool {
		sk := term.TermKey(t.Subject)
		switch term.TermKey(t.Object) {
		case term.TermKey(namespace.OWL.SymmetricProperty):
			e.symmetricProps[sk] = struct{}{}
		case term.TermKey(namespace.OWL.TransitiveProperty):
			e.transitiveProps[sk] = struct{}{}
		case term.TermKey(namespace.OWL.FunctionalProperty):
			e.functionalProps[sk] = struct{}{}
		case term.TermKey(namespace.OWL.InverseFunctionalProperty):
			e.invFuncProps[sk] = struct{}{}
		case term.TermKey(namespace.OWL.IrreflexiveProperty):
			e.irreflexiveProps[sk] = struct{}{}
		case term.TermKey(namespace.OWL.AsymmetricProperty):
			e.asymmetricProps[sk] = struct{}{}
		case term.TermKey(namespace.OWL.Restriction):
			restrictionNodes[sk] = t.Subject
		case term.TermKey(namespace.OWL.AllDisjointProperties):
			allDisjPropNodes = append(allDisjPropNodes, t.Subject)
		case term.TermKey(namespace.OWL.AllDisjointClasses):
			allDisjClassNodes = append(allDisjClassNodes, t.Subject)
		case term.TermKey(namespace.OWL.NegativePropertyAssertion):
			negPANodes = append(negPANodes, t.Subject)
		}
		return true
	})

	// === Property-level scans ===
	e.scanPropertyTriples()

	// === owl:sameAs ===
	sameAs := namespace.OWL.SameAs
	e.g.Triples(nil, &sameAs, nil)(func(t term.Triple) bool {
		e.uf.union(t.Subject, t.Object)
		return true
	})

	// === Parse restrictions ===
	e.buildRestrictions(restrictionNodes)

	// === Parse list-based constructs ===
	e.buildListBasedIndexes()

	// === AllDisjointProperties ===
	e.buildAllDisjoint(allDisjPropNodes, &e.allDisjointProps, namespace.OWL.Members)

	// === AllDisjointClasses ===
	e.buildAllDisjoint(allDisjClassNodes, &e.allDisjointClass, namespace.OWL.Members)

	// === NegativePropertyAssertions ===
	e.buildNegPropAssertions(negPANodes)

	// === owl:differentFrom ===
	diffFrom := namespace.OWL.DifferentFrom
	e.g.Triples(nil, &diffFrom, nil)(func(t term.Triple) bool {
		sk := term.TermKey(t.Subject)
		e.differentFrom[sk] = append(e.differentFrom[sk], t.Object)
		ok := term.TermKey(t.Object)
		if oSubj, isSubj := t.Object.(term.Subject); isSubj {
			_ = oSubj
			e.differentFrom[ok] = append(e.differentFrom[ok], t.Subject)
		}
		return true
	})

	// === owl:disjointWith ===
	disjWith := namespace.OWL.DisjointWith
	e.g.Triples(nil, &disjWith, nil)(func(t term.Triple) bool {
		if c2, ok := t.Object.(term.URIRef); ok {
			if c1, ok := t.Subject.(term.URIRef); ok {
				c1k := term.TermKey(c1)
				c2k := term.TermKey(c2)
				e.disjointWith[c1k] = appendUnique(e.disjointWith[c1k], c2)
				e.disjointWith[c2k] = appendUnique(e.disjointWith[c2k], c1)
			}
		}
		return true
	})
}

func (e *owlrlEngine) scanPropertyTriples() {
	invOf := namespace.OWL.InverseOf
	e.g.Triples(nil, &invOf, nil)(func(t term.Triple) bool {
		if p2, ok := t.Object.(term.URIRef); ok {
			if p1, ok := t.Subject.(term.URIRef); ok {
				p1k := term.TermKey(p1)
				p2k := term.TermKey(p2)
				e.inverseOf[p1k] = appendUnique(e.inverseOf[p1k], p2)
				e.inverseOf[p2k] = appendUnique(e.inverseOf[p2k], p1)
			}
		}
		return true
	})

	eqProp := namespace.OWL.EquivalentProperty
	e.g.Triples(nil, &eqProp, nil)(func(t term.Triple) bool {
		if p2, ok := t.Object.(term.URIRef); ok {
			if p1, ok := t.Subject.(term.URIRef); ok {
				p1k := term.TermKey(p1)
				p2k := term.TermKey(p2)
				e.equivProp[p1k] = appendUnique(e.equivProp[p1k], p2)
				e.equivProp[p2k] = appendUnique(e.equivProp[p2k], p1)
			}
		}
		return true
	})

	// owl:equivalentClass is symmetric, and either side may be an anonymous
	// class expression. Each direction is indexed on its own so that a blank
	// node on one side does not suppress the axiom entirely.
	eqClass := namespace.OWL.EquivalentClass
	e.g.Triples(nil, &eqClass, nil)(func(t term.Triple) bool {
		c2, ok := t.Object.(term.Subject)
		if !ok {
			return true
		}
		c1 := t.Subject
		c1k := term.TermKey(c1)
		c2k := term.TermKey(c2)
		e.equivClass[c1k] = appendUnique(e.equivClass[c1k], c2)
		e.equivClass[c2k] = appendUnique(e.equivClass[c2k], c1)
		return true
	})

	propDW := namespace.OWL.PropertyDisjointWith
	e.g.Triples(nil, &propDW, nil)(func(t term.Triple) bool {
		if p2, ok := t.Object.(term.URIRef); ok {
			if p1, ok := t.Subject.(term.URIRef); ok {
				p1k := term.TermKey(p1)
				p2k := term.TermKey(p2)
				e.propDisjointWith[p1k] = appendUnique(e.propDisjointWith[p1k], p2)
				e.propDisjointWith[p2k] = appendUnique(e.propDisjointWith[p2k], p1)
			}
		}
		return true
	})

	chainAxiom := namespace.OWL.PropertyChainAxiom
	e.g.Triples(nil, &chainAxiom, nil)(func(t term.Triple) bool {
		if p, ok := t.Subject.(term.URIRef); ok {
			listItems := collectList(e.g, t.Object)
			var chain []term.URIRef
			for _, item := range listItems {
				if u, ok := item.(term.URIRef); ok {
					chain = append(chain, u)
				}
			}
			if len(chain) >= 2 {
				pk := term.TermKey(p)
				e.propChainAxiom[pk] = append(e.propChainAxiom[pk], chain)
			}
		}
		return true
	})

	hasKey := namespace.OWL.HasKey
	e.g.Triples(nil, &hasKey, nil)(func(t term.Triple) bool {
		listItems := collectList(e.g, t.Object)
		var keys []term.URIRef
		for _, item := range listItems {
			if u, ok := item.(term.URIRef); ok {
				keys = append(keys, u)
			}
		}
		if len(keys) > 0 {
			ck := term.TermKey(t.Subject)
			e.hasKeyIndex[ck] = keys
		}
		return true
	})
}

func (e *owlrlEngine) buildRestrictions(nodes map[string]term.Subject) {
	onProp := namespace.OWL.OnProperty
	svf := namespace.OWL.SomeValuesFrom
	avf := namespace.OWL.AllValuesFrom
	hv := namespace.OWL.HasValue
	hs := namespace.OWL.HasSelf
	mc := namespace.OWL.MaxCardinality
	mqc := namespace.OWL.MaxQualifiedCardinality
	oc := namespace.OWL.OnClass

	for _, node := range nodes {
		r := &owlRestriction{node: node, maxCard: -1, maxQualCard: -1}

		e.g.Triples(node, &onProp, nil)(func(t term.Triple) bool {
			if u, ok := t.Object.(term.URIRef); ok {
				r.onProperty = u
				r.hasOnProperty = true
			}
			return false
		})
		e.g.Triples(node, &svf, nil)(func(t term.Triple) bool {
			r.someValuesFrom = t.Object
			return false
		})
		e.g.Triples(node, &avf, nil)(func(t term.Triple) bool {
			r.allValuesFrom = t.Object
			return false
		})
		e.g.Triples(node, &hv, nil)(func(t term.Triple) bool {
			r.hasValue = t.Object
			return false
		})
		e.g.Triples(node, &hs, nil)(func(t term.Triple) bool {
			if lit, ok := t.Object.(term.Literal); ok && lit.Lexical() == "true" {
				r.hasSelf = true
			}
			return false
		})
		e.g.Triples(node, &mc, nil)(func(t term.Triple) bool {
			if lit, ok := t.Object.(term.Literal); ok {
				if v, err := strconv.Atoi(lit.Lexical()); err == nil {
					r.maxCard = v
				}
			}
			return false
		})
		e.g.Triples(node, &mqc, nil)(func(t term.Triple) bool {
			if lit, ok := t.Object.(term.Literal); ok {
				if v, err := strconv.Atoi(lit.Lexical()); err == nil {
					r.maxQualCard = v
				}
			}
			return false
		})
		e.g.Triples(node, &oc, nil)(func(t term.Triple) bool {
			r.onClass = t.Object
			return false
		})

		if r.hasOnProperty {
			e.restrictions = append(e.restrictions, r)
			pk := term.TermKey(r.onProperty)
			e.restrByProp[pk] = append(e.restrByProp[pk], r)
		}
	}
}

func (e *owlrlEngine) buildListBasedIndexes() {
	intOf := namespace.OWL.IntersectionOf
	e.g.Triples(nil, &intOf, nil)(func(t term.Triple) bool {
		items := collectList(e.g, t.Object)
		if len(items) > 0 {
			ck := term.TermKey(t.Subject)
			e.intersectionOf[ck] = items
		}
		return true
	})

	unOf := namespace.OWL.UnionOf
	e.g.Triples(nil, &unOf, nil)(func(t term.Triple) bool {
		items := collectList(e.g, t.Object)
		if len(items) > 0 {
			ck := term.TermKey(t.Subject)
			e.unionOf[ck] = items
		}
		return true
	})

	compOf := namespace.OWL.ComplementOf
	e.g.Triples(nil, &compOf, nil)(func(t term.Triple) bool {
		ck := term.TermKey(t.Subject)
		e.complementOf[ck] = t.Object
		return true
	})

	oof := namespace.OWL.OneOf
	e.g.Triples(nil, &oof, nil)(func(t term.Triple) bool {
		items := collectList(e.g, t.Object)
		if len(items) > 0 {
			ck := term.TermKey(t.Subject)
			e.oneOfMembers[ck] = items
		}
		return true
	})
}

func (e *owlrlEngine) buildAllDisjoint(nodes []term.Subject, target *[][]term.URIRef, membersPred term.URIRef) {
	for _, node := range nodes {
		e.g.Triples(node, &membersPred, nil)(func(t term.Triple) bool {
			items := collectList(e.g, t.Object)
			var uris []term.URIRef
			for _, item := range items {
				if u, ok := item.(term.URIRef); ok {
					uris = append(uris, u)
				}
			}
			if len(uris) >= 2 {
				*target = append(*target, uris)
			}
			return false
		})
	}
}

func (e *owlrlEngine) buildNegPropAssertions(nodes []term.Subject) {
	srcPred := namespace.OWL.SourceIndividual
	propPred := namespace.OWL.AssertionProperty
	tiPred := namespace.OWL.TargetIndividual
	tvPred := namespace.OWL.TargetValue

	for _, node := range nodes {
		npa := &negPropAssertion{}

		e.g.Triples(node, &srcPred, nil)(func(t term.Triple) bool {
			if s, ok := t.Object.(term.Subject); ok {
				npa.source = s
			}
			return false
		})
		e.g.Triples(node, &propPred, nil)(func(t term.Triple) bool {
			if u, ok := t.Object.(term.URIRef); ok {
				npa.property = u
			}
			return false
		})
		e.g.Triples(node, &tiPred, nil)(func(t term.Triple) bool {
			if s, ok := t.Object.(term.Subject); ok {
				npa.targetI = s
			}
			return false
		})
		e.g.Triples(node, &tvPred, nil)(func(t term.Triple) bool {
			npa.targetV = t.Object
			return false
		})

		if npa.source != nil && npa.property != (term.URIRef{}) {
			e.negPropAsserts = append(e.negPropAsserts, npa)
		}
	}
}

// emitFunc is the type for the rule triple emitter.
type emitFunc func(s term.Subject, p term.URIRef, o term.Term)

func (e *owlrlEngine) applyRules() []term.Triple {
	var newTriples []term.Triple

	var allTriples []ruleTriple
	e.g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		allTriples = append(allTriples, ruleTriple{t.Subject, t.Predicate, t.Object})
		return true
	})

	emit := func(s term.Subject, p term.URIRef, o term.Term) {
		if e.ded.addNew(s, p, o) {
			newTriples = append(newTriples, term.Triple{Subject: s, Predicate: p, Object: o})
		}
	}

	e.applyEqualityRules(allTriples, emit)
	e.applyPropertyRules(allTriples, emit)
	e.applyClassRules(allTriples, emit)

	return newTriples
}

// appendUnique appends u to the slice if not already present.
func appendUnique[T term.Term](s []T, u T) []T {
	uk := term.TermKey(u)
	for _, existing := range s {
		if term.TermKey(existing) == uk {
			return s
		}
	}
	return append(s, u)
}
