package benchmarks_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

// --- Term creation ---

func BenchmarkNewURIRef(b *testing.B) {
	for i := 0; i < b.N; i++ {
		term.NewURIRef("http://example.org/resource")
	}
}

func BenchmarkNewBNode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		term.NewBNode()
	}
}

func BenchmarkNewLiteralString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		term.NewLiteral("hello world")
	}
}

func BenchmarkNewLiteralInt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		term.NewLiteral(42)
	}
}

// --- N3 serialization ---

func BenchmarkURIRefN3(b *testing.B) {
	u, _ := term.NewURIRef("http://example.org/resource")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		u.N3()
	}
}

func BenchmarkLiteralN3(b *testing.B) {
	l := term.NewLiteral("hello world")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.N3()
	}
}

// --- Literal equality ---

func BenchmarkLiteralEq(b *testing.B) {
	l1 := term.NewLiteral("1", term.WithDatatype(term.XSDInteger))
	l2 := term.NewLiteral("01", term.WithDatatype(term.XSDInteger))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l1.Eq(l2)
	}
}

// --- Store add 10k ---

func BenchmarkStoreAdd_10k(b *testing.B) {
	pred, _ := term.NewURIRef("http://example.org/p")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s := store.NewMemoryStore()
		for j := 0; j < 10000; j++ {
			sub := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", j))
			s.Add(term.Triple{Subject: sub, Predicate: pred, Object: term.NewLiteral(j)}, nil)
		}
	}
}

// --- Store triples lookup ---

func BenchmarkStoreTriples_1k(b *testing.B) {
	s := store.NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	for i := 0; i < 1000; i++ {
		s.Add(term.Triple{Subject: sub, Predicate: pred, Object: term.NewLiteral(i)}, nil)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Triples(term.TriplePattern{Subject: sub, Predicate: &pred}, nil)(func(term.Triple) bool {
			return true
		})
	}
}

// --- Parse Turtle ---

var turtleData = `
@prefix ex: <http://example.org/> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

ex:Alice a ex:Person ;
    rdfs:label "Alice" ;
    ex:knows ex:Bob .

ex:Bob a ex:Person ;
    rdfs:label "Bob" .
`

func BenchmarkParseTurtle(b *testing.B) {
	for i := 0; i < b.N; i++ {
		g := graph.NewGraph()
		turtle.Parse(g, strings.NewReader(turtleData))
	}
}

// --- Serialize Turtle ---

func BenchmarkSerializeTurtle(b *testing.B) {
	g := graph.NewGraph()
	turtle.Parse(g, strings.NewReader(turtleData))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		turtle.Serialize(g, &buf)
	}
}

// --- SPARQL select ---

func BenchmarkSPARQLSelect(b *testing.B) {
	g := graph.NewGraph()
	rdfType := namespace.RDF.Type
	thing, _ := term.NewURIRef("http://example.org/Thing")
	valPred, _ := term.NewURIRef("http://example.org/value")
	for i := 0; i < 100; i++ {
		s := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/s%d", i))
		g.Add(s, rdfType, thing)
		g.Add(s, valPred, term.NewLiteral(i))
	}
	query := "SELECT ?s ?v WHERE { ?s a <http://example.org/Thing> ; <http://example.org/value> ?v } LIMIT 50"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, query)
	}
}
