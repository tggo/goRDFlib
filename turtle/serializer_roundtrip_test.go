package turtle

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/testutil"
)

// assertTurtleRoundTrip checks parse(serialize(g)) is isomorphic to g.
func assertTurtleRoundTrip(t *testing.T, g *rdflibgo.Graph, opts ...Option) {
	t.Helper()
	var b bytes.Buffer
	if err := Serialize(g, &b, opts...); err != nil {
		t.Fatal(err)
	}
	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(b.String())); err != nil {
		t.Fatalf("output does not parse: %v\n%s", err, b.String())
	}
	testutil.AssertGraphEqual(t, g, g2)
	if t.Failed() {
		t.Logf("output:\n%s", b.String())
	}
}

func mustParseTurtle(t *testing.T, src string) *rdflibgo.Graph {
	t.Helper()
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(src)); err != nil {
		t.Fatalf("parse: %v\n%s", err, src)
	}
	return g
}

var roundTripLayouts = map[string][]Option{
	"compact":       nil,
	"pretty":        {WithPretty()},
	"depth1":        {WithMaxNestDepth(1)},
	"pretty-depth2": {WithPretty(), WithMaxNestDepth(2)},
}

// Lists that are not well-formed collections used to lose triples: the
// serializer skipped their nodes as subjects and never wrote them.
func TestSerializeListShapesRoundTrip(t *testing.T) {
	const prefixes = "@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> . @prefix ex: <http://example.org/> .\n"
	cases := map[string]string{
		"shared tail": `_:h rdf:first "head" ; rdf:rest _:t .
_:t rdf:first "shared" ; rdf:rest rdf:nil .
ex:a ex:items _:h . ex:b ex:items _:t .`,
		"list object of two triples": `ex:a ex:p _:l . ex:b ex:p _:l .
_:l rdf:first 1 ; rdf:rest rdf:nil .`,
		"IRI list node":            `ex:l rdf:first 1 ; rdf:rest rdf:nil .`,
		"IRI tail node":            `ex:a ex:p _:l . _:l rdf:first 1 ; rdf:rest ex:t . ex:t rdf:first 2 ; rdf:rest rdf:nil .`,
		"unreferenced list":        `_:l rdf:first 1 ; rdf:rest rdf:nil .`,
		"unreferenced longer list": `_:l rdf:first 1 ; rdf:rest _:m . _:m rdf:first 2 ; rdf:rest rdf:nil .`,
		"head with extra triple":   `ex:a ex:p _:l . _:l rdf:first 1 ; rdf:rest _:m ; ex:q 3 . _:m rdf:first 2 ; rdf:rest rdf:nil .`,
		"tail with extra triple":   `ex:a ex:p _:l . _:l rdf:first 1 ; rdf:rest _:m . _:m rdf:first 2 ; rdf:rest rdf:nil ; ex:q 3 .`,
		"two firsts":               `ex:a ex:p _:l . _:l rdf:first 1, 2 ; rdf:rest rdf:nil .`,
		"missing rest":             `ex:a ex:p _:l . _:l rdf:first 1 .`,
		"rest cycle":               `ex:a ex:p _:l . _:l rdf:first 1 ; rdf:rest _:m . _:m rdf:first 2 ; rdf:rest _:l .`,
		"self rest":                `ex:a ex:p _:l . _:l rdf:first 1 ; rdf:rest _:l .`,
		"item is own list":         `_:l rdf:first _:l ; rdf:rest rdf:nil .`,
		"item is own tail head":    `_:l rdf:first 1 ; rdf:rest _:m . _:m rdf:first _:l ; rdf:rest rdf:nil .`,
		"list as subject":          `_:l rdf:first 1 ; rdf:rest rdf:nil . ex:a ex:p _:l . _:l ex:q 2 .`,
		"rest to dangling bnode":   `ex:a ex:p _:l . _:l rdf:first 1 ; rdf:rest _:x .`,
		"rest to literal":          `ex:a ex:p _:l . _:l rdf:first 1 ; rdf:rest "no" .`,
		"nested lists":             `ex:a ex:p ( 1 ( 2 ( 3 ) ) [ ex:q ( 4 ) ] ) .`,
		"empty list":               `ex:a ex:p () .`,
	}
	for name, src := range cases {
		g := mustParseTurtle(t, prefixes+src)
		for layout, opts := range roundTripLayouts {
			t.Run(name+"/"+layout, func(t *testing.T) {
				assertTurtleRoundTrip(t, g, opts...)
			})
		}
	}
}

