package rdfloader_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/rdfloader"
)

func TestLoadFile(t *testing.T) {
	// Write a temp turtle file
	dir := t.TempDir()
	path := filepath.Join(dir, "data.ttl")
	content := `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o .
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, "file://"+path); err != nil {
		t.Fatalf("Load file:// failed: %v", err)
	}

	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

func TestLoadFileNoScheme(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.nt")
	content := `<http://example.org/s> <http://example.org/p> <http://example.org/o> .
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, path); err != nil {
		t.Fatalf("Load without scheme failed: %v", err)
	}

	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

func TestLoadHTTP(t *testing.T) {
	ttl := `@prefix ex: <http://example.org/> .
ex:a ex:b ex:c .
ex:d ex:e ex:f .
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/turtle")
		w.Write([]byte(ttl))
	}))
	defer srv.Close()

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, srv.URL+"/data.ttl"); err != nil {
		t.Fatalf("Load HTTP failed: %v", err)
	}

	if g.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", g.Len())
	}
}

func TestLoadHTTPExtensionFallback(t *testing.T) {
	nt := `<http://example.org/s> <http://example.org/p> <http://example.org/o> .
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No Content-Type, but URL has .nt extension
		w.Write([]byte(nt))
	}))
	defer srv.Close()

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, srv.URL+"/data.nt"); err != nil {
		t.Fatalf("Load HTTP extension fallback failed: %v", err)
	}

	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

func TestLoadHTTPContentSniffing(t *testing.T) {
	nt := `<http://example.org/s> <http://example.org/p> <http://example.org/o> .
`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write([]byte(nt))
	}))
	defer srv.Close()

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	// URL has no extension, Content-Type is useless — must sniff
	if err := loader.Load(context.Background(), g, srv.URL+"/data"); err != nil {
		t.Fatalf("Load HTTP content sniffing failed: %v", err)
	}

	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

func TestLoadHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	err := loader.Load(context.Background(), g, srv.URL+"/missing.ttl")
	if err == nil {
		t.Fatal("expected error for HTTP 404")
	}
}

func TestLoadUnsupportedScheme(t *testing.T) {
	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	err := loader.Load(context.Background(), g, "ftp://example.org/data.ttl")
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}

func TestLoadFileContentSniffing(t *testing.T) {
	// File with no recognized extension
	dir := t.TempDir()
	path := filepath.Join(dir, "data.rdfdata")
	content := `<http://example.org/s> <http://example.org/p> <http://example.org/o> .
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader()
	if err := loader.Load(context.Background(), g, "file://"+path); err != nil {
		t.Fatalf("Load file with content sniffing failed: %v", err)
	}

	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}
