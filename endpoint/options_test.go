package endpoint_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

func TestUpdateThreeWays(t *testing.T) {
	ds := fixture()
	ts, _ := serve(t, ds)
	r := postUpdate(t, ts.URL, `INSERT DATA { <http://example.org/a> <http://example.org/b> 1 }`)
	wantStatus(t, r, http.StatusNoContent)
	r = postForm(t, ts.URL, url.Values{"update": {`INSERT DATA { GRAPH <http://example.org/g9> { <http://example.org/a> <http://example.org/b> 2 } }`}})
	wantStatus(t, r, http.StatusNoContent)
	if ds.Default.Len() != 2 || ds.NamedGraphs[ex+"g9"] == nil {
		t.Fatalf("updates not applied: default %d, graphs %v", ds.Default.Len(), keys(ds.NamedGraphs))
	}
}

func TestUpdateErrors(t *testing.T) {
	ts, _ := serve(t, fixture())
	cases := []struct {
		name, update, query string
		status              int
		body                string
	}{
		{"syntax error with position", "INSERT DATA {\n <a:b> <a:c> }", "", 400, "line"},
		{"CLEAR of a non-graph", "CLEAR XYZ", "", 400, "not a graph"},
		{"using-graph-uri with WITH", "WITH <http://example.org/g1> DELETE { ?s ?p ?o } WHERE { ?s ?p ?o }", "?using-graph-uri=" + url.QueryEscape(ex+"g1"), 400, "§2.2.3"},
		{"using-named-graph-uri with USING", "DELETE { ?s ?p ?o } USING <http://example.org/g1> WHERE { ?s ?p ?o }", "?using-named-graph-uri=" + url.QueryEscape(ex+"g1"), 400, "§2.2.3"},
		{"LOAD is disabled", "LOAD <http://169.254.169.254/latest/meta-data/>", "", 403, "WithLoader"},
		{"COPY from a missing graph", "COPY <http://example.org/none> TO DEFAULT", "", 400, "not found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := postUpdate(t, ts.URL+c.query, c.update)
			wantStatus(t, r, c.status)
			if !strings.Contains(r.body, c.body) {
				t.Errorf("body %q does not mention %q", r.body, c.body)
			}
		})
	}
	// LOAD SILENT without a loader is a successful no-op, as the spec says for
	// any failing SILENT operation.
	wantStatus(t, postUpdate(t, ts.URL, "LOAD SILENT <http://example.org/x>"), http.StatusNoContent)
}

type recordingLoader struct {
	mu   sync.Mutex
	uris []string
}

func (l *recordingLoader) Load(_ context.Context, g *graph.Graph, uri string) error {
	l.mu.Lock()
	l.uris = append(l.uris, uri)
	l.mu.Unlock()
	g.Add(iri(uri), iri(ex+"loaded"), term.NewLiteral(true))
	return nil
}

func TestLoadWithLoader(t *testing.T) {
	ds := fixture()
	l := &recordingLoader{}
	// The dataset's own Loader must not be used.
	ds.Loader = &recordingLoader{}
	ts, _ := serve(t, ds, endpoint.WithLoader(l))
	wantStatus(t, postUpdate(t, ts.URL, "LOAD <http://example.org/data> INTO GRAPH <http://example.org/loaded>"), http.StatusNoContent)
	if len(l.uris) != 1 || ds.NamedGraphs[ex+"loaded"] == nil || ds.NamedGraphs[ex+"loaded"].Len() != 1 {
		t.Fatalf("loader calls %v, graphs %v", l.uris, keys(ds.NamedGraphs))
	}
	if len(ds.Loader.(*recordingLoader).uris) != 0 {
		t.Fatal("Dataset.Loader was used")
	}
}

func TestReadOnly(t *testing.T) {
	ds := fixture()
	ts, _ := serve(t, ds, endpoint.WithReadOnly())
	wantStatus(t, postUpdate(t, ts.URL, "CLEAR ALL"), http.StatusForbidden)
	wantStatus(t, postForm(t, ts.URL, url.Values{"update": {"CLEAR ALL"}}), http.StatusForbidden)
	wantStatus(t, do(t, newReq(t, "PUT", graphURL(ts.URL, ex+"g1"), "text/turtle", "")), http.StatusForbidden)
	wantStatus(t, do(t, newReq(t, "POST", graphURL(ts.URL, ex+"g1"), "text/turtle", "")), http.StatusForbidden)
	wantStatus(t, do(t, newReq(t, "DELETE", graphURL(ts.URL, ex+"g1"), "", "")), http.StatusForbidden)
	if ds.Default.Len() != 1 || ds.NamedGraphs[ex+"g1"].Len() != 1 {
		t.Fatal("a read-only endpoint changed the dataset")
	}
	wantStatus(t, get(t, ts.URL, url.Values{"query": {"ASK {}"}}), http.StatusOK)
	wantStatus(t, do(t, newReq(t, "GET", graphURL(ts.URL, ex+"g1"), "", "")), http.StatusOK)
}

