package endpoint_test

import (
	"context"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/endpoint"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/testutil"
	"github.com/tggo/goRDFlib/turtle"
)

// The W3C Graph Store Protocol suite
// (testdata/w3c/rdf-tests/sparql/sparql11/graph-store-protocol) is a set of
// self-contained tests, each one a list of HTTP requests run in order on one
// connection against a fresh store. This runner reads the manifests
// themselves (mf:include -> mf:entries -> mf:action -> ht:requests) and drives
// GraphStoreHandler with them, so a test added upstream runs without a code
// change. It is strict where a manifest could otherwise be skipped silently:
// a property the runner does not understand, an mf:requires feature the
// handler lacks, or a status IRI it cannot map fails the run.
//
// Every request carries the manifest's connectionAuthority as its Host, which
// is what direct graph identification derives the graph IRI from
// (http://www.example/gsp/person/1.ttl), and the handler is mounted at /gsp
// as the manifest says. The ht:body of a response is compared by graph
// isomorphism, not text.

const (
	nsMF  = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#"
	nsHT  = "http://www.w3.org/2011/http#"
	nsHTS = "http://www.w3.org/2011/http-statusCodes#"
	nsCNT = "http://www.w3.org/2011/content#"
	nsRDF = "http://www.w3.org/1999/02/22-rdf-syntax-ns#"
)

// gspFeatures are the mf:requires features GraphStoreHandler supports. A
// feature outside it fails the test instead of skipping it.
var gspFeatures = map[string]bool{
	nsMF + "DirectGraphIdentification":   true,
	nsMF + "IndirectGraphIdentification": true,
	nsMF + "POSTGraphCreation":           true,
}

// gspStatus maps the hts: status IRIs the suite may use to codes; a class is
// a negative hundred (-2 = any 2xx).
var gspStatus = map[string]int{
	"OK": 200, "Created": 201, "Accepted": 202, "NoContent": 204,
	"BadRequest": 400, "NotFound": 404, "MethodNotAllowed": 405,
	"UnsupportedMediaType": 415, "InternalServerError": 500,
	"StatusCode1xx": -1, "StatusCode2xx": -2, "StatusCode3xx": -3,
	"StatusCode4xx": -4, "StatusCode5xx": -5,
}

// gspNode reads the objects of one subject of a manifest graph.
type gspNode struct {
	t *testing.T
	g *graph.Graph
	s term.Subject
}

func (n gspNode) objects(pred string) []term.Term {
	n.t.Helper()
	p := term.NewURIRefUnsafe(pred)
	var out []term.Term
	for o, err := range n.g.Objects(context.Background(), n.s, &p) {
		if err != nil {
			n.t.Fatal(err)
		}
		out = append(out, o)
	}
	return out
}

func (n gspNode) one(pred string) (term.Term, bool) {
	n.t.Helper()
	os := n.objects(pred)
	if len(os) > 1 {
		n.t.Fatalf("%s has %d values for <%s>", n.s, len(os), pred)
	}
	if len(os) == 0 {
		return nil, false
	}
	return os[0], true
}

func (n gspNode) str(pred string) (string, bool) {
	n.t.Helper()
	o, ok := n.one(pred)
	if !ok {
		return "", false
	}
	l, isLit := o.(term.Literal)
	if !isLit {
		n.t.Fatalf("<%s> of %s is not a literal: %s", pred, n.s, o)
	}
	return l.Lexical(), true
}

func (n gspNode) node(pred string) (gspNode, bool) {
	n.t.Helper()
	o, ok := n.one(pred)
	if !ok {
		return gspNode{}, false
	}
	s, isSubj := o.(term.Subject)
	if !isSubj {
		n.t.Fatalf("<%s> of %s is not a node: %s", pred, n.s, o)
	}
	return gspNode{n.t, n.g, s}, true
}

// list returns the members of the rdf:List that pred points at.
func (n gspNode) list(pred string) []gspNode {
	n.t.Helper()
	head, ok := n.node(pred)
	if !ok {
		return nil
	}
	return n.listAt(head.s)
}

func (n gspNode) listAt(head term.Subject) []gspNode {
	n.t.Helper()
	var out []gspNode
	cur := head
	for i := 0; ; i++ {
		if u, ok := cur.(term.URIRef); ok && u.Value() == nsRDF+"nil" {
			return out
		}
		if i > 10000 {
			n.t.Fatal("rdf:List does not end")
		}
		cell := gspNode{n.t, n.g, cur}
		first, ok := cell.one(nsRDF + "first")
		if !ok {
			n.t.Fatalf("list cell %s has no rdf:first", cur)
		}
		s, isSubj := first.(term.Subject)
		if !isSubj {
			n.t.Fatalf("list member %s is not a node", first)
		}
		out = append(out, gspNode{n.t, n.g, s})
		rest, ok := cell.node(nsRDF + "rest")
		if !ok {
			n.t.Fatalf("list cell %s has no rdf:rest", cur)
		}
		cur = rest.s
	}
}

