// Package term — additional tests to raise coverage above 90%.
package term

import (
	"testing"
)

// --- key.go ---

func TestTermKey(t *testing.T) {
	u, _ := NewURIRef("http://example.org/a")
	if k := TermKey(u); k != "U:http://example.org/a" {
		t.Errorf("URIRef TermKey: %q", k)
	}

	b := NewBNode("b1")
	if k := TermKey(b); k != "B:b1" {
		t.Errorf("BNode TermKey: %q", k)
	}

	lit := NewLiteral("hello")
	if k := TermKey(lit); k == "" {
		t.Error("Literal TermKey should not be empty")
	}

	tt := NewTripleTerm(u, u, lit)
	if k := TermKey(tt); k == "" {
		t.Error("TripleTerm TermKey should not be empty")
	}

	v := NewVariable("x")
	if k := TermKey(v); k != "V:x" {
		t.Errorf("Variable TermKey: %q", k)
	}
}

func TestOptTermKey(t *testing.T) {
	if k := OptTermKey(nil); k != "" {
		t.Errorf("nil should return empty, got %q", k)
	}
	u, _ := NewURIRef("http://example.org/")
	if k := OptTermKey(u); k == "" {
		t.Error("non-nil should return non-empty")
	}
}

func TestOptPredKey(t *testing.T) {
	if k := OptPredKey(nil); k != "" {
		t.Errorf("nil should return empty, got %q", k)
	}
	u, _ := NewURIRef("http://example.org/p")
	if k := OptPredKey(&u); k == "" {
		t.Error("non-nil pred should return non-empty")
	}
}

// --- triple_term.go ---

func TestTripleTermBasic(t *testing.T) {
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	tt := NewTripleTerm(s, p, o)

	if tt.Subject() != s {
		t.Error("Subject mismatch")
	}
	if tt.Predicate() != p {
		t.Error("Predicate mismatch")
	}
	if tt.Object() != o {
		t.Error("Object mismatch")
	}
}

func TestTripleTermN3(t *testing.T) {
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	tt := NewTripleTerm(s, p, o)
	n3 := tt.N3()
	expected := "<<( <http://example.org/s> <http://example.org/p> <http://example.org/o> )>>"
	if n3 != expected {
		t.Errorf("N3: got %q, want %q", n3, expected)
	}
}

func TestTripleTermString(t *testing.T) {
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	tt := NewTripleTerm(s, p, o)
	str := tt.String()
	if str == "" {
		t.Error("String should not be empty")
	}
}

func TestTripleTermEqual(t *testing.T) {
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	tt1 := NewTripleTerm(s, p, o)
	tt2 := NewTripleTerm(s, p, o)

	if !tt1.Equal(tt2) {
		t.Error("identical triple terms should be Equal")
	}

	o2, _ := NewURIRef("http://example.org/other")
	tt3 := NewTripleTerm(s, p, o2)
	if tt1.Equal(tt3) {
		t.Error("different object should not be Equal")
	}

	if tt1.Equal(s) {
		t.Error("TripleTerm should not Equal URIRef")
	}
}

func TestTripleTermPanicNilSubject(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil subject")
		}
	}()
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	NewTripleTerm(nil, p, o)
}

func TestTripleTermPanicNilObject(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil object")
		}
	}()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	NewTripleTerm(s, p, nil)
}

func TestTripleTermWithBNodeSubject(t *testing.T) {
	b := NewBNode("b1")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("val")
	tt := NewTripleTerm(b, p, o)
	if tt.Subject() != b {
		t.Error("BNode subject mismatch")
	}
}

func TestTripleTermNestedTripleTerm(t *testing.T) {
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	inner := NewTripleTerm(s, p, o)
	outer := NewTripleTerm(s, p, inner)
	n3 := outer.N3()
	if n3 == "" {
		t.Error("nested TripleTerm N3 should not be empty")
	}
}

// --- literal.go uncovered branches ---

func TestLiteralTermTypeCov(t *testing.T) {
	lit := NewLiteral("x")
	if lit.termType() != "Literal" {
		t.Errorf("termType: %q", lit.termType())
	}
}

func TestLiteralStringCov(t *testing.T) {
	lit := NewLiteral("hello world")
	if lit.String() != "hello world" {
		t.Errorf("String: %q", lit.String())
	}
}

