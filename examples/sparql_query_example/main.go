// Package main demonstrates SPARQL queries on an RDF graph.
// Ported from: rdflib/examples/sparql_query_example.py
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
	if err := g.Bind(ctx, "foaf", rdf.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/")); err != nil {
		return err
	}
	if err := g.Bind(ctx, "ex", rdf.NewURIRefUnsafe("http://example.org/")); err != nil {
		return err
	}

	// Build a small social graph
	if err := turtle.Parse(ctx, g, strings.NewReader(`
		@prefix foaf: <http://xmlns.com/foaf/0.1/> .
		@prefix ex: <http://example.org/> .

		ex:Alice a foaf:Person ;
			foaf:name "Alice" ;
			foaf:age 30 ;
			foaf:knows ex:Bob, ex:Charlie .

		ex:Bob a foaf:Person ;
			foaf:name "Bob" ;
			foaf:age 25 .

		ex:Charlie a foaf:Person ;
			foaf:name "Charlie" ;
			foaf:age 35 .
	`)); err != nil {
		return err
	}

	n, err := g.Len(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Loaded %d triples\n", n)

	// SELECT query: find all persons
	fmt.Println("\nAll persons:")
	result, err := sparql.QueryContext(ctx, g, `
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		SELECT ?name WHERE { ?s a foaf:Person . ?s foaf:name ?name }
		ORDER BY ?name
	`)
	if err != nil {
		return err
	}
	for _, row := range result.Bindings {
		fmt.Printf("  %s\n", row["name"])
	}

	// FILTER query: persons over 28
	fmt.Println("\nPersons over 28:")
	result, err = sparql.QueryContext(ctx, g, `
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		SELECT ?name ?age WHERE {
			?s foaf:name ?name .
			?s foaf:age ?age .
			FILTER(?age > 28)
		}
		ORDER BY DESC(?age)
	`)
	if err != nil {
		return err
	}
	for _, row := range result.Bindings {
		fmt.Printf("  %s (age %s)\n", row["name"], row["age"])
	}

	// ASK query
	result, err = sparql.QueryContext(ctx, g, `
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		PREFIX ex: <http://example.org/>
		ASK { ex:Alice foaf:knows ex:Bob }
	`)
	if err != nil {
		return err
	}
	fmt.Printf("\nAlice knows Bob? %v\n", result.AskResult)

	// CONSTRUCT query
	result, err = sparql.QueryContext(ctx, g, `
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:label ?name }
		WHERE { ?s foaf:name ?name }
	`)
	if err != nil {
		return err
	}
	cn, err := result.Graph.Len(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\nConstructed graph has %d triples\n", cn)
	return nil
}
