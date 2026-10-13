// Package main demonstrates transitive closure queries using property paths.
// Ported from: rdflib/examples/transitive.py
package main

import (
	"context"
	"fmt"
	"log"
	"sort"
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
		@prefix ex: <http://example.org/> .

		ex:person ex:parent ex:mom .
		ex:person ex:parent ex:dad .
		ex:mom ex:parent ex:gm1 .
		ex:mom ex:parent ex:gp1 .
		ex:dad ex:parent ex:gm2 .
		ex:dad ex:parent ex:gp2 .
	`)); err != nil {
		return err
	}

	person, _ := rdf.NewURIRef("http://example.org/person")
	parent, _ := rdf.NewURIRef("http://example.org/parent")
	gm1, _ := rdf.NewURIRef("http://example.org/gm1")

	// Transitive objects: all ancestors of person (parent+)
	fmt.Println("All ancestors of person (parent+):")
	names, err := collectNames(ctx, g, rdf.OneOrMore(rdf.AsPath(parent)), person, nil)
	if err != nil {
		return err
	}
	printSorted(names)

	// Transitive subjects: all descendants of gm1 (^parent+)
	fmt.Println("\nAll descendants of gm1 (^parent+):")
	names, err = collectNames(ctx, g, rdf.OneOrMore(rdf.Inv(rdf.AsPath(parent))), gm1, nil)
	if err != nil {
		return err
	}
	printSorted(names)

	// ZeroOrMore: ancestors including self (parent*)
	fmt.Println("\nAncestors including self (parent*):")
	names, err = collectNames(ctx, g, rdf.ZeroOrMore(rdf.AsPath(parent)), person, nil)
	if err != nil {
		return err
	}
	printSorted(names)
	return nil
}

func collectNames(ctx context.Context, g *rdf.Graph, path rdf.Path, subj rdf.Subject, obj rdf.Term) ([]string, error) {
	var names []string
	for pair, err := range path.Eval(ctx, g, subj, obj) {
		if err != nil {
			return nil, err
		}
		names = append(names, localName(pair.Second))
	}
	return names, nil
}

func printSorted(names []string) {
	sort.Strings(names)
	for _, n := range names {
		fmt.Printf("  %s\n", n)
	}
}

func localName(t rdf.Term) string {
	s := t.String()
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return s[i+1:]
	}
	return s
}
