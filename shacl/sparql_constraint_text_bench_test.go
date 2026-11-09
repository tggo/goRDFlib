package shacl

import (
	"fmt"
	"strings"
	"testing"
)

// BenchmarkSPARQLConstraintText validates items against sh:sparql constraints
// whose query text is the same for every focus node: a node shape, a
// property shape whose query uses $PATH, and the same property shape with its
// prefixes declared through sh:prefixes (a longer preamble).
func BenchmarkSPARQLConstraintText(b *testing.B) {
	const head = `@prefix sh: <http://www.w3.org/ns/shacl#> . @prefix ex: <http://example.org/> .
@prefix owl: <http://www.w3.org/2002/07/owl#> . @prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
`
	shapes := map[string]string{
		"NodeShape": head + `ex:S a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:sparql [ sh:select """PREFIX ex: <http://example.org/>
    SELECT $this ?value WHERE { $this ex:size ?value FILTER (?value > 90) }""" ] .`,
		"PathShape": head + `ex:S a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:property [ sh:path ex:size ; sh:sparql [ sh:select """PREFIX ex: <http://example.org/>
    SELECT $this ?value WHERE { $this $PATH ?value FILTER (?value > 90) }""" ] ] .`,
		"PathShapeDeclaredPrefixes": head + `ex: sh:declare
  [ sh:prefix "ex" ; sh:namespace "http://example.org/"^^xsd:anyURI ] ,
  [ sh:prefix "rdf" ; sh:namespace "http://www.w3.org/1999/02/22-rdf-syntax-ns#"^^xsd:anyURI ] ,
  [ sh:prefix "rdfs" ; sh:namespace "http://www.w3.org/2000/01/rdf-schema#"^^xsd:anyURI ] ,
  [ sh:prefix "xsd" ; sh:namespace "http://www.w3.org/2001/XMLSchema#"^^xsd:anyURI ] ,
  [ sh:prefix "owl" ; sh:namespace "http://www.w3.org/2002/07/owl#"^^xsd:anyURI ] ,
  [ sh:prefix "skos" ; sh:namespace "http://www.w3.org/2004/02/skos/core#"^^xsd:anyURI ] .
ex:S a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:property [ sh:path ex:size ; sh:sparql [ sh:prefixes ex: ; sh:select """
    SELECT $this ?value WHERE {
      $this $PATH ?value .
      FILTER (?value > 90)
      FILTER NOT EXISTS { $this rdf:type ex:Exempt }
    }""" ] ] .`,
	}
	for _, name := range []string{"NodeShape", "PathShape", "PathShapeDeclaredPrefixes"} {
		sg, err := LoadTurtleString(shapes[name], "http://example.org/shapes")
		if err != nil {
			b.Fatal(err)
		}
		cs, err := CompileShapes(sg)
		if err != nil {
			b.Fatal(err)
		}
		for _, items := range []int{100, 1000} {
			var data strings.Builder
			data.WriteString("@prefix ex: <http://example.org/> .\n")
			for i := range items {
				fmt.Fprintf(&data, "ex:i%d a ex:Item ; ex:size %d .\n", i, i%100)
			}
			dg, err := LoadTurtleString(data.String(), "")
			if err != nil {
				b.Fatal(err)
			}
			b.Run(fmt.Sprintf("%s/%ditems", name, items), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					benchReport = cs.Validate(dg)
				}
				if got, want := len(benchReport.Results), items*9/100; got != want {
					b.Fatalf("got %d results, want %d", got, want)
				}
			})
		}
	}
}
