package shacl

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// benchItemTriples builds n items of five triples each, the way a program
// that holds its data in memory would.
func benchItemTriples(n int) []term.Triple {
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	typ, item := term.NewURIRefUnsafe(RDF+"type"), ex("Item")
	size, name, part, label := ex("size"), ex("name"), ex("part"), ex("label")
	out := make([]term.Triple, 0, 5*n)
	for i := range n {
		s := ex(fmt.Sprintf("item%d", i))
		out = append(out,
			term.Triple{Subject: s, Predicate: typ, Object: item},
			term.Triple{Subject: s, Predicate: size, Object: term.NewLiteral(i % 5)},
			term.Triple{Subject: s, Predicate: name, Object: term.NewLiteral(fmt.Sprintf("item %d", i))},
			term.Triple{Subject: s, Predicate: label, Object: term.NewLiteral(fmt.Sprintf("l%d", i), term.WithLang("en"))},
			term.Triple{Subject: s, Predicate: part, Object: ex(fmt.Sprintf("item%d", (i+1)%n))},
		)
	}
	return out
}

const benchItemShapes = `@prefix sh: <http://www.w3.org/ns/shacl#> . @prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
ex:ItemShape a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:property [ sh:path ex:size ; sh:minCount 1 ; sh:maxCount 1 ; sh:datatype xsd:integer ; sh:maxInclusive 10 ] ;
  sh:property [ sh:path ex:name ; sh:minCount 1 ; sh:datatype xsd:string ] ;
  sh:property [ sh:path ex:label ; sh:uniqueLang true ] ;
  sh:property [ sh:path ex:part ; sh:class ex:Item ] .`

const benchItemSPARQLShapes = benchItemShapes + `
ex:NoSelfPart a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:sparql [ sh:select """PREFIX ex: <http://example.org/> SELECT $this WHERE { $this ex:part $this }""" ] .`

// BenchmarkGraphFromTriples validates triples a program already holds.
// "wrapped" is the path available without NewGraphFromTriples: add the triples
// to a graph.Graph and wrap it with NewGraphFromRDF. "fromTriples" hands the
// slice over. With a SPARQL constraint the store is built in both cases.
func BenchmarkGraphFromTriples(b *testing.B) {
	for _, shapes := range []struct{ name, ttl string }{{"core", benchItemShapes}, {"sparql", benchItemSPARQLShapes}} {
		sg, err := LoadTurtleString(shapes.ttl, "")
		if err != nil {
			b.Fatal(err)
		}
		cs, err := CompileShapes(sg)
		if err != nil {
			b.Fatal(err)
		}
		for _, n := range []int{20, 200, 2000} {
			ts := benchItemTriples(n)
			name := fmt.Sprintf("%s/triples=%d", shapes.name, len(ts))
			b.Run(name+"/wrapped", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					rg := graph.NewGraph()
					for _, t := range ts {
						if err := rg.Add(b.Context(), t.Subject, t.Predicate, t.Object); err != nil {
							b.Fatal(err)
						}
					}
					g, err := NewGraphFromRDF(b.Context(), rg, "")
					if err != nil {
						b.Fatal(err)
					}
					benchReport = cs.Validate(g)
				}
			})
			b.Run(name+"/fromTriples", func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					benchReport = cs.Validate(NewGraphFromTriples(ts, ""))
				}
			})
			if r := cs.Validate(NewGraphFromTriples(ts, "")); !r.Conforms {
				b.Fatalf("%s: the benchmark data should conform: %d results", name, len(r.Results))
			}
		}
	}
}
