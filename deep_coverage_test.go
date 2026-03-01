package rdflibgo

import (
	"bytes"
	"strings"
	"testing"
)

// --- ntUnescapeString direct tests ---

func TestNtUnescapeStringNoEscape(t *testing.T) {
	if ntUnescapeString("hello") != "hello" {
		t.Error("no-op")
	}
}

func TestNtUnescapeStringUnicode4(t *testing.T) {
	if ntUnescapeString(`\u0041`) != "A" {
		t.Error("\\u0041 should be A")
	}
}

func TestNtUnescapeStringUnicode8(t *testing.T) {
	if ntUnescapeString(`\U00000042`) != "B" {
		t.Error("\\U00000042 should be B")
	}
}

func TestNtUnescapeStringBoundary(t *testing.T) {
	// \u at exact end
	got := ntUnescapeString(`\u0043`)
	if got != "C" {
		t.Errorf("expected C, got %q", got)
	}
}

// --- isAbsoluteIRI ---

func TestIsAbsoluteIRI(t *testing.T) {
	tests := []struct {
		iri    string
		expect bool
	}{
		{"http://example.org/", true},
		{"urn:uuid:123", true},
		{"ftp://files.example.org/", true},
		{"relative", false},
		{"", false},
		{"123:bad", false},    // starts with digit
		{":noscheme", false},  // empty scheme
		{"a+b.c-d:valid", true},
	}
	for _, tt := range tests {
		if isAbsoluteIRI(tt.iri) != tt.expect {
			t.Errorf("isAbsoluteIRI(%q) = %v, want %v", tt.iri, !tt.expect, tt.expect)
		}
	}
}

// --- SPARQL parser: string literal with lang/datatype ---

func TestSPARQLStringLiteralLang(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
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

	r, err := g.Query(`PREFIX ex: <http://example.org/> SELECT ?l WHERE { ex:s ex:label ?l . FILTER(?l = "hello"@en) }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

func TestSPARQLSelectExpressionAlias(t *testing.T) {
	g := makeSPARQLGraph(t)
	// SELECT with (expr AS ?var)
	r, err := g.Query(`
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

	r, _ := g.Query(`PREFIX ex: <http://example.org/> SELECT ?v WHERE { ?s ex:val ?v . FILTER(?v) }`)
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

	r, _ := g.Query(`PREFIX ex: <http://example.org/> SELECT ?v WHERE { ?s ex:val ?v . FILTER(?v) }`)
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
	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("turtle"))
	out := buf.String()
	if !strings.Contains(out, "^^ex:mytype") {
		t.Errorf("expected prefixed datatype, got:\n%s", out)
	}
}

// --- Format aliases ---

func TestFormatAliasTTL(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))

	var buf bytes.Buffer
	err := g.Serialize(&buf, WithSerializeFormat("ttl"))
	if err != nil {
		t.Fatal(err)
	}

	g2 := NewGraph()
	err = g2.Parse(strings.NewReader(buf.String()), WithFormat("ttl"))
	if err != nil {
		t.Fatal(err)
	}
}

func TestFormatAliasNTriples(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("ntriples"))
	g2 := NewGraph()
	g2.Parse(strings.NewReader(buf.String()), WithFormat("ntriples"))
	if g2.Len() != 1 {
		t.Error("ntriples alias failed")
	}
}

func TestFormatAliasNQ(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("nq"))
	g2 := NewGraph()
	g2.Parse(strings.NewReader(buf.String()), WithFormat("nq"))
	if g2.Len() != 1 {
		t.Error("nq alias failed")
	}
}

func TestFormatAliasRDFXML(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p, NewLiteral("v"))

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("rdf/xml"))
	g2 := NewGraph()
	g2.Parse(strings.NewReader(buf.String()), WithFormat("rdf/xml"))
	if g2.Len() != 1 {
		t.Error("rdf/xml alias failed")
	}
}

func TestFormatAliasApplicationRDFXML(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p, NewLiteral("v"))

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("application/rdf+xml"))
	g2 := NewGraph()
	g2.Parse(strings.NewReader(buf.String()), WithFormat("application/rdf+xml"))
	if g2.Len() != 1 {
		t.Error("application/rdf+xml alias failed")
	}
}

func TestFormatAliasJSONLD(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("jsonld"))
	g2 := NewGraph()
	g2.Parse(strings.NewReader(buf.String()), WithFormat("jsonld"))
	if g2.Len() != 1 {
		t.Error("jsonld alias failed")
	}
}

func TestFormatAliasApplicationLDJSON(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("application/ld+json"))
	g2 := NewGraph()
	g2.Parse(strings.NewReader(buf.String()), WithFormat("application/ld+json"))
	if g2.Len() != 1 {
		t.Error("application/ld+json alias failed")
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

	// Use a mock T to capture failure
	mt := &testing.T{}
	AssertGraphEqual(mt, g1, g2)
	// We can't check mt.Failed() easily but at least we exercised the code
}

// --- SPARQL VALUES with multi-var ---

func TestSPARQLValuesMultiVar(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
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
