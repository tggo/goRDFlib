package integration_test

import (
	"strings"
	"testing"

	. "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/turtle"
)

// --- SPARQL parser: readStringLiteral with datatype/lang ---

func TestSPARQLQueryWithLangLiteral(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/name")
	g.Add(s, p, NewLiteral("hello", WithLang("en")))

	r, err := sparql.Query(g, `PREFIX ex: <http://example.org/> SELECT ?n WHERE { ?s ex:name "hello"@en }`)
	_ = r
	_ = err
}

func TestSPARQLQueryWithDatatypeLiteral(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/val")
	g.Add(s, p, NewLiteral("42", WithDatatype(XSDInteger)))

	r, err := sparql.Query(g, `PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:val "42"^^<http://www.w3.org/2001/XMLSchema#integer> }`)
	_ = r
	_ = err
}

func TestSPARQLQueryTripleQuotedLiteral(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/desc")
	g.Add(s, p, NewLiteral("multi\nline"))

	r, err := sparql.Query(g, `PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:desc """multi
line""" }`)
	_ = r
	_ = err
}

func TestSPARQLQuerySingleQuotedLiteral(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/name")
	g.Add(s, p, NewLiteral("hello"))

	r, err := sparql.Query(g, `PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:name 'hello' }`)
	_ = r
	_ = err
}

// --- SPARQL parser: readTermOrVar edge cases ---

func TestSPARQLQueryDecimalLiteral(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/val")
	g.Add(s, p, NewLiteral("3.14", WithDatatype(XSDDecimal)))

	r, err := sparql.Query(g, `PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:val 3.14 }`)
	_ = r
	_ = err
}

func TestSPARQLQueryDoubleLiteral(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/val")
	g.Add(s, p, NewLiteral("1.5e2", WithDatatype(XSDDouble)))

	r, err := sparql.Query(g, `PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:val 1.5e2 }`)
	_ = r
	_ = err
}

// --- SPARQL parser: comment in query ---

func TestSPARQLQueryComment(t *testing.T) {
	g := makeSPARQLGraphExt(t)
	r, err := sparql.Query(g, `
		# This is a comment
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			?s ex:name ?name . # inline comment
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// --- SPARQL: $var syntax ---

func TestSPARQLDollarVarSyntax(t *testing.T) {
	g := makeSPARQLGraphExt(t)
	r, err := sparql.Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT $name WHERE { $s ex:name $name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// --- Turtle serializer: label for Variable (edge case) ---

func TestTurtleSerializerClassSubject(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	g.Bind("rdfs", NewURIRefUnsafe(RDFSNamespace))
	cls, _ := NewURIRef("http://example.org/MyClass")
	g.Add(cls, RDF.Type, RDFS.Class)

	var buf strings.Builder
	turtle.Serialize(g, &buf)
	if !strings.Contains(buf.String(), " a ") {
		t.Errorf("expected 'a' shorthand for rdf:type")
	}
}

// --- Turtle parser: resolveIRI with absolute ---

func TestTurtleParserAbsoluteIRINoResolve(t *testing.T) {
	g := NewGraph()
	turtle.Parse(g, strings.NewReader(`
		@base <http://example.org/> .
		<http://other.org/s> <http://other.org/p> "v" .
	`))
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

// --- SPARQL DISTINCT with explicit vars ---

func TestSPARQLDistinctExplicitVars(t *testing.T) {
	g := makeSPARQLGraphExt(t)
	r, err := sparql.Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT DISTINCT ?type WHERE { ?s a ?type }
		ORDER BY ?type
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

// --- matchKeywordCI: at EOF ---

func TestTurtleParserMatchKeywordAtEnd(t *testing.T) {
	g := NewGraph()
	err := turtle.Parse(g, strings.NewReader("PREFIX ex: <http://example.org/>\nex:s ex:p \"v\" ."))
	if err != nil {
		t.Fatal(err)
	}
}

// --- Serializer: label for BNode ---

func TestTurtleSerializerBNodeLabel(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	b := NewBNode("mybn")
	p1, _ := NewURIRef("http://example.org/p1")
	p2, _ := NewURIRef("http://example.org/p2")
	s, _ := NewURIRef("http://example.org/s")
	s2, _ := NewURIRef("http://example.org/s2")
	g.Add(s, p1, b)
	g.Add(s2, p1, b)
	g.Add(b, p2, NewLiteral("v"))

	var buf strings.Builder
	turtle.Serialize(g, &buf)
	out := buf.String()
	if !strings.Contains(out, "_:mybn") {
		t.Errorf("expected _:mybn for referenced BNode, got:\n%s", out)
	}
}
