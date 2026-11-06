package benchmarks_test

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/sparql"
)

// BenchmarkTransitivePathBothEndsBound runs a transitive path once per row
// with both endpoints bound: every typed item (Items) or every item of one
// class (Class) is tested for being below the root, which reaches every item,
// or below ex:n1, which reaches about 1/fanout of them. See
// generateTypedTree.
func BenchmarkTransitivePathBothEndsBound(b *testing.B) {
	const prefix = `PREFIX ex: <http://example.org/>
`
	queries := []struct{ name, query string }{
		{"ItemsUnderRoot", `SELECT ?x WHERE { ?x a ?c . ex:n0 ex:part+ ?x }`},
		{"ItemsUnderSubtree", `SELECT ?x WHERE { ?x a ?c . ex:n1 ex:part+ ?x }`},
		{"ClassUnderRoot", `SELECT ?x WHERE { ?x a ex:Class7 . ex:n0 ex:part+ ?x }`},
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
