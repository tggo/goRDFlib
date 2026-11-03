package sparql

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// BenchmarkDecimal measures the hot paths exact xsd:decimal touches: a
// comparison FILTER, arithmetic in an aggregate, and ORDER BY, over 10k
// decimal literals.
func BenchmarkDecimal(b *testing.B) {
	g := graph.NewGraph()
	p := term.NewURIRefUnsafe("urn:v")
	for i := range 10000 {
		s := term.NewURIRefUnsafe(fmt.Sprintf("urn:s%d", i))
		v := term.NewLiteral(fmt.Sprintf("%d.%02d", i%997, i%100), term.WithDatatype(term.XSDDecimal))
		if err := g.Add(b.Context(), s, p, v); err != nil {
			b.Fatal(err)
		}
	}
	for name, q := range map[string]string{
		"filter":  `SELECT ?s WHERE { ?s <urn:v> ?v FILTER (?v > 100.5 && ?v < 800.25) }`,
		"sum":     `SELECT (SUM(?v * 1.5 + 0.25) AS ?t) WHERE { ?s <urn:v> ?v }`,
		"orderby": `SELECT ?v WHERE { ?s <urn:v> ?v } ORDER BY ?v LIMIT 10`,
	} {
		pq, err := Parse(q)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if _, err := EvalQuery(g, pq, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
