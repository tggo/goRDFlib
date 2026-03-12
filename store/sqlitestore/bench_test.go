package sqlitestore

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/term"
)

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

func newBenchSQLite(b *testing.B) (*SQLiteStore, func()) {
	s, err := New(WithInMemory())
	if err != nil {
		b.Fatal(err)
	}
	return s, func() { s.Close() }
}

func newBenchSQLiteDisk(b *testing.B) (*SQLiteStore, func()) {
	path := b.TempDir() + "/bench.db"
	s, err := New(WithFile(path))
	if err != nil {
		b.Fatal(err)
	}
	return s, func() { s.Close() }
}

func BenchmarkSQLiteAddN1K(b *testing.B) {
	quads := genQuads(1000)
	for b.Loop() {
		s, cleanup := newBenchSQLite(b)
		s.AddN(quads)
		cleanup()
	}
}

func BenchmarkSQLiteAddN10K(b *testing.B) {
	quads := genQuads(10000)
	for b.Loop() {
		s, cleanup := newBenchSQLite(b)
		s.AddN(quads)
		cleanup()
	}
}

func BenchmarkSQLiteAddN100K(b *testing.B) {
	quads := genQuads(100000)
	for b.Loop() {
		s, cleanup := newBenchSQLite(b)
		s.AddN(quads)
		cleanup()
	}
}

func BenchmarkSQLiteDiskAddN10K(b *testing.B) {
	quads := genQuads(10000)
	for b.Loop() {
		s, cleanup := newBenchSQLiteDisk(b)
		s.AddN(quads)
		cleanup()
	}
}

func BenchmarkSQLiteTriplesScan10K(b *testing.B) {
	s, cleanup := newBenchSQLite(b)
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

func BenchmarkSQLiteTriplesSubject10K(b *testing.B) {
	s, cleanup := newBenchSQLite(b)
	defer cleanup()
	s.AddN(genQuads(10000))
	subj := term.NewURIRefUnsafe("http://example.org/s42")
	b.ResetTimer()
	for b.Loop() {
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
		}
	}
}

func BenchmarkSQLiteTriplesPredicate10K(b *testing.B) {
	s, cleanup := newBenchSQLite(b)
	defer cleanup()
	s.AddN(genQuads(10000))
	pred := term.NewURIRefUnsafe("http://example.org/p42")
	b.ResetTimer()
	for b.Loop() {
		for range s.Triples(term.TriplePattern{Predicate: &pred}, nil) {
		}
	}
}

func BenchmarkSQLiteLen10K(b *testing.B) {
	s, cleanup := newBenchSQLite(b)
	defer cleanup()
	s.AddN(genQuads(10000))
	b.ResetTimer()
	for b.Loop() {
		s.Len(nil)
	}
}

func BenchmarkSQLiteExactLookup10K(b *testing.B) {
	s, cleanup := newBenchSQLite(b)
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
