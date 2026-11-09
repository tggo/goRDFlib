package endpoint_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
)

// TestW3CProtocolManifest runs the SPARQL 1.1 Protocol tests
// (sparql11/protocol/manifest.ttl) from the manifest itself: every test is a
// list of ht:Requests with mf:expectedStatus / mf:expectedBoolean /
// mf:expectedFormat, driven by the same engine as the Graph Store suite
// (w3c_gsp_suite_test.go). Each test starts from a fresh dataset holding the
// ut:graphData files as named graphs.
//
// Older rdf-tests checkouts describe these exchanges in prose only, with no
// ht:requests; the hand transcription in TestW3CProtocol covers those, and
// this test skips with that reason rather than reading nothing.

func TestW3CProtocolManifest(t *testing.T) {
	manifest, err := filepath.Abs("../testdata/w3c/rdf-tests/sparql/sparql11/protocol/manifest.ttl")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(manifest); err != nil {
		t.Skipf("W3C test suite not checked out (git submodule update --init): %v", err)
	}
	tests := manifestTests(t, manifest, nsMF+"ProtocolTest")
	if len(tests) == 0 {
		t.Fatal("manifest lists no tests")
	}
	if a, ok := tests[0].node(nsMF + "action"); !ok || len(a.list(nsHT+"requests")) == 0 {
		t.Skip("this rdf-tests pin describes the exchanges in prose, without ht:requests; TestW3CProtocol covers it")
	}
	dir := filepath.Dir(manifest)
	seen := make(map[string]bool, len(tests))
	for _, test := range tests {
		name := gspTestName(test)
		if seen[name] {
			t.Fatalf("test %s listed twice", name)
		}
		seen[name] = true
		t.Run(name, func(t *testing.T) { runProtocolTest(t, gspNode{t, test.g, test.s}, dir) })
	}
}

func runProtocolTest(t *testing.T, test gspNode, dir string) {
	test.onlyKnown("test", nsMF+"name", nsMF+"action",
		"http://www.w3.org/2009/sparql/tests/test-update#graphData",
		"http://www.w3.org/2001/sw/DataAccess/tests/test-dawg#approval",
		"http://www.w3.org/2001/sw/DataAccess/tests/test-dawg#approvedBy")
	if types := test.objects(nsRDF + "type"); len(types) != 1 || types[0].(term.URIRef).Value() != nsMF+"ProtocolTest" {
		t.Fatalf("%s: type is %v, want mf:ProtocolTest", test.s, types)
	}

	ds := &sparql.Dataset{Default: graph.NewGraph(), NamedGraphs: map[string]*graph.Graph{}}
	for _, gd := range test.objects("http://www.w3.org/2009/sparql/tests/test-update#graphData") {
		node := gspNode{t, test.g, gd.(term.Subject)}
		node.onlyKnown("ut:graphData", "http://www.w3.org/2009/sparql/tests/test-update#graph",
			"http://www.w3.org/2000/01/rdf-schema#label")
		file, ok := node.one("http://www.w3.org/2009/sparql/tests/test-update#graph")
		name, ok2 := node.str("http://www.w3.org/2000/01/rdf-schema#label")
		if !ok || !ok2 {
			t.Fatalf("ut:graphData %s needs ut:graph and rdfs:label", gd)
		}
		path := filepath.Join(dir, filepath.Base(strings.TrimPrefix(file.(term.URIRef).Value(), "file://")))
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		g := graph.NewGraph()
		err = nt.Parse(context.Background(), g, f)
		f.Close()
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		ds.NamedGraphs[name] = g
	}

	conn, _ := test.node(nsMF + "action")
	conn.onlyKnown("connection", nsHT+"connectionAuthority", nsHT+"requests")
	authority, _ := conn.str(nsHT + "connectionAuthority")
	requests := conn.list(nsHT + "requests")
	if authority == "" || len(requests) == 0 {
		t.Fatal("connection needs ht:connectionAuthority and ht:requests")
	}

	h := endpoint.New(ds)
	mux := http.NewServeMux()
	mux.Handle("/sparql", h)
	mux.Handle("/sparql/", h)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	vars := map[string]string{}
	for i, rq := range requests {
		runGSPRequest(t, i, rq, srv.URL, authority, vars, func(s string) string { return s })
	}
}
