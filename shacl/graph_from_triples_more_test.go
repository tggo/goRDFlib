package shacl

import (
	"slices"
	"sync"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// rdf:reifies with a triple-term object feeds the reifier indexes that the
// SHACL 1.2 lookups read. A repeated rdf:reifies triple lists its reifier once,
// as in a graph.Graph.
func TestNewGraphFromTriplesReifierIndex(t *testing.T) {
	ex := func(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }
	reifies := term.NewURIRefUnsafe(RDF + "reifies")
	r := term.NewBNode()
	tt := term.NewTripleTerm(ex("a"), ex("size"), term.NewLiteral(3))
	ts := []term.Triple{
		{Subject: ex("a"), Predicate: ex("size"), Object: term.NewLiteral(3)},
		{Subject: r, Predicate: reifies, Object: tt},
		{Subject: r, Predicate: ex("note"), Object: term.NewLiteral("n")},
		{Subject: r, Predicate: reifies, Object: tt}, // repeated
	}
	rg := graph.NewGraph()
	for _, tr := range ts {
		if err := rg.Add(t.Context(), tr.Subject, tr.Predicate, tr.Object); err != nil {
			t.Fatal(err)
		}
	}
	wrapped := mustV(NewGraphFromRDF(t.Context(), rg, ""))
	g := NewGraphFromTriples(ts, "")

	a, size, three := fromRDFLib(ex("a")), fromRDFLib(ex("size")), fromRDFLib(term.NewLiteral(3))
	got := findReifiers(g, a, size, three)
	if len(got) != 1 || !got[0].Equal(fromRDFLib(r)) {
		t.Fatalf("findReifiers = %v, want exactly the one reifier", got)
	}
	if want := findReifiers(wrapped, a, size, three); len(want) != len(got) {
		t.Errorf("wrapped graph finds %d reifiers, streamed %d", len(want), len(got))
	}
	if n := len(g.ensureIndexes().reifiesBy[indexKey(fromRDFLib(r))]); n != 1 {
		t.Errorf("reifiesBy holds %d triple terms for the reifier, want 1", n)
	}
	if ann := extractAnnotatedTriple(g, fromRDFLib(r), a); ann == nil || !ann.object.Equal(three) {
		t.Errorf("extractAnnotatedTriple = %+v", ann)
	}
	if g.Len() != wrapped.Len() || g.Len() != 3 {
		t.Errorf("Len = %d, wrapped %d, want 3", g.Len(), wrapped.Len())
	}
	if g.rg.Load() != nil {
		t.Error("reifier lookups built the rdflib graph")
	}
}

// With a SPARQL constraint the rdflib graph is built exactly once, however many
// goroutines validate at the same time.
func TestNewGraphFromTriplesBuildsRDFOnceConcurrently(t *testing.T) {
	shapes := mustV(LoadTurtleString(fromTriplesSPARQLShapes, ""))
	compiled := mustV(CompileShapes(shapes))
	g := NewGraphFromTriples(fromTriplesData(), "")
	want := reportKeys(Validate(NewGraphFromTriples(fromTriplesData(), ""), shapes))

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if got := reportKeys(compiled.Validate(g)); !slices.Equal(got, want) {
				t.Errorf("report differs:\n got %v\nwant %v", got, want)
			}
		})
	}
	wg.Wait()
	first := g.rg.Load()
	if first == nil {
		t.Fatal("SPARQL constraint did not build the rdflib graph")
	}
	if got, err := g.rdf(t.Context()); err != nil || got != first {
		t.Errorf("rdf() rebuilt the graph (err %v)", err)
	}
	if n, err := first.Len(t.Context()); err != nil || n != g.Len() {
		t.Errorf("rdflib graph has %d triples (err %v), indexes %d", n, err, g.Len())
	}
}

// Add and Merge build the rdflib graph on a graph made from triples, and the
// indexes then see the new triples; the caller's slice is untouched.
func TestNewGraphFromTriplesAddAndMerge(t *testing.T) {
	ts := fromTriplesData()
	n := len(ts)
	g := NewGraphFromTriples(ts, "")
	before := g.Len()

	x := IRI("http://example.org/x")
	p := IRI("http://example.org/size")
	if err := g.Add(t.Context(), x, p, fromRDFLib(term.NewLiteral(1))); err != nil {
		t.Fatal(err)
	}
	if g.Len() != before+1 || !g.Has(&x, &p, nil) {
		t.Errorf("after Add: Len %d (was %d), Has = %v", g.Len(), before, g.Has(&x, &p, nil))
	}

	other := NewGraphFromTriples([]term.Triple{
		{Subject: term.NewURIRefUnsafe("http://example.org/y"), Predicate: term.NewURIRefUnsafe("http://example.org/size"), Object: term.NewLiteral(2)},
	}, "")
	if err := g.Merge(t.Context(), other); err != nil {
		t.Fatal(err)
	}
	y := IRI("http://example.org/y")
	if g.Len() != before+2 || !g.Has(&y, &p, nil) {
		t.Errorf("after Merge: Len %d, want %d", g.Len(), before+2)
	}
	if len(ts) != n {
		t.Errorf("caller's slice changed length: %d -> %d", n, len(ts))
	}
}
