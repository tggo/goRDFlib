package paths_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/term"
)

var next = term.NewURIRefUnsafe("http://example.org/next")

// chain links n nodes with ex:next.
func chain(n int) *graph.Graph {
	g := graph.NewGraph()
	for i := range n - 1 {
		g.Add(term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/n%d", i)), next,
			term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/n%d", i+1)))
	}
	return g
}

// TestEvalContext_CancelStopsTraversal cancels p* over a 5,000-node chain
// with neither end bound: 12.5 million pairs, seconds of work. The consumer
// never stops on its own, so only the traversal's own checks can end it.
func TestEvalContext_CancelStopsTraversal(t *testing.T) {
	g := chain(5_000)
	for _, tt := range []struct {
		name string
		path paths.Path
	}{
		{"ZeroOrMore", paths.ZeroOrMore(paths.AsPath(next))},
		// Nested alternative and sequence: their own loops must pass the
		// stop on instead of moving to the next arm or the next step.
		{"OneOrMore of alternative and sequence", paths.OneOrMore(paths.Alternative(paths.Sequence(paths.AsPath(next)), paths.AsPath(next)))},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
			defer cancel()
			start := time.Now()
			pairs := 0
			paths.EvalContext(ctx, tt.path, g, nil, nil)(func(term.Term, term.Term) bool {
				pairs++
				return true
			})
			elapsed := time.Since(start)
			t.Logf("%d pairs in %v", pairs, elapsed)
			if ctx.Err() == nil {
				t.Fatal("traversal finished before the deadline; the test proves nothing")
			}
			if elapsed > 270*time.Millisecond {
				t.Fatalf("traversal ran %v with a 20ms deadline", elapsed)
			}
		})
	}
}

func TestEvalContext_UncancelledMatchesEval(t *testing.T) {
	g := chain(50)
	p := paths.OneOrMore(paths.AsPath(next))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	count := func(it func(func(term.Term, term.Term) bool)) int {
		n := 0
		it(func(term.Term, term.Term) bool { n++; return true })
		return n
	}
	want := count(p.Eval(g, nil, nil))
	if want != 49*50/2 {
		t.Fatalf("Eval gave %d pairs, want %d", want, 49*50/2)
	}
	if got := count(paths.EvalContext(ctx, p, g, nil, nil)); got != want {
		t.Fatalf("EvalContext gave %d pairs, Eval %d", got, want)
	}
	if got := count(paths.EvalContext(context.Background(), p, g, nil, nil)); got != want {
		t.Fatalf("EvalContext(Background) gave %d pairs, Eval %d", got, want)
	}
	cancel()
	if got := count(paths.EvalContext(ctx, p, g, nil, nil)); got != 0 {
		t.Fatalf("EvalContext with a cancelled context yielded %d pairs", got)
	}
}

// TestMulPath_ConsumerStopIsFinal checks that a repetition does not call yield
// again after yield returned false. The recursion used to ignore that at every
// level above the one that emitted.
func TestMulPath_ConsumerStopIsFinal(t *testing.T) {
	g := chain(20)
	for _, tt := range []struct {
		name string
		path paths.Path
		subj term.Subject
		obj  term.Term
	}{
		{"both unbound", paths.OneOrMore(paths.Sequence(paths.AsPath(next))), nil, nil},
		{"subject bound", paths.OneOrMore(paths.Sequence(paths.AsPath(next))), term.NewURIRefUnsafe("http://example.org/n0"), nil},
		{"object bound", paths.OneOrMore(paths.Sequence(paths.AsPath(next))), nil, term.NewURIRefUnsafe("http://example.org/n19")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			tt.path.Eval(g, tt.subj, tt.obj)(func(term.Term, term.Term) bool {
				calls++
				return false
			})
			if calls != 1 {
				t.Fatalf("yield called %d times after returning false, want 1 call", calls)
			}
		})
	}
}
