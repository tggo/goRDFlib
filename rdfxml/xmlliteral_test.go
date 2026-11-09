package rdfxml_test

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/rdfxml"
)

// xmlLiteral parses doc and returns the lexical form of the one XMLLiteral.
func xmlLiteral(t *testing.T, doc string) string {
	t.Helper()
	g := graph.NewGraph()
	if err := rdfxml.Parse(t.Context(), g, strings.NewReader(doc), rdfxml.WithBase("http://example.org/")); err != nil {
		t.Fatal(err)
	}
	var got []string
	for tr, err := range g.Triples(t.Context(), nil, nil, nil) {
		if err != nil {
			t.Fatal(err)
		}
		if l, ok := tr.Object.(rdflibgo.Literal); ok && l.Datatype().Value() == "http://www.w3.org/1999/02/22-rdf-syntax-ns#XMLLiteral" {
			got = append(got, l.Lexical())
		}
	}
	if len(got) != 1 {
		t.Fatalf("want one XMLLiteral, got %q", got)
	}
	return got[0]
}

// TestXMLLiteralExclusiveC14N: the lexical form is Exclusive XML
// Canonicalization without comments (RDF 1.1 Concepts §3.3).
func TestXMLLiteralExclusiveC14N(t *testing.T) {
	const head = `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:eg="http://example.org/" xmlns="http://dflt/">
<rdf:Description rdf:about="http://example.org/a"><eg:p rdf:parseType="Literal">`
	const tail = `</eg:p></rdf:Description></rdf:RDF>`
	cases := []struct{ name, in, want string }{
		{"empty element", `<br />`, `<br xmlns="http://dflt/"></br>`},
		{"unused prefixes dropped", `<eg:a>x</eg:a>`, `<eg:a xmlns:eg="http://example.org/">x</eg:a>`},
		{"declared once", `<eg:a><eg:b/></eg:a>`, `<eg:a xmlns:eg="http://example.org/"><eg:b></eg:b></eg:a>`},
		{"attributes sorted, namespaces first", `<x z="1" a="2" eg:k="3" />`,
			`<x xmlns="http://dflt/" xmlns:eg="http://example.org/" a="2" z="1" eg:k="3"></x>`},
		{"escaping", "<x a=\"&lt;&amp;&quot;&#9;&#10;\">&lt;&gt;&amp;\"</x>",
			"<x xmlns=\"http://dflt/\" a=\"&lt;&amp;&quot;&#x9;&#xA;\">&lt;&gt;&amp;\"</x>"},
		{"comment dropped", `<x><!-- c -->t</x>`, `<x xmlns="http://dflt/">t</x>`},
		{"default undeclared", `<x><y xmlns="">t</y></x>`, `<x xmlns="http://dflt/"><y xmlns="">t</y></x>`},
		{"redeclared default", `<x><y xmlns="http://other/">t</y></x>`,
			`<x xmlns="http://dflt/"><y xmlns="http://other/">t</y></x>`},
		{"text only", `a &lt; b`, `a &lt; b`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := xmlLiteral(t, head+c.in+tail); got != c.want {
				t.Errorf("got  %q\nwant %q", got, c.want)
			}
		})
	}
}

// TestXMLLiteralNamespaceScope: a prefix bound on the property element itself
// or an ancestor reaches the literal, and a sibling's binding does not leak.
func TestXMLLiteralNamespaceScope(t *testing.T) {
	doc := `<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns:eg="http://example.org/">
<rdf:Description rdf:about="http://example.org/a" xmlns:q="http://q/"><eg:p xmlns:z="http://z/" rdf:parseType="Literal"><z:a q:b="1"/></eg:p></rdf:Description>
<rdf:Description rdf:about="http://example.org/b" xmlns:q="http://other/"><eg:p>t</eg:p></rdf:Description>
</rdf:RDF>`
	want := `<z:a xmlns:q="http://q/" xmlns:z="http://z/" q:b="1"></z:a>`
	if got := xmlLiteral(t, doc); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
