package paths

import (
	"fmt"
	"sync"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// A transitive path with both ends bound gives the pair once when the object
// is reachable, and nothing otherwise.
func TestTransitiveBothEndsBound(t *testing.T) {
	ctx := t.Context()
	g := graph.NewGraph()
	u := func(s string) term.URIRef { return term.NewURIRefUnsafe("urn:" + s) }
	p := u("p")
	g.Add(ctx, u("a"), p, u("b"))
	g.Add(ctx, u("b"), p, u("c"))
	g.Add(ctx, u("a"), p, u("c")) // two routes a->c
	g.Add(ctx, u("c"), p, term.NewLiteral("leaf"))
	ctx = WithReachabilityCache(ctx)
	for _, tc := range []struct {
		s    string
		o    term.Term
		want int
	}{
		{"a", u("c"), 1}, {"a", u("b"), 1}, {"c", u("a"), 0}, {"a", u("a"), 0},
		{"a", term.NewLiteral("leaf"), 1}, {"a", term.NewLiteral("other"), 0},
	} {
		for _, path := range []Path{OneOrMore(AsPath(p)), ZeroOrMore(AsPath(p))} {
			want := tc.want
			if path.(*MulPath).Zero && tc.o.N3() == u(tc.s).N3() {
				want = 1 // the zero-length pair
			}
			n := 0
			for _, err := range path.Eval(ctx, g, u(tc.s), tc.o) {
				if err != nil {
					t.Fatal(err)
				}
				n++
			}
			if n != want {
				t.Errorf("%s %s %s: %d pairs, want %d", tc.s, path.pathString(), tc.o.N3(), n, want)
			}
		}
	}
}

// Goroutines sharing one cache test pairs against the same answer (run with
// -race).
func TestTransitiveBothEndsBoundSharedCache(t *testing.T) {
	ctx := t.Context()
	g := graph.NewGraph()
	u := func(s string) term.URIRef { return term.NewURIRefUnsafe("urn:" + s) }
	p := u("p")
	for i := range 100 {
		g.Add(ctx, u(fmt.Sprintf("n%d", i)), p, u(fmt.Sprintf("n%d", i+1)))
	}
	ctx = WithReachabilityCache(ctx)
	var wg sync.WaitGroup
	for w := range 8 {
		wg.Go(func() {
			for i := range 120 {
				k := (i + w) % 120
				want := 0
				if k >= 1 && k <= 100 {
					want = 1 // n1..n100 are below n0
				}
				n := 0
				for _, err := range OneOrMore(AsPath(p)).Eval(ctx, g, u("n0"), u(fmt.Sprintf("n%d", k))) {
					if err != nil {
						t.Error(err)
						return
					}
					n++
				}
				if n != want {
					t.Errorf("n0 p+ n%d: %d pairs, want %d", k, n, want)
				}
			}
		})
	}
	wg.Wait()
}
