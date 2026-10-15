package jsonld

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/piprate/json-gold/ld"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/storetest"
	"github.com/tggo/goRDFlib/testutil"
)

const nodesContext = `{
  "@vocab": "https://schema.org/",
  "ex": "http://example.org/",
  "xsd": "http://www.w3.org/2001/XMLSchema#",
  "age": {"@type": "xsd:integer"},
  "knows": {"@type": "@id"},
  "tags": {"@container": "@list"}
}`

// nodeLines covers what one node of a stream can hold: an IRI subject,
// typed and language literals, a nested anonymous node, an explicit blank
// node referenced from another line, a list, @reverse, a node-local
// @context, and an array of nodes on one line.
var nodeLines = []string{
	`{"@id": "ex:a", "@type": "Person", "name": "A", "age": "41", "knows": "ex:b"}`,
	`{"@id": "ex:b", "name": {"@value": "Bé", "@language": "fr"}, "address": {"streetAddress": "1 Main"}}`,
	`{"@id": "_:shared", "name": "first half"}`,
	`{"@id": "ex:c", "owns": {"@id": "_:shared"}, "tags": ["x", "y"]}`,
	`{"@id": "_:shared", "description": "second half"}`,
	`{"@id": "ex:d", "@reverse": {"knows": {"@id": "ex:e"}}}`,
	`{"@context": {"loc": "http://example.org/local#"}, "@id": "ex:f", "loc:p": "local"}`,
	`[{"@id": "ex:g", "name": "G"}, {"@id": "ex:h", "name": "H"}]`,
	`{"@id": "ex:i", "address": {"streetAddress": "2 Side"}}`,
}

func nodesDoc() string {
	var items []string
	for _, l := range nodeLines {
		if strings.HasPrefix(l, "[") {
			items = append(items, strings.Trim(l, "[]"))
			continue
		}
		items = append(items, l)
	}
	return `{"@context": ` + nodesContext + `, "@graph": [` + strings.Join(items, ",") + `]}`
}

// TestParseNodesMatchesParse: streaming the nodes one per line gives the
// graph Parse gives for one document holding them all in @graph.
func TestParseNodesMatchesParse(t *testing.T) {
	ctx := t.Context()
	want := rdflibgo.NewGraph()
	if err := Parse(ctx, want, strings.NewReader(nodesDoc())); err != nil {
		t.Fatal(err)
	}
	for _, form := range []struct {
		name string
		ctx  any
	}{
		{"raw", json.RawMessage(nodesContext)},
		{"bytes", []byte(`{"@context": ` + nodesContext + `}`)},
		{"decoded", mustDecode(t, nodesContext)},
	} {
		t.Run(form.name, func(t *testing.T) {
			got := rdflibgo.NewGraph()
			in := strings.Join(nodeLines, "\n\n") // blank lines are skipped
			if err := ParseNodes(ctx, got, strings.NewReader(in), form.ctx); err != nil {
				t.Fatal(err)
			}
			testutil.AssertGraphEqual(t, want, got)
		})
	}
}

// TestParseNodesAcrossBatches: a blank node and an IRI described in the
// first and the last batch are still one node each.
func TestParseNodesAcrossBatches(t *testing.T) {
	ctx := t.Context()
	lines := []string{`{"@id": "_:x", "ex:first": "1"}`, `{"@id": "ex:z", "ex:first": "1"}`}
	for i := range 2 * nodeBatchSize {
		lines = append(lines, fmt.Sprintf(`{"@id": "ex:n%d", "ex:p": {"ex:q": "%d"}}`, i, i))
	}
	lines = append(lines, `{"@id": "_:x", "ex:last": "2"}`, `{"@id": "ex:z", "ex:last": "2", "ex:link": {"@id": "_:x"}}`)

	want := rdflibgo.NewGraph()
	doc := `{"@context": {"ex": "http://example.org/"}, "@graph": [` + strings.Join(lines, ",") + `]}`
	if err := Parse(ctx, want, strings.NewReader(doc)); err != nil {
		t.Fatal(err)
	}
	got := rdflibgo.NewGraph()
	if err := ParseNodes(ctx, got, strings.NewReader(strings.Join(lines, "\n")), json.RawMessage(`{"ex": "http://example.org/"}`)); err != nil {
		t.Fatal(err)
	}
	// Isomorphism over 512 look-alike blank nodes is slow; compare the
	// parts that matter and the size.
	wn, _ := want.Len(ctx)
	gn, _ := got.Len(ctx)
	if wn != gn {
		t.Fatalf("ParseNodes gave %d triples, Parse %d", gn, wn)
	}
	first := rdflibgo.NewURIRefUnsafe("http://example.org/first")
	last := rdflibgo.NewURIRefUnsafe("http://example.org/last")
	link := rdflibgo.NewURIRefUnsafe("http://example.org/link")
	z := rdflibgo.NewURIRefUnsafe("http://example.org/z")
	var x rdflibgo.Term
	for tr, err := range got.Triples(ctx, z, &link, nil) {
		if err != nil {
			t.Fatal(err)
		}
		x = tr.Object
	}
	for _, p := range []*rdflibgo.URIRef{&first, &last} {
		if ok, _ := got.Contains(ctx, x.(rdflibgo.Subject), *p, nil); !ok {
			t.Errorf("_:x lost its %s triple across batches", p.Value())
		}
	}
}

