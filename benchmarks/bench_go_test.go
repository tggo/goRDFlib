package benchmarks_test

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/shacl"
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
			if err := s.Add(b.Context(), term.Triple{Subject: sub, Predicate: pred, Object: term.NewLiteral(j)}, nil); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// --- Store triples lookup ---

func BenchmarkStoreTriples_1k(b *testing.B) {
	s := store.NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	for i := 0; i < 1000; i++ {
		if err := s.Add(b.Context(), term.Triple{Subject: sub, Predicate: pred, Object: term.NewLiteral(i)}, nil); err != nil {
			b.Fatal(err)
		}
	}
	ctx := b.Context()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, err := range s.Triples(ctx, term.TriplePattern{Subject: sub, Predicate: &pred}, nil) {
			if err != nil {
				b.Fatal(err)
			}
		}
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
		if err := turtle.Parse(b.Context(), g, strings.NewReader(turtleData)); err != nil {
			b.Fatal(err)
		}
	}
}

// --- Serialize Turtle ---

func BenchmarkSerializeTurtle(b *testing.B) {
	g := graph.NewGraph()
	if err := turtle.Parse(b.Context(), g, strings.NewReader(turtleData)); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := turtle.Serialize(b.Context(), g, &buf); err != nil {
			b.Fatal(err)
		}
	}
}

// --- SPARQL select ---

// --- SHACL validation (small: 10 nodes, 5 constraints) ---

var shaclShapesSmall = `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
        sh:maxCount 1 ;
        sh:datatype xsd:string ;
    ] ;
    sh:property [
        sh:path ex:age ;
        sh:datatype xsd:integer ;
        sh:minInclusive 0 ;
        sh:maxInclusive 150 ;
    ] .
`

func makeSHACLDataSmall() string {
	var sb strings.Builder
	sb.WriteString("@prefix ex: <http://example.org/> .\n")
	sb.WriteString("@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .\n\n")
	for i := 0; i < 10; i++ {
		fmt.Fprintf(&sb, "ex:person%d a ex:Person ; ex:name \"Person %d\" ; ex:age %d .\n", i, i, 20+i)
	}
	return sb.String()
}

func BenchmarkSHACLValidateSmall(b *testing.B) {
	shapes, _ := shacl.LoadTurtleString(shaclShapesSmall, "")
	data, _ := shacl.LoadTurtleString(makeSHACLDataSmall(), "")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		shacl.Validate(data, shapes)
	}
}

// --- SHACL validation (medium: 100 nodes, 5 constraints) ---

func makeSHACLDataMedium() string {
	var sb strings.Builder
	sb.WriteString("@prefix ex: <http://example.org/> .\n")
	sb.WriteString("@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .\n\n")
	for i := 0; i < 100; i++ {
		fmt.Fprintf(&sb, "ex:person%d a ex:Person ; ex:name \"Person %d\" ; ex:age %d .\n", i, i, 20+i%100)
	}
	return sb.String()
}

func BenchmarkSHACLValidateMedium(b *testing.B) {
	shapes, _ := shacl.LoadTurtleString(shaclShapesSmall, "")
	data, _ := shacl.LoadTurtleString(makeSHACLDataMedium(), "")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		shacl.Validate(data, shapes)
	}
}

// --- SHACL validation (complex: multiple shapes, logical constraints) ---

var shaclShapesComplex = `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
        sh:datatype xsd:string ;
        sh:minLength 1 ;
        sh:maxLength 100 ;
    ] ;
    sh:property [
        sh:path ex:email ;
        sh:pattern "^[^@]+@[^@]+$" ;
    ] ;
    sh:property [
        sh:path ex:knows ;
        sh:class ex:Person ;
    ] ;
    sh:property [
        sh:path ex:status ;
        sh:in ( "active" "inactive" "pending" ) ;
    ] .
`

func makeSHACLDataComplex() string {
	var sb strings.Builder
	sb.WriteString("@prefix ex: <http://example.org/> .\n\n")
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&sb, "ex:p%d a ex:Person ; ex:name \"Person %d\" ; ex:email \"p%d@example.org\" ; ex:status \"active\" .\n", i, i, i)
		if i > 0 {
			fmt.Fprintf(&sb, "ex:p%d ex:knows ex:p%d .\n", i, i-1)
		}
	}
	return sb.String()
}

func BenchmarkSHACLValidateComplex(b *testing.B) {
	shapes, _ := shacl.LoadTurtleString(shaclShapesComplex, "")
	data, _ := shacl.LoadTurtleString(makeSHACLDataComplex(), "")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		shacl.Validate(data, shapes)
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
		if err := g.Add(b.Context(), s, rdfType, thing); err != nil {
			b.Fatal(err)
		}
		if err := g.Add(b.Context(), s, valPred, term.NewLiteral(i)); err != nil {
			b.Fatal(err)
		}
	}
	query := "SELECT ?s ?v WHERE { ?s a <http://example.org/Thing> ; <http://example.org/value> ?v } LIMIT 50"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sparql.Query(g, query)
	}
}
