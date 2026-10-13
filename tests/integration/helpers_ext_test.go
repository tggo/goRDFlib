package integration_test

import (
	"context"
	"testing"

	. "github.com/tggo/goRDFlib"
)

func makeSPARQLGraphExt(t *testing.T) *Graph {
	t.Helper()
	g := NewGraph()
	g.Bind(t.Context(), "ex", NewURIRefUnsafe("http://example.org/"))
	alice, _ := NewURIRef("http://example.org/Alice")
	bob, _ := NewURIRef("http://example.org/Bob")
	charlie, _ := NewURIRef("http://example.org/Charlie")
	name, _ := NewURIRef("http://example.org/name")
	age, _ := NewURIRef("http://example.org/age")
	knows, _ := NewURIRef("http://example.org/knows")
	person, _ := NewURIRef("http://example.org/Person")

	g.Add(t.Context(), alice, RDF.Type, person)
	g.Add(t.Context(), alice, name, NewLiteral("Alice"))
	g.Add(t.Context(), alice, age, NewLiteral(30))
	g.Add(t.Context(), alice, knows, bob)

	g.Add(t.Context(), bob, RDF.Type, person)
	g.Add(t.Context(), bob, name, NewLiteral("Bob"))
	g.Add(t.Context(), bob, age, NewLiteral(25))
	g.Add(t.Context(), bob, knows, charlie)

	g.Add(t.Context(), charlie, RDF.Type, person)
	g.Add(t.Context(), charlie, name, NewLiteral("Charlie"))
	g.Add(t.Context(), charlie, age, NewLiteral(35))
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
	g.Add(t.Context(), a, p, b)
	g.Add(t.Context(), b, p, c)
	g.Add(t.Context(), c, p, d)
	g.Add(t.Context(), a, q, c)
	return g
}

func collectPairsExt(g *Graph, path Path, subj Subject, obj Term) [][2]string {
	var pairs [][2]string
	path.Eval(context.Background(), g, subj, obj)(func(pair_ TermPair, _ error) bool {
		s, o := pair_.First, pair_.Second
		pairs = append(pairs, [2]string{s.N3(), o.N3()})
		return true
	})
	return pairs
}
