package integration_test

import (
	"strings"
	"testing"

	. "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/turtle"
)

// --- N-Triples parser coverage ---

func TestNTParserUnicodeEscape(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "\u0041BC" .
`
	g := NewGraph()
	nt.Parse(t.Context(), g, strings.NewReader(input))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	val, ok, _ := g.Value(t.Context(), s, &p, nil)
	if !ok || val.String() != "ABC" {
		t.Errorf("expected ABC, got %v", val)
	}
}

func TestNTParserBNodeObject(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> _:b1 .
`
	g := NewGraph()
	nt.Parse(t.Context(), g, strings.NewReader(input))
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

func TestNTParserErrorMissingDot(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello"
`
	g := NewGraph()
	err := nt.Parse(t.Context(), g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for missing dot")
	}
}

func TestNTParserErrorBadSubject(t *testing.T) {
	input := `"literal" <http://example.org/p> "hello" .
`
	g := NewGraph()
	err := nt.Parse(t.Context(), g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for literal as subject")
	}
}

func TestNTParserErrorBadObject(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> .
`
	g := NewGraph()
	err := nt.Parse(t.Context(), g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error")
	}
}

func TestNTParserErrorBadPredicate(t *testing.T) {
	input := `<http://example.org/s> "notiri" "hello" .
`
	g := NewGraph()
	err := nt.Parse(t.Context(), g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for literal as predicate")
	}
}

// --- N-Triples serializer coverage ---

func TestNTSerializerBNode(t *testing.T) {
	g := NewGraph()
	b := NewBNode("b1")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(t.Context(), b, p, NewLiteral("v"))
	var buf strings.Builder
	nt.Serialize(t.Context(), g, &buf)
	if !strings.Contains(buf.String(), "_:b1") {
		t.Errorf("expected _:b1, got:\n%s", buf.String())
	}
}

func TestNTSerializerUnicodeEscape(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, NewLiteral("a\x01b"))
	var buf strings.Builder
	nt.Serialize(t.Context(), g, &buf)
	if !strings.Contains(buf.String(), `\u0001`) {
		t.Errorf("expected unicode escape, got:\n%s", buf.String())
	}
}

// --- N-Quads coverage ---

func TestNQParserBNodeGraph(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello" _:g1 .
`
	g := NewGraph()
	err := nq.Parse(t.Context(), g, strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

func TestNQSerializerWithURIRefIdentifier(t *testing.T) {
	id, _ := NewURIRef("http://example.org/g")
	g := NewGraph(WithIdentifier(id))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, NewLiteral("v"))
	var buf strings.Builder
	nq.Serialize(t.Context(), g, &buf)
	if !strings.Contains(buf.String(), "<http://example.org/g>") {
		t.Errorf("expected graph context, got:\n%s", buf.String())
	}
}

// --- RDF/XML parser coverage ---

func TestRDFXMLParserMultipleSubjects(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s1">
    <ex:p>a</ex:p>
  </rdf:Description>
  <rdf:Description rdf:about="http://example.org/s2">
    <ex:p>b</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := NewGraph()
	rdfxml.Parse(t.Context(), g, strings.NewReader(input))
	if mustV(g.Len(t.Context())) != 2 {
		t.Errorf("expected 2, got %d", mustV(g.Len(t.Context())))
	}
}

func TestRDFXMLParserNestedDescription(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:knows>
      <rdf:Description rdf:about="http://example.org/o">
        <ex:name>Bob</ex:name>
      </rdf:Description>
    </ex:knows>
  </rdf:Description>
</rdf:RDF>`
	g := NewGraph()
	rdfxml.Parse(t.Context(), g, strings.NewReader(input))
	if mustV(g.Len(t.Context())) != 2 {
		t.Errorf("expected 2, got %d", mustV(g.Len(t.Context())))
	}
}

func TestRDFXMLParserPropertyAttributes(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s" ex:name="Alice"/>
</rdf:RDF>`
	g := NewGraph()
	rdfxml.Parse(t.Context(), g, strings.NewReader(input))
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

func TestRDFXMLParserRDFTypeAttribute(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s" rdf:type="http://example.org/Person"/>
</rdf:RDF>`
	g := NewGraph()
	rdfxml.Parse(t.Context(), g, strings.NewReader(input))
	s, _ := NewURIRef("http://example.org/s")
	person, _ := NewURIRef("http://example.org/Person")
	if !mustV(g.Contains(t.Context(), s, RDF.Type, person)) {
		t.Error("expected rdf:type triple")
	}
}

