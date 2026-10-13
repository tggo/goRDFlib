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
	if err := Parse(t.Context(), g, r); err != nil {
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
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1 triple, got %d", mustV(g.Len(t.Context())))
	}
}

func TestParseTurtleMultipleTriples(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p1 "a" .
		ex:s ex:p2 "b" .
	`)
	if mustV(g.Len(t.Context())) != 2 {
		t.Errorf("expected 2, got %d", mustV(g.Len(t.Context())))
	}
}

func TestParseTurtleSemicolon(t *testing.T) {
	// Ported from: W3C turtle test — predicate-object list
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p1 "a" ;
		     ex:p2 "b" .
	`)
	if mustV(g.Len(t.Context())) != 2 {
		t.Errorf("expected 2, got %d", mustV(g.Len(t.Context())))
	}
}

func TestParseTurtleComma(t *testing.T) {
	// Ported from: W3C turtle test — object list
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p "a", "b", "c" .
	`)
	if mustV(g.Len(t.Context())) != 3 {
		t.Errorf("expected 3, got %d", mustV(g.Len(t.Context())))
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
	if !mustV(g.Contains(t.Context(), s, rdflibgo.RDF.Type, thing)) {
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
	if mustV(g.Len(t.Context())) != 3 {
		t.Errorf("expected 3, got %d", mustV(g.Len(t.Context())))
	}
}

func TestParseTurtleBooleans(t *testing.T) {
	// Ported from: W3C turtle test — boolean literals
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:t true .
		ex:s ex:f false .
	`)
	if mustV(g.Len(t.Context())) != 2 {
		t.Errorf("expected 2, got %d", mustV(g.Len(t.Context())))
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
	val, ok, _ := g.Value(t.Context(), s, &p, nil)
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
	val, ok, _ := g.Value(t.Context(), s, &p, nil)
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
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
	}
}

func TestParseTurtleBlankNodePropertyList(t *testing.T) {
	// Ported from: W3C turtle test — anonymous blank node [ ... ]
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:knows [ ex:name "Alice" ] .
	`)
	if mustV(g.Len(t.Context())) != 2 {
		t.Errorf("expected 2 triples (s→bnode, bnode→name), got %d", mustV(g.Len(t.Context())))
	}
}

func TestParseTurtleStandaloneBlankNodePropertyList(t *testing.T) {
	// Standalone blank node property list as subject with no further predicates.
	// Per Turtle grammar: triples ::= blankNodePropertyList predicateObjectList?
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		[ ex:name "Alice" ; ex:age "30" ] .
	`)
	if mustV(g.Len(t.Context())) != 2 {
		t.Errorf("expected 2 triples, got %d", mustV(g.Len(t.Context())))
	}
}

func TestParseTurtleStandaloneBlankNodeWithFollowingTriples(t *testing.T) {
	// Standalone blank node property list followed by additional predicate-object pairs.
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		[ ex:name "Alice" ] ex:knows ex:Bob .
	`)
	if mustV(g.Len(t.Context())) != 2 {
		t.Errorf("expected 2 triples, got %d", mustV(g.Len(t.Context())))
	}
}

func TestParseTurtleNestedBlankNode(t *testing.T) {
	// Ported from: W3C turtle test — nested blank nodes
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:knows [ ex:name "Alice" ; ex:knows [ ex:name "Bob" ] ] .
	`)
	if mustV(g.Len(t.Context())) != 4 {
		t.Errorf("expected 4, got %d", mustV(g.Len(t.Context())))
	}
}

func TestParseTurtleCollection(t *testing.T) {
	// Ported from: W3C turtle test — collection syntax
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:list ( "a" "b" "c" ) .
	`)
	// 1 (s→head) + 3*(first+rest) = 7
	if mustV(g.Len(t.Context())) != 7 {
		t.Errorf("expected 7, got %d", mustV(g.Len(t.Context())))
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
	if !mustV(g.Contains(t.Context(), s, list, rdflibgo.RDF.Nil)) {
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
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
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
	if mustV(g.Len(t.Context())) != 3 {
		t.Errorf("expected 3, got %d", mustV(g.Len(t.Context())))
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
	val, ok, _ := g.Value(t.Context(), s, &p, nil)
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
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
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
	if !mustV(g.Contains(t.Context(), s, p, rdflibgo.NewLiteral("hello"))) {
		t.Error("expected resolved triple")
	}
}

func TestParseTurtleSPARQLPrefix(t *testing.T) {
	// Ported from: W3C turtle test — SPARQL-style PREFIX
	g := parseTurtle(t, `
		PREFIX ex: <http://example.org/>
		ex:s ex:p "hello" .
	`)
	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1, got %d", mustV(g.Len(t.Context())))
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
	if mustV(g.Len(t.Context())) != 2 {
		t.Errorf("expected 2, got %d", mustV(g.Len(t.Context())))
	}
}

func TestParseTurtleErrorUndefinedPrefix(t *testing.T) {
	// Ported from: W3C turtle negative syntax test — undefined prefix
	g := rdflibgo.NewGraph()
	r := strings.NewReader(`unknown:s unknown:p "hello" .`)
	err := Parse(t.Context(), g, r)
	if err == nil {
		t.Error("expected error for undefined prefix")
	}
}

func TestParseTurtleErrorUnterminatedString(t *testing.T) {
	g := rdflibgo.NewGraph()
	r := strings.NewReader(`@prefix ex: <http://example.org/> . ex:s ex:p "unterminated .`)
	err := Parse(t.Context(), g, r)
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
	if err := Serialize(t.Context(), g1, &buf); err != nil {
		t.Fatal(err)
	}

	// Parse again
	g2 := rdflibgo.NewGraph()
	if err := Parse(t.Context(), g2, strings.NewReader(buf.String())); err != nil {
		t.Fatalf("roundtrip parse failed: %v\nSerialized:\n%s", err, buf.String())
	}

	// Compare triple counts
	if mustV(g1.Len(t.Context())) != mustV(g2.Len(t.Context())) {
		t.Errorf("roundtrip: g1 has %d triples, g2 has %d\nSerialized:\n%s", mustV(g1.Len(t.Context())), mustV(g2.Len(t.Context())), buf.String())
	}
}
