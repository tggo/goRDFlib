package endpoint_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store/sqlitestore"
	"github.com/tggo/goRDFlib/term"
)

// NewForStore keeps every graph in the dataset's store, including graphs an
// update creates, which the engine builds in memory first.
func TestStoreBackend(t *testing.T) {
	st, err := sqlitestore.New(sqlitestore.WithFile(t.TempDir() + "/db.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ds := graph.NewDataset(graph.WithStore(st))
	ds.DefaultContext().Add(iri(ex+"s"), iri(ex+"p"), term.NewLiteral("default"))
	ds.Graph(iri(ex+"g1")).Add(iri(ex+"s1"), iri(ex+"p"), term.NewLiteral("one"))
	ts := httptestServer(t, endpoint.NewForStore(ds))

	inStore := func(g string) int { return st.Len(iri(g)) }

	if !askResult(t, get(t, ts, url.Values{"query": {`ASK { GRAPH <` + ex + `g1> { ?s ?p "one" } ?x ?y "default" }`}})) {
		t.Fatal("store graphs are not visible to queries")
	}
	wantStatus(t, postUpdate(t, ts, `INSERT DATA { GRAPH <`+ex+`g2> { <a:a> <a:b> "new" } }`), http.StatusNoContent)
	if inStore(ex+"g2") != 1 {
		t.Fatalf("a graph created by INSERT DATA is not in the store (%d triples)", inStore(ex+"g2"))
	}
	wantStatus(t, postUpdate(t, ts, `COPY <`+ex+`g1> TO <`+ex+`g3> ; MOVE <`+ex+`g2> TO <`+ex+`g4>`), http.StatusNoContent)
	if inStore(ex+"g3") != 1 || inStore(ex+"g4") != 1 || inStore(ex+"g2") != 0 {
		t.Fatalf("COPY/MOVE: g3 %d, g4 %d, g2 %d", inStore(ex+"g3"), inStore(ex+"g4"), inStore(ex+"g2"))
	}
	wantStatus(t, do(t, newReq(t, "GET", graphURL(ts, ex+"g2"), "", "")), http.StatusNotFound)

	// Graph Store writes.
	wantStatus(t, do(t, newReq(t, "PUT", graphURL(ts, ex+"g5"), "text/turtle", `<a:a> <a:b> 1, 2 .`)), http.StatusCreated)
	if inStore(ex+"g5") != 2 {
		t.Fatalf("PUT: %d triples in the store", inStore(ex+"g5"))
	}
	r := do(t, newReq(t, "GET", graphURL(ts, ex+"g5"), "", "", "Accept", "application/n-triples"))
	wantStatus(t, r, http.StatusOK)
	if strings.Count(r.body, "\n") != 2 {
		t.Fatalf("GET: %s", r.body)
	}
	wantStatus(t, do(t, newReq(t, "DELETE", graphURL(ts, ex+"g5"), "", "")), http.StatusOK)
	if inStore(ex+"g5") != 0 {
		t.Fatal("DELETE left triples in the store")
	}
	wantStatus(t, do(t, newReq(t, "DELETE", graphURL(ts, ex+"g5"), "", "")), http.StatusNotFound)

	// Named graphs are the store's IRI contexts.
	res := parseResult(t, get(t, ts, url.Values{"query": {`SELECT DISTINCT ?g WHERE { GRAPH ?g { ?s ?p ?o } } ORDER BY ?g`}}))
	var names []string
	for _, b := range res.Bindings {
		names = append(names, b["g"].String())
	}
	if strings.Join(names, " ") != ex+"g1 "+ex+"g3 "+ex+"g4" {
		t.Fatalf("named graphs = %v", names)
	}
}