// A well-formed collection is still written with the collection syntax.
func TestSerializeWellFormedListStaysCollection(t *testing.T) {
	g := mustParseTurtle(t, `@prefix ex: <http://example.org/> . ex:a ex:p ( 1 2 3 ) .`)
	var b bytes.Buffer
	if err := Serialize(g, &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "( 1 2 3 )") {
		t.Errorf("want collection syntax, got:\n%s", b.String())
	}
	// The tail of a list whose head is shared is still a collection.
	g = mustParseTurtle(t, `@prefix ex: <http://example.org/> . @prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
ex:a ex:p _:l . ex:b ex:p _:l . _:l rdf:first 1 ; rdf:rest ( 2 3 ) .`)
	b.Reset()
	if err := Serialize(g, &b); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "( 2 3 )") {
		t.Errorf("want collection syntax for the tail, got:\n%s", b.String())
	}
}

// shapeGen builds random graphs out of the structures serializers abbreviate:
// collections (well-formed and broken), blank node trees, chains and cycles.
type shapeGen struct {
	r      *rand.Rand
	g      *rdflibgo.Graph
	bnodes []rdflibgo.BNode
	n      int
}

var (
	exP = rdflibgo.NewURIRefUnsafe("http://example.org/p")
	exQ = rdflibgo.NewURIRefUnsafe("http://example.org/q")
)

func (s *shapeGen) bnode() rdflibgo.BNode {
	s.n++
	b := rdflibgo.NewBNode(fmt.Sprintf("b%d", s.n))
	s.bnodes = append(s.bnodes, b)
	return b
}

func (s *shapeGen) iri() rdflibgo.URIRef {
	return rdflibgo.NewURIRefUnsafe(fmt.Sprintf("http://example.org/n%d", s.r.IntN(6)))
}

// existing returns a blank node created earlier, or a fresh one.
func (s *shapeGen) existing() rdflibgo.BNode {
	if len(s.bnodes) == 0 || s.r.IntN(3) == 0 {
		return s.bnode()
	}
	return s.bnodes[s.r.IntN(len(s.bnodes))]
}

func (s *shapeGen) item(depth int) rdflibgo.Term {
	switch s.r.IntN(7) {
	case 0:
		return s.iri()
	case 1:
		return rdflibgo.NewLiteral(s.r.IntN(4))
	case 2:
		if depth > 0 {
			return s.list(depth - 1)
		}
		return rdflibgo.RDF.Nil
	case 3:
		b := s.bnode()
		if depth > 0 {
			s.g.Add(b, exQ, s.item(depth-1))
		}
		return b
	case 4:
		return s.existing()
	case 5:
		if s.r.IntN(2) == 0 {
			return s.tripleTerm(depth)
		}
		return rdflibgo.NewLiteral("x")
	default:
		return rdflibgo.NewLiteral("x")
	}
}

// tripleTerm returns an RDF 1.2 triple term that mentions blank nodes, which
// may also appear elsewhere in the graph.
func (s *shapeGen) tripleTerm(depth int) rdflibgo.TripleTerm {
	var subj rdflibgo.Subject = s.iri()
	if s.r.IntN(2) == 0 {
		subj = s.existing()
	}
	var obj rdflibgo.Term
	switch {
	case depth > 0 && s.r.IntN(4) == 0:
		obj = s.tripleTerm(depth - 1)
	case s.r.IntN(2) == 0:
		obj = s.existing()
	default:
		obj = rdflibgo.NewLiteral(s.r.IntN(3))
	}
	return rdflibgo.NewTripleTerm(subj, exP, obj)
}

