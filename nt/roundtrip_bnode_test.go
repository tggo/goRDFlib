package nt_test

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/turtle"
)

// TestBlankNodeLabelsStableAcrossRoundtrips guards the choice of scoping by
// replacement rather than by prefix: a document that goes through
// parse → serialize → parse repeatedly must not accumulate a scope prefix on
// every hop. The bound is the width of one generated identifier.
func TestBlankNodeLabelsStableAcrossRoundtrips(t *testing.T) {
	const src = "_:b1 <urn:p> _:b1 .\n_:b1 <urn:q> _:b2 .\n_:b2 <urn:q> _:b1 .\n"
	width := len(rdflibgo.NewBNode().Value())

	type codec struct {
		name      string
		parse     func(*rdflibgo.Graph, string) error
		serialize func(*rdflibgo.Graph) (string, error)
	}
	codecs := []codec{
		{"nt",
			func(g *rdflibgo.Graph, s string) error { return nt.Parse(g, strings.NewReader(s)) },
			func(g *rdflibgo.Graph) (string, error) {
				var b strings.Builder
				err := nt.Serialize(g, &b)
				return b.String(), err
			}},
		{"turtle",
			func(g *rdflibgo.Graph, s string) error { return turtle.Parse(g, strings.NewReader(s)) },
			func(g *rdflibgo.Graph) (string, error) {
				var b strings.Builder
				err := turtle.Serialize(g, &b)
				return b.String(), err
			}},
	}
	for _, c := range codecs {
		t.Run(c.name, func(t *testing.T) {
			doc := src
			for hop := 1; hop <= 3; hop++ {
				g := rdflibgo.NewGraph()
				if err := c.parse(g, doc); err != nil {
					t.Fatalf("hop %d: %v", hop, err)
				}
				if g.Len() != 3 {
					t.Fatalf("hop %d: %d triples, want 3", hop, g.Len())
				}
				for tr := range g.Triples(nil, nil, nil) {
					for _, node := range []rdflibgo.Term{tr.Subject, tr.Object} {
						if bn, ok := node.(rdflibgo.BNode); ok && len(bn.Value()) != width {
							t.Fatalf("hop %d: blank node label %q is %d bytes, want %d — labels are growing per hop", hop, bn.Value(), len(bn.Value()), width)
						}
					}
				}
				var err error
				if doc, err = c.serialize(g); err != nil {
					t.Fatalf("hop %d: %v", hop, err)
				}
			}
		})
	}
}
