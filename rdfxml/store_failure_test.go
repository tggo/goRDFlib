package rdfxml

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

const failureDoc = `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s"><ex:p>a</ex:p><ex:q>b</ex:q></rdf:Description>
</rdf:RDF>`

// TestParseReturnsStoreError: a failing store.Add is Parse's error, not an
// empty graph with a nil result.
func TestParseReturnsStoreError(t *testing.T) {
	fs := storetest.NewFailingStore(store.NewMemoryStore())
	fs.Fail(nil)
	g := graph.NewGraphFromStore(fs, nil)
	err := Parse(context.Background(), g, strings.NewReader(failureDoc))
	if !errors.Is(err, storetest.ErrInjected) {
		t.Fatalf("Parse err = %v, want storetest.ErrInjected", err)
	}
}

func TestParseDoneContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	g := rdflibgo.NewGraph()
	err := Parse(ctx, g, strings.NewReader(failureDoc))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Parse err = %v, want context.Canceled", err)
	}
	if n := mustV(g.Len(context.Background())); n != 0 {
		t.Fatalf("done context still added %d triples", n)
	}
}

// TestSerializeReturnsStoreError: a store that fails before or midway through
// the read fails Serialize and nothing reaches the writer.
func TestSerializeReturnsStoreError(t *testing.T) {
	for name, arm := range map[string]func(*storetest.FailingStore){
		"fails at once":       func(f *storetest.FailingStore) { f.Fail(nil) },
		"fails mid-read":      func(f *storetest.FailingStore) { f.FailReadsAfter(1, nil) },
		"fails on first read": func(f *storetest.FailingStore) { f.FailReadsAfter(0, nil) },
	} {
		t.Run(name, func(t *testing.T) {
			ms := store.NewMemoryStore()
			fs := storetest.NewFailingStore(ms)
			g := graph.NewGraphFromStore(fs, nil)
			if err := Parse(context.Background(), g, strings.NewReader(failureDoc)); err != nil {
				t.Fatal(err)
			}
			arm(fs)
			var buf bytes.Buffer
			err := Serialize(context.Background(), g, &buf)
			if !errors.Is(err, storetest.ErrInjected) {
				t.Fatalf("Serialize err = %v, want storetest.ErrInjected", err)
			}
			if buf.Len() != 0 {
				t.Fatalf("Serialize wrote %d bytes before failing: %q", buf.Len(), buf.String())
			}
		})
	}
}

func TestSerializeDoneContext(t *testing.T) {
	g := rdflibgo.NewGraph()
	if err := Parse(context.Background(), g, strings.NewReader(failureDoc)); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var buf bytes.Buffer
	if err := Serialize(ctx, g, &buf); !errors.Is(err, context.Canceled) {
		t.Fatalf("Serialize err = %v, want context.Canceled", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("wrote %d bytes", buf.Len())
	}
}