// list builds a collection and then maybe breaks it.
func (s *shapeGen) list(depth int) rdflibgo.Term {
	length := s.r.IntN(4)
	if length == 0 {
		return rdflibgo.RDF.Nil
	}
	cells := make([]rdflibgo.Subject, length)
	for i := range cells {
		if s.r.IntN(12) == 0 {
			cells[i] = s.iri()
		} else {
			cells[i] = s.bnode()
		}
	}
	for i, c := range cells {
		s.g.Add(c, rdflibgo.RDF.First, s.item(depth))
		var rest rdflibgo.Term = rdflibgo.RDF.Nil
		if i+1 < len(cells) {
			rest = cells[i+1]
		}
		switch s.r.IntN(20) {
		case 0: // cycle back to the head
			rest = cells[0]
		case 1: // no rest at all
			continue
		case 2: // an extra triple on the cell
			s.g.Add(c, exP, rdflibgo.NewLiteral(i))
		case 3: // a second rdf:first
			s.g.Add(c, rdflibgo.RDF.First, rdflibgo.NewLiteral("extra"))
		case 4: // an outside reference to this cell (shared tail)
			s.g.Add(s.iri(), exP, c)
		}
		s.g.Add(c, rdflibgo.RDF.Rest, rest)
	}
	return cells[0]
}

func (s *shapeGen) build() {
	for range 1 + s.r.IntN(4) {
		switch s.r.IntN(5) {
		case 0: // an unreferenced collection
			s.list(1)
		case 1: // a chain of blank nodes, maybe closed into a cycle
			first := s.bnode()
			prev := first
			for range s.r.IntN(5) {
				next := s.bnode()
				s.g.Add(prev, exP, next)
				prev = next
			}
			if s.r.IntN(2) == 0 {
				s.g.Add(prev, exP, first)
			}
		default:
			var subj rdflibgo.Subject = s.iri()
			if s.r.IntN(3) == 0 {
				subj = s.existing()
			}
			s.g.Add(subj, exP, s.item(2))
		}
	}
}

// Round-trip property: parse(serialize(g)) is isomorphic to g for many
// generated shapes, in every layout.
func TestSerializeRoundTripGeneratedShapes(t *testing.T) {
	seeds := 400
	if testing.Short() {
		seeds = 150
	}
	for seed := range seeds {
		s := &shapeGen{r: rand.New(rand.NewPCG(uint64(seed), 7)), g: rdflibgo.NewGraph()}
		s.build()
		for layout, opts := range roundTripLayouts {
			t.Run(fmt.Sprintf("seed%d/%s", seed, layout), func(t *testing.T) {
				assertTurtleRoundTrip(t, s.g, opts...)
			})
		}
	}
}

// rdflib #3524: a blank node inside a triple term lost its identity. Its
// references there were not counted, so the node was written as [] or inlined
// as [ ... ] elsewhere, and inside the triple term it was inlined as
// [ ... ], which RDF 1.2 Turtle ttObject does not allow.
func TestSerializeBNodeInTripleTermKeepsIdentity(t *testing.T) {
	const prefix = "@prefix : <http://example.org/> . @prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .\n"
	cases := map[string]string{
		"subject in triple term":         `_:b :p :o . :x :q <<( _:b :p :o )>> .`,
		"object in triple term, inlined": `:x :q <<( :s :p _:b )>> . :y :z _:b . _:b :r 1 .`,
		"object only in triple term":     `:x :q <<( :s :p _:b )>> . _:b :r 1 .`,
		"nested triple term":             `:x :q <<( :s :p <<( _:b :p _:c )>> )>> . _:b :r _:c . _:c :r 2 .`,
		"list cell in triple term":       `:x :q <<( :s :p _:l )>> . :y :z _:l . _:l rdf:first 1 ; rdf:rest rdf:nil .`,
		"same node twice":                `:x :q <<( _:b :p _:b )>> .`,
	}
	for name, src := range cases {
		g := mustParseTurtle(t, prefix+src)
		for layout, opts := range roundTripLayouts {
			t.Run(name+"/"+layout, func(t *testing.T) {
				assertTurtleRoundTrip(t, g, opts...)
			})
		}
	}
}
