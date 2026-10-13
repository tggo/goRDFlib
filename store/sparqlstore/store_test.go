package sparqlstore_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/sparqlstore"
	"github.com/tggo/goRDFlib/term"
)

// newTestServer creates an httptest.Server backed by an in-memory graph and
// returns the server, a remote SPARQLStore connected to it, and a cleanup func.
func newTestServer(t *testing.T) (*httptest.Server, *sparqlstore.SPARQLStore) {
	t.Helper()
	g := graph.NewGraph()
	ds := &sparql.Dataset{
		Default:     g,
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
	srv := sparqlstore.NewServer(ds)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(
		ts.URL+"/query",
		sparqlstore.WithUpdate(ts.URL+"/update"),
	)
	return ts, remote
}

func TestAddAndTriples(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")

	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: o}, nil)

	var count int
	for range store.Triples(t.Context(), term.TriplePattern{}, nil) {
		count++
	}
	if count != 1 {
		t.Fatalf("expected 1 triple, got %d", count)
	}

	// Check the actual values
	for tr := range store.Triples(t.Context(), term.TriplePattern{Subject: s}, nil) {
		if !tr.Subject.Equal(s) {
			t.Errorf("subject = %v, want %v", tr.Subject, s)
		}
		if !tr.Predicate.Equal(p) {
			t.Errorf("predicate = %v, want %v", tr.Predicate, p)
		}
		if !tr.Object.Equal(o) {
			t.Errorf("object = %v, want %v", tr.Object, o)
		}
	}
}

func TestLen(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")

	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("one")}, nil)
	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("two")}, nil)

	if n, _ := store.Len(t.Context(), nil); n != 2 {
		t.Fatalf("Len = %d, want 2", n)
	}
}

func TestRemove(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")

	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: o}, nil)
	store.Remove(t.Context(), term.TriplePattern{Subject: s, Predicate: &p, Object: o}, nil)

	if n, _ := store.Len(t.Context(), nil); n != 0 {
		t.Fatalf("Len after remove = %d, want 0", n)
	}
}

func TestSet(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")

	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("old")}, nil)
	store.Set(t.Context(), term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("new")}, nil)

	if n, _ := store.Len(t.Context(), nil); n != 1 {
		t.Fatalf("Len after set = %d, want 1", n)
	}

	for tr := range store.Triples(t.Context(), term.TriplePattern{Subject: s}, nil) {
		lit, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatalf("object is not Literal: %T", tr.Object)
		}
		if lit.Value() != "new" {
			t.Errorf("object value = %q, want %q", lit.Value(), "new")
		}
	}
}

func TestNamespaces(t *testing.T) {
	_, store := newTestServer(t)

	ns := term.NewURIRefUnsafe("http://example.org/")
	store.Bind(t.Context(), "ex", ns)

	got, ok, _ := store.Namespace(t.Context(), "ex")
	if !ok || !got.Equal(ns) {
		t.Errorf("Namespace(ex) = %v, %v; want %v, true", got, ok, ns)
	}

	prefix, ok, _ := store.Prefix(t.Context(), ns)
	if !ok || prefix != "ex" {
		t.Errorf("Prefix(%v) = %q, %v; want %q, true", ns, prefix, ok, "ex")
	}

	var nsCount int
	for range store.Namespaces(t.Context()) {
		nsCount++
	}
	if nsCount != 1 {
		t.Errorf("namespace count = %d, want 1", nsCount)
	}
}

func TestContextAware(t *testing.T) {
	_, store := newTestServer(t)
	if !store.ContextAware() {
		t.Error("ContextAware() = false, want true")
	}
	if store.TransactionAware() {
		t.Error("TransactionAware() = true, want false")
	}
}

func TestAddN(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")

	quads := []term.Quad{
		{Triple: term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("a")}},
		{Triple: term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("b")}},
	}
	store.AddN(t.Context(), quads)

	if n, _ := store.Len(t.Context(), nil); n != 2 {
		t.Fatalf("Len after AddN = %d, want 2", n)
	}
}

