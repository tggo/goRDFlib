package turtle

import (
	"bytes"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Ported from: test/test_w3c_spec/test_turtle_w3c.py, test/test_roundtrip.py

func parseTurtle(t *testing.T, input string) *rdflibgo.Graph {
	t.Helper()
	g := rdflibgo.NewGraph()
	r := strings.NewReader(input)
	if err := Parse(g, r); err != nil {
		t.Fatal(err)
	}
	return g
}

func TestParseTurtleBasicTriple(t *testing.T) {
	// Ported from: W3C turtle test — basic subject-predicate-object
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p "hello" .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

func TestParseTurtleMultipleTriples(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p1 "a" .
		ex:s ex:p2 "b" .
	`)
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
}

func TestParseTurtleSemicolon(t *testing.T) {
	// Ported from: W3C turtle test — predicate-object list
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p1 "a" ;
		     ex:p2 "b" .
	`)
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
}

func TestParseTurtleComma(t *testing.T) {
	// Ported from: W3C turtle test — object list
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p "a", "b", "c" .
	`)
	if g.Len() != 3 {
		t.Errorf("expected 3, got %d", g.Len())
	}
}

func TestParseTurtleAShorthand(t *testing.T) {
	// Ported from: W3C turtle test — "a" for rdf:type
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s a ex:Thing .
	`)
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	thing, _ := rdflibgo.NewURIRef("http://example.org/Thing")
	if !g.Contains(s, rdflibgo.RDF.Type, thing) {
		t.Error("expected rdf:type triple")
	}
}

func TestParseTurtleNumericLiterals(t *testing.T) {
	// Ported from: W3C turtle test — numeric shorthand
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:int 42 .
		ex:s ex:dec 3.14 .
		ex:s ex:dbl 1.5e2 .
	`)
	if g.Len() != 3 {
		t.Errorf("expected 3, got %d", g.Len())
	}
}

func TestParseTurtleBooleans(t *testing.T) {
	// Ported from: W3C turtle test — boolean literals
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:t true .
		ex:s ex:f false .
	`)
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
}

func TestParseTurtleLangTag(t *testing.T) {
	// Ported from: W3C turtle test — language-tagged literal
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p "hello"@en .
	`)
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	val, ok := g.Value(s, &p, nil)
	if !ok {
		t.Fatal("expected value")
	}
	lit, ok := val.(rdflibgo.Literal)
	if !ok {
		t.Fatal("expected Literal")
	}
	if lit.Language() != "en" {
		t.Errorf("expected en, got %q", lit.Language())
	}
}

func TestParseTurtleTypedLiteral(t *testing.T) {
	// Ported from: W3C turtle test — datatype
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:s ex:p "42"^^xsd:integer .
	`)
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	val, ok := g.Value(s, &p, nil)
	if !ok {
		t.Fatal("expected value")
	}
	lit, ok := val.(rdflibgo.Literal)
	if !ok {
		t.Fatal("expected Literal")
	}
	if lit.Datatype() != rdflibgo.XSDInteger {
		t.Errorf("expected xsd:integer, got %v", lit.Datatype())
	}
}

func TestParseTurtleBlankNodeLabel(t *testing.T) {
	// Ported from: W3C turtle test — named blank node
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		_:b1 ex:p "hello" .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestParseTurtleBlankNodePropertyList(t *testing.T) {
	// Ported from: W3C turtle test — anonymous blank node [ ... ]
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:knows [ ex:name "Alice" ] .
	`)
	if g.Len() != 2 {
		t.Errorf("expected 2 triples (s→bnode, bnode→name), got %d", g.Len())
	}
}

func TestParseTurtleNestedBlankNode(t *testing.T) {
	// Ported from: W3C turtle test — nested blank nodes
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:knows [ ex:name "Alice" ; ex:knows [ ex:name "Bob" ] ] .
	`)
	if g.Len() != 4 {
		t.Errorf("expected 4, got %d", g.Len())
	}
}

