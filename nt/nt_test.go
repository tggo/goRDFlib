package nt

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/testutil"
)

// Ported from: test/test_w3c_spec/test_nt_w3c.py, test/test_nt_misc.py

func TestNTSerializerBasic(t *testing.T) {
	// Ported from: rdflib.plugins.serializers.nt.NTSerializer
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("hello"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "<http://example.org/s>") {
		t.Errorf("expected full IRI, got:\n%s", out)
	}
	if !strings.Contains(out, `"hello"`) {
		t.Errorf("expected literal, got:\n%s", out)
	}
	if !strings.HasSuffix(strings.TrimSpace(out), ".") {
		t.Errorf("expected trailing dot, got:\n%s", out)
	}
}

func TestNTSerializerEscaping(t *testing.T) {
	// Ported from: rdflib N-Triples escape handling
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("line1\nline2"))

	var buf bytes.Buffer
	Serialize(g, &buf)
	if !strings.Contains(buf.String(), `\n`) {
		t.Errorf("expected escaped newline, got:\n%s", buf.String())
	}
}

func TestNTSerializerLangAndDatatype(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("hello", rdflibgo.WithLang("en")))
	g.Add(s, p, rdflibgo.NewLiteral("42", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))

	var buf bytes.Buffer
	Serialize(g, &buf)
	out := buf.String()
	if !strings.Contains(out, `"hello"@en`) {
		t.Errorf("expected lang tag, got:\n%s", out)
	}
	if !strings.Contains(out, `"42"^^<http://www.w3.org/2001/XMLSchema#integer>`) {
		t.Errorf("expected datatype, got:\n%s", out)
	}
}

func TestNTSerializerDeterministic(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	for i := 0; i < 5; i++ {
		p, _ := rdflibgo.NewURIRef(fmt.Sprintf("http://example.org/p%d", i))
		g.Add(s, p, rdflibgo.NewLiteral(fmt.Sprintf("v%d", i)))
	}
	var buf1, buf2 bytes.Buffer
	Serialize(g, &buf1)
	Serialize(g, &buf2)
	if buf1.String() != buf2.String() {
		t.Error("N-Triples output not deterministic")
	}
}

func TestNTParserBasic(t *testing.T) {
	// Ported from: rdflib.plugins.parsers.ntriples.NTriplesParser
	input := `<http://example.org/s> <http://example.org/p> "hello" .
<http://example.org/s> <http://example.org/p2> <http://example.org/o> .
`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
}

func TestNTParserBNode(t *testing.T) {
	input := `_:b1 <http://example.org/p> "hello" .
`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestNTParserLangTag(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello"@en .
`
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(input))
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	val, ok := g.Value(s, &p, nil)
	if !ok {
		t.Fatal("expected value")
	}
	if lit, ok := val.(rdflibgo.Literal); !ok || lit.Language() != "en" {
		t.Errorf("expected lang en, got %v", val)
	}
}

func TestNTParserDatatype(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "42"^^<http://www.w3.org/2001/XMLSchema#integer> .
`
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(input))
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	val, ok := g.Value(s, &p, nil)
	if !ok {
		t.Fatal("expected value")
	}
	if lit, ok := val.(rdflibgo.Literal); !ok || lit.Datatype() != rdflibgo.XSDInteger {
		t.Errorf("expected xsd:integer, got %v", val)
	}
}

