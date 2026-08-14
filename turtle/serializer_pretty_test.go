package turtle

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/testutil"
)

func serialize(t *testing.T, g *rdflibgo.Graph, opts ...Option) string {
	t.Helper()
	var buf bytes.Buffer
	if err := Serialize(g, &buf, opts...); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// reparse round-trips serialized output back into a graph.
func reparse(t *testing.T, out string) *rdflibgo.Graph {
	t.Helper()
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(out)); err != nil {
		t.Fatalf("reparsing output failed: %v\n%s", err, out)
	}
	return g
}

// issueGraph builds the graph from issue #27.
func issueGraph() *rdflibgo.Graph {
	const ns = "https://example.org/"
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe(ns))

	document := rdflibgo.NewURIRefUnsafe(ns + "document")
	entryPred := rdflibgo.NewURIRefUnsafe(ns + "entry")
	entryClass := rdflibgo.NewURIRefUnsafe(ns + "Entry")
	labelPred := rdflibgo.NewURIRefUnsafe(ns + "label")
	posPred := rdflibgo.NewURIRefUnsafe(ns + "position")

	for i, label := range []string{"Alpha", "Beta", "Gamma"} {
		entry := rdflibgo.NewBNode("entry-" + label)
		g.Add(document, entryPred, entry)
		g.Add(entry, rdflibgo.RDF.Type, entryClass)
		g.Add(entry, labelPred, rdflibgo.NewLiteral(label))
		g.Add(entry, posPred, rdflibgo.NewLiteral(i+1))
	}
	return g
}

// TestPrettyMatchesIssue27 pins the exact layout requested in issue #27.
func TestPrettyMatchesIssue27(t *testing.T) {
	want := `@prefix ex: <https://example.org/> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .

ex:document
    ex:entry
        [
            a ex:Entry ;
            ex:label "Alpha" ;
            ex:position 1
        ],
        [
            a ex:Entry ;
            ex:label "Beta" ;
            ex:position 2
        ],
        [
            a ex:Entry ;
            ex:label "Gamma" ;
            ex:position 3
        ] .
`
	if got := serialize(t, issueGraph(), WithPretty()); got != want {
		t.Errorf("pretty output mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

// TestPrettyIsOptIn guards the default layout: pretty printing must never
// activate on its own, because callers diff and hash serializer output.
func TestPrettyIsOptIn(t *testing.T) {
	out := serialize(t, issueGraph())
	body := out[strings.Index(out, "ex:document"):]
	if strings.Contains(strings.TrimSuffix(body, "\n"), "\n") {
		t.Errorf("default output should keep the statement on one line, got:\n%s", out)
	}
}

func TestPrettyIndentWidth(t *testing.T) {
	out := serialize(t, issueGraph(), WithIndent(2))
	if !strings.Contains(out, "\n  ex:entry\n") {
		t.Errorf("expected 2-space indent, got:\n%s", out)
	}

	// 0 disables indentation but keeps the line breaks.
	out = serialize(t, issueGraph(), WithIndent(0))
	if !strings.Contains(out, "\nex:entry\n") {
		t.Errorf("expected unindented layout, got:\n%s", out)
	}

	// Out-of-range widths are clamped rather than rejected.
	out = serialize(t, issueGraph(), WithIndent(-5))
	if !strings.Contains(out, "\nex:entry\n") {
		t.Errorf("negative width should clamp to 0, got:\n%s", out)
	}
	out = serialize(t, issueGraph(), WithIndent(500))
	if !strings.Contains(out, "\n"+strings.Repeat(" ", maxIndentWidth)+"ex:entry\n") {
		t.Errorf("width should clamp to %d, got:\n%s", maxIndentWidth, out)
	}
}

// TestPrettyRoundTrip verifies that indentation carries no meaning: every
// layout must reparse to the same graph.
func TestPrettyRoundTrip(t *testing.T) {
	g := issueGraph()
	for _, tc := range []struct {
		name string
		opts []Option
	}{
		{"compact", nil},
		{"pretty", []Option{WithPretty()}},
		{"indent2", []Option{WithIndent(2)}},
		{"indent0", []Option{WithIndent(0)}},
		{"shallow", []Option{WithPretty(), WithMaxNestDepth(1)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.AssertGraphEqual(t, g, reparse(t, serialize(t, g, tc.opts...)))
		})
	}
}

// chainGraph builds ex:root -> [ ex:next [ ex:next [ ... ] ] ], depth links
// deep. This is the shape that makes inline nesting recursive.
func chainGraph(depth int) *rdflibgo.Graph {
	const ns = "https://example.org/"
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe(ns))

	next := rdflibgo.NewURIRefUnsafe(ns + "next")
	var parent rdflibgo.Subject = rdflibgo.NewURIRefUnsafe(ns + "root")
	for i := 0; i < depth; i++ {
		child := rdflibgo.NewBNode(fmt.Sprintf("n%d", i))
		g.Add(parent, next, child)
		parent = child
	}
	g.Add(parent, rdflibgo.RDF.Type, rdflibgo.NewURIRefUnsafe(ns+"Leaf"))
	return g
}

// TestNestDepthLimit is the safety guard: a deep blank-node chain must not
// recurse without bound, and flattening it must not lose a single triple.
//
// The graphs here are too large for the isomorphism check (a 500-link chain of
// blank nodes is its worst case), so the round trip is verified by triple count
// — TestNestDepthLimitIsomorphic covers term identity on a small chain.
func TestNestDepthLimit(t *testing.T) {
	const depth = 500
	g := chainGraph(depth)
	wantLen := g.Len()

	for _, tc := range []struct {
		name string
		opts []Option
	}{
		{"compact default", nil},
		{"pretty default", []Option{WithPretty()}},
		{"limit 1", []Option{WithMaxNestDepth(1)}},
		{"pretty limit 1", []Option{WithPretty(), WithMaxNestDepth(1)}},
		{"limit raised", []Option{WithMaxNestDepth(depth + 10)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := serialize(t, g, tc.opts...)
			if got := reparse(t, out).Len(); got != wantLen {
				t.Fatalf("triple count changed: got %d, want %d\n%s", got, wantLen, out)
			}
		})
	}
}

// TestNestDepthLimitIsomorphic checks that flattening past the limit preserves
// the graph itself, not just the triple count.
func TestNestDepthLimitIsomorphic(t *testing.T) {
	g := chainGraph(6)
	for _, limit := range []int{1, 2, 3, 100} {
		t.Run(fmt.Sprintf("limit%d", limit), func(t *testing.T) {
			out := serialize(t, g, WithPretty(), WithMaxNestDepth(limit))
			testutil.AssertGraphEqual(t, g, reparse(t, out))
		})
	}
}

// TestNestDepthLimitCollections covers the other recursive shape: collections
// nested inside collections. Past the limit a list head is flattened into its
// rdf:first/rdf:rest statements, which must still round-trip.
func TestNestDepthLimitCollections(t *testing.T) {
	const ns = "https://example.org/"
	const depth = 120

	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe(ns))

	var inner rdflibgo.Term = rdflibgo.NewLiteral("leaf")
	for i := 0; i < depth; i++ {
		node := rdflibgo.NewBNode(fmt.Sprintf("c%d", i))
		g.Add(node, rdflibgo.RDF.First, inner)
		g.Add(node, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)
		inner = node
	}
	g.Add(rdflibgo.NewURIRefUnsafe(ns+"s"), rdflibgo.NewURIRefUnsafe(ns+"p"), inner)

	for _, opts := range [][]Option{nil, {WithPretty()}, {WithPretty(), WithMaxNestDepth(3)}} {
		out := serialize(t, g, opts...)
		if got := reparse(t, out).Len(); got != g.Len() {
			t.Fatalf("triple count changed: got %d, want %d\n%s", got, g.Len(), out)
		}
	}
}

// TestNestDepthLimitBoundsIndentation checks that the depth limit is what keeps
// the pretty layout from growing an indentation step per link.
func TestNestDepthLimitBoundsIndentation(t *testing.T) {
	g := chainGraph(200)
	out := serialize(t, g, WithPretty())

	widest := 0
	for _, line := range strings.Split(out, "\n") {
		if w := len(line) - len(strings.TrimLeft(line, " ")); w > widest {
			widest = w
		}
	}
	// Each nesting level costs two indentation levels: one for the object the
	// [ sits on, one for the predicates inside it.
	if maxWidth := (2*defaultMaxNestDepth + 1) * defaultIndentWidth; widest > maxWidth {
		t.Errorf("indentation %d exceeds the %d implied by the depth limit", widest, maxWidth)
	}
}

// collectionGraph builds ex:s ex:p ( items... ).
func collectionGraph(items ...rdflibgo.Term) *rdflibgo.Graph {
	const ns = "https://example.org/"
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe(ns))

	var head rdflibgo.Subject = rdflibgo.RDF.Nil
	for i := len(items) - 1; i >= 0; i-- {
		node := rdflibgo.NewBNode(fmt.Sprintf("l%d", i))
		g.Add(node, rdflibgo.RDF.First, items[i])
		g.Add(node, rdflibgo.RDF.Rest, head)
		head = node
	}
	g.Add(rdflibgo.NewURIRefUnsafe(ns+"s"), rdflibgo.NewURIRefUnsafe(ns+"p"), head)
	return g
}

