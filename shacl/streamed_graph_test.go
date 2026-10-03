package shacl

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/term"
)

// Issue #48: LoadJsonLD keeps the parsed triples and indexes them directly,
// building the rdflib graph only when something needs it. Every check here
// compares the streamed graph with one wrapped around a graph.Graph that
// jsonld.Parse filled, which is what LoadJsonLD returned before.

var streamedDocs = []string{
	`{"@context":{"ex":"http://example.org/"},"@id":"ex:a","@type":"ex:Person","ex:name":"Ann","ex:age":30,"ex:knows":{"@id":"ex:b"}}`,
	`[{"@id":"http://example.org/b","@type":"http://example.org/Person","http://example.org/name":["Bob","Robert"],"http://example.org/age":{"@value":"x","@type":"http://www.w3.org/2001/XMLSchema#integer"}},
	  {"@id":"http://example.org/c","@type":"http://example.org/Person","http://example.org/knows":{"@id":"_:n"}},
	  {"@id":"_:n","http://example.org/name":{"@value":"hallo","@language":"de"}}]`,
	// A repeated statement: a graph holds it once.
	`{"@id":"http://example.org/d","@type":"http://example.org/Person","http://example.org/name":["Dup","Dup"],"http://example.org/extra":1}`,
	`{"@context":{"@vocab":"https://schema.org/"},"@id":"https://e.org/p","@type":"Place","name":"P","geo":{"@type":"GeoCoordinates","latitude":1.5},"hasPart":[{"@type":"Dataset","name":"d1"},{"@type":"Dataset","name":"d2"}]}`,
	`{}`,
}

const streamedShapes = `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix ex: <http://example.org/> .
@prefix schema: <https://schema.org/> .
ex:PersonShape a sh:NodeShape ;
	sh:targetClass ex:Person ;
	sh:closed true ; sh:ignoredProperties ( <http://www.w3.org/1999/02/22-rdf-syntax-ns#type> ) ;
	sh:property [ sh:path ex:name ; sh:minCount 1 ; sh:maxCount 1 ; sh:datatype xsd:string ] ;
	sh:property [ sh:path ex:age ; sh:datatype xsd:integer ; sh:minInclusive 18 ] ;
	sh:property [ sh:path ex:knows ; sh:node ex:Named ] .
ex:Named sh:property [ sh:path ex:name ; sh:minCount 1 ] .
ex:PlaceShape a sh:NodeShape ;
	sh:targetClass schema:Place ;
	sh:property [ sh:path schema:hasPart ; sh:qualifiedValueShape [ sh:class schema:Dataset ] ; sh:qualifiedMinCount 3 ] ;
	sh:property [ sh:path [ sh:inversePath schema:hasPart ] ; sh:maxCount 0 ] .
`

// A SPARQL constraint queries the rdflib graph, so it forces it to be built.
const streamedSPARQLShapes = streamedShapes + `
ex:SPARQLShape a sh:NodeShape ;
	sh:targetClass ex:Person ;
	sh:sparql [ sh:select """
		PREFIX ex: <http://example.org/>
		SELECT $this ?value WHERE { $this ex:name ?value . FILTER(STRLEN(?value) < 4) }""" ] .
`

func loadBoth(t *testing.T, doc string) (streamed, eager *Graph) {
	t.Helper()
	opts := []jsonld.Option{jsonld.WithPreserveBlankNodeIDs()}
	streamed, err := LoadJsonLDString(doc, "http://example.org/base/", opts...)
	if err != nil {
		t.Fatal(err)
	}
	g := graph.NewGraph(graph.WithBase("http://example.org/base/"))
	if err := jsonld.Parse(g, strings.NewReader(doc), append(opts, jsonld.WithBase("http://example.org/base/"))...); err != nil {
		t.Fatal(err)
	}
	return streamed, NewGraphFromRDF(g, "http://example.org/base/")
}

func tripleKeys(ts []Triple) []string {
	keys := make([]string, len(ts))
	for i, tr := range ts {
		keys[i] = tr.Subject.TermKey() + " " + tr.Predicate.TermKey() + " " + tr.Object.TermKey()
	}
	slices.Sort(keys)
	return keys
}

func reportKeys(r ValidationReport) []string {
	var keys []string
	var walk func([]ValidationResult, string)
	walk = func(rs []ValidationResult, indent string) {
		for _, x := range rs {
			keys = append(keys, fmt.Sprintf("%s%s %s %s %s %s", indent, x.FocusNode, x.ResultPath, x.Value, x.SourceConstraintComponent, x.SourceShape))
			walk(x.Details, indent+"  ")
		}
	}
	walk(r.Results, "")
	slices.Sort(keys)
	return keys
}

func TestStreamedGraphMatchesParsedGraph(t *testing.T) {
	for i, doc := range streamedDocs {
		streamed, eager := loadBoth(t, doc)
		want := eager.Triples()
		if got := streamed.Triples(); !slices.Equal(tripleKeys(got), tripleKeys(want)) {
			t.Errorf("doc %d: Triples differ:\n got %v\nwant %v", i, tripleKeys(got), tripleKeys(want))
		}
		if streamed.Len() != eager.Len() {
			t.Errorf("doc %d: Len = %d, want %d", i, streamed.Len(), eager.Len())
		}
		// Every pattern All answers, built from the triples themselves.
		for _, tr := range want {
			s, p, o := tr.Subject, tr.Predicate, tr.Object
			for _, pat := range [][3]*Term{
				{&s, nil, nil}, {&s, &p, nil}, {&s, &p, &o}, {&s, nil, &o},
				{nil, &p, nil}, {nil, &p, &o}, {nil, nil, &o}, {nil, nil, nil},
			} {
				got, exp := streamed.All(pat[0], pat[1], pat[2]), eager.All(pat[0], pat[1], pat[2])
				if !slices.Equal(tripleKeys(got), tripleKeys(exp)) {
					t.Errorf("doc %d: All%v = %v, want %v", i, pat, tripleKeys(got), tripleKeys(exp))
				}
			}
		}
		if streamed.rg.Load() != nil {
			t.Errorf("doc %d: reading built the rdflib graph", i)
		}
	}
}

