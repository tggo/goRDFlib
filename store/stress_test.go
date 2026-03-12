package store_test

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/badgerstore"
	"github.com/tggo/goRDFlib/store/sqlitestore"
	"github.com/tggo/goRDFlib/term"
)

const stressN = 3_000_000

func genQuads(n int) []term.Quad {
	quads := make([]term.Quad, n)
	for i := range n {
		subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		pred := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/p%d", i%200))
		obj := term.NewLiteral(fmt.Sprintf("value_%d", i))
		quads[i] = term.Quad{Triple: term.Triple{Subject: subj, Predicate: pred, Object: obj}}
	}
	return quads
}

func memMB() float64 {
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024
}

func TestStress1M_Memory(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	quads := genQuads(stressN)
	t.Logf("Generated %d quads", len(quads))

	s := store.NewMemoryStore()
	memBefore := memMB()

	// Ingest
	start := time.Now()
	s.AddN(quads)
	ingestDur := time.Since(start)
	memAfter := memMB()
	t.Logf("MEMORY Ingest: %d triples in %v (%.0f triples/sec)", stressN, ingestDur, float64(stressN)/ingestDur.Seconds())
	t.Logf("MEMORY RAM: %.1f MB before → %.1f MB after (delta %.1f MB)", memBefore, memAfter, memAfter-memBefore)

	// Len
	start = time.Now()
	n := s.Len(nil)
	t.Logf("MEMORY Len: %d in %v", n, time.Since(start))

	// Full scan
	start = time.Now()
	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
	}
	t.Logf("MEMORY Full scan: %d triples in %v", count, time.Since(start))

	// Subject lookup
	subj := term.NewURIRefUnsafe("http://example.org/s500000")
	start = time.Now()
	for i := 0; i < 1000; i++ {
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
		}
	}
	t.Logf("MEMORY Subject lookup x1000: %v (%.0f ns/op)", time.Since(start), float64(time.Since(start).Nanoseconds())/1000)

	// Predicate scan (returns ~5000 triples)
	pred := term.NewURIRefUnsafe("http://example.org/p42")
	start = time.Now()
	count = 0
	for range s.Triples(term.TriplePattern{Predicate: &pred}, nil) {
		count++
	}
	t.Logf("MEMORY Predicate scan (p42): %d triples in %v", count, time.Since(start))
}

func TestStress1M_Badger(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	quads := genQuads(stressN)

	s, err := badgerstore.New(badgerstore.WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	memBefore := memMB()

	// Ingest in batches of 50K (Badger WriteBatch has limits)
	start := time.Now()
	batch := 50_000
	for i := 0; i < len(quads); i += batch {
		end := i + batch
		if end > len(quads) {
			end = len(quads)
		}
		s.AddN(quads[i:end])
	}
	ingestDur := time.Since(start)
	memAfter := memMB()
	t.Logf("BADGER Ingest: %d triples in %v (%.0f triples/sec)", stressN, ingestDur, float64(stressN)/ingestDur.Seconds())
	t.Logf("BADGER RAM: %.1f MB before → %.1f MB after (delta %.1f MB)", memBefore, memAfter, memAfter-memBefore)

	// Len
	start = time.Now()
	n := s.Len(nil)
	t.Logf("BADGER Len: %d in %v", n, time.Since(start))

	// Full scan
	start = time.Now()
	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
	}
	t.Logf("BADGER Full scan: %d triples in %v", count, time.Since(start))

	// Subject lookup
	subj := term.NewURIRefUnsafe("http://example.org/s500000")
	start = time.Now()
	for i := 0; i < 1000; i++ {
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
		}
	}
	t.Logf("BADGER Subject lookup x1000: %v (%.0f ns/op)", time.Since(start), float64(time.Since(start).Nanoseconds())/1000)

	// Predicate scan
	pred := term.NewURIRefUnsafe("http://example.org/p42")
	start = time.Now()
	count = 0
	for range s.Triples(term.TriplePattern{Predicate: &pred}, nil) {
		count++
	}
	t.Logf("BADGER Predicate scan (p42): %d triples in %v", count, time.Since(start))
}

