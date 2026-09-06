package trig

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
)

// TestBlankNodeDatasetScope checks that one document shares labels across its
// graphs, but another parse with the same base cannot merge nodes or graph names.
func TestBlankNodeDatasetScope(t *testing.T) {
	const input = `_:x <p> _:x . GRAPH _:x { _:x <p> _:x . } _:x { _:x <q> _:y . } <g> { _:x <p> _:x . }`
	for _, sameDataset := range []bool{false, true} {
		name := "separate datasets"
		if sameDataset {
			name = "same dataset"
		}
		t.Run(name, func(t *testing.T) {
			ds := graph.NewDataset()
			var previous rdflibgo.Subject
			for i := 0; i < 2; i++ {
				if !sameDataset {
					ds = graph.NewDataset()
				}
				var labelled rdflibgo.Subject
				calls := 0
				err := ParseDataset(ds, strings.NewReader(input), WithBase("http://example.org/"), WithProvenance(
					func(s rdflibgo.Subject, p rdflibgo.URIRef, o, gr rdflibgo.Term, line int) {
						calls++
						if line != 1 || !ds.Graph(gr).Contains(s, p, o) {
							t.Errorf("provenance does not match inserted quad at line %d", line)
						}
						if labelled == nil {
							labelled = s
						}
						if !s.Equal(labelled) {
							t.Error("one document did not share its label across graphs")
						}
						if calls == 2 || calls == 3 {
							if !gr.Equal(labelled) {
								t.Error("graph label differs from triple label")
							}
						}
						if calls == 3 {
							if s.Equal(o) {
								t.Error("distinct labels merged")
							}
						} else if !s.Equal(o) {
							t.Error("repeated label differs")
						}
					},
				))
				if err != nil {
					t.Fatal(err)
				}
				if calls != 4 {
					t.Fatalf("got %d provenance calls, want 4", calls)
				}
				if labelled.Equal(rdflibgo.NewBNode("x")) {
					t.Error("default parse preserved the lexical label")
				}
				if previous != nil && previous.Equal(labelled) {
					t.Error("independent parses share a blank node graph label")
				}
				previous = labelled
			}
			// The default memory store counts the union, not individual contexts.
			// Each document contributes two unique triples and one blank graph name.
			want, wantGraphs := 2, 3
			if sameDataset {
				want, wantGraphs = 4, 4
			}
			if got := ds.DefaultContext().Len(); got != want {
				t.Errorf("dataset union has %d triples, want %d", got, want)
			}
			graphs := 0
			for range ds.Graphs() {
				graphs++
			}
			if graphs != wantGraphs {
				t.Errorf("got %d graph names, want %d", graphs, wantGraphs)
			}
		})
	}
}

// TestBlankNodeRDF12Scope checks nested labels, reifiers, graph labels, and
// anonymous nodes for both public entry points and both identity modes.
func TestBlankNodeRDF12Scope(t *testing.T) {
	generated := rdflibgo.NewBNode()
	input := strings.ReplaceAll(`GRAPH _:LABEL {
_:LABEL <http://example.org/p> <<( _:LABEL <http://example.org/q> <<( _:LABEL <http://example.org/q> _:LABEL )>> )>> ~ _:LABEL .
[] <http://example.org/anonymous> [] .
}`, "LABEL", generated.Value())
	for _, dataset := range []bool{false, true} {
		for _, preserve := range []bool{false, true} {
			ds := graph.NewDataset()
			g := rdflibgo.NewGraph()
			var previous rdflibgo.Subject
			anonymous := make(map[rdflibgo.BNode]bool, 4)
			for i := 0; i < 2; i++ {
				var labelled rdflibgo.Subject
				triples := make([]rdflibgo.Triple, 0, 3)
				opts := []Option{WithProvenance(func(s rdflibgo.Subject, p rdflibgo.URIRef, o, gr rdflibgo.Term, line int) {
					triples = append(triples, rdflibgo.Triple{Subject: s, Predicate: p, Object: o})
					if dataset && !ds.Graph(gr).Contains(s, p, o) {
						t.Error("provenance does not match inserted quad")
					}
					if p.Value() == "http://example.org/anonymous" {
						if line != 3 {
							t.Errorf("anonymous triple line = %d, want 3", line)
						}
						for _, node := range []rdflibgo.Term{s, o} {
							bn := node.(rdflibgo.BNode)
							if anonymous[bn] || bn.Equal(labelled) || bn.Equal(generated) {
								t.Error("anonymous nodes collide")
							}
							anonymous[bn] = true
						}
						return
					}
					if line != 2 {
						t.Errorf("nested triple line = %d, want 2", line)
					}
					if labelled == nil {
						labelled = s
					}
					if !s.Equal(labelled) || !gr.Equal(labelled) {
						t.Error("graph, reifier, and subject identities differ")
					}
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
				var err error
				if dataset {
					err = ParseDataset(ds, strings.NewReader(input), opts...)
				} else {
					err = Parse(g, strings.NewReader(input), opts...)
				}
				if err != nil {
					t.Fatal(err)
				}
				if len(triples) != 3 {
					t.Fatalf("got %d callbacks, want 3", len(triples))
				}
				if !dataset {
					for _, triple := range triples {
						if !g.Contains(triple.Subject, triple.Predicate, triple.Object) {
							t.Error("provenance does not match flattened graph")
						}
					}
				}
				if labelled.Equal(generated) != preserve || (previous != nil && previous.Equal(labelled) != preserve) {
					t.Errorf("label identity does not match preserve=%v", preserve)
				}
				previous = labelled
			}
			want := 6
			if preserve {
				want = 4
			}
			if dataset {
				g = ds.DefaultContext()
			}
			if g.Len() != want {
				t.Errorf("got %d triples, want %d", g.Len(), want)
			}
		}
	}
}

// TestBlankNodeFlattenedScope checks the Parse entry point separately because
// it flattens the dataset into a graph after parsing.
func TestBlankNodeFlattenedScope(t *testing.T) {
	const input = `_:x <http://example.org/p> _:x . <http://example.org/g> { _:x <http://example.org/p> _:x . }`
	for _, sameGraph := range []bool{false, true} {
		g := rdflibgo.NewGraph()
		var previous rdflibgo.Subject
		for i := 0; i < 2; i++ {
			if !sameGraph {
				g = rdflibgo.NewGraph()
			}
			var current rdflibgo.Subject
			err := Parse(g, strings.NewReader(input), WithBase("http://example.org/"), WithProvenance(
				func(s rdflibgo.Subject, p rdflibgo.URIRef, o, gr rdflibgo.Term, line int) {
					current = s
				},
			))
			if err != nil {
				t.Fatal(err)
			}
			if !g.Contains(current, rdflibgo.NewURIRefUnsafe("http://example.org/p"), current) {
				t.Error("provenance node does not match flattened graph")
			}
			if previous != nil && current.Equal(previous) {
				t.Error("independent Parse calls share a blank node")
			}
			previous = current
		}
		want := 1
		if sameGraph {
			want = 2
		}
		if g.Len() != want {
			t.Errorf("got %d triples, want %d", g.Len(), want)
		}
	}
}
