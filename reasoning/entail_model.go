package reasoning

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// nodeKind says what an interned node is.
type nodeKind uint8

const (
	nodeIRI nodeKind = iota
	nodeBNode
	nodeLiteral
	nodeTripleTerm
)

// node is one element of the checker's domain: an IRI, a premise blank node, a
// literal (identified by its value when its datatype is recognized) or a
// triple term (identified by its three components).
type node struct {
	kind    nodeKind
	iri     string  // nodeIRI
	val     *value  // nodeLiteral of a recognized, well-typed datatype
	spec    *dtSpec // the literal's datatype when recognized
	parts   [3]int  // nodeTripleTerm: subject, predicate, object
	premise bool    // appears in the premise graph (possibly nested)
}

// model is the premise graph, interned, plus whatever the entailment rules
// add. Triples may have any node in any position (generalized RDF, RDF 1.1
// Semantics §7.2): the rules derive `"5"^^xsd:integer rdf:type xsd:integer`.
//
// A model is built and used by one goroutine.
type model struct {
	nodes []node
	byKey map[string]int

	set     map[[3]int]struct{}
	triples [][3]int
	byP     map[int][]int    // predicate -> indexes into triples
	bySP    map[[2]int][]int // (subject, predicate) -> indexes
	byPO    map[[2]int][]int // (predicate, object) -> indexes

	recognized map[string]*dtSpec // recognized datatypes, always incl. string and langString
	litNodes   []int              // literals appearing in the premise graph
	litSeen    map[int]struct{}
	memberIdx  map[int]struct{} // n of every rdf:_n mentioned anywhere

	inconsistent string // non-empty: why the premise has no model
	voc          vocab
}

// vocab holds the node ids of the vocabulary the rules mention.
type vocab struct {
	typ, property, list, nilN, first, rest, value, subject, predicate, object      int
	domain, rng, subClassOf, subPropertyOf, resource, class, literal, datatype     int
	cmp, member, comment, label, seeAlso, isDefinedBy, container, bag, seq, alt    int
	statement, proposition, reifies, langString, dirLangString, xmlLiteral, xsdStr int
}

func newModel(recognized map[string]*dtSpec) *model {
	m := &model{
		byKey:      make(map[string]int, 128),
		set:        make(map[[3]int]struct{}, 128),
		byP:        make(map[int][]int, 32),
		bySP:       make(map[[2]int][]int, 128),
		byPO:       make(map[[2]int][]int, 128),
		recognized: recognized,
		memberIdx:  map[int]struct{}{1: {}},
		litSeen:    make(map[int]struct{}, 16),
	}
	iri := func(local string) int { return m.internIRI(nsRDF + local) }
	rdfs := func(local string) int { return m.internIRI(nsRDFS + local) }
	m.voc = vocab{
		typ: iri("type"), property: iri("Property"), list: iri("List"), nilN: iri("nil"),
		first: iri("first"), rest: iri("rest"), value: iri("value"), subject: iri("subject"),
		predicate: iri("predicate"), object: iri("object"),
		domain: rdfs("domain"), rng: rdfs("range"), subClassOf: rdfs("subClassOf"),
		subPropertyOf: rdfs("subPropertyOf"), resource: rdfs("Resource"), class: rdfs("Class"),
		literal: rdfs("Literal"), datatype: rdfs("Datatype"), cmp: rdfs("ContainerMembershipProperty"),
		member: rdfs("member"), comment: rdfs("comment"), label: rdfs("label"),
		seeAlso: rdfs("seeAlso"), isDefinedBy: rdfs("isDefinedBy"), container: rdfs("Container"),
		bag: iri("Bag"), seq: iri("Seq"), alt: iri("Alt"), statement: iri("Statement"),
		proposition: rdfs("Proposition"), reifies: iri("reifies"),
		langString: iri("langString"), dirLangString: iri("dirLangString"),
		xmlLiteral: iri("XMLLiteral"), xsdStr: m.internIRI(nsXSD + "string"),
	}
	return m
}

func (m *model) intern(key string, n node) int {
	if id, ok := m.byKey[key]; ok {
		return id
	}
	id := len(m.nodes)
	m.nodes = append(m.nodes, n)
	m.byKey[key] = id
	return id
}

