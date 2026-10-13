// Example demonstrating SPARQL UPDATE LOAD with rdfloader.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/rdfloader"
	"github.com/tggo/goRDFlib/sparql"
)

func main() {
	ctx := context.Background()

	// --- Example 1: Load from HTTP ---
	fmt.Println("=== Example 1: LOAD from HTTP ===")
	if err := httpExample(ctx); err != nil {
		log.Fatal(err)
	}

	// --- Example 2: Load from file:// ---
	fmt.Println("\n=== Example 2: LOAD from file:// ===")
	if err := fileExample(ctx); err != nil {
		log.Fatal(err)
	}

	// --- Example 3: Custom HTTP client ---
	fmt.Println("\n=== Example 3: LOAD with custom HTTP client ===")
	if err := customClientExample(ctx); err != nil {
		log.Fatal(err)
	}
}

func httpExample(ctx context.Context) error {
	// Start a test server serving Turtle data
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/turtle")
		fmt.Fprint(w, `@prefix ex: <http://example.org/> .
ex:Alice ex:knows ex:Bob .
ex:Bob ex:knows ex:Charlie .
`)
	}))
	defer srv.Close()

	ds := &sparql.Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: make(map[string]*rdflibgo.Graph),
		Loader:      rdfloader.DefaultLoader(),
	}

	query := fmt.Sprintf("LOAD <%s/data.ttl>", srv.URL)
	if err := sparql.UpdateContext(ctx, ds, query); err != nil {
		return err
	}
	n, err := ds.Default.Len(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Loaded %d triples into default graph\n", n)
	return nil
}

func fileExample(ctx context.Context) error {
	// Create a temp file with N-Triples data
	dir, err := os.MkdirTemp("", "rdfloader-example")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "data.nt")
	if err := os.WriteFile(path, []byte(`<http://example.org/s> <http://example.org/p> "hello" .
`), 0o644); err != nil {
		return err
	}

	ds := &sparql.Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: make(map[string]*rdflibgo.Graph),
		Loader:      rdfloader.DefaultLoader(),
	}

	query := fmt.Sprintf("LOAD <file://%s>", path)
	if err := sparql.UpdateContext(ctx, ds, query); err != nil {
		return err
	}
	n, err := ds.Default.Len(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Loaded %d triples from file\n", n)
	return nil
}

func customClientExample(ctx context.Context) error {
	// Server that requires a custom header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "secret" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "text/turtle")
		fmt.Fprint(w, `@prefix ex: <http://example.org/> .
ex:Private ex:data "secret-value" .
`)
	}))
	defer srv.Close()

	// Custom transport that adds the header
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &headerTransport{
			base:   http.DefaultTransport,
			header: "X-API-Key",
			value:  "secret",
		},
	}

	ds := &sparql.Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: make(map[string]*rdflibgo.Graph),
		Loader:      rdfloader.DefaultLoader(rdfloader.WithHTTPClient(client)),
	}

	query := fmt.Sprintf("LOAD <%s/private.ttl>", srv.URL)
	if err := sparql.UpdateContext(ctx, ds, query); err != nil {
		return err
	}
	n, err := ds.Default.Len(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Loaded %d triples with auth\n", n)
	return nil
}

type headerTransport struct {
	base   http.RoundTripper
	header string
	value  string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set(t.header, t.value)
	return t.base.RoundTrip(req)
}
