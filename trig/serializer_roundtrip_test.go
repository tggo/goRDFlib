package trig

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store/badgerstore"
	"github.com/tggo/goRDFlib/testutil"
)

var (
	exP = rdflibgo.NewURIRefUnsafe("http://example.org/p")
	exQ = rdflibgo.NewURIRefUnsafe("http://example.org/q")
)

// newBadgerDataset returns a dataset whose store keeps named graphs apart.
// The default in-memory store does not separate them.
func newBadgerDataset(t *testing.T) *graph.Dataset {
	t.Helper()
	s, err := badgerstore.New(badgerstore.WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return graph.NewDataset(graph.WithStore(s))
}

// datasetAsGraph flattens a dataset into one graph so that blank node
// identity across graphs takes part in the isomorphism check: a triple in a
// named graph g gets the predicate <p> + " in " + <g>. Graph names must be
// IRIs. (Encoding quads as triple terms instead makes the isomorphism check
// exponentially slow, because blank nodes inside triple terms get no
// signature.)
func datasetAsGraph(ds *graph.Dataset) *rdflibgo.Graph {
	out := rdflibgo.NewGraph()
	def := ds.DefaultContext().Identifier()
	for g := range ds.Graphs() {
		suffix := ""
		if id := g.Identifier(); id != def {
			suffix = " in " + id.(rdflibgo.URIRef).Value()
		}
		g.Triples(nil, nil, nil)(func(t rdflibgo.Triple) bool {
			out.Add(t.Subject, rdflibgo.NewURIRefUnsafe(t.Predicate.Value()+suffix), t.Object)
			return true
		})
	}
	return out
}

// assertDatasetRoundTrip serializes ds, parses the output into a fresh
// dataset built by fresh, and checks the two are isomorphic.
func assertDatasetRoundTrip(t *testing.T, ds *graph.Dataset, fresh func(*testing.T) *graph.Dataset) {
	t.Helper()
	var b bytes.Buffer
	if err := SerializeDataset(ds, &b); err != nil {
		t.Fatal(err)
	}
	ds2 := fresh(t)
	if err := ParseDataset(ds2, strings.NewReader(b.String())); err != nil {
		t.Fatalf("output does not parse: %v\n%s", err, b.String())
	}
	testutil.AssertGraphEqual(t, datasetAsGraph(ds), datasetAsGraph(ds2))
	if t.Failed() {
		t.Logf("output:\n%s", b.String())
	}
}

func memoryDataset(*testing.T) *graph.Dataset { return graph.NewDataset() }

func TestSerializeListShapesRoundTripTrig(t *testing.T) {
	const prefixes = "@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> . @prefix ex: <http://example.org/> .\n"
	cases := map[string]string{
		"shared tail": `_:h rdf:first "head" ; rdf:rest _:t .
_:t rdf:first "shared" ; rdf:rest rdf:nil .
ex:a ex:items _:h . ex:b ex:items _:t .`,
		"list object of two triples": `ex:a ex:p _:l . ex:b ex:p _:l . _:l rdf:first 1 ; rdf:rest rdf:nil .`,
		"IRI list node":              `ex:l rdf:first 1 ; rdf:rest rdf:nil .`,
		"unreferenced list":          `_:l rdf:first 1 ; rdf:rest rdf:nil .`,
		"tail with extra triple":     `ex:a ex:p _:l . _:l rdf:first 1 ; rdf:rest _:m . _:m rdf:first 2 ; rdf:rest rdf:nil ; ex:q 3 .`,
		"rest cycle":                 `ex:a ex:p _:l . _:l rdf:first 1 ; rdf:rest _:m . _:m rdf:first 2 ; rdf:rest _:l .`,
		"item is own list":           `_:l rdf:first _:l ; rdf:rest rdf:nil .`,
		"item is own tail head":      `_:l rdf:first 1 ; rdf:rest _:m . _:m rdf:first _:l ; rdf:rest rdf:nil .`,
		"nested lists":               `ex:a ex:p ( 1 ( 2 ( 3 ) ) [ ex:q ( 4 ) ] ) .`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			ds := graph.NewDataset()
			if err := ParseDataset(ds, strings.NewReader(prefixes+src)); err != nil {
				t.Fatal(err)
			}
			assertDatasetRoundTrip(t, ds, memoryDataset)
		})
	}
}

// shapeGen builds random datasets out of the structures the serializer
// abbreviates: collections (well-formed and broken), blank node trees, chains
// and cycles. With graphs > 1 each triple goes to a random graph, so blank
// nodes end up shared between graphs.
type shapeGen struct {
	r      *rand.Rand
	ds     *graph.Dataset
	graphs []*graph.Graph
	bnodes []rdflibgo.BNode
	n      int
}

func newShapeGen(seed int, ds *graph.Dataset, graphs int) *shapeGen {
	s := &shapeGen{r: rand.New(rand.NewPCG(uint64(seed), 11)), ds: ds}
	s.graphs = append(s.graphs, ds.DefaultContext())
	for i := 1; i < graphs; i++ {
		s.graphs = append(s.graphs, ds.Graph(rdflibgo.NewURIRefUnsafe(fmt.Sprintf("http://example.org/g%d", i))))
	}
	return s
}

func (s *shapeGen) add(subj rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) {
	s.graphs[s.r.IntN(len(s.graphs))].Add(subj, p, o)
}

