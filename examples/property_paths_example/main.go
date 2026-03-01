// Package main demonstrates SPARQL-style property paths.
// Ported from: rdflib/examples/foafpaths.py
package main

import (
	"fmt"
	"strings"

	rdf "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/turtle"
)

func main() {
	g := rdf.NewGraph()
	turtle.Parse(g, strings.NewReader(`
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
	`))

	tim, _ := rdf.NewURIRef("http://example.org/Tim")
	knows, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/knows")
	name, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/name")

	// Direct: who does Tim know?
	fmt.Println("Tim knows directly:")
	rdf.AsPath(knows).Eval(g, tim, nil)(func(s, o rdf.Term) bool {
		n, _ := g.Value(o.(rdf.Subject), &name, nil)
		fmt.Printf("  %s\n", n)
		return true
	})

	// Sequence path: knows/knows — two hops
	fmt.Println("\nTim knows via 2 hops (knows/knows):")
	rdf.Sequence(rdf.AsPath(knows), rdf.AsPath(knows)).Eval(g, tim, nil)(func(s, o rdf.Term) bool {
		n, _ := g.Value(o.(rdf.Subject), &name, nil)
		fmt.Printf("  %s\n", n)
		return true
	})

	// Transitive: knows+ — all reachable people
	fmt.Println("\nTim knows transitively (knows+):")
	rdf.OneOrMore(rdf.AsPath(knows)).Eval(g, tim, nil)(func(s, o rdf.Term) bool {
		n, _ := g.Value(o.(rdf.Subject), &name, nil)
		fmt.Printf("  %s\n", n)
		return true
	})

	// Alternative: knows | name
	fmt.Println("\nTim's knows-or-name (knows|name):")
	rdf.Alternative(rdf.AsPath(knows), rdf.AsPath(name)).Eval(g, tim, nil)(func(s, o rdf.Term) bool {
		fmt.Printf("  %s\n", o.N3())
		return true
	})
}
