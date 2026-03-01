// Package main demonstrates transitive closure queries using property paths.
// Ported from: rdflib/examples/transitive.py
package main

import (
	"fmt"
	"sort"
	"strings"

	rdf "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/turtle"
)

func main() {
	g := rdf.NewGraph()
	turtle.Parse(g, strings.NewReader(`
		@prefix ex: <http://example.org/> .

		ex:person ex:parent ex:mom .
		ex:person ex:parent ex:dad .
		ex:mom ex:parent ex:gm1 .
		ex:mom ex:parent ex:gp1 .
		ex:dad ex:parent ex:gm2 .
		ex:dad ex:parent ex:gp2 .
	`))

	person, _ := rdf.NewURIRef("http://example.org/person")
	parent, _ := rdf.NewURIRef("http://example.org/parent")
	gm1, _ := rdf.NewURIRef("http://example.org/gm1")

	// Transitive objects: all ancestors of person (parent+)
	fmt.Println("All ancestors of person (parent+):")
	printSorted(collectNames(g, rdf.OneOrMore(rdf.AsPath(parent)), person, nil))

	// Transitive subjects: all descendants of gm1 (^parent+)
	fmt.Println("\nAll descendants of gm1 (^parent+):")
	printSorted(collectNames(g, rdf.OneOrMore(rdf.Inv(rdf.AsPath(parent))), gm1, nil))

	// ZeroOrMore: ancestors including self (parent*)
	fmt.Println("\nAncestors including self (parent*):")
	printSorted(collectNames(g, rdf.ZeroOrMore(rdf.AsPath(parent)), person, nil))
}

func collectNames(g *rdf.Graph, path rdf.Path, subj rdf.Subject, obj rdf.Term) []string {
	var names []string
	path.Eval(g, subj, obj)(func(s, o rdf.Term) bool {
		names = append(names, localName(o))
		return true
	})
	return names
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
