package nt

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/provenance"
)

func TestPreserveBlankNodeIDs(t *testing.T) {
	const input = "_:same <urn:p> _:same ."
	g := rdflibgo.NewGraph()
	for range 2 {
		if err := Parse(g, strings.NewReader(input), WithPreserveBlankNodeIDs()); err != nil {
			t.Fatal(err)
		}
	}
	if g.Len() != 1 {
		t.Fatalf("explicit label preservation did not reuse identity: %d", g.Len())
	}
	for triple := range g.Triples(nil, nil, nil) {
		if !triple.Subject.Equal(rdflibgo.NewBNode("same")) || !triple.Object.Equal(triple.Subject) {
			t.Fatal("source labels were not preserved")
		}
	}
}

func TestBlankNodeNestedAndRetryScope(t *testing.T) {
	const input = "_:same <urn:p> <<( _:same <urn:p> _:other )>> .\n_:same <urn:q> _:other\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input), WithErrorHandler(func(line int, text string, err error) (string, bool) {
		return text + " .", true
	})); err != nil {
		t.Fatal(err)
	}
	var outer rdflibgo.Subject
	var inner rdflibgo.TripleTerm
	for triple := range g.Triples(nil, nil, nil) {
		if outer != nil && !outer.Equal(triple.Subject) {
			t.Fatal("retry created a different scope")
		}
		outer = triple.Subject
		if value, ok := triple.Object.(rdflibgo.TripleTerm); ok {
			inner = value
		}
	}
	if outer == nil || !outer.Equal(inner.Subject()) {
		t.Fatal("nested triple term lost document scope")
	}
}

func TestBlankNodeParseScope(t *testing.T) {
	const input = "_:same <urn:p> _:same .\n_:same <urn:q> _:other .\n"
	g := rdflibgo.NewGraph()
	idx := provenance.NewIndex()
	for range 2 {
		if err := Parse(g, strings.NewReader(input), WithProvenance(idx.Triple)); err != nil {
			t.Fatal(err)
		}
	}
	if g.Len() != 4 {
		t.Fatalf("independent parses collapsed blank nodes: got %d triples", g.Len())
	}
	for triple := range g.Triples(nil, nil, nil) {
		line, ok := idx.Line(triple.Subject, triple.Predicate, triple.Object)
		if !ok {
			t.Fatal("provenance does not match parsed terms")
		}
		if triple.Predicate.Value() == "urn:p" && (!triple.Subject.Equal(triple.Object) || line != 1) {
			t.Fatal("repeated label lost its identity or source line")
		}
		if triple.Predicate.Value() == "urn:q" && (triple.Subject.Equal(triple.Object) || line != 2) {
			t.Fatal("distinct labels share an identity or source line")
		}
	}
}

func TestBlankNodeStreamScope(t *testing.T) {
	const input = "_:same <urn:p> _:same .\n_:same <urn:q> _:same .\n"
	var previous rdflibgo.Subject
	for range 2 {
		var first rdflibgo.Subject
		err := ParseStream(strings.NewReader(input), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error {
			if !s.Equal(o) || (first != nil && !first.Equal(s)) {
				t.Fatal("stream scope changed within a document")
			}
			first = s
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if previous != nil && previous.Equal(first) {
			t.Fatal("independent streams share blank nodes")
		}
		previous = first
	}
}
