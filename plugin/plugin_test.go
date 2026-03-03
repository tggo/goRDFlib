package plugin_test

import (
	"io"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/plugin"
	"github.com/tggo/goRDFlib/store"
)

func TestFormatFromFilename(t *testing.T) {
	tests := []struct {
		filename string
		format   string
	}{
		{"data.ttl", "turtle"},
		{"data.nt", "nt"},
		{"data.nq", "nquads"},
		{"data.rdf", "xml"},
		{"data.owl", "xml"},
		{"data.jsonld", "json-ld"},
		{"data.json", "json-ld"},
	}
	for _, tt := range tests {
		f, ok := plugin.FormatFromFilename(tt.filename)
		if !ok || f != tt.format {
			t.Errorf("FormatFromFilename(%q) = %q, %v; want %q", tt.filename, f, ok, tt.format)
		}
	}
}

func TestFormatFromFilenameUnknown(t *testing.T) {
	_, ok := plugin.FormatFromFilename("data.xyz")
	if ok {
		t.Error("expected false for unknown extension")
	}
}

func TestFormatFromMIME(t *testing.T) {
	tests := []struct {
		mime   string
		format string
	}{
		{"text/turtle", "turtle"},
		{"application/n-triples", "nt"},
		{"application/rdf+xml", "xml"},
		{"application/ld+json", "json-ld"},
		{"text/turtle; charset=utf-8", "turtle"},
	}
	for _, tt := range tests {
		f, ok := plugin.FormatFromMIME(tt.mime)
		if !ok || f != tt.format {
			t.Errorf("FormatFromMIME(%q) = %q, %v; want %q", tt.mime, f, ok, tt.format)
		}
	}
}

func TestFormatFromContent(t *testing.T) {
	tests := []struct {
		content string
		format  string
	}{
		{`<?xml version="1.0"?>`, "xml"},
		{`{"@context": {}}`, "json-ld"},
		{`@prefix ex: <http://example.org/> .`, "turtle"},
		{`<http://example.org/s> <http://example.org/p> "hello" .`, "nt"},
	}
	for _, tt := range tests {
		f, ok := plugin.FormatFromContent([]byte(tt.content))
		if !ok || f != tt.format {
			t.Errorf("FormatFromContent(%q...) = %q, %v; want %q", tt.content[:20], f, ok, tt.format)
		}
	}
}

// --- Plugin Registry tests ---

type mockParser struct{}

func (m *mockParser) Parse(g *graph.Graph, r io.Reader) error { return nil }

type mockSerializer struct{}

func (m *mockSerializer) Serialize(g *graph.Graph, w io.Writer) error { return nil }

func TestRegisterAndGetParser(t *testing.T) {
	plugin.RegisterParser("test-format", func() plugin.Parser { return &mockParser{} })
	p, ok := plugin.GetParser("test-format")
	if !ok || p == nil {
		t.Error("expected registered parser")
	}
	_, ok = plugin.GetParser("nonexistent")
	if ok {
		t.Error("expected false for unregistered parser")
	}
}

func TestRegisterAndGetSerializer(t *testing.T) {
	plugin.RegisterSerializer("test-format", func() plugin.Serializer { return &mockSerializer{} })
	s, ok := plugin.GetSerializer("test-format")
	if !ok || s == nil {
		t.Error("expected registered serializer")
	}
	_, ok = plugin.GetSerializer("nonexistent")
	if ok {
		t.Error("expected false for unregistered serializer")
	}
}

func TestRegisterAndGetStore(t *testing.T) {
	plugin.RegisterStore("memory", func() store.Store { return store.NewMemoryStore() })
	s, ok := plugin.GetStore("memory")
	if !ok || s == nil {
		t.Error("expected registered store")
	}
	_, ok = plugin.GetStore("nonexistent")
	if ok {
		t.Error("expected false for unregistered store")
	}
}

// --- Duplicate registration panic tests ---

func TestRegisterParserDuplicatePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on duplicate parser registration")
		}
	}()
	plugin.RegisterParser("dup-parser-format", func() plugin.Parser { return &mockParser{} })
	plugin.RegisterParser("dup-parser-format", func() plugin.Parser { return &mockParser{} })
}

func TestRegisterSerializerDuplicatePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on duplicate serializer registration")
		}
	}()
	plugin.RegisterSerializer("dup-serializer-format", func() plugin.Serializer { return &mockSerializer{} })
	plugin.RegisterSerializer("dup-serializer-format", func() plugin.Serializer { return &mockSerializer{} })
}

func TestRegisterStoreDuplicatePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic on duplicate store registration")
		}
	}()
	plugin.RegisterStore("dup-store-name", func() store.Store { return store.NewMemoryStore() })
	plugin.RegisterStore("dup-store-name", func() store.Store { return store.NewMemoryStore() })
}

// --- FormatFromContent edge case tests ---

func TestFormatFromContentEmpty(t *testing.T) {
	_, ok := plugin.FormatFromContent([]byte{})
	if ok {
		t.Error("expected false for empty data")
	}
}

func TestFormatFromContentJSONLDArray(t *testing.T) {
	// JSON-LD can start with '[' (array of objects)
	f, ok := plugin.FormatFromContent([]byte(`[{"@context": {}}]`))
	if !ok || f != "json-ld" {
		t.Errorf("FormatFromContent('[...') = %q, %v; want json-ld, true", f, ok)
	}
}

func TestFormatFromContentAtBase(t *testing.T) {
	f, ok := plugin.FormatFromContent([]byte(`@base <http://example.org/> .`))
	if !ok || f != "turtle" {
		t.Errorf("FormatFromContent('@base ...') = %q, %v; want turtle, true", f, ok)
	}
}

func TestFormatFromContentBASEKeyword(t *testing.T) {
	f, ok := plugin.FormatFromContent([]byte(`BASE <http://example.org/>`))
	if !ok || f != "turtle" {
		t.Errorf("FormatFromContent('BASE ...') = %q, %v; want turtle, true", f, ok)
	}
}

func TestFormatFromContentNQuads(t *testing.T) {
	// N-Quads: subject predicate object graph .  (5 fields)
	nq := `<http://example.org/s> <http://example.org/p> "hello" <http://example.org/g> .`
	f, ok := plugin.FormatFromContent([]byte(nq))
	if !ok || f != "nquads" {
		t.Errorf("FormatFromContent(nquads line) = %q, %v; want nquads, true", f, ok)
	}
}

func TestFormatFromContentNQuadsBlankSubject(t *testing.T) {
	// N-Quads with blank node subject
	nq := `_:b0 <http://example.org/p> "val" <http://example.org/g> .`
	f, ok := plugin.FormatFromContent([]byte(nq))
	if !ok || f != "nquads" {
		t.Errorf("FormatFromContent(nquads blank subject) = %q, %v; want nquads, true", f, ok)
	}
}

func TestFormatFromContentUnknown(t *testing.T) {
	_, ok := plugin.FormatFromContent([]byte(`# just a comment with no recognisable prefix`))
	if ok {
		t.Error("expected false for unrecognised content")
	}
}

func TestFormatFromContentBOM(t *testing.T) {
	// UTF-8 BOM followed by XML declaration
	bom := "\xEF\xBB\xBF<?xml version=\"1.0\"?>"
	f, ok := plugin.FormatFromContent([]byte(bom))
	if !ok || f != "xml" {
		t.Errorf("FormatFromContent(BOM+xml) = %q, %v; want xml, true", f, ok)
	}
}
