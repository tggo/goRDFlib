package endpoint_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/storetest"
	"github.com/tggo/goRDFlib/term"
)

// failingEndpoint serves a dataset over a store that can be told to fail. It
// holds a default-graph triple and two in a named graph.
func failingEndpoint(t *testing.T) (base string, fs *storetest.FailingStore) {
	t.Helper()
	fs = storetest.NewFailingStore(store.NewMemoryStore())
	ds := graph.NewDataset(graph.WithStore(fs))
	ds.DefaultContext().Add(t.Context(), iri(ex+"s"), iri(ex+"p"), term.NewLiteral("default"))
	g := ds.Graph(iri(ex + "g1"))
	g.Add(t.Context(), iri(ex+"s1"), iri(ex+"p"), term.NewLiteral("one"))
	g.Add(t.Context(), iri(ex+"s2"), iri(ex+"p"), term.NewLiteral("two"))
	return httptestServer(t, endpoint.NewForStore(ds)), fs
}

// A store that fails is a 5xx with the store's error in the body. It is never
// a 200 with an empty or partial document: a client cannot tell that from data.
func TestStoreFailureIsServerError(t *testing.T) {
	cases := []struct {
		name string
		call func(base string) response
	}{
		{"SELECT", func(b string) response {
			return get(t, b, url.Values{"query": {`SELECT * WHERE { ?s ?p ?o }`}})
		}},
		{"ASK", func(b string) response {
			return get(t, b, url.Values{"query": {`ASK { ?s ?p ?o }`}})
		}},
		{"CONSTRUCT", func(b string) response {
			return get(t, b, url.Values{"query": {`CONSTRUCT { ?s ?p ?o } WHERE { ?s ?p ?o }`}})
		}},
		{"SELECT in a named graph", func(b string) response {
			return get(t, b, url.Values{"query": {`SELECT * WHERE { GRAPH <` + ex + `g1> { ?s ?p ?o } }`}})
		}},
		{"update", func(b string) response {
			return postUpdate(t, b, `INSERT DATA { <a:a> <a:b> "x" }`)
		}},
		{"update with WHERE", func(b string) response {
			return postUpdate(t, b, `DELETE { ?s ?p ?o } WHERE { ?s ?p ?o }`)
		}},
		{"GSP GET default", func(b string) response {
			return do(t, newReq(t, "GET", b+"/?default", "", ""))
		}},
		{"GSP GET named", func(b string) response {
			return do(t, newReq(t, "GET", graphURL(b, ex+"g1"), "", ""))
		}},
		{"GSP PUT", func(b string) response {
			return do(t, newReq(t, "PUT", graphURL(b, ex+"g1"), "text/turtle", `<a:a> <a:b> 1 .`))
		}},
		{"GSP POST", func(b string) response {
			return do(t, newReq(t, "POST", graphURL(b, ex+"g1"), "text/turtle", `<a:a> <a:b> 1 .`))
		}},
		{"GSP DELETE", func(b string) response {
			return do(t, newReq(t, "DELETE", graphURL(b, ex+"g1"), "", ""))
		}},
		{"GSP DELETE default", func(b string) response {
			return do(t, newReq(t, "DELETE", b+"/?default", "", ""))
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			base, fs := failingEndpoint(t)
			fs.Fail(nil)
			r := c.call(base)
			wantStatus(t, r, http.StatusInternalServerError)
			if !strings.Contains(r.body, storetest.ErrInjected.Error()) {
				t.Fatalf("the response does not carry the store's error: %q", r.body)
			}
		})
	}
}

// A read that fails after some triples were produced must still end in an
// error status, never a complete-looking document with the rest missing.
func TestStoreFailsMidRead(t *testing.T) {
	for _, accept := range []string{"text/turtle", "application/n-triples", "application/rdf+xml", "application/ld+json"} {
		t.Run(accept, func(t *testing.T) {
			base, fs := failingEndpoint(t)
			fs.FailReadsAfter(1, nil)
			r := do(t, newReq(t, "GET", graphURL(base, ex+"g1"), "", "", "Accept", accept))
			wantStatus(t, r, http.StatusInternalServerError)
			if strings.Contains(r.body, "one") && strings.Contains(r.body, "two") {
				t.Fatalf("the error response carries the document: %q", r.body)
			}
		})
	}
}

// The same holds when the graph is handed over directly (NewForGraph) instead
// of through a dataset in one store.
func TestStoreFailureWithGraphBackend(t *testing.T) {
	fs := storetest.NewFailingStore(store.NewMemoryStore())
	g := graph.NewGraph(graph.WithStore(fs))
	g.Add(t.Context(), iri(ex+"s"), iri(ex+"p"), term.NewLiteral("v"))
	base := httptestServer(t, endpoint.NewForGraph(g))
	fs.Fail(nil)

	wantStatus(t, get(t, base, url.Values{"query": {`SELECT * WHERE { ?s ?p ?o }`}}), http.StatusInternalServerError)
	wantStatus(t, postUpdate(t, base, `INSERT DATA { <a:a> <a:b> "x" }`), http.StatusInternalServerError)
	wantStatus(t, do(t, newReq(t, "GET", base+"/?default", "", "")), http.StatusInternalServerError)
	wantStatus(t, do(t, newReq(t, "PUT", base+"/?default", "text/turtle", `<a:a> <a:b> 1 .`)), http.StatusInternalServerError)
}
