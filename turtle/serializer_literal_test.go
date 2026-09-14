package turtle

import (
	"bytes"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/testutil"
)

// A multi-line literal ending in a quote was written as """...\"hi"""" and the
// output did not parse (Turtle 1.1 [24] STRING_LITERAL_LONG_QUOTE).
func TestSerializeLongLiteralQuotesAtDelimiter(t *testing.T) {
	for _, lex := range []string{
		"line1\nsaid \"hi\"",
		"line1\n\"\"",
		"line1\n\"\"\"\"",
		"\"\"\nstart",
		"a\n\"\"x\"\"\"y",
		"a\n\\\"",
	} {
		g := rdflibgo.NewGraph()
		g.Add(rdflibgo.NewURIRefUnsafe("http://e/s"), rdflibgo.NewURIRefUnsafe("http://e/p"), rdflibgo.NewLiteral(lex))
		var b bytes.Buffer
		if err := Serialize(g, &b); err != nil {
			t.Fatal(err)
		}
		g2 := rdflibgo.NewGraph()
		if err := Parse(g2, strings.NewReader(b.String())); err != nil {
			t.Errorf("%q: output does not parse: %v\n%s", lex, err, b.String())
			continue
		}
		testutil.AssertGraphEqual(t, g, g2)
	}
}
