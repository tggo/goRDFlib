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
	"github.com/tggo/goRDFlib/sparql/results"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/turtle"
)

// A value the negotiated format cannot carry is found before the first byte
// is sent, even when it is in a row far past the writer's first flush, so the
// client gets an error status instead of a 200 with a truncated document.
func TestResultEncodingFailureBeforeFirstByte(t *testing.T) {
	g := benchGraph(5_000)
	g.Add(iri(ex+"zz"), iri(ex+"p"), term.NewLiteral("control \u0001 char"))
	ts, _ := serve(t, &sparql.Dataset{Default: g})
	q := url.Values{"query": {benchQuery + " ORDER BY ?s"}}
	r := get(t, ts.URL, q, "Accept", "application/sparql-results+xml")
	wantStatus(t, r, http.StatusInternalServerError)
	if ct := r.header.Get("Content-Type"); ct != "text/plain; charset=utf-8" || !strings.Contains(r.body, "Accept") {
		t.Fatalf("Content-Type %q, body %q", ct, r.body)
	}
	// JSON can carry it.
	wantStatus(t, get(t, ts.URL, q, "Accept", "application/sparql-results+json"), http.StatusOK)
}

func TestQueryThreeWays(t *testing.T) {
	ts, _ := serve(t, fixture())
	const q = `SELECT ?o WHERE { ?s ?p ?o }`
	for name, r := range map[string]response{
		"GET":         get(t, ts.URL, url.Values{"query": {q}}),
		"POST form":   postForm(t, ts.URL, url.Values{"query": {q}}),
		"POST direct": postQuery(t, ts.URL, q),
	} {
		t.Run(name, func(t *testing.T) {
			wantStatus(t, r, http.StatusOK)
			res := parseResult(t, r)
			if len(res.Bindings) != 1 || res.Bindings[0]["o"].String() != "default" {
				t.Fatalf("bindings = %v, want the one default-graph literal", res.Bindings)
			}
			if got := r.header.Get("Vary"); got != "Accept" {
				t.Errorf("Vary = %q, want Accept", got)
			}
			if got := r.header.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options = %q", got)
			}
		})
	}
}

func TestQueryNamedGraphsOfTheService(t *testing.T) {
	ts, _ := serve(t, fixture())
	r := get(t, ts.URL, url.Values{"query": {`SELECT ?g ?o WHERE { GRAPH ?g { ?s ?p ?o } }`}})
	wantStatus(t, r, http.StatusOK)
	res := parseResult(t, r)
	if len(res.Bindings) != 1 || res.Bindings[0]["g"].String() != ex+"g1" {
		t.Fatalf("bindings = %v, want one row from <g1>", res.Bindings)
	}
}

// A dataset without named graphs must not evaluate GRAPH patterns against the
// default graph (the engine does that when it is given a nil map).
func TestQueryGraphPatternWithoutNamedGraphs(t *testing.T) {
	ts := httptestServer(t, endpoint.NewForGraph(fixture().Default))
	r := get(t, ts, url.Values{"query": {`ASK { GRAPH ?g { ?s ?p ?o } }`}})
	if askResult(t, r) {
		t.Fatal("GRAPH ?g matched although the dataset has no named graphs")
	}
}

