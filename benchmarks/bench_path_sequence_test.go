package benchmarks_test

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// Property-path benchmarks over a typed tree.
//
// generateTypedTree builds a tree of items linked by ex:part (a root, fanout
// children per node, depth levels below it). Every item has an rdf:type among
// 100 classes, each class a subclass of one of 10 groups, each group a
// subclass of ex:Item. One class therefore has about 1% of the items, one
// group about 10%. It returns the graph and its number of items.
func generateTypedTree(fanout, depth int) (*graph.Graph, int) {
	const ex = "http://example.org/"
	g := graph.NewGraph()
	part := term.NewURIRefUnsafe(ex + "part")
	sub := term.NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#subClassOf")
	item := term.NewURIRefUnsafe(ex + "Item")
	for j := range 10 {
		mustAdd(g, term.NewURIRefUnsafe(fmt.Sprintf("%sGroup%d", ex, j)), sub, item)
	}
	for c := range 100 {
		mustAdd(g, term.NewURIRefUnsafe(fmt.Sprintf("%sClass%d", ex, c)), sub,
			term.NewURIRefUnsafe(fmt.Sprintf("%sGroup%d", ex, c%10)))
	}
	n := 0
	node := func() term.URIRef {
		u := term.NewURIRefUnsafe(fmt.Sprintf("%sn%d", ex, n))
		mustAdd(g, u, rdfType, term.NewURIRefUnsafe(fmt.Sprintf("%sClass%d", ex, n%100)))
		n++
		return u
	}
	level := []term.URIRef{node()}
	for range depth {
		var next []term.URIRef
		for _, parent := range level {
			for range fanout {
				child := node()
				mustAdd(g, parent, part, child)
				next = append(next, child)
			}
		}
		level = next
	}
	return g, n
}

// BenchmarkSequencePathBoundObject evaluates rdf:type/rdfs:subClassOf* with
// the class bound and the instance unbound, alone and joined with a
// transitive path from the root. RareFirstStep is the opposite shape: a
// sequence whose first step has 10 triples and whose last step has 10,000
// triples into the bound object, where starting from the object would cost
// more than starting from the first step.
func BenchmarkSequencePathBoundObject(b *testing.B) {
	const prefix = `PREFIX ex: <http://example.org/>
PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
`
	queries := []struct{ name, query string }{
		{"Class", `SELECT ?x WHERE { ?x a/rdfs:subClassOf* ex:Class7 }`},
		{"Group", `SELECT ?x WHERE { ?x a/rdfs:subClassOf* ex:Group7 }`},
		{"ClassUnderRoot", `SELECT ?x WHERE { ?x a/rdfs:subClassOf* ex:Class7 . ex:n0 ex:part+ ?x }`},
	}
	for _, size := range []struct{ fanout, depth int }{{10, 3}, {20, 3}} {
		g, items := generateTypedTree(size.fanout, size.depth)
		for _, q := range queries {
			b.Run(fmt.Sprintf("%s/%ditems", q.name, items), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := sparql.Query(g, prefix+q.query); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
	b.Run("RareFirstStep", func(b *testing.B) {
		const ex = "http://example.org/"
		g := graph.NewGraph()
		rare, common, hub := term.NewURIRefUnsafe(ex+"rare"), term.NewURIRefUnsafe(ex+"common"), term.NewURIRefUnsafe(ex+"hub")
		for i := range 10_000 {
			mustAdd(g, term.NewURIRefUnsafe(fmt.Sprintf("%sm%d", ex, i)), common, hub)
		}
		for i := range 10 {
			mustAdd(g, term.NewURIRefUnsafe(fmt.Sprintf("%ss%d", ex, i)), rare, term.NewURIRefUnsafe(fmt.Sprintf("%sm%d", ex, i)))
		}
		b.ReportAllocs()
		for b.Loop() {
			if _, err := sparql.Query(g, prefix+`SELECT ?x WHERE { ?x ex:rare/ex:common ex:hub }`); err != nil {
				b.Fatal(err)
			}
		}
	})
}
