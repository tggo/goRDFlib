package shacl

import (
	"fmt"
	"strings"
	"testing"
)

// BenchmarkValidateSPARQLConstraint validates items against a SPARQL-based
// constraint (sh:sparql with sh:select) and a SPARQL-based constraint
// component (sh:ask validator). Each query is run once per focus node or
// value node; its text is the same every time.
func BenchmarkValidateSPARQLConstraint(b *testing.B) {
	shapes := map[string]string{
		"Select": `@prefix sh: <http://www.w3.org/ns/shacl#> . @prefix ex: <http://example.org/> .
ex:S a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:sparql [ sh:select """PREFIX ex: <http://example.org/>
    SELECT $this ?value WHERE { $this ex:size ?value FILTER (?value > 90) }""" ] .`,
		"AskComponent": `@prefix sh: <http://www.w3.org/ns/shacl#> . @prefix ex: <http://example.org/> .
ex:MaxSizeComponent a sh:ConstraintComponent ;
  sh:parameter [ sh:path ex:maxSize ] ;
  sh:propertyValidator [ a sh:SPARQLAskValidator ; sh:ask """ASK { FILTER ($value <= $maxSize) }""" ] .
ex:A a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:property [ sh:path ex:size ; ex:maxSize 90 ] .`,
	}
	for _, name := range []string{"Select", "AskComponent"} {
		sg, err := LoadTurtleString(shapes[name], "http://example.org/shapes")
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
					benchReport = Validate(dg, sg)
				}
				if got, want := len(benchReport.Results), items*9/100; got != want {
					b.Fatalf("got %d results, want %d", got, want)
				}
			})
		}
	}
}
