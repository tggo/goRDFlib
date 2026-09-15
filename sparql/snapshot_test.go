package sparql

import (
	"sync/atomic"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
)

// countingSnapshotStore is a MemoryStore that offers read snapshots and counts
// how many were taken and released.
type countingSnapshotStore struct {
	*store.MemoryStore
	taken, released atomic.Int64
}

func (s *countingSnapshotStore) ReadSnapshot() (store.Store, func()) {
	s.taken.Add(1)
	return s.MemoryStore, func() { s.released.Add(1) }
}

// A query takes one snapshot per store, shared by the default and the named
// graphs, and releases it; the caller's query keeps its own graphs.
func TestEvalQuery_OneSnapshotPerStore(t *testing.T) {
	st := &countingSnapshotStore{MemoryStore: store.NewMemoryStore()}
	ds := graph.NewDataset(graph.WithStore(st))
	s := rdflibgo.NewURIRefUnsafe("urn:s")
	p := rdflibgo.NewURIRefUnsafe("urn:p")
	ds.DefaultContext().Add(s, p, rdflibgo.NewLiteral("d"))
	named := ds.GetContext(rdflibgo.NewURIRefUnsafe("urn:g"))
	named.Add(s, p, rdflibgo.NewLiteral("n"))

	q, err := Parse(`SELECT ?d ?n WHERE { <urn:s> <urn:p> ?d GRAPH <urn:g> { <urn:s> <urn:p> ?n } }`)
	if err != nil {
		t.Fatal(err)
	}
	q.NamedGraphs = map[string]*rdflibgo.Graph{"urn:g": named}
	res, err := EvalQuery(ds.DefaultContext(), q, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 1 {
		t.Fatalf("got %d rows, want 1", len(res.Bindings))
	}
	if st.taken.Load() != 1 || st.released.Load() != 1 {
		t.Errorf("snapshots taken %d, released %d; want 1 and 1", st.taken.Load(), st.released.Load())
	}
	if q.NamedGraphs["urn:g"] != named {
		t.Error("the caller's NamedGraphs map was changed")
	}
}
