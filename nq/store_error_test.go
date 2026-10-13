package nq

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/storetest"
)

const threeTriples = "<http://e/a> <http://e/p> \"1\" .\n<http://e/b> <http://e/p> \"2\" .\n<http://e/c> <http://e/p> \"3\" .\n"

func TestParseReturnsStoreError(t *testing.T) {
	fs := storetest.NewFailingStore(store.NewMemoryStore())
	fs.Fail(nil)
	g := graph.NewGraphFromStore(fs, nil)
	// An error handler must not swallow a store failure: it sees bad lines only.
	handled := false
	err := Parse(t.Context(), g, strings.NewReader(threeTriples),
		WithErrorHandler(func(int, string, error) (string, bool) { handled = true; return "", false }))
	if !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("Parse error = %v, want ErrInjected", err)
	}
	if !strings.Contains(err.Error(), "line 1") {
		t.Errorf("error should name the line: %v", err)
	}
	if handled {
		t.Error("store error was passed to the error handler")
	}
}

func TestSerializeReturnsStoreErrorAndWritesNothing(t *testing.T) {
	mem := store.NewMemoryStore()
	g := graph.NewGraphFromStore(mem, nil)
	if err := Parse(t.Context(), g, strings.NewReader(threeTriples)); err != nil {
		t.Fatal(err)
	}
	fs := storetest.NewFailingStore(mem)
	fs.FailReadsAfter(2, nil)
	var buf bytes.Buffer
	err := Serialize(t.Context(), graph.NewGraphFromStore(fs, nil), &buf)
	if !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("Serialize error = %v, want ErrInjected", err)
	}
	if buf.Len() != 0 {
		t.Errorf("Serialize wrote %q before failing", buf.String())
	}

	fs2 := storetest.NewFailingStore(mem)
	fs2.Fail(nil)
	if err := Serialize(t.Context(), graph.NewGraphFromStore(fs2, nil), &buf); !errors.Is(err, storetest.ErrInjected) || buf.Len() != 0 {
		t.Fatalf("Len failure: err=%v written=%q", err, buf.String())
	}
}

func TestParseDoneContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := Parse(ctx, rdflibgo.NewGraph(), strings.NewReader(threeTriples))
	if !errors.Is(err, ctx.Err()) {
		t.Fatalf("Parse with done context = %v", err)
	}
}

func TestWriteGraphReturnsStoreError(t *testing.T) {
	mem := store.NewMemoryStore()
	if err := Parse(t.Context(), graph.NewGraphFromStore(mem, nil), strings.NewReader(threeTriples)); err != nil {
		t.Fatal(err)
	}
	fs := storetest.NewFailingStore(mem)
	fs.FailReadsAfter(1, nil)
	var buf bytes.Buffer
	w := NewWriter(&buf)
	if err := w.WriteGraph(t.Context(), graph.NewGraphFromStore(fs, nil)); !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("WriteGraph error = %v, want ErrInjected", err)
	}
}