func TestParseTurtleCollection(t *testing.T) {
	// Ported from: W3C turtle test — collection syntax
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:list ( "a" "b" "c" ) .
	`)
	// 1 (s→head) + 3*(first+rest) = 7
	if g.Len() != 7 {
		t.Errorf("expected 7, got %d", g.Len())
	}
}

func TestParseTurtleEmptyCollection(t *testing.T) {
	// Ported from: W3C turtle test — empty collection = rdf:nil
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:list () .
	`)
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	list, _ := rdflibgo.NewURIRef("http://example.org/list")
	if !g.Contains(s, list, rdflibgo.RDF.Nil) {
		t.Error("expected rdf:nil as object")
	}
}

func TestParseTurtleMultilineString(t *testing.T) {
	// Ported from: W3C turtle test — triple-quoted string
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p """line1
line2""" .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestParseTurtleEscapeSequences(t *testing.T) {
	// Ported from: W3C turtle test — escape sequences
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p "hello\nworld" .
		ex:s ex:p2 "tab\there" .
		ex:s ex:p3 "quote\"here" .
	`)
	if g.Len() != 3 {
		t.Errorf("expected 3, got %d", g.Len())
	}
}

func TestParseTurtleUnicodeEscape(t *testing.T) {
	// Ported from: W3C turtle test — unicode escapes
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p "\u0041BC" .
	`)
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	val, ok := g.Value(s, &p, nil)
	if !ok {
		t.Fatal("expected value")
	}
	if val.String() != "ABC" {
		t.Errorf("expected ABC, got %q", val.String())
	}
}

func TestParseTurtleComments(t *testing.T) {
	g := parseTurtle(t, `
		# This is a comment
		@prefix ex: <http://example.org/> .
		ex:s ex:p "hello" . # inline comment
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestParseTurtleBaseDirective(t *testing.T) {
	// Ported from: W3C turtle test — @base
	g := parseTurtle(t, `
		@base <http://example.org/> .
		<s> <p> "hello" .
	`)
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	if !g.Contains(s, p, rdflibgo.NewLiteral("hello")) {
		t.Error("expected resolved triple")
	}
}

func TestParseTurtleSPARQLPrefix(t *testing.T) {
	// Ported from: W3C turtle test — SPARQL-style PREFIX
	g := parseTurtle(t, `
		PREFIX ex: <http://example.org/>
		ex:s ex:p "hello" .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestParseTurtleTrailingSemicolon(t *testing.T) {
	// Ported from: W3C turtle test — trailing semicolon
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p "a" ;
		     ex:q "b" ;
		.
	`)
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
}

func TestParseTurtleErrorUndefinedPrefix(t *testing.T) {
	// Ported from: W3C turtle negative syntax test — undefined prefix
	g := rdflibgo.NewGraph()
	r := strings.NewReader(`unknown:s unknown:p "hello" .`)
	err := Parse(g, r)
	if err == nil {
		t.Error("expected error for undefined prefix")
	}
}

func TestParseTurtleErrorUnterminatedString(t *testing.T) {
	g := rdflibgo.NewGraph()
	r := strings.NewReader(`@prefix ex: <http://example.org/> . ex:s ex:p "unterminated .`)
	err := Parse(g, r)
	if err == nil {
		t.Error("expected error for unterminated string")
	}
}

// --- Roundtrip test ---

func TestTurtleRoundtrip(t *testing.T) {
	// Ported from: test/test_roundtrip.py — parse → serialize → parse → compare
	input := `@prefix ex: <http://example.org/> .

ex:Alice a ex:Person ;
    ex:name "Alice" ;
    ex:age 30 .

ex:Bob a ex:Person ;
    ex:name "Bob" .
`
	g1 := parseTurtle(t, input)

	// Serialize
	var buf bytes.Buffer
	if err := Serialize(g1, &buf); err != nil {
		t.Fatal(err)
	}

	// Parse again
	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(buf.String())); err != nil {
		t.Fatalf("roundtrip parse failed: %v\nSerialized:\n%s", err, buf.String())
	}

	// Compare triple counts
	if g1.Len() != g2.Len() {
		t.Errorf("roundtrip: g1 has %d triples, g2 has %d\nSerialized:\n%s", g1.Len(), g2.Len(), buf.String())
	}
}