// TestParseNodesBothPathsAgree: a node that must take the N-Quads text path
// labels its blank nodes exactly as the fast path does, so _:shared stays
// one node across lines either way.
func TestParseNodesBothPathsAgree(t *testing.T) {
	ctx := t.Context()
	fast := rdflibgo.NewGraph()
	if err := ParseNodes(ctx, fast, strings.NewReader(strings.Join(nodeLines, "\n")), json.RawMessage(nodesContext)); err != nil {
		t.Fatal(err)
	}
	text := rdflibgo.NewGraph()
	if err := ParseNodes(ctx, text, strings.NewReader(strings.Join(nodeLines, "\n")), json.RawMessage(nodesContext),
		func(c *config) { c.forceTextPath = true }); err != nil {
		t.Fatal(err)
	}
	testutil.AssertGraphEqual(t, fast, text)
}

// TestParseNodesBlankNodeScope: _:x is one node within a stream, two
// streams never share one, and anonymous nodes are always fresh.
func TestParseNodesBlankNodeScope(t *testing.T) {
	ctx := t.Context()
	parse := func() *rdflibgo.Graph {
		g := rdflibgo.NewGraph()
		in := `{"@id": "_:x", "ex:p": "1"}` + "\n" + `{"@id": "_:x", "ex:q": "2"}` + "\n" +
			`{"ex:r": {"ex:s": "3"}}` + "\n" + `{"ex:r": {"ex:s": "3"}}`
		if err := ParseNodes(ctx, g, strings.NewReader(in), json.RawMessage(`{"ex": "http://example.org/"}`)); err != nil {
			t.Fatal(err)
		}
		return g
	}
	a, b := parse(), parse()
	subjects := func(g *rdflibgo.Graph) map[string]bool {
		out := map[string]bool{}
		for tr, err := range g.Triples(ctx, nil, nil, nil) {
			if err != nil {
				t.Fatal(err)
			}
			out[tr.Subject.N3()] = true
		}
		return out
	}
	sa, sb := subjects(a), subjects(b)
	// _:x, two anonymous outer nodes and two anonymous inner ones.
	if len(sa) != 5 {
		t.Fatalf("got %d subjects, want 5: %v", len(sa), sa)
	}
	for s := range sa {
		if sb[s] {
			t.Fatalf("two streams share blank node %s", s)
		}
	}
}

// TestParseNodesLabelsHaveBoundedWidth: labels are a fixed prefix and
// json-gold's counter, never a growing chain, and nothing is kept per label.
func TestParseNodesLabelsHaveBoundedWidth(t *testing.T) {
	label := regexp.MustCompile(`^(N[0-9a-f]{32})b[0-9]+$`)
	prefixes := map[string]bool{}
	h := func(s rdflibgo.Subject, _ rdflibgo.URIRef, _ rdflibgo.Term) error {
		if b, ok := s.(rdflibgo.BNode); ok {
			m := label.FindStringSubmatch(b.Value())
			if m == nil {
				t.Fatalf("label %q is not prefix + json-gold counter", b.Value())
			}
			prefixes[m[1]] = true
		}
		return nil
	}
	p, err := NewNodeParser(json.RawMessage(`{"ex": "http://example.org/"}`), h)
	if err != nil {
		t.Fatal(err)
	}
	for range 3 * nodeBatchSize / 2 { // one full batch and a flushed rest
		if err := p.ParseNode(t.Context(), []byte(`{"ex:r": {"ex:s": "v"}}`)); err != nil {
			t.Fatal(err)
		}
	}
	if err := p.Flush(t.Context()); err != nil {
		t.Fatal(err)
	}
	if len(prefixes) != 1 {
		t.Fatalf("one parser used %d label prefixes, want 1", len(prefixes))
	}
}

