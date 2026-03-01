package rdflibgo_test

import (
	"testing"

	. "github.com/tggo/goRDFlib"
)

func makeSPARQLGraphExt(t *testing.T) *Graph {
	t.Helper()
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	alice, _ := NewURIRef("http://example.org/Alice")
	bob, _ := NewURIRef("http://example.org/Bob")
	charlie, _ := NewURIRef("http://example.org/Charlie")
	name, _ := NewURIRef("http://example.org/name")
	age, _ := NewURIRef("http://example.org/age")
	knows, _ := NewURIRef("http://example.org/knows")
	person, _ := NewURIRef("http://example.org/Person")

	g.Add(alice, RDF.Type, person)
	g.Add(alice, name, NewLiteral("Alice"))
	g.Add(alice, age, NewLiteral(30))
	g.Add(alice, knows, bob)

	g.Add(bob, RDF.Type, person)
	g.Add(bob, name, NewLiteral("Bob"))
	g.Add(bob, age, NewLiteral(25))
	g.Add(bob, knows, charlie)

	g.Add(charlie, RDF.Type, person)
	g.Add(charlie, name, NewLiteral("Charlie"))
	g.Add(charlie, age, NewLiteral(35))
	return g
}

func makePathGraphExt(t *testing.T) *Graph {
	t.Helper()
	g := NewGraph()
	a, _ := NewURIRef("http://example.org/a")
	b, _ := NewURIRef("http://example.org/b")
	c, _ := NewURIRef("http://example.org/c")
	d, _ := NewURIRef("http://example.org/d")
	p, _ := NewURIRef("http://example.org/p")
	q, _ := NewURIRef("http://example.org/q")
	g.Add(a, p, b)
	g.Add(b, p, c)
	g.Add(c, p, d)
	g.Add(a, q, c)
	return g
}

func collectPairsExt(g *Graph, path Path, subj Subject, obj Term) [][2]string {
	var pairs [][2]string
	path.Eval(g, subj, obj)(func(s, o Term) bool {
		pairs = append(pairs, [2]string{s.N3(), o.N3()})
		return true
	})
	return pairs
}
