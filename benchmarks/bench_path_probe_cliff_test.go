package benchmarks_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// generateOrgTree builds n employees in a tree of the given fanout: employee
// i reports to (i-1)/fanout, every employee works for ex:org, and every
// employee with reports holds a budget.
func generateOrgTree(n, fanout int) *graph.Graph {
	ctx := context.Background()
	g := graph.NewGraph()
	u := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	emp := func(i int) term.URIRef { return u(fmt.Sprintf("e%d", i)) }
	for i := range n {
		g.Add(ctx, emp(i), u("worksFor"), u("org"))
		if i > 0 {
			g.Add(ctx, emp(i), u("reportsTo"), emp((i-1)/fanout))
		}
		if i*fanout+1 < n {
			g.Add(ctx, emp(i), u("holdsBudget"), term.NewLiteral(fmt.Sprintf("b%d", i)))
		}
	}
	return g
}

// BenchmarkPathProbeCliff: an enclosing pattern plans the inner BGP once per
// outer row. The path ex:reportsTo+ from the bound ?e reaches a handful of
// managers; ex:holdsBudget matches a quarter of the employees. Once the
// planner's path probes run out, the plan starts from the large pattern.
func BenchmarkPathProbeCliff(b *testing.B) {
	queries := map[string]string{
		"NotExists": `PREFIX ex: <http://example.org/>
SELECT ?e WHERE { ?e ex:worksFor ex:org
  FILTER NOT EXISTS { ?e ex:reportsTo+ ?m . ?m ex:holdsBudget ?b } }`,
		"Optional": `PREFIX ex: <http://example.org/>
SELECT ?e ?b WHERE { ?e ex:worksFor ex:org
  OPTIONAL { ?e ex:reportsTo+ ?m . ?m ex:holdsBudget ?b } }`,
	}
	for _, n := range []int{250, 500, 1000, 2000, 4000, 16000, 64000} {
		g := generateOrgTree(n, 4)
		for _, name := range []string{"NotExists", "Optional"} {
			query := queries[name]
			b.Run(fmt.Sprintf("%s/%demployees", name, n), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if _, err := sparql.Query(g, query); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
