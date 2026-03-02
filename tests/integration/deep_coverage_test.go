package integration_test

import (
	"strings"
	"testing"

	. "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/testutil"
	"github.com/tggo/goRDFlib/turtle"
)

// --- SPARQL parser: string literal with lang/datatype ---

func TestSPARQLStringLiteralLang(t *testing.T) {
	g := makeSPARQLGraphExt(t)
	r, err := sparql.Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE { ?s ex:name ?name . FILTER(?name = "Alice") }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

func TestSPARQLStringLiteralWithLang(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/label")
	g.Add(s, p, NewLiteral("hello", WithLang("en")))

	r, err := sparql.Query(g, `PREFIX ex: <http://example.org/> SELECT ?l WHERE { ex:s ex:label ?l . FILTER(?l = "hello"@en) }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

func TestSPARQLSelectExpressionAlias(t *testing.T) {
	g := makeSPARQLGraphExt(t)
	r, err := sparql.Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name (UCASE(?name) AS ?upper) WHERE { ?s ex:name ?name }
		LIMIT 1
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

// --- SPARQL effective boolean value for more types ---

func TestSPARQLEBVFloat(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/val")
	g.Add(s, p, NewLiteral(0.0))
	g.Add(s, p, NewLiteral(1.5))

	r, _ := sparql.Query(g, `PREFIX ex: <http://example.org/> SELECT ?v WHERE { ?s ex:val ?v . FILTER(?v) }`)
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 truthy float, got %d", len(r.Bindings))
	}
}

func TestSPARQLEBVBool(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/val")
	g.Add(s, p, NewLiteral(true))
	g.Add(s, p, NewLiteral(false))

	r, _ := sparql.Query(g, `PREFIX ex: <http://example.org/> SELECT ?v WHERE { ?s ex:val ?v . FILTER(?v) }`)
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1 truthy bool, got %d", len(r.Bindings))
	}
}

// --- Turtle serializer: typed literal with prefixed datatype ---

func TestTurtleSerializerPrefixedDatatype(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	dt, _ := NewURIRef("http://example.org/mytype")
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p, NewLiteral("val", WithDatatype(dt)))
	var buf strings.Builder
	turtle.Serialize(g, &buf)
	out := buf.String()
	if !strings.Contains(out, "^^ex:mytype") {
		t.Errorf("expected prefixed datatype, got:\n%s", out)
	}
}

// --- Format round-trip tests ---

func TestFormatRoundTripTurtle(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))

	var buf strings.Builder
	err := turtle.Serialize(g, &buf)
	if err != nil {
		t.Fatal(err)
	}

	g2 := NewGraph()
	err = turtle.Parse(g2, strings.NewReader(buf.String()))
	if err != nil {
		t.Fatal(err)
	}
}

func TestFormatRoundTripNTriples(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))

	var buf strings.Builder
	nt.Serialize(g, &buf)
	g2 := NewGraph()
	nt.Parse(g2, strings.NewReader(buf.String()))
	if g2.Len() != 1 {
		t.Error("ntriples round-trip failed")
	}
}

func TestFormatRoundTripNQ(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))

	var buf strings.Builder
	nq.Serialize(g, &buf)
	g2 := NewGraph()
	nq.Parse(g2, strings.NewReader(buf.String()))
	if g2.Len() != 1 {
		t.Error("nq round-trip failed")
	}
}

func TestFormatRoundTripRDFXML(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p, NewLiteral("v"))

	var buf strings.Builder
	rdfxml.Serialize(g, &buf)
	g2 := NewGraph()
	rdfxml.Parse(g2, strings.NewReader(buf.String()))
	if g2.Len() != 1 {
		t.Error("rdf/xml round-trip failed")
	}
}

func TestFormatRoundTripJSONLD(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))

	var buf strings.Builder
	jsonld.Serialize(g, &buf)
	g2 := NewGraph()
	jsonld.Parse(g2, strings.NewReader(buf.String()))
	if g2.Len() != 1 {
		t.Error("jsonld round-trip failed")
	}
}

// --- AssertGraphEqual with mismatch ---

func TestAssertGraphEqualMismatch(t *testing.T) {
	g1 := NewGraph()
	g2 := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g1.Add(s, p, NewLiteral("a"))
	g2.Add(s, p, NewLiteral("b"))

	mt := &testing.T{}
	testutil.AssertGraphEqual(mt, g1, g2)
}

// --- SPARQL VALUES with multi-var ---

func TestSPARQLValuesMultiVar(t *testing.T) {
	g := makeSPARQLGraphExt(t)
	r, err := sparql.Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?s ?name WHERE {
			?s ex:name ?name .
			VALUES (?name) { ("Alice") ("Bob") }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}
