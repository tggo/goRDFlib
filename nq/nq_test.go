package nq

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/testutil"
)

// Ported from: test/test_w3c_spec/test_nquads_w3c.py, test/test_parsers/test_nquads.py

func TestNQSerializerBasic(t *testing.T) {
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
		t.Errorf("expected IRI, got:\n%s", out)
	}
}

func TestNQParserBasic(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello" <http://example.org/g> .
<http://example.org/s> <http://example.org/p2> "world" .
`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 2 {
		t.Errorf("expected 2, got %d", g.Len())
	}
}

func TestNQParserComments(t *testing.T) {
	input := `# comment
<http://example.org/s> <http://example.org/p> "hello" .
`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	if g.Len() != 1 {
		t.Errorf("expected 1, got %d", g.Len())
	}
}

func TestNQRoundtrip(t *testing.T) {
	g1 := rdflibgo.NewGraph()
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	g1.Add(s, p, rdflibgo.NewLiteral("hello"))

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

// --- Graph context tests ---

func TestNQParserGraphContextPreserved(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello" <http://example.org/g1> .
<http://example.org/s> <http://example.org/p2> "world" <http://example.org/g2> .
<http://example.org/s> <http://example.org/p3> "no graph" .
`
	g := rdflibgo.NewGraph()
	var quads []struct {
		graph rdflibgo.Term
	}
	err := Parse(g, strings.NewReader(input), WithQuadHandler(func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) {
		quads = append(quads, struct{ graph rdflibgo.Term }{graph})
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(quads) != 3 {
		t.Fatalf("expected 3 quads, got %d", len(quads))
	}
	// First quad has graph g1
	if u, ok := quads[0].graph.(rdflibgo.URIRef); !ok || u.Value() != "http://example.org/g1" {
		t.Errorf("quad 0: expected g1, got %v", quads[0].graph)
	}
	// Second quad has graph g2
	if u, ok := quads[1].graph.(rdflibgo.URIRef); !ok || u.Value() != "http://example.org/g2" {
		t.Errorf("quad 1: expected g2, got %v", quads[1].graph)
	}
	// Third quad has no graph
	if quads[2].graph != nil {
		t.Errorf("quad 2: expected nil graph, got %v", quads[2].graph)
	}
}

func TestNQParserBNodeGraphContext(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "hello" _:g1 .
`
	g := rdflibgo.NewGraph()
	var graphCtx rdflibgo.Term
	err := Parse(g, strings.NewReader(input), WithQuadHandler(func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) {
		graphCtx = graph
	}))
	if err != nil {
		t.Fatal(err)
	}
	if b, ok := graphCtx.(rdflibgo.BNode); !ok || b.Value() != "g1" {
		t.Errorf("expected BNode g1, got %v", graphCtx)
	}
}

// --- Negative syntax tests ---

func TestNQParserMalformed(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"unterminated IRI", `<http://s <http://p> "hello" .` + "\n"},
		{"missing dot", `<http://s> <http://p> "hello"` + "\n"},
		{"bad escape in literal", `<http://s> <http://p> "\uZZZZ" .` + "\n"},
		{"unterminated string", `<http://s> <http://p> "hello .` + "\n"},
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

// --- Escape handling tests ---

func TestNQParserEscapes(t *testing.T) {
	input := `<http://s> <http://p> "a\tb\nc\\d\"e" .
`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s := rdflibgo.NewURIRefUnsafe("http://s")
	p := rdflibgo.NewURIRefUnsafe("http://p")
	v, ok := g.Value(s, &p, nil)
	if !ok {
		t.Fatal("expected value")
	}
	want := "a\tb\nc\\d\"e"
	if v.String() != want {
		t.Errorf("got %q, want %q", v.String(), want)
	}
}

func TestNQParserLangAndDatatype(t *testing.T) {
	input := `<http://s> <http://p1> "hello"@en .
<http://s> <http://p2> "42"^^<http://www.w3.org/2001/XMLSchema#integer> .
`
	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input)); err != nil {
		t.Fatal(err)
	}
	s := rdflibgo.NewURIRefUnsafe("http://s")
	p1 := rdflibgo.NewURIRefUnsafe("http://p1")
	v1, _ := g.Value(s, &p1, nil)
	if lit, ok := v1.(rdflibgo.Literal); !ok || lit.Language() != "en" {
		t.Errorf("expected lang en, got %v", v1)
	}
	p2 := rdflibgo.NewURIRefUnsafe("http://p2")
	v2, _ := g.Value(s, &p2, nil)
	if lit, ok := v2.(rdflibgo.Literal); !ok || lit.Datatype() != rdflibgo.XSDInteger {
		t.Errorf("expected xsd:integer, got %v", v2)
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

func TestNQParserErrorHandlerSkip(t *testing.T) {
	input := `<http://example.org/s1> <http://example.org/p> "good" <http://example.org/g> .
<http://example.org/s 2> <http://example.org/p> "bad iri" <http://example.org/g> .
<http://example.org/s3> <http://example.org/p> "also good" <http://example.org/g> .
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

func TestNQParserErrorHandlerRetry(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> <http://example.org/o with space> <http://example.org/g> .
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

func TestNQParserNoErrorHandler(t *testing.T) {
	input := `<http://example.org/s 2> <http://example.org/p> "bad" .
`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNQParserErrorHandlerRetryFails(t *testing.T) {
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

func TestNQParserErrorHandlerMultipleErrors(t *testing.T) {
	input := `<http://example.org/s1> <http://example.org/p> "good" <http://example.org/g> .
<bad 1> <http://example.org/p> "x" <http://example.org/g> .
<http://example.org/s2> <http://example.org/p> "good2" <http://example.org/g> .
<bad 2> <http://example.org/p> "y" <http://example.org/g> .
<http://example.org/s3> <http://example.org/p> "good3" <http://example.org/g> .
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

func TestNQParserErrorHandlerWithQuadHandler(t *testing.T) {
	input := `<http://example.org/s> <http://example.org/p> "ok" <http://example.org/g> .
<bad iri> <http://example.org/p> "fail" <http://example.org/g> .
`
	g := rdflibgo.NewGraph()
	var graphs []string
	err := Parse(g, strings.NewReader(input),
		WithQuadHandler(func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) {
			if graph != nil {
				graphs = append(graphs, graph.(rdflibgo.URIRef).Value())
			}
		}),
		WithErrorHandler(func(lineNum int, line string, err error) (string, bool) {
			return "", false
		}),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(graphs) != 1 || graphs[0] != "http://example.org/g" {
		t.Errorf("expected 1 graph callback, got %v", graphs)
	}
}

// TestNQParserLongLineDefaultErrors verifies that a line larger than bufio.Scanner's
// default 64KB max token size produces an actionable error under default settings:
// it matches the ErrLineTooLong sentinel, names the failing line number, and points
// at the options that lift the cap — instead of the opaque bufio "token too long".
func TestNQParserLongLineDefaultErrors(t *testing.T) {
	bigLiteral := strings.Repeat("x", 100*1024)
	line := "# leading comment\n" +
		`<http://example.org/s> <http://example.org/p> "` + bigLiteral + `" <http://example.org/g> .` + "\n"

	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(line))
	if err == nil {
		t.Fatal("expected error parsing line > 64KB with default buffer, got nil")
	}
	if !errors.Is(err, ErrLineTooLong) {
		t.Fatalf("expected errors.Is(err, ErrLineTooLong), got: %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "line 2") {
		t.Errorf("expected error to name the failing line number (line 2), got: %v", err)
	}
	if !strings.Contains(msg, "WithUnboundedLines") || !strings.Contains(msg, "WithMaxLineLength") {
		t.Errorf("expected error to suggest the size options, got: %v", err)
	}
}

// TestNQParserWithMaxLineLength verifies that WithMaxLineLength raises the scanner
// buffer so long literals (e.g. WKT polygons) parse successfully.
func TestNQParserWithMaxLineLength(t *testing.T) {
	bigLiteral := strings.Repeat("x", 100*1024)
	line := `<http://example.org/s> <http://example.org/p> "` + bigLiteral + `" <http://example.org/g> .` + "\n"

	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(line), WithMaxLineLength(1<<20)); err != nil {
		t.Fatalf("unexpected error with WithMaxLineLength(1MB): %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("expected 1 triple, got %d", g.Len())
	}
}

// TestNQParseStreamBasic verifies that ParseStream dispatches every quad to the
// handler with the correct subject, predicate, object, and graph terms — and
// does not require (nor populate) a graph.
func TestNQParseStreamBasic(t *testing.T) {
	input := `<http://example.org/s1> <http://example.org/p> "a" <http://example.org/g> .
<http://example.org/s2> <http://example.org/p> "b" .
`
	type row struct {
		s, p, o, g string
	}
	var got []row
	err := ParseStream(strings.NewReader(input), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) error {
		r := row{s: s.N3(), p: p.N3(), o: o.N3()}
		if graph != nil {
			r.g = graph.N3()
		}
		got = append(got, r)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []row{
		{s: "<http://example.org/s1>", p: "<http://example.org/p>", o: `"a"`, g: "<http://example.org/g>"},
		{s: "<http://example.org/s2>", p: "<http://example.org/p>", o: `"b"`, g: ""},
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d quads, got %d (%v)", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("quad %d: want %+v, got %+v", i, want[i], got[i])
		}
	}
}

// TestNQParseStreamHandlerError verifies that an error returned from the handler
// aborts parsing and propagates back to the caller.
func TestNQParseStreamHandlerError(t *testing.T) {
	input := `<http://example.org/s1> <http://example.org/p> "a" .
<http://example.org/s2> <http://example.org/p> "b" .
<http://example.org/s3> <http://example.org/p> "c" .
`
	sentinel := fmt.Errorf("sentinel")
	count := 0
	err := ParseStream(strings.NewReader(input), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) error {
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

// TestNQParseStreamNilHandler verifies that ParseStream rejects a nil handler
// rather than panicking.
func TestNQParseStreamNilHandler(t *testing.T) {
	err := ParseStream(strings.NewReader(`<http://s> <http://p> "o" .`), nil)
	if err == nil {
		t.Fatal("expected error for nil handler, got nil")
	}
	if !strings.Contains(err.Error(), "nil") {
		t.Errorf("expected error mentioning nil, got: %v", err)
	}
}

// TestNQParseStreamWithMaxLineLength verifies that ParseStream honors the same
// WithMaxLineLength option as Parse — the streaming path is the primary
// motivator for large-line inputs (WKT polygons in gz dumps).
func TestNQParseStreamWithMaxLineLength(t *testing.T) {
	bigLiteral := strings.Repeat("x", 100*1024)
	line := `<http://example.org/s> <http://example.org/p> "` + bigLiteral + `" <http://example.org/g> .` + "\n"

	count := 0
	err := ParseStream(strings.NewReader(line), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) error {
		count++
		return nil
	}, WithMaxLineLength(1<<20))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 quad, got %d", count)
	}
}

// TestNQParserWithUnboundedLines verifies that WithUnboundedLines parses a quad
// whose literal far exceeds the 64KB scanner default, without the caller having
// to set WithMaxLineLength to a guessed upper bound.
func TestNQParserWithUnboundedLines(t *testing.T) {
	bigLiteral := strings.Repeat("x", 5*1024*1024) // 5MB
	line := `<http://example.org/s> <http://example.org/p> "` + bigLiteral + `" <http://example.org/g> .` + "\n"

	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(line), WithUnboundedLines()); err != nil {
		t.Fatalf("unexpected error with WithUnboundedLines: %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("expected 1 triple, got %d", g.Len())
	}
}

// TestNQParserUnboundedLinesMultiple verifies the unbounded reader splits on
// newlines correctly across many lines, including a final line with no trailing
// newline.
func TestNQParserUnboundedLinesMultiple(t *testing.T) {
	input := `<http://example.org/s1> <http://example.org/p> "a" <http://example.org/g> .
# comment
<http://example.org/s2> <http://example.org/p> "b" .

<http://example.org/s3> <http://example.org/p> "c" <http://example.org/g> .` // no trailing newline

	g := rdflibgo.NewGraph()
	if err := Parse(g, strings.NewReader(input), WithUnboundedLines()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if g.Len() != 3 {
		t.Fatalf("expected 3 triples, got %d", g.Len())
	}
}

// TestNQParseStreamWithUnboundedLines verifies ParseStream honors the option.
func TestNQParseStreamWithUnboundedLines(t *testing.T) {
	bigLiteral := strings.Repeat("y", 2*1024*1024)
	line := `<http://example.org/s> <http://example.org/p> "` + bigLiteral + `" <http://example.org/g> .` + "\n"

	count := 0
	err := ParseStream(strings.NewReader(line), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) error {
		count++
		return nil
	}, WithUnboundedLines())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 quad dispatched, got %d", count)
	}
}

func benchNQInput() string {
	var b strings.Builder
	for i := 0; i < 1000; i++ {
		fmt.Fprintf(&b, "<http://example.org/s%d> <http://example.org/p> \"value number %d\" <http://example.org/g> .\n", i, i)
	}
	return b.String()
}

func BenchmarkNQParseScanner(b *testing.B) {
	input := benchNQInput()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g := rdflibgo.NewGraph()
		if err := Parse(g, strings.NewReader(input)); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkNQParseUnbounded(b *testing.B) {
	input := benchNQInput()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g := rdflibgo.NewGraph()
		if err := Parse(g, strings.NewReader(input), WithUnboundedLines()); err != nil {
			b.Fatal(err)
		}
	}
}

// TestNQParseStreamBNodeIdentity verifies that bnode labels stay stable across
// lines — same `_:b1` on different lines yields the same BNode value. This is
// load-bearing for streaming consumers that need to correlate triples sharing
// a bnode subject without materializing a graph.
func TestNQParseStreamBNodeIdentity(t *testing.T) {
	input := `_:b1 <http://example.org/p> "a" .
_:b1 <http://example.org/q> "b" .
_:b2 <http://example.org/p> "c" .
`
	var subjects []string
	err := ParseStream(strings.NewReader(input), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) error {
		subjects = append(subjects, s.N3())
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"_:b1", "_:b1", "_:b2"}
	if len(subjects) != len(want) {
		t.Fatalf("expected %d subjects, got %d", len(want), len(subjects))
	}
	for i := range want {
		if subjects[i] != want[i] {
			t.Errorf("subject %d: want %q, got %q", i, want[i], subjects[i])
		}
	}
}

// TestNQParseStreamSkipsCommentsAndBlanks verifies the scanner loop honors
// comment and blank-line rules in streaming mode.
func TestNQParseStreamSkipsCommentsAndBlanks(t *testing.T) {
	input := `# a comment

<http://example.org/s> <http://example.org/p> "a" .
   # indented comment

<http://example.org/s> <http://example.org/p> "b" .
`
	count := 0
	err := ParseStream(strings.NewReader(input), func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term) error {
		count++
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 quads, got %d", count)
	}
}
