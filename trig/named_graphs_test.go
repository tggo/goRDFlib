package trig

import (
	"bytes"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
)

// TestDatasetKeepsGraphsApart is rdflib #2393/#1842: on the default in-memory
// store a named graph used to see every triple in the dataset, because
// MemoryStore ignored the context it was given.
func TestDatasetKeepsGraphsApart(t *testing.T) {
	ds := graph.NewDataset()
	src := `@prefix : <http://e/> . :a :b :c . :g { :x :y :z . }`
	if err := ParseDataset(ds, strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}
	g := rdflibgo.NewURIRefUnsafe("http://e/g")
	if n := ds.Graph(g).Len(); n != 1 {
		t.Errorf("named graph :g has %d triples, want 1", n)
	}
	if n := ds.DefaultContext().Len(); n != 1 {
		t.Errorf("default graph has %d triples, want 1", n)
	}
	if n := ds.Len(); n != 2 {
		t.Errorf("dataset union has %d triples, want 2", n)
	}

	var b bytes.Buffer
	if err := SerializeDataset(ds, &b); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if strings.Count(out, "<http://e/a>")+strings.Count(out, ":a ") != 1 {
		t.Errorf("default-graph triple written more than once:\n%s", out)
	}
}
