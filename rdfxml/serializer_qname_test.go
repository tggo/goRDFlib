package rdfxml

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// rdflib #2409: a predicate in an unbound namespace was written as
// <http://b.example/p>, which is not XML, and the triple was lost on reparse.
func TestSerializeUnboundPredicateNamespace(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Add(testIRI("http://a.example/s"), testIRI("http://b.example/p"), rdflibgo.NewLiteral("v"))
	out := assertRoundTrip(t, g)
	if !strings.Contains(out, `xmlns:ns1="http://b.example/"`) {
		t.Errorf("expected a generated ns1 prefix:\n%s", out)
	}
}

// An empty prefix binding (@prefix : <...>) produced xmlns:="..." and <:p>.
func TestSerializeEmptyPrefixBinding(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("", testIRI("http://e/"))
	g.Add(testIRI("http://e/s"), testIRI("http://e/p"), rdflibgo.NewLiteral("v"))
	g.Add(testIRI("http://e/s"), rdflibgo.RDF.Type, testIRI("http://e/C"))
	out := assertRoundTrip(t, g)
	if strings.Contains(out, "xmlns:=") || strings.Contains(out, "<:") {
		t.Errorf("empty prefix leaked into output:\n%s", out)
	}
}

// Local names that are not NCNames (ex:1Class) were used as element names.
func TestSerializeNonNCNameLocalName(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", testIRI("http://e/"))
	g.Add(testIRI("http://e/s"), testIRI("http://e/1prop"), rdflibgo.NewLiteral("v"))
	g.Add(testIRI("http://e/s"), rdflibgo.RDF.Type, testIRI("http://e/1Class"))
	out := assertRoundTrip(t, g)
	if strings.Contains(out, "<ex:1") {
		t.Errorf("non-NCName local name used as element name:\n%s", out)
	}
}

func TestSerializePredicateWithoutQNameFails(t *testing.T) {
	for _, p := range []string{"http://e/123", "http://e/dir/", "urn:x:1-2"} {
		g := rdflibgo.NewGraph()
		g.Bind("ex", testIRI("http://e/"))
		g.Add(testIRI("http://e/s"), testIRI(p), rdflibgo.NewLiteral("v"))
		var buf bytes.Buffer
		err := Serialize(g, &buf)
		if !errors.Is(err, ErrNoQName) {
			t.Errorf("%s: want ErrNoQName, got %v\n%s", p, err, buf.String())
			continue
		}
		if !strings.Contains(err.Error(), p) || !strings.Contains(err.Error(), "Turtle") {
			t.Errorf("error should name the IRI and a remedy: %v", err)
		}
		if buf.Len() != 0 {
			t.Errorf("%s: output written despite error:\n%s", p, buf.String())
		}
	}
}

func TestSerializeReservedPredicateFails(t *testing.T) {
	for _, p := range []string{rdfNS + "about", rdfNS + "li", rdfNS + "Description"} {
		g := rdflibgo.NewGraph()
		g.Add(testIRI("http://e/s"), testIRI(p), rdflibgo.NewLiteral("v"))
		if err := Serialize(g, &bytes.Buffer{}); !errors.Is(err, ErrReservedPropertyName) {
			t.Errorf("%s: want ErrReservedPropertyName, got %v", p, err)
		}
	}
}

// rdflib #2408: rdf bound to another namespace wrote xmlns:rdf twice.
func TestSerializeRDFPrefixRebound(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("rdf", testIRI("http://other/"))
	g.Bind("ex", testIRI("http://e/"))
	g.Add(testIRI("http://e/s"), testIRI("http://other/p"), rdflibgo.NewLiteral("v"))
	g.Add(testIRI("http://e/s"), testIRI("http://e/p"), rdflibgo.NewLiteral("w"))
	out := assertRoundTrip(t, g)
	if n := strings.Count(out, "xmlns:rdf="); n != 1 {
		t.Errorf("xmlns:rdf declared %d times:\n%s", n, out)
	}
}

// The RDF namespace bound to another prefix must not leave rdf: undeclared.
func TestSerializeRDFNamespaceUnderOtherPrefix(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("r", testIRI(rdfNS))
	g.Add(testIRI("http://e/s"), testIRI("http://e/p"), rdflibgo.NewLiteral("v"))
	assertRoundTrip(t, g)
}

func TestSerializeBlankNodeLabelNotNCName(t *testing.T) {
	g := rdflibgo.NewGraph()
	b := rdflibgo.NewBNode("1a")
	g.Add(b, testIRI("http://e/p"), rdflibgo.NewBNode("genid1"))
	g.Add(testIRI("http://e/s"), testIRI("http://e/q"), b)
	assertRoundTrip(t, g)
}

func TestSerializeNonASCIILocalName(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("ex", testIRI("http://e/"))
	g.Add(testIRI("http://e/s"), testIRI("http://e/имя"), rdflibgo.NewLiteral("v"))
	out := assertRoundTrip(t, g)
	if !strings.Contains(out, "<ex:имя>") {
		t.Errorf("expected ex:имя:\n%s", out)
	}
}

func TestSplitIRI(t *testing.T) {
	cases := []struct{ in, ns, local string }{
		{"http://e/p", "http://e/", "p"},
		{"http://e/a#b", "http://e/a#", "b"},
		{"http://e/1Class", "http://e/1", "Class"},
		{"http://e/a-b.c", "http://e/", "a-b.c"},
		{"http://e/123", "", ""},
		{"http://e/", "", ""},
		{"plain", "", ""},
	}
	for _, c := range cases {
		ns, local := splitIRI(c.in)
		if ns != c.ns || local != c.local {
			t.Errorf("splitIRI(%q) = %q, %q; want %q, %q", c.in, ns, local, c.ns, c.local)
		}
	}
}
