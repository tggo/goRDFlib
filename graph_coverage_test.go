package rdflibgo

import (
	"testing"
)

func TestGraphWithStore(t *testing.T) {
	s := NewMemoryStore()
	g := NewGraph(WithStore(s))
	if g.Store() != s {
		t.Error("WithStore not applied")
	}
}

func TestGraphWithIdentifier(t *testing.T) {
	id, _ := NewURIRef("http://example.org/g")
	g := NewGraph(WithIdentifier(id))
	if g.Identifier().(URIRef) != id {
		t.Error("WithIdentifier not applied")
	}
}

func TestGraphWithBase(t *testing.T) {
	g := NewGraph(WithBase("http://example.org/"))
	_ = g // just verify construction
}

func TestGraphPredicates(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p1, _ := NewURIRef("http://example.org/p1")
	p2, _ := NewURIRef("http://example.org/p2")
	g.Add(s, p1, NewLiteral("a"))
	g.Add(s, p2, NewLiteral("b"))

	count := 0
	g.Predicates(s, nil)(func(Term) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2 predicates, got %d", count)
	}
}

func TestGraphSubjectPredicates(t *testing.T) {
	g := NewGraph()
	s1, _ := NewURIRef("http://example.org/s1")
	s2, _ := NewURIRef("http://example.org/s2")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	g.Add(s1, p, o)
	g.Add(s2, p, o)

	count := 0
	g.SubjectPredicates(o)(func(Term, Term) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestGraphSubjectObjects(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("a"))
	g.Add(s, p, NewLiteral("b"))

	count := 0
	g.SubjectObjects(&p)(func(Term, Term) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestGraphValuePredicate(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))

	// Look for predicate (s given, o given, p nil)
	val, ok := g.Value(s, nil, NewLiteral("v"))
	if !ok {
		t.Fatal("expected value")
	}
	if val.(URIRef) != p {
		t.Errorf("expected %v, got %v", p, val)
	}
}

func TestGraphValueSubject(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))

	val, ok := g.Value(nil, &p, NewLiteral("v"))
	if !ok {
		t.Fatal("expected subject")
	}
	if val.(URIRef) != s {
		t.Errorf("expected %v, got %v", s, val)
	}
}

func TestGraphValueNotFound(t *testing.T) {
	g := NewGraph()
	p, _ := NewURIRef("http://example.org/p")
	_, ok := g.Value(nil, &p, NewLiteral("nope"))
	if ok {
		t.Error("expected not found")
	}
}

func TestGraphQNameNoMatch(t *testing.T) {
	g := NewGraph()
	got := g.QName("http://unknown.org/Thing")
	if got != "http://unknown.org/Thing" {
		t.Errorf("expected raw URI, got %q", got)
	}
}

func TestGraphConnectedEmpty(t *testing.T) {
	g := NewGraph()
	if !g.Connected() {
		t.Error("empty graph should be connected")
	}
}

func TestGraphConnectedSingleNode(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))
	if !g.Connected() {
		t.Error("single-subject graph should be connected")
	}
}

func TestGraphStoreAccessor(t *testing.T) {
	g := NewGraph()
	if g.Store() == nil {
		t.Error("expected non-nil store")
	}
}
