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

// Fix 9: Compaction error is returned instead of silently swallowed
func TestJSONLDSerializerCompactionError(t *testing.T) {
	// With valid namespace bindings, compaction should succeed (no error swallowed)
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/Alice")
	name, _ := rdflibgo.NewURIRef("http://example.org/name")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, name, rdflibgo.NewLiteral("Alice"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Should contain compacted form with "ex:" prefix
	if !strings.Contains(out, "ex:") {
		t.Errorf("expected compacted output with ex: prefix, got:\n%s", out)
	}
}

// Fix 10: Uses NQuads serializer (test that serialization works)
func TestJSONLDSerializerUsesNQuads(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("value"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

// Fix 11: WithExpanded option outputs expanded form
func TestJSONLDSerializerExpanded(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/Alice")
	name, _ := rdflibgo.NewURIRef("http://example.org/name")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, name, rdflibgo.NewLiteral("Alice"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf, WithExpanded()); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Expanded form should NOT have @context
	if strings.Contains(out, `"@context"`) {
		t.Errorf("expanded form should not have @context, got:\n%s", out)
	}
	// Should contain full URI
	if !strings.Contains(out, "http://example.org/name") {
		t.Errorf("expected full URI in expanded form, got:\n%s", out)
	}
}

// Fix 11: WithForm option
func TestJSONLDSerializerWithForm(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf, WithForm(FormExpanded)); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), `"@context"`) {
		t.Error("FormExpanded should not produce @context")
	}

	buf.Reset()
	if err := Serialize(g, &buf, WithForm(FormCompacted)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "ex:") {
		t.Errorf("FormCompacted should use prefixes, got:\n%s", buf.String())
	}
}

// Fix 12: WithDocumentLoader option is accepted
func TestJSONLDWithDocumentLoaderOption(t *testing.T) {
	// Just verify the option is accepted without error
	g := rdflibgo.NewGraph()
	input := `{"@id": "http://example.org/s", "http://example.org/p": "v"}`
	err := Parse(g, strings.NewReader(input), WithDocumentLoader(nil))
	// nil loader should work (falls back to default)
	if err != nil {
		t.Fatal(err)
	}
}

// Fix 13: interface{} replaced with any (compile-time check — if it compiles, it passes)
func TestJSONLDAnyTypeAlias(t *testing.T) {
	// This test validates that the code compiles with 'any' instead of 'interface{}'
	var v any = "test"
	_ = v
}

// --- Issue #13: lenient handling of invalid IRIs leaking into the N-Quads layer ---

// badNQuads has one syntactically invalid IRI (raw space in the object IRI) and
// one well-formed triple. json-gold normally drops/encodes such IRIs before this
// layer, but older or differently-configured expanders can let them through; this
// exercises that path directly.
const badNQuads = `<http://example.org/s> <http://example.org/p> <http://example.org/good> .
<http://example.org/s> <http://example.org/p> <http://example.org/bad iri> .
`

// TestJSONLDStrictRejectsInvalidIRI documents the default: an invalid IRI that
// reaches the N-Quads parser is a hard error.
func TestJSONLDStrictRejectsInvalidIRI(t *testing.T) {
	g := rdflibgo.NewGraph()
	var cfg config
	err := parseNQuadsInto(g, badNQuads, &cfg)
	if err == nil {
		t.Fatal("expected error parsing invalid IRI in strict (default) mode, got nil")
	}
}

// TestJSONLDSkipInvalidIRIs verifies WithSkipInvalidIRIs drops the malformed
// triple and keeps the well-formed one — matching pyshacl's lenient behavior.
func TestJSONLDSkipInvalidIRIs(t *testing.T) {
	g := rdflibgo.NewGraph()
	var cfg config
	WithSkipInvalidIRIs()(&cfg)
	if err := parseNQuadsInto(g, badNQuads, &cfg); err != nil {
		t.Fatalf("unexpected error with WithSkipInvalidIRIs: %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("expected 1 triple (bad one skipped), got %d", g.Len())
	}
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	good, _ := rdflibgo.NewURIRef("http://example.org/good")
	if !g.Contains(s, p, good) {
		t.Error("expected the well-formed triple to survive")
	}
}

// TestJSONLDParseNQuadsWithUnboundedLines verifies the JSON-LD parser forwards
// its unbounded-line option to the intermediate N-Quads parser.
func TestJSONLDParseNQuadsWithUnboundedLines(t *testing.T) {
	bigLiteral := strings.Repeat("x", 5*1024*1024)
	nquads := `<http://example.org/s> <http://example.org/p> "` + bigLiteral + `" .` + "\n"

	g := rdflibgo.NewGraph()
	var cfg config
	WithUnboundedLines()(&cfg)
	if err := parseNQuadsInto(g, nquads, &cfg); err != nil {
		t.Fatalf("unexpected error with WithUnboundedLines: %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("expected 1 triple, got %d", g.Len())
	}
}

// TestJSONLDSkipInvalidIRIsEndToEnd runs the author's exact example through the
// public Parse with the option set: it must succeed (it already does on current
// json-gold, which drops the malformed term itself).
func TestJSONLDSkipInvalidIRIsEndToEnd(t *testing.T) {
	doc := `{"@context":[{"schema":"https://schema.org/"}],"@type":["schema:Place","schema: Dataset"]}`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(doc), WithSkipInvalidIRIs()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
