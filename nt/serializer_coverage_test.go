package nt

import (
	"bytes"
	"io"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// TestWithBase ensures WithBase constructs an Option that sets the base field.
// The nt parser does not currently use it, but the option must be accepted
// without error by both Parse and Serialize so the exported API is exercised.
func TestWithBase(t *testing.T) {
	opt := WithBase("http://base.example/")
	var cfg config
	opt(&cfg)
	if cfg.base != "http://base.example/" {
		t.Errorf("WithBase: got %q, want %q", cfg.base, "http://base.example/")
	}
}

// TestSerializeWithBaseOption passes WithBase to Serialize to cover the
// option-iteration code path inside Serialize.
func TestSerializeWithBaseOption(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("hi"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf, WithBase("http://base.example/")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"hi"`) {
		t.Errorf("unexpected output: %s", buf.String())
	}
}

// TestSerializeBNode ensures blank-node subjects serialise as _:id.
func TestSerializeBNode(t *testing.T) {
	g := rdflibgo.NewGraph()
	bn := rdflibgo.NewBNode("b1")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(bn, p, rdflibgo.NewLiteral("value"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "_:b1") {
		t.Errorf("expected blank-node subject _:b1, got:\n%s", out)
	}
}

// TestSerializeLiteralWithLang exercises the lang-tag branch in ntsyntax.Literal.
func TestSerializeLiteralWithLang(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("bonjour", rdflibgo.WithLang("fr")))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"bonjour"@fr`) {
		t.Errorf("expected lang-tagged literal, got:\n%s", out)
	}
}

// TestSerializeLiteralWithDatatype exercises the datatype branch in ntsyntax.Literal.
func TestSerializeLiteralWithDatatype(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("3.14", rdflibgo.WithDatatype(rdflibgo.XSDDouble)))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"3.14"^^<http://www.w3.org/2001/XMLSchema#double>`) {
		t.Errorf("expected datatype IRI, got:\n%s", out)
	}
}

// TestSerializeTripleTerm exercises the TripleTerm branch in ntsyntax.Term.
func TestSerializeTripleTerm(t *testing.T) {
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	tt := rdflibgo.NewTripleTerm(s, p, o)

	// Use the triple term as an object
	g := rdflibgo.NewGraph()
	outer_p := rdflibgo.NewURIRefUnsafe("http://example.org/asserts")
	g.Add(s, outer_p, tt)

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<<(") {
		t.Errorf("expected triple term syntax <<(...), got:\n%s", out)
	}
}

// TestSerializeTripleTermAsSubject exercises TripleTerm used as graph subject.
func TestSerializeTripleTermAsSubject(t *testing.T) {
	inner_s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	inner_p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	inner_o := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	tt := rdflibgo.NewTripleTerm(inner_s, inner_p, inner_o)

	g := rdflibgo.NewGraph()
	outer_p := rdflibgo.NewURIRefUnsafe("http://example.org/occursIn")
	outer_o := rdflibgo.NewURIRefUnsafe("http://example.org/doc")
	g.Add(tt, outer_p, outer_o)

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<<(") {
		t.Errorf("expected triple term syntax <<(...), got:\n%s", out)
	}
}

