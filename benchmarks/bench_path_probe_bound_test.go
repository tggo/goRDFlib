package benchmarks_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/shacl"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// BenchmarkPathProbeBound checks, in one evaluation, that every employee of
// an org chart has someone above them who holds a budget:
//
//	?e ex:worksFor ex:org
//	FILTER NOT EXISTS { ?e ex:reportsTo+ ?m . ?m ex:holdsBudget ?b }
//
// The chart is a binary tree and every employee with reports holds a budget,
// so the BGP inside NOT EXISTS is planned once per employee and probes its
// path once per plan. A probed plan starts from the few managers above ?e. A
// plan made after the evaluation has spent its path probes
// (sparql.WithMaxPathProbes, 64 by default) ranks the path unprobed and
// starts from ex:holdsBudget, reading every budget for that employee. Query
// runs it as a SELECT, SHACL as a SHACL-SPARQL constraint on ex:org (one
// focus node). The bound of 256 covers every row of the small chart (255
// employees) and an eighth of the large one (2,047): the bound a query
// needs grows with its data.
func BenchmarkPathProbeBound(b *testing.B) {
	const query = `PREFIX ex: <http://example.org/>
SELECT ?e WHERE {
	?e ex:worksFor ex:org
	FILTER NOT EXISTS { ?e ex:reportsTo+ ?m . ?m ex:holdsBudget ?b }
}`
	shapes, err := shacl.LoadTurtleString(pathProbeBoundShapes, "")
	if err != nil {
		b.Fatal(err)
	}
	bounds := []struct {
		name string
		n    int // -1: the option is not set
	}{{"default", -1}, {"256", 256}, {"none", 0}}
	for _, depth := range []int{7, 10} {
		g, employees := orgChartGraph(depth)
		data, err := shacl.NewGraphFromRDF(context.Background(), g, "")
		if err != nil {
			b.Fatal(err)
		}
		for _, bound := range bounds {
			ctx := context.Background()
			if bound.n >= 0 {
				ctx = sparql.WithMaxPathProbes(ctx, bound.n)
			}
			b.Run(fmt.Sprintf("Query/%demployees/bound=%s", employees, bound.name), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					res, err := sparql.QueryContext(ctx, g, query)
					if err != nil {
						b.Fatal(err)
					}
					if len(res.Bindings) != 1 {
						b.Fatalf("%d rows, want 1 (the root)", len(res.Bindings))
					}
				}
			})
			b.Run(fmt.Sprintf("SHACL/%demployees/bound=%s", employees, bound.name), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					report, err := shacl.ValidateContext(ctx, data, shapes)
					if err != nil {
						b.Fatal(err)
					}
					if len(report.Results) != 1 {
						b.Fatalf("%d results, want 1 (the root)", len(report.Results))
					}
				}
			})
		}
	}
}

const pathProbeBoundShapes = `
@prefix ex: <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .

ex:OrgShape a sh:NodeShape ;
    sh:targetNode ex:org ;
    sh:sparql [
        sh:select """SELECT $this ?value WHERE {
            ?value <http://example.org/worksFor> $this
            FILTER NOT EXISTS { ?value <http://example.org/reportsTo>+ ?m . ?m <http://example.org/holdsBudget> ?b }
        }""" ;
    ] .
`

// orgChartGraph builds a complete binary tree of depth levels below its root
// (2^(depth+1)-1 employees of ex:org) linked by ex:reportsTo; every employee
// with reports holds a budget.
func orgChartGraph(depth int) (*graph.Graph, int) {
	const ex = "http://example.org/"
	g := graph.NewGraph()
	u := func(f string, a ...any) term.URIRef { return term.NewURIRefUnsafe(ex + fmt.Sprintf(f, a...)) }
	employees := 1<<(depth+1) - 1
	for i := range employees {
		e := u("e%d", i)
		mustAdd(g, e, u("worksFor"), u("org"))
		if i > 0 {
			mustAdd(g, e, u("reportsTo"), u("e%d", (i-1)/2))
		}
		if 2*i+1 < employees {
			mustAdd(g, e, u("holdsBudget"), u("budget%d", i))
		}
	}
	return g, employees
}