func TestQueryProtocolDataset(t *testing.T) {
	ds := fixture()
	g2 := graph.NewGraph()
	g2.Add(iri(ex+"s2"), iri(ex+"p"), iri(ex+"o2"))
	ds.NamedGraphs[ex+"g2"] = g2
	ts, _ := serve(t, ds)

	t.Run("default-graph-uri replaces the default graph", func(t *testing.T) {
		r := get(t, ts.URL, url.Values{
			"query":             {`SELECT ?s WHERE { ?s ?p ?o }`},
			"default-graph-uri": {ex + "g1"},
		})
		wantStatus(t, r, http.StatusOK)
		res := parseResult(t, r)
		if len(res.Bindings) != 1 || res.Bindings[0]["s"].String() != ex+"s1" {
			t.Fatalf("bindings = %v, want <s1> only", res.Bindings)
		}
	})
	t.Run("several default graphs are merged", func(t *testing.T) {
		r := postForm(t, ts.URL, url.Values{
			"query":             {`ASK { <` + ex + `s1> ?p ?o . <` + ex + `s2> ?q ?r }`},
			"default-graph-uri": {ex + "g1", ex + "g2"},
		})
		if !askResult(t, r) {
			t.Fatal("merged default graph lacks a triple")
		}
	})
	t.Run("named-graph-uri limits GRAPH and empties the default graph", func(t *testing.T) {
		r := postQuery(t, ts.URL+"?named-graph-uri="+url.QueryEscape(ex+"g2"),
			`SELECT ?g (COUNT(*) AS ?n) WHERE { { GRAPH ?g { ?s ?p ?o } } UNION { ?s ?p ?o } } GROUP BY ?g`)
		wantStatus(t, r, http.StatusOK)
		res := parseResult(t, r)
		if len(res.Bindings) != 1 || res.Bindings[0]["g"] == nil || res.Bindings[0]["g"].String() != ex+"g2" {
			t.Fatalf("bindings = %v, want only <g2> and no default-graph rows", res.Bindings)
		}
	})
	t.Run("the service default graph has a name", func(t *testing.T) {
		r := get(t, ts.URL, url.Values{
			"query":           {`ASK { GRAPH <urn:x-rdflib:default> { ?s ?p "default" } }`},
			"named-graph-uri": {"urn:x-rdflib:default"},
		})
		if !askResult(t, r) {
			t.Fatal("urn:x-rdflib:default did not name the service default graph")
		}
	})
	t.Run("unknown graphs are empty", func(t *testing.T) {
		r := get(t, ts.URL, url.Values{
			"query":             {`ASK { ?s ?p ?o }`},
			"default-graph-uri": {"http://nowhere.example/"},
		})
		if askResult(t, r) {
			t.Fatal("an unknown default graph was not empty")
		}
	})
	t.Run("a relative graph IRI is rejected", func(t *testing.T) {
		r := get(t, ts.URL, url.Values{"query": {`ASK {}`}, "default-graph-uri": {"g1"}})
		wantStatus(t, r, http.StatusBadRequest)
	})
	t.Run("FROM in the query selects the dataset", func(t *testing.T) {
		r := get(t, ts.URL, url.Values{"query": {`PREFIX ex: <` + ex + `>
			SELECT ?s FROM ex:g2 WHERE { ?s ?p ?o }`}})
		wantStatus(t, r, http.StatusOK)
		res := parseResult(t, r)
		if len(res.Bindings) != 1 || res.Bindings[0]["s"].String() != ex+"s2" {
			t.Fatalf("bindings = %v, want <s2> from FROM ex:g2", res.Bindings)
		}
	})
	t.Run("FROM NAMED in the query selects the named graphs", func(t *testing.T) {
		r := get(t, ts.URL, url.Values{"query": {`SELECT ?g FROM NAMED <` + ex + `g2> WHERE { GRAPH ?g {} }`}})
		wantStatus(t, r, http.StatusOK)
		res := parseResult(t, r)
		if len(res.Bindings) != 1 || res.Bindings[0]["g"].String() != ex+"g2" {
			t.Fatalf("bindings = %v, want <g2> only", res.Bindings)
		}
	})
	t.Run("protocol parameters override FROM", func(t *testing.T) {
		r := get(t, ts.URL, url.Values{
			"query":             {`ASK FROM <` + ex + `g1> { <` + ex + `s2> ?p ?o }`},
			"default-graph-uri": {ex + "g2"},
		})
		if !askResult(t, r) {
			t.Fatal("FROM won over default-graph-uri")
		}
	})
}