func TestAuthorizer(t *testing.T) {
	var mu sync.Mutex
	var seen []endpoint.Operation
	auth := func(r *http.Request, op endpoint.Operation) error {
		mu.Lock()
		seen = append(seen, op)
		mu.Unlock()
		switch r.Header.Get("Authorization") {
		case "":
			return &endpoint.HTTPError{Status: 401, Header: http.Header{"Www-Authenticate": {`Bearer realm="sparql"`}}, Err: errors.New("login required")}
		case "reader":
			if op == endpoint.OpUpdate || op == endpoint.OpGraphWrite {
				return errors.New("readers cannot write")
			}
		case "anon":
			return fmt.Errorf("token expired: %w", endpoint.ErrUnauthenticated)
		}
		return nil
	}
	ds := fixture()
	ts, _ := serve(t, ds, endpoint.WithAuthorizer(auth))

	r := get(t, ts.URL, url.Values{"query": {"ASK {}"}})
	wantStatus(t, r, http.StatusUnauthorized)
	if r.header.Get("WWW-Authenticate") == "" {
		t.Error("HTTPError headers were not sent")
	}
	wantStatus(t, get(t, ts.URL, url.Values{"query": {"ASK {}"}}, "Authorization", "anon"), http.StatusUnauthorized)
	wantStatus(t, get(t, ts.URL, url.Values{"query": {"ASK {}"}}, "Authorization", "reader"), http.StatusOK)
	wantStatus(t, postUpdate(t, ts.URL, "CLEAR ALL", "Authorization", "reader"), http.StatusForbidden)
	wantStatus(t, do(t, newReq(t, "GET", graphURL(ts.URL, ex+"g1"), "", "", "Authorization", "reader")), http.StatusOK)
	wantStatus(t, do(t, newReq(t, "DELETE", graphURL(ts.URL, ex+"g1"), "", "", "Authorization", "reader")), http.StatusForbidden)
	wantStatus(t, get(t, ts.URL, nil, "Authorization", "reader"), http.StatusOK) // service description
	if ds.Default.Len() != 1 || ds.NamedGraphs[ex+"g1"] == nil {
		t.Fatal("a refused request changed the dataset")
	}
	wantStatus(t, postUpdate(t, ts.URL, "CLEAR ALL", "Authorization", "admin"), http.StatusNoContent)

	mu.Lock()
	defer mu.Unlock()
	want := []endpoint.Operation{endpoint.OpQuery, endpoint.OpQuery, endpoint.OpQuery, endpoint.OpUpdate,
		endpoint.OpGraphRead, endpoint.OpGraphWrite, endpoint.OpQuery, endpoint.OpUpdate}
	if fmt.Sprint(seen) != fmt.Sprint(want) {
		t.Fatalf("operations = %v, want %v", seen, want)
	}
}

func TestRequestHook(t *testing.T) {
	var mu sync.Mutex
	var infos []endpoint.RequestInfo
	hook := func(_ *http.Request, info endpoint.RequestInfo) {
		mu.Lock()
		infos = append(infos, info)
		mu.Unlock()
	}
	ds := fixture()
	ds.Default.Add(iri(ex+"s2"), iri(ex+"p"), term.NewLiteral("x"))
	ts, _ := serve(t, ds, endpoint.WithRequestHook(hook))
	get(t, ts.URL, url.Values{"query": {"SELECT * WHERE { ?s ?p ?o }"}})
	get(t, ts.URL, url.Values{"query": {"SELEC"}})
	postUpdate(t, ts.URL, "CLEAR GRAPH <http://example.org/g1>")
	do(t, newReq(t, "PUT", graphURL(ts.URL, ex+"h"), "application/n-triples", "<a:a> <a:b> <a:c> .\n<a:a> <a:b> <a:d> ."))
	do(t, newReq(t, "PATCH", ts.URL, "", ""))

	mu.Lock()
	defer mu.Unlock()
	if len(infos) != 5 {
		t.Fatalf("%d hook calls, want 5", len(infos))
	}
	check := func(i int, op endpoint.Operation, status, rows int, wantErr bool) {
		t.Helper()
		in := infos[i]
		if in.Op != op || in.Status != status || in.Rows != rows || (in.Err != nil) != wantErr || in.Duration <= 0 {
			t.Errorf("call %d = %+v, want op %v status %d rows %d err %v", i, in, op, status, rows, wantErr)
		}
	}
	check(0, endpoint.OpQuery, 200, 2, false)
	if infos[0].Bytes == 0 {
		t.Error("Bytes not counted")
	}
	check(1, endpoint.OpQuery, 400, 0, true)
	check(2, endpoint.OpUpdate, 204, 0, false)
	check(3, endpoint.OpGraphWrite, 201, 2, false)
	check(4, endpoint.OpUnknown, 405, 0, true)
}