func TestLiteralDir(t *testing.T) {
	lit := NewLiteral("hello", WithLang("en"), WithDir("ltr"))
	if lit.Dir() != "ltr" {
		t.Errorf("Dir: %q", lit.Dir())
	}
	if lit.Language() != "en" {
		t.Errorf("Language: %q", lit.Language())
	}
	// Verify datatype is rdf:dirLangString
	if lit.Datatype() != RDFDirLangString {
		t.Errorf("Datatype should be rdf:dirLangString, got %q", lit.Datatype())
	}
}

func TestWithDirInvalidIgnored(t *testing.T) {
	lit := NewLiteral("hello", WithLang("en"), WithDir("invalid"))
	if lit.Dir() != "" {
		t.Errorf("invalid dir should be ignored, got %q", lit.Dir())
	}
}

func TestWithLangInvalid(t *testing.T) {
	// Invalid tag should be silently ignored
	lit := NewLiteral("hello", WithLang("123-bad!"))
	if lit.Language() != "" {
		t.Errorf("invalid lang should be empty, got %q", lit.Language())
	}
}

func TestLiteralN3DirectionalLang(t *testing.T) {
	lit := NewLiteral("مرحبا", WithLang("ar"), WithDir("rtl"))
	n3 := lit.N3()
	if n3 == "" {
		t.Error("N3 should not be empty")
	}
	// should contain @ar--rtl
	expected := `"مرحبا"@ar--rtl`
	if n3 != expected {
		t.Errorf("N3 directional: got %q, want %q", n3, expected)
	}
}

func TestLiteralN3MultilineString(t *testing.T) {
	lit := NewLiteral("line1\nline2")
	n3 := lit.N3()
	// Should use triple-quoted form
	if len(n3) < 7 {
		t.Errorf("N3 for multiline should be triple-quoted, got %q", n3)
	}
}

func TestLiteralN3XSDDouble(t *testing.T) {
	// Directly test with explicit lexical forms
	lit2 := Literal{lexical: "1.5e10", datatype: XSDDouble}
	n3 := lit2.N3()
	if n3 != "1.5e10" {
		t.Errorf("XSDDouble with e: %q", n3)
	}
	// Without e notation, should fall through to quoted form
	lit3 := Literal{lexical: "1.5", datatype: XSDDouble}
	n3b := lit3.N3()
	if n3b == "1.5" {
		t.Errorf("XSDDouble without e should be quoted, not shorthand: %q", n3b)
	}
}

func TestLiteralN3XSDDecimal(t *testing.T) {
	// With dot — shorthand
	lit := Literal{lexical: "3.14", datatype: XSDDecimal}
	if n3 := lit.N3(); n3 != "3.14" {
		t.Errorf("XSDDecimal with dot: %q", n3)
	}
	// Without dot — quoted
	lit2 := Literal{lexical: "314", datatype: XSDDecimal}
	n3 := lit2.N3()
	if n3 == "314" {
		t.Errorf("XSDDecimal without dot should be quoted: %q", n3)
	}
}

func TestLiteralN3XSDBoolean(t *testing.T) {
	litT := Literal{lexical: "true", datatype: XSDBoolean}
	if litT.N3() != "true" {
		t.Errorf("boolean true: %q", litT.N3())
	}
	litF := Literal{lexical: "false", datatype: XSDBoolean}
	if litF.N3() != "false" {
		t.Errorf("boolean false: %q", litF.N3())
	}
	// non-canonical boolean falls through to quoted
	litX := Literal{lexical: "yes", datatype: XSDBoolean}
	n3 := litX.N3()
	if n3 == "yes" {
		t.Errorf("non-canonical boolean should be quoted: %q", n3)
	}
}

func TestLiteralValueFloat32(t *testing.T) {
	lit := NewLiteral(float32(3.14))
	v := lit.Value()
	if _, ok := v.(float32); !ok {
		t.Errorf("expected float32, got %T", v)
	}
}

func TestLiteralValueBool(t *testing.T) {
	lit := NewLiteral(true)
	v := lit.Value()
	if b, ok := v.(bool); !ok || !b {
		t.Errorf("expected bool true, got %v (%T)", v, v)
	}
}