func TestTriplesWithPattern(t *testing.T) {
	_, store := newTestServer(t)

	s1 := term.NewURIRefUnsafe("http://example.org/s1")
	s2 := term.NewURIRefUnsafe("http://example.org/s2")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("val")

	store.Add(t.Context(), term.Triple{Subject: s1, Predicate: p, Object: o}, nil)
	store.Add(t.Context(), term.Triple{Subject: s2, Predicate: p, Object: o}, nil)

	// Filter by subject
	var count int
	for range store.Triples(t.Context(), term.TriplePattern{Subject: s1}, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("triples with subject s1 = %d, want 1", count)
	}
}

func TestGraphIntegration(t *testing.T) {
	ts, remote := newTestServer(t)
	_ = ts

	// Use the remote store with a graph
	g := graph.NewGraph(graph.WithStore(remote))

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("graph test")

	g.Add(t.Context(), s, p, o)

	if mustV(g.Len(t.Context())) != 1 {
		t.Fatalf("graph Len = %d, want 1", mustV(g.Len(t.Context())))
	}

	// Query through SPARQL
	result, err := sparql.Query(g, `SELECT ?o WHERE { <http://example.org/s> <http://example.org/p> ?o }`)
	if err != nil {
		t.Fatalf("SPARQL query error: %v", err)
	}
	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}
	if lit, ok := result.Bindings[0]["o"].(rdflibgo.Literal); !ok || lit.Value() != "graph test" {
		t.Errorf("unexpected result: %v", result.Bindings[0]["o"])
	}
}

func TestContexts(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a mock SRX with one graph URI.
		w.Header().Set("Content-Type", "application/sparql-results+xml")
		w.Write([]byte(`<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
<head><variable name="g"/></head>
<results>
<result><binding name="g"><uri>http://example.org/graph1</uri></binding></result>
</results>
</sparql>`))
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	var count int
	for g := range remote.Contexts(t.Context(), nil) {
		count++
		if u, ok := g.(term.URIRef); !ok || u.Value() != "http://example.org/graph1" {
			t.Errorf("unexpected context: %v", g)
		}
	}
	if count != 1 {
		t.Errorf("Contexts count = %d, want 1", count)
	}
}

func TestContextsWithTriple(t *testing.T) {
	_, store := newTestServer(t)

	// Contexts with a specific triple filter
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")
	triple := term.Triple{Subject: s, Predicate: p, Object: o}

	var count int
	for range store.Contexts(t.Context(), &triple) {
		count++
	}
	// No named graphs, so expect 0
	if count != 0 {
		t.Errorf("Contexts count = %d, want 0", count)
	}
}

func TestWithHTTPClient(t *testing.T) {
	_, store := newTestServer(t)
	_ = store

	// Verify WithHTTPClient option works
	custom := &http.Client{Timeout: 5 * time.Second}
	remote := sparqlstore.New("http://localhost:0/query",
		sparqlstore.WithHTTPClient(custom),
	)
	// The store should be created without error
	if remote == nil {
		t.Fatal("New returned nil with WithHTTPClient")
	}
}

func TestWithTimeout(t *testing.T) {
	remote := sparqlstore.New("http://localhost:0/query",
		sparqlstore.WithTimeout(10*time.Second),
	)
	if remote == nil {
		t.Fatal("New returned nil with WithTimeout")
	}
}

func TestAddWithNamedGraph(t *testing.T) {
	g := graph.NewGraph()
	ds := &sparql.Dataset{
		Default:     g,
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
	srv := sparqlstore.NewServer(ds)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	graphURI := term.NewURIRefUnsafe("http://example.org/graph1")
	remote := sparqlstore.New(
		ts.URL+"/query",
		sparqlstore.WithUpdate(ts.URL+"/update"),
	)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("named")

	// Add with a URIRef context (named graph) — sends GRAPH <uri> { ... }
	remote.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: o}, graphURI)

	// The named graph should exist in the server dataset after the update
	ng, ok := ds.NamedGraphs["http://example.org/graph1"]
	if !ok {
		t.Fatal("expected named graph to be created in dataset")
	}
	if mustV(ng.Len(t.Context())) != 1 {
		t.Errorf("named graph len = %d, want 1", mustV(ng.Len(t.Context())))
	}
}