func (s *shapeGen) bnode() rdflibgo.BNode {
	s.n++
	b := rdflibgo.NewBNode(fmt.Sprintf("b%d", s.n))
	s.bnodes = append(s.bnodes, b)
	return b
}

func (s *shapeGen) iri() rdflibgo.URIRef {
	return rdflibgo.NewURIRefUnsafe(fmt.Sprintf("http://example.org/n%d", s.r.IntN(6)))
}

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
			s.add(b, exQ, s.item(depth-1))
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
		s.add(c, rdflibgo.RDF.First, s.item(depth))
		var rest rdflibgo.Term = rdflibgo.RDF.Nil
		if i+1 < len(cells) {
			rest = cells[i+1]
		}
		switch s.r.IntN(20) {
		case 0:
			rest = cells[0]
		case 1:
			continue
		case 2:
			s.add(c, exP, rdflibgo.NewLiteral(i))
		case 3:
			s.add(c, rdflibgo.RDF.First, rdflibgo.NewLiteral("extra"))
		case 4:
			s.add(s.iri(), exP, c)
		}
		s.add(c, rdflibgo.RDF.Rest, rest)
	}
	return cells[0]
}

func (s *shapeGen) build() {
	for range 1 + s.r.IntN(4) {
		switch s.r.IntN(5) {
		case 0:
			s.list(1)
		case 1:
			first := s.bnode()
			prev := first
			for range s.r.IntN(5) {
				next := s.bnode()
				s.add(prev, exP, next)
				prev = next
			}
			if s.r.IntN(2) == 0 {
				s.add(prev, exP, first)
			}
		default:
			var subj rdflibgo.Subject = s.iri()
			if s.r.IntN(3) == 0 {
				subj = s.existing()
			}
			s.add(subj, exP, s.item(2))
		}
	}
}

// Round-trip property on the default graph: parse(serialize(ds)) is
// isomorphic to ds for many generated shapes.
func TestSerializeRoundTripGeneratedShapesTrig(t *testing.T) {
	seeds := 300
	if testing.Short() {
		seeds = 100
	}
	for seed := range seeds {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			s := newShapeGen(seed, graph.NewDataset(), 1)
			s.build()
			assertDatasetRoundTrip(t, s.ds, memoryDataset)
		})
	}
}

// rdflib #3524: blank nodes inside triple terms lost their identity, and were
// inlined as [ ... ] inside the triple term, which RDF 1.2 ttObject forbids.
func TestSerializeBNodeInTripleTermKeepsIdentityTrig(t *testing.T) {
	const prefix = "@prefix : <http://example.org/> . @prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .\n"
	cases := map[string]string{
		"subject in triple term":         `_:b :p :o . :x :q <<( _:b :p :o )>> .`,
		"object in triple term, inlined": `:x :q <<( :s :p _:b )>> . :y :z _:b . _:b :r 1 .`,
		"object only in triple term":     `:x :q <<( :s :p _:b )>> . _:b :r 1 .`,
		"nested triple term":             `:x :q <<( :s :p <<( _:b :p _:c )>> )>> . _:b :r _:c . _:c :r 2 .`,
		"list cell in triple term":       `:x :q <<( :s :p _:l )>> . :y :z _:l . _:l rdf:first 1 ; rdf:rest rdf:nil .`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			ds := graph.NewDataset()
			if err := ParseDataset(ds, strings.NewReader(prefix+src)); err != nil {
				t.Fatal(err)
			}
			assertDatasetRoundTrip(t, ds, memoryDataset)
		})
	}
}

// A blank node label is scoped to the document, not to a graph block.
// References were counted per graph, so a node used as an object in one graph
// and as a subject in another was written as [] or inlined, and the link
// between the graphs was lost.
func TestSerializeBNodeSharedAcrossGraphs(t *testing.T) {
	const prefix = "@prefix : <http://example.org/> . @prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .\n"
	cases := map[string]string{
		"object here, subject there": `:s :p _:x . :g { _:x :q :o . }`,
		"inlined in one graph":       `:g1 { :s :p _:x . _:x :q 1 . } :g2 { _:x :q 2 . }`,
		"subject in two graphs":      `:g1 { _:x :q 1 . } :g2 { _:x :q 2 . }`,
		"list cell used elsewhere":   `:s :p _:l . _:l rdf:first 1 ; rdf:rest rdf:nil . :g { _:l :q 3 . }`,
		"list split over graphs":     `:s :p _:l . _:l rdf:first 1 ; rdf:rest _:m . :g { _:m rdf:first 2 ; rdf:rest rdf:nil . }`,
		"triple term across graphs":  `:s :p <<( _:x :q 1 )>> . :g { _:x :q 2 . }`,
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			ds := newBadgerDataset(t)
			if err := ParseDataset(ds, strings.NewReader(prefix+src)); err != nil {
				t.Fatal(err)
			}
			assertDatasetRoundTrip(t, ds, newBadgerDataset)
		})
	}
}

// Round-trip property over several graphs sharing blank nodes.
func TestSerializeRoundTripGeneratedDatasets(t *testing.T) {
	seeds := 150
	if testing.Short() {
		seeds = 50
	}
	for seed := range seeds {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			s := newShapeGen(seed, newBadgerDataset(t), 3)
			s.build()
			assertDatasetRoundTrip(t, s.ds, newBadgerDataset)
		})
	}
}
