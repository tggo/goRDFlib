package trig

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
)

var chainNext = rdflibgo.NewURIRefUnsafe("http://example.org/next")

// blankChainDataset builds root -> _:b0 -> ... -> _:b<n> in the default graph.
func blankChainDataset(n int) *graph.Dataset {
	ds := graph.NewDataset()
	g := ds.DefaultContext()
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/root"), chainNext, rdflibgo.NewBNode("b0"))
	for i := range n {
		g.Add(rdflibgo.NewBNode(fmt.Sprintf("b%d", i)), chainNext, rdflibgo.NewBNode(fmt.Sprintf("b%d", i+1)))
	}
	return ds
}

// rdflib #1424: the TriG serializer inlined a blank node chain recursively
// with no depth limit, and a 2,000,000-link chain crashed the process with a
// fatal stack overflow. Nesting now stops at maxNestDepth.
func TestSerializeDeepBlankNodeChainTrig(t *testing.T) {
	n := 200000
	if testing.Short() {
		n = 50000
	}
	ds := blankChainDataset(n)
	var b bytes.Buffer
	if err := SerializeDataset(ds, &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), fmt.Sprintf("_:b%d", maxNestDepth)) {
		t.Errorf("nesting was not cut at %d", maxNestDepth)
	}
	ds2 := graph.NewDataset()
	if err := ParseDataset(ds2, strings.NewReader(b.String())); err != nil {
		t.Fatalf("output does not parse: %v", err)
	}
	assertChainTrig(t, ds2.DefaultContext(), n)
}

// assertChainTrig checks g is exactly root -> n+1 distinct blank nodes, linked
// in order. testutil.AssertGraphEqual cannot be used: every link of a chain
// has the same signature, and its backtracking search is exponential on them.
func assertChainTrig(t *testing.T, g *rdflibgo.Graph, n int) {
	t.Helper()
	if g.Len() != n+1 {
		t.Fatalf("got %d triples, want %d", g.Len(), n+1)
	}
	var node rdflibgo.Subject = rdflibgo.NewURIRefUnsafe("http://example.org/root")
	seen := make(map[string]bool, n+1)
	for i := 0; i <= n; i++ {
		next, ok := g.Value(node, &chainNext, nil)
		b, isBNode := next.(rdflibgo.BNode)
		if !ok || !isBNode || seen[b.Value()] {
			t.Fatalf("link %d: got %v", i, next)
		}
		seen[b.Value()] = true
		node = b
	}
}

// A collection nested deeper than the limit is cut the same way: the head at
// the limit is written as a label and its statement follows.
func TestSerializeDeepNestedListsTrig(t *testing.T) {
	const levels = 3 * maxNestDepth
	src := "@prefix ex: <http://example.org/> .\nex:root ex:next " + strings.Repeat("( ", levels) + strings.Repeat(") ", levels) + ".\n"
	ds := graph.NewDataset()
	if err := ParseDataset(ds, strings.NewReader(src)); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	if err := SerializeDataset(ds, &b); err != nil {
		t.Fatal(err)
	}
	ds2 := graph.NewDataset()
	if err := ParseDataset(ds2, strings.NewReader(b.String())); err != nil {
		t.Fatalf("output does not parse: %v\n%s", err, b.String())
	}
	g := ds2.DefaultContext()
	if g.Len() != ds.DefaultContext().Len() {
		t.Fatalf("got %d triples, want %d", g.Len(), ds.DefaultContext().Len())
	}
	// Walk root -> list -> first -> first ... down to the innermost ().
	node, _ := g.Value(rdflibgo.NewURIRefUnsafe("http://example.org/root"), &chainNext, nil)
	for i := 1; i < levels; i++ {
		s, ok := node.(rdflibgo.BNode)
		if !ok {
			t.Fatalf("level %d: got %v", i, node)
		}
		if rest, _ := g.Value(s, &rdflibgo.RDF.Rest, nil); rest != rdflibgo.RDF.Nil {
			t.Fatalf("level %d: rest %v", i, rest)
		}
		node, _ = g.Value(s, &rdflibgo.RDF.First, nil)
	}
	if node != rdflibgo.RDF.Nil {
		t.Fatalf("innermost: got %v, want rdf:nil", node)
	}
}
