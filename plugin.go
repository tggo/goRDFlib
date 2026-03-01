package rdflibgo

import (
	"path/filepath"
	"strings"
)

// Store registry (complements parser/serializer registries in graph.go).
// Ported from: rdflib.plugin

var stores = make(map[string]func() Store)

// RegisterStore registers a store factory by name.
func RegisterStore(name string, factory func() Store) {
	stores[name] = factory
}

// GetStore creates a store by registered name.
func GetStore(name string) (Store, bool) {
	f, ok := stores[name]
	if !ok {
		return nil, false
	}
	return f(), true
}

func init() {
	RegisterStore("default", func() Store { return NewMemoryStore() })
	RegisterStore("memory", func() Store { return NewMemoryStore() })
}

// --- MIME type and file extension mappings ---

var mimeToFormat = map[string]string{
	"text/turtle":               "turtle",
	"application/x-turtle":      "turtle",
	"application/n-triples":     "nt",
	"application/n-quads":       "nquads",
	"application/rdf+xml":       "xml",
	"application/ld+json":       "json-ld",
	"text/n3":                   "turtle",
	"text/plain":                "nt",
}

var extToFormat = map[string]string{
	".ttl":    "turtle",
	".turtle": "turtle",
	".nt":     "nt",
	".ntriples": "nt",
	".nq":     "nquads",
	".nquads": "nquads",
	".rdf":    "xml",
	".xml":    "xml",
	".owl":    "xml",
	".jsonld": "json-ld",
	".json":   "json-ld",
	".trig":   "turtle", // simplified — TriG is close to Turtle
}

// FormatFromFilename detects the RDF format from a file path extension.
// Ported from: rdflib.plugin — format detection by extension
func FormatFromFilename(filename string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(filename))
	f, ok := extToFormat[ext]
	return f, ok
}

// FormatFromMIME detects the RDF format from a MIME content-type.
// Ported from: rdflib.plugin — format detection by MIME type
func FormatFromMIME(contentType string) (string, bool) {
	// Strip parameters (e.g., "text/turtle; charset=utf-8")
	ct := strings.TrimSpace(contentType)
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	f, ok := mimeToFormat[strings.ToLower(ct)]
	return f, ok
}

// FormatFromContent detects the RDF format by sniffing the first bytes.
// Ported from: rdflib.plugin — content-based detection
func FormatFromContent(data []byte) (string, bool) {
	s := strings.TrimSpace(string(data[:min(len(data), 500)]))
	if strings.HasPrefix(s, "<?xml") || strings.HasPrefix(s, "<rdf:RDF") {
		return "xml", true
	}
	if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
		return "json-ld", true
	}
	if strings.HasPrefix(s, "@prefix") || strings.HasPrefix(s, "@base") || strings.HasPrefix(s, "PREFIX") || strings.HasPrefix(s, "BASE") {
		return "turtle", true
	}
	// N-Triples: lines starting with < or _:
	if strings.HasPrefix(s, "<") || strings.HasPrefix(s, "_:") {
		// Could be NT or NQ — check for 4th element
		firstLine := s
		if i := strings.Index(s, "\n"); i >= 0 {
			firstLine = s[:i]
		}
		parts := strings.Fields(firstLine)
		if len(parts) >= 5 && parts[len(parts)-1] == "." {
			return "nquads", true
		}
		return "nt", true
	}
	return "", false
}

// ListParsers returns all registered parser format names.
func ListParsers() []string {
	var names []string
	for name := range parsers {
		names = append(names, name)
	}
	return names
}

// ListSerializers returns all registered serializer format names.
func ListSerializers() []string {
	var names []string
	for name := range serializers {
		names = append(names, name)
	}
	return names
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
