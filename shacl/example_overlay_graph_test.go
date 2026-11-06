package shacl_test

import (
	"context"
	"fmt"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/shacl"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

// Many small data graphs are validated over one large shared graph (here a
// vocabulary of two kinds). The shared graph is indexed once; each overlay
// indexes only its own triples.
func ExampleNewOverlayGraph() {
	ctx := context.Background()
	shapes, _ := shacl.LoadTurtleString(`
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
[] a sh:NodeShape ; sh:targetClass ex:Item ;
   sh:property [ sh:path ex:kind ; sh:minCount 1 ; sh:class ex:Kind ] .
`, "")
	compiled, _ := shacl.CompileShapes(shapes)

	shared := rdflibgo.NewGraph()
	vocabulary := `@prefix ex: <http://example.org/> . ex:small a ex:Kind . ex:large a ex:Kind .`
	if err := turtle.Parse(ctx, shared, strings.NewReader(vocabulary)); err != nil {
		fmt.Println(err)
		return
	}
	base, err := shacl.NewGraphFromRDF(ctx, shared, "") // indexed once
	if err != nil {
		fmt.Println(err)
		return
	}

	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	rdfType := term.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#type")
	docs := [][]term.Triple{
		{{Subject: ex("a"), Predicate: rdfType, Object: ex("Item")}, {Subject: ex("a"), Predicate: ex("kind"), Object: ex("small")}},
		{{Subject: ex("b"), Predicate: rdfType, Object: ex("Item")}, {Subject: ex("b"), Predicate: ex("kind"), Object: ex("huge")}},
	}
	for _, doc := range docs {
		// SPARQL-based constraints query the store: it holds the document's
		// triples (none of them is in the vocabulary) during the validation.
		for _, t := range doc {
			_ = shared.Add(ctx, t.Subject, t.Predicate, t.Object)
		}
		overlay, err := shacl.NewOverlayGraph(ctx, base, shared, doc)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("conforms:", compiled.Validate(overlay).Conforms)
		for _, t := range doc {
			_ = shared.Remove(ctx, t.Subject, &t.Predicate, t.Object)
		}
	}
	// Output:
	// conforms: true
	// conforms: false
}
