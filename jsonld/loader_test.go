package jsonld

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/piprate/json-gold/ld"
	rdflibgo "github.com/tggo/goRDFlib"
)

const loaderTestContext = `{"@context": {"name": "http://schema.org/name", "Place": "http://schema.org/Place"}}`

// contextServer serves loaderTestContext and counts requests. failFirst makes
// the first n requests answer 500.
func contextServer(t *testing.T, failFirst int32) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) <= failFirst {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/ld+json")
		_, _ = w.Write([]byte(loaderTestContext))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func loaderTestDoc(ctxURL string, i int) string {
	return `{"@context": "` + ctxURL + `", "@id": "http://example.org/p` + string(rune('a'+i%26)) + `", "@type": "Place", "name": "x"}`
}

// Many goroutines parsing documents that share one remote @context fetch it
// once, and json-gold must not modify the shared cached document (-race).
func TestCachingDocumentLoaderConcurrent(t *testing.T) {
	srv, hits := contextServer(t, 0)
	loader := NewCachingDocumentLoader(nil)

	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g := rdflibgo.NewGraph()
			if err := Parse(g, strings.NewReader(loaderTestDoc(srv.URL, i)), WithDocumentLoader(loader)); err != nil {
				errs <- err
				return
			}
			if g.Len() != 2 {
				errs <- errors.New("expected 2 triples")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("context fetched %d times, want 1", n)
	}
}

// A failed fetch is not cached: the next parse tries again and succeeds.
func TestCachingDocumentLoaderDoesNotCacheErrors(t *testing.T) {
	srv, hits := contextServer(t, 1)
	loader := NewCachingDocumentLoader(nil)

	if err := Parse(rdflibgo.NewGraph(), strings.NewReader(loaderTestDoc(srv.URL, 0)), WithDocumentLoader(loader)); err == nil {
		t.Fatal("first parse succeeded against a failing server")
	}
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(loaderTestDoc(srv.URL, 0)), WithDocumentLoader(loader)); err != nil {
		t.Fatalf("second parse: %v", err)
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("server hit %d times, want 2", n)
	}
}

// AddDocument serves a bundled context without any network access.
func TestCachingDocumentLoaderAddDocument(t *testing.T) {
	loader := NewCachingDocumentLoader(failingLoader{})
	doc, err := ld.DocumentFromReader(strings.NewReader(loaderTestContext))
	if err != nil {
		t.Fatal(err)
	}
	loader.AddDocument("https://schema.example/", doc)
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(loaderTestDoc("https://schema.example/", 0)), WithDocumentLoader(loader)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Fatalf("got %d triples, want 2", g.Len())
	}
}

type failingLoader struct{}

func (failingLoader) LoadDocument(u string) (*ld.RemoteDocument, error) {
	return nil, errors.New("no network in this test: " + u)
}