func TestRDFXMLParserCollection(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:items rdf:parseType="Collection">
      <rdf:Description rdf:about="http://example.org/a"/>
      <rdf:Description rdf:about="http://example.org/b"/>
    </ex:items>
  </rdf:Description>
</rdf:RDF>`
	g := NewGraph()
	err := rdfxml.Parse(t.Context(), g, strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
	if mustV(g.Len(t.Context())) < 5 {
		t.Errorf("expected >=5, got %d", mustV(g.Len(t.Context())))
	}
}

func TestRDFXMLParserXMLBase(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/">
  <rdf:Description rdf:about="s">
    <ex:p>hello</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := NewGraph()
	rdfxml.Parse(t.Context(), g, strings.NewReader(input))
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

func TestRDFXMLParserEmptyInput(t *testing.T) {
	g := NewGraph()
	err := rdfxml.Parse(t.Context(), g, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
}

func TestRDFXMLParserRDFID(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/">
  <rdf:Description rdf:ID="s">
    <ex:p>hello</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := NewGraph()
	rdfxml.Parse(t.Context(), g, strings.NewReader(input))
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

// --- Turtle parser extra coverage ---

func TestTurtleParserEmptyInput(t *testing.T) {
	g := NewGraph()
	err := turtle.Parse(t.Context(), g, strings.NewReader(""))
	if err != nil {
		t.Fatal(err)
	}
}

func TestTurtleParserSingleQuotedString(t *testing.T) {
	g := NewGraph()
	turtle.Parse(t.Context(), g, strings.NewReader(`
		@prefix ex: <http://example.org/> .
		ex:s ex:p 'hello' .
	`))
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

func TestTurtleParserDatatypeIRI(t *testing.T) {
	g := NewGraph()
	turtle.Parse(t.Context(), g, strings.NewReader(`
		@prefix ex: <http://example.org/> .
		ex:s ex:p "42"^^<http://www.w3.org/2001/XMLSchema#integer> .
	`))
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

func TestTurtleParserSubjectAsBlankNodePropertyList(t *testing.T) {
	g := NewGraph()
	turtle.Parse(t.Context(), g, strings.NewReader(`
		@prefix ex: <http://example.org/> .
		[ ex:name "Alice" ] ex:age 30 .
	`))
	if mustV(g.Len(t.Context())) != 2 {
		t.Errorf("expected 2, got %d", mustV(g.Len(t.Context())))
	}
}

func TestTurtleParserSubjectAsCollection(t *testing.T) {
	g := NewGraph()
	turtle.Parse(t.Context(), g, strings.NewReader(`
		@prefix ex: <http://example.org/> .
		( "a" "b" ) ex:type ex:List .
	`))
	if mustV(g.Len(t.Context())) < 3 {
		t.Errorf("expected >=3, got %d", mustV(g.Len(t.Context())))
	}
}

func TestTurtleParserNegativeNumeric(t *testing.T) {
	g := NewGraph()
	turtle.Parse(t.Context(), g, strings.NewReader(`
		@prefix ex: <http://example.org/> .
		ex:s ex:val -5 .
	`))
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

func TestTurtleParserDecimalNoDot(t *testing.T) {
	g := NewGraph()
	err := turtle.Parse(t.Context(), g, strings.NewReader(`@prefix ex: <http://example.org/> . ex:s ex:p .5 .`))
	_ = err
	_ = g
}