func TestNTParserComments(t *testing.T) {
	input := `# comment
<http://example.org/s> <http://example.org/p> "hello" .

# another comment
`
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(input))
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestNTParserEscape(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "line1\nline2" .
`
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(input))
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	val, _ := g.Value(s, &p, nil)
	if val.String() != "line1\nline2" {
		t.Errorf("expected newline in value, got %q", val.String())
	}
}

func TestNTRoundtrip(t *testing.T) {
	// Ported from: test/test_roundtrip.py — N-Triples roundtrip
	g1 := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g1.Add(s, p, rdflibgo.NewLiteral("hello"))
	g1.Add(s, p, rdflibgo.NewLiteral("world", rdflibgo.WithLang("en")))
	g1.Add(s, p, rdflibgo.NewLiteral("42", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))

	var buf bytes.Buffer
	Serialize(g1, &buf)

	g2 := rdflibgo.NewGraph()
	Parse(g2, strings.NewReader(buf.String()))

	if g1.Len() != g2.Len() {
		t.Errorf("roundtrip: %d vs %d", g1.Len(), g2.Len())
	}
}

// --- N-Triples parser edge cases ---

func TestNTParserEscapeTab(t *testing.T) {
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(`<http://s> <http://p> "a\tb" .`+"\n"))
	s := rdflibgo.NewURIRefUnsafe("http://s")
	p := rdflibgo.NewURIRefUnsafe("http://p")
	v, _ := g.Value(s, &p, nil)
	if v.String() != "a\tb" {
		t.Errorf("got %q", v.String())
	}
}

func TestNTParserEscapeQuote(t *testing.T) {
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(`<http://s> <http://p> "say \"hi\"" .`+"\n"))
	s := rdflibgo.NewURIRefUnsafe("http://s")
	p := rdflibgo.NewURIRefUnsafe("http://p")
	v, _ := g.Value(s, &p, nil)
	if v.String() != `say "hi"` {
		t.Errorf("got %q", v.String())
	}
}

func TestNTParserEscapeBackslash(t *testing.T) {
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(`<http://s> <http://p> "a\\b" .`+"\n"))
	s := rdflibgo.NewURIRefUnsafe("http://s")
	p := rdflibgo.NewURIRefUnsafe("http://p")
	v, _ := g.Value(s, &p, nil)
	if v.String() != `a\b` {
		t.Errorf("got %q", v.String())
	}
}

func TestNTParserEscapeCarriageReturn(t *testing.T) {
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(`<http://s> <http://p> "a\rb" .`+"\n"))
	s := rdflibgo.NewURIRefUnsafe("http://s")
	p := rdflibgo.NewURIRefUnsafe("http://p")
	v, _ := g.Value(s, &p, nil)
	if v.String() != "a\rb" {
		t.Errorf("got %q", v.String())
	}
}

func TestNTParserUnicodeEscape8(t *testing.T) {
	g := rdflibgo.NewGraph()
	Parse(g, strings.NewReader(`<http://s> <http://p> "\U00000042" .`+"\n"))
	s := rdflibgo.NewURIRefUnsafe("http://s")
	p := rdflibgo.NewURIRefUnsafe("http://p")
	v, _ := g.Value(s, &p, nil)
	if v.String() != "B" {
		t.Errorf("expected B, got %q", v.String())
	}
}

// --- Serializer edge cases ---

func TestNTSerializerTab(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://s")
	p, _ := rdflibgo.NewURIRef("http://p")
	g.Add(s, p, rdflibgo.NewLiteral("a\tb"))
	var buf strings.Builder
	Serialize(g, &buf)
	out := buf.String()
	if !strings.Contains(out, `\t`) {
		t.Errorf("expected tab escape, got:\n%s", out)
	}
}

// --- Negative syntax tests ---

func TestNTParserMalformedEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unknown escape", `<http://s> <http://p> "hello\x" .` + "\n"},
		{"truncated \\u", `<http://s> <http://p> "\u00" .` + "\n"},
		{"truncated \\U", `<http://s> <http://p> "\U0000" .` + "\n"},
		{"invalid hex \\u", `<http://s> <http://p> "\uZZZZ" .` + "\n"},
		{"unterminated string", `<http://s> <http://p> "hello .` + "\n"},
		{"unterminated IRI", `<http://s <http://p> "hello" .` + "\n"},
		{"missing dot", `<http://s> <http://p> "hello"` + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := rdflibgo.NewGraph()
			err := Parse(g, strings.NewReader(tt.input))
			if err == nil {
				t.Error("expected error for malformed input")
			}
		})
	}
}