func TestLiteralValueInvalidInt(t *testing.T) {
	lit := Literal{lexical: "not-a-number", datatype: XSDInteger}
	v := lit.Value()
	if _, ok := v.(string); !ok {
		t.Errorf("invalid int should return string, got %T", v)
	}
}

func TestLiteralValueInvalidFloat(t *testing.T) {
	lit := Literal{lexical: "not-a-float", datatype: XSDFloat}
	v := lit.Value()
	if _, ok := v.(string); !ok {
		t.Errorf("invalid float should return string, got %T", v)
	}
}

func TestLiteralValueInvalidDouble(t *testing.T) {
	lit := Literal{lexical: "nan-ish", datatype: XSDDouble}
	v := lit.Value()
	if _, ok := v.(string); !ok {
		t.Errorf("invalid double should return string, got %T", v)
	}
}

func TestLiteralValueInvalidBool(t *testing.T) {
	lit := Literal{lexical: "maybe", datatype: XSDBoolean}
	v := lit.Value()
	if _, ok := v.(string); !ok {
		t.Errorf("invalid bool should return string, got %T", v)
	}
}

func TestLiteralValueEqualFloat32(t *testing.T) {
	a := NewLiteral(float32(1.0))
	b := NewLiteral(float32(1.0))
	if !a.ValueEqual(b) {
		t.Error("float32 1.0 == 1.0")
	}
}

func TestLiteralValueEqualBool(t *testing.T) {
	a := NewLiteral(true)
	b := NewLiteral(true)
	if !a.ValueEqual(b) {
		t.Error("bool true == true")
	}
	c := NewLiteral(false)
	if a.ValueEqual(c) {
		t.Error("true != false")
	}
}

func TestLiteralValueEqualStringFallback(t *testing.T) {
	a := NewLiteral("hello")
	b := NewLiteral("hello")
	if !a.ValueEqual(b) {
		t.Error("strings should be value-equal")
	}
	c := NewLiteral("world")
	if a.ValueEqual(c) {
		t.Error("different strings should not be value-equal")
	}
}

func TestLiteralValueEqualDifferentDatatypes(t *testing.T) {
	a := NewLiteral(int64(1))
	b := NewLiteral(float64(1.0))
	if a.ValueEqual(b) {
		t.Error("different datatypes should not be value-equal")
	}
}

// --- term.go uncovered ---

func TestURIRefTermTypeCov(t *testing.T) {
	u, _ := NewURIRef("http://example.org/")
	if u.termType() != "URIRef" {
		t.Errorf("URIRef termType: %q", u.termType())
	}
}

func TestBNodeTermType2(t *testing.T) {
	b := NewBNode("x")
	if b.termType() != "BNode" {
		t.Errorf("BNode termType: %q", b.termType())
	}
}

func TestBNodeSubjectMarker(t *testing.T) {
	b := NewBNode("x")
	// Verify BNode implements Subject (compile-time); calling subject() for coverage.
	b.subject()
}

func TestURIRefSubjectMarker(t *testing.T) {
	u, _ := NewURIRef("http://example.org/")
	u.subject()
}

func TestVariableTermType2(t *testing.T) {
	v := NewVariable("x")
	if v.termType() != "Variable" {
		t.Errorf("Variable termType: %q", v.termType())
	}
}

func TestURIRefN3WithNamespaceManager(t *testing.T) {
	u, _ := NewURIRef("http://example.org/name")
	// With a nil namespace manager (len(ns)>0 but ns[0]==nil)
	n3 := u.N3(nil)
	if n3 != "<http://example.org/name>" {
		t.Errorf("N3 with nil ns: %q", n3)
	}
}

func TestURIRefN3WithPrefixHit(t *testing.T) {
	u, _ := NewURIRef("http://example.org/name")
	nm := mockNS{"http://example.org/name": "ex:name"}
	n3 := u.N3(nm)
	if n3 != "ex:name" {
		t.Errorf("N3 with prefix: %q", n3)
	}
}

func TestURIRefN3WithPrefixMiss(t *testing.T) {
	u, _ := NewURIRef("http://example.org/name")
	nm := mockNS{}
	n3 := u.N3(nm)
	if n3 != "<http://example.org/name>" {
		t.Errorf("N3 with prefix miss: %q", n3)
	}
}

