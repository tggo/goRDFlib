package endpoint_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

func graphURL(base, g string) string { return base + "?graph=" + url.QueryEscape(g) }

func TestGraphStoreIndirectOnTheEndpoint(t *testing.T) {
	ds := fixture()
	ts, _ := serve(t, ds)
	g := ex + "new"
	const body = `<http://example.org/a> <http://example.org/b> "c" .`

	wantStatus(t, do(t, newReq(t, "GET", graphURL(ts.URL, g), "", "")), http.StatusNotFound)
	wantStatus(t, do(t, newReq(t, "PUT", graphURL(ts.URL, g), "application/n-triples", body)), http.StatusCreated)
	if ds.NamedGraphs[g] == nil || ds.NamedGraphs[g].Len() != 1 {
		t.Fatal("PUT did not create the graph in the dataset")
	}
	wantStatus(t, do(t, newReq(t, "PUT", graphURL(ts.URL, g), "application/n-triples", body)), http.StatusNoContent)
	if ds.NamedGraphs[g].Len() != 1 {
		t.Fatalf("PUT did not replace: %d triples", ds.NamedGraphs[g].Len())
	}
	wantStatus(t, do(t, newReq(t, "POST", graphURL(ts.URL, g), "text/turtle", `<http://example.org/a> <http://example.org/b> "d" .`)), http.StatusOK)
	if ds.NamedGraphs[g].Len() != 2 {
		t.Fatalf("POST did not merge: %d triples", ds.NamedGraphs[g].Len())
	}

	r := do(t, newReq(t, "GET", graphURL(ts.URL, g), "", "", "Accept", "application/n-triples"))
	wantStatus(t, r, http.StatusOK)
	out := graph.NewGraph()
	if err := nt.Parse(out, strings.NewReader(r.body)); err != nil || out.Len() != 2 {
		t.Fatalf("GET body: err %v, %d triples: %s", err, out.Len(), r.body)
	}
	if r.header.Get("Vary") != "Accept" {
		t.Error("GET without Vary: Accept")
	}

	// The graph is visible to queries.
	if !askResult(t, get(t, ts.URL, url.Values{"query": {`ASK { GRAPH <` + g + `> { ?s ?p "d" } }`}})) {
		t.Fatal("a graph written through GSP is not visible to SPARQL")
	}

	wantStatus(t, do(t, newReq(t, "DELETE", graphURL(ts.URL, g), "", "")), http.StatusOK)
	if _, ok := ds.NamedGraphs[g]; ok {
		t.Fatal("DELETE left the graph in the dataset")
	}
	wantStatus(t, do(t, newReq(t, "DELETE", graphURL(ts.URL, g), "", "")), http.StatusNotFound)

	// The default graph.
	r = do(t, newReq(t, "GET", ts.URL+"?default", "", ""))
	wantStatus(t, r, http.StatusOK)
	if !strings.Contains(r.body, "default") {
		t.Fatalf("GET ?default body: %s", r.body)
	}
	wantStatus(t, do(t, newReq(t, "DELETE", ts.URL+"?default", "", "")), http.StatusOK)
	if ds.Default.Len() != 0 {
		t.Fatal("DELETE ?default did not clear the default graph")
	}
	wantStatus(t, do(t, newReq(t, "GET", ts.URL+"?default", "", "")), http.StatusOK)
}

