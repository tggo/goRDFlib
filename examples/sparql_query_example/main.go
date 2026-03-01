// Package main demonstrates SPARQL queries on an RDF graph.
// Ported from: rdflib/examples/sparql_query_example.py
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
	g.Bind("foaf", rdf.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))
	g.Bind("ex", rdf.NewURIRefUnsafe("http://example.org/"))

	// Build a small social graph
	turtle.Parse(g, strings.NewReader(`
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
	`))

	fmt.Printf("Loaded %d triples\n", g.Len())

	// SELECT query: find all persons
	fmt.Println("\nAll persons:")
	result, _ := sparql.Query(g, `
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		SELECT ?name WHERE { ?s a foaf:Person . ?s foaf:name ?name }
		ORDER BY ?name
	`)
	for _, row := range result.Bindings {
		fmt.Printf("  %s\n", row["name"])
	}

	// FILTER query: persons over 28
	fmt.Println("\nPersons over 28:")
	result, _ = sparql.Query(g, `
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		SELECT ?name ?age WHERE {
			?s foaf:name ?name .
			?s foaf:age ?age .
			FILTER(?age > 28)
		}
		ORDER BY DESC(?age)
	`)
	for _, row := range result.Bindings {
		fmt.Printf("  %s (age %s)\n", row["name"], row["age"])
	}

	// ASK query
	result, _ = sparql.Query(g, `
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		PREFIX ex: <http://example.org/>
		ASK { ex:Alice foaf:knows ex:Bob }
	`)
	fmt.Printf("\nAlice knows Bob? %v\n", result.AskResult)

	// CONSTRUCT query
	result, _ = sparql.Query(g, `
		PREFIX foaf: <http://xmlns.com/foaf/0.1/>
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:label ?name }
		WHERE { ?s foaf:name ?name }
	`)
	fmt.Printf("\nConstructed graph has %d triples\n", result.Graph.Len())
}
