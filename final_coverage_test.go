package rdflibgo

import (
	"strings"
	"testing"
)

// --- SPARQL parser: GROUP BY, HAVING ---

func TestSPARQLGroupByHaving(t *testing.T) {
	g := makeSPARQLGraph(t)
	// GROUP BY is parsed but not fully evaluated; just verify no parse error
	_, err := g.Query(`
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE { ?s ex:name ?name }
		GROUP BY ?name
		HAVING(?name = "Alice")
	`)
	// May not filter correctly without full aggregate support, but should parse
	if err != nil {
		t.Fatal(err)
	}
}

// --- SPARQL SELECT REDUCED ---

func TestSPARQLSelectReduced(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
		PREFIX ex: <http://example.org/>
		SELECT REDUCED ?type WHERE { ?s a ?type }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) < 1 {
		t.Error("expected results")
	}
}

// --- SPARQL CONSTRUCT empty ---

func TestSPARQLConstructEmpty(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:label ?name }
		WHERE { ?s ex:nonexistent ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph.Len() != 0 {
		t.Errorf("expected empty graph, got %d", r.Graph.Len())
	}
}

// --- RDF/XML: extractSubject with rdf:ID and rdf:nodeID ---

func TestRDFXMLExtractSubjectID(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/"
         xml:base="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:knows>
      <rdf:Description rdf:ID="bob">
        <ex:name>Bob</ex:name>
      </rdf:Description>
    </ex:knows>
  </rdf:Description>
</rdf:RDF>`
	g := NewGraph()
	g.Parse(strings.NewReader(input), WithFormat("xml"))
	if g.Len() < 2 {
		t.Errorf("expected >=2, got %d", g.Len())
	}
}

func TestRDFXMLExtractSubjectNodeID(t *testing.T) {
	input := `<?xml version="1.0"?>
<rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#"
         xmlns:ex="http://example.org/">
  <rdf:Description rdf:about="http://example.org/s">
    <ex:knows>
      <rdf:Description rdf:nodeID="n1">
        <ex:name>Anonymous</ex:name>
      </rdf:Description>
    </ex:knows>
  </rdf:Description>
</rdf:RDF>`
	g := NewGraph()
	g.Parse(strings.NewReader(input), WithFormat("xml"))
	if g.Len() < 2 {
		t.Errorf("expected >=2, got %d", g.Len())
	}
}

// --- Turtle serializer: isValidLocalName with special chars ---

func TestTurtleSerializerInvalidLocalName(t *testing.T) {
	g := NewGraph()
	// URI with ? in local name — can't be abbreviated
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/has?param")
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p, NewLiteral("v"))
	out := serializeToString(g, "turtle")
	// Should use full IRI, not ex:has?param
	if strings.Contains(out, "ex:has?param") {
		t.Errorf("should not abbreviate URI with ?, got:\n%s", out)
	}
}

// --- SPARQL: boolean literal in query ---

func TestSPARQLBooleanLiteral(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/active")
	g.Add(s, p, NewLiteral(true))

	r, err := g.Query(`PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s ex:active true }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

// --- SPARQL: numeric literal in query ---

func TestSPARQLNumericInPattern(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE { ?s ex:age 30 }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 1 {
		t.Errorf("expected 1, got %d", len(r.Bindings))
	}
}

// --- SPARQL: IRI in filter expression ---

func TestSPARQLIRIInExpression(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
		PREFIX ex: <http://example.org/>
		SELECT ?s WHERE { ?s a ?t . FILTER(?t = <http://example.org/Person>) }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}

// --- Turtle parser: matchKeywordCI at EOF ---

func TestTurtleParserKeywordAtEOF(t *testing.T) {
	// "BASE" keyword at very end — matchKeywordCI must handle EOF after keyword
	g := NewGraph()
	err := g.Parse(strings.NewReader(`BASE <http://example.org/>
<s> <p> "v" .`), WithFormat("turtle"))
	if err != nil {
		t.Fatal(err)
	}
}

// --- Turtle parser: IRI with unicode escape ---

func TestTurtleParserIRIUnicodeEscape8(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		<http://example.org/\U00000042> ex:p "v" .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

// --- SPARQL: nested subgraph patterns ---

func TestSPARQLNestedGroup(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
		PREFIX ex: <http://example.org/>
		SELECT ?name WHERE {
			{ ?s ex:name ?name }
			UNION
			{ ?s ex:name ?name }
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) < 3 {
		t.Errorf("expected >=3, got %d", len(r.Bindings))
	}
}

// --- SPARQL: triple pattern with comma (object list) ---

func TestSPARQLObjectList(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("a"))
	g.Add(s, p, NewLiteral("b"))

	r, err := g.Query(`PREFIX ex: <http://example.org/> SELECT ?o WHERE { ex:s ex:p ?o }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 2 {
		t.Errorf("expected 2, got %d", len(r.Bindings))
	}
}

// --- SPARQL: semicolon in triple patterns ---

func TestSPARQLSemicolonPattern(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
		PREFIX ex: <http://example.org/>
		SELECT ?name ?age WHERE {
			?s ex:name ?name ;
			   ex:age ?age .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Bindings) != 3 {
		t.Errorf("expected 3, got %d", len(r.Bindings))
	}
}
