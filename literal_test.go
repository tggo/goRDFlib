package rdflibgo

import "testing"

// Ported from: rdflib.term.Literal

func TestLiteralString(t *testing.T) {
	l := NewLiteral("hello")
	if l.Lexical() != "hello" {
		t.Errorf("got %q", l.Lexical())
	}
	if l.Datatype() != XSDString {
		t.Errorf("datatype: got %v", l.Datatype())
	}
	if got := l.N3(); got != `"hello"` {
		t.Errorf("N3: got %q", got)
	}
}

func TestLiteralInt(t *testing.T) {
	l := NewLiteral(42)
	if l.Lexical() != "42" {
		t.Errorf("got %q", l.Lexical())
	}
	if l.Datatype() != XSDInteger {
		t.Errorf("datatype: got %v", l.Datatype())
	}
	if got := l.N3(); got != "42" {
		t.Errorf("N3: got %q", got)
	}
}

func TestLiteralInt64(t *testing.T) {
	l := NewLiteral(int64(99))
	if l.Lexical() != "99" {
		t.Errorf("got %q", l.Lexical())
	}
	if l.Datatype() != XSDInteger {
		t.Errorf("datatype: got %v", l.Datatype())
	}
}

func TestLiteralFloat32(t *testing.T) {
	l := NewLiteral(float32(1.5))
	if l.Datatype() != XSDFloat {
		t.Errorf("datatype: got %v", l.Datatype())
	}
	v := l.Value()
	if _, ok := v.(float32); !ok {
		t.Errorf("expected float32, got %T", v)
	}
}

func TestLiteralFloat64(t *testing.T) {
	l := NewLiteral(3.14)
	if l.Datatype() != XSDDouble {
		t.Errorf("datatype: got %v", l.Datatype())
	}
	v := l.Value()
	if _, ok := v.(float64); !ok {
		t.Errorf("expected float64, got %T", v)
	}
}

func TestLiteralBool(t *testing.T) {
	l := NewLiteral(true)
	if got := l.N3(); got != "true" {
		t.Errorf("N3: got %q", got)
	}
	l2 := NewLiteral(false)
	if got := l2.N3(); got != "false" {
		t.Errorf("N3: got %q", got)
	}
	if l.Value() != true || l2.Value() != false {
		t.Error("bool Value() mismatch")
	}
}

func TestLiteralDefaultType(t *testing.T) {
	// Non-standard type falls back to string
	type myType struct{}
	l := NewLiteral(myType{})
	if l.Datatype() != XSDString {
		t.Errorf("expected XSDString, got %v", l.Datatype())
	}
}

func TestLiteralLang(t *testing.T) {
	l := NewLiteral("hello", WithLang("EN"))
	if l.Language() != "en" {
		t.Errorf("lang: got %q", l.Language())
	}
	if l.Datatype() != RDFLangString {
		t.Errorf("datatype: got %v", l.Datatype())
	}
	if got := l.N3(); got != `"hello"@en` {
		t.Errorf("N3: got %q", got)
	}
}

func TestLiteralDatatype(t *testing.T) {
	l := NewLiteral("42", WithDatatype(XSDInteger))
	if l.Datatype() != XSDInteger {
		t.Errorf("datatype: got %v", l.Datatype())
	}
	if got := l.N3(); got != "42" {
		t.Errorf("N3: got %q", got)
	}
}

func TestLiteralDecimalN3(t *testing.T) {
	l := NewLiteral("3.14", WithDatatype(XSDDecimal))
	if got := l.N3(); got != "3.14" {
		t.Errorf("N3: got %q", got)
	}
}

func TestLiteralDoubleN3(t *testing.T) {
	l := NewLiteral("1.5e2", WithDatatype(XSDDouble))
	if got := l.N3(); got != "1.5e2" {
		t.Errorf("N3: got %q", got)
	}
}

func TestLiteralNonShortcutN3(t *testing.T) {
	// An integer that can't be parsed should not use shorthand
	l := NewLiteral("abc", WithDatatype(XSDInteger))
	got := l.N3()
	if got == "abc" {
		t.Error("non-parseable integer should not use shorthand")
	}
}

func TestLiteralMultiline(t *testing.T) {
	l := NewLiteral("line1\nline2")
	n3 := l.N3()
	if n3 != `"""line1\nline2"""` {
		t.Errorf("N3: got %q", n3)
	}
}

func TestLiteralStructEquality(t *testing.T) {
	a := NewLiteral("hello")
	b := NewLiteral("hello")
	if a != b {
		t.Error("identical literals should be ==")
	}
	c := NewLiteral("hello", WithLang("en"))
	if a == c {
		t.Error("different lang should not be ==")
	}
}

