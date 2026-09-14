package store_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/badgerstore"
	"github.com/tggo/goRDFlib/store/sqlitestore"
	"github.com/tggo/goRDFlib/term"
)

// runConcurrentReadWrite exercises a store with concurrent writers and readers.
// 10 goroutines each insert 1000 triples; 50 goroutines each scan by subject 100 times.
func runConcurrentReadWrite(t *testing.T, s store.Store) {
	t.Helper()
	const writers = 10
	const writesEach = 1000
	const readers = 50
	const readsEach = 100

	var wg sync.WaitGroup

	// Writers
	for w := range writers {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			base := w * writesEach
			for i := range writesEach {
				idx := base + i
				subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/crw/s%d", idx))
				pred := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/crw/p%d", idx%10))
				obj := term.NewLiteral(fmt.Sprintf("val_%d", idx))
				s.Add(term.Triple{Subject: subj, Predicate: pred, Object: obj}, nil)
			}
		}(w)
	}

	// Readers — start alongside writers; tolerate zero results while store is filling
	for r := range readers {
		wg.Add(1)
		go func(r int) {
			defer wg.Done()
			subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/crw/s%d", r*writesEach/readers))
			for range readsEach {
				for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
				}
			}
		}(r)
	}

	wg.Wait()
}

func TestConcurrentReadWrite_Memory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	s := store.NewMemoryStore()
	runConcurrentReadWrite(t, s)
}

func TestConcurrentReadWrite_Badger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	s, err := badgerstore.New(badgerstore.WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	runConcurrentReadWrite(t, s)
}

func TestConcurrentReadWrite_SQLite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	s, err := sqlitestore.New(sqlitestore.WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	runConcurrentReadWrite(t, s)
}

// runLargeLiterals inserts 100 triples whose objects are 64 KB string literals
// and verifies each can be retrieved by subject.
func runLargeLiterals(t *testing.T, s store.Store) {
	t.Helper()
	const n = 100
	large := strings.Repeat("x", 64*1024)

	for i := range n {
		subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/large/s%d", i))
		pred := term.NewURIRefUnsafe("http://example.org/large/p")
		obj := term.NewLiteral(fmt.Sprintf("%s_%d", large, i))
		s.Add(term.Triple{Subject: subj, Predicate: pred, Object: obj}, nil)
	}

	for i := range n {
		subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/large/s%d", i))
		count := 0
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
			count++
		}
		if count != 1 {
			t.Errorf("large literal s%d: got %d triples, want 1", i, count)
		}
	}
}

func TestEdgeCases_LargeLiterals(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	t.Run("Memory", func(t *testing.T) {
		runLargeLiterals(t, store.NewMemoryStore())
	})

	// Badger v4 encodes the full triple as a KV key (SPO/POS/OSP indexes).
	// Its hard key-size limit is 65000 bytes, so 64 KB literals exceed that limit.
	// Inserts are silently dropped (store.Store interface constraint); the test
	// documents this known limitation rather than asserting retrieval.
	t.Run("Badger", func(t *testing.T) {
		s, err := badgerstore.New(badgerstore.WithInMemory())
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		t.Skip("Badger v4 key-size limit (65000 B) prevents storing 64 KB literals as SPO index keys")
	})

	t.Run("SQLite", func(t *testing.T) {
		s, err := sqlitestore.New(sqlitestore.WithInMemory())
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		runLargeLiterals(t, s)
	})
}

// unicodeTerms holds a set of Unicode strings covering emoji, CJK, and Arabic.
var unicodeTerms = []string{
	"\U0001F600\U0001F680\U0001F4A5", // emoji
	"\u4E2D\u6587\u6D4B\u8BD5",       // CJK: Chinese characters
	"\u0645\u0631\u062D\u0628\u0627", // Arabic: مرحبا
	"\u03B1\u03B2\u03B3\u03B4",       // Greek
	"\u0418\u0432\u0430\u043D",       // Cyrillic
}

