package plugin

import (
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
)

// Parser reads RDF data from a reader into a graph.
type Parser interface {
	Parse(g *graph.Graph, r io.Reader) error
}

// Serializer writes RDF data from a graph to a writer.
type Serializer interface {
	Serialize(g *graph.Graph, w io.Writer) error
}

// --- Plugin Registry ---

var (
	parsersMu sync.RWMutex
	parsers   = make(map[string]func() Parser)

	serializersMu sync.RWMutex
	serializers   = make(map[string]func() Serializer)

	storesMu sync.RWMutex
	stores   = make(map[string]func() store.Store)
)

// RegisterParser registers a parser factory for the given format name.
// Typically called from init() functions in format packages.
func RegisterParser(name string, factory func() Parser) {
	parsersMu.Lock()
	defer parsersMu.Unlock()
	if _, exists := parsers[name]; exists {
		panic("plugin: duplicate parser registration for format " + name)
	}
	parsers[name] = factory
}

// GetParser returns a new Parser for the given format name.
func GetParser(name string) (Parser, bool) {
	parsersMu.RLock()
	defer parsersMu.RUnlock()
	f, ok := parsers[name]
	if !ok {
		return nil, false
	}
	return f(), true
}

// RegisterSerializer registers a serializer factory for the given format name.
func RegisterSerializer(name string, factory func() Serializer) {
	serializersMu.Lock()
	defer serializersMu.Unlock()
	if _, exists := serializers[name]; exists {
		panic("plugin: duplicate serializer registration for format " + name)
	}
	serializers[name] = factory
}

// GetSerializer returns a new Serializer for the given format name.
func GetSerializer(name string) (Serializer, bool) {
	serializersMu.RLock()
	defer serializersMu.RUnlock()
	f, ok := serializers[name]
	if !ok {
		return nil, false
	}
	return f(), true
}

// RegisterStore registers a store factory for the given name.
func RegisterStore(name string, factory func() store.Store) {
	storesMu.Lock()
	defer storesMu.Unlock()
	if _, exists := stores[name]; exists {
		panic("plugin: duplicate store registration for name " + name)
	}
	stores[name] = factory
}

// GetStore returns a new Store for the given name.
func GetStore(name string) (store.Store, bool) {
	storesMu.RLock()
	defer storesMu.RUnlock()
	f, ok := stores[name]
	if !ok {
		return nil, false
	}
	return f(), true
}

// --- MIME type and file extension mappings ---

var mimeToFormat = map[string]string{
	"text/turtle":           "turtle",
	"application/x-turtle":  "turtle",
	"application/n-triples": "nt",
	"application/n-quads":   "nquads",
	"application/trig":      "trig",
	"application/rdf+xml":   "xml",
	"application/ld+json":   "json-ld",
	"text/n3":               "turtle",
	"text/plain":            "nt",
}

var extToFormat = map[string]string{
	".ttl":      "turtle",
	".turtle":   "turtle",
	".nt":       "nt",
	".ntriples": "nt",
	".nq":       "nquads",
	".nquads":   "nquads",
	".trig":     "trig",
	".rdf":      "xml",
	".xml":      "xml",
	".owl":      "xml",
	".jsonld":   "json-ld",
	".json":     "json-ld",
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
	if len(data) == 0 {
		return "", false
	}
	n := len(data)
	if n > 500 {
		n = 500
	}
	s := string(data[:n])
	// Strip UTF-8 BOM if present
	s = strings.TrimPrefix(s, "\xEF\xBB\xBF")
	s = strings.TrimSpace(s)
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
