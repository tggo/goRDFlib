package testutil_test

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/testutil"
)

func TestAssertGraphEqual(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g1.Add(s, p, term.NewLiteral("hello"))
	g2.Add(s, p, term.NewLiteral("hello"))

	testutil.AssertGraphEqual(t, g1, g2)
}

func TestAssertGraphEqualDifferentGraphs(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g1.Add(s, p, term.NewLiteral("hello"))
	g2.Add(s, p, term.NewLiteral("world"))

	mockT := &testing.T{}
	testutil.AssertGraphEqual(mockT, g1, g2)
	if !mockT.Failed() {
		t.Error("expected AssertGraphEqual to report failure for different graphs")
	}
}

func TestAssertGraphContains(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(s, p, term.NewLiteral("hello"))
	testutil.AssertGraphContains(t, g, s, p, term.NewLiteral("hello"))
}

func TestAssertGraphLen(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(s, p, term.NewLiteral("hello"))
	testutil.AssertGraphLen(t, g, 1)
}
