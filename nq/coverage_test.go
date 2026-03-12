package nq

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

// TestSerializeDirLangLiteral exercises the directional language tag serialization.
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

// TestSerializeTripleTermNamedGraph exercises TripleTerm in a named graph.
func TestSerializeTripleTermNamedGraph(t *testing.T) {
	graphIRI := rdflibgo.NewURIRefUnsafe("http://example.org/g")
	g := rdflibgo.NewGraph(rdflibgo.WithIdentifier(graphIRI))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	o := rdflibgo.NewURIRefUnsafe("http://example.org/o")
	tt := rdflibgo.NewTripleTerm(s, p, o)
	asserts := rdflibgo.NewURIRefUnsafe("http://example.org/asserts")
	g.Add(s, asserts, tt)

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<<(") {
		t.Errorf("expected triple term in named graph output, got:\n%s", out)
	}
	if !strings.Contains(out, "<http://example.org/g>") {
		t.Errorf("expected graph IRI, got:\n%s", out)
	}
}

// TestSerializeEscapedString exercises string escaping in NQ.
func TestSerializeEscapedString(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("line\n\ttab\\quote\""))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `\n`) || !strings.Contains(out, `\t`) {
		t.Errorf("expected escaped chars, got:\n%s", out)
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

// TestParseGraphLabelBNode covers blank node graph labels.
func TestParseGraphLabelBNode(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello" _:g1 .` + "\n"
	g := rdflibgo.NewGraph()
	var gotGraph rdflibgo.Term
	handler := func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) {
		gotGraph = graph
	}
	if err := Parse(g, strings.NewReader(input), WithQuadHandler(handler)); err != nil {
		t.Fatal(err)
	}
	if gotGraph == nil {
		t.Error("expected non-nil graph term for bnode graph label")
	}
}

// TestParseInvalidGraphLabel covers error for invalid graph label.
func TestParseInvalidGraphLabel(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello" "badgraph" .` + "\n"
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for invalid graph label")
	}
}

// TestParseMalformedInput covers various malformed NQ inputs.
func TestParseMalformedInput(t *testing.T) {
	inputs := []string{
		"not valid nquads\n",
		`<http://example.org/s> <http://example.org/p>` + "\n",                      // missing object
		`<http://example.org/s> <http://example.org/p> "hello"` + "\n",              // missing dot
		`<relative> <http://example.org/p> "hello" .` + "\n",                         // relative subject
		`<http://example.org/s> <http://example.org/p> "hello\uD800world" .` + "\n", // surrogate
	}
	for _, input := range inputs {
		g := rdflibgo.NewGraph()
		err := Parse(g, strings.NewReader(input))
		if err == nil {
			t.Errorf("expected parse error for %q, got nil", input)
		}
	}
}

// TestParseRelativeGraphIRI covers relative IRI rejection in graph position.
func TestParseRelativeGraphIRI(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello" <relative> .` + "\n"
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Error("expected error for relative IRI in graph position")
	}
}

// TestParseTripleTerm covers triple term in NQ.
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

// TestParseDirLangTag covers parsing of directional language tags in NQ.
func TestParseDirLangTag(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello"@en--ltr .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

// TestSerializeSupplementaryPlaneChar exercises the \U escape.
func TestSerializeSupplementaryPlaneChar(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("\U0001F600"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, `\U`) {
		t.Errorf("expected \\U escape, got:\n%s", out)
	}
}

// TestSerializeNamedGraphWithTripleTermAndLiteral covers serialize with named graph and mixed terms.
func TestSerializeNamedGraphWithTripleTermAndLiteral(t *testing.T) {
	graphIRI := rdflibgo.NewURIRefUnsafe("http://example.org/g")
	g := rdflibgo.NewGraph(rdflibgo.WithIdentifier(graphIRI))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("hello"))
	g.Add(s, p, rdflibgo.NewLiteral("42", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))
	g.Add(s, p, rdflibgo.NewLiteral("hello", rdflibgo.WithLang("en")))
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<http://example.org/g>") {
		t.Error("expected graph IRI")
	}
}

// TestParseTypedLiteral covers typed literal in NQ.
func TestParseTypedLiteral(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "42"^^<http://www.w3.org/2001/XMLSchema#integer> .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestParseBNodeSubjectAndObject covers bnodes in both positions.
func TestParseBNodeSubjectAndObject(t *testing.T) {
	input := `_:b1 <http://example.org/p> _:b2 .` + "\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestParseWithGraphIRI covers parsing NQ with a named graph.
func TestParseWithGraphIRI(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello" <http://example.org/g> .` + "\n"
	g := rdflibgo.NewGraph()
	var gotGraph rdflibgo.Term
	handler := func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) {
		gotGraph = graph
	}
	if err := Parse(g, strings.NewReader(input), WithQuadHandler(handler)); err != nil {
		t.Fatal(err)
	}
	if gotGraph == nil {
		t.Error("expected graph IRI")
	}
}

// TestParseCommentAndBlankLines covers comment/blank line handling.
func TestParseCommentAndBlankLines(t *testing.T) {
	input := "# comment\n\n<http://example.org/s> <http://example.org/p> \"hello\" .\n"
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
}

// TestSerializeMultipleTriples covers sorted output of multiple triples.
func TestSerializeMultipleTriples(t *testing.T) {
	g := rdflibgo.NewGraph()
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/b"), p, rdflibgo.NewLiteral("v1"))
	g.Add(rdflibgo.NewURIRefUnsafe("http://example.org/a"), p, rdflibgo.NewLiteral("v2"))
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Errorf("expected 2 lines, got %d", len(lines))
	}
}

// TestSerializeBNodeGraph covers graph identifier that is a BNode (should be ignored).
func TestSerializeBNodeGraph(t *testing.T) {
	bn := rdflibgo.NewBNode("g1")
	g := rdflibgo.NewGraph(rdflibgo.WithIdentifier(bn))
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("v"))
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	// BNode context should not produce a graph term suffix
	if strings.Contains(buf.String(), "_:g1") {
		t.Error("did not expect bnode graph in NQ output")
	}
}
