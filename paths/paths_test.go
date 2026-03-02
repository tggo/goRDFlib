package paths_test

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/paths"
	"github.com/tggo/goRDFlib/term"
)

func makePathGraph(t *testing.T) *graph.Graph {
	t.Helper()
	g := graph.NewGraph()
	a, _ := term.NewURIRef("http://example.org/a")
	b, _ := term.NewURIRef("http://example.org/b")
	c, _ := term.NewURIRef("http://example.org/c")
	d, _ := term.NewURIRef("http://example.org/d")
	p, _ := term.NewURIRef("http://example.org/p")
	q, _ := term.NewURIRef("http://example.org/q")
	g.Add(a, p, b)
	g.Add(b, p, c)
	g.Add(c, p, d)
	g.Add(a, q, c)
	return g
}

func collectPairs(g *graph.Graph, path paths.Path, subj term.Subject, obj term.Term) [][2]string {
	var result [][2]string
	path.Eval(g, subj, obj)(func(s, o term.Term) bool {
		result = append(result, [2]string{s.N3(), o.N3()})
		return true
	})
	return result
}

func TestURIRefPath(t *testing.T) {
	g := makePathGraph(t)
	a, _ := term.NewURIRef("http://example.org/a")
	p, _ := term.NewURIRef("http://example.org/p")

	pairs := collectPairs(g, paths.AsPath(p), a, nil)
	if len(pairs) != 1 || pairs[0][1] != "<http://example.org/b>" {
		t.Errorf("expected [(a,b)], got %v", pairs)
	}
}

func TestInvPath(t *testing.T) {
	g := makePathGraph(t)
	b, _ := term.NewURIRef("http://example.org/b")
	p, _ := term.NewURIRef("http://example.org/p")

	pairs := collectPairs(g, paths.Inv(paths.AsPath(p)), b, nil)
	if len(pairs) != 1 || pairs[0][1] != "<http://example.org/a>" {
		t.Errorf("expected [(b,a)], got %v", pairs)
	}
}

func TestSequencePath(t *testing.T) {
	g := makePathGraph(t)
	a, _ := term.NewURIRef("http://example.org/a")
	p, _ := term.NewURIRef("http://example.org/p")

	path := paths.Sequence(paths.AsPath(p), paths.AsPath(p))
	pairs := collectPairs(g, path, a, nil)
	if len(pairs) != 1 || pairs[0][1] != "<http://example.org/c>" {
		t.Errorf("expected [(a,c)], got %v", pairs)
	}
}

func TestSequencePathTriple(t *testing.T) {
	g := makePathGraph(t)
	a, _ := term.NewURIRef("http://example.org/a")
	p, _ := term.NewURIRef("http://example.org/p")

	path := paths.Sequence(paths.AsPath(p), paths.AsPath(p), paths.AsPath(p))
	pairs := collectPairs(g, path, a, nil)
	if len(pairs) != 1 || pairs[0][1] != "<http://example.org/d>" {
		t.Errorf("expected [(a,d)], got %v", pairs)
	}
}

func TestAlternativePath(t *testing.T) {
	g := makePathGraph(t)
	a, _ := term.NewURIRef("http://example.org/a")
	p, _ := term.NewURIRef("http://example.org/p")
	q, _ := term.NewURIRef("http://example.org/q")

	path := paths.Alternative(paths.AsPath(p), paths.AsPath(q))
	pairs := collectPairs(g, path, a, nil)
	if len(pairs) != 2 {
		t.Errorf("expected 2 pairs, got %d: %v", len(pairs), pairs)
	}
}

func TestZeroOrMorePath(t *testing.T) {
	g := makePathGraph(t)
	a, _ := term.NewURIRef("http://example.org/a")
	p, _ := term.NewURIRef("http://example.org/p")

	path := paths.ZeroOrMore(paths.AsPath(p))
	pairs := collectPairs(g, path, a, nil)
	if len(pairs) != 4 {
		t.Errorf("expected 4 pairs for p*, got %d: %v", len(pairs), pairs)
	}
}

func TestOneOrMorePath(t *testing.T) {
	g := makePathGraph(t)
	a, _ := term.NewURIRef("http://example.org/a")
	p, _ := term.NewURIRef("http://example.org/p")

	path := paths.OneOrMore(paths.AsPath(p))
	pairs := collectPairs(g, path, a, nil)
	if len(pairs) != 3 {
		t.Errorf("expected 3 pairs for p+, got %d: %v", len(pairs), pairs)
	}
}

func TestZeroOrOnePath(t *testing.T) {
	g := makePathGraph(t)
	a, _ := term.NewURIRef("http://example.org/a")
	p, _ := term.NewURIRef("http://example.org/p")

	path := paths.ZeroOrOne(paths.AsPath(p))
	pairs := collectPairs(g, path, a, nil)
	if len(pairs) != 2 {
		t.Errorf("expected 2 pairs for p?, got %d: %v", len(pairs), pairs)
	}
}

func TestNegatedPath(t *testing.T) {
	g := makePathGraph(t)
	a, _ := term.NewURIRef("http://example.org/a")
	p, _ := term.NewURIRef("http://example.org/p")

	path := paths.Negated(p)
	pairs := collectPairs(g, path, a, nil)
	if len(pairs) != 1 || pairs[0][1] != "<http://example.org/c>" {
		t.Errorf("expected [(a,c)] via !p, got %v", pairs)
	}
}

func TestPathCycleDetection(t *testing.T) {
	g := graph.NewGraph()
	a, _ := term.NewURIRef("http://example.org/a")
	b, _ := term.NewURIRef("http://example.org/b")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(a, p, b)
	g.Add(b, p, a)

	path := paths.ZeroOrMore(paths.AsPath(p))
	pairs := collectPairs(g, path, a, nil)
	// a→b→a cycle with ZeroOrMore from a: (a,a) via zero-length, (a,b) via one step
	// b→a loop is handled by cycle detection, no duplicate (a,a)
	if len(pairs) != 2 {
		t.Errorf("expected 2 pairs with cycle, got %d: %v", len(pairs), pairs)
	}
}

func TestSequencePathEarlyTermination(t *testing.T) {
	g := graph.NewGraph()
	a, _ := term.NewURIRef("http://example.org/a")
	b, _ := term.NewURIRef("http://example.org/b")
	c, _ := term.NewURIRef("http://example.org/c")
	d, _ := term.NewURIRef("http://example.org/d")
	e, _ := term.NewURIRef("http://example.org/e")
	p, _ := term.NewURIRef("http://example.org/p")
	q, _ := term.NewURIRef("http://example.org/q")
	g.Add(a, p, b)
	g.Add(a, p, c)
	g.Add(b, q, d)
	g.Add(c, q, e)

	path := paths.Sequence(paths.AsPath(p), paths.AsPath(q))
	count := 0
	path.Eval(g, a, nil)(func(s, o term.Term) bool {
		count++
		return false
	})
	if count != 1 {
		t.Errorf("expected early termination after 1, got %d", count)
	}
}

func TestPathDSL(t *testing.T) {
	p, _ := term.NewURIRef("http://example.org/p")
	q, _ := term.NewURIRef("http://example.org/q")

	seq := paths.AsPath(p).Slash(paths.AsPath(q))
	if len(seq.Args) != 2 {
		t.Errorf("expected 2 args in sequence")
	}

	alt := paths.AsPath(p).Or(paths.AsPath(q))
	if len(alt.Args) != 2 {
		t.Errorf("expected 2 args in alternative")
	}
}
