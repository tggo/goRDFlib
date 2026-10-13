// Package main demonstrates SPARQL-style property paths.
// Ported from: rdflib/examples/foafpaths.py
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

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
	if err := turtle.Parse(ctx, g, strings.NewReader(`
		@prefix foaf: <http://xmlns.com/foaf/0.1/> .
		@prefix ex: <http://example.org/> .

		ex:Tim a foaf:Person ;
			foaf:name "Tim" ;
			foaf:knows ex:Alice .

		ex:Alice a foaf:Person ;
			foaf:name "Alice" ;
			foaf:knows ex:Bob .

		ex:Bob a foaf:Person ;
			foaf:name "Bob" ;
			foaf:knows ex:Charlie .

		ex:Charlie a foaf:Person ;
			foaf:name "Charlie" .
	`)); err != nil {
		return err
	}

	tim, _ := rdf.NewURIRef("http://example.org/Tim")
	knows, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/knows")
	name, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/name")

	// printNames prints the foaf:name of every node the path reaches from Tim.
	printNames := func(path rdf.Path) error {
		for pair, err := range path.Eval(ctx, g, tim, nil) {
			if err != nil {
				return err
			}
			n, _, err := g.Value(ctx, pair.Second.(rdf.Subject), &name, nil)
			if err != nil {
				return err
			}
			fmt.Printf("  %s\n", n)
		}
		return nil
	}

	// Direct: who does Tim know?
	fmt.Println("Tim knows directly:")
	if err := printNames(rdf.AsPath(knows)); err != nil {
		return err
	}

	// Sequence path: knows/knows — two hops
	fmt.Println("\nTim knows via 2 hops (knows/knows):")
	if err := printNames(rdf.Sequence(rdf.AsPath(knows), rdf.AsPath(knows))); err != nil {
		return err
	}

	// Transitive: knows+ — all reachable people
	fmt.Println("\nTim knows transitively (knows+):")
	if err := printNames(rdf.OneOrMore(rdf.AsPath(knows))); err != nil {
		return err
	}

	// Alternative: knows | name
	fmt.Println("\nTim's knows-or-name (knows|name):")
	for pair, err := range rdf.Alternative(rdf.AsPath(knows), rdf.AsPath(name)).Eval(ctx, g, tim, nil) {
		if err != nil {
			return err
		}
		fmt.Printf("  %s\n", pair.Second.N3())
	}
	return nil
}
