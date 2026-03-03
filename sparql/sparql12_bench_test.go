package sparql_test

import (
	"fmt"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
)

func BenchmarkTripleTermMatch(b *testing.B) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://ex/s")
	p := rdflibgo.NewURIRefUnsafe("http://ex/p")
	for i := 0; i < 1000; i++ {
		g.Add(s, p, rdflibgo.NewTripleTerm(
			rdflibgo.NewURIRefUnsafe(fmt.Sprintf("http://ex/a%d", i)),
			rdflibgo.NewURIRefUnsafe("http://ex/q"),
			rdflibgo.NewLiteral(i),
		))
	}
	query := `SELECT ?val WHERE { <http://ex/s> <http://ex/p> <<( ?x <http://ex/q> ?val )>> }`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, query)
	}
}

func BenchmarkNestedTripleTermParse(b *testing.B) {
	query := `PREFIX : <http://ex/>
	SELECT * WHERE {
		:s :p <<( :a :b <<( :c :d <<( :e :f ?x )>> )>> )>>
	}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Parse(query)
	}
}

func BenchmarkCodepointEscapes(b *testing.B) {
	query := `SELECT * WHERE { ` + strings.Repeat(`\u0041`, 100) + ` ?s ?p ?o }`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Parse(query)
	}
}

func BenchmarkAnnotationExpansion(b *testing.B) {
	query := `PREFIX : <http://ex/>
	SELECT * WHERE {
		:s :p :o {| :q1 :z1 |} {| :q2 :z2 |} {| :q3 :z3 |} .
	}`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Parse(query)
	}
}