// TestPrettyCollections keeps collections on one line unless an item expands.
func TestPrettyCollections(t *testing.T) {
	const ns = "https://example.org/"

	flat := collectionGraph(rdflibgo.NewLiteral("a"), rdflibgo.NewLiteral("b"), rdflibgo.NewLiteral("c"))
	out := serialize(t, flat, WithPretty())
	if !strings.Contains(out, `ex:p ( "a" "b" "c" ) .`) {
		t.Errorf("a flat collection should stay on the predicate line, got:\n%s", out)
	}
	testutil.AssertGraphEqual(t, flat, reparse(t, out))

	// A collection holding a blank node expands one item per line.
	inner := rdflibgo.NewBNode("inner")
	nested := collectionGraph(rdflibgo.NewLiteral("a"), inner, rdflibgo.NewLiteral("c"))
	nested.Add(inner, rdflibgo.NewURIRefUnsafe(ns+"k"), rdflibgo.NewLiteral("v"))

	out = serialize(t, nested, WithPretty())
	if !strings.Contains(out, "(\n") || !strings.Contains(out, "ex:k \"v\"") {
		t.Errorf("a collection with a nested block should expand, got:\n%s", out)
	}
	testutil.AssertGraphEqual(t, nested, reparse(t, out))
}

// TestPrettyEmptyGraph makes sure the layout options do not emit stray output.
func TestPrettyEmptyGraph(t *testing.T) {
	if out := serialize(t, rdflibgo.NewGraph(), WithPretty()); out != "" {
		t.Errorf("expected empty output, got %q", out)
	}
}
