package sparqlstore_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/sparqlstore"
	"github.com/tggo/goRDFlib/term"
)

func newTestDataset(t *testing.T) (*httptest.Server, *sparql.Dataset) {
	t.Helper()
	g := graph.NewGraph()
	g.Add(
		term.NewURIRefUnsafe("http://example.org/s"),
		term.NewURIRefUnsafe("http://example.org/p"),
		rdflibgo.NewLiteral("hello"),
	)
	ds := &sparql.Dataset{
		Default:     g,
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}
	srv := sparqlstore.NewServer(ds)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, ds
}

func TestQueryGET(t *testing.T) {
	ts, _ := newTestDataset(t)

	resp, err := http.Get(ts.URL + "/query?query=" + url.QueryEscape("SELECT ?o WHERE { ?s ?p ?o }"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "sparql-results") {
		t.Errorf("unexpected content type: %s", ct)
	}
	result, err := sparql.ParseSRX(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 {
		t.Errorf("expected 1 binding, got %d", len(result.Bindings))
	}
}

func TestQueryPOSTForm(t *testing.T) {
	ts, _ := newTestDataset(t)

	resp, err := http.PostForm(ts.URL+"/query", url.Values{
		"query": {"ASK { ?s ?p ?o }"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	result, err := sparql.ParseSRX(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !result.AskResult {
		t.Error("ASK should be true")
	}
}

func TestQueryPOSTDirect(t *testing.T) {
	ts, _ := newTestDataset(t)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/query",
		strings.NewReader("SELECT ?s ?p ?o WHERE { ?s ?p ?o }"))
	req.Header.Set("Content-Type", "application/sparql-query")
	req.Header.Set("Accept", "application/sparql-results+xml")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	result, err := sparql.ParseSRX(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 {
		t.Errorf("expected 1 binding, got %d", len(result.Bindings))
	}
}

func TestQueryBadMethod(t *testing.T) {
	ts, _ := newTestDataset(t)

	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/query", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestQueryBadContentType(t *testing.T) {
	ts, _ := newTestDataset(t)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/query",
		strings.NewReader("SELECT * WHERE { ?s ?p ?o }"))
	req.Header.Set("Content-Type", "text/plain")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnsupportedMediaType)
	}
}

func TestUpdatePOSTDirect(t *testing.T) {
	ts, ds := newTestDataset(t)

	body := `INSERT DATA { <http://example.org/s2> <http://example.org/p2> "world" }`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/update", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/sparql-update")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	if ds.Default.Len() != 2 {
		t.Errorf("default graph len = %d, want 2", ds.Default.Len())
	}
}

func TestUpdatePOSTForm(t *testing.T) {
	ts, ds := newTestDataset(t)

	resp, err := http.PostForm(ts.URL+"/update", url.Values{
		"update": {`INSERT DATA { <http://example.org/s3> <http://example.org/p3> "form" }`},
	})
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}

	if ds.Default.Len() != 2 {
		t.Errorf("default graph len = %d, want 2", ds.Default.Len())
	}
}

func TestUpdateBadMethod(t *testing.T) {
	ts, _ := newTestDataset(t)

	resp, err := http.Get(ts.URL + "/update")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestUpdateBadContentType(t *testing.T) {
	ts, _ := newTestDataset(t)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/update",
		strings.NewReader("INSERT DATA {}"))
	req.Header.Set("Content-Type", "text/plain")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusUnsupportedMediaType)
	}
}

