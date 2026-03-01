package rdflibgo

import (
	"bytes"
	"strings"
	"testing"
)

// --- Turtle serializer: label for BNode and Variable ---

func TestTurtleSerializerRDFSClassFirst(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	g.Bind("rdfs", NewURIRefUnsafe(RDFSNamespace))
	cls, _ := NewURIRef("http://example.org/MyClass")
	g.Add(cls, RDF.Type, RDFS.Class)
	g.Add(cls, RDFS.Label, NewLiteral("My Class"))

	s, _ := NewURIRef("http://example.org/instance")
	g.Add(s, RDF.Type, cls)

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("turtle"))
	out := buf.String()
	// RDFS.Class typed subject should appear before regular subjects
	classIdx := strings.Index(out, "ex:MyClass")
	instanceIdx := strings.Index(out, "ex:instance")
	if classIdx < 0 || instanceIdx < 0 {
		t.Fatalf("missing subjects in:\n%s", out)
	}
	if classIdx > instanceIdx {
		t.Errorf("rdfs:Class subjects should come first")
	}
}

func TestTurtleSerializerInvalidList(t *testing.T) {
	// A list-like structure that's invalid (has extra properties)
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/list")
	b := NewBNode()
	g.Add(s, p, b)
	g.Add(b, RDF.First, NewLiteral("a"))
	g.Add(b, RDF.Rest, RDF.Nil)
	extra, _ := NewURIRef("http://example.org/extra")
	g.Add(b, extra, NewLiteral("x")) // invalidates list

	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("turtle"))
	// Should NOT use ( ) syntax since list is invalid
	if strings.Contains(buf.String(), "( ") {
		t.Errorf("invalid list should not use collection syntax:\n%s", buf.String())
	}
}

// --- Turtle parser: error conditions ---

func TestTurtleParserEscapeError(t *testing.T) {
	g := NewGraph()
	err := g.Parse(strings.NewReader(`@prefix ex: <http://example.org/> . ex:s ex:p "\z" .`), WithFormat("turtle"))
	if err == nil {
		t.Error("expected error for unknown escape \\z")
	}
}

func TestTurtleParserUnterminatedLongString(t *testing.T) {
	g := NewGraph()
	err := g.Parse(strings.NewReader(`@prefix ex: <http://example.org/> . ex:s ex:p """unterminated`), WithFormat("turtle"))
	if err == nil {
		t.Error("expected error")
	}
}

func TestTurtleParserUnterminatedShortString(t *testing.T) {
	g := NewGraph()
	err := g.Parse(strings.NewReader("@prefix ex: <http://example.org/> . ex:s ex:p \"unterminated\n"), WithFormat("turtle"))
	if err == nil {
		t.Error("expected error for newline in short string")
	}
}

func TestTurtleParserUnterminatedUnicodeEscape(t *testing.T) {
	g := NewGraph()
	err := g.Parse(strings.NewReader(`@prefix ex: <http://example.org/> . ex:s ex:p "\u00" .`), WithFormat("turtle"))
	if err == nil {
		t.Error("expected error for truncated unicode escape")
	}
}

// --- Turtle parser: tryNumeric edge cases ---

func TestTurtleParserPositiveNumeric(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p +42 .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestTurtleParserDecimalNumeric(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p 3.14 .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestTurtleParserDoubleNumeric(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p 1.5e10 .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

// --- MulPath backward eval ---

func TestMulPathBackwardEval(t *testing.T) {
	g := makePathGraph(t)
	p, _ := NewURIRef("http://example.org/p")
	d, _ := NewURIRef("http://example.org/d")

	path := ZeroOrMore(AsPath(p))
	var pairs [][2]string
	path.Eval(g, nil, d)(func(s, o Term) bool {
		pairs = append(pairs, [2]string{s.N3(), o.N3()})
		return true
	})
	// d→d (identity), c→d, b→d (via b→c→d), a→d (via a→b→c→d)
	if len(pairs) < 4 {
		t.Errorf("expected >=4 from backward p* to d, got %d: %v", len(pairs), pairs)
	}
}

// --- AlternativePath early stop ---

func TestAlternativePathEarlyStop(t *testing.T) {
	g := makePathGraph(t)
	a, _ := NewURIRef("http://example.org/a")
	p, _ := NewURIRef("http://example.org/p")
	q, _ := NewURIRef("http://example.org/q")

	path := Alternative(AsPath(p), AsPath(q))
	count := 0
	path.Eval(g, a, nil)(func(s, o Term) bool {
		count++
		return false // stop after first
	})
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}

// --- SPARQL: resolveTermRef branches ---

func TestSPARQLPrefixedNameInConstruct(t *testing.T) {
	g := makeSPARQLGraph(t)
	r, err := g.Query(`
		PREFIX ex: <http://example.org/>
		CONSTRUCT { ?s ex:alias ?name }
		WHERE { ?s ex:name ?name }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if r.Graph.Len() != 3 {
		t.Errorf("expected 3, got %d", r.Graph.Len())
	}
}

// --- SPARQL eval: unary minus on float ---

func TestSPARQLUnaryMinusFloat(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/val")
	g.Add(s, p, NewLiteral(3.14))

	r, _ := g.Query(`PREFIX ex: <http://example.org/> SELECT ?neg WHERE { ?s ex:val ?v . BIND(-?v AS ?neg) }`)
	if len(r.Bindings) != 1 {
		t.Fatalf("expected 1, got %d", len(r.Bindings))
	}
}

// --- Serializer: writePredicates empty ---

func TestTurtleSerializerSinglePredicate(t *testing.T) {
	g := NewGraph()
	g.Bind("ex", NewURIRefUnsafe("http://example.org/"))
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("v"))
	var buf bytes.Buffer
	g.Serialize(&buf, WithSerializeFormat("turtle"))
	// Should have "." terminator
	if !strings.Contains(buf.String(), " .") {
		t.Error("expected dot terminator")
	}
}
