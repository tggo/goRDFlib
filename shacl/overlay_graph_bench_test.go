package shacl

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

func benchEx(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }

// benchVocabulary is a shared graph of about n triples: 100 kinds under
// ex:Kind, and instances of them with a label and a code.
func benchVocabulary(b *testing.B, n int) *graph.Graph {
	b.Helper()
	g := graph.NewGraph()
	add := func(s term.Subject, p term.URIRef, o term.Term) {
		if err := g.Add(b.Context(), s, p, o); err != nil {
			b.Fatal(err)
		}
	}
	typ, sub := term.NewURIRefUnsafe(RDF+"type"), term.NewURIRefUnsafe(RDFS+"subClassOf")
	for c := range 100 {
		add(benchEx(fmt.Sprintf("Kind%d", c)), sub, benchEx("Kind"))
	}
	for i := range (n - 100) / 3 {
		k := benchEx(fmt.Sprintf("k%d", i))
		add(k, typ, benchEx(fmt.Sprintf("Kind%d", i%100)))
		add(k, benchEx("label"), term.NewLiteral(fmt.Sprintf("kind %d", i)))
		add(k, benchEx("code"), term.NewLiteral(i))
	}
	return g
}

// benchDocument is a data graph of items, six triples each, that refer to
// kinds of the vocabulary.
func benchDocument(items, kinds int) []term.Triple {
	typ := term.NewURIRefUnsafe(RDF + "type")
	out := make([]term.Triple, 0, 6*items)
	for j := range items {
		s := benchEx(fmt.Sprintf("doc-item%d", j))
		out = append(out,
			term.Triple{Subject: s, Predicate: typ, Object: benchEx("Item")},
			term.Triple{Subject: s, Predicate: benchEx("kind"), Object: benchEx(fmt.Sprintf("k%d", (j*7919)%kinds))},
			term.Triple{Subject: s, Predicate: benchEx("size"), Object: term.NewLiteral(j % 5)},
			term.Triple{Subject: s, Predicate: benchEx("name"), Object: term.NewLiteral(fmt.Sprintf("item %d", j))},
			term.Triple{Subject: s, Predicate: benchEx("part"), Object: benchEx(fmt.Sprintf("doc-item%d", (j+1)%items))},
			term.Triple{Subject: s, Predicate: benchEx("count"), Object: term.NewLiteral(j)},
		)
	}
	return out
}

const benchOverlayShapes = `@prefix sh: <http://www.w3.org/ns/shacl#> . @prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
ex:ItemShape a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:property [ sh:path ex:kind ; sh:minCount 1 ; sh:maxCount 1 ; sh:class ex:Kind ] ;
  sh:property [ sh:path ( ex:kind ex:label ) ; sh:minCount 1 ; sh:datatype xsd:string ] ;
  sh:property [ sh:path ex:size ; sh:datatype xsd:integer ; sh:maxInclusive 10 ] ;
  sh:property [ sh:path ex:name ; sh:minCount 1 ] ;
  sh:property [ sh:path ex:part ; sh:class ex:Item ] .`

const benchOverlaySPARQLShapes = benchOverlayShapes + `
ex:NoSharedName a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:sparql [ sh:select """PREFIX ex: <http://example.org/>
    SELECT $this WHERE { $this ex:name ?n . ?o ex:name ?n FILTER (?o != $this) }""" ] .`

// BenchmarkOverlayGraph validates one small document over a large shared
// graph, the document's triples added to the shared store for the validation
// and removed afterwards. "rewrap" is the path available without
// NewOverlayGraph: wrap the store with NewGraphFromRDF, which indexes the
// whole store each time. "overlay" indexes the shared graph once and each
// document's triples only.
func BenchmarkOverlayGraph(b *testing.B) {
	for _, shapes := range []struct{ name, ttl string }{{"core", benchOverlayShapes}, {"sparql", benchOverlaySPARQLShapes}} {
		sg, err := LoadTurtleString(shapes.ttl, "")
		if err != nil {
			b.Fatal(err)
		}
		cs, err := CompileShapes(sg)
		if err != nil {
			b.Fatal(err)
		}
		for _, n := range []int{10_000, 100_000} {
			shared := benchVocabulary(b, n)
			base, err := NewGraphFromRDF(b.Context(), shared, "")
			if err != nil {
				b.Fatal(err)
			}
			doc := benchDocument(50, (n-100)/3)
			validate := func(b *testing.B, build func() (*Graph, error)) {
				for _, t := range doc {
					if err := shared.Add(b.Context(), t.Subject, t.Predicate, t.Object); err != nil {
						b.Fatal(err)
					}
				}
				g, err := build()
				if err != nil {
					b.Fatal(err)
				}
				benchReport = cs.Validate(g)
				if !benchReport.Conforms {
					b.Fatalf("the benchmark document should conform: %d results", len(benchReport.Results))
				}
				for _, t := range doc {
					if err := shared.Remove(b.Context(), t.Subject, &t.Predicate, t.Object); err != nil {
						b.Fatal(err)
					}
				}
			}
			name := fmt.Sprintf("%s/shared=%d/doc=%d", shapes.name, base.Len(), len(doc))
			b.Run(name+"/rewrap", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					validate(b, func() (*Graph, error) { return NewGraphFromRDF(b.Context(), shared, "") })
				}
			})
			b.Run(name+"/overlay", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					validate(b, func() (*Graph, error) { return NewOverlayGraph(b.Context(), base, shared, doc) })
				}
			})
		}
	}
}
