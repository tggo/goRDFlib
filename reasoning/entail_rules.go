package reasoning

import (
	"context"
	"fmt"
	"sort"
	"strconv"
)

// Semantics selects how much meaning the checker reads into a graph
// (RDF 1.1 Semantics, RDF 1.2 Semantics).
type Semantics int

const (
	// SimpleEntailment is graph homomorphism: the conclusion is entailed when
	// its blank nodes can be mapped to terms of the premise so that every
	// triple is found (RDF 1.1 Semantics §2).
	SimpleEntailment Semantics = iota
	// RDFEntailment adds the RDF axiomatic triples, rdf:type Property for every
	// predicate, and (with recognized datatypes) the type of every literal.
	RDFEntailment
	// RDFSEntailment adds the RDFS axioms and rules rdfs2..rdfs13, plus the
	// datatype inconsistencies of a D-interpretation.
	RDFSEntailment
)

// String names the semantics as the W3C manifests do.
func (s Semantics) String() string {
	switch s {
	case SimpleEntailment:
		return "simple"
	case RDFEntailment:
		return "RDF"
	case RDFSEntailment:
		return "RDFS"
	}
	return "Semantics(" + strconv.Itoa(int(s)) + ")"
}

// saturate adds the axiomatic triples and applies the rules of the chosen
// semantics until nothing new is derived. Every rule only combines triples
// over nodes already interned, so the process terminates.
func (m *model) saturate(ctx context.Context, sem Semantics) error {
	if sem >= RDFEntailment {
		m.addRDFAxioms()
	}
	if sem >= RDFSEntailment {
		m.addRDFSAxioms()
	}
	if sem >= RDFEntailment {
		m.typeLiterals(sem)
	}
	if sem == SimpleEntailment {
		return nil
	}
	v := m.voc
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		before := len(m.triples)
		for i := 0; i < len(m.triples); i++ {
			t := m.triples[i]
			// rdf1: any predicate is a property.
			m.add(t[1], v.typ, v.property)
			if sem >= RDFSEntailment {
				// rdfs4a, rdfs4b: any subject or object is a resource.
				m.add(t[0], v.typ, v.resource)
				m.add(t[2], v.typ, v.resource)
			}
		}
		if sem >= RDFSEntailment {
			m.rdfsRound()
		}
		if len(m.triples) == before {
			return nil
		}
	}
}

// pairs returns the (s, o) of every triple with predicate p, as a snapshot.
func (m *model) pairs(p int) [][2]int {
	idx := m.byP[p]
	out := make([][2]int, len(idx))
	for i, ti := range idx {
		out[i] = [2]int{m.triples[ti][0], m.triples[ti][2]}
	}
	return out
}

func (m *model) rdfsRound() {
	v := m.voc
	// rdfs2, rdfs3.
	for _, d := range m.pairs(v.domain) {
		for _, t := range m.pairs(d[0]) {
			m.add(t[0], v.typ, d[1])
		}
	}
	for _, r := range m.pairs(v.rng) {
		for _, t := range m.pairs(r[0]) {
			m.add(t[1], v.typ, r[1])
		}
	}
	// rdfs5, rdfs7: subPropertyOf is transitive and inherits triples.
	m.transitive(v.subPropertyOf)
	for _, sp := range m.pairs(v.subPropertyOf) {
		for _, t := range m.pairs(sp[0]) {
			m.add(t[0], sp[1], t[1])
		}
	}
	// rdfs6: every property is a subproperty of itself.
	for _, p := range m.pairs(v.typ) {
		if p[1] == v.property {
			m.add(p[0], v.subPropertyOf, p[0])
		}
		// rdfs8, rdfs10: every class is a subclass of Resource and of itself.
		if p[1] == v.class {
			m.add(p[0], v.subClassOf, v.resource)
			m.add(p[0], v.subClassOf, p[0])
		}
		// rdfs12: container membership properties are subproperties of member.
		if p[1] == v.cmp {
			m.add(p[0], v.subPropertyOf, v.member)
		}
		// rdfs13: every datatype is a subclass of Literal.
		if p[1] == v.datatype {
			m.add(p[0], v.subClassOf, v.literal)
		}
	}
	// rdfs11, rdfs9.
	m.transitive(v.subClassOf)
	for _, sc := range m.pairs(v.subClassOf) {
		for _, ti := range m.byPO[[2]int{v.typ, sc[0]}] {
			m.add(m.triples[ti][0], v.typ, sc[1])
		}
	}
}

