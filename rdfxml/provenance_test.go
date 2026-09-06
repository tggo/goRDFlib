package rdfxml_test

import (
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/provenance"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/term"
)

func ex(local string) term.URIRef { return term.NewURIRefUnsafe("http://example.org/" + local) }

// TestBlankNodeParseScope checks reference identity, document isolation, and
// source positions against the actual terms inserted into a reused graph.
func TestBlankNodeParseScope(t *testing.T) {
	const doc = `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:ex="http://example.org/">
  <rdf:Description rdf:nodeID="shared">
    <ex:self rdf:nodeID="shared"/>
    <ex:name>labelled</ex:name>
  </rdf:Description>
  <rdf:Description>
    <ex:name>anonymous</ex:name>
  </rdf:Description>
</rdf:RDF>`
	for _, preserve := range []bool{false, true} {
		t.Run(map[bool]string{false: "scoped", true: "preserved"}[preserve], func(t *testing.T) {
			g := graph.NewGraph()
			idx := provenance.NewIndex()
			calls := 0
			opts := []rdfxml.Option{rdfxml.WithProvenance(func(s term.Subject, p term.URIRef, o term.Term, line int) {
				calls++
				if !g.Contains(s, p, o) {
					t.Error("callback terms do not identify an inserted triple")
				}
				want := 4
				if p == ex("self") {
					want = 3
					if !s.Equal(o) {
						t.Error("repeated nodeID lost its identity")
					}
				} else if o.Equal(term.NewLiteral("anonymous")) {
					want = 7
				}
				if line != want {
					t.Errorf("source line = %d, want %d", line, want)
				}
				idx.Triple(s, p, o, line)
			})}
			if preserve {
				opts = append(opts, rdfxml.WithPreserveBlankNodeIDs())
			}
			for i := 0; i < 2; i++ {
				if err := rdfxml.Parse(g, strings.NewReader(doc), opts...); err != nil {
					t.Fatal(err)
				}
			}
			want := 6
			if preserve {
				want = 4 // The labelled triples merge, but anonymous nodes stay separate.
			}
			if g.Len() != want || idx.Len() != want || calls != 6 {
				t.Fatalf("graph/index/callback counts = %d/%d/%d, want %d/%d/6", g.Len(), idx.Len(), calls, want, want)
			}
			if got := g.Contains(term.NewBNode("shared"), ex("self"), term.NewBNode("shared")); got != preserve {
				t.Errorf("raw source label present = %v, preserve = %v", got, preserve)
			}
		})
	}
}

// TestBlankNodeScopeInTripleTermsAndAnnotations checks the same nodeID in RDF
// 1.2 productions and rejects provenance for unasserted, captured triples.
func TestBlankNodeScopeInTripleTermsAndAnnotations(t *testing.T) {
	const doc = `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:ex="http://example.org/" rdf:version="1.2">
  <rdf:Description rdf:nodeID="shared">
    <ex:claim rdf:parseType="Triple" rdf:annotationNodeID="shared">
      <rdf:Description rdf:nodeID="shared">
        <ex:self rdf:nodeID="shared"/>
      </rdf:Description>
    </ex:claim>
    <ex:name>labelled</ex:name>
  </rdf:Description>
</rdf:RDF>`
	for _, preserve := range []bool{false, true} {
		t.Run(map[bool]string{false: "scoped", true: "preserved"}[preserve], func(t *testing.T) {
			g := graph.NewGraph()
			calls := 0
			opts := []rdfxml.Option{rdfxml.WithProvenance(func(s term.Subject, p term.URIRef, o term.Term, line int) {
				calls++
				if !g.Contains(s, p, o) {
					t.Errorf("callback for a triple absent from the output: %s %s %s", s.N3(), p.N3(), o.N3())
				}
				want := 7
				if p == ex("name") {
					want = 8
				}
				if line != want {
					t.Errorf("source line = %d, want %d", line, want)
				}
			})}
			if preserve {
				opts = append(opts, rdfxml.WithPreserveBlankNodeIDs())
			}
			for i := 0; i < 2; i++ {
				if err := rdfxml.Parse(g, strings.NewReader(doc), opts...); err != nil {
					t.Fatal(err)
				}
			}
			want := 6
			if preserve {
				want = 3
			}
			if g.Len() != want || calls != 6 {
				t.Fatalf("graph/callback counts = %d/%d, want %d/6", g.Len(), calls, want)
			}
			g.Triples(nil, nil, nil)(func(tr term.Triple) bool {
				if tr.Predicate == ex("claim") {
					inner := term.NewTripleTerm(tr.Subject, ex("self"), tr.Subject)
					if !tr.Object.Equal(inner) {
						t.Error("triple term uses different blank-node identities")
					}
					reifies := term.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#reifies")
					annotation := term.NewTripleTerm(tr.Subject, tr.Predicate, tr.Object)
					if !g.Contains(tr.Subject, reifies, annotation) {
						t.Error("annotation reifier uses a different blank-node identity")
					}
				}
				return true
			})
		})
	}
}