func TestNTParserUnicodeEscape4(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`<http://s> <http://p> "\u0041" .`+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	s := rdflibgo.NewURIRefUnsafe("http://s")
	p := rdflibgo.NewURIRefUnsafe("http://p")
	v, _ := g.Value(s, &p, nil)
	if v.String() != "A" {
		t.Errorf("expected A, got %q", v.String())
	}
}

func TestNTParserIRIWithMalformedEscape(t *testing.T) {
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(`<http://s/\uZZZZ> <http://p> "hello" .`+"\n"))
	if err == nil {
		t.Error("expected error for malformed IRI escape")
	}
}

func TestNTRoundtripWithAssertGraphEqual(t *testing.T) {
	g1 := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g1.Add(s, p, rdflibgo.NewLiteral("hello"))
	g1.Add(s, p, rdflibgo.NewLiteral("world", rdflibgo.WithLang("en")))
	g1.Add(s, p, rdflibgo.NewLiteral("42", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))

	var buf bytes.Buffer
	if err := Serialize(g1, &buf); err != nil {
		t.Fatal(err)
	}

	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(buf.String())); err != nil {
		t.Fatal(err)
	}

	testutil.AssertGraphEqual(t, g1, g2)
}

func TestNTSerializerDatatypeIRIEscaped(t *testing.T) {
	g := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://s")
	p, _ := rdflibgo.NewURIRef("http://p")
	g.Add(s, p, rdflibgo.NewLiteral("42", rdflibgo.WithDatatype(rdflibgo.XSDInteger)))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "^^<http://www.w3.org/2001/XMLSchema#integer>") {
		t.Errorf("expected escaped datatype IRI, got:\n%s", out)
	}
}

func TestWithErrorHandler(t *testing.T) {
	opt := WithErrorHandler(func(lineNum int, line string, err error) (string, bool) {
		return "", false
	})
	var cfg config
	opt(&cfg)
	if cfg.errorHandler == nil {
		t.Error("WithErrorHandler: handler not set")
	}
}

func TestNTParserErrorHandlerSkip(t *testing.T) {
	input := `<http://example.org/s1> <http://example.org/p> "good" .
<http://example.org/s 2> <http://example.org/p> "bad iri" .
<http://example.org/s3> <http://example.org/p> "also good" .
`
	g := rdflibgo.NewGraph()
	var skipped []int
	err := Parse(g, strings.NewReader(input), WithErrorHandler(
		func(lineNum int, line string, err error) (string, bool) {
			skipped = append(skipped, lineNum)
			return "", false
		},
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", g.Len())
	}
	if len(skipped) != 1 || skipped[0] != 2 {
		t.Errorf("expected skipped=[2], got %v", skipped)
	}
}

func TestNTParserErrorHandlerRetry(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> <http://example.org/o with space> .
`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input), WithErrorHandler(
		func(lineNum int, line string, err error) (string, bool) {
			fixed := strings.ReplaceAll(line, "o with space", "o%20with%20space")
			return fixed, true
		},
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1 triple, got %d", g.Len())
	}
}

func TestNTParserNoErrorHandler(t *testing.T) {
	input := `<http://example.org/s 2> <http://example.org/p> "bad" .
`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNTParserErrorHandlerRetryFails(t *testing.T) {
	input := `<http://example.org/s 2> <http://example.org/p> "bad" .
`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input), WithErrorHandler(
		func(lineNum int, line string, err error) (string, bool) {
			return line, true // return same broken line
		},
	))
	if err == nil {
		t.Fatal("expected error on failed retry, got nil")
	}
	if !strings.Contains(err.Error(), "retry failed") {
		t.Errorf("expected 'retry failed' in error, got: %v", err)
	}
}

func TestNTParserErrorHandlerMultipleErrors(t *testing.T) {
	input := `<http://example.org/s1> <http://example.org/p> "good" .
<bad 1> <http://example.org/p> "x" .
<http://example.org/s2> <http://example.org/p> "good2" .
<bad 2> <http://example.org/p> "y" .
<http://example.org/s3> <http://example.org/p> "good3" .
`
	g := rdflibgo.NewGraph()
	var skippedLines []int
	err := Parse(g, strings.NewReader(input), WithErrorHandler(
		func(lineNum int, line string, err error) (string, bool) {
			skippedLines = append(skippedLines, lineNum)
			return "", false
		},
	))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.Len() != 3 {
		t.Errorf("expected 3 triples, got %d", g.Len())
	}
	if len(skippedLines) != 2 || skippedLines[0] != 2 || skippedLines[1] != 4 {
		t.Errorf("expected skipped=[2,4], got %v", skippedLines)
	}
}

func TestNTParserErrorHandlerReceivesLineText(t *testing.T) {
	badLine := `<http://example.org/s 2> <http://example.org/p> "bad" .`
	input := badLine + "\n"
	g := rdflibgo.NewGraph()
	var receivedLine string
	Parse(g, strings.NewReader(input), WithErrorHandler(
		func(lineNum int, line string, err error) (string, bool) {
			receivedLine = line
			return "", false
		},
	))
	if receivedLine != badLine {
		t.Errorf("handler received wrong line text: %q", receivedLine)
	}
}

// TestNTParserLongLineDefaultErrors verifies that a line larger than bufio.Scanner's
// default 64KB max token size produces an actionable error under default settings:
// it matches the ErrLineTooLong sentinel, names the failing line number, and points
// at the options that lift the cap — instead of the opaque bufio "token too long".
func TestNTParserLongLineDefaultErrors(t *testing.T) {
	bigLiteral := strings.Repeat("x", 100*1024)
	line := `<http://example.org/s> <http://example.org/p> "` + bigLiteral + `" .` + "\n"

	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(line))
	if err == nil {
		t.Fatal("expected error parsing line > 64KB with default buffer, got nil")
	}
	if !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("expected errors.Is(err, ErrLineTooLong), got: %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "line 1") {
		t.Errorf("expected error to name the failing line number (line 1), got: %v", err)
	}
	if !strings.Contains(msg, "WithUnboundedLines") || !strings.Contains(msg, "WithMaxLineLength") {
		t.Errorf("expected error to suggest the size options, got: %v", err)
	}
}