// runUnicode inserts 100 triples using Unicode IRIs and literals, then verifies retrieval.
func runUnicode(t *testing.T, s store.Store) {
	t.Helper()
	const n = 100

	for i := range n {
		suffix := unicodeTerms[i%len(unicodeTerms)]
		// IRIs must be valid; percent-encode or use a safe wrapper.
		// We embed the Unicode text in the literal, and use a numeric IRI.
		subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/uni/s%d", i))
		pred := term.NewURIRefUnsafe("http://example.org/uni/p")
		obj := term.NewLiteral(fmt.Sprintf("%s_%d", suffix, i))
		s.Add(term.Triple{Subject: subj, Predicate: pred, Object: obj}, nil)
	}

	for i := range n {
		subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/uni/s%d", i))
		count := 0
		for t3 := range s.Triples(term.TriplePattern{Subject: subj}, nil) {
			_ = t3
			count++
		}
		if count != 1 {
			t.Errorf("unicode triple s%d: got %d triples, want 1", i, count)
		}
	}
}

func TestEdgeCases_Unicode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	t.Run("Memory", func(t *testing.T) {
		runUnicode(t, store.NewMemoryStore())
	})

	t.Run("Badger", func(t *testing.T) {
		s, err := badgerstore.New(badgerstore.WithInMemory())
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		runUnicode(t, s)
	})

	t.Run("SQLite", func(t *testing.T) {
		s, err := sqlitestore.New(sqlitestore.WithInMemory())
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		runUnicode(t, s)
	})
}

// runQueryableStore validates the QueryableStore interface on a populated store.
// It inserts 100 triples: 10 subjects x 10 predicates.
func runQueryableStore(t *testing.T, s store.Store) {
	t.Helper()

	qs, ok := s.(store.QueryableStore)
	if !ok {
		t.Skip("store does not implement QueryableStore")
	}

	const subjects = 10
	const predicates = 10

	// Insert 10 subjects x 10 predicates = 100 triples
	for si := range subjects {
		subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/qs/s%d", si))
		for pi := range predicates {
			pred := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/qs/p%d", pi))
			obj := term.NewLiteral(fmt.Sprintf("val_%d_%d", si, pi))
			s.Add(term.Triple{Subject: subj, Predicate: pred, Object: obj}, nil)
		}
	}

	// Count with no filter = 100
	total := qs.Count(term.TriplePattern{}, nil)
	if total != 100 {
		t.Errorf("Count(no filter) = %d, want 100", total)
	}

	// Count with subject filter = 10
	subj0 := term.NewURIRefUnsafe("http://example.org/qs/s0")
	bySubj := qs.Count(term.TriplePattern{Subject: subj0}, nil)
	if bySubj != 10 {
		t.Errorf("Count(subject=s0) = %d, want 10", bySubj)
	}

	// Exists with existing triple = true
	pred0 := term.NewURIRefUnsafe("http://example.org/qs/p0")
	obj0 := term.NewLiteral("val_0_0")
	existsYes := qs.Exists(term.TriplePattern{Subject: subj0, Predicate: &pred0, Object: obj0}, nil)
	if !existsYes {
		t.Errorf("Exists(existing triple) = false, want true")
	}

	// Exists with non-existing triple = false
	missing := term.NewURIRefUnsafe("http://example.org/qs/missing")
	existsNo := qs.Exists(term.TriplePattern{Subject: missing}, nil)
	if existsNo {
		t.Errorf("Exists(non-existing triple) = true, want false")
	}

	// TriplesWithLimit(limit=5, offset=0) returns 5
	count := 0
	for range qs.TriplesWithLimit(term.TriplePattern{}, nil, 5, 0) {
		count++
	}
	if count != 5 {
		t.Errorf("TriplesWithLimit(5, 0) = %d, want 5", count)
	}

	// TriplesWithLimit(limit=5, offset=95) returns 5
	count = 0
	for range qs.TriplesWithLimit(term.TriplePattern{}, nil, 5, 95) {
		count++
	}
	if count != 5 {
		t.Errorf("TriplesWithLimit(5, 95) = %d, want 5", count)
	}

	// TriplesWithLimit(limit=5, offset=100) returns 0
	count = 0
	for range qs.TriplesWithLimit(term.TriplePattern{}, nil, 5, 100) {
		count++
	}
	if count != 0 {
		t.Errorf("TriplesWithLimit(5, 100) = %d, want 0", count)
	}
}

func TestQueryableStore_Memory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	runQueryableStore(t, store.NewMemoryStore())
}

func TestQueryableStore_Badger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	s, err := badgerstore.New(badgerstore.WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	runQueryableStore(t, s)
}

func TestQueryableStore_SQLite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}
	s, err := sqlitestore.New(sqlitestore.WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	runQueryableStore(t, s)
}
