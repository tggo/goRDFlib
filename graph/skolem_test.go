package graph_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/testutil"
	"github.com/tggo/goRDFlib/turtle"
)

const skolemAuthority = "https://example.org"

func parseTTL(t *testing.T, src string) *graph.Graph {
	t.Helper()
	g := graph.NewGraph()
	if err := turtle.Parse(t.Context(), g, strings.NewReader("@prefix ex: <http://example.org/> .\n"+src)); err != nil {
		t.Fatal(err)
	}
	return g
}

// ntLines returns g as sorted N-Triples lines.
func ntLines(t *testing.T, g *graph.Graph) []string {
	t.Helper()
	var sb strings.Builder
	if err := nt.Serialize(t.Context(), g, &sb); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(sb.String()), "\n")
	slices.Sort(lines)
	return lines
}

func bnodeCount(g *graph.Graph) int {
	n := 0
	g.Triples(context.Background(), nil, nil, nil)(func(tr term.Triple, _ error) bool {
		if _, ok := tr.Subject.(term.BNode); ok {
			n++
		}
		if _, ok := tr.Object.(term.BNode); ok {
			n++
		}
		return true
	})
	return n
}

// Two nodes that differ only two hops away, two interchangeable nodes, a
// chain and a loop.
const skolemDoc = `
ex:a ex:p [ ex:q [ ex:r "1" ] ] .
ex:a ex:p [ ex:q [ ex:r "2" ] ] .
ex:a ex:twin [ ex:v "same" ] , [ ex:v "same" ] .
ex:a ex:list ( "x" "y" "z" ) .
_:loop ex:self _:loop .
`

func TestSkolemizeReplacesEveryBlankNode(t *testing.T) {
	g := parseTTL(t, skolemDoc)
	before, _ := g.Len(t.Context())
	sk, _ := g.Skolemize(t.Context(), skolemAuthority)
	if mustV(sk.Len(t.Context())) != before || mustV(g.Len(t.Context())) != before {
		t.Fatalf("len: original %d→%d, skolemized %d", before, mustV(g.Len(t.Context())), mustV(sk.Len(t.Context())))
	}
	if n := bnodeCount(sk); n != 0 {
		t.Fatalf("%d blank nodes left", n)
	}
	for _, l := range ntLines(t, sk) {
		for _, f := range strings.Fields(l) {
			if strings.HasPrefix(f, "<https://example.org/") && !strings.HasPrefix(f, "<https://example.org/.well-known/genid/") {
				t.Fatalf("unexpected Skolem IRI %s", f)
			}
		}
	}
	if bnodeCount(g) == 0 {
		t.Fatal("Skolemize modified the original graph")
	}
}

// The same document parsed twice has different blank node labels; stable IDs
// must still give identical output. Triples must not merge.
func TestSkolemizeStableAcrossParses(t *testing.T) {
	a, _ := parseTTL(t, skolemDoc).Skolemize(t.Context(), skolemAuthority, graph.WithStableSkolemIDs())
	// Same content, statements in another order.
	reordered := `
_:loop ex:self _:loop .
ex:a ex:list ( "x" "y" "z" ) .
ex:a ex:twin [ ex:v "same" ] , [ ex:v "same" ] .
ex:a ex:p [ ex:q [ ex:r "2" ] ] .
ex:a ex:p [ ex:q [ ex:r "1" ] ] .
`
	b, _ := parseTTL(t, reordered).Skolemize(t.Context(), skolemAuthority, graph.WithStableSkolemIDs())
	la, lb := ntLines(t, a), ntLines(t, b)
	if !slices.Equal(la, lb) {
		t.Fatalf("stable Skolem IRIs differ between parses:\n%s\n---\n%s", strings.Join(la, "\n"), strings.Join(lb, "\n"))
	}
	if mustV(a.Len(t.Context())) != mustV(parseTTL(t, skolemDoc).Len(t.Context())) {
		t.Fatalf("stable Skolemize merged triples: %d", mustV(a.Len(t.Context())))
	}
}

// ex:q's two subjects look alike one hop out; only refinement separates them.
func TestSkolemizeStableSeparatesDistantDifferences(t *testing.T) {
	g := parseTTL(t, `ex:a ex:p [ ex:q [ ex:r "1" ] ] . ex:a ex:p [ ex:q [ ex:r "2" ] ] .`)
	sk, _ := g.Skolemize(t.Context(), skolemAuthority, graph.WithStableSkolemIDs())
	for _, l := range ntLines(t, sk) {
		if strings.Contains(l, "-1>") {
			t.Fatalf("distinguishable nodes were numbered as twins: %s", l)
		}
	}
}

func TestDeSkolemizeRoundTrip(t *testing.T) {
	g := parseTTL(t, `ex:a ex:p [ ex:q "1" ] . ex:a ex:list ( "x" "y" ) .`)
	for _, opts := range [][]graph.SkolemOption{nil, {graph.WithStableSkolemIDs()}, {graph.WithSkolemBasepath("bnode")}} {
		back := mustV(mustV(g.Skolemize(t.Context(), skolemAuthority, opts...)).DeSkolemize(t.Context(), skolemAuthority, opts...))
		testutil.AssertGraphEqual(t, g, back)
	}
}
