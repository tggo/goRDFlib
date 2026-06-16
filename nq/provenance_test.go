package nq

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

func TestProvenanceTracksLineNumbersAndGraph(t *testing.T) {
	input := `<http://example.org/s1> <http://example.org/p> <http://example.org/o1> <http://example.org/g> .

<http://example.org/s2> <http://example.org/p> <http://example.org/o2> .
`
	type entry struct {
		line  int
		graph string
	}
	prov := map[string]entry{}
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input), WithProvenance(
		func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term, lineNum int) {
			gs := ""
			if graph != nil {
				gs = graph.String()
			}
			prov[s.String()] = entry{lineNum, gs}
		}))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got := prov["http://example.org/s1"]; got.line != 1 || got.graph != "http://example.org/g" {
		t.Errorf("s1: got %+v, want line 1 graph http://example.org/g", got)
	}
	if got := prov["http://example.org/s2"]; got.line != 3 || got.graph != "" {
		t.Errorf("s2: got %+v, want line 3 default graph", got)
	}
}

func TestProvenanceUnsetIsNoop(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> <http://example.org/o> .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n := g.Len(); n != 1 {
		t.Fatalf("got %d triples, want 1", n)
	}
}