func (m *model) internIRI(iri string) int {
	if rest, ok := strings.CutPrefix(iri, nsRDF+"_"); ok {
		if n, err := strconv.Atoi(rest); err == nil && n > 0 && strconv.Itoa(n) == rest {
			m.memberIdx[n] = struct{}{}
		}
	}
	return m.intern("I"+iri, node{kind: nodeIRI, iri: iri})
}

// internTerm interns t. premise marks a term of the premise graph: only there
// does an ill-typed literal make the graph inconsistent.
func (m *model) internTerm(t term.Term, premise bool) int {
	var id int
	switch x := t.(type) {
	case term.URIRef:
		id = m.internIRI(x.Value())
	case term.BNode:
		id = m.intern("_"+x.Value(), node{kind: nodeBNode})
	case term.Literal:
		id = m.internLiteral(x, premise)
	case term.TripleTerm:
		s := m.internTerm(x.Subject(), premise)
		p := m.internTerm(x.Predicate(), premise)
		o := m.internTerm(x.Object(), premise)
		key := "T" + strconv.Itoa(s) + "," + strconv.Itoa(p) + "," + strconv.Itoa(o)
		id = m.intern(key, node{kind: nodeTripleTerm, parts: [3]int{s, p, o}})
	default:
		id = m.intern("?"+t.String(), node{kind: nodeIRI, iri: t.String()})
	}
	return id
}

// internLiteral interns a literal under its value when its datatype is
// recognized, so that "010"^^xsd:integer and "10.0"^^xsd:decimal are one node
// (RDF 1.1 Semantics §7.1, §7.3). Language tags are case-insensitive
// (RDF 1.1 Concepts §3.3; term.Literal already lowercases the tag, which
// is what makes this key case-insensitive). A recognized literal outside its lexical space is
// ill-typed: a premise that contains one has no model (RDF 1.1 Semantics
// §7.3), whereas one in the conclusion is simply never entailed.
func (m *model) internLiteral(l term.Literal, premise bool) int {
	lex := l.Lexical()
	if lang := l.Language(); lang != "" {
		fam, spec := famLang, dtSpecs[nsRDF+"langString"]
		if l.Dir() != "" {
			fam, spec = famDirLng, dtSpecs[nsRDF+"dirLangString"]
		}
		key := "G" + lang + "\x00" + l.Dir() + "\x00" + lex
		return m.intern(key, node{kind: nodeLiteral, spec: spec, val: &value{fam: fam, key: key}})
	}
	dt := l.Datatype().Value()
	if dt == "" {
		dt = nsXSD + "string"
	}
	if spec, ok := m.recognized[dt]; ok {
		if v, ok := spec.valueOf(lex); ok {
			return m.intern(v.key, node{kind: nodeLiteral, spec: spec, val: &v})
		}
		if premise {
			m.inconsistent = fmt.Sprintf("ill-typed literal %s: %q is not in the lexical space of a recognized datatype", l.N3(), lex)
		}
		return m.intern("!"+term.TermKey(l), node{kind: nodeLiteral, spec: spec})
	}
	return m.intern("L"+term.TermKey(l), node{kind: nodeLiteral})
}

// add inserts a triple and reports whether it was new.
func (m *model) add(s, p, o int) bool {
	k := [3]int{s, p, o}
	if _, dup := m.set[k]; dup {
		return false
	}
	m.set[k] = struct{}{}
	i := len(m.triples)
	m.triples = append(m.triples, k)
	m.byP[p] = append(m.byP[p], i)
	m.bySP[[2]int{s, p}] = append(m.bySP[[2]int{s, p}], i)
	m.byPO[[2]int{p, o}] = append(m.byPO[[2]int{p, o}], i)
	return true
}

// loadPremise interns every triple of g. The premise is read in full or not
// at all: a store error is returned, never a partial model.
func (m *model) loadPremise(ctx context.Context, g *graph.Graph) error {
	for t, err := range g.Triples(ctx, nil, nil, nil) {
		if err != nil {
			return err
		}
		s := m.internTerm(t.Subject, true)
		p := m.internTerm(t.Predicate, true)
		o := m.internTerm(t.Object, true)
		for _, id := range [2]int{s, o} {
			if _, seen := m.litSeen[id]; m.nodes[id].kind == nodeLiteral && !seen {
				m.litSeen[id] = struct{}{}
				m.litNodes = append(m.litNodes, id)
			}
		}
		m.add(s, p, o)
	}
	return nil
}
