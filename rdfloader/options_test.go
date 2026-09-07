package rdfloader_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/rdfloader"
	"github.com/tggo/goRDFlib/term"
)

// TestLoadBlankNodeScope checks repeated loads of the same source through both
// transports. Preserving IDs must compose with the line-length options.
func TestLoadBlankNodeScope(t *testing.T) {
	formats := []struct {
		name, extension, contentType, data string
	}{
		{"turtle", ".ttl", "text/turtle", `_:source <http://example.org/p> _:source .`},
		{"nt", ".nt", "application/n-triples", `_:source <http://example.org/p> _:source .`},
		{"nq", ".nq", "application/n-quads", `_:source <http://example.org/p> _:source <http://example.org/g> .`},
		{"trig", ".trig", "application/trig", `<http://example.org/g> { _:source <http://example.org/p> _:source . }`},
		{"rdfxml", ".rdf", "application/rdf+xml", `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:ex="http://example.org/"><rdf:Description rdf:nodeID="source"><ex:p rdf:nodeID="source"/></rdf:Description></rdf:RDF>`},
		{"jsonld", ".jsonld", "application/ld+json", `{"@id":"_:source","http://example.org/p":{"@id":"_:source"}}`},
	}
	for _, format := range formats {
		t.Run(format.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", format.contentType)
				_, _ = w.Write([]byte(format.data))
			}))
			defer srv.Close()
			path := filepath.Join(t.TempDir(), "source"+format.extension)
			if err := os.WriteFile(path, []byte(format.data), 0o600); err != nil {
				t.Fatal(err)
			}
			for _, transport := range []struct{ name, uri string }{{"http", srv.URL}, {"file", "file://" + path}} {
				t.Run(transport.name, func(t *testing.T) {
					for _, mode := range []struct {
						name     string
						opts     []rdfloader.Option
						preserve bool
					}{
						{name: "default"},
						{"preserve", []rdfloader.Option{rdfloader.WithPreserveBlankNodeIDs()}, true},
						{"preserve-bounded", []rdfloader.Option{rdfloader.WithPreserveBlankNodeIDs(), rdfloader.WithMaxLineLength(1 << 20)}, true},
						{"preserve-unbounded", []rdfloader.Option{rdfloader.WithPreserveBlankNodeIDs(), rdfloader.WithUnboundedLines()}, true},
					} {
						t.Run(mode.name, func(t *testing.T) {
							loader := rdfloader.DefaultLoader(mode.opts...)
							g := graph.NewGraph()
							for i := 0; i < 2; i++ {
								if err := loader.Load(context.Background(), g, transport.uri); err != nil {
									t.Fatal(err)
								}
							}
							want := 2
							if mode.preserve {
								want = 1
							}
							if g.Len() != want {
								t.Fatalf("repeated load: got %d triples, want %d", g.Len(), want)
							}
							for triple := range g.Triples(nil, nil, nil) {
								bn, ok := triple.Subject.(term.BNode)
								if !ok || !triple.Subject.Equal(triple.Object) {
									t.Fatalf("blank-node identity within document lost: %v", triple)
								}
								// json-gold assigns expansion IDs before the N-Quads parse.
								if mode.preserve && format.name != "jsonld" && bn != term.NewBNode("source") {
									t.Fatalf("source ID not preserved: %v", bn)
								}
							}
						})
					}
				})
			}
		})
	}
}

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
