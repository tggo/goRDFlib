package rdflibgo

import "testing"

func TestAssertGraphEqual(t *testing.T) {
	g1 := NewGraph()
	g2 := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g1.Add(s, p, NewLiteral("hello"))
	g2.Add(s, p, NewLiteral("hello"))

	AssertGraphEqual(t, g1, g2)
}

func TestAssertGraphContains(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("hello"))
	AssertGraphContains(t, g, s, p, NewLiteral("hello"))
}

func TestAssertGraphLen(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("hello"))
	AssertGraphLen(t, g, 1)
}
