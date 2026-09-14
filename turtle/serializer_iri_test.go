package turtle

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// The serializer must never write an IRIREF its own parser rejects. Such an
// IRI can still be built with NewURIRefUnsafe, so it returns an error.
func TestSerializeRejectsUnwritableIRI(t *testing.T) {
	p := rdflibgo.NewURIRefUnsafe("http://e/p")
	bad := rdflibgo.NewURIRefUnsafe("http://e/a\tb")
	cases := map[string]func(g *rdflibgo.Graph){
		"subject":   func(g *rdflibgo.Graph) { g.Add(bad, p, rdflibgo.NewLiteral("v")) },
		"predicate": func(g *rdflibgo.Graph) { g.Add(p, rdflibgo.NewURIRefUnsafe("http://e/a>b"), rdflibgo.NewLiteral("v")) },
		"object":    func(g *rdflibgo.Graph) { g.Add(p, p, rdflibgo.NewURIRefUnsafe("http://e/{x}")) },
		"datatype":  func(g *rdflibgo.Graph) { g.Add(p, p, rdflibgo.NewLiteral("v", rdflibgo.WithDatatype(bad))) },
		"triple term": func(g *rdflibgo.Graph) {
			g.Add(p, p, rdflibgo.NewTripleTerm(p, p, bad))
		},
	}
	for name, add := range cases {
		g := rdflibgo.NewGraph()
		add(g)
		var b bytes.Buffer
		err := Serialize(g, &b)
		if !errors.Is(err, rdflibgo.ErrInvalidIRI) {
			t.Errorf("%s: err = %v, want ErrInvalidIRI; output:\n%s", name, err, b.String())
		}
	}

	g := rdflibgo.NewGraph()
	g.Add(p, p, rdflibgo.NewLiteral("v"))
	if err := Serialize(g, &bytes.Buffer{}, WithBase("http://e/ x")); !errors.Is(err, rdflibgo.ErrInvalidIRI) {
		t.Errorf("base: err = %v, want ErrInvalidIRI", err)
	}

	// A valid non-ASCII IRI still round-trips.
	g = rdflibgo.NewGraph()
	g.Add(rdflibgo.NewURIRefUnsafe("http://e/é"), p, rdflibgo.NewLiteral("v"))
	var b bytes.Buffer
	if err := Serialize(g, &b); err != nil {
		t.Fatal(err)
	}
	if err := Parse(rdflibgo.NewGraph(), strings.NewReader(b.String())); err != nil {
		t.Errorf("output does not parse: %v\n%s", err, b.String())
	}
}
