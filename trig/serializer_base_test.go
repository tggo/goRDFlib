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
	ds.DefaultContext().Add(u(base+"#s"), u("http://localhost/p"), u("http://elsewhere/o"))
	ds.Graph(u(base+"#g")).Add(u(base+"#s"), u("http://localhost/p"), rdflibgo.NewLiteral("v", rdflibgo.WithDatatype(u(base+"#dt"))))

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
		if a.Len() != b.Len() {
			t.Fatalf("graph %v: %d triples, parsed back %d\n%s", ctx, a.Len(), b.Len(), out)
		}
		a.Triples(nil, nil, nil)(func(tr rdflibgo.Triple) bool {
			if !b.Contains(tr.Subject, tr.Predicate, tr.Object) {
				t.Errorf("graph %v lost %v", ctx, tr)
			}
			return true
		})
	}
}
