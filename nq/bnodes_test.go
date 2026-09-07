package nq

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

func TestPreserveBlankNodeIDs(t *testing.T) {
	const input = "_:same <urn:p> _:same _:same ."
	for range 2 {
		calls := 0
		if err := ParseStream(strings.NewReader(input), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, g rdflibgo.Term) error {
			calls++
			if !s.Equal(rdflibgo.NewBNode("same")) || !s.Equal(o) || !s.Equal(g) {
				t.Fatal("source labels were not preserved")
			}
			return nil
		}, WithPreserveBlankNodeIDs()); err != nil {
			t.Fatal(err)
		}
		if calls != 1 {
			t.Fatalf("got %d quads", calls)
		}
	}
}

func TestBlankNodeRetryAndProvenance(t *testing.T) {
	const input = "_:same <urn:p> <<( _:same <urn:p> _:other )>> _:same .\n_:same <urn:q> _:other _:same\n"
	var subject rdflibgo.Subject
	var object rdflibgo.Term
	calls := 0
	err := ParseStream(strings.NewReader(input), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, g rdflibgo.Term) error {
		if !s.Equal(g) || (subject != nil && !s.Equal(subject)) {
			t.Fatal("retry or graph name changed the scope")
		}
		if inner, ok := o.(rdflibgo.TripleTerm); ok && !s.Equal(inner.Subject()) {
			t.Fatal("nested triple term changed the scope")
		}
		subject, object = s, o
		return nil
	}, WithErrorHandler(func(line int, text string, err error) (string, bool) {
		return text + " .", true
	}), WithProvenance(func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, g rdflibgo.Term, line int) {
		calls++
		if !s.Equal(subject) || !o.Equal(object) || !s.Equal(g) || line != calls {
			t.Fatal("provenance did not receive the dispatched terms and source line")
		}
	}))
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("got %d provenance calls", calls)
	}
}

func TestBlankNodeStreamScope(t *testing.T) {
	const input = "_:same <urn:p> _:same _:same .\n_:same <urn:p> _:same <urn:graph> .\n"
	var previous rdflibgo.Subject
	for range 2 {
		var first rdflibgo.Subject
		count := 0
		err := ParseStream(strings.NewReader(input), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, g rdflibgo.Term) error {
			if !s.Equal(o) || (first != nil && !first.Equal(s)) {
				t.Fatal("named graphs in one document must share label identity")
			}
			if count == 0 && !s.Equal(g) {
				t.Fatal("blank graph name must use the document scope")
			}
			first = s
			count++
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if count != 2 {
			t.Fatalf("got %d quads", count)
		}
		if previous != nil && previous.Equal(first) {
			t.Fatal("independent streams share blank nodes")
		}
		previous = first
	}
}

func TestBlankNodeGraphScope(t *testing.T) {
	const input = "_:same <urn:p> _:same <urn:graph> ."
	g := rdflibgo.NewGraph()
	for range 2 {
		if err := Parse(g, strings.NewReader(input)); err != nil {
			t.Fatal(err)
		}
	}
	if g.Len() != 2 {
		t.Fatalf("independent parses collapsed blank nodes: got %d triples", g.Len())
	}
}