// TestNTParserWithMaxLineLength verifies that WithMaxLineLength raises the scanner
// buffer so long literals parse successfully.
func TestNTParserWithMaxLineLength(t *testing.T) {
	bigLiteral := strings.Repeat("x", 100*1024)
	line := `<http://example.org/s> <http://example.org/p> "` + bigLiteral + `" .` + "\n"

	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(line), WithMaxLineLength(1<<20)); err != nil {
		t.Fatalf("unexpected error with WithMaxLineLength(1MB): %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("expected 1 triple, got %d", g.Len())
	}
}

// TestNTParserWithUnboundedLines verifies that WithUnboundedLines parses a line
// far larger than the 64KB scanner default without the caller having to know or
// set an upper bound via WithMaxLineLength.
func TestNTParserWithUnboundedLines(t *testing.T) {
	bigLiteral := strings.Repeat("x", 5*1024*1024) // 5MB, well past any fixed buffer
	line := `<http://example.org/s> <http://example.org/p> "` + bigLiteral + `" .` + "\n"

	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(line), WithUnboundedLines()); err != nil {
		t.Fatalf("unexpected error with WithUnboundedLines: %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("expected 1 triple, got %d", g.Len())
	}
}

// TestNTParserUnboundedLinesMultiple verifies the unbounded reader still splits
// on newlines correctly across many lines, including a final line without a
// trailing newline.
func TestNTParserUnboundedLinesMultiple(t *testing.T) {
	input := `<http://example.org/s1> <http://example.org/p> "a" .
# comment
<http://example.org/s2> <http://example.org/p> "b" .

<http://example.org/s3> <http://example.org/p> "c" .` // no trailing newline

	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input), WithUnboundedLines()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.Len() != 3 {
		t.Fatalf("expected 3 triples, got %d", g.Len())
	}
}

