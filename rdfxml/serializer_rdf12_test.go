package rdfxml

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// rdflib #3524: a triple term object was silently dropped.
func TestSerializeTripleTermObject(t *testing.T) {
	g := rdflibgo.NewGraph()
	inner := rdflibgo.NewTripleTerm(testIRI("http://e/a"), testIRI("http://e/b"), testIRI("http://e/c"))
	g.Add(testIRI("http://e/s"), testIRI("http://e/p"), inner)
	out := assertRoundTrip(t, g)
	if !strings.Contains(out, `rdf:version="1.2"`) || !strings.Contains(out, `rdf:parseType="Triple"`) {
		t.Errorf("expected RDF 1.2 triple term syntax:\n%s", out)
	}
}

func TestSerializeNestedTripleTermWithBlankNodeAndLiteral(t *testing.T) {
	g := rdflibgo.NewGraph()
	b := rdflibgo.NewBNode()
	lit := rdflibgo.NewTripleTerm(b, rdflibgo.RDF.Type, testIRI("http://e/C"))
	nested := rdflibgo.NewTripleTerm(testIRI("http://e/x"), testIRI("http://e/says"), lit)
	g.Add(testIRI("http://e/s"), testIRI("http://e/p"), nested)
	g.Add(testIRI("http://e/s"), testIRI("http://e/q"), rdflibgo.NewTripleTerm(b, testIRI("http://e/name"), rdflibgo.NewLiteral("n", rdflibgo.WithLang("en"))))
	g.Add(b, testIRI("http://e/r"), rdflibgo.NewLiteral("asserted"))
	assertRoundTrip(t, g)
}

// RDF 1.1 output keeps its old root element.
func TestSerializeNoVersionWithoutRDF12Terms(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Add(testIRI("http://e/s"), testIRI("http://e/p"), rdflibgo.NewLiteral("v", rdflibgo.WithLang("en")))
	if out := serializeWellFormed(t, g); strings.Contains(out, "rdf:version") || strings.Contains(out, "its:") {
		t.Errorf("RDF 1.1 graph written with RDF 1.2 markers:\n%s", out)
	}
}

// "abc"@ar--rtl was written with xml:lang only and came back as @ar.
func TestSerializeBaseDirection(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Add(testIRI("http://e/s"), testIRI("http://e/p"), rdflibgo.NewLiteral("abc", rdflibgo.WithLang("ar"), rdflibgo.WithDir("rtl")))
	g.Add(testIRI("http://e/s"), testIRI("http://e/p"), rdflibgo.NewLiteral("abc", rdflibgo.WithLang("en"), rdflibgo.WithDir("ltr")))
	g.Add(testIRI("http://e/s"), testIRI("http://e/p"), rdflibgo.NewLiteral("abc", rdflibgo.WithLang("en")))
	g.Add(testIRI("http://e/s"), testIRI("http://e/q"), rdflibgo.NewTripleTerm(testIRI("http://e/a"), testIRI("http://e/b"),
		rdflibgo.NewLiteral("x", rdflibgo.WithLang("he"), rdflibgo.WithDir("rtl"))))
	out := assertRoundTrip(t, g)
	for _, want := range []string{`xmlns:its="http://www.w3.org/2005/11/its"`, `its:version="2.0"`, `rdf:version="1.2"`, `its:dir="rtl"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s:\n%s", want, out)
		}
	}
}

// A user prefix "its" bound elsewhere must not shadow the ITS namespace.
func TestSerializeBaseDirectionItsPrefixTaken(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Bind("its", testIRI("http://e/"))
	g.Add(testIRI("http://e/s"), testIRI("http://e/p"), rdflibgo.NewLiteral("abc", rdflibgo.WithLang("ar"), rdflibgo.WithDir("rtl")))
	assertRoundTrip(t, g)
}
