package benchmarks_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// BGP groups that share no variable.
//
// Each query is one BGP made of two groups joined only through a constant.
// Evaluated as nested loops, the second group runs again for every solution of
// the first; evaluated once per group, the cost is the two groups plus their
// cross product.
var bgpGroupsQueries = []struct {
	name, query string
}{
	{"TransitiveWalks", `PREFIX ex: <http://example.org/>
		SELECT ?a ?b WHERE {
			ex:root ex:part+ ?a . ?a ex:type ex:ClassA .
			ex:root ex:part+ ?b . ?b ex:type ex:ClassB .
		}`},
	{"Plain", `PREFIX ex: <http://example.org/>
		SELECT ?a ?b ?v WHERE {
			?a ex:type ex:ClassA .
			?b ex:val ?v .
		}`},
}

// bgpGroupsGraph is a two-level tree under ex:root: n/20 assemblies of 20
// items each. One item in 20 is an ex:ClassA, one in 20 an ex:ClassB, and one
// in 10 has an ex:val.
func bgpGroupsGraph(b *testing.B, n int) *graph.Graph {
	ctx := context.Background()
	g := graph.NewGraph()
	ex := func(f string, a ...any) term.URIRef {
		return term.NewURIRefUnsafe("http://example.org/" + fmt.Sprintf(f, a...))
	}
	add := func(s term.Subject, p term.URIRef, o term.Term) {
		if err := g.Add(ctx, s, p, o); err != nil {
			b.Fatal(err)
		}
	}
	for i := range n {
		asm := ex("asm%d", i/20)
		if i%20 == 0 {
			add(ex("root"), ex("part"), asm)
		}
		item := ex("item%d", i)
		add(asm, ex("part"), item)
		switch i % 20 {
		case 0:
			add(item, ex("type"), ex("ClassA"))
		case 1:
			add(item, ex("type"), ex("ClassB"))
		default:
			add(item, ex("type"), ex("ClassC"))
		}
		if i%10 == 0 {
			add(item, ex("val"), term.NewLiteral(i))
		}
	}
	return g
}

func BenchmarkBGPGroups(b *testing.B) {
	for _, size := range []int{1_000} {
		g := bgpGroupsGraph(b, size)
		for _, q := range bgpGroupsQueries {
			b.Run(fmt.Sprintf("%s/%d", q.name, size), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := sparql.Query(g, q.query); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
