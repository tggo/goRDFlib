package rdflibgo

import (
	"bytes"
	"strings"
	"testing"
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
	g := NewGraph()
	if err := g.Parse(strings.NewReader(input), WithFormat("json-ld")); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", g.Len())
	}
	alice, _ := NewURIRef("http://example.org/Alice")
	name, _ := NewURIRef("http://example.org/name")
	if !g.Contains(alice, name, NewLiteral("Alice")) {
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
	g := NewGraph()
	if err := g.Parse(strings.NewReader(input), WithFormat("json-ld")); err != nil {
		t.Fatal(err)
	}
	alice, _ := NewURIRef("http://example.org/Alice")
	person, _ := NewURIRef("http://example.org/Person")
	if !g.Contains(alice, RDF.Type, person) {
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
	g := NewGraph()
	if err := g.Parse(strings.NewReader(input), WithFormat("json-ld")); err != nil {
		t.Fatal(err)
	}
	s, _ := NewURIRef("http://example.org/s")
	label, _ := NewURIRef("http://example.org/label")
	val, ok := g.Value(s, &label, nil)
	if !ok {
		t.Fatal("expected value")
	}
	lit, ok := val.(Literal)
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
	g := NewGraph()
	if err := g.Parse(strings.NewReader(input), WithFormat("json-ld")); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", g.Len())
	}
}

func TestJSONLDSerializerBasic(t *testing.T) {
	// Ported from: rdflib.plugins.serializers.jsonld
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/Alice")
	name, _ := NewURIRef("http://example.org/name")
	g.Add(s, name, NewLiteral("Alice"))

	var buf bytes.Buffer
	if err := g.Serialize(&buf, WithSerializeFormat("json-ld")); err != nil {
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
	g1 := NewGraph()
	if err := g1.Parse(strings.NewReader(input), WithFormat("json-ld")); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := g1.Serialize(&buf, WithSerializeFormat("json-ld")); err != nil {
		t.Fatal(err)
	}

	g2 := NewGraph()
	if err := g2.Parse(strings.NewReader(buf.String()), WithFormat("json-ld")); err != nil {
		t.Fatalf("roundtrip parse failed: %v\nSerialized:\n%s", err, buf.String())
	}

	if g1.Len() != g2.Len() {
		t.Errorf("roundtrip: %d vs %d\nSerialized:\n%s", g1.Len(), g2.Len(), buf.String())
	}
}
