package rdfxml

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// rdflib #3524: a triple term object was silently dropped.
func TestSerializeTripleTermObject(t *testing.T) {
	g := rdflibgo.NewGraph()
	inner := rdflibgo.NewTripleTerm(iri("http://e/a"), iri("http://e/b"), iri("http://e/c"))
	g.Add(iri("http://e/s"), iri("http://e/p"), inner)
	out := assertRoundTrip(t, g)
	if !strings.Contains(out, `rdf:version="1.2"`) || !strings.Contains(out, `rdf:parseType="Triple"`) {
		t.Errorf("expected RDF 1.2 triple term syntax:\n%s", out)
	}
}

func TestSerializeNestedTripleTermWithBlankNodeAndLiteral(t *testing.T) {
	g := rdflibgo.NewGraph()
	b := rdflibgo.NewBNode()
	lit := rdflibgo.NewTripleTerm(b, rdflibgo.RDF.Type, iri("http://e/C"))
	nested := rdflibgo.NewTripleTerm(iri("http://e/x"), iri("http://e/says"), lit)
	g.Add(iri("http://e/s"), iri("http://e/p"), nested)
	g.Add(iri("http://e/s"), iri("http://e/q"), rdflibgo.NewTripleTerm(b, iri("http://e/name"), rdflibgo.NewLiteral("n", rdflibgo.WithLang("en"))))
	g.Add(b, iri("http://e/r"), rdflibgo.NewLiteral("asserted"))
	assertRoundTrip(t, g)
}

// RDF 1.1 output keeps its old root element.
func TestSerializeNoVersionWithoutRDF12Terms(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Add(iri("http://e/s"), iri("http://e/p"), rdflibgo.NewLiteral("v", rdflibgo.WithLang("en")))
	if out := serializeWellFormed(t, g); strings.Contains(out, "rdf:version") || strings.Contains(out, "its:") {
		t.Errorf("RDF 1.1 graph written with RDF 1.2 markers:\n%s", out)
	}
}
