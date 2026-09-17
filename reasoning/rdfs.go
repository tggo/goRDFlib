package reasoning

import (
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

// RDFSClosure applies RDFS closure rules to the graph, adding inferred triples.
// Implements rules rdfs2, rdfs3, rdfs5, rdfs7, rdfs9, rdfs11.
// Returns the total number of triples added.
//
// Not safe for concurrent use.
func RDFSClosure(g *graph.Graph) int {
	e := newRDFSEngine(g, nil)
	return e.run()
}

// rdfsEngine holds schema indexes and dedup state for RDFS closure computation.
type rdfsEngine struct {
	g    *graph.Graph
	ded  *dedupSet
	stop *stopper // nil never stops

	// Schema indexes.
	//
	// Class-valued indexes hold term.Subject, not term.URIRef: a class may be a
	// blank node, because an anonymous class expression such as an
	// owl:Restriction has no IRI. term.Subject admits URIRef and BNode and
	// excludes Literal, which is exactly the set of terms that can denote a
	// class. Property-valued indexes stay term.URIRef, as RDFS and OWL 2 RL
	// both require a property to be named.
	domains    map[string][]term.Subject // predicate key → domain classes
	ranges     map[string][]term.Subject // predicate key → range classes
	subClassOf map[string][]term.Subject // class key → transitive superclasses
	subPropOf  map[string][]term.URIRef  // property key → transitive superproperties
}

func newRDFSEngine(g *graph.Graph, stop *stopper) *rdfsEngine {
	return &rdfsEngine{
		g:    g,
		ded:  newDedupSet(g.Len()),
		stop: stop,
	}
}

func (e *rdfsEngine) run() int {
	// Build existing set from all triples
	e.g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		e.ded.addNew(t.Subject, t.Predicate, t.Object)
		return true
	})

	// Build schema indexes
	e.buildSchemaIndexes()

	// Compute transitive closures
	e.closeTransitive()

	totalAdded := 0

	// Fixed-point loop
	for {
		newTriples := e.applyRules()
		// A pass cut short found only some of its triples, and the dedup set
		// already counts the rest as known: stop before adding any of them.
		if e.stop.poll() || len(newTriples) == 0 {
			break
		}

		hasSchemaTriples := false
		for i, t := range newTriples {
			// One pass can derive hundreds of thousands of triples, and adding
			// them is the slow part. Stopping here leaves only the triples added
			// so far, each of them a valid entailment.
			if e.stop.tick() {
				return totalAdded + i
			}
			e.g.Add(t.Subject, t.Predicate, t.Object)
			pk := term.TermKey(t.Predicate)
			if pk == term.TermKey(namespace.RDFS.SubClassOf) ||
				pk == term.TermKey(namespace.RDFS.SubPropertyOf) ||
				pk == term.TermKey(namespace.RDFS.Domain) ||
				pk == term.TermKey(namespace.RDFS.Range) {
				hasSchemaTriples = true
			}
		}
		totalAdded += len(newTriples)

		if hasSchemaTriples {
			e.buildSchemaIndexes()
			e.closeTransitive()
		}
	}

	return totalAdded
}

