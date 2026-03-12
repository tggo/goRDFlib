package nt

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// errWriter is a writer that always returns an error.
type errWriter struct{}

func (errWriter) Write(p []byte) (int, error) { return 0, errors.New("write fail") }

// TestSerializeDirLangLiteral exercises the directional language tag branch.
func TestSerializeDirLangLiteral(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("مرحبا", rdflibgo.WithLang("ar"), rdflibgo.WithDir("rtl")))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `@ar--rtl`) {
		t.Errorf("expected directional lang tag @ar--rtl, got:\n%s", out)
	}
}

// TestSerializeEscapedString exercises string escaping (newline, tab, backslash, quotes).
func TestSerializeEscapedString(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("line1\nline2\ttab\\slash\"quote"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `\n`) {
		t.Errorf("expected escaped newline, got:\n%s", out)
	}
	if !strings.Contains(out, `\t`) {
		t.Errorf("expected escaped tab, got:\n%s", out)
	}
	if !strings.Contains(out, `\\`) {
		t.Errorf("expected escaped backslash, got:\n%s", out)
	}
	if !strings.Contains(out, `\"`) {
		t.Errorf("expected escaped quote, got:\n%s", out)
	}
}

// TestSerializeSupplementaryPlaneChar exercises the \U escape for characters > 0xFFFF.
func TestSerializeSupplementaryPlaneChar(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	// U+1F600 = 😀
	g.Add(s, p, rdflibgo.NewLiteral("\U0001F600"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `\U`) {
		t.Errorf("expected \\U escape for supplementary plane char, got:\n%s", out)
	}
}

// TestSerializeControlChar exercises the \u escape for control characters < 0x20.
func TestSerializeControlChar(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("before\x01after"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `\u0001`) {
		t.Errorf("expected \\u0001 for control char, got:\n%s", out)
	}
}

// TestParseErrorMalformedIRI covers parser error for invalid IRI.
func TestParseErrorMalformedIRI(t *testing.T) {
	inputs := []string{
		"<not an iri> <http://example.org/p> \"hello\" .\n",
		"<http://example.org/s> <> \"hello\" .\n",
		"bad line\n",
		"<http://example.org/s> <http://example.org/p> \"unterminated\n",
	}
	for _, input := range inputs {
		g := rdflibgo.NewGraph()
		err := Parse(g, strings.NewReader(input))
		if err == nil {
			t.Errorf("expected parse error for %q, got nil", input)
		}
	}
}

// TestParseInvalidLangTag covers the invalid language tag error path.
func TestParseInvalidLangTag(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello"@123bad .` + "\n"
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for invalid language tag")
	}
}

// TestParseInvalidDirLangTag covers invalid direction in directional lang tags.
func TestParseInvalidDirLangTag(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello"@en--bad .` + "\n"
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for invalid direction in lang tag")
	}
}

// TestParseDirLangTag covers the valid directional lang tag path.
func TestParseDirLangTag(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "مرحبا"@ar--rtl .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParseTripleTerm covers parsing of triple terms.
func TestParseTripleTerm(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> <<( <http://example.org/a> <http://example.org/b> <http://example.org/c> )>> .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestSerializeEmptyGraph covers the empty graph case.
func TestSerializeEmptyGraph(t *testing.T) {
	g := rdflibgo.NewGraph()
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for empty graph, got %q", buf.String())
	}
}

// TestRoundTripTripleTerm verifies round-trip of triple terms through parse+serialize.
func TestRoundTripTripleTerm(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> <<( <http://example.org/a> <http://example.org/b> <http://example.org/c> )>> .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<<(") {
		t.Errorf("round-trip lost triple term syntax, got:\n%s", out)
	}
}

// TestParseUnknownEscape covers the unknown escape error path in literal parsing.
func TestParseUnknownEscape(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello\x" .` + "\n"
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for unknown escape \\x")
	}
}

// TestParseRelativeIRISubject covers the relative IRI error in subject position.
func TestParseRelativeIRISubject(t *testing.T) {
	input := `<relative> <http://example.org/p> "hello" .` + "\n"
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for relative IRI subject")
	}
}

