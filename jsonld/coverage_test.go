package jsonld

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/piprate/json-gold/ld"

	rdflibgo "github.com/tggo/goRDFlib"
)

// failingDocumentLoader is a DocumentLoader that always returns an error,
// used to trigger the proc.ToRDF error path without making network calls.
type failingDocumentLoader struct{}

func (failingDocumentLoader) LoadDocument(u string) (*ld.RemoteDocument, error) {
	return nil, errors.New("document load failed: " + u)
}

// errWriter is an io.Writer that always returns an error.
type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, errors.New("write error") }

// TestWithBase covers the WithBase option (options.go:25 - 0.0%)
func TestWithBase(t *testing.T) {
	// WithBase sets base IRI used during JSON-LD processing
	input := `{
		"@id": "http://example.org/doc",
		"http://example.org/p": "v"
	}`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input), WithBase("http://example.org/"))
	if err != nil {
		t.Fatal(err)
	}
}

// TestWithBaseInSerializer covers WithBase in Serialize path
func TestWithBaseInSerializer(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	var buf strings.Builder
	err := Serialize(g, &buf, WithBase("http://example.org/"))
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

// TestParseInvalidJSON covers the json decode error branch in Parse
func TestParseInvalidJSON(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`not valid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// TestParseEmptyDocument covers the empty nquads string branch (nqStr == "")
func TestParseEmptyDocument(t *testing.T) {
	// An empty JSON-LD object with no triples produces empty N-Quads
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 0 {
		t.Errorf("expected 0 triples, got %d", g.Len())
	}
}

// TestParseNullJSON covers the nil/non-string nquads branch
// json-gold may return nil for a JSON null input
func TestParseNullJSON(t *testing.T) {
	g := rdflibgo.NewGraph()
	// A JSON null value — json.Decode succeeds, ToRDF result handling exercised
	err := Parse(g, strings.NewReader(`null`))
	// Either nil error (empty result) or an error from json-gold; just don't panic
	_ = err
}

// TestSerializeWithDocumentLoader covers cfg.documentLoader != nil in Serialize
func TestSerializeWithDocumentLoader(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	var buf strings.Builder
	loader := ld.NewCachingDocumentLoader(ld.NewDefaultDocumentLoader(nil))
	err := Serialize(g, &buf, WithDocumentLoader(loader))
	if err != nil {
		t.Fatal(err)
	}
}

// TestParseWithDocumentLoaderNonNil covers cfg.documentLoader != nil in Parse
func TestParseWithDocumentLoaderNonNil(t *testing.T) {
	input := `{"@id": "http://example.org/s", "http://example.org/p": "v"}`
	g := rdflibgo.NewGraph()
	loader := ld.NewCachingDocumentLoader(ld.NewDefaultDocumentLoader(nil))
	err := Parse(g, strings.NewReader(input), WithDocumentLoader(loader))
	if err != nil {
		t.Fatal(err)
	}
}

// TestParseToRDFError covers the proc.ToRDF error return path in Parse.
// A remote @context reference with a failing loader triggers the error.
func TestParseToRDFError(t *testing.T) {
	input := `{"@context": "http://example.org/ctx.jsonld", "@id": "http://example.org/s"}`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input), WithDocumentLoader(failingDocumentLoader{}))
	if err == nil {
		t.Error("expected error when document loader fails")
	}
}

// TestSerializeWriterError covers the enc.Encode error path in Serialize.
func TestSerializeWriterError(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	err := Serialize(g, errWriter{})
	if err == nil {
		t.Error("expected error from broken writer")
	}
}

// TestSerializeWriterErrorWithNamespace covers enc.Encode error when compaction
// path is taken (graph has namespace bindings → len(context) > 0).
func TestSerializeWriterErrorWithNamespace(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	err := Serialize(g, errWriter{})
	if err == nil {
		t.Error("expected error from broken writer")
	}
}

// TestSerializeCompactError covers the proc.Compact error path in Serialize.
// When the namespace map contains "@context" as a key, json-gold interprets its
// value as a remote context URL and tries to load it. Using a failing document
// loader causes Compact to return an error.
func TestSerializeCompactError(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	// Bind "@context" as a prefix — json-gold treats its IRI value as a remote context URL.
	g.Bind("@context", rdflibgo.NewURIRefUnsafe("http://example.org/remote-ctx.jsonld"))
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	var buf strings.Builder
	err := Serialize(g, &buf, WithDocumentLoader(failingDocumentLoader{}))
	if err == nil {
		t.Error("expected Compact error when remote context loading fails")
	}
}

// --- Additional coverage tests ---

// TestWithFormOption covers the WithForm option.
func TestWithFormOption(t *testing.T) {
	var cfg config
	WithForm(FormExpanded)(&cfg)
	if cfg.form != FormExpanded {
		t.Errorf("expected FormExpanded, got %d", cfg.form)
	}
}

// TestSerializeExpandedForm covers the expanded form output path.
func TestSerializeExpandedForm(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))

	var buf strings.Builder
	err := Serialize(g, &buf, WithExpanded())
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

// TestSerializeNoNamespaces covers the path where no namespace bindings exist (no compaction).
func TestSerializeNoNamespaces(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	var buf strings.Builder
	err := Serialize(g, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

// TestSerializeEmptyGraph covers serializing an empty graph.
func TestSerializeEmptyGraph(t *testing.T) {
	g := rdflibgo.NewGraph()
	var buf strings.Builder
	err := Serialize(g, &buf)
	if err != nil {
		t.Fatal(err)
	}
}

// TestParseWithRemoteContext covers the Parse path where a context is in the document.
func TestParseWithRemoteContext(t *testing.T) {
	// This uses a built-in context that json-gold can handle inline
	input := `{
		"@context": {"ex": "http://example.org/"},
		"@id": "ex:s",
		"ex:p": "hello"
	}`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParseArrayDocument covers parsing a JSON-LD array document.
func TestParseArrayDocument(t *testing.T) {
	input := `[{
		"@id": "http://example.org/s",
		"http://example.org/p": [{"@value": "v"}]
	}]`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestSerializeRoundTrip covers a round-trip: serialize then parse.
func TestSerializeRoundTrip(t *testing.T) {
	g1 := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g1.Add(s, p, rdflibgo.NewLiteral("hello"))
	g1.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))

	var buf strings.Builder
	if err := Serialize(g1, &buf); err != nil {
		t.Fatal(err)
	}

	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(buf.String())); err != nil {
		t.Fatal(err)
	}
	if g2.Len() != 1 {
		t.Errorf("expected 1 triple after round-trip, got %d", g2.Len())
	}
}

// TestWithExpandedOption covers the WithExpanded convenience option.
func TestWithExpandedOption(t *testing.T) {
	var cfg config
	WithExpanded()(&cfg)
	if cfg.form != FormExpanded {
		t.Error("expected FormExpanded")
	}
}

// TestParseMultipleTriples covers parsing a document with multiple triples.
func TestParseMultipleTriples(t *testing.T) {
	input := `{
		"@id": "http://example.org/s",
		"http://example.org/p1": "v1",
		"http://example.org/p2": "v2"
	}`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", g.Len())
	}
}

// TestSerializeWithDocumentLoaderInParse covers the document loader in Parse.
func TestSerializeWithDocumentLoaderInParse(t *testing.T) {
	input := `{
		"@context": {"ex": "http://example.org/"},
		"@id": "ex:s",
		"ex:p": "v"
	}`
	g := rdflibgo.NewGraph()
	loader := ld.NewCachingDocumentLoader(ld.NewDefaultDocumentLoader(nil))
	err := Parse(g, strings.NewReader(input), WithDocumentLoader(loader))
	if err != nil {
		t.Fatal(err)
	}
}

// ─── Additional coverage tests (batch 2) ────────────────────────────────────

// TestParseTypedLiteral covers parsing a document with a typed literal value.
func TestParseTypedLiteral(t *testing.T) {
	input := `{
		"@id": "http://example.org/s",
		"http://example.org/age": {"@value": "42", "@type": "http://www.w3.org/2001/XMLSchema#integer"}
	}`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParseLangLiteral covers parsing a document with a language-tagged literal.
func TestParseLangLiteral(t *testing.T) {
	input := `{
		"@id": "http://example.org/s",
		"http://example.org/label": {"@value": "Hello", "@language": "en"}
	}`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParseLinkedNodes covers parsing a document with linked node references.
func TestParseLinkedNodes(t *testing.T) {
	input := `{
		"@id": "http://example.org/s",
		"http://example.org/knows": {"@id": "http://example.org/o"}
	}`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParseBlankNodes covers parsing a document with blank nodes.
func TestParseBlankNodes(t *testing.T) {
	input := `{
		"@id": "http://example.org/s",
		"http://example.org/knows": {
			"http://example.org/name": "Bob"
		}
	}`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", g.Len())
	}
}

// TestSerializeMultipleTriples covers serializing a graph with multiple triples.
func TestSerializeMultipleTriples(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p1"), rdflibgo.NewLiteral("v1"))
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p2"), rdflibgo.NewLiteral("v2"))
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))

	var buf strings.Builder
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

// TestSerializeURIRefObject covers serializing a graph with URIRef objects.
func TestSerializeURIRefObject(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	o, _ := rdflibgo.NewURIRef("http://example.org/o")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/knows"), o)

	var buf strings.Builder
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "http://example.org/o") {
		t.Errorf("expected object URI in output, got:\n%s", out)
	}
}

// TestSerializeTypedLiteral covers serializing typed literals.
func TestSerializeTypedLiteral(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/age"),
		rdflibgo.NewLiteral("42", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))

	var buf strings.Builder
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "42") {
		t.Errorf("expected '42' in output, got:\n%s", out)
	}
}

// TestSerializeLangLiteral covers serializing language-tagged literals.
func TestSerializeLangLiteral(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/label"),
		rdflibgo.NewLiteral("Hola", rdflibgo.WithLang("es")))

	var buf strings.Builder
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "es") {
		t.Errorf("expected 'es' lang tag in output, got:\n%s", out)
	}
}

// TestParseEmptyArray covers parsing an empty JSON-LD array.
func TestParseEmptyArray(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`[]`))
	if err != nil {
		t.Fatal(err)
	}
	if g.Len() != 0 {
		t.Errorf("expected 0 triples for empty array, got %d", g.Len())
	}
}

// TestParseBoolJSON covers parsing a JSON boolean value.
func TestParseBoolJSON(t *testing.T) {
	g := rdflibgo.NewGraph()
	// A boolean value — should be handled by json-gold
	err := Parse(g, strings.NewReader(`true`))
	// Either nil or an error; just don't panic
	_ = err
}

// TestParseNumberJSON covers parsing a JSON number value.
func TestParseNumberJSON(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`42`))
	_ = err
}

// TestSerializeRoundTripWithTypes covers round-trip with rdf:type.
func TestSerializeRoundTripWithTypes(t *testing.T) {
	g1 := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	g1.Add(s, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe("http://example.org/Person"))
	g1.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/name"), rdflibgo.NewLiteral("Alice"))
	g1.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))

	var buf strings.Builder
	if err := Serialize(g1, &buf); err != nil {
		t.Fatal(err)
	}

	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(buf.String())); err != nil {
		t.Fatal(err)
	}
	if g2.Len() != 2 {
		t.Errorf("expected 2 triples after round-trip, got %d", g2.Len())
	}
}

// TestSerializeExpandedRoundTrip covers round-trip in expanded form.
func TestSerializeExpandedRoundTrip(t *testing.T) {
	g1 := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	g1.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("val"))

	var buf strings.Builder
	if err := Serialize(g1, &buf, WithExpanded()); err != nil {
		t.Fatal(err)
	}

	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(buf.String())); err != nil {
		t.Fatal(err)
	}
	if g2.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g2.Len())
	}
}

// TestParseNestedContext covers parsing with nested @context.
func TestParseNestedContext(t *testing.T) {
	input := `{
		"@context": {
			"ex": "http://example.org/",
			"name": "ex:name"
		},
		"@id": "ex:alice",
		"name": "Alice"
	}`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestSerializeMultipleSubjects covers serializing multiple subjects.
func TestSerializeMultipleSubjects(t *testing.T) {
	g := rdflibgo.NewGraph()
	for i := 0; i < 3; i++ {
		s, _ := rdflibgo.NewURIRef(fmt.Sprintf("http://example.org/s%d", i))
		g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral(fmt.Sprintf("v%d", i)))
	}
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))

	var buf strings.Builder
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

// TestSerializeWithBaseAndNamespaces covers both base and namespace compaction.
func TestSerializeWithBaseAndNamespaces(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral("v"))
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))

	var buf strings.Builder
	err := Serialize(g, &buf, WithBase("http://example.org/"), WithDocumentLoader(ld.NewCachingDocumentLoader(ld.NewDefaultDocumentLoader(nil))))
	if err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

// TestParseGraphWithList covers parsing a document with a JSON-LD list.
func TestParseGraphWithList(t *testing.T) {
	input := `{
		"@id": "http://example.org/s",
		"http://example.org/p": {"@list": ["a", "b", "c"]}
	}`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() < 1 {
		t.Errorf("expected at least 1 triple, got %d", g.Len())
	}
}

// TestSerializeFromRDFError covers the proc.FromRDF error path by serializing a
// graph with RDF 1.2 triple terms that json-gold can't parse as N-Quads.
func TestSerializeFromRDFError(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	tt := rdflibgo.NewTripleTerm(
		rdflibgo.NewURIRefUnsafe("http://example.org/a"),
		rdflibgo.NewURIRefUnsafe("http://example.org/b"),
		rdflibgo.NewURIRefUnsafe("http://example.org/c"),
	)
	g.Add(s, rdflibgo.NewURIRefUnsafe("http://example.org/p"), tt)

	var buf strings.Builder
	err := Serialize(g, &buf)
	if err == nil {
		t.Error("expected error from proc.FromRDF for triple terms")
	}
}

// TestSerializeBNodeSubject covers serializing blank node subjects.
func TestSerializeBNodeSubject(t *testing.T) {
	g := rdflibgo.NewGraph()
	bn := rdflibgo.NewBNode("x1")
	g.Add(bn, rdflibgo.NewURIRefUnsafe("http://example.org/name"), rdflibgo.NewLiteral("Bob"))

	var buf strings.Builder
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output for bnode subject")
	}
}
