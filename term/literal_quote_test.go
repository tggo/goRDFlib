package term

import "testing"

// Lexical forms that force the triple-quoted N3 form (they contain a newline)
// and put quotes next to the delimiters.
var tripleQuoteCases = []string{
	"line1\nsaid \"hi\"",
	"line1\n\"",
	"line1\n\"\"",
	"line1\n\"\"\"",
	"line1\n\"\"\"\"",
	"\"\nstart",
	"a\n\"\"x\"\"y\"\"\"z",
	"a\n\\\"",
	"a\n\\",
	"\n",
}

// The closing """ must not be preceded by a raw quote (Turtle 1.1 [24]).
func TestTripleQuotedN3NeverTouchesDelimiter(t *testing.T) {
	for _, lex := range tripleQuoteCases {
		n3 := NewLiteral(lex).N3()
		body := n3[3 : len(n3)-3]
		if len(body) > 0 && body[len(body)-1] == '"' && (len(body) < 2 || body[len(body)-2] != '\\') {
			t.Errorf("%q: N3 %s ends its body in a raw quote", lex, n3)
		}
	}
}

// Store keys embed N3, so the decoder must find the real closing delimiter.
func TestTripleQuotedKeyRoundTrip(t *testing.T) {
	for _, lex := range tripleQuoteCases {
		for _, lit := range []Literal{NewLiteral(lex), NewLiteral(lex, WithLang("en")), NewLiteral(lex, WithDatatype(XSDAnyURI))} {
			got, err := TermFromKey(TermKey(lit))
			if err != nil {
				t.Errorf("%q: %v", lex, err)
				continue
			}
			if !got.Equal(lit) {
				t.Errorf("%q: key %q decoded to %#v", lex, TermKey(lit), got)
			}
			if n := consumeLiteralN3(lit.N3() + " rest"); n != len(lit.N3()) {
				t.Errorf("%q: consumeLiteralN3 = %d, want %d", lex, n, len(lit.N3()))
			}
		}
	}
}
