package sparql

import (
	"context"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// TestSortSolutionsStopsWhenCancelled checks that ORDER BY gives up inside the
// sort rather than finishing its n·log n comparisons. The query-level tests
// cannot pin this down: the rows are usually still being produced when the
// cancellation arrives.
func TestSortSolutionsStopsWhenCancelled(t *testing.T) {
	sols := make([]map[string]rdflibgo.Term, 100_000)
	for i := range sols {
		sols[i] = map[string]rdflibgo.Term{"x": rdflibgo.NewLiteral((i * 7919) % len(sols))}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ec := newEvalCtx(ctx)

	calls := 0
	sortSolutions(ec, sols, func(a, b map[string]rdflibgo.Term) int {
		calls++
		return compareTermValues(a["x"], b["x"])
	})
	if ec.failure() == nil {
		t.Fatal("sort did not record the cancellation")
	}
	if calls >= cancelCheckInterval {
		t.Fatalf("comparator ran %d times after the context was cancelled, want < %d", calls, cancelCheckInterval)
	}
}

func TestSortSolutionsRepanicsForeignPanics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		r := recover()
		if s, ok := r.(string); !ok || !strings.Contains(s, "boom") {
			t.Fatalf("recovered %v, want the comparator's own panic", r)
		}
	}()
	sols := []map[string]rdflibgo.Term{{}, {}}
	sortSolutions(newEvalCtx(ctx), sols, func(a, b map[string]rdflibgo.Term) int { panic("boom") })
}

func TestNilEvalCtxNeverStops(t *testing.T) {
	var ec *evalCtx
	for range 3 * cancelCheckInterval {
		if ec.stop() {
			t.Fatal("nil evalCtx stopped")
		}
	}
	if ec.poll() || ec.cancellable() || ec.failure() != nil {
		t.Fatal("nil evalCtx reports a cancellation")
	}
	if newEvalCtx(context.Background()).cancellable() {
		t.Fatal("context.Background is never done; its evalCtx must not poll")
	}
}