func parseWithProvenance(t *testing.T, doc string) (*graph.Graph, *provenance.Index) {
	t.Helper()
	g := graph.NewGraph()
	idx := provenance.NewIndex()
	if err := rdfxml.Parse(g, strings.NewReader(doc), rdfxml.WithProvenance(idx.Triple)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return g, idx
}

// TestProvenanceElementForm covers the ordinary shape: one property element per
// line, each triple reported against its own line.
//
//	1  <?xml ...
//	2  <rdf:RDF ...
//	3       ...>
//	4    <ex:Person rdf:about="...alice">
//	5      <ex:name>Alice</ex:name>
//	6      <ex:age>34</ex:age>
//	7    </ex:Person>
//	8  </rdf:RDF>
func TestProvenanceElementForm(t *testing.T) {
	const doc = `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <ex:Person rdf:about="http://example.org/alice">
    <ex:name>Alice</ex:name>
    <ex:age>34</ex:age>
  </ex:Person>
</rdf:RDF>
`
	_, idx := parseWithProvenance(t, doc)

	if idx.Len() != 3 {
		t.Fatalf("recorded %d triples, want 3 (rdf:type, ex:name, ex:age)", idx.Len())
	}

	cases := []struct {
		what string
		p    term.URIRef
		o    term.Term
		want int
	}{
		{"ex:name", ex("name"), term.NewLiteral("Alice"), 5},
		{"ex:age", ex("age"), term.NewLiteral("34"), 6},
	}
	for _, tc := range cases {
		line, ok := idx.Line(ex("alice"), tc.p, tc.o)
		if !ok {
			t.Errorf("%s: no line recorded", tc.what)
			continue
		}
		if line != tc.want {
			t.Errorf("%s: line %d, want %d", tc.what, line, tc.want)
		}
	}

	// rdf:type comes from the node element itself, on line 4.
	if line, ok := idx.Line(ex("alice"), term.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#type"), ex("Person")); !ok || line != 4 {
		t.Errorf("rdf:type recorded line %d (%v), want 4", line, ok)
	}
}

// TestProvenanceAttributeForm covers property attributes, where the whole
// triple lives on the start tag.
func TestProvenanceAttributeForm(t *testing.T) {
	const doc = `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/bob"
                   ex:name="Bob" />
</rdf:RDF>
`
	_, idx := parseWithProvenance(t, doc)

	line, ok := idx.Line(ex("bob"), ex("name"), term.NewLiteral("Bob"))
	if !ok {
		t.Fatal("no line for the attribute-form triple")
	}
	// The element spans lines 4-5 and the parser is at its start when the
	// attribute is read.
	if line != 4 && line != 5 {
		t.Errorf("line %d, want the element's own lines 4 or 5", line)
	}
}

// TestProvenanceMultilineProperty checks a property element whose value sits on
// a line of its own. The line reported is the one the triple was completed on,
// which is where a reader finds the value — not where the subject was opened.
func TestProvenanceMultilineProperty(t *testing.T) {
	const doc = `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/carol">
    <ex:note>
      a long note
    </ex:note>
  </rdf:Description>
</rdf:RDF>
`
	_, idx := parseWithProvenance(t, doc)

	line, ok := idx.SubjectLine(ex("carol"))
	if !ok {
		t.Fatal("no subject line for ex:carol")
	}
	// The single triple is completed at </ex:note> on line 7, so that is also
	// the subject's earliest line.
	if line != 7 {
		t.Errorf("subject line %d, want 7 — the line the triple completes on", line)
	}
}

// TestProvenanceIsOptIn checks that the parse is unchanged and costs nothing
// when the option is absent.
func TestProvenanceIsOptIn(t *testing.T) {
	const doc = `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/dave" ex:name="Dave" />
</rdf:RDF>
`
	plain := graph.NewGraph()
	if err := rdfxml.Parse(plain, strings.NewReader(doc)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	traced, _ := parseWithProvenance(t, doc)

	if plain.Len() != traced.Len() {
		t.Errorf("the option changed the graph: %d triples vs %d", plain.Len(), traced.Len())
	}
}

// TestProvenanceEveryTripleIsReported is the guard against a new production
// calling g.Add directly and quietly skipping provenance. Every triple in the
// graph must have a line.
func TestProvenanceEveryTripleIsReported(t *testing.T) {
	const doc = `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <ex:Person rdf:about="http://example.org/erin" ex:nick="E">
    <ex:name>Erin</ex:name>
    <ex:knows>
      <ex:Person rdf:about="http://example.org/frank">
        <ex:name>Frank</ex:name>
      </ex:Person>
    </ex:knows>
    <ex:tags rdf:parseType="Collection">
      <rdf:Description rdf:about="http://example.org/t1"/>
      <rdf:Description rdf:about="http://example.org/t2"/>
    </ex:tags>
  </ex:Person>
</rdf:RDF>
`
	g, idx := parseWithProvenance(t, doc)

	missing := 0
	g.Triples(nil, nil, nil)(func(tr term.Triple) bool {
		if _, ok := idx.Line(tr.Subject, tr.Predicate, tr.Object); !ok {
			t.Errorf("no line for %s %s %s", tr.Subject.N3(), tr.Predicate.N3(), tr.Object.N3())
			missing++
		}
		return true
	})
	if missing > 0 {
		t.Errorf("%d of %d triples were added without reporting a line", missing, g.Len())
	}
	if idx.Len() != g.Len() {
		t.Errorf("index holds %d triples, graph holds %d", idx.Len(), g.Len())
	}
}
