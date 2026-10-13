// Package main demonstrates SPARQL Update operations on RDF graphs.
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	rdf "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
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

	// Build initial data
	if err := turtle.Parse(ctx, g, strings.NewReader(`
		@prefix ex: <http://example.org/> .
		@prefix foaf: <http://xmlns.com/foaf/0.1/> .

		ex:Alice a foaf:Person ;
			foaf:name "Alice" ;
			ex:status "draft" .

		ex:Bob a foaf:Person ;
			foaf:name "Bob" ;
			ex:status "draft" .

		ex:Charlie a foaf:Person ;
			foaf:name "Charlie" ;
			ex:status "published" .
	`)); err != nil {
		return err
	}
	if err := printLen(ctx, "Initial", g); err != nil {
		return err
	}

	ds := &sparql.Dataset{Default: g}

	// INSERT DATA: add a new triple
	if err := sparql.UpdateContext(ctx, ds, `
		PREFIX ex: <http://example.org/>
		INSERT DATA { ex:Alice ex:email "alice@example.org" }
	`); err != nil {
		return err
	}
	if err := printLen(ctx, "After INSERT DATA", g); err != nil {
		return err
	}

	// DELETE/INSERT WHERE: bulk status change
	if err := sparql.UpdateContext(ctx, ds, `
		PREFIX ex: <http://example.org/>
		DELETE { ?s ex:status "draft" }
		INSERT { ?s ex:status "published" }
		WHERE  { ?s ex:status "draft" }
	`); err != nil {
		return err
	}

	// Verify the update
	result, err := sparql.QueryContext(ctx, g, `
		PREFIX ex: <http://example.org/>
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		SELECT ?name ?status WHERE {
			?s foaf:name ?name .
			?s ex:status ?status .
		}
		ORDER BY ?name
	`)
	if err != nil {
		return err
	}
	fmt.Println("\nAfter bulk status update:")
	for _, row := range result.Bindings {
		fmt.Printf("  %s -> %s\n", row["name"], row["status"])
	}

	// DELETE WHERE shorthand: remove all email triples
	if err := sparql.UpdateContext(ctx, ds, `
		PREFIX ex: <http://example.org/>
		DELETE WHERE { ?s ex:email ?o }
	`); err != nil {
		return err
	}
	if err := printLen(ctx, "\nAfter DELETE WHERE", g); err != nil {
		return err
	}

	// Named graphs: COPY between graphs
	named := rdf.NewGraph()
	ds.NamedGraphs = map[string]*rdf.Graph{
		"http://example.org/archive": named,
	}

	if err := sparql.UpdateContext(ctx, ds, `COPY DEFAULT TO <http://example.org/archive>`); err != nil {
		return err
	}
	return printLen(ctx, "Archive graph", named)
}

func printLen(ctx context.Context, label string, g *rdf.Graph) error {
	n, err := g.Len(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d triples\n", label, n)
	return nil
}
