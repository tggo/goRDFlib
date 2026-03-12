package rdfloader

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tggo/goRDFlib/graph"
)

// TestLoadFileByExtension covers loadFile with a known extension (.ttl).
func TestLoadFileByExtension(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.ttl")
	if err := os.WriteFile(path, []byte(`<http://example.org/s> <http://example.org/p> "hello" .`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	l := DefaultLoader()
	g := graph.NewGraph()
	if err := l.Load(context.Background(), g, "file://"+path); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestLoadFileNoExtensionSniff covers content sniffing fallback.
func TestLoadFileNoExtensionSniff(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noext")
	content := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p>hello</ex:p>
  </rdf:Description>
</rdf:RDF>`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	l := DefaultLoader()
	g := graph.NewGraph()
	if err := l.Load(context.Background(), g, "file://"+path); err != nil {
		t.Fatal(err)
	}
}

// TestLoadFileEmpty covers the empty file error path.
func TestLoadFileEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	l := DefaultLoader()
	g := graph.NewGraph()
	err := l.Load(context.Background(), g, "file://"+path)
	if err == nil {
		t.Error("expected error for empty file")
	}
}

// TestLoadFileNotFound covers the file not found error path.
func TestLoadFileNotFound(t *testing.T) {
	l := DefaultLoader()
	g := graph.NewGraph()
	err := l.Load(context.Background(), g, "file:///nonexistent/path/file.ttl")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// TestLoadFileUndetectable covers file with no extension and unrecognized content.
func TestLoadFileUndetectable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noext")
	if err := os.WriteFile(path, []byte("just random text that is not RDF"), 0644); err != nil {
		t.Fatal(err)
	}

	l := DefaultLoader()
	g := graph.NewGraph()
	err := l.Load(context.Background(), g, "file://"+path)
	if err == nil {
		t.Error("expected error for undetectable format")
	}
}

// TestLoadHTTPWithContentType covers HTTP loading with Content-Type header.
func TestLoadHTTPWithContentType(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/turtle")
		w.Write([]byte(`<http://example.org/s> <http://example.org/p> "hello" .` + "\n"))
	}))
	defer srv.Close()

	l := DefaultLoader()
	g := graph.NewGraph()
	if err := l.Load(context.Background(), g, srv.URL+"/data.ttl"); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestLoadHTTPByExtension covers HTTP loading with URL extension fallback.
func TestLoadHTTPByExtension(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(`<http://example.org/s> <http://example.org/p> "hello" .` + "\n"))
	}))
	defer srv.Close()

	l := DefaultLoader()
	g := graph.NewGraph()
	if err := l.Load(context.Background(), g, srv.URL+"/data.nt"); err != nil {
		t.Fatal(err)
	}
}

// TestLoadHTTPByContentSniffing covers HTTP content sniffing fallback.
func TestLoadHTTPByContentSniffing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(`<http://example.org/s> <http://example.org/p> "hello" .` + "\n"))
	}))
	defer srv.Close()

	l := DefaultLoader()
	g := graph.NewGraph()
	if err := l.Load(context.Background(), g, srv.URL+"/data"); err != nil {
		t.Fatal(err)
	}
}

// TestLoadHTTPEmptyResponse covers the empty HTTP response error path.
func TestLoadHTTPEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		// Write nothing
	}))
	defer srv.Close()

	l := DefaultLoader()
	g := graph.NewGraph()
	err := l.Load(context.Background(), g, srv.URL+"/data")
	if err == nil {
		t.Error("expected error for empty HTTP response")
	}
}

// TestLoadHTTPUndetectableFormat covers undetectable format from HTTP content sniffing.
func TestLoadHTTPUndetectableFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte("random non-rdf text"))
	}))
	defer srv.Close()

	l := DefaultLoader()
	g := graph.NewGraph()
	err := l.Load(context.Background(), g, srv.URL+"/data")
	if err == nil {
		t.Error("expected error for undetectable format")
	}
}

// TestLoadHTTPNon200 covers HTTP non-200 status code.
func TestLoadHTTPNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	l := DefaultLoader()
	g := graph.NewGraph()
	err := l.Load(context.Background(), g, srv.URL+"/notfound")
	if err == nil {
		t.Error("expected error for HTTP 404")
	}
}

// TestLoadUnsupportedScheme covers unsupported URI scheme.
func TestLoadUnsupportedScheme(t *testing.T) {
	l := DefaultLoader()
	g := graph.NewGraph()
	err := l.Load(context.Background(), g, "ftp://example.org/data.ttl")
	if err == nil {
		t.Error("expected error for unsupported URI scheme")
	}
}

// TestLoadInvalidURI covers invalid URI.
func TestLoadInvalidURI(t *testing.T) {
	l := DefaultLoader()
	g := graph.NewGraph()
	err := l.Load(context.Background(), g, "://bad")
	if err == nil {
		t.Error("expected error for invalid URI")
	}
}

