package rdflibgo

import "testing"

// Ported from: rdflib triple pattern matching

func TestTripleString(t *testing.T) {
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("hello")
	tr := Triple{Subject: s, Predicate: p, Object: o}
	got := tr.String()
	if got == "" {
		t.Error("empty string")
	}
}

func TestTriplePatternMatchesAll(t *testing.T) {
	// Ported from: rdflib triple pattern matching — all wildcards
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("hello")
	tr := Triple{Subject: s, Predicate: p, Object: o}

	pat := TriplePattern{} // all nil = match everything
	if !pat.Matches(tr) {
		t.Error("empty pattern should match everything")
	}
}

func TestTriplePatternMatchesSubject(t *testing.T) {
	s, _ := NewURIRef("http://example.org/s")
	s2, _ := NewURIRef("http://example.org/other")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("hello")
	tr := Triple{Subject: s, Predicate: p, Object: o}

	pat := TriplePattern{Subject: s}
	if !pat.Matches(tr) {
		t.Error("should match same subject")
	}
	pat2 := TriplePattern{Subject: s2}
	if pat2.Matches(tr) {
		t.Error("should not match different subject")
	}
}

func TestTriplePatternMatchesPredicate(t *testing.T) {
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	p2, _ := NewURIRef("http://example.org/other")
	o := NewLiteral("hello")
	tr := Triple{Subject: s, Predicate: p, Object: o}

	pat := TriplePattern{Predicate: &p}
	if !pat.Matches(tr) {
		t.Error("should match same predicate")
	}
	pat2 := TriplePattern{Predicate: &p2}
	if pat2.Matches(tr) {
		t.Error("should not match different predicate")
	}
}

func TestTriplePatternMatchesObject(t *testing.T) {
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("hello")
	o2 := NewLiteral("world")
	tr := Triple{Subject: s, Predicate: p, Object: o}

	pat := TriplePattern{Object: o}
	if !pat.Matches(tr) {
		t.Error("should match same object")
	}
	pat2 := TriplePattern{Object: o2}
	if pat2.Matches(tr) {
		t.Error("should not match different object")
	}
}

func TestCompareTerm(t *testing.T) {
	// Ported from: rdflib term ordering for serialization
	u, _ := NewURIRef("http://example.org/a")
	b := NewBNode("b1")
	l := NewLiteral("hello")

	if CompareTerm(b, u) >= 0 {
		t.Error("BNode should sort before URIRef")
	}
	if CompareTerm(u, l) >= 0 {
		t.Error("URIRef should sort before Literal")
	}
	if CompareTerm(b, l) >= 0 {
		t.Error("BNode should sort before Literal")
	}

	u2, _ := NewURIRef("http://example.org/b")
	if CompareTerm(u, u2) >= 0 {
		t.Error("a should sort before b")
	}
	if CompareTerm(u, u) != 0 {
		t.Error("same term should compare equal")
	}
}