func TestAddNoUpdateEndpoint(t *testing.T) {
	// Store without update endpoint — every write is refused with a sentinel
	remote := sparqlstore.New("http://localhost:0/query")
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")

	tr := term.Triple{Subject: s, Predicate: p, Object: o}
	for name, err := range map[string]error{
		"Add":    remote.Add(t.Context(), tr, nil),
		"Remove": remote.Remove(t.Context(), term.TriplePattern{Subject: s}, nil),
		"Set":    remote.Set(t.Context(), tr, nil),
		"AddN":   remote.AddN(t.Context(), []term.Quad{{Triple: tr}}),
	} {
		if !errors.Is(err, sparqlstore.ErrNoUpdateEndpoint) {
			t.Errorf("%s without an update endpoint: err = %v, want ErrNoUpdateEndpoint", name, err)
		}
	}
	if err := remote.AddN(t.Context(), nil); !errors.Is(err, sparqlstore.ErrNoUpdateEndpoint) {
		t.Errorf("AddN(nil) without an update endpoint: err = %v, want ErrNoUpdateEndpoint", err)
	}
}

func TestTriplesWithBNodeObjects(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	bnode := term.NewBNode()

	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: bnode}, nil)

	var count int
	for tr := range store.Triples(t.Context(), term.TriplePattern{}, nil) {
		count++
		if _, ok := tr.Object.(term.BNode); !ok {
			t.Errorf("expected BNode object, got %T", tr.Object)
		}
	}
	if count != 1 {
		t.Errorf("expected 1 triple, got %d", count)
	}
}

func TestTriplesWithTypedLiteral(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral(42)

	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: o}, nil)

	var count int
	for tr := range store.Triples(t.Context(), term.TriplePattern{}, nil) {
		count++
		lit, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatalf("expected Literal, got %T", tr.Object)
		}
		if lit.Datatype() != term.XSDInteger {
			t.Errorf("datatype = %v, want xsd:integer", lit.Datatype())
		}
	}
	if count != 1 {
		t.Errorf("expected 1 triple, got %d", count)
	}
}

func TestTriplesWithLangLiteral(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("bonjour", rdflibgo.WithLang("fr"))

	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: o}, nil)

	for tr := range store.Triples(t.Context(), term.TriplePattern{}, nil) {
		lit, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatalf("expected Literal, got %T", tr.Object)
		}
		if lit.Language() != "fr" {
			t.Errorf("language = %q, want %q", lit.Language(), "fr")
		}
	}
}

func TestTriplesErrorHandling(t *testing.T) {
	// Store pointing at a server that returns errors
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	n, err := drainTriples(remote.Triples(t.Context(), term.TriplePattern{}, nil))
	if n != 0 || err == nil {
		t.Errorf("Triples on a 500 = %d triples, err %v; want 0 and an error", n, err)
	}
}

// drainTriples counts the triples an iterator yields and returns its error.
func drainTriples(it store.TripleIterator) (int, error) {
	n := 0
	for _, err := range it {
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func drainTerms(it store.TermIterator) (int, error) {
	n := 0
	for _, err := range it {
		if err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func TestLenErrorHandling(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	if n, err := remote.Len(t.Context(), nil); n != 0 || err == nil {
		t.Errorf("Len on a 500 = %d, err %v; want 0 and an error", n, err)
	}
}

func TestContextsErrorHandling(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "error", http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	if n, err := drainTerms(remote.Contexts(t.Context(), nil)); n != 0 || err == nil {
		t.Errorf("Contexts on a 500 = %d, err %v; want 0 and an error", n, err)
	}
}

func TestExecUpdateError(t *testing.T) {
	// Server returns 500 on update
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "update failed", http.StatusInternalServerError)
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL+"/query", sparqlstore.WithUpdate(ts.URL+"/update"))
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")

	if err := remote.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: o}, nil); err == nil {
		t.Error("Add against a server that answers 500 returned no error")
	}
}

func TestExecQueryJSONResponse(t *testing.T) {
	// Server returns JSON results
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/sparql-results+json")
		w.Write([]byte(`{"head":{"vars":["s","p","o"]},"results":{"bindings":[{"s":{"type":"uri","value":"http://example.org/x"},"p":{"type":"uri","value":"http://example.org/y"},"o":{"type":"literal","value":"z"}}]}}`))
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	var count int
	for range remote.Triples(t.Context(), term.TriplePattern{}, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 triple from JSON, got %d", count)
	}
}

