package trig

import (
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
)

// TestTripleTermObjectGrammar pins RDF 1.2 Turtle [34] ttObject ::= iri |
// BlankNode | literal | tripleTerm. The parser used to read the object with the
// general object rule, so a blank node property list — which asserts triples —
// was accepted inside a triple term.
func TestTripleTermObjectGrammar(t *testing.T) {
	const prefix = "@prefix : <http://example.org/> .\n"
	valid := []string{
		":x :q <<( :s :p :o )>> .",
		":x :q <<( :s :p <http://example.org/o> )>> .",
		":x :q <<( :s :p _:b )>> .",
		":x :q <<( :s :p [] )>> .",
		":x :q <<( :s :p [ ] )>> .",
		":x :q <<( [] :p :o )>> .",
		":x :q <<( :s :p \"v\"@en )>> .",
		":x :q <<( :s :p 42 )>> .",
		":x :q <<( :s :p true )>> .",
		":x :q <<( :s :p <<( :a :b :c )>> )>> .",
	}
	for _, body := range valid {
		src := prefix + body
		if err := ParseDataset(graph.NewDataset(), strings.NewReader(src)); err != nil {
			t.Errorf("rejected valid triple term %q: %v", body, err)
		}
	}
	invalid := []string{
		":x :q <<( :s :p [ :r 1 ] )>> .",
		":x :q <<( :s :p ( 1 2 ) )>> .",
		":x :q <<( :s :p << :a :b :c >> )>> .",
		":x :q <<( [ :r 1 ] :p :o )>> .",
	}
	for _, body := range invalid {
		src := prefix + body
		if err := ParseDataset(graph.NewDataset(), strings.NewReader(src)); err == nil {
			t.Errorf("accepted invalid triple term %q", body)
		}
	}
}