func TestGraphStoreErrors(t *testing.T) {
	ds := fixture()
	ts, _ := serve(t, ds, endpoint.WithMaxGraphBytes(150))
	g1 := graphURL(ts.URL, ex+"g1")
	cases := []struct {
		name   string
		req    *http.Request
		status int
		body   string
	}{
		{"relative graph IRI", newReq(t, "GET", ts.URL+"?graph=g1", "", ""), 400, "absolute"},
		{"both default and graph", newReq(t, "GET", ts.URL+"?default&graph="+url.QueryEscape(ex+"g1"), "", ""), 400, "either"},
		{"two graphs", newReq(t, "GET", g1+"&graph="+url.QueryEscape(ex+"g2"), "", ""), 400, "exactly one"},
		{"blank node graph name", newReq(t, "GET", ts.URL+"?graph=_:b0", "", ""), 400, "blank node"},
		{"unsupported media type", newReq(t, "PUT", g1, "text/html", "<p>"), 415, "text/html"},
		{"dataset media type", newReq(t, "PUT", g1, "application/trig", "{}"), 415, "application/trig"},
		{"non-UTF-8 charset", newReq(t, "PUT", g1, "text/turtle; charset=latin1", "<a:a> <a:b> <a:c> ."), 415, "UTF-8"},
		{"malformed payload", newReq(t, "PUT", g1, "text/turtle", "<a:a> <a:b> ."), 400, "malformed text/turtle"},
		{"body too large", newReq(t, "POST", g1, "application/n-triples", strings.Repeat("<a:a> <a:b> <a:c> .\n", 10)), 413, "WithMaxGraphBytes"},
		{"not acceptable", newReq(t, "GET", g1, "", "", "Accept", "application/sparql-results+json"), 406, "available"},
		{"PATCH", newReq(t, "PATCH", g1, "", ""), 405, "not allowed"},
		{"remote JSON-LD context", newReq(t, "PUT", g1, "application/ld+json", `{"@context":"http://127.0.0.1:1/ctx","@id":"http://example.org/x","http://example.org/p":"v"}`), 400, "remote"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := do(t, c.req)
			wantStatus(t, r, c.status)
			if !strings.Contains(r.body, c.body) {
				t.Errorf("body %q does not mention %q", r.body, c.body)
			}
			if c.status == 405 && r.header.Get("Allow") == "" {
				t.Error("405 without Allow")
			}
		})
	}
	if ds.NamedGraphs[ex+"g1"].Len() != 1 {
		t.Fatalf("a rejected write changed <g1>: %d triples", ds.NamedGraphs[ex+"g1"].Len())
	}
}

func TestGraphStoreOptionsAndMissingContentType(t *testing.T) {
	ds := fixture()
	ts, _ := serve(t, ds)
	r := do(t, newReq(t, "OPTIONS", graphURL(ts.URL, ex+"g1"), "", ""))
	wantStatus(t, r, http.StatusNoContent)
	if !strings.Contains(r.header.Get("Allow"), "PUT") {
		t.Errorf("Allow = %q", r.header.Get("Allow"))
	}
	// GSP §5.3: without a Content-Type the payload is parsed as RDF/XML.
	const rdfXML = `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/x"><ex:p>v</ex:p></rdf:Description></rdf:RDF>`
	wantStatus(t, do(t, newReq(t, "PUT", graphURL(ts.URL, ex+"xml"), "", rdfXML)), http.StatusCreated)
	if ds.NamedGraphs[ex+"xml"].Len() != 1 {
		t.Fatal("RDF/XML payload without Content-Type was not parsed")
	}
	// An empty body creates an empty graph.
	wantStatus(t, do(t, newReq(t, "PUT", graphURL(ts.URL, ex+"empty"), "text/turtle", "")), http.StatusCreated)
	wantStatus(t, do(t, newReq(t, "GET", graphURL(ts.URL, ex+"empty"), "", "")), http.StatusOK)
}