func TestNewURIRefWithBaseInvalidRef(t *testing.T) {
	// url.Parse is very permissive; use a value that parses but causes bad resolve
	// The main case is: invalid base
	_, err := NewURIRefWithBase("foo", "://invalid")
	if err == nil {
		t.Error("expected error for invalid base")
	}
}

func TestBNodeSkolemizeCustomBasepath(t *testing.T) {
	b := NewBNode("abc")
	s := b.Skolemize("http://example.org", "custom/path")
	if s.Value() != "http://example.org/custom/path/abc" {
		t.Errorf("got %q", s.Value())
	}
	// With trailing slash on basepath
	s2 := b.Skolemize("http://example.org", "custom/path/")
	if s2.Value() != "http://example.org/custom/path/abc" {
		t.Errorf("got %q", s2.Value())
	}
}

// --- order.go uncovered ---

func TestTermTypeOrderTripleTerm(t *testing.T) {
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	tt := NewTripleTerm(s, p, o)
	lit := NewLiteral("hello")
	// TripleTerm should sort after Literal
	if CompareTerm(lit, tt) >= 0 {
		t.Error("Literal should sort before TripleTerm")
	}
}

func TestCompareLiteralsXSDInt(t *testing.T) {
	a := Literal{lexical: "5", datatype: XSDInt}
	b := Literal{lexical: "10", datatype: XSDInt}
	if compareLiterals(a, b) >= 0 {
		t.Error("5 should be less than 10")
	}
	if compareLiterals(b, a) <= 0 {
		t.Error("10 should be greater than 5")
	}
	if compareLiterals(a, a) != 0 {
		t.Error("same literal should compare equal")
	}
}

func TestCompareLiteralsXSDLong(t *testing.T) {
	a := Literal{lexical: "100", datatype: XSDLong}
	b := Literal{lexical: "200", datatype: XSDLong}
	if compareLiterals(a, b) >= 0 {
		t.Error("100 < 200")
	}
}

func TestCompareLiteralsXSDFloat(t *testing.T) {
	a := Literal{lexical: "1.5", datatype: XSDFloat}
	b := Literal{lexical: "2.5", datatype: XSDFloat}
	if compareLiterals(a, b) >= 0 {
		t.Error("1.5 < 2.5")
	}
	if compareLiterals(b, a) <= 0 {
		t.Error("2.5 > 1.5")
	}
	if compareLiterals(a, a) != 0 {
		t.Error("equal floats")
	}
}

func TestCompareLiteralsXSDDouble(t *testing.T) {
	a := Literal{lexical: "1.1", datatype: XSDDouble}
	b := Literal{lexical: "2.2", datatype: XSDDouble}
	if compareLiterals(a, b) >= 0 {
		t.Error("1.1 < 2.2")
	}
}

func TestCompareLiteralsXSDDecimal(t *testing.T) {
	a := Literal{lexical: "3.14", datatype: XSDDecimal}
	b := Literal{lexical: "6.28", datatype: XSDDecimal}
	if compareLiterals(a, b) >= 0 {
		t.Error("3.14 < 6.28")
	}
}

func TestCompareLiteralsInvalidNumeric(t *testing.T) {
	// invalid lexical falls back to string compare
	a := Literal{lexical: "bad", datatype: XSDInteger}
	b := Literal{lexical: "bad", datatype: XSDInteger}
	if compareLiterals(a, b) != 0 {
		t.Error("same invalid lexical should compare equal via N3")
	}
}

func TestCompareLiteralsDifferentDatatypes(t *testing.T) {
	a := Literal{lexical: "hello", datatype: XSDString}
	b := Literal{lexical: "world", datatype: XSDDouble}
	// Different datatypes → N3 compare
	_ = compareLiterals(a, b) // just must not panic
}

func TestSortTermsWithTripleTerm(t *testing.T) {
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o, _ := NewURIRef("http://example.org/o")
	tt := NewTripleTerm(s, p, o)
	b := NewBNode("b")
	lit := NewLiteral("hello")
	v := NewVariable("x")

	terms := []Term{v, tt, lit, s, b}
	SortTerms(terms)
	// BNode should be first
	if _, ok := terms[0].(BNode); !ok {
		t.Errorf("first should be BNode, got %T", terms[0])
	}
}

// mockNS is a simple NamespaceManager for testing.
type mockNS map[string]string

func (m mockNS) Prefix(uri string) (string, bool) {
	v, ok := m[uri]
	return v, ok
}
