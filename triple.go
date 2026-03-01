package rdflibgo

import "fmt"

// Triple represents an RDF triple (subject, predicate, object).
// Ported from: rdflib.graph (triple handling)
type Triple struct {
	Subject   Subject
	Predicate URIRef
	Object    Term
}

// String returns a human-readable representation of the triple.
func (t Triple) String() string {
	return fmt.Sprintf("(%s, %s, %s)", t.Subject.N3(), t.Predicate.N3(), t.Object.N3())
}

// Quad represents an RDF quad (triple + named graph).
type Quad struct {
	Triple
	Graph Subject
}

// String returns a human-readable representation of the quad.
func (q Quad) String() string {
	if q.Graph != nil {
		return fmt.Sprintf("(%s, %s, %s, %s)", q.Subject.N3(), q.Predicate.N3(), q.Object.N3(), q.Graph.N3())
	}
	return q.Triple.String()
}

// TriplePattern is used for matching triples. Nil fields act as wildcards.
type TriplePattern struct {
	Subject   Subject
	Predicate *URIRef
	Object    Term
}

// Matches returns true if the triple matches this pattern.
// Nil fields in the pattern act as wildcards matching any value.
func (p TriplePattern) Matches(t Triple) bool {
	if p.Subject != nil && !p.Subject.Equal(t.Subject) {
		return false
	}
	if p.Predicate != nil && *p.Predicate != t.Predicate {
		return false
	}
	if p.Object != nil && !p.Object.Equal(t.Object) {
		return false
	}
	return true
}