func TestQueryContentNegotiation(t *testing.T) {
	ts, _ := serve(t, fixture())
	sel := url.Values{"query": {`SELECT ?o WHERE { ?s ?p ?o }`}}
	ask := url.Values{"query": {`ASK {}`}}
	cons := url.Values{"query": {`CONSTRUCT { ?s ?p ?o } WHERE { ?s ?p ?o }`}}
	cases := []struct {
		name   string
		params url.Values
		accept string
		status int
		ctype  string
	}{
		{"select default is JSON", sel, "", 200, "application/sparql-results+json"},
		{"select xml", sel, "application/sparql-results+xml", 200, "application/sparql-results+xml"},
		{"select csv", sel, "text/csv", 200, "text/csv; charset=utf-8"},
		{"select tsv", sel, "text/tab-separated-values", 200, "text/tab-separated-values; charset=utf-8"},
		{"select q-values", sel, "application/sparql-results+json;q=0.5, application/sparql-results+xml", 200, "application/sparql-results+xml"},
		{"select alias", sel, "application/xml", 200, "application/sparql-results+xml"},
		{"select browser", sel, "text/html,application/xhtml+xml,*/*;q=0.8", 200, "application/sparql-results+json"},
		{"select not acceptable", sel, "text/turtle", 406, "text/plain; charset=utf-8"},
		{"ask csv not acceptable", ask, "text/csv", 406, "text/plain; charset=utf-8"},
		{"ask csv or xml", ask, "text/csv, application/sparql-results+xml;q=0.1", 200, "application/sparql-results+xml"},
		{"construct default is turtle", cons, "", 200, "text/turtle; charset=utf-8"},
		{"construct n-triples", cons, "application/n-triples", 200, "application/n-triples"},
		{"construct json-ld", cons, "application/ld+json", 200, "application/ld+json"},
		{"construct rdf/xml", cons, "application/rdf+xml", 200, "application/rdf+xml"},
		{"construct trig", cons, "application/trig", 200, "application/trig"},
		{"construct n-quads", cons, "application/n-quads", 200, "application/n-quads"},
		{"construct results type not acceptable", cons, "application/sparql-results+json", 406, "text/plain; charset=utf-8"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var h []string
			if c.accept != "" {
				h = []string{"Accept", c.accept}
			}
			r := get(t, ts.URL, c.params, h...)
			wantStatus(t, r, c.status)
			if got := r.header.Get("Content-Type"); got != c.ctype {
				t.Errorf("Content-Type = %q, want %q", got, c.ctype)
			}
			if c.status == 406 && !strings.Contains(r.body, "available") {
				t.Errorf("406 body does not list the available types: %s", r.body)
			}
		})
	}
}

func TestQueryConstructBodies(t *testing.T) {
	ts, _ := serve(t, fixture())
	q := url.Values{"query": {`CONSTRUCT { ?s ?p ?o } WHERE { ?s ?p ?o }`}}
	r := get(t, ts.URL, q, "Accept", "text/turtle")
	wantStatus(t, r, http.StatusOK)
	g := graph.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(r.body)); err != nil || g.Len() != 1 {
		t.Fatalf("turtle body did not parse to one triple (err %v): %s", err, r.body)
	}
	if r.header.Get("Content-Length") == "" {
		t.Error("a CONSTRUCT response has no Content-Length")
	}
	r = get(t, ts.URL, q, "Accept", "application/n-triples")
	g = graph.NewGraph()
	if err := nt.Parse(g, strings.NewReader(r.body)); err != nil || g.Len() != 1 {
		t.Fatalf("n-triples body did not parse to one triple (err %v): %s", err, r.body)
	}
}

func TestQueryCSVAndTSVBodies(t *testing.T) {
	ts, _ := serve(t, fixture())
	q := url.Values{"query": {`SELECT ?o WHERE { ?s ?p ?o }`}}
	if r := get(t, ts.URL, q, "Accept", "text/csv"); r.body != "o\r\ndefault\r\n" {
		t.Errorf("csv body = %q", r.body)
	}
	if r := get(t, ts.URL, q, "Accept", "text/tab-separated-values"); r.body != "?o\n\"default\"\n" {
		t.Errorf("tsv body = %q", r.body)
	}
}