func TestLenWithNamedGraphContext(t *testing.T) {
	_, store := newTestServer(t)

	// Len with a URIRef context wraps in GRAPH — exercises wrapGraph with URIRef
	graphURI := term.NewURIRefUnsafe("http://example.org/graph1")
	n, _ := store.Len(t.Context(), graphURI)
	if n != 0 {
		t.Errorf("Len(graph1) = %d, want 0", n)
	}
}

func TestRemoveWithWildcard(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")

	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("a")}, nil)
	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("b")}, nil)

	// Remove all triples with subject s (wildcard predicate and object)
	store.Remove(t.Context(), term.TriplePattern{Subject: s}, nil)

	if n, _ := store.Len(t.Context(), nil); n != 0 {
		t.Errorf("Len after wildcard remove = %d, want 0", n)
	}
}

func TestExecQueryInvalidURL(t *testing.T) {
	// Invalid URL triggers NewRequestWithContext error
	remote := sparqlstore.New("://bad\x00url")

	if n, err := drainTriples(remote.Triples(t.Context(), term.TriplePattern{}, nil)); n != 0 || err == nil {
		t.Errorf("Triples with a bad URL = %d, err %v; want 0 and an error", n, err)
	}
	if n, err := remote.Len(t.Context(), nil); n != 0 || err == nil {
		t.Errorf("Len with a bad URL = %d, err %v; want 0 and an error", n, err)
	}
	if n, err := drainTerms(remote.Contexts(t.Context(), nil)); n != 0 || err == nil {
		t.Errorf("Contexts with a bad URL = %d, err %v; want 0 and an error", n, err)
	}
}

func TestExecUpdateInvalidURL(t *testing.T) {
	// Invalid update URL triggers NewRequestWithContext error
	remote := sparqlstore.New("http://localhost:0/query",
		sparqlstore.WithUpdate("://bad\x00url"),
	)
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")

	tr := term.Triple{Subject: s, Predicate: p, Object: o}
	for name, err := range map[string]error{
		"Add":    remote.Add(t.Context(), tr, nil),
		"Set":    remote.Set(t.Context(), tr, nil),
		"Remove": remote.Remove(t.Context(), term.TriplePattern{Subject: s}, nil),
	} {
		if err == nil {
			t.Errorf("%s with an unusable update URL returned no error", name)
		}
	}
}

func TestExecQueryConnectionRefused(t *testing.T) {
	// Valid URL but nothing listening — client.Do error
	remote := sparqlstore.New("http://127.0.0.1:1/query")

	n, err := drainTriples(remote.Triples(t.Context(), term.TriplePattern{}, nil))
	if n != 0 || err == nil {
		t.Errorf("Triples with the server down = %d, err %v; want 0 and an error", n, err)
	}
	if _, err := remote.Len(t.Context(), nil); err == nil {
		t.Error("Len with the server down returned no error")
	}
	if _, err := drainTerms(remote.Contexts(t.Context(), nil)); err == nil {
		t.Error("Contexts with the server down returned no error")
	}
}

func TestExecUpdateConnectionRefused(t *testing.T) {
	remote := sparqlstore.New("http://127.0.0.1:1/query",
		sparqlstore.WithUpdate("http://127.0.0.1:1/update"),
	)
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello")

	if err := remote.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: o}, nil); err == nil {
		t.Error("Add with the server down returned no error")
	}
}

func TestTriplesWithObjectPattern(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o1 := rdflibgo.NewLiteral("one")
	o2 := rdflibgo.NewLiteral("two")

	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: o1}, nil)
	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: o2}, nil)

	// Filter by all three positions
	var count int
	for range store.Triples(t.Context(), term.TriplePattern{Subject: s, Predicate: &p, Object: o1}, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("triples with full pattern = %d, want 1", count)
	}
}

func TestTriplesEarlyBreak(t *testing.T) {
	_, store := newTestServer(t)

	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("a")}, nil)
	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("b")}, nil)
	store.Add(t.Context(), term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("c")}, nil)

	// Break after first triple
	var count int
	for range store.Triples(t.Context(), term.TriplePattern{}, nil) {
		count++
		if count == 1 {
			break
		}
	}
	if count != 1 {
		t.Errorf("early break: got %d, want 1", count)
	}
}

