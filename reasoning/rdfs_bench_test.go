package reasoning

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/term"
)

func BenchmarkRDFSClosure_Small(b *testing.B) {
	// Build a template graph with 50 triples and 5 schema triples
	buildGraph := func() *graph.Graph {
		g := graph.NewGraph()
		ns := namespace.NewNamespace("http://bench.example.org/")

		// 5 schema triples
		classA := ns.Term("ClassA")
		classB := ns.Term("ClassB")
		classC := ns.Term("ClassC")
		prop1 := ns.Term("prop1")
		prop2 := ns.Term("prop2")

		g.Add(b.Context(), classA, namespace.RDFS.SubClassOf, classB)
		g.Add(b.Context(), classB, namespace.RDFS.SubClassOf, classC)
		g.Add(b.Context(), prop1, namespace.RDFS.Domain, classA)
		g.Add(b.Context(), prop2, namespace.RDFS.Range, classB)
		g.Add(b.Context(), prop1, namespace.RDFS.SubPropertyOf, prop2)

		// 45 instance triples
		for i := range 45 {
			s := ns.Term(fmt.Sprintf("inst%d", i))
			g.Add(b.Context(), s, namespace.RDF.Type, classA)
		}

		return g
	}

	b.ResetTimer()
	for range b.N {
		g := buildGraph()
		RDFSClosure(b.Context(), g)
	}
}

func BenchmarkRDFSClosure_DeepHierarchy(b *testing.B) {
	// 10-level subClassOf chain, 100 instances of the leaf class
	buildGraph := func() *graph.Graph {
		g := graph.NewGraph()
		ns := namespace.NewNamespace("http://bench.example.org/")

		classes := make([]term.URIRef, 10)
		for i := range 10 {
			classes[i] = ns.Term(fmt.Sprintf("Class%d", i))
		}
		for i := 0; i < 9; i++ {
			g.Add(b.Context(), classes[i], namespace.RDFS.SubClassOf, classes[i+1])
		}

		// 100 instances of the leaf (Class0)
		for i := range 100 {
			s := ns.Term(fmt.Sprintf("inst%d", i))
			g.Add(b.Context(), s, namespace.RDF.Type, classes[0])
		}

		return g
	}

	b.ResetTimer()
	for range b.N {
		g := buildGraph()
		RDFSClosure(b.Context(), g)
	}
}
