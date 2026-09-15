package endpoint_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/testutil"
	"github.com/tggo/goRDFlib/turtle"
)

// The W3C Graph Store Protocol tests
// (testdata/w3c/rdf-tests/sparql/sparql11/http-rdf-update/manifest.ttl) form
// one stateful sequence against a graph store at $GRAPHSTORE$, mostly with
// direct graph identification. They run here in manifest order against
// GraphStoreHandler mounted at /rdf-graph-store.
//
// Where the manifest cannot be taken literally, the deviation is noted at the
// entry: "DELETE - existing graph" deletes person/2.ttl, which no earlier entry
// creates, and "PUT - mismatched payload" sends a well-formed Turtle body with
// a Turtle media type, which is not a mismatch; the expected body of "GET of
// POST - existing graph" does not include the triple the preceding POST added;
// and the bodies of the two POST entries end their last triple without the
// '.' Turtle requires, which a conforming parser rejects, so it is added.
func TestW3CGraphStore(t *testing.T) {
	h := endpoint.New(&sparql.Dataset{})
	mux := http.NewServeMux()
	gs := h.GraphStoreHandler()
	mux.Handle("/rdf-graph-store", http.StripPrefix("/rdf-graph-store", gs))
	mux.Handle("/rdf-graph-store/", http.StripPrefix("/rdf-graph-store", gs))
	base := httptestServer(t, mux)
	host := strings.TrimPrefix(base, "http://")
	store := base + "/rdf-graph-store"

	fill := func(s string) string {
		return strings.NewReplacer("$HOST$", host, "$GRAPHSTORE$", "rdf-graph-store").Replace(s)
	}
	const turtleCT = "text/turtle; charset=utf-8"
	const prefixes = "@prefix foaf: <http://xmlns.com/foaf/0.1/> .\n@prefix v: <http://www.w3.org/2006/vcard/ns#> .\n"
	john := prefixes + `<http://$HOST$/$GRAPHSTORE$/person/1> a foaf:Person;
    foaf:businessCard [ a v:VCard; v:fn "John Doe" ].`
	jane := prefixes + `<http://$HOST$/$GRAPHSTORE$/person/1> a foaf:Person;
    foaf:businessCard [ a v:VCard; v:fn "Jane Doe" ].`
	alice := prefixes + `[] a foaf:Person; foaf:businessCard [ a v:VCard; v:given-name "Alice" ] .`

	wantGraph := func(t *testing.T, r response, expected string) {
		t.Helper()
		wantStatus(t, r, http.StatusOK)
		if ct := r.header.Get("Content-Type"); ct != turtleCT {
			t.Fatalf("Content-Type = %q, want %q", ct, turtleCT)
		}
		if r.header.Get("Content-Length") == "" {
			t.Error("no Content-Length")
		}
		exp, act := graph.NewGraph(), graph.NewGraph()
		if err := turtle.Parse(exp, strings.NewReader(fill(expected))); err != nil {
			t.Fatalf("expected graph: %v", err)
		}
		if err := turtle.Parse(act, strings.NewReader(r.body)); err != nil {
			t.Fatalf("response is not Turtle: %v\n%s", err, r.body)
		}
		testutil.AssertGraphEqual(t, exp, act)
	}
	req := func(method, u, ct, body string, headers ...string) response {
		return do(t, newReq(t, method, fill(u), ct, fill(body), headers...))
	}
	var newPath string

	steps := map[string]func(t *testing.T){
		"put__initial_state": func(t *testing.T) {
			wantStatus(t, req("PUT", store+"/person/1.ttl", turtleCT, john), http.StatusCreated)
		},
		"get_of_put__initial_state": func(t *testing.T) {
			wantGraph(t, req("GET", store+"?graph="+url.QueryEscape(fill("http://$HOST$/$GRAPHSTORE$/person/1.ttl")), "", "", "Accept", "text/turtle"), john)
		},
		"put__graph_already_in_store": func(t *testing.T) {
			wantStatus(t, req("PUT", store+"/person/1.ttl", turtleCT, jane), http.StatusNoContent)
		},
		"get_of_put__graph_already_in_store": func(t *testing.T) {
			wantGraph(t, req("GET", store+"/person/1.ttl", "", "", "Accept", "text/turtle"), jane)
		},
		"put__default_graph": func(t *testing.T) {
			wantStatus(t, req("PUT", store+"?default", turtleCT, alice), http.StatusCreated)
		},
		"get_of_put__default_graph": func(t *testing.T) {
			wantGraph(t, req("GET", store+"?default", "", "", "Accept", "text/turtle"), alice)
		},
		"put__mismatched_payload": func(t *testing.T) {
			// Deviation: the payload is Turtle declared as RDF/XML.
			wantStatus(t, req("PUT", store+"/person/1.ttl", "application/rdf+xml", jane), http.StatusBadRequest)
			// And a body that is not Turtle at all under a Turtle media type.
			wantStatus(t, req("PUT", store+"/person/1.ttl", turtleCT, `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"/>`), http.StatusBadRequest)
			// Neither changed the graph.
			wantGraph(t, req("GET", store+"/person/1.ttl", "", ""), jane)
		},
		"delete__existing_graph": func(t *testing.T) {
			// Deviation: person/2.ttl is created first; no manifest entry does.
			wantStatus(t, req("PUT", store+"/person/2.ttl", turtleCT, john), http.StatusCreated)
			wantStatus(t, req("DELETE", store+"/person/2.ttl", "", ""), http.StatusOK)
		},
		"get_of_delete__existing_graph": func(t *testing.T) {
			wantStatus(t, req("GET", store+"/person/2.ttl", "", ""), http.StatusNotFound)
		},
		"delete__nonexistent_graph": func(t *testing.T) {
			wantStatus(t, req("DELETE", store+"/person/2.ttl", "", ""), http.StatusNotFound)
		},
		"post__existing_graph": func(t *testing.T) {
			wantStatus(t, req("POST", store+"/person/1.ttl", turtleCT,
				"@prefix foaf: <http://xmlns.com/foaf/0.1/> .\n<http://$HOST$/$GRAPHSTORE$/person/1> foaf:name \"Jane Doe\" ."), http.StatusOK)
		},
		"get_of_post__existing_graph": func(t *testing.T) {
			// Deviation: the expected graph includes the triple POST added.
			wantGraph(t, req("GET", store+"/person/1.ttl", "", "", "Accept", "text/turtle"),
				jane+"\n<http://$HOST$/$GRAPHSTORE$/person/1> foaf:name \"Jane Doe\" .")
		},
		"post__multipart_formdata": func(t *testing.T) {
			body := "--a6fe4cd636164618814be9f8d3d1a0de\r\n" +
				"Content-Disposition: form-data; name=\"lastName.ttl\"; filename=\"lastName.ttl\"\r\n" +
				"Content-Type: text/turtle; charset=utf-8\r\n\r\n" +
				"@prefix foaf: <http://xmlns.com/foaf/0.1/> .\n<http://$HOST$/$GRAPHSTORE$/person/1> foaf:familyName \"Doe\" .\n\r\n" +
				"--a6fe4cd636164618814be9f8d3d1a0de\r\n" +
				"Content-Disposition: form-data; name=\"firstName.ttl\"; filename=\"firstName.ttl\"\r\n" +
				"Content-Type: text/turtle; charset=utf-8\r\n\r\n" +
				"@prefix foaf: <http://xmlns.com/foaf/0.1/> .\n<http://$HOST$/$GRAPHSTORE$/person/1> foaf:givenName \"Jane\" .\n\r\n" +
				"--a6fe4cd636164618814be9f8d3d1a0de--\r\n"
			wantStatus(t, req("POST", store+"/person/1.ttl", "multipart/form-data; boundary=a6fe4cd636164618814be9f8d3d1a0de", body), http.StatusOK)
		},
		"get_of_post__multipart_formdata": func(t *testing.T) {
			wantGraph(t, req("GET", store+"/person/1.ttl", "", ""), prefixes+`<http://$HOST$/$GRAPHSTORE$/person/1> a foaf:Person;
    foaf:name "Jane Doe"; foaf:givenName "Jane"; foaf:familyName "Doe";
    foaf:businessCard [ a v:VCard; v:fn "Jane Doe" ] .`)
		},
		"post__create__new_graph": func(t *testing.T) {
			r := req("POST", store, turtleCT, alice)
			wantStatus(t, r, http.StatusCreated)
			newPath = r.header.Get("Location")
			if !strings.HasPrefix(newPath, store+"/") {
				t.Fatalf("Location = %q, want a graph under %s", newPath, store)
			}
		},
		"get_of_post__create__new_graph": func(t *testing.T) {
			wantGraph(t, req("GET", newPath, "", "", "Accept", "text/turtle"), alice)
		},
		"get_of_post__after_noop": func(t *testing.T) {
			wantGraph(t, req("GET", newPath, "", "", "Accept", "text/turtle"), alice)
		},
		"head_on_an_existing_graph": func(t *testing.T) {
			r := req("HEAD", store+"/person/1.ttl", "", "")
			wantStatus(t, r, http.StatusOK)
			if ct := r.header.Get("Content-Type"); ct != turtleCT {
				t.Errorf("Content-Type = %q", ct)
			}
			if r.header.Get("Content-Length") == "" || r.body != "" {
				t.Errorf("HEAD: Content-Length %q, body %q", r.header.Get("Content-Length"), r.body)
			}
		},
		"head_on_a_nonexisting_graph": func(t *testing.T) {
			wantStatus(t, req("HEAD", store+"/person/4.ttl", "", ""), http.StatusNotFound)
		},
	}

	entries := manifestEntries(t, "http-rdf-update")
	if len(entries) != len(steps) {
		t.Errorf("%d transcriptions for %d manifest entries", len(steps), len(entries))
	}
	for _, name := range entries {
		fn, ok := steps[name]
		if !ok {
			t.Fatalf("manifest entry %s has no transcription", name)
		}
		if !t.Run(name, fn) {
			t.FailNow() // later steps depend on the state this one leaves
		}
	}
}
