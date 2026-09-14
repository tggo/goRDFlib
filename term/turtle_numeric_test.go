package term

import "testing"

func TestTurtleNumericShorthandGrammar(t *testing.T) {
	decimals := map[string]bool{
		"1.5": true, "-1.5": true, "+.5": true, ".5": true, "0.0": true,
		"5.": false, "1.5e3": false, "15": false, "1..5": false, ".": false, "": false, "1.5x": false,
	}
	for in, want := range decimals {
		if got := isTurtleDecimal(in); got != want {
			t.Errorf("isTurtleDecimal(%q) = %v, want %v", in, got, want)
		}
	}
	doubles := map[string]bool{
		"1.5e3": true, "1e3": true, "-1E-3": true, ".5e1": true, "1.e2": true, "+2.0E+10": true,
		"1.5": false, "0x1ep3": false, "Inf": false, "NaN": false, "e3": false, ".e3": false, "1e": false, "1e+": false, "1e3.0": false,
	}
	for in, want := range doubles {
		if got := isTurtleDouble(in); got != want {
			t.Errorf("isTurtleDouble(%q) = %v, want %v", in, got, want)
		}
	}
}

// Distinct literals must never share a TermKey: stores key their indexes on
// it, so a collision silently merges two triples into one.
func TestTermKeyDistinguishesIllTypedNumerics(t *testing.T) {
	cases := []Literal{
		NewLiteral("1.5e3", WithDatatype(XSDDecimal)),
		NewLiteral("1.5e3", WithDatatype(XSDDouble)),
		NewLiteral("5.", WithDatatype(XSDDecimal)),
		NewLiteral("0x1ep3", WithDatatype(XSDDouble)),
		NewLiteral("1.5", WithDatatype(XSDDecimal)),
		NewLiteral("1.5", WithDatatype(XSDDouble)),
		NewLiteral("15", WithDatatype(XSDInteger)),
		NewLiteral("15", WithDatatype(XSDDecimal)),
		NewLiteral("2.0E10", WithDatatype(XSDDouble)),
		NewLiteral(".5", WithDatatype(XSDDecimal)),
		NewLiteral("-7", WithDatatype(XSDInteger)),
	}
	seen := map[string]Literal{}
	for _, l := range cases {
		k := TermKey(l)
		if prev, dup := seen[k]; dup {
			t.Errorf("%q^^%s and %q^^%s share key %q", prev.Lexical(), prev.Datatype().Value(), l.Lexical(), l.Datatype().Value(), k)
		}
		seen[k] = l
		back, err := TermFromKey(k)
		if err != nil {
			t.Fatalf("TermFromKey(%q): %v", k, err)
		}
		if !back.Equal(l) {
			t.Errorf("key %q decodes to %s, want %q^^%s", k, back.N3(), l.Lexical(), l.Datatype().Value())
		}
	}
}