func TestStreamedGraphValidatesTheSame(t *testing.T) {
	for _, shapesTTL := range []string{streamedShapes, streamedSPARQLShapes} {
		shapes, err := LoadTurtleString(shapesTTL, "")
		if err != nil {
			t.Fatal(err)
		}
		compiled, err := CompileShapes(shapes)
		if err != nil {
			t.Fatal(err)
		}
		sparql := shapesTTL == streamedSPARQLShapes
		for i, doc := range streamedDocs {
			streamed, eager := loadBoth(t, doc)
			want := reportKeys(Validate(eager, shapes))
			if got := reportKeys(Validate(streamed, shapes)); !slices.Equal(got, want) {
				t.Errorf("doc %d (sparql=%v): Validate differs:\n got %v\nwant %v", i, sparql, got, want)
			}
			if got := reportKeys(compiled.Validate(streamed)); !slices.Equal(got, want) {
				t.Errorf("doc %d (sparql=%v): CompiledShapes.Validate differs", i, sparql)
			}
			built := streamed.rg.Load() != nil
			if !sparql && built {
				t.Errorf("doc %d: validation without SPARQL built the rdflib graph", i)
			}
		}
	}
}

// Advanced features infer into a copy of the data graph (prepareAdvanced);
// the streamed graph must serve as the source of that copy unchanged.
func TestStreamedGraphWithRules(t *testing.T) {
	shapes, err := LoadTurtleString(`
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
ex:S a sh:NodeShape ; sh:targetClass ex:Person ;
	sh:rule [ a sh:TripleRule ; sh:subject sh:this ; sh:predicate ex:tagged ; sh:object ex:yes ] ;
	sh:property [ sh:path ex:tagged ; sh:minCount 1 ] .
`, "")
	if err != nil {
		t.Fatal(err)
	}
	streamed, eager := loadBoth(t, streamedDocs[1])
	before := streamed.Len()
	got := reportKeys(Validate(streamed, shapes, WithAdvancedFeatures()))
	want := reportKeys(Validate(eager, shapes, WithAdvancedFeatures()))
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if streamed.Len() != before {
		t.Errorf("Validate changed the caller's graph: %d -> %d triples", before, streamed.Len())
	}
}

func TestStreamedGraphAddAndMerge(t *testing.T) {
	streamed, _ := loadBoth(t, streamedDocs[0])
	n := streamed.Len()
	streamed.Add(IRI("http://example.org/a"), IRI("http://example.org/new"), Literal("v", "", ""))
	if streamed.Len() != n+1 || !streamed.Has(ptr(IRI("http://example.org/a")), ptr(IRI("http://example.org/new")), nil) {
		t.Errorf("Add on a streamed graph lost or hid the triple: Len %d -> %d", n, streamed.Len())
	}
	other, _ := loadBoth(t, streamedDocs[2])
	merged := NewGraph()
	merged.Merge(other)
	if merged.Len() != other.Len() {
		t.Errorf("Merge from a streamed graph: %d triples, want %d", merged.Len(), other.Len())
	}
}

func ptr(t Term) *Term { return &t }

// The rdflib graph is built at most once, also when goroutines race to it.
func TestStreamedGraphConcurrentBuild(t *testing.T) {
	streamed, eager := loadBoth(t, streamedDocs[1])
	shapes, _ := LoadTurtleString(streamedSPARQLShapes, "")
	compiled, _ := CompileShapes(shapes)
	want := reportKeys(compiled.Validate(eager))
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if got := reportKeys(compiled.Validate(streamed)); !slices.Equal(got, want) {
				t.Errorf("concurrent Validate differs")
			}
		})
	}
	wg.Wait()
}

// A source that repeats a triple must look like a graph that holds it once.
// json-gold already merges the repeats a document can spell, so this builds
// the source directly.
func TestStreamedGraphDeduplicates(t *testing.T) {
	s, p := term.NewURIRefUnsafe("http://example.org/s"), term.NewURIRefUnsafe("http://example.org/p")
	o := term.NewLiteral("v")
	tr := term.Triple{Subject: s, Predicate: p, Object: o}
	g := &Graph{src: []term.Triple{tr, tr, {Subject: s, Predicate: p, Object: term.NewLiteral("w")}}}
	if g.Len() != 2 || len(g.Triples()) != 2 || len(g.All(nil, ptr(fromRDFLib(p)), nil)) != 2 {
		t.Errorf("Len %d, Triples %d; want 2 distinct triples", g.Len(), len(g.Triples()))
	}
	if got := g.Objects(fromRDFLib(s), fromRDFLib(p)); len(got) != 2 {
		t.Errorf("Objects = %v, want 2", got)
	}
	if g.rdf().Len() != 2 {
		t.Errorf("built rdflib graph has %d triples, want 2", g.rdf().Len())
	}
}
