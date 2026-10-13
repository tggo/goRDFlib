// Package main demonstrates basic RDF graph creation, triple manipulation, and serialization.
// Ported from: rdflib/examples/simple_example.py
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	rdf "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/turtle"
)

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	g := rdf.NewGraph()

	// Bind namespace prefixes
	for prefix, ns := range map[string]string{
		"dc":   "http://purl.org/dc/elements/1.1/",
		"foaf": "http://xmlns.com/foaf/0.1/",
		"ex":   "http://example.org/",
	} {
		if err := g.Bind(ctx, prefix, rdf.NewURIRefUnsafe(ns)); err != nil {
			return err
		}
	}

	// Create terms
	donna, _ := rdf.NewURIRef("http://example.org/Donna")
	name, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/name")
	knows, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/knows")
	person, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/Person")
	title, _ := rdf.NewURIRef("http://purl.org/dc/elements/1.1/title")
	bob, _ := rdf.NewURIRef("http://example.org/Bob")

	// Add triples
	for _, t := range []struct {
		s rdf.Subject
		p rdf.URIRef
		o rdf.Term
	}{
		{donna, rdf.RDF.Type, person},
		{donna, name, rdf.NewLiteral("Donna Fales")},
		{donna, knows, bob},
		{bob, rdf.RDF.Type, person},
		{bob, name, rdf.NewLiteral("Bob")},
		{bob, title, rdf.NewLiteral("The Builder", rdf.WithLang("en"))},
	} {
		if err := g.Add(ctx, t.s, t.p, t.o); err != nil {
			return err
		}
	}

	// Print triple count
	n, err := g.Len(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Graph has %d triples\n\n", n)

	// Serialize to Turtle (deterministic output)
	fmt.Println("Turtle output:")
	return turtle.Serialize(ctx, g, os.Stdout)
}
