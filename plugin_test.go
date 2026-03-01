package rdflibgo

import "testing"

// Ported from: rdflib.plugin

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
		f, ok := FormatFromFilename(tt.filename)
		if !ok || f != tt.format {
			t.Errorf("FormatFromFilename(%q) = %q, %v; want %q", tt.filename, f, ok, tt.format)
		}
	}
}

func TestFormatFromFilenameUnknown(t *testing.T) {
	_, ok := FormatFromFilename("data.xyz")
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
		f, ok := FormatFromMIME(tt.mime)
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
		f, ok := FormatFromContent([]byte(tt.content))
		if !ok || f != tt.format {
			t.Errorf("FormatFromContent(%q...) = %q, %v; want %q", tt.content[:20], f, ok, tt.format)
		}
	}
}
