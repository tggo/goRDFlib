package trig

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
)

// Issue #34: TriG writes IRIs under the base relative to it too, graph names
// included, and the output parses back to the same quads.
func TestSerializeDatasetWithBaseWritesRelativeIRIs(t *testing.T) {
	const base = "http://localhost/doc.trig"
	u := rdflibgo.NewURIRefUnsafe
	ds := graph.NewDataset()
	ds.DefaultContext().Add(t.Context(), u(base+"#s"), u("http://localhost/p"), u("http://elsewhere/o"))
	ds.Graph(u(base+"#g")).Add(t.Context(), u(base+"#s"), u("http://localhost/p"), rdflibgo.NewLiteral("v", rdflibgo.WithDatatype(u(base+"#dt"))))

	out := mustSerializeDS(t, ds, WithBase(base))
	for _, want := range []string{"<#g> {", "<#s> <p>", "^^<#dt>", "<http://elsewhere/o>"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %s:\n%s", want, out)
		}
	}

	ds2 := mustParseDS(t, out)
	for _, ctx := range []rdflibgo.Term{nil, u(base + "#g")} {
		var a, b *rdflibgo.Graph
		if ctx == nil {
			a, b = ds.DefaultContext(), ds2.DefaultContext()
		} else {
			a, b = ds.Graph(ctx.(rdflibgo.URIRef)), ds2.Graph(ctx.(rdflibgo.URIRef))
		}
		if mustV(a.Len(t.Context())) != mustV(b.Len(t.Context())) {
			t.Fatalf("graph %v: %d triples, parsed back %d\n%s", ctx, mustV(a.Len(t.Context())), mustV(b.Len(t.Context())), out)
		}
		a.Triples(t.Context(), nil, nil, nil)(func(tr rdflibgo.Triple, _ error) bool {
			if !mustV(b.Contains(t.Context(), tr.Subject, tr.Predicate, tr.Object)) {
				t.Errorf("graph %v lost %v", ctx, tr)
			}
			return true
		})
	}
}
