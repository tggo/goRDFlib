package rdfxml

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// U+0001 in a literal was silently replaced by U+FFFD.
func TestSerializeUnrepresentableCharFails(t *testing.T) {
	cases := map[string]func(g *rdflibgo.Graph){
		"control char in literal": func(g *rdflibgo.Graph) {
			g.Add(iri("http://e/s"), iri("http://e/p"), rdflibgo.NewLiteral("a\x01b"))
		},
		"U+FFFE in literal": func(g *rdflibgo.Graph) {
			g.Add(iri("http://e/s"), iri("http://e/p"), rdflibgo.NewLiteral("a\uFFFEb"))
		},
		"invalid UTF-8 in literal": func(g *rdflibgo.Graph) {
			g.Add(iri("http://e/s"), iri("http://e/p"), rdflibgo.NewLiteral("a\xffb"))
		},
		"control char in subject IRI": func(g *rdflibgo.Graph) {
			g.Add(iri("http://e/s\x02"), iri("http://e/p"), rdflibgo.NewLiteral("v"))
		},
		"control char in object IRI": func(g *rdflibgo.Graph) {
			g.Add(iri("http://e/s"), iri("http://e/p"), iri("http://e/o\x1f"))
		},
		"control char inside triple term": func(g *rdflibgo.Graph) {
			g.Add(iri("http://e/s"), iri("http://e/p"), rdflibgo.NewTripleTerm(iri("http://e/a"), iri("http://e/b"), rdflibgo.NewLiteral("\x00")))
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			g := rdflibgo.NewGraph()
			build(g)
			var buf bytes.Buffer
			err := Serialize(g, &buf)
			if !errors.Is(err, ErrUnrepresentableChar) {
				t.Fatalf("want ErrUnrepresentableChar, got %v\n%s", err, buf.String())
			}
			if !strings.Contains(err.Error(), "N-Triples") {
				t.Errorf("error should suggest a remedy: %v", err)
			}
			if buf.Len() != 0 {
				t.Errorf("output written despite error:\n%s", buf.String())
			}
		})
	}
}

// Characters XML can carry, including those it must write as references,
// survive a round trip.
func TestSerializeRepresentableCharsRoundTrip(t *testing.T) {
	g := rdflibgo.NewGraph()
	g.Add(iri("http://e/s"), iri("http://e/p"), rdflibgo.NewLiteral("tab\there\r\nnew & <x> \"q\" \U0001F600 \uFFFD"))
	assertRoundTrip(t, g)
}
