// Package main demonstrates SPARQL Update operations on RDF graphs.
package main

import (
	"fmt"
	"strings"

	rdf "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/turtle"
)

func main() {
	g := rdf.NewGraph()

	// Build initial data
	turtle.Parse(g, strings.NewReader(`
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
	`))
	fmt.Printf("Initial: %d triples\n", g.Len())

	ds := &sparql.Dataset{Default: g}

	// INSERT DATA: add a new triple
	_ = sparql.Update(ds, `
		PREFIX ex: <http://example.org/>
		INSERT DATA { ex:Alice ex:email "alice@example.org" }
	`)
	fmt.Printf("After INSERT DATA: %d triples\n", g.Len())

	// DELETE/INSERT WHERE: bulk status change
	_ = sparql.Update(ds, `
		PREFIX ex: <http://example.org/>
		DELETE { ?s ex:status "draft" }
		INSERT { ?s ex:status "published" }
		WHERE  { ?s ex:status "draft" }
	`)

	// Verify the update
	result, _ := sparql.Query(g, `
		PREFIX ex: <http://example.org/>
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		SELECT ?name ?status WHERE {
			?s foaf:name ?name .
			?s ex:status ?status .
		}
		ORDER BY ?name
	`)
	fmt.Println("\nAfter bulk status update:")
	for _, row := range result.Bindings {
		fmt.Printf("  %s -> %s\n", row["name"], row["status"])
	}

	// DELETE WHERE shorthand: remove all email triples
	_ = sparql.Update(ds, `
		PREFIX ex: <http://example.org/>
		DELETE WHERE { ?s ex:email ?o }
	`)
	fmt.Printf("\nAfter DELETE WHERE: %d triples\n", g.Len())

	// Named graphs: COPY between graphs
	named := rdf.NewGraph()
	ds.NamedGraphs = map[string]*rdf.Graph{
		"http://example.org/archive": named,
	}

	_ = sparql.Update(ds, `COPY DEFAULT TO <http://example.org/archive>`)
	fmt.Printf("Archive graph: %d triples\n", named.Len())
}
