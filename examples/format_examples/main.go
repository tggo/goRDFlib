// Package main demonstrates parsing and serializing RDF in multiple formats.
// Ported from: rdflib/examples/jsonld_serialization.py + simple_example.py
package main

import (
	"bytes"
	"fmt"
	"strings"

	rdf "github.com/tggo/goRDFlib"
)

func main() {
	// Build a graph
	g := rdf.NewGraph()
	g.Bind("ex", rdf.NewURIRefUnsafe("http://example.org/"))
	g.Bind("foaf", rdf.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))

	alice, _ := rdf.NewURIRef("http://example.org/Alice")
	name, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/name")
	age, _ := rdf.NewURIRef("http://example.org/age")
	person, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/Person")

	g.Add(alice, rdf.RDF.Type, person)
	g.Add(alice, name, rdf.NewLiteral("Alice"))
	g.Add(alice, age, rdf.NewLiteral(30))

	fmt.Printf("Graph has %d triples\n\n", g.Len())

	// Serialize to each format
	for _, format := range []string{"turtle", "nt", "xml", "json-ld"} {
		var buf bytes.Buffer
		if err := g.Serialize(&buf, rdf.WithSerializeFormat(format)); err != nil {
			fmt.Printf("%s: error: %v\n", format, err)
			continue
		}
		fmt.Printf("--- %s ---\n%s\n", format, buf.String())
	}

	// Parse from N-Triples
	ntData := `<http://example.org/Bob> <http://xmlns.com/foaf/0.1/name> "Bob" .
<http://example.org/Bob> <http://example.org/age> "25"^^<http://www.w3.org/2001/XMLSchema#integer> .
`
	g2 := rdf.NewGraph()
	g2.Parse(strings.NewReader(ntData), rdf.WithFormat("nt"))
	fmt.Printf("Parsed %d triples from N-Triples\n", g2.Len())

	// Format auto-detection
	for _, filename := range []string{"data.ttl", "data.nt", "data.rdf", "data.jsonld"} {
		format, ok := rdf.FormatFromFilename(filename)
		if ok {
			fmt.Printf("  %s → %s\n", filename, format)
		}
	}
}
