package nt_test

import (
	"bytes"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/nt"
)

func TestWriterCanonicalEscapes(t *testing.T) {
	s := rdflibgo.NewURIRefUnsafe("http://e/s")
	p := rdflibgo.NewURIRefUnsafe("http://e/p")
	lit := rdflibgo.NewLiteral("a\b\f\x01\x7f￾\U00010000\"\\")

	var plain, canon bytes.Buffer
	for _, c := range []struct {
		buf  *bytes.Buffer
		opts []nt.Option
	}{{&plain, nil}, {&canon, []nt.Option{nt.WithCanonical()}}} {
		w := nt.NewWriter(c.buf, c.opts...)
		if err := w.Write(s, p, lit); err != nil {
			t.Fatal(err)
		}
		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}
	}
	wantCanon := "<http://e/s> <http://e/p> \"a\\b\\f\\u0001\\u007F\\uFFFE\U00010000\\\"\\\\\" .\n"
	if canon.String() != wantCanon {
		t.Errorf("canonical:\n got %q\nwant %q", canon.String(), wantCanon)
	}
	if !strings.Contains(plain.String(), `\U00010000`) || !strings.Contains(plain.String(), `\u0008`) {
		t.Errorf("default output must keep its existing spelling, got %q", plain.String())
	}
}

func TestParseAllowsSpaceBeforeLangTagAndDatatype(t *testing.T) {
	doc := `<http://e/s> <http://e/p> "Alice" @en .
<http://e/s> <http://e/p> "2" ^^ <http://www.w3.org/2001/XMLSchema#integer> .
`
	g := rdflibgo.NewGraph()
	if err := nt.Parse(t.Context(), g, strings.NewReader(doc)); err != nil {
		t.Fatal(err)
	}
	if n, _ := g.Len(t.Context()); n != 2 {
		t.Fatalf("want 2 triples, got %d", n)
	}
}