func TestRequestBodyLimit(t *testing.T) {
	ts, _ := serve(t, fixture(), endpoint.WithMaxRequestBytes(32))
	big := "ASK { " + strings.Repeat(" ", 64) + "}"
	r := postQuery(t, ts.URL, big)
	wantStatus(t, r, http.StatusRequestEntityTooLarge)
	if !strings.Contains(r.body, "WithMaxRequestBytes") {
		t.Errorf("413 body does not say how to raise the limit: %s", r.body)
	}
	wantStatus(t, postForm(t, ts.URL, url.Values{"query": {big}}), http.StatusRequestEntityTooLarge)
	wantStatus(t, postUpdate(t, ts.URL, "CLEAR ALL ; "+big), http.StatusRequestEntityTooLarge)
	// Without Content-Length the limit still holds.
	req := newReq(t, "POST", ts.URL, "application/sparql-query", "")
	req.Body = readerNoLen{strings.NewReader(big)}
	req.ContentLength = -1
	wantStatus(t, do(t, req), http.StatusRequestEntityTooLarge)
	wantStatus(t, postQuery(t, ts.URL, "ASK {}"), http.StatusOK)
}

type readerNoLen struct{ *strings.Reader }

func (readerNoLen) Close() error { return nil }

func TestMaxResultRows(t *testing.T) {
	ds := fixture()
	for i := range 5 {
		ds.Default.Add(iri(fmt.Sprintf("%sr%d", ex, i)), iri(ex+"q"), term.NewLiteral(i))
	}
	ts, _ := serve(t, ds, endpoint.WithMaxResultRows(5))
	wantStatus(t, get(t, ts.URL, url.Values{"query": {`SELECT * WHERE { ?s <http://example.org/q> ?o }`}}), http.StatusOK)
	r := get(t, ts.URL, url.Values{"query": {`SELECT * WHERE { ?s ?p ?o }`}})
	wantStatus(t, r, http.StatusUnprocessableEntity)
	if !errors.Is(endpoint.ErrTooManyRows, endpoint.ErrTooManyRows) || !strings.Contains(r.body, "LIMIT") {
		t.Errorf("body: %s", r.body)
	}
	// The query's own smaller LIMIT is respected; a larger one is not enough.
	wantStatus(t, get(t, ts.URL, url.Values{"query": {`SELECT * WHERE { ?s ?p ?o } LIMIT 3`}}), http.StatusOK)
	wantStatus(t, get(t, ts.URL, url.Values{"query": {`SELECT * WHERE { ?s ?p ?o } LIMIT 100`}}), http.StatusUnprocessableEntity)
	// OFFSET still applies before the limit check.
	res := parseResult(t, get(t, ts.URL, url.Values{"query": {`SELECT * WHERE { ?s ?p ?o } ORDER BY ?s OFFSET 2`}}))
	if len(res.Bindings) != 4 {
		t.Fatalf("OFFSET 2 of 6 rows gave %d rows", len(res.Bindings))
	}
	wantStatus(t, get(t, ts.URL, url.Values{"query": {`CONSTRUCT WHERE { ?s ?p ?o }`}}), http.StatusUnprocessableEntity)
	wantStatus(t, do(t, newReq(t, "GET", ts.URL+"?default", "", "")), http.StatusUnprocessableEntity)
	wantStatus(t, get(t, ts.URL, url.Values{"query": {`ASK { ?s ?p ?o }`}}), http.StatusOK)
}