func (e *rdfsEngine) buildSchemaIndexes() {
	e.domains = make(map[string][]term.Subject)
	e.ranges = make(map[string][]term.Subject)
	e.subClassOf = make(map[string][]term.Subject)
	e.subPropOf = make(map[string][]term.URIRef)

	domainPred := namespace.RDFS.Domain
	rangePred := namespace.RDFS.Range
	subClassPred := namespace.RDFS.SubClassOf
	subPropPred := namespace.RDFS.SubPropertyOf

	e.g.Triples(nil, &domainPred, nil)(func(t term.Triple) bool {
		if c, ok := t.Object.(term.Subject); ok {
			pk := term.TermKey(t.Subject)
			e.domains[pk] = append(e.domains[pk], c)
		}
		return true
	})

	e.g.Triples(nil, &rangePred, nil)(func(t term.Triple) bool {
		if c, ok := t.Object.(term.Subject); ok {
			pk := term.TermKey(t.Subject)
			e.ranges[pk] = append(e.ranges[pk], c)
		}
		return true
	})

	e.g.Triples(nil, &subClassPred, nil)(func(t term.Triple) bool {
		if c, ok := t.Object.(term.Subject); ok {
			sk := term.TermKey(t.Subject)
			e.subClassOf[sk] = append(e.subClassOf[sk], c)
		}
		return true
	})

	e.g.Triples(nil, &subPropPred, nil)(func(t term.Triple) bool {
		if c, ok := t.Object.(term.URIRef); ok {
			sk := term.TermKey(t.Subject)
			e.subPropOf[sk] = append(e.subPropOf[sk], c)
		}
		return true
	})
}

// closeTransitive computes transitive closures for subClassOf and subPropertyOf.
func (e *rdfsEngine) closeTransitive() {
	e.subClassOf = transitiveClose(e.subClassOf)
	e.subPropOf = transitiveClose(e.subPropOf)
}

// transitiveClose computes the transitive closure of a relation map. It is
// generic over the term type so that it serves both the class indexes, whose
// values may be blank nodes, and the property indexes, whose values are named.
func transitiveClose[T term.Term](m map[string][]T) map[string][]T {
	result := make(map[string][]T, len(m))
	for k := range m {
		visited := make(map[string]struct{})
		var supers []T
		var queue []T
		queue = append(queue, m[k]...)
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			ck := term.TermKey(cur)
			if _, seen := visited[ck]; seen {
				continue
			}
			visited[ck] = struct{}{}
			supers = append(supers, cur)
			queue = append(queue, m[ck]...)
		}
		if len(supers) > 0 {
			result[k] = supers
		}
	}
	return result
}

// applyRules applies rdfs2, rdfs3, rdfs7, rdfs9 and returns new triples to add.
func (e *rdfsEngine) applyRules() []term.Triple {
	var newTriples []term.Triple
	rdfType := namespace.RDF.Type

	// Every derivation passes through emit, which is where the stop is counted:
	// one scanned triple can derive a type for every superclass in a long
	// chain, so counting scanned triples alone polls far too rarely.
	emit := func(s term.Subject, p term.URIRef, o term.Term) {
		if e.stop.tick() {
			return
		}
		if e.ded.addNew(s, p, o) {
			newTriples = append(newTriples, term.Triple{Subject: s, Predicate: p, Object: o})
		}
	}

	e.g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		if e.stop.tick() {
			return false
		}
		pk := term.TermKey(t.Predicate)

		// rdfs2: ?p rdfs:domain ?C, ?s ?p ?o → ?s rdf:type ?C
		for _, c := range e.domains[pk] {
			emit(t.Subject, rdfType, c)
		}

		// rdfs3: ?p rdfs:range ?C, ?s ?p ?o → ?o rdf:type ?C (only if o is a Subject)
		if ranges, ok := e.ranges[pk]; ok {
			if oSubj, ok := t.Object.(term.Subject); ok {
				for _, c := range ranges {
					emit(oSubj, rdfType, c)
				}
			}
		}

		// rdfs7: ?p rdfs:subPropertyOf ?q, ?s ?p ?o → ?s ?q ?o
		for _, q := range e.subPropOf[pk] {
			emit(t.Subject, q, t.Object)
		}

		// rdfs9: ?s rdf:type ?C1, ?C1 rdfs:subClassOf ?C2 → ?s rdf:type ?C2
		// C1 is keyed directly, so an anonymous class expression is looked up
		// like a named one. A literal simply never matches an index entry.
		if pk == term.TermKey(rdfType) {
			for _, c2 := range e.subClassOf[term.TermKey(t.Object)] {
				emit(t.Subject, rdfType, c2)
			}
		}

		return !e.stop.halted()
	})

	return newTriples
}