func TestLiteralEqual(t *testing.T) {
	a := NewLiteral("hello")
	b := NewLiteral("hello")
	if !a.Equal(b) {
		t.Error("should be Equal")
	}
	c := NewLiteral("world")
	if a.Equal(c) {
		t.Error("should not be Equal")
	}
	u, _ := NewURIRef("http://example.org/")
	if a.Equal(u) {
		t.Error("Literal should not Equal URIRef")
	}
}

func TestLiteralEqValueSpace(t *testing.T) {
	a := NewLiteral("1", WithDatatype(XSDInteger))
	b := NewLiteral("01", WithDatatype(XSDInteger))
	if a == b {
		t.Error("struct equality should differ for '1' vs '01'")
	}
	if !a.Eq(b) {
		t.Error("Eq() should match '1' and '01' as integers")
	}
}

func TestLiteralEqFloat(t *testing.T) {
	a := NewLiteral("1.0", WithDatatype(XSDDouble))
	b := NewLiteral("1.00", WithDatatype(XSDDouble))
	if !a.Eq(b) {
		t.Error("Eq() should match 1.0 and 1.00 as floats")
	}
}

func TestLiteralEqBool(t *testing.T) {
	a := NewLiteral("true", WithDatatype(XSDBoolean))
	b := NewLiteral("true", WithDatatype(XSDBoolean))
	if !a.Eq(b) {
		t.Error("Eq() should match booleans")
	}
}

func TestLiteralEqDifferentDatatype(t *testing.T) {
	a := NewLiteral("1", WithDatatype(XSDInteger))
	b := NewLiteral("1", WithDatatype(XSDString))
	if a.Eq(b) {
		t.Error("different datatypes should not Eq")
	}
}

func TestLiteralEqString(t *testing.T) {
	a := NewLiteral("hello")
	b := NewLiteral("hello")
	if !a.Eq(b) {
		t.Error("identical strings should Eq")
	}
}

func TestLiteralValue(t *testing.T) {
	l := NewLiteral(42)
	v := l.Value()
	if v != int64(42) {
		t.Errorf("got %v (%T)", v, v)
	}

	lb := NewLiteral(true)
	if lb.Value() != true {
		t.Errorf("got %v", lb.Value())
	}
}

func TestLiteralValueUnparseable(t *testing.T) {
	// If lexical can't be parsed for the datatype, Value returns the lexical string
	l := NewLiteral("notanumber", WithDatatype(XSDInteger))
	if _, ok := l.Value().(string); !ok {
		t.Errorf("expected string fallback, got %T", l.Value())
	}
}

func TestLiteralEscape(t *testing.T) {
	l := NewLiteral(`say "hi"`)
	got := l.N3()
	expected := `"say \"hi\""`
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestLiteralEscapeBackslash(t *testing.T) {
	l := NewLiteral(`path\to`)
	got := l.N3()
	if got != `"path\\to"` {
		t.Errorf("got %q", got)
	}
}

func TestLiteralEscapeCarriageReturn(t *testing.T) {
	l := NewLiteral("a\rb")
	got := l.N3()
	if got != `"a\rb"` {
		t.Errorf("got %q", got)
	}
}

func TestLiteralTermType(t *testing.T) {
	l := NewLiteral("x")
	if l.N3() != `"x"` {
		t.Errorf("unexpected N3: %q", l.N3())
	}
}

func TestLiteralCustomDatatype(t *testing.T) {
	dt, _ := NewURIRef("http://example.org/mytype")
	l := NewLiteral("val", WithDatatype(dt))
	expected := `"val"^^<http://example.org/mytype>`
	if got := l.N3(); got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

// --- Benchmarks ---

func BenchmarkNewLiteralString(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewLiteral("hello world")
	}
}

func BenchmarkNewLiteralInt(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewLiteral(42)
	}
}

func BenchmarkLiteralN3(b *testing.B) {
	l := NewLiteral("hello world")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.N3()
	}
}

func BenchmarkLiteralEq(b *testing.B) {
	a := NewLiteral("1", WithDatatype(XSDInteger))
	c := NewLiteral("01", WithDatatype(XSDInteger))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		a.Eq(c)
	}
}

func BenchmarkEscapeLiteral(b *testing.B) {
	s := `This is a "test" with \backslash and
newline`
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Exercise escaping via Literal.N3()
		NewLiteral(s).N3()
	}
}
