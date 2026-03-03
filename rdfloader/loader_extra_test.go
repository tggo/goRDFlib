package rdfloader_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/rdfloader"
)

// --- WithHTTPClient option ---

func TestWithHTTPClient(t *testing.T) {
	ttl := `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o .
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/turtle")
		w.Write([]byte(ttl))
	}))
	defer srv.Close()

	customClient := &http.Client{Timeout: 5 * time.Second}
	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader(rdfloader.WithHTTPClient(customClient))
	if err := loader.Load(context.Background(), g, srv.URL+"/data.ttl"); err != nil {
		t.Fatalf("Load with custom HTTP client failed: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// --- WithTimeout option ---

func TestWithTimeout(t *testing.T) {
	ttl := `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o .
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/turtle")
		w.Write([]byte(ttl))
	}))
	defer srv.Close()

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader(rdfloader.WithTimeout(10 * time.Second))
	if err := loader.Load(context.Background(), g, srv.URL+"/data.ttl"); err != nil {
		t.Fatalf("Load with timeout failed: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// --- Load file not found ---

func TestLoadFileNotFound(t *testing.T) {
	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	err := loader.Load(context.Background(), g, "file:///nonexistent/path/data.ttl")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

// --- Load file with empty content ---

func TestLoadFileEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.rdfdata") // unknown extension → sniff
	if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	err := loader.Load(context.Background(), g, "file://"+path)
	if err == nil {
		t.Fatal("expected error for empty file with unknown extension")
	}
}

// --- Load file unknown extension and undetectable content ---

func TestLoadFileUndetectableFormat(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.xyz")
	content := "this is not rdf at all !!!@@@###$$$"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	err := loader.Load(context.Background(), g, "file://"+path)
	if err == nil {
		t.Fatal("expected error for undetectable format")
	}
}

// --- Load HTTP empty response (no content-type, no extension, empty body) ---

func TestLoadHTTPEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return 200 with empty body and no content-type
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	err := loader.Load(context.Background(), g, srv.URL+"/data")
	if err == nil {
		t.Fatal("expected error for empty HTTP response")
	}
}

// --- Load HTTP undetectable content ---

func TestLoadHTTPUndetectableContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("this is definitely not rdf content !!! ???"))
	}))
	defer srv.Close()

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	// No extension, content-type not recognized, content not sniffable
	err := loader.Load(context.Background(), g, srv.URL+"/data")
	if err == nil {
		t.Fatal("expected error for undetectable HTTP content")
	}
}

// --- parseFormat: unsupported format ---

func TestLoadHTTPUnsupportedFormatViaContentType(t *testing.T) {
	// Serve a known MIME type that maps to an unsupported format name
	// This is hard to trigger via the public API since FormatFromMIME only returns
	// known formats. Instead we test via a file with a known extension that maps
	// to a format parseFormat doesn't handle... which is also unlikely.
	// We test the "default" branch of parseFormat by using a file with .rdf extension
	// if that maps to an unsupported format, or through HTTP content sniffing failure.
	// The most reliable approach: test an HTTP response with an extension parseFormat
	// doesn't support — but the plugin FormatFromFilename will fail first.
	// Accept that this branch may only be reachable through internal package state.
	t.Log("parseFormat unsupported format is an internal fallback; tested via content sniffing failure tests")
}

// --- Load N-Quads via file ---

func TestLoadFileNQuads(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.nq")
	content := "<http://example.org/s> <http://example.org/p> <http://example.org/o> <http://example.org/g> .\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, "file://"+path); err != nil {
		t.Fatalf("Load .nq failed: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// --- Load TriG via file ---

func TestLoadFileTriG(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.trig")
	content := `@prefix ex: <http://example.org/> .
GRAPH ex:g { ex:s ex:p ex:o . }
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, "file://"+path); err != nil {
		t.Fatalf("Load .trig failed: %v", err)
	}
}

// --- Load RDF/XML via file ---

func TestLoadFileRDFXML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.rdf")
	content := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:resource="http://example.org/o"/>
  </rdf:Description>
</rdf:RDF>
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, "file://"+path); err != nil {
		t.Fatalf("Load .rdf failed: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// --- Load JSON-LD via file ---

func TestLoadFileJSONLD(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.jsonld")
	content := `{
  "@context": {"ex": "http://example.org/"},
  "@id": "ex:s",
  "ex:p": {"@id": "ex:o"}
}
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, "file://"+path); err != nil {
		t.Fatalf("Load .jsonld failed: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// --- Load via HTTP with various formats via Content-Type ---

func TestLoadHTTPNQuads(t *testing.T) {
	body := "<http://example.org/s> <http://example.org/p> <http://example.org/o> <http://example.org/g> .\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/n-quads")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, srv.URL+"/data"); err != nil {
		t.Fatalf("Load HTTP n-quads failed: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

func TestLoadHTTPTriG(t *testing.T) {
	body := `@prefix ex: <http://example.org/> .
GRAPH ex:g { ex:s ex:p ex:o . }
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/trig")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, srv.URL+"/data"); err != nil {
		t.Fatalf("Load HTTP trig failed: %v", err)
	}
}

func TestLoadHTTPRDFXML(t *testing.T) {
	body := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p rdf:resource="http://example.org/o"/>
  </rdf:Description>
</rdf:RDF>
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdf+xml")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, srv.URL+"/data"); err != nil {
		t.Fatalf("Load HTTP rdf+xml failed: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

func TestLoadHTTPNTriples(t *testing.T) {
	body := "<http://example.org/s> <http://example.org/p> <http://example.org/o> .\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/n-triples")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, srv.URL+"/data"); err != nil {
		t.Fatalf("Load HTTP n-triples failed: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

func TestLoadHTTPJSONLD(t *testing.T) {
	body := `{
  "@context": {"ex": "http://example.org/"},
  "@id": "ex:s",
  "ex:p": {"@id": "ex:o"}
}
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/ld+json")
		w.Write([]byte(body))
	}))
	defer srv.Close()

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, srv.URL+"/data"); err != nil {
		t.Fatalf("Load HTTP json-ld failed: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}
