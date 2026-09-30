package shacl_test

import (
	"fmt"
	"os"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/shacl"
)

// A JSON-LD document parsed once serves both validation and N-Quads output:
// shacl.NewGraphFromRDF wraps the parsed graph instead of parsing again.
func ExampleCompiledShapes_parseOnce() {
	shapes, _ := shacl.LoadTurtleString(`
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix schema: <https://schema.org/> .
[] a sh:NodeShape ; sh:targetClass schema:Place ;
   sh:property [ sh:path schema:name ; sh:minCount 1 ] .
`, "")
	compiled, _ := shacl.CompileShapes(shapes)     // once, at startup
	loader := jsonld.NewCachingDocumentLoader(nil) // once, shared

	doc := `{"@context": {"@vocab": "https://schema.org/"},
	         "@id": "https://example.org/place/1", "@type": "Place", "name": "Well 1"}`

	g := rdflibgo.NewGraph()
	if err := jsonld.Parse(g, strings.NewReader(doc), jsonld.WithDocumentLoader(loader)); err != nil {
		fmt.Println(err)
		return
	}
	report := compiled.Validate(shacl.NewGraphFromRDF(g, ""))
	fmt.Println("conforms:", report.Conforms)

	if report.Conforms {
		_ = nq.Serialize(g, os.Stdout)
	}
	// Unordered output:
	// conforms: true
	// <https://example.org/place/1> <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> <https://schema.org/Place> .
	// <https://example.org/place/1> <https://schema.org/name> "Well 1" .
}