// TestNTParseStreamWithUnboundedLines verifies ParseStream honors the option too.
func TestNTParseStreamWithUnboundedLines(t *testing.T) {
	bigLiteral := strings.Repeat("y", 2*1024*1024)
	line := `<http://example.org/s> <http://example.org/p> "` + bigLiteral + `" .` + "\n"

	count := 0
	err := ParseStream(strings.NewReader(line), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error {
		count++
		return nil
	}, WithUnboundedLines())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 triple dispatched, got %d", count)
	}
}

// BenchmarkNTParseScanner and BenchmarkNTParseUnbounded compare the two line
// sources on identical input to confirm the default scanner path is not
// regressed by adding the unbounded option.
func benchNTInput() string {
	var b strings.Builder
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&b, "<http://example.org/s%d> <http://example.org/p> \"value number %d here\" .\n", i, i)
	}
	return b.String()
}

func BenchmarkNTParseScanner(b *testing.B) {
	input := benchNTInput()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g := rdflibgo.NewGraph()
		if err := Parse(g, strings.NewReader(input)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNTParseUnbounded(b *testing.B) {
	input := benchNTInput()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g := rdflibgo.NewGraph()
		if err := Parse(g, strings.NewReader(input), WithUnboundedLines()); err != nil {
			b.Fatal(err)
		}
	}
}

// TestNTParseStreamBasic verifies that ParseStream dispatches every triple to
// the handler with the correct subject, predicate, and object — and does not
// require (nor populate) a graph.
func TestNTParseStreamBasic(t *testing.T) {
	input := `<http://example.org/s1> <http://example.org/p> "a" .
<http://example.org/s2> <http://example.org/p> "b" .
`
	type row struct {
		s, p, o string
	}
	var got []row
	err := ParseStream(strings.NewReader(input), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error {
		got = append(got, row{s: s.N3(), p: p.N3(), o: o.N3()})
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []row{
		{s: "<http://example.org/s1>", p: "<http://example.org/p>", o: `"a"`},
		{s: "<http://example.org/s2>", p: "<http://example.org/p>", o: `"b"`},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d triples, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("triple %d: want %+v, got %+v", i, want[i], got[i])
		}
	}
}

// TestNTParseStreamHandlerError verifies that an error returned from the handler
// aborts parsing and propagates back to the caller.
func TestNTParseStreamHandlerError(t *testing.T) {
	input := `<http://example.org/s1> <http://example.org/p> "a" .
<http://example.org/s2> <http://example.org/p> "b" .
<http://example.org/s3> <http://example.org/p> "c" .
`
	sentinel := fmt.Errorf("sentinel")
	count := 0
	err := ParseStream(strings.NewReader(input), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error {
		count++
		if count == 2 {
			return sentinel
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected handler error to propagate, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected sentinel error, got: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected handler called exactly 2 times before abort, got %d", count)
	}
}

// TestNTParseStreamNilHandler verifies that ParseStream rejects a nil handler
// rather than panicking.
func TestNTParseStreamNilHandler(t *testing.T) {
	err := ParseStream(strings.NewReader(`<http://s> <http://p> "o" .`), nil)
	if err == nil {
		t.Fatal("expected error for nil handler, got nil")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("expected error mentioning nil, got: %v", err)
	}
}

// TestNTParseStreamWithMaxLineLength verifies that ParseStream honors the same
// WithMaxLineLength option as Parse.
func TestNTParseStreamWithMaxLineLength(t *testing.T) {
	bigLiteral := strings.Repeat("x", 100*1024)
	line := `<http://example.org/s> <http://example.org/p> "` + bigLiteral + `" .` + "\n"

	count := 0
	err := ParseStream(strings.NewReader(line), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error {
		count++
		return nil
	}, WithMaxLineLength(1<<20))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 triple, got %d", count)
	}
}

// TestNTParseStreamSkipsCommentsAndBlanks verifies the scanner loop honors
// comment and blank-line rules in streaming mode.
func TestNTParseStreamSkipsCommentsAndBlanks(t *testing.T) {
	input := `# a comment

<http://example.org/s> <http://example.org/p> "a" .
   # indented comment

<http://example.org/s> <http://example.org/p> "b" .
`
	count := 0
	err := ParseStream(strings.NewReader(input), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 triples, got %d", count)
	}
}
