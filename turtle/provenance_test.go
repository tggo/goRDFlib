package turtle

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

func TestProvenanceTracksLineNumbers(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .

ex:s1 ex:p ex:o1 .

ex:s2 ex:p ex:o2 ,
            ex:o3 .
`
	// subject+object -> reported line
	prov := map[string]int{}
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input), WithProvenance(
		func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, lineNum int) {
			prov[s.String()+" "+o.String()] = lineNum
		}))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if got := prov["http://example.org/s1 http://example.org/o1"]; got != 3 {
		t.Errorf("s1 o1: got line %d, want 3", got)
	}
	// The object list spans lines 5-6; each object is reported at the line where
	// the parser sits when it is emitted.
	if got := prov["http://example.org/s2 http://example.org/o2"]; got != 5 {
		t.Errorf("s2 o2: got line %d, want 5", got)
	}
	if got := prov["http://example.org/s2 http://example.org/o3"]; got != 6 {
		t.Errorf("s2 o3: got line %d, want 6", got)
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
