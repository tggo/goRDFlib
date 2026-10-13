package integration_test

import (
	"strings"
	"testing"

	. "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/turtle"
)

// --- MustTerm success path (panic path already tested) ---

func TestClosedNamespaceMustTermSuccess(t *testing.T) {
	ns := NewClosedNamespace("http://example.org/", []string{"Foo"})
	u := ns.MustTerm("Foo")
	if u.Value() != "http://example.org/Foo" {
		t.Errorf("got %q", u.Value())
	}
}

// --- RDF/XML: parseRDFRoot xml:base ---

func TestRDFXMLParserXMLBaseOnRoot(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://base.org/">
  <rdf:Description rdf:about="s">
    <ex:p>v</ex:p>
  </rdf:Description>
</rdf:RDF>`
	g := NewGraph()
	rdfxml.Parse(t.Context(), g, strings.NewReader(input))
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

// --- RDF/XML: document without rdf:RDF wrapper ---

func TestRDFXMLParserNoWrapper(t *testing.T) {
	input := `<?xml version="1.0"?>
<ex:Thing xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
          xmlns:ex="http://example.org/"
          rdf:about="http://example.org/s">
  <ex:p>v</ex:p>
</ex:Thing>`
	g := NewGraph()
	err := rdfxml.Parse(t.Context(), g, strings.NewReader(input))
	if err != nil {
		t.Fatal(err)
	}
}

// --- Turtle: local name with backslash escape ---

func TestTurtleParserLocalNameWithBackslash(t *testing.T) {
	g := NewGraph()
	turtle.Parse(t.Context(), g, strings.NewReader(`
		@prefix ex: <http://example.org/> .
		ex:s ex:p ex:hello\.world .
	`))
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

// --- Turtle: matchKeywordCI short input ---

func TestTurtleParserShortInput(t *testing.T) {
	g := NewGraph()
	err := turtle.Parse(t.Context(), g, strings.NewReader("@prefix e: <http://e.o/> . e:s e:p e:o ."))
	if err != nil {
		t.Fatal(err)
	}
}

// --- Turtle: resolveIRI with fragment ---

func TestTurtleParserFragmentIRI(t *testing.T) {
	g := NewGraph()
	turtle.Parse(t.Context(), g, strings.NewReader(`
		@base <http://example.org/doc> .
		<#section> <#p> "v" .
	`))
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

// --- Turtle serializer: write with base ---

func TestTurtleSerializerWithBase(t *testing.T) {
	g := NewGraph()
	g.Bind(t.Context(), "ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, NewLiteral("v"))

	var buf strings.Builder
	turtle.Serialize(t.Context(), g, &buf, turtle.WithBase("http://example.org/"))
	if !strings.Contains(buf.String(), "@base") {
		t.Errorf("expected @base, got:\n%s", buf.String())
	}
}

// --- Turtle serializer: multiple objects ---

func TestTurtleSerializerMultipleObjectsSameSubjectPred(t *testing.T) {
	g := NewGraph()
	g.Bind(t.Context(), "ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, NewLiteral("a"))
	g.Add(t.Context(), s, p, NewLiteral("b"))
	g.Add(t.Context(), s, p, NewLiteral("c"))

	var buf strings.Builder
	turtle.Serialize(t.Context(), g, &buf)
	out := buf.String()
	if strings.Count(out, ",") < 2 {
		t.Errorf("expected commas for multiple objects, got:\n%s", out)
	}
}

// --- Turtle serializer: rdfs:label predicate ---

func TestTurtleSerializerRDFSLabelOrder(t *testing.T) {
	g := NewGraph()
	g.Bind(t.Context(), "ex", NewURIRefUnsafe("http://example.org/"))
	g.Bind(t.Context(), "rdfs", NewURIRefUnsafe(RDFSNamespace))
	s, _ := NewURIRef("http://example.org/s")
	other, _ := NewURIRef("http://example.org/zzz")
	g.Add(t.Context(), s, RDFS.Label, NewLiteral("My Label"))
	g.Add(t.Context(), s, other, NewLiteral("zzz"))

	var buf strings.Builder
	turtle.Serialize(t.Context(), g, &buf)
	out := buf.String()
	labelIdx := strings.Index(out, "rdfs:label")
	otherIdx := strings.Index(out, "ex:zzz")
	if labelIdx < 0 || otherIdx < 0 || labelIdx > otherIdx {
		t.Errorf("rdfs:label should come before other predicates:\n%s", out)
	}
}

// --- SPARQL: parseGroupGraphPattern BIND without parens ---

func TestSPARQLSubGroupPattern(t *testing.T) {
	g := makeSPARQLGraphExt(t)
	r, err := sparql.Query(g, `
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			{ ?s ex:name ?name . ?s ex:age ?age . FILTER(?age > 30) }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}