// TestWithHTTPClient covers the WithHTTPClient option.
func TestWithHTTPClient(t *testing.T) {
	client := &http.Client{Timeout: 5 * time.Second}
	l := DefaultLoader(WithHTTPClient(client))
	if l.client != client {
		t.Error("expected custom client to be set")
	}
}

// TestWithTimeout covers the WithTimeout option.
func TestWithTimeout(t *testing.T) {
	l := DefaultLoader(WithTimeout(10 * time.Second))
	if l.timeout != 10*time.Second {
		t.Errorf("expected 10s timeout, got %v", l.timeout)
	}
}

// TestParseFormatUnsupported covers the unsupported format error path.
func TestParseFormatUnsupported(t *testing.T) {
	g := graph.NewGraph()
	err := parseFormat(g, nil, "unknown-format")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

// TestParseFormatNT covers NT format dispatch.
func TestParseFormatNT(t *testing.T) {
	g := graph.NewGraph()
	r := readerFromString(`<http://example.org/s> <http://example.org/p> "hello" .` + "\n")
	if err := parseFormat(g, r, "nt"); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParseFormatNQuads covers NQuads format dispatch.
func TestParseFormatNQuads(t *testing.T) {
	g := graph.NewGraph()
	r := readerFromString(`<http://example.org/s> <http://example.org/p> "hello" .` + "\n")
	if err := parseFormat(g, r, "nquads"); err != nil {
		t.Fatal(err)
	}
}

// TestParseFormatTrig covers TriG format dispatch.
func TestParseFormatTrig(t *testing.T) {
	g := graph.NewGraph()
	r := readerFromString(`@prefix ex: <http://example.org/> . ex:s ex:p "v" .`)
	if err := parseFormat(g, r, "trig"); err != nil {
		t.Fatal(err)
	}
}

// TestParseFormatXML covers XML format dispatch.
func TestParseFormatXML(t *testing.T) {
	g := graph.NewGraph()
	content := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p>hello</ex:p>
  </rdf:Description>
</rdf:RDF>`
	r := readerFromString(content)
	if err := parseFormat(g, r, "xml"); err != nil {
		t.Fatal(err)
	}
}

// TestParseFormatJSONLD covers JSON-LD format dispatch.
func TestParseFormatJSONLD(t *testing.T) {
	g := graph.NewGraph()
	content := `{"@id": "http://example.org/s", "http://example.org/p": "hello"}`
	r := readerFromString(content)
	if err := parseFormat(g, r, "json-ld"); err != nil {
		t.Fatal(err)
	}
}

// TestLoadLocalPathNoScheme covers loading a local file path without file:// scheme.
func TestLoadLocalPathNoScheme(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.ttl")
	if err := os.WriteFile(path, []byte(`<http://example.org/s> <http://example.org/p> "hello" .`+"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	l := DefaultLoader()
	g := graph.NewGraph()
	if err := l.Load(context.Background(), g, path); err != nil {
		t.Fatal(err)
	}
}

// TestHTTPClientDefault covers the default HTTP client path.
func TestHTTPClientDefault(t *testing.T) {
	l := DefaultLoader()
	c := l.httpClient()
	if c == nil {
		t.Error("expected non-nil default client")
	}
}

// TestHTTPClientCustom covers the custom HTTP client path.
func TestHTTPClientCustom(t *testing.T) {
	custom := &http.Client{Timeout: 1 * time.Second}
	l := DefaultLoader(WithHTTPClient(custom))
	c := l.httpClient()
	if c != custom {
		t.Error("expected custom client")
	}
}

// TestLoadFileFormats covers loading various file formats by extension.
func TestLoadFileFormats(t *testing.T) {
	ttlContent := `<http://example.org/s> <http://example.org/p> "hello" .` + "\n"
	nqContent := `<http://example.org/s> <http://example.org/p> "hello" .` + "\n"
	xmlContent := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:p>hello</ex:p>
  </rdf:Description>
</rdf:RDF>`
	jsonldContent := `{"@id": "http://example.org/s", "http://example.org/p": "hello"}`

	tests := []struct {
		ext     string
		content string
	}{
		{".nt", ttlContent},
		{".nq", nqContent},
		{".rdf", xmlContent},
		{".jsonld", jsonldContent},
	}

	for _, tc := range tests {
		t.Run(tc.ext, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test"+tc.ext)
			if err := os.WriteFile(path, []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
			l := DefaultLoader()
			g := graph.NewGraph()
			if err := l.Load(context.Background(), g, "file://"+path); err != nil {
				t.Fatalf("failed loading %s: %v", tc.ext, err)
			}
		})
	}
}

func readerFromString(s string) *strings.Reader {
	return strings.NewReader(s)
}