func TestTurtleParserEmptyBlankNode(t *testing.T) {
	g := NewGraph()
	turtle.Parse(t.Context(), g, strings.NewReader(`
		@prefix ex: <http://example.org/> .
		ex:s ex:p [] .
	`))
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

func TestTurtleParserBaseResolution(t *testing.T) {
	g := NewGraph()
	turtle.Parse(t.Context(), g, strings.NewReader(`
		@base <http://example.org/dir/> .
		<file> <prop> "val" .
	`))
	s, _ := NewURIRef("http://example.org/dir/file")
	p, _ := NewURIRef("http://example.org/dir/prop")
	if !mustV(g.Contains(t.Context(), s, p, NewLiteral("val"))) {
		t.Error("expected resolved triple")
	}
}

// --- Serializer RDF/XML coverage ---

func TestRDFXMLSerializerLang(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Bind(t.Context(), "ex", NewURIRefUnsafe("http://example.org/"))
	g.Add(t.Context(), s, p, NewLiteral("hello", WithLang("en")))
	var buf strings.Builder
	rdfxml.Serialize(t.Context(), g, &buf)
	if !strings.Contains(buf.String(), `xml:lang="en"`) {
		t.Errorf("expected xml:lang, got:\n%s", buf.String())
	}
}

func TestRDFXMLSerializerDatatype(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Bind(t.Context(), "ex", NewURIRefUnsafe("http://example.org/"))
	g.Add(t.Context(), s, p, NewLiteral("42", WithDatatype(XSDInteger)))
	var buf strings.Builder
	rdfxml.Serialize(t.Context(), g, &buf)
	if !strings.Contains(buf.String(), "rdf:datatype") {
		t.Errorf("expected rdf:datatype, got:\n%s", buf.String())
	}
}

func TestRDFXMLSerializerBNode(t *testing.T) {
	g := NewGraph()
	b := NewBNode("b1")
	p, _ := NewURIRef("http://example.org/p")
	g.Bind(t.Context(), "ex", NewURIRefUnsafe("http://example.org/"))
	g.Add(t.Context(), b, p, NewLiteral("v"))
	var buf strings.Builder
	rdfxml.Serialize(t.Context(), g, &buf)
	if !strings.Contains(buf.String(), "rdf:nodeID") {
		t.Errorf("expected rdf:nodeID, got:\n%s", buf.String())
	}
}

func TestRDFXMLSerializerResourceObject(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	g.Bind(t.Context(), "ex", NewURIRefUnsafe("http://example.org/"))
	g.Add(t.Context(), s, p, o)
	var buf strings.Builder
	rdfxml.Serialize(t.Context(), g, &buf)
	if !strings.Contains(buf.String(), "rdf:resource") {
		t.Errorf("expected rdf:resource, got:\n%s", buf.String())
	}
}

func TestRDFXMLSerializerBNodeObject(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	b := NewBNode("obj1")
	g.Bind(t.Context(), "ex", NewURIRefUnsafe("http://example.org/"))
	g.Add(t.Context(), s, p, b)
	var buf strings.Builder
	rdfxml.Serialize(t.Context(), g, &buf)
	if !strings.Contains(buf.String(), "rdf:nodeID") {
		t.Errorf("expected rdf:nodeID, got:\n%s", buf.String())
	}
}

func TestRDFXMLSerializerBase(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, NewLiteral("v"))
	var buf strings.Builder
	rdfxml.Serialize(t.Context(), g, &buf, rdfxml.WithBase("http://example.org/"))
	if !strings.Contains(buf.String(), `xml:base="http://example.org/"`) {
		t.Errorf("expected xml:base, got:\n%s", buf.String())
	}
}

// --- JSON-LD coverage ---

func TestJSONLDParserInvalidJSON(t *testing.T) {
	g := NewGraph()
	err := jsonld.Parse(t.Context(), g, strings.NewReader("not json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// --- Turtle serializer extra coverage ---

func TestTurtleSerializerEmpty(t *testing.T) {
	g := NewGraph()
	var buf strings.Builder
	err := turtle.Serialize(t.Context(), g, &buf)
	if err != nil {
		t.Fatal(err)
	}
}

func TestTurtleSerializerBNodeSubjectNotReferenced(t *testing.T) {
	g := NewGraph()
	b := NewBNode()
	p, _ := NewURIRef("http://example.org/p")
	g.Bind(t.Context(), "ex", NewURIRefUnsafe("http://example.org/"))
	g.Add(t.Context(), b, p, NewLiteral("v"))
	var buf strings.Builder
	turtle.Serialize(t.Context(), g, &buf)
	if !strings.Contains(buf.String(), "[]") {
		t.Errorf("expected [] for unreferenced BNode, got:\n%s", buf.String())
	}
}

// --- Plugin coverage ---

func TestFormatFromContentEmpty(t *testing.T) {
	_, ok := FormatFromContent(nil)
	if ok {
		t.Error("expected false for nil")
	}
	_, ok = FormatFromContent([]byte{})
	if ok {
		t.Error("expected false for empty")
	}
}

func TestFormatFromContentNQuads(t *testing.T) {
	f, ok := FormatFromContent([]byte(`<http://s> <http://p> "o" <http://g> .`))
	if !ok || f != "nquads" {
		t.Errorf("expected nquads, got %q", f)
	}
}
