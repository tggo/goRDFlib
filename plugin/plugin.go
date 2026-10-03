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
	// text/plain is deliberately absent: servers routinely send Turtle and
	// JSON-LD as text/plain, so it names no format and detection falls through to FormatFromContent.
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

// MediaType returns contentType lower-cased and without parameters.
func MediaType(contentType string) string {
	ct := strings.TrimSpace(contentType)
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return strings.ToLower(ct)
}

// FormatFromMIME detects the RDF format from a MIME content-type.
// Ported from: rdflib.plugin — format detection by MIME type
func FormatFromMIME(contentType string) (string, bool) {
	// Parameters (e.g. "text/turtle; charset=utf-8") are ignored.
	f, ok := mimeToFormat[MediaType(contentType)]
	return f, ok
}
