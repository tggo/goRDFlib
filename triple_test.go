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
	if got != `(<http://example.org/s>, <http://example.org/p>, "hello")` {
		t.Errorf("got %q", got)
	}
}

func TestQuadString(t *testing.T) {
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("hello")
	g, _ := NewURIRef("http://example.org/g")

	q := Quad{Triple: Triple{Subject: s, Predicate: p, Object: o}, Graph: g}
	got := q.String()
	if got == "" {
		t.Error("empty string")
	}

	// Nil graph falls back to triple string
	q2 := Quad{Triple: Triple{Subject: s, Predicate: p, Object: o}, Graph: nil}
	expected := (Triple{Subject: s, Predicate: p, Object: o}).String()
	if q2.String() != expected {
		t.Error("nil graph should use triple string")
	}
}

func TestTriplePatternMatchesAll(t *testing.T) {
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

func TestTriplePatternMatchesObjectURIRef(t *testing.T) {
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	o2, _ := NewURIRef("http://example.org/other")
	tr := Triple{Subject: s, Predicate: p, Object: o}

	pat := TriplePattern{Object: o}
	if !pat.Matches(tr) {
		t.Error("should match same URIRef object")
	}
	pat2 := TriplePattern{Object: o2}
	if pat2.Matches(tr) {
		t.Error("should not match different URIRef object")
	}
}

func TestTriplePatternMatchesBNodeSubject(t *testing.T) {
	b := NewBNode("b1")
	b2 := NewBNode("b2")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("v")
	tr := Triple{Subject: b, Predicate: p, Object: o}

	patB := TriplePattern{Subject: b}
	if !patB.Matches(tr) {
		t.Error("should match same BNode subject")
	}
	patB2 := TriplePattern{Subject: b2}
	if patB2.Matches(tr) {
		t.Error("should not match different BNode subject")
	}
}

func TestCompareTerm(t *testing.T) {
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

	// Variable
	v := NewVariable("x")
	if CompareTerm(l, v) >= 0 {
		t.Error("Literal should sort before Variable")
	}
}

// --- Benchmarks ---

func BenchmarkTriplePatternMatches(b *testing.B) {
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("hello")
	tr := Triple{Subject: s, Predicate: p, Object: o}
	pat := TriplePattern{Subject: s, Predicate: &p, Object: o}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pat.Matches(tr)
	}
}
