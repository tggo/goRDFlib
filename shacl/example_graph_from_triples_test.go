package shacl_test

import (
	"fmt"

	"github.com/tggo/goRDFlib/shacl"
	"github.com/tggo/goRDFlib/term"
)

// Triples a program builds itself are validated as they are: no parse, and no
// store unless a SPARQL-based constraint needs one.
func ExampleNewGraphFromTriples() {
	shapes, _ := shacl.LoadTurtleString(`
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
[] a sh:NodeShape ; sh:targetClass ex:Item ;
   sh:property [ sh:path ex:size ; sh:minCount 1 ] .
`, "")
	compiled, _ := shacl.CompileShapes(shapes) // once, at startup

	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	rdfType := term.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#type")
	triples := []term.Triple{
		{Subject: ex("a"), Predicate: rdfType, Object: ex("Item")},
		{Subject: ex("a"), Predicate: ex("size"), Object: term.NewLiteral(3)},
		{Subject: ex("b"), Predicate: rdfType, Object: ex("Item")},
	}

	report := compiled.Validate(shacl.NewGraphFromTriples(triples, ""))
	fmt.Println("conforms:", report.Conforms)
	for _, r := range report.Results {
		fmt.Println(r.FocusNode, r.ResultPath)
	}
	// Output:
	// conforms: false
	// <http://example.org/b> <http://example.org/size>
}
