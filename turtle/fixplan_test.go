package turtle

import (
	"bytes"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Tests for fix.plan.md items — Turtle parser/serializer

// R1. Serializer special chars — must fall back to <full-IRI>
func TestR1_SerializerSpecialChars(t *testing.T) {
	uris := []string{
		"http://example.org/name(1)",
		"http://example.org/ends.",
		"http://example.org/a+b=c",
	}
	for _, uri := range uris {
		g := rdflibgo.NewGraph()
		g.Bind("ex", rdflibgo.NewURIRefUnsafe("http://example.org/"))
		s := rdflibgo.NewURIRefUnsafe(uri)
		p := rdflibgo.NewURIRefUnsafe("http://example.org/p")
		g.Add(s, p, rdflibgo.NewLiteral("val"))

		var buf bytes.Buffer
		if err := Serialize(g, &buf); err != nil {
			t.Errorf("R1: serialize failed for %s: %v", uri, err)
			continue
		}
		out := buf.String()

		// Re-parse to verify round-trip
		g2 := rdflibgo.NewGraph()
		if err := Parse(g2, strings.NewReader(out)); err != nil {
			t.Errorf("R1: re-parse failed for %s:\nOutput:\n%s\nError: %v", uri, out, err)
		}
	}
}

// R10. Deterministic serialization
func TestR10_DeterministicSerialization(t *testing.T) {
	g := rdflibgo.NewGraph()
	ex := "http://example.org/"
	g.Bind("ex", rdflibgo.NewURIRefUnsafe(ex))
	for i := 0; i < 20; i++ {
		s := rdflibgo.NewURIRefUnsafe(ex + "s" + string(rune('A'+i)))
		p := rdflibgo.NewURIRefUnsafe(ex + "p")
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
			t.Fatalf("R10: serialization not deterministic on iteration %d", i)
		}
	}
}

// P4. Prefixed name with empty local part before dot
func TestP4_EmptyLocalBeforeDot(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .
ex: ex:p ex:o .
`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err != nil {
		t.Fatalf("P4: parse failed: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("P4: expected 1 triple, got %d", g.Len())
	}
}

// R4. Dot in middle of local name
func TestR4_DotInLocalName(t *testing.T) {
	input := `@prefix ex: <http://example.org/> .
ex:local.name ex:p ex:o .
`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err != nil {
		t.Fatalf("R4: parse failed: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("R4: expected 1 triple, got %d", g.Len())
	}
}

// P5. \uXXXX escapes in IRIs
func TestP5_UnicodeEscapeInIRI(t *testing.T) {
	input := `<http://example.org/\u00E9> <http://example.org/p> "val" .
`
	g := rdflibgo.NewGraph()
	err := Parse(g, strings.NewReader(input))
	if err != nil {
		t.Fatalf("P5: parse failed: %v", err)
	}
	if g.Len() != 1 {
		t.Errorf("P5: expected 1 triple, got %d", g.Len())
	}
}
