package nq

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// RDFLib #162 — Language tags with uppercase subtags like @en-US
func TestLanguageTagUppercase(t *testing.T) {
	input := `<http://ex/s> <http://ex/p> "hello"@en-US .
`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err != nil {
		t.Fatalf("#162: language tag @en-US parse failed: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// N-Quads with graph context
func TestNQuadsWithGraph(t *testing.T) {
	input := `<http://ex/s> <http://ex/p> "val" <http://ex/g1> .
<http://ex/s2> <http://ex/p> "val2" .
`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err != nil {
		t.Fatalf("N-Quads with graph: %v", err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", g.Len())
	}
}
