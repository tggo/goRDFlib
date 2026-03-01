package rdflibgo

import "testing"

// Ported from: rdflib.term.Literal

func TestLiteralString(t *testing.T) {
	// Ported from: rdflib.term.Literal("hello")
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
	// Ported from: rdflib.term.Literal(42)
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

func TestLiteralFloat64(t *testing.T) {
	// Ported from: rdflib.term.Literal(3.14) — Python float → xsd:double
	l := NewLiteral(3.14)
	if l.Datatype() != XSDDouble {
		t.Errorf("datatype: got %v", l.Datatype())
	}
}

func TestLiteralBool(t *testing.T) {
	// Ported from: rdflib.term.Literal(True)
	l := NewLiteral(true)
	if got := l.N3(); got != "true" {
		t.Errorf("N3: got %q", got)
	}
	l2 := NewLiteral(false)
	if got := l2.N3(); got != "false" {
		t.Errorf("N3: got %q", got)
	}
}

func TestLiteralLang(t *testing.T) {
	// Ported from: rdflib.term.Literal("hello", lang="en")
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
	// Ported from: rdflib.term.Literal("42", datatype=XSD.integer)
	l := NewLiteral("42", WithDatatype(XSDInteger))
	if l.Datatype() != XSDInteger {
		t.Errorf("datatype: got %v", l.Datatype())
	}
	if got := l.N3(); got != "42" {
		t.Errorf("N3: got %q", got)
	}
}

func TestLiteralMultiline(t *testing.T) {
	// Ported from: rdflib.term.Literal with newlines
	l := NewLiteral("line1\nline2")
	n3 := l.N3()
	if n3 != `"""line1\nline2"""` {
		t.Errorf("N3: got %q", n3)
	}
}

func TestLiteralStructEquality(t *testing.T) {
	// Ported from: rdflib.term.Literal.__eq__ (exact match)
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

func TestLiteralEqValueSpace(t *testing.T) {
	// Ported from: rdflib.term.Literal.eq() — value-space comparison
	a := NewLiteral("1", WithDatatype(XSDInteger))
	b := NewLiteral("01", WithDatatype(XSDInteger))
	if a == b {
		t.Error("struct equality should differ for '1' vs '01'")
	}
	if !a.Eq(b) {
		t.Error("Eq() should match '1' and '01' as integers")
	}
}

func TestLiteralValue(t *testing.T) {
	// Ported from: rdflib.term.Literal.toPython()
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

func TestLiteralEscape(t *testing.T) {
	// Ported from: rdflib.term.Literal.n3() escape handling
	l := NewLiteral(`say "hi"`)
	got := l.N3()
	expected := `"say \"hi\""`
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}
