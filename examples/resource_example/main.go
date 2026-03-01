// Package main demonstrates the Resource API for node-centric graph access.
// Ported from: rdflib/examples/resource_example.py
package main

import (
	"fmt"
	"sort"

	rdf "github.com/tggo/goRDFlib"
)

func main() {
	g := rdf.NewGraph()
	g.Bind("foaf", rdf.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))

	name, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/name")
	knows, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/knows")
	age, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/age")

	alice, _ := rdf.NewURIRef("http://example.org/Alice")
	bob, _ := rdf.NewURIRef("http://example.org/Bob")

	// Use Resource API
	aliceR := rdf.NewResource(g, alice)
	aliceR.Add(name, rdf.NewLiteral("Alice"))
	aliceR.Add(age, rdf.NewLiteral(30))
	aliceR.Add(knows, bob)

	bobR := rdf.NewResource(g, bob)
	bobR.Add(name, rdf.NewLiteral("Bob"))
	bobR.Add(age, rdf.NewLiteral(25))

	// Read values
	aliceName, _ := aliceR.Value(name)
	fmt.Printf("Name: %s\n", aliceName)

	aliceAge, _ := aliceR.Value(age)
	fmt.Printf("Age: %s\n", aliceAge)

	// List objects
	fmt.Println("\nAlice knows:")
	aliceR.Objects(knows)(func(t rdf.Term) bool {
		fmt.Printf("  %s\n", t.N3())
		return true
	})

	// Set (replace) value
	aliceR.Set(age, rdf.NewLiteral(31))
	newAge, _ := aliceR.Value(age)
	fmt.Printf("\nUpdated age: %s\n", newAge)

	// List all predicate-object pairs (sorted for determinism)
	fmt.Println("\nAlice's properties:")
	var props []string
	aliceR.PredicateObjects()(func(p, o rdf.Term) bool {
		props = append(props, fmt.Sprintf("  %s = %s", p.N3(), o.N3()))
		return true
	})
	sort.Strings(props)
	for _, s := range props {
		fmt.Println(s)
	}

	fmt.Printf("\nGraph has %d triples\n", g.Len())
}
