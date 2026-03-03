package term

// TripleTerm represents an RDF 1.2 triple term — a triple used as a term.
// Per the RDF 1.2 spec, triple terms can only appear in the object position.
// The subject of a triple term must be a URIRef or BNode (not another TripleTerm).
// The predicate must be a URIRef. The object can be any Term including another TripleTerm.
// Safe for concurrent use; immutable after construction.
type TripleTerm struct {
	subj Subject
	pred URIRef
	obj  Term
	key  string // cached TermKey
}

func (t TripleTerm) termType() string { return "TripleTerm" }
func (t TripleTerm) subject()         {} // implements Subject interface

// Subject returns the subject of the triple term.
func (t TripleTerm) Subject() Subject { return t.subj }

// Predicate returns the predicate of the triple term.
func (t TripleTerm) Predicate() URIRef { return t.pred }

// Object returns the object of the triple term.
func (t TripleTerm) Object() Term { return t.obj }

// N3 returns the N-Triples/N3 representation: <<( <s> <p> <o> )>>.
func (t TripleTerm) N3(ns ...NamespaceManager) string {
	return "<<( " + t.subj.N3(ns...) + " " + t.pred.N3(ns...) + " " + t.obj.N3(ns...) + " )>>"
}

// String returns a human-readable representation.
func (t TripleTerm) String() string {
	return "<<( " + t.subj.String() + " " + t.pred.String() + " " + t.obj.String() + " )>>"
}

// Equal returns true if other is a TripleTerm with equal subject, predicate, and object.
func (t TripleTerm) Equal(other Term) bool {
	if o, ok := other.(TripleTerm); ok {
		return t.subj.Equal(o.subj) && t.pred.Equal(o.pred) && t.obj.Equal(o.obj)
	}
	return false
}

// NewTripleTerm creates a new TripleTerm. Panics if subject or object is nil.
func NewTripleTerm(subject Subject, predicate URIRef, object Term) TripleTerm {
	if subject == nil {
		panic("term: NewTripleTerm called with nil subject")
	}
	if object == nil {
		panic("term: NewTripleTerm called with nil object")
	}
	key := "T:" + TermKey(subject) + "\x00" + TermKey(predicate) + "\x00" + TermKey(object)
	return TripleTerm{
		subj: subject,
		pred: predicate,
		obj:  object,
		key:  key,
	}
}