func TestQueryResultFormatsOption(t *testing.T) {
	ts, _ := serve(t, fixture(), endpoint.WithResultFormats(results.FormatXML, results.FormatJSON))
	r := get(t, ts.URL, url.Values{"query": {`SELECT * {}`}})
	if ct := r.header.Get("Content-Type"); ct != "application/sparql-results+xml" {
		t.Errorf("Content-Type = %q, want the first configured format", ct)
	}
	r = get(t, ts.URL, url.Values{"query": {`SELECT * {}`}}, "Accept", "text/csv")
	wantStatus(t, r, http.StatusNotAcceptable)
}

func TestQueryErrors(t *testing.T) {
	ts, _ := serve(t, fixture())
	cases := []struct {
		name   string
		req    *http.Request
		status int
		body   string
	}{
		{"syntax error has a position", newReq(t, "GET", ts.URL+"?query="+url.QueryEscape("SELECT ?x\nWHERE { ?x ?y }"), "", ""), 400, "line 2"},
		{"describe is not implemented", newReq(t, "GET", ts.URL+"?query="+url.QueryEscape("DESCRIBE <http://example.org/>"), "", ""), 501, "DESCRIBE"},
		{"an update sent as a query", newReq(t, "POST", ts.URL, "application/sparql-query", "INSERT DATA { <a:b> <a:c> <a:d> }"), 400, "update="},
		{"update via GET", newReq(t, "GET", ts.URL+"?update="+url.QueryEscape("CLEAR ALL"), "", ""), 405, "POST"},
		{"PUT without graph", newReq(t, "PUT", ts.URL+"?query=ASK%7B%7D", "", ""), 405, "not allowed"},
		{"PATCH", newReq(t, "PATCH", ts.URL, "", ""), 405, "not allowed"},
		{"two queries", newReq(t, "GET", ts.URL+"?query=ASK%7B%7D&query=ASK%7B%7D", "", ""), 400, "exactly one"},
		{"query in URL and form body", newReq(t, "POST", ts.URL+"?query=ASK%7B%7D", "application/x-www-form-urlencoded", "query=ASK%7B%7D"), 400, "exactly one"},
		{"query and update in one form", newReq(t, "POST", ts.URL, "application/x-www-form-urlencoded", "query=ASK%7B%7D&update=CLEAR%20ALL"), 400, "not both"},
		{"direct query with a query parameter", newReq(t, "POST", ts.URL+"?query=ASK%7B%7D", "application/sparql-query", "ASK {}"), 400, "must not be combined"},
		{"empty query parameter", newReq(t, "GET", ts.URL+"?query=", "", ""), 400, "empty"},
		{"empty body", newReq(t, "POST", ts.URL, "application/sparql-query", "  "), 400, "empty"},
		{"form without query", newReq(t, "POST", ts.URL, "application/x-www-form-urlencoded", "foo=bar"), 400, "missing"},
		{"malformed form", newReq(t, "POST", ts.URL, "application/x-www-form-urlencoded", "query=%zz"), 400, "malformed"},
		{"wrong media type", newReq(t, "POST", ts.URL, "text/plain", "ASK {}"), 415, "text/plain"},
		{"missing media type", newReq(t, "POST", ts.URL, "", "query=ASK%20%7B%7D"), 415, "missing Content-Type"},
		{"utf-16", newReq(t, "POST", ts.URL, "application/sparql-query; charset=UTF-16", "\xff\xfeA\x00S\x00K\x00"), 415, "UTF-8"},
		{"invalid utf-8", newReq(t, "POST", ts.URL, "application/sparql-query", "ASK { <a:\xff> ?p ?o }"), 400, "UTF-8"},
		{"query dataset parameter on an update", newReq(t, "POST", ts.URL+"?default-graph-uri="+url.QueryEscape(ex), "application/sparql-update", "CLEAR ALL"), 400, "does not apply"},
		{"update dataset parameter on a query", newReq(t, "GET", ts.URL+"?query=ASK%7B%7D&using-graph-uri="+url.QueryEscape(ex), "", ""), 400, "does not apply"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := do(t, c.req)
			wantStatus(t, r, c.status)
			if ct := r.header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
				t.Errorf("error Content-Type = %q", ct)
			}
			if !strings.Contains(r.body, c.body) {
				t.Errorf("body %q does not mention %q", r.body, c.body)
			}
			if c.status == 405 && r.header.Get("Allow") == "" {
				t.Error("405 without Allow")
			}
		})
	}
}

