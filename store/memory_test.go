package store

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/term"
)

func TestMemoryStoreAddAndLen(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	obj := term.NewLiteral("hello")
	s.Add(term.Triple{Subject: sub, Predicate: pred, Object: obj}, nil)
	if s.Len(nil) != 1 { t.Errorf("expected 1, got %d", s.Len(nil)) }
}

func TestMemoryStoreDuplicateAdd(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	obj := term.NewLiteral("hello")
	s.Add(term.Triple{Subject: sub, Predicate: pred, Object: obj}, nil)
	s.Add(term.Triple{Subject: sub, Predicate: pred, Object: obj}, nil)
	if s.Len(nil) != 1 { t.Errorf("duplicate add should not increase count, got %d", s.Len(nil)) }
}

func TestMemoryStoreRemove(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	obj := term.NewLiteral("hello")
	s.Add(term.Triple{Subject: sub, Predicate: pred, Object: obj}, nil)
	s.Remove(term.TriplePattern{Subject: sub, Predicate: &pred, Object: obj}, nil)
	if s.Len(nil) != 0 { t.Errorf("expected 0 after remove, got %d", s.Len(nil)) }
}

func TestMemoryStoreTriplesSubjectPattern(t *testing.T) {
	s := NewMemoryStore()
	s1, _ := term.NewURIRef("http://example.org/s1")
	s2, _ := term.NewURIRef("http://example.org/s2")
	p, _ := term.NewURIRef("http://example.org/p")
	o := term.NewLiteral("v")
	s.Add(term.Triple{Subject: s1, Predicate: p, Object: o}, nil)
	s.Add(term.Triple{Subject: s2, Predicate: p, Object: o}, nil)
	count := 0
	s.Triples(term.TriplePattern{Subject: s1}, nil)(func(term.Triple) bool { count++; return true })
	if count != 1 { t.Errorf("expected 1 match for s1, got %d", count) }
}

func TestMemoryStoreTriplesPredPattern(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	p1, _ := term.NewURIRef("http://example.org/p1")
	p2, _ := term.NewURIRef("http://example.org/p2")
	o := term.NewLiteral("v")
	s.Add(term.Triple{Subject: sub, Predicate: p1, Object: o}, nil)
	s.Add(term.Triple{Subject: sub, Predicate: p2, Object: o}, nil)
	count := 0
	s.Triples(term.TriplePattern{Predicate: &p1}, nil)(func(term.Triple) bool { count++; return true })
	if count != 1 { t.Errorf("expected 1 match for p1, got %d", count) }
}

func TestMemoryStoreTriplesAll(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	s.Add(term.Triple{Subject: sub, Predicate: p, Object: term.NewLiteral("a")}, nil)
	s.Add(term.Triple{Subject: sub, Predicate: p, Object: term.NewLiteral("b")}, nil)
	count := 0
	s.Triples(term.TriplePattern{}, nil)(func(term.Triple) bool { count++; return true })
	if count != 2 { t.Errorf("expected 2, got %d", count) }
}

func TestMemoryStoreAddNConcurrent(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	var quads []term.Quad
	for i := 0; i < 100; i++ {
		quads = append(quads, term.Quad{Triple: term.Triple{Subject: sub, Predicate: pred, Object: term.NewLiteral(fmt.Sprintf("v%d", i))}})
	}
	done := make(chan struct{})
	go func() { s.AddN(quads); close(done) }()
	for i := 0; i < 10; i++ { _ = s.Len(nil) }
	<-done
	if s.Len(nil) != 100 { t.Errorf("expected 100 after AddN, got %d", s.Len(nil)) }
}

func TestMemoryStoreRemoveConcurrent(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	for i := 0; i < 50; i++ {
		s.Add(term.Triple{Subject: sub, Predicate: pred, Object: term.NewLiteral(fmt.Sprintf("v%d", i))}, nil)
	}
	done := make(chan struct{})
	go func() { s.Remove(term.TriplePattern{Subject: sub}, nil); close(done) }()
	for i := 0; i < 10; i++ { _ = s.Len(nil) }
	<-done
	if s.Len(nil) != 0 { t.Errorf("expected 0 after remove, got %d", s.Len(nil)) }
}

func TestMemoryStoreObjectPattern(t *testing.T) {
	s := NewMemoryStore()
	s1, _ := term.NewURIRef("http://example.org/s1")
	s2, _ := term.NewURIRef("http://example.org/s2")
	p, _ := term.NewURIRef("http://example.org/p")
	o := term.NewLiteral("v")
	s.Add(term.Triple{Subject: s1, Predicate: p, Object: o}, nil)
	s.Add(term.Triple{Subject: s2, Predicate: p, Object: o}, nil)
	count := 0
	s.Triples(term.TriplePattern{Object: o}, nil)(func(term.Triple) bool { count++; return true })
	if count != 2 { t.Errorf("expected 2 match for object pattern, got %d", count) }
}

func TestMemoryStoreExactPattern(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	obj := term.NewLiteral("v")
	s.Add(term.Triple{Subject: sub, Predicate: pred, Object: obj}, nil)
	count := 0
	s.Triples(term.TriplePattern{Subject: sub, Predicate: &pred, Object: obj}, nil)(func(term.Triple) bool { count++; return true })
	if count != 1 { t.Errorf("expected 1, got %d", count) }
}

func TestMemoryStoreNamespaces(t *testing.T) {
	s := NewMemoryStore()
	ns, _ := term.NewURIRef("http://example.org/ns#")
	s.Bind("ex", ns)
	got, ok := s.Namespace("ex")
	if !ok || got != ns { t.Errorf("namespace lookup failed") }
	prefix, ok := s.Prefix(ns)
	if !ok || prefix != "ex" { t.Errorf("prefix lookup failed") }
}

func TestMemoryStoreContexts(t *testing.T) {
	s := NewMemoryStore()
	count := 0
	s.Contexts(nil)(func(term.Term) bool { count++; return true })
	if count != 0 { t.Errorf("expected 0 contexts, got %d", count) }
}

func BenchmarkMemoryStoreAdd(b *testing.B) {
	s := NewMemoryStore()
	pred, _ := term.NewURIRef("http://example.org/p")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sub := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		s.Add(term.Triple{Subject: sub, Predicate: pred, Object: term.NewLiteral(i)}, nil)
	}
}

func BenchmarkMemoryStoreTriples(b *testing.B) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	for i := 0; i < 1000; i++ {
		s.Add(term.Triple{Subject: sub, Predicate: pred, Object: term.NewLiteral(i)}, nil)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Triples(term.TriplePattern{Subject: sub}, nil)(func(term.Triple) bool { return true })
	}
}
