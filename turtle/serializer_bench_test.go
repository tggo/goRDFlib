package turtle

import (
	"context"
	"fmt"
	"io"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// benchGraph builds a graph of n subjects, each with a type, two literals and a
// nested blank node, over several namespaces.
func benchGraph(n int) *rdflibgo.Graph {
	const ns = "https://example.org/"
	g := rdflibgo.NewGraph()
	g.Bind(context.Background(), "ex", rdflibgo.NewURIRefUnsafe(ns))
	g.Bind(context.Background(), "other", rdflibgo.NewURIRefUnsafe("https://other.example/"))
	g.Bind(context.Background(), "rdfs", rdflibgo.NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#"))

	cls := rdflibgo.NewURIRefUnsafe(ns + "Thing")
	label := rdflibgo.NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#label")
	weight := rdflibgo.NewURIRefUnsafe(ns + "weight")
	detail := rdflibgo.NewURIRefUnsafe(ns + "detail")
	note := rdflibgo.NewURIRefUnsafe("https://other.example/note")

	for i := 0; i < n; i++ {
		s := rdflibgo.NewURIRefUnsafe(fmt.Sprintf("%ss%d", ns, i))
		b := rdflibgo.NewBNode(fmt.Sprintf("d%d", i))
		g.Add(context.Background(), s, rdflibgo.RDF.Type, cls)
		g.Add(context.Background(), s, label, rdflibgo.NewLiteral(fmt.Sprintf("subject %d", i)))
		g.Add(context.Background(), s, weight, rdflibgo.NewLiteral(i))
		g.Add(context.Background(), s, detail, b)
		g.Add(context.Background(), b, note, rdflibgo.NewLiteral("nested"))
	}
	return g
}

func benchmarkSerialize(b *testing.B, opts ...Option) {
	g := benchGraph(2000)
	b.ReportAllocs()
	for b.Loop() {
		if err := Serialize(b.Context(), g, io.Discard, opts...); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSerializeCompact(b *testing.B) { benchmarkSerialize(b) }
func BenchmarkSerializePretty(b *testing.B)  { benchmarkSerialize(b, WithPretty()) }
