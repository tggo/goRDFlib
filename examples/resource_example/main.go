// Package main demonstrates the Resource API for node-centric graph access.
// Ported from: rdflib/examples/resource_example.py
package main

import (
	"context"
	"fmt"
	"log"
	"sort"

	rdf "github.com/tggo/goRDFlib"
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

	name, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/name")
	knows, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/knows")
	age, _ := rdf.NewURIRef("http://xmlns.com/foaf/0.1/age")

	alice, _ := rdf.NewURIRef("http://example.org/Alice")
	bob, _ := rdf.NewURIRef("http://example.org/Bob")

	// Use Resource API
	aliceR := rdf.NewResource(g, alice)
	bobR := rdf.NewResource(g, bob)
	for _, add := range []struct {
		r *rdf.Resource
		p rdf.URIRef
		o rdf.Term
	}{
		{aliceR, name, rdf.NewLiteral("Alice")},
		{aliceR, age, rdf.NewLiteral(30)},
		{aliceR, knows, bob},
		{bobR, name, rdf.NewLiteral("Bob")},
		{bobR, age, rdf.NewLiteral(25)},
	} {
		if err := add.r.Add(ctx, add.p, add.o); err != nil {
			return err
		}
	}

	// Read values
	aliceName, _, err := aliceR.Value(ctx, name)
	if err != nil {
		return err
	}
	fmt.Printf("Name: %s\n", aliceName)

	aliceAge, _, err := aliceR.Value(ctx, age)
	if err != nil {
		return err
	}
	fmt.Printf("Age: %s\n", aliceAge)

	// List objects
	fmt.Println("\nAlice knows:")
	for t, err := range aliceR.Objects(ctx, knows) {
		if err != nil {
			return err
		}
		fmt.Printf("  %s\n", t.N3())
	}

	// Set (replace) value
	if err := aliceR.Set(ctx, age, rdf.NewLiteral(31)); err != nil {
		return err
	}
	newAge, _, err := aliceR.Value(ctx, age)
	if err != nil {
		return err
	}
	fmt.Printf("\nUpdated age: %s\n", newAge)

	// List all predicate-object pairs (sorted for determinism)
	fmt.Println("\nAlice's properties:")
	var props []string
	for po, err := range aliceR.PredicateObjects(ctx) {
		if err != nil {
			return err
		}
		props = append(props, fmt.Sprintf("  %s = %s", po.First.N3(), po.Second.N3()))
	}
	sort.Strings(props)
	for _, s := range props {
		fmt.Println(s)
	}

	n, err := g.Len(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("\nGraph has %d triples\n", n)
	return nil
}
