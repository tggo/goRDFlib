package nq

import (
	"bytes"
	"io"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// TestWithBase ensures WithBase constructs an Option that sets the base field.
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

// TestSerializeNamedGraph exercises the named-graph code path in the NQ serializer
// where g.Identifier() is a URIRef and the graph IRI is appended to every line.
func TestSerializeNamedGraph(t *testing.T) {
	graphIRI := rdflibgo.NewURIRefUnsafe("http://example.org/graph1")
	g := rdflibgo.NewGraph(rdflibgo.WithIdentifier(graphIRI))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("hello"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<http://example.org/graph1>") {
		t.Errorf("expected named graph IRI in output, got:\n%s", out)
	}
}

// TestSerializeAnonymousGraph confirms that a graph with a BNode identifier
// does NOT emit a graph suffix (only URIRef identifiers are emitted).
func TestSerializeAnonymousGraph(t *testing.T) {
	g := rdflibgo.NewGraph() // identifier is a BNode by default
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("world"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// The triple line must end with " ." (no graph IRI between object and dot).
	if strings.Count(out, "<") > 2 {
		t.Errorf("anonymous graph should not emit graph IRI, got:\n%s", out)
	}
}

// TestSerializeWriteError covers the error path when the writer fails.
func TestSerializeWriteError(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("value"))

	// Discard writer should succeed.
	if err := Serialize(g, io.Discard); err != nil {
		t.Fatal("unexpected error with Discard writer:", err)
	}

	// Always-failing writer hits the Fprintln error return.
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

// TestSerializeLiteralPlain ensures a plain literal serialises without annotations.
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

// TestSerializeTripleTermAsSubject exercises TripleTerm used as graph subject.
func TestSerializeTripleTermAsSubject(t *testing.T) {
	innerS := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	innerP := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	innerO := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	tt := rdflibgo.NewTripleTerm(innerS, innerP, innerO)

	g := rdflibgo.NewGraph()
	outerP := rdflibgo.NewURIRefUnsafe("http://example.org/occursIn")
	outerO := rdflibgo.NewURIRefUnsafe("http://example.org/doc")
	g.Add(tt, outerP, outerO)

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<<(") {
		t.Errorf("expected triple term as subject, got:\n%s", out)
	}
}

// TestSerializeDirLangLiteralNQ exercises directional lang tags in NQ.
func TestSerializeDirLangLiteralNQ(t *testing.T) {
	graphIRI := rdflibgo.NewURIRefUnsafe("http://example.org/g")
	g := rdflibgo.NewGraph(rdflibgo.WithIdentifier(graphIRI))
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
	if !strings.Contains(out, "<http://example.org/g>") {
		t.Errorf("expected graph IRI, got:\n%s", out)
	}
}

// TestSerializeIRIWithEscapes exercises IRI escaping in NQ.
func TestSerializeIRIWithEscapes(t *testing.T) {
	g := rdflibgo.NewGraph()
	// IRI with supplementary plane character
	s := rdflibgo.NewURIRefUnsafe("http://example.org/\U0001F600")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `\U`) {
		t.Errorf("expected \\U escape in IRI, got:\n%s", buf.String())
	}
}

// TestSerializeWriteErrorMultiple covers writer failure with multiple triples.
func TestSerializeWriteErrorMultiple(t *testing.T) {
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

// TestSerializeTripleTermWithBNodeObj exercises TripleTerm containing BNode object.
func TestSerializeTripleTermWithBNodeObj(t *testing.T) {
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	bn := rdflibgo.NewBNode("inner")
	tt := rdflibgo.NewTripleTerm(s, p, bn)

	g := rdflibgo.NewGraph()
	outerP := rdflibgo.NewURIRefUnsafe("http://example.org/asserts")
	g.Add(s, outerP, tt)

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "_:inner") {
		t.Errorf("expected _:inner in triple term, got:\n%s", buf.String())
	}
}

// TestSerializeMultipleTriplesNamedGraph verifies deterministic sorted output
// for named graphs.
func TestSerializeMultipleTriplesNamedGraph(t *testing.T) {
	graphIRI := rdflibgo.NewURIRefUnsafe("http://example.org/g")
	g := rdflibgo.NewGraph(rdflibgo.WithIdentifier(graphIRI))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p1 := rdflibgo.NewURIRefUnsafe("http://example.org/p1")
	p2 := rdflibgo.NewURIRefUnsafe("http://example.org/p2")
	g.Add(s, p1, rdflibgo.NewLiteral("alpha"))
	g.Add(s, p2, rdflibgo.NewLiteral("beta"))

	var buf1, buf2 bytes.Buffer
	if err := Serialize(g, &buf1); err != nil {
		t.Fatal(err)
	}
	if err := Serialize(g, &buf2); err != nil {
		t.Fatal(err)
	}
	if buf1.String() != buf2.String() {
		t.Error("N-Quads output not deterministic for named graph")
	}
	out := buf1.String()
	if strings.Count(out, "<http://example.org/g>") < 2 {
		t.Errorf("expected graph IRI on every line, got:\n%s", out)
	}
}
