package benchmarks_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// Groups split by a FILTER or a nested group.
//
// The parser closes a BGP at a FILTER and at a nested group, so each query
// below is a join of two BGPs. The first BGP of FilterSplit holds two
// independent walks from ex:root and cannot see that ?b is restricted to the
// one ex:ClassB by the second; NestedGroup restricts the only variable in the
// nested group.
var joinBGPsQueries = []struct {
	name, query string
	pre         bool // run with ?this pre-bound to ex:root
}{
	{"FilterSplit", `PREFIX ex: <http://example.org/>
		SELECT ?a ?b WHERE {
			ex:root ex:part ?a , ?b . ?a ex:type ?c .
			FILTER (?c IN (ex:ClassA))
			?b ex:type ex:ClassB .
		}`, false},
	{"FilterSplitPreBound", `PREFIX ex: <http://example.org/>
		SELECT ?a ?b WHERE {
			?this ex:part ?a , ?b . ?a ex:type ?c .
			FILTER (?c IN (ex:ClassA))
			?b ex:type ex:ClassB .
		}`, true},
	{"NestedGroup", `PREFIX ex: <http://example.org/>
		SELECT ?b WHERE {
			ex:root ex:part ?b .
			{ ?b ex:type ex:ClassB }
		}`, false},
}

// joinBGPsGraph gives ex:root n parts: two ex:ClassA, one ex:ClassB, the rest
// ex:ClassC.
func joinBGPsGraph(b *testing.B, n int) *graph.Graph {
	ctx := context.Background()
	g := graph.NewGraph()
	ex := func(f string, a ...any) term.URIRef {
		return term.NewURIRefUnsafe("http://example.org/" + fmt.Sprintf(f, a...))
	}
	for i := range n {
		class := "ClassC"
		switch i {
		case 0, 1:
			class = "ClassA"
		case 2:
			class = "ClassB"
		}
		p := ex("part%d", i)
		for _, t := range [][3]term.Term{{ex("root"), ex("part"), p}, {p, ex("type"), ex("%s", class)}} {
			if err := g.Add(ctx, t[0].(term.Subject), t[1].(term.URIRef), t[2]); err != nil {
				b.Fatal(err)
			}
		}
	}
	return g
}

func BenchmarkJoinBGPs(b *testing.B) {
	for _, size := range []int{300} {
		g := joinBGPsGraph(b, size)
		root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
		for _, q := range joinBGPsQueries {
			b.Run(fmt.Sprintf("%s/%d", q.name, size), func(b *testing.B) {
				b.ReportAllocs()
				var pre []map[string]term.Term
				if q.pre {
					pre = append(pre, root)
				}
				for b.Loop() {
					if _, err := sparql.Query(g, q.query, pre...); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