func TestQuerySyntaxErrorLineAndColumn(t *testing.T) {
	ts, _ := serve(t, fixture())
	r := postQuery(t, ts.URL, "SELECT ?x\nWHERE { ?x ?y }")
	wantStatus(t, r, http.StatusBadRequest)
	if !strings.Contains(r.body, "at pos") || !strings.Contains(r.body, "(line 2, column") {
		t.Fatalf("body %q lacks the parser position and its line/column", r.body)
	}
}

func TestQueryRelativeIRIsResolveAgainstBase(t *testing.T) {
	ds := fixture()
	ds.Default.Add(iri("http://base.example/dir/thing"), iri(ex+"p"), iri(ex+"o"))
	ts, _ := serve(t, ds, endpoint.WithBaseIRI("http://base.example/dir/"))
	if !askResult(t, get(t, ts.URL, url.Values{"query": {`ASK { <thing> ?p ?o }`}})) {
		t.Fatal("<thing> did not resolve against WithBaseIRI")
	}
	// A BASE in the query wins.
	if askResult(t, get(t, ts.URL, url.Values{"query": {`BASE <http://other.example/> ASK { <thing> ?p ?o }`}})) {
		t.Fatal("the query's BASE was overridden")
	}
}

func TestServiceDescription(t *testing.T) {
	ts, _ := serve(t, fixture())
	r := get(t, ts.URL, nil)
	wantStatus(t, r, http.StatusOK)
	if ct := r.header.Get("Content-Type"); ct != "text/turtle; charset=utf-8" {
		t.Fatalf("Content-Type = %q", ct)
	}
	g := graph.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(r.body)); err != nil {
		t.Fatalf("service description is not Turtle: %v\n%s", err, r.body)
	}
	for _, want := range []string{"SPARQL11Query", "SPARQL11Update", "SPARQL_Results_JSON", "SPARQL_Results_TSV", "formats:Turtle", ts.URL} {
		if !strings.Contains(r.body, want) {
			t.Errorf("service description lacks %s:\n%s", want, r.body)
		}
	}
	if strings.Contains(r.body, "UnionDefaultGraph") {
		t.Error("service description claims sd:UnionDefaultGraph, which is not true")
	}

	r = get(t, ts.URL, nil, "Accept", "application/n-triples")
	if ct := r.header.Get("Content-Type"); ct != "application/n-triples" {
		t.Errorf("negotiated Content-Type = %q", ct)
	}
	wantStatus(t, get(t, ts.URL, nil, "Accept", "image/png"), http.StatusNotAcceptable)

	ro, _ := serve(t, fixture(), endpoint.WithReadOnly())
	if r := get(t, ro.URL, nil); strings.Contains(r.body, "SPARQL11Update") {
		t.Error("a read-only endpoint advertises SPARQL11Update")
	}
}

func TestHeadAndOptions(t *testing.T) {
	ts, _ := serve(t, fixture())
	r := do(t, newReq(t, http.MethodHead, ts.URL+"?query="+url.QueryEscape("SELECT * {}"), "", ""))
	wantStatus(t, r, http.StatusOK)
	if r.body != "" || r.header.Get("Content-Type") != "application/sparql-results+json" {
		t.Errorf("HEAD: body %q, Content-Type %q", r.body, r.header.Get("Content-Type"))
	}
	r = do(t, newReq(t, http.MethodOptions, ts.URL, "", ""))
	wantStatus(t, r, http.StatusNoContent)
	if r.header.Get("Allow") != "GET, HEAD, POST" {
		t.Errorf("Allow = %q", r.header.Get("Allow"))
	}
}
