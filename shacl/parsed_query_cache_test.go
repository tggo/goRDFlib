package shacl

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// The parsed form of a constraint's query is reused across focus nodes,
// validations and goroutines: results must not depend on it (run with -race).
// The shapes cover sh:select, sh:select reading $shapesGraph (named graphs set
// per call) and an sh:ask validator.
func TestCachedSPARQLConstraintsGiveTheSameResults(t *testing.T) {
	shapes := `@prefix sh: <http://www.w3.org/ns/shacl#> . @prefix ex: <http://example.org/> .
ex:S a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:sparql [ sh:select """PREFIX ex: <http://example.org/>
    SELECT $this ?value WHERE { $this ex:size ?value FILTER (?value > 5) }""" ] .
ex:G a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:sparql [ sh:select """PREFIX ex: <http://example.org/>
    SELECT $this WHERE { GRAPH $shapesGraph { ex:S a ?t } FILTER NOT EXISTS { $this ex:size ?s } }""" ] .
ex:MaxSizeComponent a sh:ConstraintComponent ;
  sh:parameter [ sh:path ex:maxSize ] ;
  sh:propertyValidator [ a sh:SPARQLAskValidator ; sh:ask """ASK { FILTER ($value <= $maxSize) }""" ] .
ex:A a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:property [ sh:path ex:size ; ex:maxSize 12 ] .`
	sg, err := LoadTurtleString(shapes, "http://example.org/shapes")
	if err != nil {
		t.Fatal(err)
	}
	var data strings.Builder
	data.WriteString("@prefix ex: <http://example.org/> .\n")
	for i := range 20 {
		if i%5 == 4 {
			fmt.Fprintf(&data, "ex:i%d a ex:Item .\n", i)
		} else {
			fmt.Fprintf(&data, "ex:i%d a ex:Item ; ex:size %d .\n", i, i)
		}
	}
	dg, err := LoadTurtleString(data.String(), "")
	if err != nil {
		t.Fatal(err)
	}
	// i4, i9, i14 and i19 have no size. ex:S: the 11 sizes above 5; ex:G:
	// the 4 items without a size; ex:A: the 5 sizes above 12.
	want := 11 + 4 + 5
	for range 2 {
		if n := len(Validate(dg, sg).Results); n != want {
			t.Fatalf("got %d results, want %d", n, want)
		}
	}
	cs, err := CompileShapes(sg)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 5 {
				if n := len(cs.Validate(dg).Results); n != want {
					t.Errorf("concurrent run: got %d results, want %d", n, want)
				}
			}
		})
	}
	wg.Wait()
}
