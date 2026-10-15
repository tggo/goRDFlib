package nt

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"testing"
	"unsafe"

	rdflibgo "github.com/tggo/goRDFlib"
)

// footprintDoc is issue #53's shape: distinct subjects, a handful of
// predicates repeated on every line, half the objects typed literals.
func footprintDoc(n int) []byte {
	var b bytes.Buffer
	for i := range n {
		fmt.Fprintf(&b, "<https://example.test/s/%d> <https://schema.org/name> \"name %d\" .\n", i, i)
		fmt.Fprintf(&b, "<https://example.test/s/%d> <https://schema.org/age> \"%d\"^^<http://www.w3.org/2001/XMLSchema#integer> .\n", i, i)
	}
	return b.Bytes()
}

// TestParseFootprint: a predicate or datatype IRI repeated on every line is
// one URIRef for the whole parse, not one per occurrence (issue #53).
func TestParseFootprint(t *testing.T) {
	if testing.Short() {
		t.Skip("parses 200k triples")
	}
	ctx := context.Background()
	doc := footprintDoc(100_000)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	n := 0
	err := ParseStream(bytes.NewReader(doc), func(rdflibgo.Subject, rdflibgo.URIRef, rdflibgo.Term) error { n++; return nil })
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatal(err)
	}
	allocs := float64(after.Mallocs-before.Mallocs) / float64(n)

	g := rdflibgo.NewGraph()
	runtime.GC()
	runtime.ReadMemStats(&before)
	if err := Parse(ctx, g, bytes.NewReader(doc)); err != nil {
		t.Fatal(err)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(g)
	retained := float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)) / float64(n)

	t.Logf("%.1f allocs/triple streaming, %.0f bytes/triple retained in a graph", allocs, retained)
}

// TestParseSharesRepeatedIRIs pins the mechanism: the second occurrence of a
// predicate or datatype is the first one's URIRef, same bytes.
func TestParseSharesRepeatedIRIs(t *testing.T) {
	var preds []rdflibgo.URIRef
	var dts []rdflibgo.URIRef
	err := ParseStream(bytes.NewReader(footprintDoc(2)), func(_ rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error {
		preds = append(preds, p)
		if l, ok := o.(rdflibgo.Literal); ok && l.Datatype() != rdflibgo.XSDString {
			dts = append(dts, l.Datatype())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	same := func(a, b rdflibgo.URIRef) bool {
		return a == b && unsafe.StringData(a.Value()) == unsafe.StringData(b.Value())
	}
	if !same(preds[0], preds[2]) || !same(preds[1], preds[3]) {
		t.Error("a repeated predicate was built twice")
	}
	if len(dts) != 2 || !same(dts[0], dts[1]) {
		t.Error("a repeated datatype was built twice")
	}
}
