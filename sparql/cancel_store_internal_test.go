package sparql

import (
	"context"
	"errors"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// abandoningStore's lookup notices the context is done and refuses it, as
// every store must (storetest DoneContext).
type abandoningStore struct {
	store.Store
	cancel context.CancelFunc
}

func (s *abandoningStore) Triples(ctx context.Context, _ term.TriplePattern, _ term.Term) store.TripleIterator {
	s.cancel()
	return store.ErrorIterator[term.Triple](ctx.Err())
}

// A store that gives up because of the cancellation reports it, and the query
// reports it as a cancellation, never as an empty result.
func TestStoreCancellationIsNotAnEmptyResult(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g := graph.NewGraph(graph.WithStore(&abandoningStore{Store: store.NewMemoryStore(), cancel: cancel}))
	res, err := QueryContext(ctx, g, `SELECT * WHERE { ?s ?p ?o }`)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, ErrQueryCancelled) {
		t.Fatalf("err = %v, result = %+v; want the cancellation", err, res)
	}
}
