package sparql

import (
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

func setupBenchGraph(n int) *rdflibgo.Graph {
	g := rdflibgo.NewGraph()
	ns := "http://example.org/"
	for i := range n {
		s := rdflibgo.NewURIRefUnsafe(ns + "s" + string(rune('0'+i%10)))
		p := rdflibgo.NewURIRefUnsafe(ns + "p")
		o := rdflibgo.NewLiteral(i, rdflibgo.WithDatatype(rdflibgo.XSDInteger))
		g.Add(s, p, o)
	}
	return g
}

func BenchmarkSimpleSelect(b *testing.B) {
	g := setupBenchGraph(1000)
	q := `PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE { ?s ex:p ?o }`
	b.ResetTimer()
	for range b.N {
		_, err := Query(g, q)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSelectWithFilter(b *testing.B) {
	g := setupBenchGraph(1000)
	q := `PREFIX ex: <http://example.org/>
SELECT ?s ?o WHERE { ?s ex:p ?o . FILTER(?o > 500) }`
	b.ResetTimer()
	for range b.N {
		_, err := Query(g, q)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSelectWithRegex(b *testing.B) {
	g := rdflibgo.NewGraph()
	ns := "http://example.org/"
	p := rdflibgo.NewURIRefUnsafe(ns + "name")
	for i := range 500 {
		s := rdflibgo.NewURIRefUnsafe(ns + "s" + string(rune('A'+i%26)))
		g.Add(s, p, rdflibgo.NewLiteral("Alice"+string(rune('0'+i%10))))
	}
	q := `PREFIX ex: <http://example.org/>
SELECT ?s WHERE { ?s ex:name ?n . FILTER(REGEX(?n, "^Ali")) }`
	b.ResetTimer()
	for range b.N {
		_, err := Query(g, q)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAggregateCount(b *testing.B) {
	g := setupBenchGraph(1000)
	q := `PREFIX ex: <http://example.org/>
SELECT (COUNT(?o) AS ?c) WHERE { ?s ex:p ?o }`
	b.ResetTimer()
	for range b.N {
		_, err := Query(g, q)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAsk(b *testing.B) {
	g := setupBenchGraph(100)
	q := `PREFIX ex: <http://example.org/>
ASK { ex:s0 ex:p ?o }`
	b.ResetTimer()
	for range b.N {
		_, err := Query(g, q)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	q := `PREFIX ex: <http://example.org/>
SELECT ?s ?p ?o WHERE {
  ?s ?p ?o .
  OPTIONAL { ?s ex:name ?name }
  FILTER(?o > 10 && BOUND(?name))
}
ORDER BY ?s
LIMIT 100`
	b.ResetTimer()
	for range b.N {
		_, err := Parse(q)
		if err != nil {
			b.Fatal(err)
		}
	}
}
