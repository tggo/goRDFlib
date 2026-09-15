package endpoint_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

const ex = "http://example.org/"

func iri(s string) term.URIRef { return term.NewURIRefUnsafe(s) }

// fixture is a dataset with one triple in the default graph and one named
// graph with one triple.
func fixture() *sparql.Dataset {
	def := graph.NewGraph()
	def.Add(iri(ex+"s"), iri(ex+"p"), term.NewLiteral("default"))
	g1 := graph.NewGraph()
	g1.Add(iri(ex+"s1"), iri(ex+"p"), term.NewLiteral("one"))
	return &sparql.Dataset{Default: def, NamedGraphs: map[string]*graph.Graph{ex + "g1": g1}}
}

func serve(t testing.TB, ds *sparql.Dataset, opts ...endpoint.Option) (*httptest.Server, *endpoint.Handler) {
	t.Helper()
	h := endpoint.New(ds, opts...)
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts, h
}

type response struct {
	status int
	header http.Header
	body   string
}

func do(t testing.TB, req *http.Request) response {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", req.Method, req.URL, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return response{status: resp.StatusCode, header: resp.Header, body: string(b)}
}

func newReq(t testing.TB, method, u, contentType, body string, headers ...string) *http.Request {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, u, r)
	if err != nil {
		t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	return req
}

func get(t testing.TB, base string, params url.Values, headers ...string) response {
	t.Helper()
	u := base
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	return do(t, newReq(t, http.MethodGet, u, "", "", headers...))
}

func postQuery(t testing.TB, u, query string, headers ...string) response {
	t.Helper()
	return do(t, newReq(t, http.MethodPost, u, "application/sparql-query", query, headers...))
}

func postUpdate(t testing.TB, u, update string, headers ...string) response {
	t.Helper()
	return do(t, newReq(t, http.MethodPost, u, "application/sparql-update", update, headers...))
}

func postForm(t testing.TB, u string, form url.Values, headers ...string) response {
	t.Helper()
	return do(t, newReq(t, http.MethodPost, u, "application/x-www-form-urlencoded", form.Encode(), headers...))
}

func wantStatus(t testing.TB, r response, status int) {
	t.Helper()
	if r.status != status {
		t.Fatalf("status = %d, want %d; body: %s", r.status, status, r.body)
	}
}

// parseResult reads a SPARQL JSON or XML result body.
func parseResult(t testing.TB, r response) *sparql.Result {
	t.Helper()
	ct := r.header.Get("Content-Type")
	var res *sparql.Result
	var err error
	switch {
	case strings.HasPrefix(ct, "application/sparql-results+json"):
		res, err = sparql.ParseSRJ(strings.NewReader(r.body))
	case strings.HasPrefix(ct, "application/sparql-results+xml"):
		res, err = sparql.ParseSRX(strings.NewReader(r.body))
	default:
		t.Fatalf("not a SPARQL result: Content-Type %q, body %s", ct, r.body)
	}
	if err != nil {
		t.Fatalf("parse result: %v\n%s", err, r.body)
	}
	return res
}

func askResult(t testing.TB, r response) bool {
	t.Helper()
	wantStatus(t, r, http.StatusOK)
	res := parseResult(t, r)
	return res.AskResult
}

func httptestServer(t testing.TB, h http.Handler) string {
	t.Helper()
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	return ts.URL
}
