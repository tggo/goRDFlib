package turtle

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// --- Turtle parser edge cases for coverage ---

func TestTurtleParserIRIWithUnicodeEscape(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		<http://example.org/\u0041> ex:p "v" .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestTurtleParserLocalNameEscape(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p ex:hello%20world .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestTurtleParserEscapeTab(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p "a\tb" .
	`)
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	v, _ := g.Value(s, &p, nil)
	if v.String() != "a\tb" {
		t.Errorf("got %q", v.String())
	}
}

func TestTurtleParserEscapeSingleQuote(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p "it\'s" .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestTurtleParserUnicodeEscape8(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p "\U00000041BC" .
	`)
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	v, _ := g.Value(s, &p, nil)
	if v.String() != "ABC" {
		t.Errorf("expected ABC, got %q", v.String())
	}
}

func TestTurtleParserTripleQuotedSingleQuote(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p '''multi
line''' .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestTurtleParserPredicateAsIRI(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s <http://example.org/p> "v" .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestTurtleParserObjectAsIRI(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p <http://example.org/o> .
	`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestTurtleParserBooleanObject(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p true .
		ex:s ex:q false .
	`)
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
}

func TestTurtleParserErrorUnterminatedIRI(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`<http://example.org/s`))
	if err == nil {
		t.Error("expected error")
	}
}

func TestTurtleParserErrorMissingColon(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`@prefix ex <http://example.org/> .`))
	if err == nil {
		t.Error("expected error")
	}
}

func TestTurtleParserErrorMissingDot(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`@prefix ex: <http://example.org/> . ex:s ex:p "v"`))
	if err == nil {
		t.Error("expected error for missing dot")
	}
}

func TestTurtleParserErrorUnterminatedGroup(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`@prefix ex: <http://example.org/> . ex:s ex:p [`))
	if err == nil {
		t.Error("expected error for unterminated [")
	}
}

func TestTurtleParserErrorUnterminatedCollection(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`@prefix ex: <http://example.org/> . ex:s ex:p (`))
	if err == nil {
		t.Error("expected error for unterminated (")
	}
}

func TestTurtleParserErrorUnknownDirective(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`@unknown directive .`))
	if err == nil {
		t.Error("expected error for unknown directive")
	}
}
