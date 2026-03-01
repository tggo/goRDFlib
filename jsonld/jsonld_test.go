package jsonld

import (
	"bytes"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Ported from: rdflib.plugins.parsers.jsonld, rdflib.plugins.serializers.jsonld

func TestJSONLDParserBasic(t *testing.T) {
	// Ported from: rdflib JSON-LD parser — basic document with @context
	input := `{
		"@context": {
			"name": "http://example.org/name",
			"knows": { "@id": "http://example.org/knows", "@type": "@id" }
		},
		"@id": "http://example.org/Alice",
		"name": "Alice",
		"knows": "http://example.org/Bob"
	}`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", g.Len())
	}
	alice, _ := rdflibgo.NewURIRef("http://example.org/Alice")
	name, _ := rdflibgo.NewURIRef("http://example.org/name")
	if !g.Contains(alice, name, rdflibgo.NewLiteral("Alice")) {
		t.Error("expected name triple")
	}
}

func TestJSONLDParserTypes(t *testing.T) {
	// Ported from: rdflib JSON-LD parser — @type handling
	input := `{
		"@context": {
			"ex": "http://example.org/"
		},
		"@id": "http://example.org/Alice",
		"@type": "ex:Person"
	}`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	alice, _ := rdflibgo.NewURIRef("http://example.org/Alice")
	person, _ := rdflibgo.NewURIRef("http://example.org/Person")
	if !g.Contains(alice, rdflibgo.RDF.Type, person) {
		t.Error("expected rdf:type triple")
	}
}

func TestJSONLDParserLanguage(t *testing.T) {
	// Ported from: rdflib JSON-LD parser — @language
	input := `{
		"@context": {
			"label": "http://example.org/label"
		},
		"@id": "http://example.org/s",
		"label": { "@value": "hello", "@language": "en" }
	}`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	label, _ := rdflibgo.NewURIRef("http://example.org/label")
	val, ok := g.Value(s, &label, nil)
	if !ok {
		t.Fatal("expected value")
	}
	lit, ok := val.(rdflibgo.Literal)
	if !ok || lit.Language() != "en" {
		t.Errorf("expected lang en, got %v", val)
	}
}

func TestJSONLDParserGraph(t *testing.T) {
	// Ported from: rdflib JSON-LD parser — @graph with multiple nodes
	input := `{
		"@context": { "name": "http://example.org/name" },
		"@graph": [
			{ "@id": "http://example.org/Alice", "name": "Alice" },
			{ "@id": "http://example.org/Bob", "name": "Bob" }
		]
	}`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", g.Len())
	}
}

func TestJSONLDSerializerBasic(t *testing.T) {
	// Ported from: rdflib.plugins.serializers.jsonld
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/Alice")
	name, _ := rdflibgo.NewURIRef("http://example.org/name")
	g.Add(s, name, rdflibgo.NewLiteral("Alice"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "Alice") {
		t.Errorf("expected Alice in output, got:\n%s", out)
	}
	if !strings.Contains(out, "http://example.org/name") {
		t.Errorf("expected predicate URI, got:\n%s", out)
	}
}

func TestJSONLDRoundtrip(t *testing.T) {
	// Ported from: roundtrip test — parse JSON-LD → serialize → parse → compare
	input := `{
		"@context": {
			"name": "http://example.org/name",
			"knows": { "@id": "http://example.org/knows", "@type": "@id" }
		},
		"@id": "http://example.org/Alice",
		"name": "Alice",
		"knows": "http://example.org/Bob"
	}`
	g1 := rdflibgo.NewGraph()
	if err := Parse(g1, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := Serialize(g1, &buf); err != nil {
		t.Fatal(err)
	}

	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(buf.String())); err != nil {
		t.Fatalf("roundtrip parse failed: %v\nSerialized:\n%s", err, buf.String())
	}

	if g1.Len() != g2.Len() {
		t.Errorf("roundtrip: %d vs %d\nSerialized:\n%s", g1.Len(), g2.Len(), buf.String())
	}
}
