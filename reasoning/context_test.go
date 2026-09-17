package reasoning

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

type spanKey struct{}

// spanStore records the span of every read and write (issue #35).
type spanStore struct {
	store.Store
	ctx   context.Context
	mu    *sync.Mutex
	spans *[]string
}

func (s *spanStore) BindContext(ctx context.Context) store.Store {
	c := *s
	c.ctx = ctx
	return &c
}

func (s *spanStore) record() {
	v := ""
	if s.ctx != nil {
		v, _ = s.ctx.Value(spanKey{}).(string)
	}
	s.mu.Lock()
	*s.spans = append(*s.spans, v)
	s.mu.Unlock()
}

func (s *spanStore) Add(t term.Triple, g term.Term) { s.record(); s.Store.Add(t, g) }
func (s *spanStore) Triples(p term.TriplePattern, g term.Term) store.TripleIterator {
	s.record()
	return s.Store.Triples(p, g)
}

func TestExpandContextReachesStore(t *testing.T) {
	var spans []string
	st := &spanStore{Store: store.NewMemoryStore(), mu: &sync.Mutex{}, spans: &spans}
	g := graph.NewGraph(graph.WithStore(st))
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	g.Add(ex("Dog"), namespace.RDFS.SubClassOf, ex("Animal"))
	g.Add(ex("rex"), namespace.RDF.Type, ex("Dog"))
	g.Add(ex("owns"), namespace.OWL.InverseOf, ex("ownedBy"))
	g.Add(ex("ann"), ex("owns"), ex("rex"))
	spans = nil

	n, err := ExpandContext(context.WithValue(context.Background(), spanKey{}, "span-1"), g, RDFS|OWLRL)
	if err != nil || n == 0 {
		t.Fatalf("ExpandContext = %d, %v", n, err)
	}
	calls := append([]string(nil), spans...)
	if len(calls) == 0 {
		t.Fatal("the store was never called")
	}
	for i, v := range calls {
		if v != "span-1" {
			t.Fatalf("store call %d of %d ran without the caller's context", i+1, len(calls))
		}
	}
	if !g.Contains(ex("rex"), namespace.RDF.Type, ex("Animal")) || !g.Contains(ex("rex"), ex("ownedBy"), ex("ann")) {
		t.Fatal("closure is missing an RDFS or OWL RL inference")
	}
}

// chainGraph is a subclass chain with instances at the bottom. Its RDFS
// closure is depth×instances type triples, far more work than 20 ms.
func chainGraph(depth, instances int) *graph.Graph {
	g := graph.NewGraph()
	class := func(i int) term.URIRef { return term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/C%d", i)) }
	for i := 0; i < depth; i++ {
		g.Add(class(i), namespace.RDFS.SubClassOf, class(i+1))
	}
	for i := 0; i < instances; i++ {
		g.Add(term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/i%d", i)), namespace.RDF.Type, class(0))
	}
	return g
}

// transitiveChain is a chain over an owl:TransitiveProperty: nothing for RDFS
// to do, and a quadratic closure for OWL RL (prp-trp).
func transitiveChain(n int) *graph.Graph {
	g := graph.NewGraph()
	p := term.NewURIRefUnsafe("http://example.org/before")
	g.Add(p, namespace.RDF.Type, namespace.OWL.TransitiveProperty)
	node := func(i int) term.URIRef { return term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/n%d", i)) }
	for i := 0; i < n; i++ {
		g.Add(node(i), p, node(i+1))
	}
	return g
}

func TestExpandContextCancelled(t *testing.T) {
	g := chainGraph(10, 10)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if n, err := ExpandContext(ctx, g, RDFS); n != 0 || !errors.Is(err, ErrCancelled) || !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-cancelled: %d, %v", n, err)
	}
	if n, ic, err := ExpandCheckContext(ctx, g, 0); n != 0 || ic != nil || !errors.Is(err, ErrUnknownRegime) {
		t.Fatalf("an unknown regime must be reported before the context: %d, %v, %v", n, ic, err)
	}

	for name, c := range map[string]struct {
		g *graph.Graph
		r Regime
	}{
		"RDFS":   {chainGraph(150, 1500), RDFS},
		"OWL RL": {transitiveChain(1500), OWLRL},
	} {
		g, r := c.g, c.r
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		start := time.Now()
		_, err := ExpandContext(ctx, g, r)
		elapsed := time.Since(start)
		cancel()
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("%s: err = %v after %v; the closure finished instead of stopping", name, err, elapsed)
		}
		t.Logf("%s: stopped after %v", name, elapsed)
		if elapsed > 250*time.Millisecond {
			t.Fatalf("%s: stopped after %v", name, elapsed)
		}
	}
}
