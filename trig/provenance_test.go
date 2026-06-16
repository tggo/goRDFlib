package trig

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
)

func TestProvenanceTracksLineAndGraph(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .

ex:s1 ex:p ex:o1 .

ex:g {
    ex:s2 ex:p ex:o2 .
}
`
	type entry struct {
		line  int
		graph string
	}
	prov := map[string]entry{}
	ds := graph.NewDataset()
	err := ParseDataset(ds, strings.NewReader(input), WithProvenance(
		func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, gr rdflibgo.Term, lineNum int) {
			gs := ""
			if gr != nil {
				gs = gr.String()
			}
			prov[s.String()] = entry{lineNum, gs}
		}))
	if err != nil {
		t.Fatalf("ParseDataset: %v", err)
	}

	if got := prov["http://example.org/s1"]; got.line != 3 {
		t.Errorf("s1: got line %d, want 3", got.line)
	}
	if got := prov["http://example.org/s2"]; got.line != 6 || got.graph != "http://example.org/g" {
		t.Errorf("s2: got %+v, want line 6 graph http://example.org/g", got)
	}
}

func TestProvenanceUnsetIsNoop(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .
ex:s ex:p ex:o .
`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n := g.Len(); n != 1 {
		t.Fatalf("got %d triples, want 1", n)
	}
}
