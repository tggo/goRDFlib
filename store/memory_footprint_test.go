package store

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/tggo/goRDFlib/term"
)

// heapPerTriple loads n triples <s/i> schema:name "name i" (issue #53's
// shape: distinct subjects, a unique literal object per triple) and returns
// the retained heap per triple: index overhead alone, and with the terms.
func heapPerTriple(t testing.TB, n int) (index, total float64) {
	t.Helper()
	ctx := context.Background()
	var start runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&start)
	p := term.NewURIRefUnsafe("https://schema.org/name")
	triples := make([]term.Triple, n)
	for i := range triples {
		triples[i] = term.Triple{
			Subject:   term.NewURIRefUnsafe(fmt.Sprintf("https://example.test/s/%d", i)),
			Predicate: p,
			Object:    term.NewLiteral(fmt.Sprintf("name %d", i)),
		}
	}
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	s := NewMemoryStore()
	for _, tr := range triples {
		if err := s.Add(ctx, tr, nil); err != nil {
			t.Fatal(err)
		}
	}
	triples = nil
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(s)
	index = float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)) / float64(n)
	total = float64(int64(after.HeapAlloc)-int64(start.HeapAlloc)) / float64(n)
	return index, total
}

func TestMemoryStoreFootprint(t *testing.T) {
	if testing.Short() {
		t.Skip("allocates ~100 MB")
	}
	// Index overhead only (terms are allocated before the measurement).
	// Nested Go maps cost ~1400 B/triple here; smallMap inner levels ~400.
	const budget = 600
	got, total := heapPerTriple(t, 200_000)
	t.Logf("%.0f bytes/triple in the index, %.0f with terms", got, total)
	if got > budget {
		t.Errorf("MemoryStore retains %.0f bytes/triple, budget %d (issue #53)", got, budget)
	}
}
