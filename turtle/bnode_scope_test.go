package turtle

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// TestBlankNodeDocumentScope checks label reuse within a document and isolation
// between documents, even when their destination graph or base IRI is the same.
func TestBlankNodeDocumentScope(t *testing.T) {
	const input = `_:x <p> _:x . _:x <q> _:y . [] <p> [] .`
	for _, sameGraph := range []bool{false, true} {
		name := "separate graphs"
		if sameGraph {
			name = "same graph"
		}
		t.Run(name, func(t *testing.T) {
			g := rdflibgo.NewGraph()
			var previous rdflibgo.Subject
			for i := 0; i < 2; i++ {
				if !sameGraph {
					g = rdflibgo.NewGraph()
				}
				var labelled rdflibgo.Subject
				var anonymous rdflibgo.Subject
				calls := 0
				err := Parse(g, strings.NewReader(input), WithBase("http://example.org/"), WithProvenance(
					func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, line int) {
						calls++
						if line != 1 || !g.Contains(s, p, o) {
							t.Errorf("provenance does not match inserted triple: %v %v %v at %d", s, p, o, line)
						}
						switch calls {
						case 1:
							labelled = s
							if !s.Equal(o) {
								t.Error("repeated label has different identities")
							}
						case 2:
							if !s.Equal(labelled) || s.Equal(o) {
								t.Error("labels were not reused or distinct labels merged")
							}
						case 3:
							anonymous = s
							if s.Equal(o) || s.Equal(labelled) || o.Equal(labelled) {
								t.Error("anonymous nodes collide")
							}
						}
					},
				))
				if err != nil {
					t.Fatal(err)
				}
				if calls != 3 || anonymous == nil {
					t.Fatalf("got %d provenance calls, want 3", calls)
				}
				if labelled.Equal(rdflibgo.NewBNode("x")) {
					t.Error("default parse preserved the lexical label")
				}
				if previous != nil && previous.Equal(labelled) {
					t.Error("independent parses share a blank node")
				}
				previous = labelled
			}
			want := 3
			if sameGraph {
				want = 6
			}
			if g.Len() != want {
				t.Errorf("got %d triples, want %d", g.Len(), want)
			}
		})
	}
}

// TestBlankNodeRDF12Scope checks that nested triple terms and explicit reifiers
// use the same identity as ordinary labels, with and without preservation.
func TestBlankNodeRDF12Scope(t *testing.T) {
	const input = `_:x <http://example.org/p> <<( _:x <http://example.org/q> <<( _:x <http://example.org/q> _:x )>> )>> ~ _:x .`
	for _, preserve := range []bool{false, true} {
		g := rdflibgo.NewGraph()
		var previous rdflibgo.Subject
		for i := 0; i < 2; i++ {
			var labelled rdflibgo.Subject
			calls := 0
			opts := []Option{WithProvenance(func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, line int) {
				calls++
				if !g.Contains(s, p, o) || line != 1 {
					t.Error("RDF 1.2 provenance does not match inserted triple")
				}
				if labelled == nil {
					labelled = s
				}
				if !s.Equal(labelled) {
					t.Error("reifier and subject have different identities")
				}
				// Every subject and the innermost object must be the same node,
				// including the extra triple-term layer emitted for rdf:reifies.
				for {
					tt, ok := o.(rdflibgo.TripleTerm)
					if !ok {
						break
					}
					if !tt.Subject().Equal(labelled) {
						t.Error("nested subject has a different identity")
					}
					o = tt.Object()
				}
				if !o.Equal(labelled) {
					t.Error("nested object has a different identity")
				}
			})}
			if preserve {
				opts = append(opts, WithPreserveBlankNodeIDs())
			}
			if err := Parse(g, strings.NewReader(input), opts...); err != nil {
				t.Fatal(err)
			}
			if calls != 2 {
				t.Fatalf("got %d callbacks, want 2", calls)
			}
			if labelled.Equal(rdflibgo.NewBNode("x")) != preserve {
				t.Errorf("lexical label preservation = %v, want %v", !preserve, preserve)
			}
			if previous != nil && previous.Equal(labelled) != preserve {
				t.Errorf("identity reuse between documents differs from preserve=%v", preserve)
			}
			previous = labelled
		}
		want := 4
		if preserve {
			want = 2
		}
		if g.Len() != want {
			t.Errorf("got %d triples, want %d", g.Len(), want)
		}
	}
}

// TestBlankNodeGeneratedLabelSeparation ensures a source label that copies a
// generated ID cannot capture that node, and preservation never reuses [] nodes.
func TestBlankNodeGeneratedLabelSeparation(t *testing.T) {
	generated := rdflibgo.NewBNode()
	input := strings.ReplaceAll(`_:LABEL <http://example.org/p> [] .`, "LABEL", generated.Value())
	for _, preserve := range []bool{false, true} {
		g := rdflibgo.NewGraph()
		var previous rdflibgo.Term
		for i := 0; i < 2; i++ {
			opts := []Option{WithProvenance(func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, line int) {
				if s.Equal(generated) != preserve || s.Equal(o) || o.Equal(generated) || (previous != nil && o.Equal(previous)) {
					t.Error("labelled and generated identities are not separate")
				}
				previous = o
			})}
			if preserve {
				opts = append(opts, WithPreserveBlankNodeIDs())
			}
			if err := Parse(g, strings.NewReader(input), opts...); err != nil {
				t.Fatal(err)
			}
		}
		if g.Len() != 2 {
			t.Errorf("got %d triples, want 2 distinct anonymous objects", g.Len())
		}
	}
}