func TestQuerySyntaxError(t *testing.T) {
	ts, _ := newTestDataset(t)

	resp, err := http.Get(ts.URL + "/query?query=" + url.QueryEscape("SELEC ?x"))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestUpdateSyntaxError(t *testing.T) {
	ts, _ := newTestDataset(t)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/update",
		strings.NewReader("INSER DATA {}"))
	req.Header.Set("Content-Type", "application/sparql-update")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestQueryJSONAccept(t *testing.T) {
	ts, _ := newTestDataset(t)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/query?query="+url.QueryEscape("SELECT ?o WHERE { ?s ?p ?o }"), nil)
	req.Header.Set("Accept", "application/sparql-results+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "json") {
		t.Errorf("content type = %s, want json", ct)
	}
	result, err := sparql.ParseSRJ(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 {
		t.Errorf("expected 1 binding, got %d", len(result.Bindings))
	}
}

func TestNewServerWithGraph(t *testing.T) {
	g := graph.NewGraph()
	g.Add(
		term.NewURIRefUnsafe("http://example.org/s"),
		term.NewURIRefUnsafe("http://example.org/p"),
		rdflibgo.NewLiteral("test"),
	)
	srv := sparqlstore.NewServerWithGraph(g)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/query?query=" + url.QueryEscape("SELECT ?o WHERE { ?s ?p ?o }"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	result, err := sparql.ParseSRX(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 {
		t.Errorf("expected 1 binding, got %d", len(result.Bindings))
	}
}

func TestConstructQuery(t *testing.T) {
	ts, _ := newTestDataset(t)

	query := `CONSTRUCT { ?s ?p ?o } WHERE { ?s ?p ?o }`
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/query?query="+url.QueryEscape(query), nil)
	req.Header.Set("Accept", "application/n-triples")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "n-triples") {
		t.Errorf("content type = %s, want application/n-triples", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "http://example.org/s") {
		t.Errorf("CONSTRUCT result should contain the triple, got: %s", body)
	}
}

func TestASKFalseResult(t *testing.T) {
	ts, _ := newTestDataset(t)

	resp, err := http.Get(ts.URL + "/query?query=" + url.QueryEscape("ASK { <http://nothing> <http://nothing> <http://nothing> }"))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	result, err := sparql.ParseSRX(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if result.AskResult {
		t.Error("ASK should be false")
	}
}

func TestProtocolDataset(t *testing.T) {
	defaultG := graph.NewGraph()
	namedG := graph.NewGraph()
	namedG.Add(
		term.NewURIRefUnsafe("http://example.org/ns"),
		term.NewURIRefUnsafe("http://example.org/np"),
		rdflibgo.NewLiteral("from-named"),
	)

	ds := &sparql.Dataset{
		Default: defaultG,
		NamedGraphs: map[string]*rdflibgo.Graph{
			"http://example.org/g1": namedG,
		},
	}
	srv := sparqlstore.NewServer(ds)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	q := "SELECT ?o WHERE { ?s ?p ?o }"
	u := ts.URL + "/query?query=" + url.QueryEscape(q) + "&default-graph-uri=" + url.QueryEscape("http://example.org/g1")
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	result, err := sparql.ParseSRX(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 {
		t.Errorf("expected 1 binding from protocol dataset, got %d", len(result.Bindings))
	}
}

func TestProtocolDatasetNamedGraph(t *testing.T) {
	namedG := graph.NewGraph()
	namedG.Add(
		term.NewURIRefUnsafe("http://example.org/ns"),
		term.NewURIRefUnsafe("http://example.org/np"),
		rdflibgo.NewLiteral("named-val"),
	)

	ds := &sparql.Dataset{
		Default: graph.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{
			"http://example.org/g1": namedG,
		},
	}
	srv := sparqlstore.NewServer(ds)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	q := "SELECT ?o WHERE { GRAPH <http://example.org/g1> { ?s ?p ?o } }"
	u := ts.URL + "/query?query=" + url.QueryEscape(q) + "&named-graph-uri=" + url.QueryEscape("http://example.org/g1")
	resp, err := http.Get(u)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
}

func TestWriteNTriples(t *testing.T) {
	g := graph.NewGraph()
	g.Add(
		term.NewURIRefUnsafe("http://example.org/s"),
		term.NewURIRefUnsafe("http://example.org/p"),
		rdflibgo.NewLiteral("hello"),
	)
	var buf bytes.Buffer
	err := sparqlstore.WriteNTriples(g, &buf)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "http://example.org/s") {
		t.Errorf("output should contain the subject, got: %s", buf.String())
	}
}

// A GET without a query is answered with the service description.
func TestQueryMissingParameter(t *testing.T) {
	ts, _ := newTestDataset(t)

	resp, err := http.Get(ts.URL + "/query")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "sd:Service") {
		t.Errorf("status = %d, body %s; want the service description", resp.StatusCode, body)
	}
}

func TestUpdateMissingBody(t *testing.T) {
	ts, _ := newTestDataset(t)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/update", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/sparql-update")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestQueryWithBNodeAndTypedLiteralResults(t *testing.T) {
	g := graph.NewGraph()
	bnode := term.NewBNode()
	g.Add(bnode, term.NewURIRefUnsafe("http://example.org/p"), rdflibgo.NewLiteral(42))
	g.Add(bnode, term.NewURIRefUnsafe("http://example.org/q"), rdflibgo.NewLiteral("text", rdflibgo.WithLang("en")))

	srv := sparqlstore.NewServerWithGraph(g)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	q := "SELECT ?s ?p ?o WHERE { ?s ?p ?o }"
	resp, err := http.Get(ts.URL + "/query?query=" + url.QueryEscape(q))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	result, err := sparql.ParseSRX(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 2 {
		t.Errorf("expected 2 bindings, got %d", len(result.Bindings))
	}

	// Test JSON format
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/query?query="+url.QueryEscape(q), nil)
	req.Header.Set("Accept", "application/sparql-results+json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	result2, err := sparql.ParseSRJ(resp2.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(result2.Bindings) != 2 {
		t.Errorf("expected 2 bindings (JSON), got %d", len(result2.Bindings))
	}
}

func TestPluginRegistration(t *testing.T) {
	s, ok := rdflibgo.GetStore("sparql")
	if !ok {
		t.Fatal("store 'sparql' not registered")
	}
	if s == nil {
		t.Fatal("GetStore returned nil")
	}
}

func TestQueryFormDatasetParams(t *testing.T) {
	namedG := graph.NewGraph()
	namedG.Add(
		term.NewURIRefUnsafe("http://example.org/ns"),
		term.NewURIRefUnsafe("http://example.org/np"),
		rdflibgo.NewLiteral("form-ds"),
	)
	ds := &sparql.Dataset{
		Default: graph.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{
			"http://example.org/g1": namedG,
		},
	}
	srv := sparqlstore.NewServer(ds)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	resp, err := http.PostForm(ts.URL+"/query", url.Values{
		"query":             {"SELECT ?o WHERE { ?s ?p ?o }"},
		"default-graph-uri": {"http://example.org/g1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
	result, err := sparql.ParseSRX(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 {
		t.Errorf("expected 1 binding from form dataset params, got %d", len(result.Bindings))
	}
}

func TestConstructEmptyResult(t *testing.T) {
	g := graph.NewGraph()
	srv := sparqlstore.NewServerWithGraph(g)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	query := `CONSTRUCT { ?s ?p ?o } WHERE { ?s ?p ?o }`
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/query?query="+url.QueryEscape(query), nil)
	req.Header.Set("Accept", "application/n-triples")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "n-triples") {
		t.Errorf("content type = %s, want n-triples", ct)
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}

func TestQueryDirectBodyReadError(t *testing.T) {
	g := graph.NewGraph()
	srv := sparqlstore.NewServerWithGraph(g)
	handler := srv.QueryHandler()

	// Use httptest.NewRequest + ResponseRecorder to bypass HTTP transport
	req := httptest.NewRequest(http.MethodPost, "/query", errReader{})
	req.Header.Set("Content-Type", "application/sparql-query")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateDirectBodyReadError(t *testing.T) {
	g := graph.NewGraph()
	srv := sparqlstore.NewServerWithGraph(g)
	handler := srv.UpdateHandler()

	req := httptest.NewRequest(http.MethodPost, "/update", errReader{})
	req.Header.Set("Content-Type", "application/sparql-update")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestQueryFormParseError(t *testing.T) {
	g := graph.NewGraph()
	srv := sparqlstore.NewServerWithGraph(g)
	handler := srv.QueryHandler()

	// Body that errors on read — form parsing will fail
	req := httptest.NewRequest(http.MethodPost, "/query", errReader{})
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateFormParseError(t *testing.T) {
	g := graph.NewGraph()
	srv := sparqlstore.NewServerWithGraph(g)
	handler := srv.UpdateHandler()

	req := httptest.NewRequest(http.MethodPost, "/update", errReader{})
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestASKJSONTrue(t *testing.T) {
	ts, _ := newTestDataset(t)

	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/query?query="+url.QueryEscape("ASK { ?s ?p ?o }"), nil)
	req.Header.Set("Accept", "application/sparql-results+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	result, err := sparql.ParseSRJ(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !result.AskResult {
		t.Error("ASK JSON should be true")
	}
}

func TestASKJSONFalse(t *testing.T) {
	ts, _ := newTestDataset(t)

	req, _ := http.NewRequest(http.MethodGet,
		ts.URL+"/query?query="+url.QueryEscape("ASK { <http://nothing> <http://nothing> <http://nothing> }"), nil)
	req.Header.Set("Accept", "application/sparql-results+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	result, err := sparql.ParseSRJ(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if result.AskResult {
		t.Error("ASK JSON should be false")
	}
}

func TestDirLangLiteralXMLAndJSON(t *testing.T) {
	g := graph.NewGraph()
	g.Add(
		term.NewURIRefUnsafe("http://example.org/s"),
		term.NewURIRefUnsafe("http://example.org/p"),
		rdflibgo.NewLiteral("مرحبا", rdflibgo.WithLang("ar"), rdflibgo.WithDir("rtl")),
	)

	srv := sparqlstore.NewServerWithGraph(g)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	q := "SELECT ?o WHERE { ?s ?p ?o }"

	// XML
	resp, err := http.Get(ts.URL + "/query?query=" + url.QueryEscape(q))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	// SPARQL 1.2 results: the direction is an its:dir attribute.
	if !strings.Contains(string(body), `xml:lang="ar" its:dir="rtl"`) {
		t.Errorf("XML should carry xml:lang and its:dir, got: %s", body)
	}
	res, err := sparql.ParseSRX(strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	if lit, ok := res.Bindings[0]["o"].(term.Literal); !ok || lit.Language() != "ar" || lit.Dir() != "rtl" {
		t.Errorf("round trip lost the direction: %v", res.Bindings[0]["o"])
	}

	// JSON
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/query?query="+url.QueryEscape(q), nil)
	req.Header.Set("Accept", "application/sparql-results+json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	body2, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(body2), `"xml:lang":"ar","its:dir":"rtl"`) {
		t.Errorf("JSON should carry xml:lang and its:dir, got: %s", body2)
	}
}

func TestOPTIONALNullBindingXML(t *testing.T) {
	g := graph.NewGraph()
	g.Add(
		term.NewURIRefUnsafe("http://example.org/s1"),
		term.NewURIRefUnsafe("http://example.org/p"),
		rdflibgo.NewLiteral("val"),
	)

	srv := sparqlstore.NewServerWithGraph(g)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	// OPTIONAL produces null binding for ?label
	q := "SELECT ?s ?label WHERE { ?s ?p ?o . OPTIONAL { ?s <http://example.org/label> ?label } }"
	resp, err := http.Get(ts.URL + "/query?query=" + url.QueryEscape(q))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	result, err := sparql.ParseSRX(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}
	// ?label should be nil
	if result.Bindings[0]["label"] != nil {
		t.Errorf("expected nil label, got %v", result.Bindings[0]["label"])
	}
}

func TestQueryFormWithNamedGraphURI(t *testing.T) {
	namedG := graph.NewGraph()
	namedG.Add(
		term.NewURIRefUnsafe("http://example.org/ns"),
		term.NewURIRefUnsafe("http://example.org/np"),
		rdflibgo.NewLiteral("ng-form"),
	)
	ds := &sparql.Dataset{
		Default: graph.NewGraph(),
		NamedGraphs: map[string]*rdflibgo.Graph{
			"http://example.org/g1": namedG,
		},
	}
	srv := sparqlstore.NewServer(ds)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	// POST form with named-graph-uri
	resp, err := http.PostForm(ts.URL+"/query", url.Values{
		"query":           {"SELECT ?o WHERE { GRAPH <http://example.org/g1> { ?s ?p ?o } }"},
		"named-graph-uri": {"http://example.org/g1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, body)
	}
}

var _ store.Store = (*sparqlstore.SPARQLStore)(nil)
