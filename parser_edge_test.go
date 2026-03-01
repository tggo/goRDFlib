package rdflibgo

import (
	"strings"
	"testing"
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
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
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
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
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
	g := NewGraph()
	err := g.Parse(strings.NewReader(`<http://example.org/s`), WithFormat("turtle"))
	if err == nil {
		t.Error("expected error")
	}
}

func TestTurtleParserErrorMissingColon(t *testing.T) {
	g := NewGraph()
	err := g.Parse(strings.NewReader(`@prefix ex <http://example.org/> .`), WithFormat("turtle"))
	if err == nil {
		t.Error("expected error")
	}
}

func TestTurtleParserErrorMissingDot(t *testing.T) {
	g := NewGraph()
	err := g.Parse(strings.NewReader(`@prefix ex: <http://example.org/> . ex:s ex:p "v"`), WithFormat("turtle"))
	if err == nil {
		t.Error("expected error for missing dot")
	}
}

func TestTurtleParserErrorUnterminatedGroup(t *testing.T) {
	g := NewGraph()
	err := g.Parse(strings.NewReader(`@prefix ex: <http://example.org/> . ex:s ex:p [`), WithFormat("turtle"))
	if err == nil {
		t.Error("expected error for unterminated [")
	}
}

func TestTurtleParserErrorUnterminatedCollection(t *testing.T) {
	g := NewGraph()
	err := g.Parse(strings.NewReader(`@prefix ex: <http://example.org/> . ex:s ex:p (`), WithFormat("turtle"))
	if err == nil {
		t.Error("expected error for unterminated (")
	}
}

func TestTurtleParserErrorUnknownDirective(t *testing.T) {
	g := NewGraph()
	err := g.Parse(strings.NewReader(`@unknown directive .`), WithFormat("turtle"))
	if err == nil {
		t.Error("expected error for unknown directive")
	}
}

// --- N-Triples parser edge cases ---

func TestNTParserEscapeTab(t *testing.T) {
	g := NewGraph()
	g.Parse(strings.NewReader(`<http://s> <http://p> "a\tb" .`+"\n"), WithFormat("nt"))
	s := NewURIRefUnsafe("http://s")
	p := NewURIRefUnsafe("http://p")
	v, _ := g.Value(s, &p, nil)
	if v.String() != "a\tb" {
		t.Errorf("got %q", v.String())
	}
}

func TestNTParserEscapeQuote(t *testing.T) {
	g := NewGraph()
	g.Parse(strings.NewReader(`<http://s> <http://p> "say \"hi\"" .`+"\n"), WithFormat("nt"))
	s := NewURIRefUnsafe("http://s")
	p := NewURIRefUnsafe("http://p")
	v, _ := g.Value(s, &p, nil)
	if v.String() != `say "hi"` {
		t.Errorf("got %q", v.String())
	}
}

func TestNTParserEscapeBackslash(t *testing.T) {
	g := NewGraph()
	g.Parse(strings.NewReader(`<http://s> <http://p> "a\\b" .`+"\n"), WithFormat("nt"))
	s := NewURIRefUnsafe("http://s")
	p := NewURIRefUnsafe("http://p")
	v, _ := g.Value(s, &p, nil)
	if v.String() != `a\b` {
		t.Errorf("got %q", v.String())
	}
}

func TestNTParserEscapeCarriageReturn(t *testing.T) {
	g := NewGraph()
	g.Parse(strings.NewReader(`<http://s> <http://p> "a\rb" .`+"\n"), WithFormat("nt"))
	s := NewURIRefUnsafe("http://s")
	p := NewURIRefUnsafe("http://p")
	v, _ := g.Value(s, &p, nil)
	if v.String() != "a\rb" {
		t.Errorf("got %q", v.String())
	}
}

func TestNTParserUnicodeEscape8(t *testing.T) {
	g := NewGraph()
	g.Parse(strings.NewReader(`<http://s> <http://p> "\U00000042" .`+"\n"), WithFormat("nt"))
	s := NewURIRefUnsafe("http://s")
	p := NewURIRefUnsafe("http://p")
	v, _ := g.Value(s, &p, nil)
	if v.String() != "B" {
		t.Errorf("expected B, got %q", v.String())
	}
}

// --- Serializer edge cases ---

func TestNTSerializerTab(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://s")
	p, _ := NewURIRef("http://p")
	g.Add(s, p, NewLiteral("a\tb"))
	out := serializeToString(g, "nt")
	if !strings.Contains(out, `\t`) {
		t.Errorf("expected tab escape, got:\n%s", out)
	}
}

func serializeToString(g *Graph, format string) string {
	var buf strings.Builder
	g.Serialize(&buf, WithSerializeFormat(format))
	return buf.String()
}
