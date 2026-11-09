package benchmarks_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// Joins evaluated with initial bindings, and in EXISTS bodies.
//
// A group split by a FILTER, a group followed by a UNION or a subquery is a
// Join. With ?this pre-bound (as SHACL runs every SPARQL constraint), and in
// an EXISTS body (evaluated with the outer row pre-bound), the right side was
// evaluated again for every left row without the row's values.
var preboundJoinQueries = []struct {
	name, query string
	pre         bool // run with ?this pre-bound to ex:root
}{
	{"FilterSplit", `PREFIX ex: <http://example.org/>
		SELECT ?p ?k WHERE {
			?this ex:part ?p FILTER (?p != ex:item0)
			?p ex:type ?k .
		}`, true},
	{"UnionRight", `PREFIX ex: <http://example.org/>
		SELECT ?p ?z WHERE {
			?this ex:part ?p
			{ ?p ex:val ?z } UNION { ?p ex:type ?z }
		}`, true},
	{"SubqueryRight", `PREFIX ex: <http://example.org/>
		SELECT ?p ?n WHERE {
			?this ex:part ?p
			{ SELECT (COUNT(?x) AS ?n) WHERE { ?x ex:type ex:ClassA } }
		}`, true},
	{"ExistsBody", `PREFIX ex: <http://example.org/>
		SELECT ?p WHERE {
			ex:root ex:part ?p
			FILTER EXISTS { ?p ex:group ?k FILTER (?k != ex:group0) ?k ex:label ?l }
		}`, false},
}

// preboundJoinGraph gives ex:root n parts, of types ex:ClassA, ex:ClassB and
// ex:ClassC in turn; every other part has an ex:val. Part i is in group
// i mod 200, and each group has an ex:label.
func preboundJoinGraph(b *testing.B, n int) *graph.Graph {
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
	classes := []string{"ClassA", "ClassB", "ClassC"}
	for i := range n {
		item := ex("item%d", i)
		add(ex("root"), ex("part"), item)
		add(item, ex("type"), ex("%s", classes[i%3]))
		if i%2 == 0 {
			add(item, ex("val"), term.NewLiteral(i))
		}
		add(item, ex("group"), ex("group%d", i%200))
	}
	for i := range 200 {
		add(ex("group%d", i), ex("label"), term.NewLiteral(fmt.Sprintf("group %d", i)))
	}
	return g
}

func BenchmarkPreBoundJoin(b *testing.B) {
	for _, size := range []int{500} {
		g := preboundJoinGraph(b, size)
		root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
		for _, q := range preboundJoinQueries {
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