// onlyKnown fails when the node has a predicate outside the allowed set: an
// assertion the runner does not know about must not be dropped.
func (n gspNode) onlyKnown(what string, allowed ...string) {
	n.t.Helper()
	ok := map[string]bool{nsRDF + "type": true}
	for _, a := range allowed {
		ok[a] = true
	}
	for p, err := range n.g.Predicates(context.Background(), n.s, nil) {
		if err != nil {
			n.t.Fatal(err)
		}
		if u := p.(term.URIRef).Value(); !ok[u] {
			n.t.Errorf("%s %s has a property the runner does not handle: <%s>", what, n.s, u)
		}
	}
}

func loadManifestGraph(t *testing.T, path string) *graph.Graph {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Skipf("W3C test suite not checked out (git submodule update --init): %v", err)
	}
	defer f.Close()
	g := graph.NewGraph()
	if err := turtle.Parse(t.Context(), g, f, turtle.WithBase("file://"+filepath.ToSlash(path))); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return g
}

// manifestTests returns the test nodes of manifest path in order, following
// mf:include.
func manifestTests(t *testing.T, path, testType string) []gspNode {
	t.Helper()
	g := loadManifestGraph(t, path)
	var manifest term.Subject
	mp := term.NewURIRefUnsafe(nsRDF + "type")
	for s, err := range g.Subjects(t.Context(), &mp, term.NewURIRefUnsafe(nsMF+"Manifest")) {
		if err != nil {
			t.Fatal(err)
		}
		manifest = s.(term.Subject)
	}
	if manifest == nil {
		t.Fatalf("%s: no mf:Manifest", path)
	}
	m := gspNode{t, g, manifest}
	m.onlyKnown("manifest", nsMF+"entries", nsMF+"include", nsMF+"specVersion",
		"http://www.w3.org/2000/01/rdf-schema#label", "http://www.w3.org/2000/01/rdf-schema#comment")
	var out []gspNode
	for _, inc := range m.list(nsMF + "include") {
		u, ok := inc.s.(term.URIRef)
		if !ok {
			t.Fatalf("mf:include member %s is not an IRI", inc.s)
		}
		out = append(out, manifestTests(t, strings.TrimPrefix(u.Value(), "file://"), testType)...)
	}
	listed := m.list(nsMF + "entries")
	out = append(out, listed...)
	// A test defined in the manifest but missing from mf:entries is still a
	// test: upstream's manifest-direct.ttl defines head_non_existing_direct
	// and never lists it, so an entries-only runner would skip it.
	in := make(map[string]bool, len(listed))
	for _, e := range listed {
		in[e.s.String()] = true
	}
	var unlisted []gspNode
	tp := term.NewURIRefUnsafe(nsRDF + "type")
	for s, err := range g.Subjects(t.Context(), &tp, term.NewURIRefUnsafe(testType)) {
		if err != nil {
			t.Fatal(err)
		}
		if !in[s.String()] {
			unlisted = append(unlisted, gspNode{t, g, s.(term.Subject)})
		}
	}
	sort.Slice(unlisted, func(i, j int) bool { return unlisted[i].s.String() < unlisted[j].s.String() })
	out = append(out, unlisted...)
	return out
}

// gspLocal is the part of an IRI after the last # or /.
func gspLocal(s string) string {
	if i := strings.LastIndexAny(s, "#/"); i >= 0 {
		return s[i+1:]
	}
	return s
}

func gspTestName(n gspNode) string {
	if u, ok := n.s.(term.URIRef); ok {
		return gspLocal(u.Value())
	}
	return n.s.String()
}

func TestW3CGraphStoreProtocol(t *testing.T) {
	manifest, err := filepath.Abs("../testdata/w3c/rdf-tests/sparql/sparql11/graph-store-protocol/manifest.ttl")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(manifest); err != nil {
		t.Skipf("graph-store-protocol suite not present at this rdf-tests pin: %v", err)
	}
	tests := manifestTests(t, manifest, nsMF+"GraphStoreProtocolTest")
	if len(tests) == 0 {
		t.Fatal("manifest lists no tests")
	}
	seen := make(map[string]bool, len(tests))
	for _, test := range tests {
		name := gspTestName(test)
		if seen[name] {
			t.Fatalf("test %s listed twice", name)
		}
		seen[name] = true
		t.Run(name, func(t *testing.T) { runGSPTest(t, gspNode{t, test.g, test.s}) })
	}
}

