package nt

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// rdflib #3544: relative IRIs were written, and our own parser rejected the
// output.
func TestSerializeRelativeIRIFails(t *testing.T) {
	rel := rdflibgo.NewURIRefUnsafe("relative/thing")
	s := rdflibgo.NewURIRefUnsafe("http://e/s")
	p := rdflibgo.NewURIRefUnsafe("http://e/p")
	cases := map[string]func(g *rdflibgo.Graph){
		"subject":   func(g *rdflibgo.Graph) { g.Add(rel, p, rdflibgo.NewLiteral("v")) },
		"predicate": func(g *rdflibgo.Graph) { g.Add(s, rel, rdflibgo.NewLiteral("v")) },
		"object":    func(g *rdflibgo.Graph) { g.Add(s, p, rel) },
		"datatype":  func(g *rdflibgo.Graph) { g.Add(s, p, rdflibgo.NewLiteral("v", rdflibgo.WithDatatype(rel))) },
		"triple term": func(g *rdflibgo.Graph) {
			g.Add(s, p, rdflibgo.NewTripleTerm(s, p, rel))
		},
	}
	for name, build := range cases {
		t.Run(name, func(t *testing.T) {
			g := rdflibgo.NewGraph()
			build(g)
			var buf bytes.Buffer
			err := Serialize(g, &buf)
			if !errors.Is(err, ErrRelativeIRI) {
				t.Fatalf("want ErrRelativeIRI, got %v\n%s", err, buf.String())
			}
			if !strings.Contains(err.Error(), "relative/thing") || !strings.Contains(err.Error(), "base") {
				t.Errorf("error should name the IRI and the remedy: %v", err)
			}
			if buf.Len() != 0 {
				t.Errorf("output written despite error: %q", buf.String())
			}
		})
	}
}

// rdflib #2590 analogue: invalid UTF-8 in a literal was written raw.
func TestSerializeInvalidUTF8Fails(t *testing.T) {
	s := rdflibgo.NewURIRefUnsafe("http://e/s")
	p := rdflibgo.NewURIRefUnsafe("http://e/p")
	for name, o := range map[string]rdflibgo.Term{
		"literal (encoded surrogate)": rdflibgo.NewLiteral("a\xed\xba\xadb"),
		"literal (stray byte)":        rdflibgo.NewLiteral("a\xffb", rdflibgo.WithLang("en")),
		"IRI":                         rdflibgo.NewURIRefUnsafe("http://e/\xff"),
	} {
		g := rdflibgo.NewGraph()
		g.Add(s, p, o)
		var buf bytes.Buffer
		if err := Serialize(g, &buf); !errors.Is(err, ErrInvalidUTF8) {
			t.Errorf("%s: want ErrInvalidUTF8, got %v (%q)", name, err, buf.String())
		}
	}
}

func TestSerializeInvalidIRICharFails(t *testing.T) {
	for _, bad := range []string{"http://e/a b", "http://e/{x}", "http://e/a|b", "http://e/a^b", "http://e/a`b", `http://e/a\b`, `http://e/"`} {
		g := rdflibgo.NewGraph()
		g.Add(rdflibgo.NewURIRefUnsafe("http://e/s"), rdflibgo.NewURIRefUnsafe("http://e/p"), rdflibgo.NewURIRefUnsafe(bad))
		if err := Serialize(g, &bytes.Buffer{}); !errors.Is(err, ErrInvalidIRI) {
			t.Errorf("%q: want ErrInvalidIRI, got %v", bad, err)
		}
	}
}

// Whatever Serialize accepts, Parse reads back.
func TestSerializeOutputReparses(t *testing.T) {
	g := rdflibgo.NewGraph()
	s := rdflibgo.NewURIRefUnsafe("urn:x:s")
	p := rdflibgo.NewURIRefUnsafe("http://e/p")
	g.Add(s, p, rdflibgo.NewLiteral("tab\t\x01 \"q\" \\ \U0001F600 é"))
	g.Add(s, p, rdflibgo.NewURIRefUnsafe("http://e/é\U0001F600?q=1#f"))
	g.Add(s, p, rdflibgo.NewLiteral("x", rdflibgo.WithLang("ar"), rdflibgo.WithDir("rtl")))
	var buf bytes.Buffer
	if err := Serialize(g, &buf); err != nil {
		t.Fatal(err)
	}
	g2 := rdflibgo.NewGraph()
	if err := Parse(g2, strings.NewReader(buf.String())); err != nil {
		t.Fatalf("%v\n%s", err, buf.String())
	}
	if g2.Len() != g.Len() {
		t.Errorf("got %d triples, want %d\n%s", g2.Len(), g.Len(), buf.String())
	}
}