// TestSerializeLiteralPlain ensures a plain literal (no lang, xsd:string
// datatype suppressed) serialises as just a quoted string.
func TestSerializeLiteralPlain(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("plain"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"plain"`) {
		t.Errorf("expected plain literal, got:\n%s", out)
	}
	if strings.Contains(out, "^^") {
		t.Errorf("plain literal must not carry datatype annotation, got:\n%s", out)
	}
}

// TestSerializeWriteError covers the error path when the writer fails.
func TestSerializeWriteError(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("value"))

	if err := Serialize(g, io.Discard); err != nil {
		t.Fatal("unexpected error with Discard writer:", err)
	}

	// Use an always-failing writer to hit the Fprintln error return.
	if err := Serialize(g, &failWriter{}); err == nil {
		t.Error("expected write error, got nil")
	}
}

type failWriter struct{}

func (f *failWriter) Write(p []byte) (int, error) {
	return 0, io.ErrClosedPipe
}

// TestSerializeBNodeObject exercises a BNode used as an object term.
func TestSerializeBNodeObject(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	bn := rdflibgo.NewBNode("obj1")
	g.Add(s, p, bn)

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "_:obj1") {
		t.Errorf("expected _:obj1 in output, got:\n%s", out)
	}
}

// TestParseWithBaseOption passes WithBase to Parse to cover option handling.
func TestParseWithBaseOption(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello" .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input), WithBase("http://base.example/")); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestSerializeLiteralDirLang exercises the directional lang tag branch.
func TestSerializeLiteralDirLang(t *testing.T) {
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
		t.Errorf("expected directional lang tag, got:\n%s", out)
	}
}

// TestSerializeMultipleTriplesOrder verifies deterministic sorted output.
func TestSerializeMultipleTriplesOrder(t *testing.T) {
	g := rdflibgo.NewGraph()
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/z"), p, rdflibgo.NewLiteral("z"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/a"), p, rdflibgo.NewLiteral("a"))

	var buf1, buf2 bytes.Buffer
	if err := Serialize(g, &buf1); err != nil {
		t.Fatal(err)
	}
	if err := Serialize(g, &buf2); err != nil {
		t.Fatal(err)
	}
	if buf1.String() != buf2.String() {
		t.Error("N-Triples output not deterministic")
	}
	// Verify sorted: "a" line should come before "z" line
	lines := strings.Split(strings.TrimSpace(buf1.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] > lines[1] {
		t.Error("lines not sorted")
	}
}

// TestSerializeTripleTermNestedInSubject exercises TripleTerm containing a BNode.
func TestSerializeTripleTermNestedInSubject(t *testing.T) {
	bn := rdflibgo.NewBNode("inner")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	tt := rdflibgo.NewTripleTerm(bn, p, o)

	g := rdflibgo.NewGraph()
	outerP := rdflibgo.NewURIRefUnsafe("http://example.org/asserts")
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	g.Add(s, outerP, tt)

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "_:inner") {
		t.Errorf("expected _:inner in triple term, got:\n%s", out)
	}
}

// TestSerializeTripleTermLiteralObject exercises TripleTerm with literal object.
func TestSerializeTripleTermLiteralObject(t *testing.T) {
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewLiteral("hello", rdflibgo.WithLang("en"))
	tt := rdflibgo.NewTripleTerm(s, p, o)

	g := rdflibgo.NewGraph()
	outerP := rdflibgo.NewURIRefUnsafe("http://example.org/asserts")
	g.Add(s, outerP, tt)

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `"hello"@en`) {
		t.Errorf("expected lang literal in triple term, got:\n%s", out)
	}
}

// TestSerializeIRIWithSpecialChars exercises EscapeIRI with control chars and supplementary.
func TestSerializeIRIWithSpecialChars(t *testing.T) {
	g := rdflibgo.NewGraph()
	// IRI with supplementary plane character
	s := rdflibgo.NewURIRefUnsafe("http://example.org/\U0001F600")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `\U`) {
		t.Errorf("expected \\U escape in IRI, got:\n%s", out)
	}
}

// TestSerializeIRIWithControlChar exercises EscapeIRI with control character.
func TestSerializeIRIWithControlChar(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/a\x01b")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `\u0001`) {
		t.Errorf("expected \\u0001 in IRI, got:\n%s", out)
	}
}

// TestSerializeIRIWithAngleBrackets exercises EscapeIRI with < and > chars.
func TestSerializeIRIWithAngleBrackets(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/a<b>c")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	// Should escape < and > in IRI
	out := buf.String()
	if !strings.Contains(out, `\u003C`) && !strings.Contains(out, `\u003c`) {
		t.Errorf("expected escaped angle brackets in IRI, got:\n%s", out)
	}
}

// TestSerializeWriteErrorMultipleTriples covers the Fprintln error path with multiple triples.
func TestSerializeWriteErrorMultipleTriples(t *testing.T) {
	g := rdflibgo.NewGraph()
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	for i := 0; i < 10; i++ {
		s := rdflibgo.NewURIRefUnsafe("http://example.org/s" + string(rune('0'+i)))
		g.Add(s, p, rdflibgo.NewLiteral("v"))
	}
	err := Serialize(g, &failWriter{})
	if err == nil {
		t.Error("expected write error, got nil")
	}
}

// TestSerializeCarriageReturn exercises the \r escape in strings.
func TestSerializeCarriageReturn(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("a\rb"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `\r`) {
		t.Error("expected \\r in output")
	}
}