func TestGraphStoreRelativeIRIsInPayload(t *testing.T) {
	ds := fixture()
	ts, _ := serve(t, ds)
	g := ex + "docs/one"
	wantStatus(t, do(t, newReq(t, "PUT", graphURL(ts.URL, g), "text/turtle", `<#it> <p> <other> .`)), http.StatusCreated)
	want := term.Triple{Subject: iri(ex + "docs/one#it"), Predicate: iri(ex + "docs/p"), Object: iri(ex + "docs/other")}
	if !ds.NamedGraphs[g].Contains(want.Subject, want.Predicate, want.Object) {
		t.Fatalf("relative IRIs did not resolve against the graph IRI: %v", triplesOf(ds.NamedGraphs[g]))
	}

	ts2, _ := serve(t, fixture(), endpoint.WithBaseIRI("http://base.example/"))
	r := do(t, newReq(t, "PUT", graphURL(ts2.URL, g), "text/turtle", `<x> <p> <y> .`))
	wantStatus(t, r, http.StatusCreated)
	r = do(t, newReq(t, "GET", graphURL(ts2.URL, g), "", "", "Accept", "application/n-triples"))
	if !strings.Contains(r.body, "<http://base.example/x>") {
		t.Fatalf("WithBaseIRI not used for the payload: %s", r.body)
	}
}

func TestGraphStoreBlankNodesAreMerged(t *testing.T) {
	ds := fixture()
	ts, _ := serve(t, ds)
	g := graphURL(ts.URL, ex+"b")
	const body = `_:x <http://example.org/p> "v" .`
	wantStatus(t, do(t, newReq(t, "POST", g, "application/n-triples", body)), http.StatusCreated)
	wantStatus(t, do(t, newReq(t, "POST", g, "application/n-triples", body)), http.StatusOK)
	// An RDF merge keeps the blank nodes of two documents apart.
	if n := ds.NamedGraphs[ex+"b"].Len(); n != 2 {
		t.Fatalf("%d triples after posting the same blank-node document twice, want 2", n)
	}
}

func TestGraphStoreHandlerRoot(t *testing.T) {
	h := endpoint.New(&sparql.Dataset{})
	mux := http.NewServeMux()
	mux.Handle("/gs/", http.StripPrefix("/gs", h.GraphStoreHandler()))
	base := httptestServer(t, mux)
	wantStatus(t, do(t, newReq(t, "GET", base+"/gs/", "", "")), http.StatusBadRequest)
	wantStatus(t, do(t, newReq(t, "PUT", base+"/gs/", "text/turtle", "")), http.StatusBadRequest)
	wantStatus(t, do(t, newReq(t, "DELETE", base+"/gs/", "", "")), http.StatusBadRequest)
	// Queries are not served there.
	wantStatus(t, do(t, newReq(t, "POST", base+"/gs/x", "application/sparql-query", "ASK {}")), http.StatusUnsupportedMediaType)
}

func TestGraphStoreRequestIRIOption(t *testing.T) {
	ds := &sparql.Dataset{}
	h := endpoint.New(ds, endpoint.WithRequestIRI(func(r *http.Request) string {
		return "https://public.example" + r.URL.Path
	}))
	base := httptestServer(t, h.GraphStoreHandler())
	wantStatus(t, do(t, newReq(t, "PUT", base+"/people/ann", "text/turtle", `<a:a> <a:b> <a:c> .`)), http.StatusCreated)
	if _, ok := ds.NamedGraphs["https://public.example/people/ann"]; !ok {
		t.Fatalf("graphs = %v, want the public IRI", keys(ds.NamedGraphs))
	}
}

func TestWithoutGraphStore(t *testing.T) {
	ts, _ := serve(t, fixture(), endpoint.WithoutGraphStore())
	// ?graph= is now just an unknown parameter of a SPARQL request.
	r := do(t, newReq(t, "GET", graphURL(ts.URL, ex+"g1"), "", ""))
	wantStatus(t, r, http.StatusOK)
	if !strings.Contains(r.body, "sd:Service") {
		t.Fatalf("expected the service description, got %s", r.body)
	}
	wantStatus(t, do(t, newReq(t, "PUT", graphURL(ts.URL, ex+"g1"), "text/turtle", "")), http.StatusMethodNotAllowed)
}

func triplesOf(g *graph.Graph) []term.Triple {
	var out []term.Triple
	for t := range g.Triples(nil, nil, nil) {
		out = append(out, t)
	}
	return out
}

func keys[V any](m map[string]V) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
