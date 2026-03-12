package badgerstore

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// --- Helpers ---

func benchStore(b *testing.B, s store.Store, cleanup func()) {
	b.Helper()
	if cleanup != nil {
		b.Cleanup(cleanup)
	}
}

func genTriples(n int) []term.Triple {
	triples := make([]term.Triple, n)
	for i := 0; i < n; i++ {
		subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		pred := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/p%d", i%100))
		obj := term.NewLiteral(fmt.Sprintf("value_%d", i))
		triples[i] = term.Triple{Subject: subj, Predicate: pred, Object: obj}
	}
	return triples
}

func genQuads(n int) []term.Quad {
	quads := make([]term.Quad, n)
	for i := 0; i < n; i++ {
		subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		pred := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/p%d", i%100))
		obj := term.NewLiteral(fmt.Sprintf("value_%d", i))
		quads[i] = term.Quad{Triple: term.Triple{Subject: subj, Predicate: pred, Object: obj}}
	}
	return quads
}

func newBenchBadger(b *testing.B) (*BadgerStore, func()) {
	s, err := New(WithInMemory())
	if err != nil {
		b.Fatal(err)
	}
	return s, func() { s.Close() }
}

func newBenchBadgerDisk(b *testing.B) (*BadgerStore, func()) {
	dir := b.TempDir()
	s, err := New(WithDir(dir))
	if err != nil {
		b.Fatal(err)
	}
	return s, func() { s.Close() }
}

// --- Ingestion Benchmarks ---

func BenchmarkBadgerAdd1K(b *testing.B) {
	triples := genTriples(1000)
	for b.Loop() {
		s, cleanup := newBenchBadger(b)
		for _, t := range triples {
			s.Add(t, nil)
		}
		cleanup()
	}
}

func BenchmarkBadgerAddN1K(b *testing.B) {
	quads := genQuads(1000)
	for b.Loop() {
		s, cleanup := newBenchBadger(b)
		s.AddN(quads)
		cleanup()
	}
}

func BenchmarkBadgerAddN10K(b *testing.B) {
	quads := genQuads(10000)
	for b.Loop() {
		s, cleanup := newBenchBadger(b)
		s.AddN(quads)
		cleanup()
	}
}

func BenchmarkBadgerAddN100K(b *testing.B) {
	quads := genQuads(100000)
	for b.Loop() {
		s, cleanup := newBenchBadger(b)
		s.AddN(quads)
		cleanup()
	}
}

func BenchmarkBadgerDiskAddN10K(b *testing.B) {
	quads := genQuads(10000)
	for b.Loop() {
		s, cleanup := newBenchBadgerDisk(b)
		s.AddN(quads)
		cleanup()
	}
}

func BenchmarkMemoryAdd1K(b *testing.B) {
	triples := genTriples(1000)
	for b.Loop() {
		s := store.NewMemoryStore()
		for _, t := range triples {
			s.Add(t, nil)
		}
	}
}

func BenchmarkMemoryAddN1K(b *testing.B) {
	quads := genQuads(1000)
	for b.Loop() {
		s := store.NewMemoryStore()
		s.AddN(quads)
	}
}

func BenchmarkMemoryAddN10K(b *testing.B) {
	quads := genQuads(10000)
	for b.Loop() {
		s := store.NewMemoryStore()
		s.AddN(quads)
	}
}

func BenchmarkMemoryAddN100K(b *testing.B) {
	quads := genQuads(100000)
	for b.Loop() {
		s := store.NewMemoryStore()
		s.AddN(quads)
	}
}

// --- Query Benchmarks ---

func BenchmarkBadgerTriplesScan10K(b *testing.B) {
	s, cleanup := newBenchBadger(b)
	defer cleanup()
	s.AddN(genQuads(10000))

	b.ResetTimer()
	for b.Loop() {
		count := 0
		for range s.Triples(term.TriplePattern{}, nil) {
			count++
		}
	}
}

func BenchmarkMemoryTriplesScan10K(b *testing.B) {
	s := store.NewMemoryStore()
	s.AddN(genQuads(10000))

	b.ResetTimer()
	for b.Loop() {
		count := 0
		for range s.Triples(term.TriplePattern{}, nil) {
			count++
		}
	}
}

func BenchmarkBadgerTriplesSubject10K(b *testing.B) {
	s, cleanup := newBenchBadger(b)
	defer cleanup()
	s.AddN(genQuads(10000))
	subj := term.NewURIRefUnsafe("http://example.org/s42")

	b.ResetTimer()
	for b.Loop() {
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
		}
	}
}

func BenchmarkMemoryTriplesSubject10K(b *testing.B) {
	s := store.NewMemoryStore()
	s.AddN(genQuads(10000))
	subj := term.NewURIRefUnsafe("http://example.org/s42")

	b.ResetTimer()
	for b.Loop() {
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
		}
	}
}

func BenchmarkBadgerTriplesPredicate10K(b *testing.B) {
	s, cleanup := newBenchBadger(b)
	defer cleanup()
	s.AddN(genQuads(10000))
	pred := term.NewURIRefUnsafe("http://example.org/p42")

	b.ResetTimer()
	for b.Loop() {
		for range s.Triples(term.TriplePattern{Predicate: &pred}, nil) {
		}
	}
}

func BenchmarkMemoryTriplesPredicate10K(b *testing.B) {
	s := store.NewMemoryStore()
	s.AddN(genQuads(10000))
	pred := term.NewURIRefUnsafe("http://example.org/p42")

	b.ResetTimer()
	for b.Loop() {
		for range s.Triples(term.TriplePattern{Predicate: &pred}, nil) {
		}
	}
}

func BenchmarkBadgerLen10K(b *testing.B) {
	s, cleanup := newBenchBadger(b)
	defer cleanup()
	s.AddN(genQuads(10000))

	b.ResetTimer()
	for b.Loop() {
		s.Len(nil)
	}
}

func BenchmarkMemoryLen10K(b *testing.B) {
	s := store.NewMemoryStore()
	s.AddN(genQuads(10000))

	b.ResetTimer()
	for b.Loop() {
		s.Len(nil)
	}
}

func BenchmarkBadgerExactLookup10K(b *testing.B) {
	s, cleanup := newBenchBadger(b)
	defer cleanup()
	s.AddN(genQuads(10000))
	subj := term.NewURIRefUnsafe("http://example.org/s5000")
	pred := term.NewURIRefUnsafe("http://example.org/p0")
	obj := term.NewLiteral("value_5000")

	b.ResetTimer()
	for b.Loop() {
		for range s.Triples(term.TriplePattern{Subject: subj, Predicate: &pred, Object: obj}, nil) {
		}
	}
}

func BenchmarkMemoryExactLookup10K(b *testing.B) {
	s := store.NewMemoryStore()
	s.AddN(genQuads(10000))
	subj := term.NewURIRefUnsafe("http://example.org/s5000")
	pred := term.NewURIRefUnsafe("http://example.org/p0")
	obj := term.NewLiteral("value_5000")

	b.ResetTimer()
	for b.Loop() {
		for range s.Triples(term.TriplePattern{Subject: subj, Predicate: &pred, Object: obj}, nil) {
		}
	}
}
