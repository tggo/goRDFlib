package shacl

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

func benchEx(s string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + s) }

// benchVocabulary is a shared graph of about n triples: 100 kinds under
// ex:Kind, and instances of them with a label and a code.
func benchVocabulary(b *testing.B, n int) *graph.Graph {
	b.Helper()
	g := graph.NewGraph()
	add := func(s term.Subject, p term.URIRef, o term.Term) {
		if err := g.Add(b.Context(), s, p, o); err != nil {
			b.Fatal(err)
		}
	}
	typ, sub := term.NewURIRefUnsafe(RDF+"type"), term.NewURIRefUnsafe(RDFS+"subClassOf")
	for c := range 100 {
		add(benchEx(fmt.Sprintf("Kind%d", c)), sub, benchEx("Kind"))
	}
	for i := range (n - 100) / 3 {
		k := benchEx(fmt.Sprintf("k%d", i))
		add(k, typ, benchEx(fmt.Sprintf("Kind%d", i%100)))
		add(k, benchEx("label"), term.NewLiteral(fmt.Sprintf("kind %d", i)))
		add(k, benchEx("code"), term.NewLiteral(i))
	}
	return g
}

// benchDocument is a data graph of items, six triples each, that refer to
// kinds of the vocabulary.
func benchDocument(items, kinds int) []term.Triple {
	typ := term.NewURIRefUnsafe(RDF + "type")
	out := make([]term.Triple, 0, 6*items)
	for j := range items {
		s := benchEx(fmt.Sprintf("doc-item%d", j))
		out = append(out,
			term.Triple{Subject: s, Predicate: typ, Object: benchEx("Item")},
			term.Triple{Subject: s, Predicate: benchEx("kind"), Object: benchEx(fmt.Sprintf("k%d", (j*7919)%kinds))},
			term.Triple{Subject: s, Predicate: benchEx("size"), Object: term.NewLiteral(j % 5)},
			term.Triple{Subject: s, Predicate: benchEx("name"), Object: term.NewLiteral(fmt.Sprintf("item %d", j))},
			term.Triple{Subject: s, Predicate: benchEx("part"), Object: benchEx(fmt.Sprintf("doc-item%d", (j+1)%items))},
			term.Triple{Subject: s, Predicate: benchEx("count"), Object: term.NewLiteral(j)},
		)
	}
	return out
}

const benchOverlayShapes = `@prefix sh: <http://www.w3.org/ns/shacl#> . @prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
ex:ItemShape a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:property [ sh:path ex:kind ; sh:minCount 1 ; sh:maxCount 1 ; sh:class ex:Kind ] ;
  sh:property [ sh:path ( ex:kind ex:label ) ; sh:minCount 1 ; sh:datatype xsd:string ] ;
  sh:property [ sh:path ex:size ; sh:datatype xsd:integer ; sh:maxInclusive 10 ] ;
  sh:property [ sh:path ex:name ; sh:minCount 1 ] ;
  sh:property [ sh:path ex:part ; sh:class ex:Item ] .`

const benchOverlaySPARQLShapes = benchOverlayShapes + `
ex:NoSharedName a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:sparql [ sh:select """PREFIX ex: <http://example.org/>
    SELECT $this WHERE { $this ex:name ?n . ?o ex:name ?n FILTER (?o != $this) }""" ] .`

// benchOverlaySharedShapes adds SPARQL constraints that join each item with
// the shared graph, where the patterns match many triples: the planner orders
// them once per focus node, from the store's counts when it has them,
// otherwise by probing each pattern.
const benchOverlaySharedShapes = benchOverlayShapes + `
ex:UniqueKindLabel a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:sparql [ sh:select """PREFIX ex: <http://example.org/>
    SELECT $this WHERE { ?other ex:label ?l . ?k ex:label ?l . $this ex:kind ?k FILTER (?other != ?k) }""" ] .
ex:KindUnderKind a sh:NodeShape ; sh:targetClass ex:Item ;
  sh:sparql [ sh:select """PREFIX ex: <http://example.org/>
    PREFIX rdfs: <http://www.w3.org/2000/01/rdf-schema#>
    SELECT $this WHERE { $this ex:kind ?k FILTER NOT EXISTS { ?k a/rdfs:subClassOf* ex:Kind } }""" ] .`
