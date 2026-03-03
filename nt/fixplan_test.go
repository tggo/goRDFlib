package nt

import (
	"bytes"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Tests for fix.plan.md items — N-Triples

// P6. N-Triples round-trip with special characters
func TestP6_NTriplesRoundTrip(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")

	cases := []rdflibgo.Term{
		rdflibgo.NewLiteral("hello world"),
		rdflibgo.NewLiteral("line\nnewline"),
		rdflibgo.NewLiteral("tab\there"),
		rdflibgo.NewLiteral(`quote"inside`),
		rdflibgo.NewLiteral(""),
		rdflibgo.NewLiteral("日本語"),
		rdflibgo.NewLiteral("emoji 🎉"),
	}

	for _, obj := range cases {
		g.Add(s, p, obj)
	}

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}

	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(buf.String())); err != nil {
		t.Fatalf("P6: re-parse failed: %v\nOutput:\n%s", err, buf.String())
	}

	if g.Len() != g2.Len() {
		t.Errorf("P6: round-trip lost triples: %d → %d", g.Len(), g2.Len())
	}
}

// R9. N-Triples trailing newline
func TestR9_TrailingNewline(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("http://example.org/s")
	p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
	g.Add(s, p, rdflibgo.NewLiteral("val"))

	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Should end with exactly one newline
	trimmed := strings.TrimRight(out, "\n")
	trailing := len(out) - len(trimmed)
	if trailing != 1 {
		t.Errorf("R9: expected 1 trailing newline, got %d", trailing)
	}
}

// R10. Deterministic N-Triples serialization
func TestR10_NTriplesDeterministic(t *testing.T) {
	g := rdflibgo.NewGraph()
	for i := 0; i < 20; i++ {
		s := rdflibgo.NewURIRefUnsafe("http://example.org/s" + string(rune('A'+i)))
		p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
		g.Add(s, p, rdflibgo.NewLiteral(i))
	}

	var first string
	for i := 0; i < 50; i++ {
		var buf bytes.Buffer
		if err := Serialize(g, &buf); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = buf.String()
		} else if buf.String() != first {
			t.Fatalf("R10: N-Triples not deterministic on iteration %d", i)
		}
	}
}
