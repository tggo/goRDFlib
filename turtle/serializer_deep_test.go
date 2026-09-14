package turtle

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// blankChain builds root -> _:b0 -> _:b1 -> ... -> _:b<n>.
func blankChain(n int) *rdflibgo.Graph {
	g := rdflibgo.NewGraph()
	p := rdflibgo.NewURIRefUnsafe("http://example.org/next")
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/root"), p, rdflibgo.NewBNode("b0"))
	for i := range n {
		g.Add(rdflibgo.NewBNode(fmt.Sprintf("b%d", i)), p, rdflibgo.NewBNode(fmt.Sprintf("b%d", i+1)))
	}
	return g
}

// rdflib #1424: a long blank node chain must not recurse once per link.
// WithMaxNestDepth used to accept any depth, so a caller raising it could
// overflow the stack; it is clamped to maxNestDepthLimit now.
func TestSerializeDeepBlankNodeChain(t *testing.T) {
	n := 200000
	if testing.Short() {
		n = 50000
	}
	long := blankChain(n)
	// The pretty layout indents every nesting level, so at the clamped depth
	// its output grows by megabytes per thousand links; a shorter chain still
	// crosses the limit several times.
	short := blankChain(3 * maxNestDepthLimit)
	for name, c := range map[string]struct {
		g    *rdflibgo.Graph
		opts []Option
	}{
		"default":            {long, nil},
		"huge depth":         {long, []Option{WithMaxNestDepth(1 << 30)}},
		"pretty, huge depth": {short, []Option{WithPretty(), WithMaxNestDepth(1 << 30)}},
	} {
		t.Run(name, func(t *testing.T) {
			var b bytes.Buffer
			if err := Serialize(c.g, &b, c.opts...); err != nil {
				t.Fatal(err)
			}
			g2 := rdflibgo.NewGraph()
			if err := Parse(g2, strings.NewReader(b.String())); err != nil {
				t.Fatalf("output does not parse: %v", err)
			}
			assertChain(t, g2, c.g.Len()-1)
			// Nesting stops at the limit: the node just past it starts a
			// statement of its own.
			if !strings.Contains(b.String(), fmt.Sprintf("_:b%d", maxNestDepthLimit)) && len(c.opts) > 0 {
				t.Errorf("nesting was not cut at %d", maxNestDepthLimit)
			}
		})
	}
	for _, opts := range [][]Option{{WithMaxNestDepth(7)}, {WithPretty(), WithMaxNestDepth(7)}} {
		var b bytes.Buffer
		if err := Serialize(blankChain(300), &b, opts...); err != nil {
			t.Fatal(err)
		}
		assertChain(t, mustParseTurtle(t, b.String()), 300)
	}
}

// assertChain checks g is exactly root -> n+1 distinct blank nodes, linked in
// order. testutil.AssertGraphEqual cannot be used: every link of a chain has
// the same signature, and its backtracking search is exponential on them.
func assertChain(t *testing.T, g *rdflibgo.Graph, n int) {
	t.Helper()
	if g.Len() != n+1 {
		t.Fatalf("got %d triples, want %d", g.Len(), n+1)
	}
	p := rdflibgo.NewURIRefUnsafe("http://example.org/next")
	var node rdflibgo.Subject = rdflibgo.NewURIRefUnsafe("http://example.org/root")
	seen := make(map[string]bool, n+1)
	for i := 0; i <= n; i++ {
		next, ok := g.Value(node, &p, nil)
		b, isBNode := next.(rdflibgo.BNode)
		if !ok || !isBNode || seen[b.Value()] {
			t.Fatalf("link %d: got %v", i, next)
		}
		seen[b.Value()] = true
		node = b
	}
}
