package shacl

import (
	"slices"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

const fromTriplesShapes = `@prefix sh: <http://www.w3.org/ns/shacl#> . @prefix ex: <http://example.org/> .
ex:Core a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:property [ sh:path ex:size ; sh:minCount 1 ; sh:maxInclusive 5 ] ;
  sh:property [ sh:path ex:part ; sh:class ex:Item ] .`

// A SPARQL constraint queries the rdflib graph, so it makes the graph build one.
const fromTriplesSPARQLShapes = fromTriplesShapes + `
ex:Sparql a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:sparql [ sh:select """PREFIX ex: <http://example.org/>
    SELECT $this ?value WHERE { $this ex:size ?value . ?o ex:size ?value FILTER (?o != $this) }""" ] .`

// fromTriplesData builds, in Go code, the triples a program would hold.
func fromTriplesData() []term.Triple {
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	typ := term.NewURIRefUnsafe(RDF + "type")
	size := func(n int) term.Literal { return term.NewLiteral(n) }
	return []term.Triple{
		{Subject: ex("a"), Predicate: typ, Object: ex("Item")},
		{Subject: ex("a"), Predicate: ex("size"), Object: size(3)},
		{Subject: ex("a"), Predicate: ex("part"), Object: ex("b")},
		{Subject: ex("b"), Predicate: typ, Object: ex("Item")},
		{Subject: ex("b"), Predicate: ex("size"), Object: size(3)},
		{Subject: ex("b"), Predicate: ex("part"), Object: ex("x")}, // x is not an Item
		{Subject: ex("c"), Predicate: typ, Object: ex("Item")},
		{Subject: ex("c"), Predicate: ex("size"), Object: size(9)}, // too large
		{Subject: ex("d"), Predicate: typ, Object: ex("Item")},     // no size
		{Subject: ex("a"), Predicate: ex("size"), Object: size(3)}, // repeated: held once
	}
}

// A graph made from in-memory triples reads and validates exactly as the same
// triples added to a graph.Graph and wrapped with NewGraphFromRDF, Core and
// SPARQL constraints alike, and builds no rdflib graph unless SPARQL needs it.
func TestNewGraphFromTriplesValidatesLikeAWrappedGraph(t *testing.T) {
	ts := fromTriplesData()
	rg := graph.NewGraph()
	for _, tr := range ts {
		if err := rg.Add(t.Context(), tr.Subject, tr.Predicate, tr.Object); err != nil {
			t.Fatal(err)
		}
	}
	wrapped := mustV(NewGraphFromRDF(t.Context(), rg, ""))

	for _, shapesTTL := range []string{fromTriplesShapes, fromTriplesSPARQLShapes} {
		sparql := shapesTTL == fromTriplesSPARQLShapes
		shapes, err := LoadTurtleString(shapesTTL, "")
		if err != nil {
			t.Fatal(err)
		}
		compiled, err := CompileShapes(shapes)
		if err != nil {
			t.Fatal(err)
		}
		g := NewGraphFromTriples(ts, "")
		if g.Len() != wrapped.Len() || g.Len() != len(ts)-1 {
			t.Fatalf("Len %d, want %d", g.Len(), wrapped.Len())
		}
		if got, want := tripleKeys(g.Triples()), tripleKeys(wrapped.Triples()); !slices.Equal(got, want) {
			t.Errorf("Triples differ:\n got %v\nwant %v", got, want)
		}
		want := reportKeys(Validate(wrapped, shapes))
		// b's part is not an Item, c is too large, d has no size; with SPARQL
		// also a and b, which share a size.
		if n := map[bool]int{false: 3, true: 5}[sparql]; len(want) != n {
			t.Fatalf("sparql=%v: reference report has %d results, want %d: %v", sparql, len(want), n, want)
		}
		if got := reportKeys(Validate(g, shapes)); !slices.Equal(got, want) {
			t.Errorf("sparql=%v: Validate differs:\n got %v\nwant %v", sparql, got, want)
		}
		if got := reportKeys(compiled.Validate(g)); !slices.Equal(got, want) {
			t.Errorf("sparql=%v: CompiledShapes.Validate differs:\n got %v\nwant %v", sparql, got, want)
		}
		if built := g.rg.Load() != nil; built != sparql {
			t.Errorf("sparql=%v: rdflib graph built = %v", sparql, built)
		}
	}
}