func TestPanicIsRecovered(t *testing.T) {
	var logs bytes.Buffer
	var mu sync.Mutex
	logger := slog.New(slog.NewTextHandler(&lockedWriter{w: &logs, mu: &mu}, nil))
	var status int
	h := endpoint.New(fixture(),
		endpoint.WithLogger(logger),
		endpoint.WithAuthorizer(func(r *http.Request, op endpoint.Operation) error {
			if r.Header.Get("X-Boom") != "" {
				panic("secret internal detail")
			}
			return nil
		}),
		endpoint.WithRequestHook(func(_ *http.Request, info endpoint.RequestInfo) { status = info.Status }))
	ts := httptestServer(t, h)
	r := get(t, ts, url.Values{"query": {"ASK {}"}}, "X-Boom", "1")
	wantStatus(t, r, http.StatusInternalServerError)
	if strings.Contains(r.body, "secret") {
		t.Errorf("the panic value leaked to the client: %s", r.body)
	}
	if status != http.StatusInternalServerError {
		t.Errorf("hook status = %d", status)
	}
	mu.Lock()
	if !strings.Contains(logs.String(), "secret internal detail") {
		t.Errorf("the panic was not logged: %s", logs.String())
	}
	mu.Unlock()
	wantStatus(t, get(t, ts, url.Values{"query": {"ASK {}"}}), http.StatusOK)
}

type panickingLoader struct{}

func (panickingLoader) Load(context.Context, *graph.Graph, string) error { panic("loader bug") }

// A panic while the write lock is held must release it, or every later
// request waits forever.
func TestPanicUnderWriteLockReleasesIt(t *testing.T) {
	ts, _ := serve(t, fixture(), endpoint.WithLoader(panickingLoader{}), endpoint.WithQueryTimeout(2*time.Second), endpoint.WithUpdateTimeout(2*time.Second))
	wantStatus(t, postUpdate(t, ts.URL, "LOAD <http://example.org/x>"), http.StatusInternalServerError)
	wantStatus(t, get(t, ts.URL, url.Values{"query": {"ASK {}"}}), http.StatusOK)
	wantStatus(t, postUpdate(t, ts.URL, "CLEAR ALL"), http.StatusNoContent)
}

type lockedWriter struct {
	w  *bytes.Buffer
	mu *sync.Mutex
}

func (l *lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}

func TestSeparateQueryAndUpdateHandlers(t *testing.T) {
	ds := fixture()
	h := endpoint.New(ds)
	mux := http.NewServeMux()
	mux.Handle("/query", h.QueryHandler())
	mux.Handle("/update", h.UpdateHandler())
	base := httptestServer(t, mux)

	wantStatus(t, get(t, base+"/query", url.Values{"query": {"ASK {}"}}), http.StatusOK)
	wantStatus(t, postUpdate(t, base+"/query", "CLEAR ALL"), http.StatusBadRequest)
	wantStatus(t, postUpdate(t, base+"/update", "CLEAR DEFAULT"), http.StatusNoContent)
	r := get(t, base+"/update", nil)
	wantStatus(t, r, http.StatusMethodNotAllowed)
	if r.header.Get("Allow") != "POST" {
		t.Errorf("Allow = %q", r.header.Get("Allow"))
	}
	wantStatus(t, postQuery(t, base+"/update", "ASK {}"), http.StatusBadRequest)
	// Graph Store parameters are not served by either.
	wantStatus(t, do(t, newReq(t, "PUT", graphURL(base+"/query", ex+"g"), "text/turtle", "")), http.StatusMethodNotAllowed)
	if ds.Default.Len() != 0 {
		t.Fatal("CLEAR DEFAULT via /update was not applied")
	}
}

func TestReadAndWriteHelpers(t *testing.T) {
	ds := fixture()
	ts, h := serve(t, ds)
	err := h.Write(context.Background(), func() {
		ds.Default.Add(iri(ex+"w"), iri(ex+"p"), term.NewLiteral("written"))
	})
	if err != nil {
		t.Fatal(err)
	}
	if !askResult(t, get(t, ts.URL, url.Values{"query": {`ASK { ?s ?p "written" }`}})) {
		t.Fatal("a write through Handler.Write is not visible")
	}
	var n int
	if err := h.Read(context.Background(), func() { n = ds.Default.Len() }); err != nil || n != 2 {
		t.Fatalf("Read: err %v, len %d", err, n)
	}
}
