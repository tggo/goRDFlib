// Package main demonstrates basic RDF graph creation, triple manipulation, and serialization.
// Ported from: rdflib/examples/simple_example.py
package main

import (
	"fmt"
	"os"

	rdf "github.com/tggo/goRDFlib"
)

func main() {
	g := rdf.NewGraph()

	// Bind namespace prefixes
	g.Bind("dc", rdf.NewURIRefUnsafe("http://purl.org/dc/elements/1.1/"))
	g.Bind("foaf", rdf.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))
	g.Bind("ex", rdf.NewURIRefUnsafe("http://example.org/"))

	// Create terms
	donna, _ := rdf.NewURIRef("http://example.org/Donna")
	name, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/name")
	knows, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/knows")
	person, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/Person")
	title, _ := rdf.NewURIRef("http://purl.org/dc/elements/1.1/title")
	bob, _ := rdf.NewURIRef("http://example.org/Bob")

	// Add triples
	g.Add(donna, rdf.RDF.Type, person)
	g.Add(donna, name, rdf.NewLiteral("Donna Fales"))
	g.Add(donna, knows, bob)
	g.Add(bob, rdf.RDF.Type, person)
	g.Add(bob, name, rdf.NewLiteral("Bob"))
	g.Add(bob, title, rdf.NewLiteral("The Builder", rdf.WithLang("en")))

	// Print triple count
	fmt.Printf("Graph has %d triples\n\n", g.Len())

	// Serialize to Turtle (deterministic output)
	fmt.Println("Turtle output:")
	g.Serialize(os.Stdout, rdf.WithSerializeFormat("turtle"))
}