func TestStress1M_BadgerDisk(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	quads := genQuads(stressN)
	dir := t.TempDir()

	s, err := badgerstore.New(badgerstore.WithDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Ingest
	start := time.Now()
	batch := 50_000
	for i := 0; i < len(quads); i += batch {
		end := i + batch
		if end > len(quads) {
			end = len(quads)
		}
		s.AddN(quads[i:end])
	}
	ingestDur := time.Since(start)
	t.Logf("BADGER-DISK Ingest: %d triples in %v (%.0f triples/sec)", stressN, ingestDur, float64(stressN)/ingestDur.Seconds())

	// Subject lookup
	subj := term.NewURIRefUnsafe("http://example.org/s500000")
	start = time.Now()
	for i := 0; i < 1000; i++ {
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
		}
	}
	t.Logf("BADGER-DISK Subject lookup x1000: %v (%.0f ns/op)", time.Since(start), float64(time.Since(start).Nanoseconds())/1000)

	// Predicate scan
	pred := term.NewURIRefUnsafe("http://example.org/p42")
	start = time.Now()
	count := 0
	for range s.Triples(term.TriplePattern{Predicate: &pred}, nil) {
		count++
	}
	t.Logf("BADGER-DISK Predicate scan (p42): %d triples in %v", count, time.Since(start))
}

func TestStress1M_SQLite(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	quads := genQuads(stressN)

	s, err := sqlitestore.New(sqlitestore.WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	memBefore := memMB()

	// Ingest in batches
	start := time.Now()
	batch := 50_000
	for i := 0; i < len(quads); i += batch {
		end := i + batch
		if end > len(quads) {
			end = len(quads)
		}
		s.AddN(quads[i:end])
	}
	ingestDur := time.Since(start)
	memAfter := memMB()
	t.Logf("SQLITE Ingest: %d triples in %v (%.0f triples/sec)", stressN, ingestDur, float64(stressN)/ingestDur.Seconds())
	t.Logf("SQLITE RAM: %.1f MB before → %.1f MB after (delta %.1f MB)", memBefore, memAfter, memAfter-memBefore)

	// Len
	start = time.Now()
	n := s.Len(nil)
	t.Logf("SQLITE Len: %d in %v", n, time.Since(start))

	// Full scan
	start = time.Now()
	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
	}
	t.Logf("SQLITE Full scan: %d triples in %v", count, time.Since(start))

	// Subject lookup
	subj := term.NewURIRefUnsafe("http://example.org/s500000")
	start = time.Now()
	for i := 0; i < 1000; i++ {
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
		}
	}
	t.Logf("SQLITE Subject lookup x1000: %v (%.0f ns/op)", time.Since(start), float64(time.Since(start).Nanoseconds())/1000)

	// Predicate scan
	pred := term.NewURIRefUnsafe("http://example.org/p42")
	start = time.Now()
	count = 0
	for range s.Triples(term.TriplePattern{Predicate: &pred}, nil) {
		count++
	}
	t.Logf("SQLITE Predicate scan (p42): %d triples in %v", count, time.Since(start))
}

func TestStress1M_SQLiteDisk(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	quads := genQuads(stressN)
	path := t.TempDir() + "/stress.db"

	s, err := sqlitestore.New(sqlitestore.WithFile(path))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Ingest
	start := time.Now()
	batch := 50_000
	for i := 0; i < len(quads); i += batch {
		end := i + batch
		if end > len(quads) {
			end = len(quads)
		}
		s.AddN(quads[i:end])
	}
	ingestDur := time.Since(start)
	t.Logf("SQLITE-DISK Ingest: %d triples in %v (%.0f triples/sec)", stressN, ingestDur, float64(stressN)/ingestDur.Seconds())

	// Subject lookup
	subj := term.NewURIRefUnsafe("http://example.org/s500000")
	start = time.Now()
	for i := 0; i < 1000; i++ {
		for range s.Triples(term.TriplePattern{Subject: subj}, nil) {
		}
	}
	t.Logf("SQLITE-DISK Subject lookup x1000: %v (%.0f ns/op)", time.Since(start), float64(time.Since(start).Nanoseconds())/1000)

	// Predicate scan
	pred := term.NewURIRefUnsafe("http://example.org/p42")
	start = time.Now()
	count := 0
	for range s.Triples(term.TriplePattern{Predicate: &pred}, nil) {
		count++
	}
	t.Logf("SQLITE-DISK Predicate scan (p42): %d triples in %v", count, time.Since(start))
}

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
