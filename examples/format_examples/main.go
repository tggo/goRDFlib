// Package main demonstrates parsing and serializing RDF in multiple formats.
// Ported from: rdflib/examples/jsonld_serialization.py + simple_example.py
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"strings"

	rdf "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/turtle"
)

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	// Build a graph
	g := rdf.NewGraph()
	if err := g.Bind(ctx, "ex", rdf.NewURIRefUnsafe("http://example.org/")); err != nil {
		return err
	}
	if err := g.Bind(ctx, "foaf", rdf.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/")); err != nil {
		return err
	}

	alice, _ := rdf.NewURIRef("http://example.org/Alice")
	name, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/name")
	age, _ := rdf.NewURIRef("http://example.org/age")
	person, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/Person")

	for _, t := range []struct {
		p rdf.URIRef
		o rdf.Term
	}{
		{rdf.RDF.Type, person},
		{name, rdf.NewLiteral("Alice")},
		{age, rdf.NewLiteral(30)},
	} {
		if err := g.Add(ctx, alice, t.p, t.o); err != nil {
			return err
		}
	}

	n, err := g.Len(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Graph has %d triples\n\n", n)

	// Serialize to each format
	serializers := []struct {
		name string
		fn   func(context.Context, *rdf.Graph, *bytes.Buffer) error
	}{
		{"turtle", func(ctx context.Context, g *rdf.Graph, buf *bytes.Buffer) error { return turtle.Serialize(ctx, g, buf) }},
		{"nt", func(ctx context.Context, g *rdf.Graph, buf *bytes.Buffer) error { return nt.Serialize(ctx, g, buf) }},
		{"xml", func(ctx context.Context, g *rdf.Graph, buf *bytes.Buffer) error { return rdfxml.Serialize(ctx, g, buf) }},
		{"json-ld", func(ctx context.Context, g *rdf.Graph, buf *bytes.Buffer) error { return jsonld.Serialize(ctx, g, buf) }},
	}

	for _, s := range serializers {
		var buf bytes.Buffer
		if err := s.fn(ctx, g, &buf); err != nil {
			fmt.Printf("%s: error: %v\n", s.name, err)
			continue
		}
		fmt.Printf("--- %s ---\n%s\n", s.name, buf.String())
	}

	// Parse from N-Triples
	ntData := `<http://example.org/Bob> <http://xmlns.com/foaf/0.1/name> "Bob" .
<http://example.org/Bob> <http://example.org/age> "25"^^<http://www.w3.org/2001/XMLSchema#integer> .
`
	g2 := rdf.NewGraph()
	if err := nt.Parse(ctx, g2, strings.NewReader(ntData)); err != nil {
		return err
	}
	n2, err := g2.Len(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Parsed %d triples from N-Triples\n", n2)

	// Format auto-detection
	for _, filename := range []string{"data.ttl", "data.nt", "data.rdf", "data.jsonld"} {
		format, ok := rdf.FormatFromFilename(filename)
		if ok {
			fmt.Printf("  %s → %s\n", filename, format)
		}
	}
	return nil
}
