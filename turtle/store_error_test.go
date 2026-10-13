package turtle

import (
	"bytes"
	"context"
	"errors"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/storetest"
)

// failingGraph is a graph over a store holding n triples that can be told to
// fail.
func failingGraph(t *testing.T, n int) (*rdflibgo.Graph, *storetest.FailingStore) {
	t.Helper()
	fs := storetest.NewFailingStore(store.NewMemoryStore())
	g := graph.NewGraph(graph.WithStore(fs))
	ex := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	for i := range n {
		s := rdflibgo.NewURIRefUnsafe("http://example.org/s" + string(rune('a'+i)))
		if err := g.Add(t.Context(), s, ex, rdflibgo.NewLiteral("v")); err != nil {
			t.Fatal(err)
		}
	}
	return g, fs
}

func TestParseReturnsStoreError(t *testing.T) {
	g, fs := failingGraph(t, 0)
	fs.Fail(nil)
	err := Parse(t.Context(), g, bytes.NewBufferString("<http://e/s> <http://e/p> <http://e/o> ."))
	if !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("Parse error = %v, want ErrInjected", err)
	}
}

func TestParseReturnsContextError(t *testing.T) {
	g, _ := failingGraph(t, 0)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := Parse(ctx, g, bytes.NewBufferString("<http://e/s> <http://e/p> <http://e/o> ."))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Parse error = %v, want context.Canceled", err)
	}
}

func TestSerializeReturnsStoreErrorAndWritesNothing(t *testing.T) {
	g, fs := failingGraph(t, 3)
	fs.Fail(nil)
	var out bytes.Buffer
	err := Serialize(t.Context(), g, &out)
	if !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("Serialize error = %v, want ErrInjected", err)
	}
	if out.Len() != 0 {
		t.Fatalf("Serialize wrote %d bytes before failing:\n%s", out.Len(), out.String())
	}
}

func TestSerializeMidReadFailureWritesNothing(t *testing.T) {
	g, fs := failingGraph(t, 5)
	fs.FailReadsAfter(2, nil)
	var out bytes.Buffer
	err := Serialize(t.Context(), g, &out)
	if !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("Serialize error = %v, want ErrInjected", err)
	}
	if out.Len() != 0 {
		t.Fatalf("Serialize wrote a truncated document:\n%s", out.String())
	}
}