func runGSPTest(t *testing.T, test gspNode) {
	test.onlyKnown("test", nsMF+"name", nsMF+"requires", nsMF+"action")
	if types := test.objects(nsRDF + "type"); len(types) != 1 || types[0].(term.URIRef).Value() != nsMF+"GraphStoreProtocolTest" {
		t.Fatalf("%s: type is %v, want mf:GraphStoreProtocolTest", test.s, types)
	}
	for _, req := range test.objects(nsMF + "requires") {
		if u, ok := req.(term.URIRef); !ok || !gspFeatures[u.Value()] {
			t.Fatalf("test requires %s, which GraphStoreHandler is not known to support", req)
		}
	}
	conn, ok := test.node(nsMF + "action")
	if !ok {
		t.Fatal("no mf:action")
	}
	conn.onlyKnown("connection", nsHT+"connectionAuthority", nsHT+"requests")
	authority, ok := conn.str(nsHT + "connectionAuthority")
	if !ok {
		t.Fatal("no ht:connectionAuthority")
	}
	requests := conn.list(nsHT + "requests")
	if len(requests) == 0 {
		t.Fatal("no ht:requests")
	}

	// A fresh store per test: the tests assume they start from nothing (a
	// first PUT must be 201).
	h := endpoint.New(&sparql.Dataset{})
	gs := h.GraphStoreHandler()
	mux := http.NewServeMux()
	mux.Handle("/gsp", http.StripPrefix("/gsp", gs))
	mux.Handle("/gsp/", http.StripPrefix("/gsp", gs))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	vars := map[string]string{} // template variables set by mf:expectedLocation
	subst := func(s string) string {
		for k, v := range vars {
			s = strings.ReplaceAll(s, k, v)
		}
		return s
	}
	for i, rq := range requests {
		runGSPRequest(t, i, rq, srv.URL, authority, vars, subst)
	}
}

func runGSPRequest(t *testing.T, i int, rq gspNode, base, authority string, vars map[string]string, subst func(string) string) {
	t.Helper()
	rq.onlyKnown("request", nsHT+"methodName", nsHT+"httpVersion", nsHT+"absolutePath", nsHT+"headers", nsHT+"body", nsHT+"resp")
	method, _ := rq.str(nsHT + "methodName")
	path, _ := rq.str(nsHT + "absolutePath")
	if method == "" || path == "" {
		t.Fatalf("request %d: method %q, path %q", i, method, path)
	}
	if v, ok := rq.str(nsHT + "httpVersion"); !ok || v != "1.1" {
		t.Fatalf("request %d: ht:httpVersion %q, want 1.1", i, v)
	}
	var body io.Reader
	if b, ok := rq.node(nsHT + "body"); ok {
		b.onlyKnown("request body", nsCNT+"chars", nsCNT+"characterEncoding")
		chars, _ := b.str(nsCNT + "chars")
		enc, _ := b.str(nsCNT + "characterEncoding")
		switch strings.ToUpper(enc) {
		case "UTF-8":
			body = strings.NewReader(subst(chars))
		case "UTF-16": // with a byte order mark, as the protocol tests intend
			body = strings.NewReader(utf16(subst(chars)))
		default:
			t.Fatalf("request %d: cnt:characterEncoding %q is not handled by the runner", i, enc)
		}
	}
	label := fmt.Sprintf("request %d %s %s", i, method, subst(path))
	req, err := http.NewRequest(method, base+subst(path), body)
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	req.Host = authority
	for _, h := range rq.list(nsHT + "headers") {
		h.onlyKnown("request header", nsHT+"fieldName", nsHT+"fieldValue")
		name, _ := h.str(nsHT + "fieldName")
		val, _ := h.str(nsHT + "fieldValue")
		req.Header.Add(name, val)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}

	want, ok := rq.node(nsHT + "resp")
	if !ok {
		t.Fatalf("%s: no ht:resp", label)
	}
	want.onlyKnown("response", nsHT+"headers", nsHT+"body", nsMF+"expectedStatus", nsMF+"expectedLocation", nsMF+"expectedBoolean", nsMF+"expectedFormat")

	statuses := want.objects(nsMF + "expectedStatus")
	if len(statuses) == 0 {
		t.Fatalf("%s: no mf:expectedStatus", label)
	}
	okStatus := false
	var names []string
	for _, s := range statuses {
		u, isIRI := s.(term.URIRef)
		if !isIRI || !strings.HasPrefix(u.Value(), nsHTS) {
			t.Fatalf("%s: expected status %s is not an hts: IRI", label, s)
		}
		local := strings.TrimPrefix(u.Value(), nsHTS)
		code, known := gspStatus[local]
		if !known {
			t.Fatalf("%s: unknown status hts:%s; add it to gspStatus", label, local)
		}
		names = append(names, local)
		if code == resp.StatusCode || (code < 0 && resp.StatusCode/100 == -code) {
			okStatus = true
		}
	}
	if !okStatus {
		sort.Strings(names)
		t.Fatalf("%s: status %d, want one of %v\nbody: %s", label, resp.StatusCode, names, data)
	}

	checkExpectedResult(t, label, want, response{status: resp.StatusCode, header: resp.Header, body: string(data)})

	if v, ok := want.str(nsMF + "expectedLocation"); ok {
		loc := resp.Header.Get("Location")
		if loc == "" {
			t.Fatalf("%s: no Location header", label)
		}
		vars[v] = loc
	}

	for _, h := range want.list(nsHT + "headers") {
		h.onlyKnown("response header", nsHT+"fieldName", nsHT+"fieldValue")
		name, _ := h.str(nsHT + "fieldName")
		val, _ := h.str(nsHT + "fieldValue")
		got := resp.Header.Get(name)
		if !sameHeader(name, val, got) {
			t.Fatalf("%s: header %s = %q, want %q", label, name, got, val)
		}
	}

	if b, ok := want.node(nsHT + "body"); ok {
		b.onlyKnown("response body", nsCNT+"chars", nsCNT+"characterEncoding")
		chars, _ := b.str(nsCNT + "chars")
		mt, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
		if err != nil || mt != "text/turtle" {
			t.Fatalf("%s: Content-Type %q: the runner compares text/turtle bodies only", label, resp.Header.Get("Content-Type"))
		}
		exp, act := graph.NewGraph(), graph.NewGraph()
		if err := turtle.Parse(t.Context(), exp, strings.NewReader(subst(chars))); err != nil {
			t.Fatalf("%s: expected body: %v", label, err)
		}
		if err := turtle.Parse(t.Context(), act, strings.NewReader(string(data))); err != nil {
			t.Fatalf("%s: response is not Turtle: %v\n%s", label, err, data)
		}
		testutil.AssertGraphEqual(t, exp, act)
	}
	if method == http.MethodHead && len(data) != 0 {
		t.Errorf("%s: HEAD response has a body", label)
	}
}

