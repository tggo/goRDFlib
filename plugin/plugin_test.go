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
