package rdfxml_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/provenance"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/term"
)

// FuzzParseProvenance checks the properties a source line has to satisfy for a
// diagnostic printed next to a file to be trustworthy: it must exist, and it
// must belong to a triple the parser actually produced.
//
// It also exercises the parser itself against arbitrary bytes, which nothing
// here did before — only SPARQL was fuzzed.
func FuzzParseProvenance(f *testing.F) {
	f.Add(`<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/a"><ex:p>v</ex:p></rdf:Description>
</rdf:RDF>`)
	f.Add(`<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"/>`)
	f.Add(`<`)
	f.Add(``)
	f.Add(`<a></a>`)

	// Seed from the W3C suite, which is the densest collection of legal and
	// deliberately illegal RDF/XML available.
	for _, pat := range []string{
		"../testdata/w3c/rdf-tests/rdf/rdf11/rdf-xml/*/*.rdf",
		"../testdata/w3c/rdf-tests/rdf/rdf12/rdf-xml/*/*.rdf",
	} {
		files, _ := filepath.Glob(pat)
		for i, file := range files {
			if i >= 60 {
				break
			}
			if data, err := os.ReadFile(file); err == nil && len(data) > 0 && len(data) < 8192 {
				f.Add(string(data))
			}
		}
	}

	f.Fuzz(func(t *testing.T, src string) {
		g := graph.NewGraph()
		idx := provenance.NewIndex()

		type record struct {
			triple term.Triple
			line   int
		}
		var seen []record

		err := rdfxml.Parse(g, strings.NewReader(src), rdfxml.WithProvenance(
			func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, line int) {
				idx.Triple(s, p, o, line)
				seen = append(seen, record{term.Triple{Subject: s, Predicate: p, Object: o}, line})
			}))

		lineCount := 1 + strings.Count(src, "\n")
		for _, r := range seen {
			if r.line < 1 || r.line > lineCount {
				t.Fatalf("line %d is outside the %d-line document", r.line, lineCount)
			}
		}
		if err != nil {
			// A failed parse may have reported the triples it managed before
			// giving up; those are still real and still in the graph.
			return
		}

		// On a successful parse, every triple in the graph must have a line —
		// the invariant that keeps a new production from calling g.Add
		// directly and silently losing provenance.
		g.Triples(nil, nil, nil)(func(tr term.Triple) bool {
			if _, ok := idx.Line(tr.Subject, tr.Predicate, tr.Object); !ok {
				t.Fatalf("no line for %s %s %s",
					tr.Subject.N3(), tr.Predicate.N3(), tr.Object.N3())
			}
			return true
		})
	})
}
