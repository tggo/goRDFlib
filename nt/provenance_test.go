package nt

import (
	"fmt"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

func TestProvenanceTracksLineNumbers(t *testing.T) {
	input := `# a comment

<http://example.org/s1> <http://example.org/p> <http://example.org/o1> .
<http://example.org/s2> <http://example.org/p> "literal" .

<http://example.org/s3> <http://example.org/p> <http://example.org/o3> .
`
	prov := map[string]int{}
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input), WithProvenance(
		func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, lineNum int) {
			prov[s.String()] = lineNum
		}))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := map[string]int{
		"http://example.org/s1": 3,
		"http://example.org/s2": 4,
		"http://example.org/s3": 6,
	}
	if len(prov) != len(want) {
		t.Fatalf("got %d provenance entries, want %d: %v", len(prov), len(want), prov)
	}
	for s, line := range want {
		if prov[s] != line {
			t.Errorf("subject %s: got line %d, want %d", s, prov[s], line)
		}
	}
}

// Provenance must be reported once per parsed triple, even when the same triple
// appears on multiple lines (the graph deduplicates, the callback does not).
func TestProvenanceReportsDuplicates(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> <http://example.org/o> .
<http://example.org/s> <http://example.org/p> <http://example.org/o> .
`
	var lines []int
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input), WithProvenance(
		func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, lineNum int) {
			lines = append(lines, lineNum)
		}))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(lines) != 2 || lines[0] != 1 || lines[1] != 2 {
		t.Fatalf("got lines %v, want [1 2]", lines)
	}
}

// A nil provenance handler (the default) must not affect parsing.
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

// BenchmarkNTParseNoProvenance and BenchmarkNTParseWithProvenance confirm the
// default (unset) path carries no per-triple overhead from the feature.
func benchProvInput() string {
	var b strings.Builder
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&b, "<http://example.org/s%d> <http://example.org/p> \"value number %d here\" .\n", i, i)
	}
	return b.String()
}

func BenchmarkNTParseNoProvenance(b *testing.B) {
	input := benchProvInput()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g := rdflibgo.NewGraph()
		if err := Parse(g, strings.NewReader(input)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNTParseWithProvenance(b *testing.B) {
	input := benchProvInput()
	sink := 0
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g := rdflibgo.NewGraph()
		err := Parse(g, strings.NewReader(input), WithProvenance(
			func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, lineNum int) {
				sink += lineNum
			}))
		if err != nil {
			b.Fatal(err)
		}
	}
	_ = sink
}