// transitive closes predicate p under (a p b), (b p c) => (a p c).
func (m *model) transitive(p int) {
	for _, ab := range m.pairs(p) {
		for _, ti := range m.bySP[[2]int{ab[1], p}] {
			m.add(ab[0], p, m.triples[ti][2])
		}
	}
}

func (m *model) addRDFAxioms() {
	v := m.voc
	for _, p := range []int{v.typ, v.subject, v.predicate, v.object, v.first, v.rest, v.value, v.reifies} {
		m.add(p, v.typ, v.property)
	}
	m.add(v.nilN, v.typ, v.list)
	for _, i := range m.memberNumbers() {
		m.add(m.internIRI(nsRDF+"_"+strconv.Itoa(i)), v.typ, v.property)
	}
}

// memberNumbers lists the n of rdf:_1 and of every rdf:_n the graphs mention.
// The axioms are infinite; the ones for unmentioned members cannot change
// whether a conclusion that does not mention them is entailed.
func (m *model) memberNumbers() []int {
	out := make([]int, 0, len(m.memberIdx))
	for i := range m.memberIdx {
		out = append(out, i)
	}
	sort.Ints(out)
	return out
}

func (m *model) addRDFSAxioms() {
	v := m.voc
	// Domains and ranges (RDF 1.1 Semantics §9.3 and the RDF 1.2 additions).
	dom := func(p, c int) { m.add(p, v.domain, c) }
	rng := func(p, c int) { m.add(p, v.rng, c) }
	dom(v.typ, v.resource)
	dom(v.domain, v.property)
	dom(v.rng, v.property)
	dom(v.subPropertyOf, v.property)
	dom(v.subClassOf, v.class)
	dom(v.subject, v.statement)
	dom(v.predicate, v.statement)
	dom(v.object, v.statement)
	dom(v.first, v.list)
	dom(v.rest, v.list)
	dom(v.member, v.resource)
	dom(v.seeAlso, v.resource)
	dom(v.isDefinedBy, v.resource)
	dom(v.comment, v.resource)
	dom(v.label, v.resource)
	dom(v.value, v.resource)
	dom(v.reifies, v.resource)
	rng(v.typ, v.class)
	rng(v.domain, v.class)
	rng(v.rng, v.class)
	rng(v.subPropertyOf, v.property)
	rng(v.subClassOf, v.class)
	rng(v.subject, v.resource)
	rng(v.predicate, v.resource)
	rng(v.object, v.resource)
	rng(v.first, v.resource)
	rng(v.rest, v.list)
	rng(v.member, v.resource)
	rng(v.seeAlso, v.resource)
	rng(v.isDefinedBy, v.resource)
	rng(v.comment, v.literal)
	rng(v.label, v.literal)
	rng(v.value, v.resource)
	// RDF 1.2 Semantics: rdf:reifies relates a reifier to a proposition.
	rng(v.reifies, v.proposition)

	for _, c := range []int{v.resource, v.class, v.literal, v.datatype, v.statement, v.container,
		v.bag, v.seq, v.alt, v.property, v.list, v.cmp, v.proposition} {
		m.add(c, v.typ, v.class)
	}
	m.add(v.datatype, v.subClassOf, v.class)
	m.add(v.bag, v.subClassOf, v.container)
	m.add(v.seq, v.subClassOf, v.container)
	m.add(v.alt, v.subClassOf, v.container)
	m.add(v.cmp, v.subClassOf, v.property)
	m.add(v.isDefinedBy, v.subPropertyOf, v.seeAlso)
	m.add(v.member, v.typ, v.property)
	m.add(v.nilN, v.typ, v.list)

	for _, i := range m.memberNumbers() {
		n := m.internIRI(nsRDF + "_" + strconv.Itoa(i))
		m.add(n, v.typ, v.cmp)
		dom(n, v.resource)
		rng(n, v.resource)
	}

	// Datatypes: every recognized one is an rdfs:Datatype, and a recognized
	// datatype whose value space lies inside another's is its subclass.
	iris := make([]string, 0, len(m.recognized))
	for iri := range m.recognized {
		iris = append(iris, iri)
	}
	sort.Strings(iris)
	for _, a := range iris {
		an := m.internIRI(a)
		m.add(an, v.typ, v.datatype)
		for _, b := range iris {
			if a != b && m.recognized[b].subsumes(m.recognized[a]) {
				m.add(an, v.subClassOf, m.internIRI(b))
			}
		}
	}
	// Triple terms are propositions (RDF 1.2 Semantics, RDFS interpretations).
	for id := range m.nodes {
		if m.nodes[id].kind == nodeTripleTerm {
			m.add(id, v.typ, v.proposition)
		}
	}
}

