package paths

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// WithStepLimit bounds the work of a traversal, not its pairs: walking a
// 5,000-node chain towards an object that is not on it yields nothing and
// still stops with ErrStepLimit. The store hides MemoryStore's Reachable, so
// the walk is local; a pushed-down Reachable is one step (see the end).
func TestStepLimit(t *testing.T) {
	ctx := t.Context()
	g := graph.NewGraph(graph.WithStore(struct{ store.Store }{store.NewMemoryStore()}))
	u := func(s string) term.URIRef { return term.NewURIRefUnsafe("urn:" + s) }
	next := u("next")
	for i := range 5000 {
		g.Add(ctx, u(fmt.Sprintf("n%d", i)), next, u(fmt.Sprintf("n%d", i+1)))
	}
	p := ZeroOrMore(URIRefPath{URI: next})
	walk := func(ctx context.Context) (pairs int, err error) {
		for _, e := range p.Eval(ctx, g, u("n0"), u("missing")) {
			if e != nil {
				return pairs, e
			}
			pairs++
		}
		return pairs, nil
	}
	if n, err := walk(WithStepLimit(ctx, 100)); !errors.Is(err, ErrStepLimit) || n != 0 {
		t.Errorf("limit 100: %d pairs, err %v; want ErrStepLimit", n, err)
	}
	for _, lim := range []int{0, -1, 1_000_000} {
		if n, err := walk(WithStepLimit(ctx, lim)); err != nil || n != 0 {
			t.Errorf("limit %d: %d pairs, err %v; want a complete empty walk", lim, n, err)
		}
	}
	if n, err := walk(ctx); err != nil || n != 0 {
		t.Errorf("no limit: %d pairs, err %v", n, err)
	}

	// Over a ReachabilityStore the closure is one Reachable call: its cost
	// is the store's, and the limit counts it as one step.
	rg := graph.NewGraph()
	for i := range 5000 {
		rg.Add(ctx, u(fmt.Sprintf("n%d", i)), next, u(fmt.Sprintf("n%d", i+1)))
	}
	g = rg
	if n, err := walk(WithStepLimit(ctx, 100)); err != nil || n != 0 {
		t.Errorf("pushed down, limit 100: %d pairs, err %v; want a complete empty walk", n, err)
	}
}
