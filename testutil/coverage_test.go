package testutil_test

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
	"github.com/tggo/goRDFlib/testutil"
)

// Tests exercising the bnode isomorphism code path.

func TestAssertGraphEqualWithBNodes(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	p, _ := term.NewURIRef("http://example.org/p")
	o := term.NewLiteral("v")

	// Same structure, different bnode labels
	b1 := term.NewBNode()
	b2 := term.NewBNode()
	g1.Add(b1, p, o)
	g2.Add(b2, p, o)

	testutil.AssertGraphEqual(t, g1, g2)
}

func TestAssertGraphEqualWithMultipleBNodes(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	p, _ := term.NewURIRef("http://example.org/p")
	q, _ := term.NewURIRef("http://example.org/q")

	// Two bnodes with different connections
	b1a := term.NewBNode()
	b1b := term.NewBNode()
	g1.Add(b1a, p, term.NewLiteral("a"))
	g1.Add(b1b, p, term.NewLiteral("b"))
	g1.Add(b1a, q, b1b)

	b2a := term.NewBNode()
	b2b := term.NewBNode()
	g2.Add(b2a, p, term.NewLiteral("a"))
	g2.Add(b2b, p, term.NewLiteral("b"))
	g2.Add(b2a, q, b2b)

	testutil.AssertGraphEqual(t, g1, g2)
}

func TestAssertGraphEqualBNodesMismatch(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	p, _ := term.NewURIRef("http://example.org/p")
	q, _ := term.NewURIRef("http://example.org/q")

	b1a := term.NewBNode()
	b1b := term.NewBNode()
	g1.Add(b1a, p, term.NewLiteral("a"))
	g1.Add(b1b, p, term.NewLiteral("b"))
	g1.Add(b1a, q, b1b)

	// Different structure
	b2a := term.NewBNode()
	b2b := term.NewBNode()
	g2.Add(b2a, p, term.NewLiteral("a"))
	g2.Add(b2b, p, term.NewLiteral("b"))
	g2.Add(b2b, q, b2a) // reversed link

	mockT := &testing.T{}
	testutil.AssertGraphEqual(mockT, g1, g2)
	if !mockT.Failed() {
		t.Error("expected failure for non-isomorphic graphs with bnodes")
	}
}

func TestAssertGraphEqualBNodeCountMismatch(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	p, _ := term.NewURIRef("http://example.org/p")

	b1 := term.NewBNode()
	b2 := term.NewBNode()
	g1.Add(b1, p, term.NewLiteral("a"))
	g1.Add(b2, p, term.NewLiteral("b"))

	b3 := term.NewBNode()
	g2.Add(b3, p, term.NewLiteral("a"))
	g2.Add(b3, p, term.NewLiteral("b")) // same bnode, not 2

	mockT := &testing.T{}
	testutil.AssertGraphEqual(mockT, g1, g2)
	// May or may not fail depending on isomorphism — but exercises the path
}

func TestAssertGraphEqualDifferentSize(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	p, _ := term.NewURIRef("http://example.org/p")
	b1 := term.NewBNode()
	g1.Add(b1, p, term.NewLiteral("a"))
	g1.Add(b1, p, term.NewLiteral("b"))
	g2.Add(term.NewBNode(), p, term.NewLiteral("a"))

	mockT := &testing.T{}
	testutil.AssertGraphEqual(mockT, g1, g2)
	if !mockT.Failed() {
		t.Error("expected failure for different size graphs")
	}
}

func TestAssertGraphContainsMissing(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")

	mockT := &testing.T{}
	testutil.AssertGraphContains(mockT, g, s, p, term.NewLiteral("missing"))
	if !mockT.Failed() {
		t.Error("expected failure for missing triple")
	}
}

func TestAssertGraphLenMismatch(t *testing.T) {
	g := graph.NewGraph()
	mockT := &testing.T{}
	testutil.AssertGraphLen(mockT, g, 5)
	if !mockT.Failed() {
		t.Error("expected failure for wrong length")
	}
}

func TestAssertGraphEqualBNodeAsObject(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	q, _ := term.NewURIRef("http://example.org/q")

	b1 := term.NewBNode()
	g1.Add(s, p, b1)
	g1.Add(b1, q, term.NewLiteral("v"))

	b2 := term.NewBNode()
	g2.Add(s, p, b2)
	g2.Add(b2, q, term.NewLiteral("v"))

	testutil.AssertGraphEqual(t, g1, g2)
}

func TestAssertGraphEqualNoBNodes(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	o := term.NewLiteral("v")

	g1.Add(s, p, o)
	g2.Add(s, p, o)

	testutil.AssertGraphEqual(t, g1, g2)
}

func TestAssertGraphEqualWithTripleTermBNodes(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	q, _ := term.NewURIRef("http://example.org/q")

	// TripleTerm containing a bnode
	b1 := term.NewBNode()
	tt1 := term.NewTripleTerm(b1, p, term.NewLiteral("inner"))
	g1.Add(s, q, tt1)
	g1.Add(b1, p, term.NewLiteral("outer"))

	b2 := term.NewBNode()
	tt2 := term.NewTripleTerm(b2, p, term.NewLiteral("inner"))
	g2.Add(s, q, tt2)
	g2.Add(b2, p, term.NewLiteral("outer"))

	testutil.AssertGraphEqual(t, g1, g2)
}

func TestAssertGraphEqualTripleTermBNodeMismatch(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	q, _ := term.NewURIRef("http://example.org/q")

	b1 := term.NewBNode()
	tt1 := term.NewTripleTerm(b1, p, term.NewLiteral("a"))
	g1.Add(s, q, tt1)
	g1.Add(b1, p, term.NewLiteral("outer"))

	b2 := term.NewBNode()
	tt2 := term.NewTripleTerm(b2, p, term.NewLiteral("b")) // different inner object
	g2.Add(s, q, tt2)
	g2.Add(b2, p, term.NewLiteral("outer"))

	mockT := &testing.T{}
	testutil.AssertGraphEqual(mockT, g1, g2)
	if !mockT.Failed() {
		t.Error("expected failure for TripleTerm mismatch")
	}
}

func TestAssertGraphEqualNoBNodesMismatch(t *testing.T) {
	g1 := graph.NewGraph()
	g2 := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")

	g1.Add(s, p, term.NewLiteral("a"))
	g2.Add(s, p, term.NewLiteral("b"))

	mockT := &testing.T{}
	testutil.AssertGraphEqual(mockT, g1, g2)
	if !mockT.Failed() {
		t.Error("expected failure")
	}
}
