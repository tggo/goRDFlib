package sparql

import (
	"context"
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

type reachCountingStore struct {
	*store.MemoryStore
	calls int
}

func (c *reachCountingStore) Reachable(ctx context.Context, q store.ReachabilityQuery) ([]term.Term, error) {
	c.calls++
	return c.MemoryStore.Reachable(ctx, q)
}

var _ store.ReachabilityStore = (*reachCountingStore)(nil)

// tggo/rdflibgo-mongostore#3: a path evaluated once per row asks the store
// once per distinct start node over the whole query, and a later query (after
// an update) is not answered from the earlier one's cache.
func TestQuerySharesReachabilityAnswers(t *testing.T) {
	ctx := t.Context()
	cs := &reachCountingStore{MemoryStore: store.NewMemoryStore()}
	g := graph.NewGraph(graph.WithStore(cs))
	u := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	for i := 1; i < 20; i++ {
		if err := g.Add(ctx, u(fmt.Sprintf("C%d", i)), u("sub"), u(fmt.Sprintf("C%d", i-1))); err != nil {
			t.Fatal(err)
		}
	}
	for i := range 340 {
		if err := g.Add(ctx, u(fmt.Sprintf("item%d", i)), u("type"), u(fmt.Sprintf("C%d", i%20))); err != nil {
			t.Fatal(err)
		}
	}
	q := `PREFIX ex: <http://example.org/>
SELECT ?item ?c WHERE { ?item ex:type ?t . ?t ex:sub+ ?c }`
	res, err := QueryContext(ctx, g, q)
	if err != nil {
		t.Fatal(err)
	}
	if cs.calls > 20 {
		t.Errorf("Reachable called %d times, want at most 20 (one per class)", cs.calls)
	}
	before := len(res.Bindings)

	if err := g.Add(ctx, u("C0"), u("sub"), u("Top")); err != nil {
		t.Fatal(err)
	}
	res, err = QueryContext(ctx, g, q)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(res.Bindings); got != before+340 {
		t.Errorf("after adding C0 sub Top: %d rows, want %d", got, before+340)
	}
}
