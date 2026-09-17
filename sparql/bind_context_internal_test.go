package sparql

import (
	"context"
	"errors"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// abandoningStore is a bound store whose lookup notices the context is done
// and gives up, which to the engine is indistinguishable from no match.
type abandoningStore struct {
	store.Store
	cancel context.CancelFunc
}

func (s *abandoningStore) BindContext(context.Context) store.Store { return s }

func (s *abandoningStore) Triples(term.TriplePattern, term.Term) store.TripleIterator {
	s.cancel()
	return func(func(term.Triple) bool) {}
}

// A query over a bound store must not return an empty result as complete when
// the store gave up because of the cancellation.
func TestBoundStoreCancellationIsNotAnEmptyResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g := graph.NewGraph(graph.WithStore(&abandoningStore{Store: store.NewMemoryStore(), cancel: cancel}))
	res, err := QueryContext(ctx, g, `SELECT * WHERE { ?s ?p ?o }`)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, result = %+v; want the cancellation", err, res)
	}
}
