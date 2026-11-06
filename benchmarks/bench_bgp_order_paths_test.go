package benchmarks_test

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/sparql"
)

// BenchmarkBGPOrderPaths joins a transitive path from a root with
// rdf:type/rdfs:subClassOf* to a class, both with a known endpoint, written
// with the large path first. The type path matches about 1% (Class) or 10%
// (Group) of the items; ex:n0 reaches every item, ex:n1 about 1/fanout of
// them. PerRowOptional evaluates such a pair once per left row, where the
// planner's probes are paid on every row. See generateTypedTree.
func BenchmarkBGPOrderPaths(b *testing.B) {
	const prefix = `PREFIX ex: <http://example.org/>
PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
`
	queries := []struct{ name, query string }{
		{"ClassUnderRoot", `SELECT ?x WHERE { ex:n0 ex:part+ ?x . ?x a/rdfs:subClassOf* ex:Class7 }`},
		{"ClassUnderSubtree", `SELECT ?x WHERE { ex:n1 ex:part+ ?x . ?x a/rdfs:subClassOf* ex:Class7 }`},
		{"GroupUnderRoot", `SELECT ?x WHERE { ex:n0 ex:part+ ?x . ?x a/rdfs:subClassOf* ex:Group7 }`},
		{"PerRowOptional", `SELECT ?y ?x WHERE { ?y a ex:Class3 . OPTIONAL { ?y ex:part ?x . ?x a/rdfs:subClassOf* ex:Group7 } }`},
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
}
