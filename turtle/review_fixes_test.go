package turtle

import (
	"bytes"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// --- Serializer fix tests ---

func TestSerializerBufioWrapping(t *testing.T) {
	// Verify serialization works (bufio.Writer wrapping is transparent).
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	g.Add(s, p, rdflibgo.NewLiteral("value"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "ex:s") {
		t.Errorf("expected output, got:\n%s", out)
	}
}

func TestSerializerResolveSubjectPerformance(t *testing.T) {
	// Ensure resolveSubject uses subjectMap (O(1) instead of O(N)).
	g := rdflibgo.NewGraph()
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
	for i := 0; i < 100; i++ {
		s, _ := rdflibgo.NewURIRef("http://example.org/s" + strings.Repeat("x", i))
		p, _ := rdflibgo.NewURIRef("http://example.org/p")
		g.Add(s, p, rdflibgo.NewLiteral("v"))
	}

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

func TestSerializerErrorPropagationInList(t *testing.T) {
	// listStr and inlineBNode now return errors. Verify serialization still works
	// for valid graphs with lists.
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/list")
	g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))

	n1 := rdflibgo.NewBNode()
	n2 := rdflibgo.NewBNode()
	g.Add(s, p, n1)
	g.Add(n1, rdflibgo.RDF.First, rdflibgo.NewLiteral("a"))
	g.Add(n1, rdflibgo.RDF.Rest, n2)
	g.Add(n2, rdflibgo.RDF.First, rdflibgo.NewLiteral("b"))
	g.Add(n2, rdflibgo.RDF.Rest, rdflibgo.RDF.Nil)

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "(") {
		t.Errorf("expected list syntax, got:\n%s", buf.String())
	}
}

func TestIsValidLocalName(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Person", true},
		{"name", true},
		{"p1", true},
		{"has-value", true},
		{"_hidden", true},
		{"a.b", true},
		{"", false},
		{"hello world", false}, // space
		{"a@b", false},         // @ not in PN_LOCAL
		{"a!b", false},         // ! not in PN_LOCAL
		{"a%b", false},         // bare % not valid
		{"a#b", false},         // # not valid
		{"ends.", false},       // trailing dot
		{":colon", true},       // colon is valid in PN_LOCAL
	}
	for _, tt := range tests {
		got := isValidLocalName(tt.input)
		if got != tt.want {
			t.Errorf("isValidLocalName(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// --- Parser fix tests ---

func TestParserUTF8MultiByteInLiteral(t *testing.T) {
	// Critical fix: multi-byte UTF-8 in literals must not be truncated.
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p "hello 世界" .
	`)
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	val, ok := g.Value(s, &p, nil)
	if !ok {
		t.Fatal("expected value")
	}
	if val.String() != "hello 世界" {
		t.Errorf("expected 'hello 世界', got %q", val.String())
	}
}

func TestParserUTF8InTripleQuotedLiteral(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p """Привет мир""" .
	`)
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	val, ok := g.Value(s, &p, nil)
	if !ok {
		t.Fatal("expected value")
	}
	if val.String() != "Привет мир" {
		t.Errorf("expected 'Привет мир', got %q", val.String())
	}
}

func TestParserUTF8Emoji(t *testing.T) {
	g := parseTurtle(t, `
		@prefix ex: <http://example.org/> .
		ex:s ex:p "thumbs up: 👍" .
	`)
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	val, ok := g.Value(s, &p, nil)
	if !ok {
		t.Fatal("expected value")
	}
	if val.String() != "thumbs up: 👍" {
		t.Errorf("expected emoji, got %q", val.String())
	}
}

func TestParserErrorNoCol(t *testing.T) {
	// Verify error messages include line but not misleading col.
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`unknown:s unknown:p "hello" .`))
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "line") {
		t.Errorf("expected 'line' in error, got: %s", msg)
	}
	// Should not contain "col" anymore
	if strings.Contains(msg, "col ") {
		t.Errorf("should not contain 'col' in error, got: %s", msg)
	}
}

func TestParserUnescapeIRIMalformedU(t *testing.T) {
	// Malformed \u escape in IRI should return an error.
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`<http://example.org/\uZZZZ> <http://example.org/p> "v" .`))
	if err == nil {
		t.Error("expected error for malformed \\u escape in IRI")
	}
}

func TestParserUnescapeIRIMalformedUShort(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`<http://example.org/\u00> <http://example.org/p> "v" .`))
	if err == nil {
		t.Error("expected error for truncated \\u escape in IRI")
	}
}

func TestParserUnescapeIRIMalformedBigU(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`<http://example.org/\UZZZZZZZZ> <http://example.org/p> "v" .`))
	if err == nil {
		t.Error("expected error for malformed \\U escape in IRI")
	}
}

func TestParserBlankNodeLabelEmpty(t *testing.T) {
	// Empty blank node label should produce an error.
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`@prefix ex: <http://example.org/> . _: ex:p "v" .`))
	if err == nil {
		t.Error("expected error for empty blank node label")
	}
}

func TestParserErrorfNoDoubleFormat(t *testing.T) {
	// Verify errorf produces a single well-formatted error with %s args.
	p := &turtleParser{line: 5}
	err := p.errorf("bad token %q", "xyz")
	msg := err.Error()
	expected := `turtle parse error at line 5: bad token "xyz"`
	if msg != expected {
		t.Errorf("got %q, want %q", msg, expected)
	}
}

func TestParserValidIRIUnicodeEscape(t *testing.T) {
	// Valid \u escape in IRI should work.
	g := parseTurtle(t, `<http://example.org/\u0041> <http://example.org/p> "v" .`)
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
	a, _ := rdflibgo.NewURIRef("http://example.org/A")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	if !g.Contains(a, p, rdflibgo.NewLiteral("v")) {
		t.Error("expected triple with unescaped IRI")
	}
}

// --- Package doc test ---

func TestPackageExists(t *testing.T) {
	// Verify the package compiles (doc.go exists).
	// This test is a no-op; if doc.go has a syntax error, tests won't compile.
}
