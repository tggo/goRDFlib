package jsonld

import (
	"errors"
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