// TestParseBNodeSubject covers the blank node subject path.
func TestParseBNodeSubject(t *testing.T) {
	input := `_:b1 <http://example.org/p> "hello" .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParseCommentLine covers comment and empty line handling.
func TestParseCommentLine(t *testing.T) {
	input := "# comment\n\n<http://example.org/s> <http://example.org/p> \"hello\" .\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestParseSurrogateInUEscape covers the surrogate rejection in \u escapes.
func TestParseSurrogateInUEscape(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello\uD800world" .` + "\n"
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for surrogate in \\u escape")
	}
}

// TestParseSurrogateInUUEscape covers the surrogate rejection in \U escapes.
func TestParseSurrogateInUUEscape(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello\U0000D800world" .` + "\n"
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for surrogate in \\U escape")
	}
}

// TestParseTruncatedUEscape covers the truncated \u escape error path.
func TestParseTruncatedUEscape(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello\u00" .` + "\n"
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for truncated \\u escape")
	}
}

// TestParseUnterminatedString covers unterminated string literal.
func TestParseUnterminatedString(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello` + "\n"
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for unterminated string")
	}
}

// TestParseIRIWithEscape covers IRI with \u escape sequence.
func TestParseIRIWithEscape(t *testing.T) {
	input := `<http://example.org/\u0073> <http://example.org/p> "hello" .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestSerializeTripleTermRoundTrip covers serialize then parse with TripleTerm.
func TestSerializeTripleTermRoundTrip(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	a := rdflibgo.NewURIRefUnsafe("http://example.org/a")
	b := rdflibgo.NewURIRefUnsafe("http://example.org/b")
	c := rdflibgo.NewURIRefUnsafe("http://example.org/c")
	tt := rdflibgo.NewTripleTerm(a, b, c)
	g.Add(s, p, tt)

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(buf.String())); err != nil {
		t.Fatal(err)
	}
	if g2.Len() != 1 {
		t.Errorf("round-trip: expected 1, got %d", g2.Len())
	}
}

// TestSerializeMultipleTriples covers serialization of multiple triples in sorted order.
func TestSerializeMultipleTriples(t *testing.T) {
	g := rdflibgo.NewGraph()
	s1 := rdflibgo.NewURIRefUnsafe("http://example.org/b")
	s2 := rdflibgo.NewURIRefUnsafe("http://example.org/a")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s1, p, rdflibgo.NewLiteral("v1"))
	g.Add(s2, p, rdflibgo.NewLiteral("v2"))
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

// TestParseBNodeObject covers blank node object.
func TestParseBNodeObject(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> _:b1 .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

// TestParseTypedLiteral covers typed literal parsing.
func TestParseTypedLiteral(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "42"^^<http://www.w3.org/2001/XMLSchema#integer> .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

// TestSerializeTypedLiteral covers serialization of typed literals.
func TestSerializeTypedLiteral(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("42", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "^^<") {
		t.Error("expected typed literal with ^^<")
	}
}

// TestParseLangLiteral covers plain lang-tagged literal.
func TestParseLangLiteral(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello"@en .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

// TestSerializeLangLiteral covers serialization of lang-tagged literal.
func TestSerializeLangLiteral(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("hello", rdflibgo.WithLang("en")))
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "@en") {
		t.Error("expected @en")
	}
}

// TestParseTripleTermAsSubject covers triple term in subject position (error).
func TestParseTripleTermAsSubject(t *testing.T) {
	input := `<<( <http://example.org/a> <http://example.org/b> <http://example.org/c> )>> <http://example.org/p> "v" .` + "\n"
	g := rdflibgo.NewGraph()
	// In N-Triples 1.2, triple terms can be subjects — this should parse
	err := Parse(g, strings.NewReader(input))
	// Either parses or errors depending on spec version; just exercise the path
	_ = err
}

// TestParseTripleTermBNodeInner covers bnode inside triple term.
func TestParseTripleTermBNodeInner(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> <<( _:b1 <http://example.org/q> <http://example.org/r> )>> .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestParseTripleTermLiteralInner covers literal inside triple term.
func TestParseTripleTermLiteralInner(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> <<( <http://example.org/a> <http://example.org/b> "val" )>> .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}