func TestContextsEarlyBreak(t *testing.T) {
	// Mock server returning multiple graph URIs
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/sparql-results+xml")
		w.Write([]byte(`<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
<head><variable name="g"/></head>
<results>
<result><binding name="g"><uri>http://example.org/g1</uri></binding></result>
<result><binding name="g"><uri>http://example.org/g2</uri></binding></result>
<result><binding name="g"><uri>http://example.org/g3</uri></binding></result>
</results>
</sparql>`))
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	var count int
	for range remote.Contexts(t.Context(), nil) {
		count++
		if count == 1 {
			break
		}
	}
	if count != 1 {
		t.Errorf("early break in Contexts: got %d, want 1", count)
	}
}

func TestNamespacesEarlyBreak(t *testing.T) {
	_, store := newTestServer(t)

	store.Bind(t.Context(), "ex", term.NewURIRefUnsafe("http://example.org/"))
	store.Bind(t.Context(), "foaf", term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))
	store.Bind(t.Context(), "dc", term.NewURIRefUnsafe("http://purl.org/dc/elements/1.1/"))

	var count int
	for range store.Namespaces(t.Context()) {
		count++
		if count == 1 {
			break
		}
	}
	if count != 1 {
		t.Errorf("early break in Namespaces: got %d, want 1", count)
	}
}

func TestTriplesMalformedResult(t *testing.T) {
	// Server returns results where subject is a literal (can't cast to term.Subject)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/sparql-results+xml")
		w.Write([]byte(`<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
<head><variable name="s"/><variable name="p"/><variable name="o"/></head>
<results>
<result>
<binding name="s"><literal>not-a-subject</literal></binding>
<binding name="p"><uri>http://example.org/p</uri></binding>
<binding name="o"><literal>val</literal></binding>
</result>
</results>
</sparql>`))
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	var count int
	for range remote.Triples(t.Context(), term.TriplePattern{}, nil) {
		count++
	}
	// Should skip the malformed row
	if count != 0 {
		t.Errorf("expected 0 triples from malformed result, got %d", count)
	}
}

func TestLenNonLiteralCount(t *testing.T) {
	// Server returns a non-literal for COUNT result
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/sparql-results+xml")
		w.Write([]byte(`<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
<head><variable name="c"/></head>
<results>
<result>
<binding name="c"><uri>http://example.org/not-a-number</uri></binding>
</result>
</results>
</sparql>`))
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	if n, _ := remote.Len(t.Context(), nil); n != 0 {
		t.Errorf("Len with non-literal count = %d, want 0", n)
	}
}

func TestLenMissingCountVar(t *testing.T) {
	// Server returns a result without the "c" variable
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/sparql-results+xml")
		w.Write([]byte(`<?xml version="1.0"?>
<sparql xmlns="http://www.w3.org/2005/sparql-results#">
<head><variable name="x"/></head>
<results>
<result>
<binding name="x"><literal datatype="http://www.w3.org/2001/XMLSchema#integer">5</literal></binding>
</result>
</results>
</sparql>`))
	}))
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(ts.URL)
	if n, _ := remote.Len(t.Context(), nil); n != 0 {
		t.Errorf("Len with missing count var = %d, want 0", n)
	}
}

func TestAddNWithGraphContext(t *testing.T) {
	// Tests termKeyOrDefault with non-nil Graph context
	g := graph.NewGraph()
	ds := &sparql.Dataset{
		Default:     g,
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
	srv := sparqlstore.NewServer(ds)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	remote := sparqlstore.New(
		ts.URL+"/query",
		sparqlstore.WithUpdate(ts.URL+"/update"),
	)

	graphURI := term.NewURIRefUnsafe("http://example.org/graph1")
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")

	quads := []term.Quad{
		{Triple: term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("a")}, Graph: graphURI},
		{Triple: term.Triple{Subject: s, Predicate: p, Object: rdflibgo.NewLiteral("b")}, Graph: graphURI},
	}
	remote.AddN(t.Context(), quads)

	// Verify named graph was created
	ng, ok := ds.NamedGraphs["http://example.org/graph1"]
	if !ok {
		t.Fatal("expected named graph to be created")
	}
	if mustV(ng.Len(t.Context())) != 2 {
		t.Errorf("named graph len = %d, want 2", mustV(ng.Len(t.Context())))
	}
}