// TestParseNodesProcessesContextOnce: a remote shared context is loaded
// once for the stream, not once per node — the point of issue #53's
// request over batching Parse calls.
func TestParseNodesProcessesContextOnce(t *testing.T) {
	loader := &countingLoader{doc: mustDecode(t, `{"@context": `+nodesContext+`}`)}
	var in strings.Builder
	for i := range 100 {
		fmt.Fprintf(&in, `{"@id": "ex:n%d", "name": "n"}`+"\n", i)
	}
	g := rdflibgo.NewGraph()
	if err := ParseNodes(t.Context(), g, strings.NewReader(in.String()), "http://example.org/context.jsonld",
		WithDocumentLoader(loader)); err != nil {
		t.Fatal(err)
	}
	if n, _ := g.Len(t.Context()); n != 100 {
		t.Fatalf("graph has %d triples, want 100", n)
	}
	if loader.calls != 1 {
		t.Fatalf("context loaded %d times, want 1", loader.calls)
	}
}

type countingLoader struct {
	doc   any
	calls int
}

func (l *countingLoader) LoadDocument(u string) (*ld.RemoteDocument, error) {
	l.calls++
	return &ld.RemoteDocument{DocumentURL: u, Document: l.doc}, nil
}

// TestParseNodesErrors: a bad line names its line; WithNodeErrorHandler can
// skip it; the nodes before it stay delivered.
func TestParseNodesErrors(t *testing.T) {
	ctx := t.Context()
	in := `{"@id": "http://e/a", "http://e/p": "1"}` + "\n" + `{"@id": ` + "\n" + `{"@id": "http://e/b", "http://e/p": "2"}`

	// A node that expands with an error leaves nothing of itself behind.
	bad := `{"@id": "http://e/a", "http://e/p": "1"}` + "\n" + `{"@id": "http://e/x", "http://e/p": "x", "@type": 5}`
	g := rdflibgo.NewGraph()
	err := ParseNodes(ctx, g, strings.NewReader(bad), nil, WithNodeErrorHandler(func(int, []byte, error) error { return nil }))
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := g.Len(ctx); n != 1 {
		t.Fatalf("a failed node left %d triples, want only the 1 of the good node", n)
	}

	g = rdflibgo.NewGraph()
	err = ParseNodes(ctx, g, strings.NewReader(in), nil)
	if !errors.Is(err, ErrInvalidJSON) || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("error = %v, want ErrInvalidJSON naming line 2", err)
	}
	if n, _ := g.Len(ctx); n != 1 {
		t.Fatalf("graph has %d triples after the failure, want the 1 before it", n)
	}

	g = rdflibgo.NewGraph()
	var skipped []int
	err = ParseNodes(ctx, g, strings.NewReader(in), nil, WithNodeErrorHandler(func(line int, node []byte, err error) error {
		if !errors.Is(err, ErrInvalidJSON) || string(node) != `{"@id":` {
			t.Errorf("handler got line %d %q %v", line, node, err)
		}
		skipped = append(skipped, line)
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := g.Len(ctx); n != 2 || len(skipped) != 1 || skipped[0] != 2 {
		t.Fatalf("got %d triples, skipped %v; want 2 and [2]", n, skipped)
	}

	stop := errors.New("stop")
	err = ParseNodes(ctx, rdflibgo.NewGraph(), strings.NewReader(in), nil,
		WithNodeErrorHandler(func(int, []byte, error) error { return stop }))
	if !errors.Is(err, stop) {
		t.Fatalf("error = %v, want the handler's", err)
	}
}

// TestParseNodesStoreErrorIsNotSkipped: the store failing is not a bad node;
// WithNodeErrorHandler never sees it.
func TestParseNodesStoreErrorIsNotSkipped(t *testing.T) {
	fs := storetest.NewFailingStore(store.NewMemoryStore())
	fs.Fail(nil)
	called := false
	err := ParseNodes(t.Context(), graph.NewGraphFromStore(fs, nil),
		strings.NewReader(`{"@id": "http://e/a", "http://e/p": "1"}`), nil,
		WithNodeErrorHandler(func(int, []byte, error) error { called = true; return nil }))
	if !errors.Is(err, storetest.ErrInjected) || called {
		t.Fatalf("error = %v, handler called %v; want ErrInjected, not called", err, called)
	}
}

func TestParseNodesContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	err := ParseNodes(ctx, rdflibgo.NewGraph(), strings.NewReader(`{"@id": "http://e/a", "http://e/p": "1"}`), nil,
		WithNodeErrorHandler(func(int, []byte, error) error { return nil }))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func TestParseNodesProvenanceIsTheNodeLine(t *testing.T) {
	in := `{"@id": "http://e/a", "http://e/p": "1"}` + "\n\n" + `{"@id": "http://e/b", "http://e/p": {"http://e/q": "2"}}`
	lines := map[string]int{} // predicate on subject -> line
	err := ParseNodes(t.Context(), rdflibgo.NewGraph(), strings.NewReader(in), nil,
		WithProvenance(func(s rdflibgo.Subject, p rdflibgo.URIRef, _ rdflibgo.Term, line int) {
			key := p.Value()
			if _, iri := s.(rdflibgo.URIRef); iri {
				key = s.N3() + " " + key
			}
			lines[key] = line
		}))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"<http://e/a> http://e/p": 1,
		"<http://e/b> http://e/p": 3,
		"http://e/q":              3, // the nested blank node, on its node's line
	}
	for k, w := range want {
		if lines[k] != w {
			t.Errorf("%s reported at line %d, want %d", k, lines[k], w)
		}
	}
}

func TestParseNodesLineTooLong(t *testing.T) {
	in := `{"@id": "http://e/a", "http://e/p": "` + strings.Repeat("x", 100) + `"}`
	err := ParseNodes(t.Context(), rdflibgo.NewGraph(), strings.NewReader(in), nil, WithMaxNodeSize(50))
	if !errors.Is(err, ErrNodeTooLong) {
		t.Fatalf("error = %v, want ErrNodeTooLong", err)
	}
}

func TestNewNodeParserRejectsBadContext(t *testing.T) {
	h := func(rdflibgo.Subject, rdflibgo.URIRef, rdflibgo.Term) error { return nil }
	if _, err := NewNodeParser(json.RawMessage(`{"@vocab": `), h); !errors.Is(err, ErrInvalidJSON) {
		t.Fatalf("truncated context: %v, want ErrInvalidJSON", err)
	}
	if _, err := NewNodeParser(map[string]any{"@vocab": 5}, h); err == nil {
		t.Fatal("an invalid @vocab was accepted")
	}
	if _, err := NewNodeParser(nil, nil); !errors.Is(err, ErrNilHandler) {
		t.Fatalf("nil handler: %v", err)
	}
}

func mustDecode(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

// FuzzParseNodes: a single node streamed gives as many triples as Parse of
// the same node in a document with the same context, and never panics.
func FuzzParseNodes(f *testing.F) {
	for _, l := range nodeLines {
		f.Add(l)
	}
	f.Add(`{"@id": "%", "ex:p": "1"}`)
	f.Add(`{"@context": {"@base": "http://b/"}, "@id": "x", "ex:p": {"@list": [1, {"@id": "_:a"}]}}`)
	f.Fuzz(func(t *testing.T, node string) {
		if strings.ContainsAny(node, "\n\r") {
			return // one line is one node
		}
		ctx := t.Context()
		want := rdflibgo.NewGraph()
		doc := `{"@context": ` + nodesContext + `, "@graph": [` + node + `]}`
		werr := Parse(ctx, want, strings.NewReader(doc), WithBase("http://base/"))
		got := rdflibgo.NewGraph()
		gerr := ParseNodes(ctx, got, strings.NewReader(node), json.RawMessage(nodesContext), WithBase("http://base/"))
		if werr != nil || gerr != nil {
			return // the wrapping changes what is valid JSON; only agreement on success is checked
		}
		wn, _ := want.Len(ctx)
		gn, _ := got.Len(ctx)
		if wn != gn {
			t.Fatalf("node %q: ParseNodes %d triples, Parse %d", node, gn, wn)
		}
	})
}