// typeLiterals gives every literal of the premise the datatypes whose value
// space holds its value (rdf2 of D-entailment, RDF 1.1 Semantics §7.3), and
// under RDFS makes it an rdfs:Literal.
func (m *model) typeLiterals(sem Semantics) {
	v := m.voc
	for _, id := range m.litNodes {
		n := m.nodes[id]
		if n.val != nil {
			for iri, spec := range m.recognized {
				if spec.holds(n.val) {
					m.add(id, v.typ, m.internIRI(iri))
				}
			}
		}
		if sem >= RDFSEntailment && !(n.spec != nil && n.val == nil) {
			m.add(id, v.typ, v.literal)
		}
	}
}

// holds reports whether val lies in this datatype's value space.
func (s *dtSpec) holds(val *value) bool {
	if s.fam != val.fam {
		return false
	}
	if s.fam == famDec {
		return s.contains(val.rat)
	}
	return true
}

// checkConsistent looks for the inconsistencies of a D-interpretation that the
// RDFS rules expose: a literal in the extension of a datatype that does not
// contain its value, two datatypes with no common value that share a member,
// and a subclass link between datatypes whose value spaces are not nested
// (RDF 1.1 Semantics §8, §7.3).
func (m *model) checkConsistent() {
	if m.inconsistent != "" {
		return
	}
	v := m.voc
	typesOf := make(map[int][]*dtSpec)
	for _, ti := range m.byP[v.typ] {
		t := m.triples[ti]
		cls := m.nodes[t[2]]
		spec := m.recognized[cls.iri]
		if cls.kind != nodeIRI || spec == nil {
			continue
		}
		n := m.nodes[t[0]]
		if n.kind == nodeLiteral && n.val != nil && !spec.holds(n.val) {
			m.inconsistent = fmt.Sprintf("%s is typed %s, but its value is not in that datatype's value space", n.val.key, cls.iri)
			return
		}
		typesOf[t[0]] = append(typesOf[t[0]], spec)
	}
	for id, specs := range typesOf {
		for i := range specs {
			for j := i + 1; j < len(specs); j++ {
				if !specs[i].overlaps(specs[j]) {
					m.inconsistent = fmt.Sprintf("node %d is a member of the disjoint datatypes %s and %s", id, specs[i].iri, specs[j].iri)
					return
				}
			}
		}
	}
	for _, ti := range m.byP[v.subClassOf] {
		t := m.triples[ti]
		a, b := m.nodes[t[0]], m.nodes[t[2]]
		if a.kind != nodeIRI || b.kind != nodeIRI {
			continue
		}
		sa, sb := m.recognized[a.iri], m.recognized[b.iri]
		if sa != nil && sb != nil && !sb.subsumes(sa) {
			m.inconsistent = fmt.Sprintf("%s is a subclass of %s, but its values are not all values of %s", a.iri, b.iri, b.iri)
			return
		}
	}
}
