package rdfloader_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/rdfloader"
)

// bigNQuads builds an N-Quads document whose single line exceeds the 64KB cap.
func bigNQuads() string {
	big := strings.Repeat("x", 100*1024)
	return `<http://example.org/s> <http://example.org/p> "` + big + `" <http://example.org/g> .` + "\n"
}

// TestLoadHTTP_UnboundedLines verifies the loader forwards WithUnboundedLines to
// the N-Quads parser: the same payload fails by default and succeeds with the option.
func TestLoadHTTP_UnboundedLines(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/n-quads")
		_, _ = w.Write([]byte(bigNQuads()))
	}))
	defer srv.Close()

	// Default: fails with an actionable, sentinel-matchable error.
	g1 := graph.NewGraph()
	err := rdfloader.DefaultLoader().Load(context.Background(), g1, srv.URL)
	if err == nil {
		t.Fatal("expected error loading oversized N-Quads with default cap")
	}
	if !errors.Is(err, nq.ErrLineTooLong) {
		t.Fatalf("expected errors.Is(err, nq.ErrLineTooLong), got: %v", err)
	}

	// Opt in: succeeds.
	g2 := graph.NewGraph()
	if err := rdfloader.DefaultLoader(rdfloader.WithUnboundedLines()).Load(context.Background(), g2, srv.URL); err != nil {
		t.Fatalf("unexpected error with WithUnboundedLines: %v", err)
	}
	if g2.Len() != 1 {
		t.Fatalf("expected 1 triple, got %d", g2.Len())
	}
}

// TestLoadHTTP_MaxLineLength verifies the bounded option is forwarded too.
func TestLoadHTTP_MaxLineLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/n-quads")
		_, _ = w.Write([]byte(bigNQuads()))
	}))
	defer srv.Close()

	g := graph.NewGraph()
	loader := rdfloader.DefaultLoader(rdfloader.WithMaxLineLength(1 << 20))
	if err := loader.Load(context.Background(), g, srv.URL); err != nil {
		t.Fatalf("unexpected error with WithMaxLineLength(1MB): %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("expected 1 triple, got %d", g.Len())
	}
}