// sameHeader compares a response header to the manifest's value: media types
// by type and parameters (case-insensitively, as HTTP does), anything else
// exactly.
func sameHeader(name, want, got string) bool {
	if strings.EqualFold(name, "content-type") {
		wt, wp, err1 := mime.ParseMediaType(want)
		gt, gp, err2 := mime.ParseMediaType(got)
		if err1 != nil || err2 != nil || wt != gt || len(wp) != len(gp) {
			return false
		}
		for k, v := range wp {
			if !strings.EqualFold(gp[k], v) {
				return false
			}
		}
		return true
	}
	return want == got
}

// rdfMediaTypes are the RDF serializations a CONSTRUCT or DESCRIBE answer may
// use for mf:expectedFormat "RDF".
var rdfMediaTypes = map[string]bool{
	"text/turtle": true, "application/rdf+xml": true, "application/n-triples": true,
	"application/ld+json": true, "application/n-quads": true, "application/trig": true,
}

// checkExpectedResult checks the protocol suite's mf:expectedBoolean and
// mf:expectedFormat ("boolean", "tabular", "RDF") against a response. A
// format the runner does not know is a failure.
func checkExpectedResult(t *testing.T, label string, want gspNode, r response) {
	t.Helper()
	ct, _, _ := mime.ParseMediaType(r.header.Get("Content-Type"))
	if f, ok := want.str(nsMF + "expectedFormat"); ok {
		switch f {
		case "boolean", "tabular":
			res := parseResult(t, r) // fails unless it is SPARQL JSON or XML
			if (f == "boolean") != (res.Type == "ASK") {
				t.Fatalf("%s: expected a %s result, got a %s result", label, f, res.Type)
			}
		case "RDF":
			if !rdfMediaTypes[ct] {
				t.Fatalf("%s: Content-Type %q is not an RDF serialization", label, r.header.Get("Content-Type"))
			}
			if ct == "text/turtle" || ct == "application/n-triples" {
				if err := turtle.Parse(t.Context(), graph.NewGraph(), strings.NewReader(r.body)); err != nil {
					t.Fatalf("%s: response is not %s: %v", label, ct, err)
				}
			}
		default:
			t.Fatalf("%s: mf:expectedFormat %q is not handled by the runner", label, f)
		}
	}
	if o, ok := want.one(nsMF + "expectedBoolean"); ok {
		l, isLit := o.(term.Literal)
		if !isLit {
			t.Fatalf("%s: mf:expectedBoolean %s is not a literal", label, o)
		}
		if got := parseResult(t, r).AskResult; got != (l.Lexical() == "true") {
			t.Fatalf("%s: ASK = %v, want %s", label, got, l.Lexical())
		}
	}
}
