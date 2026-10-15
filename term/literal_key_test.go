package term

import (
	"strings"
	"testing"
	"unsafe"
)

func literalKeyCases() []Literal {
	return []Literal{
		NewLiteral("plain"),
		NewLiteral(""),
		NewLiteral(`say "hi"`),
		NewLiteral("two\nlines"),
		NewLiteral("two\nlines with \"\"\" inside"),
		NewLiteral(`back\slash`),
		NewLiteral("hallo", WithLang("DE")),
		NewLiteral("salam", WithLang("ar"), WithDir("rtl")),
		NewLiteral(42),
		NewLiteral(1.5),
		NewLiteral(true),
		NewLiteral("042", WithDatatype(XSDInteger)),
		NewLiteral("1.5e3", WithDatatype(XSDDouble)),
		NewLiteral("x", WithDatatype(NewURIRefUnsafe("http://example.org/dt"))),
		{}, // zero Literal: no cached key
	}
}

// TestLiteralKeyMatchesN3 pins the cached key to its definition.
func TestLiteralKeyMatchesN3(t *testing.T) {
	for _, l := range literalKeyCases() {
		if got, want := TermKey(l), "L:"+l.N3(); got != want {
			t.Errorf("TermKey(%#v) = %q, want %q", l, got, want)
		}
		back, err := TermFromKey(TermKey(l))
		if l != (Literal{}) && (err != nil || !back.(Literal).Equal(l)) {
			t.Errorf("TermFromKey(TermKey(%#v)) = %#v, %v", l, back, err)
		}
	}
}

// TestLiteralTermKeyDoesNotAllocate: store add, remove and lookup call
// TermKey on every literal (issue #53).
func TestLiteralTermKeyDoesNotAllocate(t *testing.T) {
	var l Term = NewLiteral("name 1", WithLang("en"))
	if n := testing.AllocsPerRun(100, func() { _ = TermKey(l) }); n != 0 {
		t.Fatalf("TermKey(Literal) allocates %v times", n)
	}
}

// TestLargeLiteralHeldOnce: a lexical form that needs no escaping shares the
// key's bytes, so a multi-megabyte WKT literal is not held twice.
func TestLargeLiteralHeldOnce(t *testing.T) {
	wkt := "POLYGON((" + strings.Repeat("30 10, 40 40, ", 1<<16) + "30 10))"
	for _, l := range []Literal{
		NewLiteral(wkt, WithDatatype(NewURIRefUnsafe("http://www.opengis.net/ont/geosparql#wktLiteral"))),
		NewLiteral(wkt),
	} {
		lex, key := l.Lexical(), TermKey(l)
		start := uintptr(unsafe.Pointer(unsafe.StringData(key)))
		p := uintptr(unsafe.Pointer(unsafe.StringData(lex)))
		if p < start || p >= start+uintptr(len(key)) {
			t.Errorf("lexical form of %d bytes is a separate copy of the key", len(lex))
		}
	}
	// Escaped forms cannot share: documented on shareLexical.
	if l := NewLiteral(wkt + "\n"); l.Lexical() != wkt+"\n" {
		t.Error("multi-line lexical form changed")
	}
}

// FuzzLiteralKey holds literalKey to its definition, "L:" + N3.
func FuzzLiteralKey(f *testing.F) {
	f.Add("plain", "", "", "")
	f.Add(`q"\`+"\t\r", "en", "rtl", "")
	f.Add("x\ny", "", "", "")
	f.Add("42", "", "", XSDInteger.Value())
	f.Add("4 2", "", "", XSDInteger.Value())
	f.Add("v", "", "", "http://example.org/dt")
	f.Fuzz(func(t *testing.T, lex, lang, dir, dt string) {
		var opts []LiteralOption
		if lang != "" {
			opts = append(opts, WithLang(lang), WithDir(dir))
		}
		if dt != "" {
			opts = append(opts, WithDatatype(NewURIRefUnsafe(dt)))
		}
		l := NewLiteral(lex, opts...)
		if got, want := TermKey(l), "L:"+l.N3(); got != want {
			t.Fatalf("TermKey = %q, want %q", got, want)
		}
		if l.Lexical() != lex {
			t.Fatalf("Lexical = %q, want %q", l.Lexical(), lex)
		}
	})
}
