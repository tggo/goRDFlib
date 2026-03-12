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

// =============================================================================
// decode.go — tripleTermFromN3
// =============================================================================

func TestTripleTermFromN3_Basic(t *testing.T) {
	tt, err := tripleTermFromN3("<<( <http://example.org/s> <http://example.org/p> <http://example.org/o> )>>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tt.Subject().(URIRef).Value() != "http://example.org/s" {
		t.Errorf("subject: %v", tt.Subject())
	}
	if tt.Predicate().Value() != "http://example.org/p" {
		t.Errorf("predicate: %v", tt.Predicate())
	}
	if tt.Object().(URIRef).Value() != "http://example.org/o" {
		t.Errorf("object: %v", tt.Object())
	}
}

func TestTripleTermFromN3_BNodeSubject(t *testing.T) {
	tt, err := tripleTermFromN3("<<( _:b1 <http://example.org/p> <http://example.org/o> )>>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tt.Subject().(BNode); !ok {
		t.Errorf("expected BNode subject, got %T", tt.Subject())
	}
}

func TestTripleTermFromN3_LiteralObject(t *testing.T) {
	tt, err := tripleTermFromN3(`<<( <http://example.org/s> <http://example.org/p> "hello" )>>`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lit, ok := tt.Object().(Literal)
	if !ok {
		t.Fatalf("expected Literal object, got %T", tt.Object())
	}
	if lit.Lexical() != "hello" {
		t.Errorf("literal lexical: %q", lit.Lexical())
	}
}

func TestTripleTermFromN3_LangLiteralObject(t *testing.T) {
	tt, err := tripleTermFromN3(`<<( <http://example.org/s> <http://example.org/p> "bonjour"@fr )>>`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lit, ok := tt.Object().(Literal)
	if !ok {
		t.Fatalf("expected Literal object, got %T", tt.Object())
	}
	if lit.Language() != "fr" {
		t.Errorf("expected lang fr, got %q", lit.Language())
	}
}

func TestTripleTermFromN3_TypedLiteralObject(t *testing.T) {
	tt, err := tripleTermFromN3(`<<( <http://example.org/s> <http://example.org/p> "42"^^<http://www.w3.org/2001/XMLSchema#integer> )>>`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lit, ok := tt.Object().(Literal)
	if !ok {
		t.Fatalf("expected Literal object, got %T", tt.Object())
	}
	if lit.Datatype() != XSDInteger {
		t.Errorf("expected xsd:integer, got %v", lit.Datatype())
	}
}

func TestTripleTermFromN3_InvalidFormat(t *testing.T) {
	_, err := tripleTermFromN3("not a triple term")
	if err == nil {
		t.Error("expected error for invalid format")
	}
}

func TestTripleTermFromN3_InvalidSubject(t *testing.T) {
	// A literal cannot be a subject
	_, err := tripleTermFromN3(`<<( "bad" <http://example.org/p> <http://example.org/o> )>>`)
	if err == nil {
		t.Error("expected error for literal subject")
	}
}

func TestTripleTermFromN3_InvalidPredicate(t *testing.T) {
	// A BNode cannot be a predicate
	_, err := tripleTermFromN3("<<( <http://example.org/s> _:b1 <http://example.org/o> )>>")
	if err == nil {
		t.Error("expected error for non-URIRef predicate")
	}
}

func TestTripleTermFromN3_Nested(t *testing.T) {
	n3 := "<<( <http://example.org/s> <http://example.org/p> <<( <http://example.org/a> <http://example.org/b> <http://example.org/c> )>> )>>"
	tt, err := tripleTermFromN3(n3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inner, ok := tt.Object().(TripleTerm)
	if !ok {
		t.Fatalf("expected nested TripleTerm, got %T", tt.Object())
	}
	if inner.Subject().(URIRef).Value() != "http://example.org/a" {
		t.Errorf("inner subject: %v", inner.Subject())
	}
}

// =============================================================================
// decode.go — parseOneTermN3
// =============================================================================

func TestParseOneTermN3_URIRef(t *testing.T) {
	term, rest, err := parseOneTermN3("<http://example.org/x> rest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	u, ok := term.(URIRef)
	if !ok {
		t.Fatalf("expected URIRef, got %T", term)
	}
	if u.Value() != "http://example.org/x" {
		t.Errorf("value: %q", u.Value())
	}
	if rest != " rest" {
		t.Errorf("rest: %q", rest)
	}
}

func TestParseOneTermN3_BNode(t *testing.T) {
	term, rest, err := parseOneTermN3("_:node1 more")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, ok := term.(BNode)
	if !ok {
		t.Fatalf("expected BNode, got %T", term)
	}
	if b.Value() != "node1" {
		t.Errorf("value: %q", b.Value())
	}
	if rest != " more" {
		t.Errorf("rest: %q", rest)
	}
}

func TestParseOneTermN3_BNodeNoRemainder(t *testing.T) {
	term, rest, err := parseOneTermN3("_:node1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := term.(BNode); !ok {
		t.Fatalf("expected BNode, got %T", term)
	}
	if rest != "" {
		t.Errorf("rest should be empty, got %q", rest)
	}
}

func TestParseOneTermN3_Literal(t *testing.T) {
	term, rest, err := parseOneTermN3(`"hello" more`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lit, ok := term.(Literal)
	if !ok {
		t.Fatalf("expected Literal, got %T", term)
	}
	if lit.Lexical() != "hello" {
		t.Errorf("lexical: %q", lit.Lexical())
	}
	if rest != " more" {
		t.Errorf("rest: %q", rest)
	}
}

func TestParseOneTermN3_LiteralWithLang(t *testing.T) {
	// parseOneTermN3 parses the literal via literalFromN3, and consumes via consumeLiteralN3
	term, _, err := parseOneTermN3(`"hello"@en`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lit := term.(Literal)
	if lit.Language() != "en" {
		t.Errorf("lang: %q", lit.Language())
	}
}

func TestParseOneTermN3_LiteralWithDatatype(t *testing.T) {
	term, _, err := parseOneTermN3(`"42"^^<http://www.w3.org/2001/XMLSchema#integer>`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lit := term.(Literal)
	if lit.Datatype() != XSDInteger {
		t.Errorf("datatype: %v", lit.Datatype())
	}
}

func TestParseOneTermN3_BareInteger(t *testing.T) {
	term, rest, err := parseOneTermN3("42 more")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lit := term.(Literal)
	if lit.Lexical() != "42" {
		t.Errorf("lexical: %q", lit.Lexical())
	}
	if lit.Datatype() != XSDInteger {
		t.Errorf("datatype: %v", lit.Datatype())
	}
	if rest != " more" {
		t.Errorf("rest: %q", rest)
	}
}

func TestParseOneTermN3_BareIntegerNoRemainder(t *testing.T) {
	term, rest, err := parseOneTermN3("42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lit := term.(Literal)
	if lit.Lexical() != "42" {
		t.Errorf("lexical: %q", lit.Lexical())
	}
	if rest != "" {
		t.Errorf("rest: %q", rest)
	}
}

func TestParseOneTermN3_BareBoolean(t *testing.T) {
	term, _, err := parseOneTermN3("true")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lit := term.(Literal)
	if lit.Datatype() != XSDBoolean {
		t.Errorf("datatype: %v", lit.Datatype())
	}
}

func TestParseOneTermN3_Empty(t *testing.T) {
	_, _, err := parseOneTermN3("")
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestParseOneTermN3_UnterminatedURI(t *testing.T) {
	_, _, err := parseOneTermN3("<http://example.org/no-close")
	if err == nil {
		t.Error("expected error for unterminated URI")
	}
}

func TestParseOneTermN3_TripleTerm(t *testing.T) {
	input := "<<( <http://example.org/s> <http://example.org/p> <http://example.org/o> )>> rest"
	term, rest, err := parseOneTermN3(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := term.(TripleTerm); !ok {
		t.Fatalf("expected TripleTerm, got %T", term)
	}
	if rest != " rest" {
		t.Errorf("rest: %q", rest)
	}
}

// =============================================================================
// decode.go — consumeLiteralN3
// =============================================================================

func TestConsumeLiteralN3_Simple(t *testing.T) {
	n := consumeLiteralN3(`"hello" rest`)
	if n != 7 { // "hello" is 7 bytes
		t.Errorf("consumed %d, want 7", n)
	}
}

func TestConsumeLiteralN3_WithLang(t *testing.T) {
	n := consumeLiteralN3(`"hello"@en rest`)
	if n != 10 { // "hello"@en
		t.Errorf("consumed %d, want 10", n)
	}
}

func TestConsumeLiteralN3_WithDatatype(t *testing.T) {
	input := `"42"^^<http://www.w3.org/2001/XMLSchema#integer> rest`
	n := consumeLiteralN3(input)
	expected := len(`"42"^^<http://www.w3.org/2001/XMLSchema#integer>`)
	if n != expected {
		t.Errorf("consumed %d, want %d", n, expected)
	}
}

func TestConsumeLiteralN3_TripleQuoted(t *testing.T) {
	input := `"""hello world""" rest`
	n := consumeLiteralN3(input)
	if n != 17 { // """hello world"""
		t.Errorf("consumed %d, want 17", n)
	}
}

func TestConsumeLiteralN3_TripleQuotedWithLang(t *testing.T) {
	input := `"""hello"""@en rest`
	n := consumeLiteralN3(input)
	if n != 14 { // """hello"""@en
		t.Errorf("consumed %d, want 14", n)
	}
}

func TestConsumeLiteralN3_Empty(t *testing.T) {
	n := consumeLiteralN3("")
	if n != 0 {
		t.Errorf("consumed %d, want 0", n)
	}
}

func TestConsumeLiteralN3_NotQuoted(t *testing.T) {
	n := consumeLiteralN3("not-quoted")
	if n != 0 {
		t.Errorf("consumed %d, want 0", n)
	}
}

func TestConsumeLiteralN3_UnterminatedSingle(t *testing.T) {
	n := consumeLiteralN3(`"unterminated`)
	if n != 13 { // returns len(s)
		t.Errorf("consumed %d, want 13", n)
	}
}

func TestConsumeLiteralN3_UnterminatedTriple(t *testing.T) {
	n := consumeLiteralN3(`"""unterminated`)
	if n != 15 { // returns len(s)
		t.Errorf("consumed %d, want 15", n)
	}
}

func TestConsumeLiteralN3_EscapedQuote(t *testing.T) {
	input := `"say \"hi\"" rest`
	n := consumeLiteralN3(input)
	if n != 12 { // "say \"hi\""
		t.Errorf("consumed %d, want 12", n)
	}
}

func TestConsumeLiteralN3_WithDatatypeNoCloseBracket(t *testing.T) {
	// ^^< without closing > — should still consume what it can
	input := `"x"^^<http://no-close`
	n := consumeLiteralN3(input)
	// ^^< found but no >, so just i stays at position after ^^
	if n != 5 { // "x"^^ — 5 bytes consumed (just the ^^ part, < not consumed without >)
		t.Errorf("consumed %d, want 5", n)
	}
}

// =============================================================================
// decode.go — unescapeLiteral
// =============================================================================

func TestUnescapeLiteral_NoEscapes(t *testing.T) {
	if got := unescapeLiteral("hello"); got != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestUnescapeLiteral_Quote(t *testing.T) {
	if got := unescapeLiteral(`say \"hi\"`); got != `say "hi"` {
		t.Errorf("got %q", got)
	}
}

func TestUnescapeLiteral_Backslash(t *testing.T) {
	if got := unescapeLiteral(`a\\b`); got != `a\b` {
		t.Errorf("got %q", got)
	}
}

func TestUnescapeLiteral_Newline(t *testing.T) {
	if got := unescapeLiteral(`line1\nline2`); got != "line1\nline2" {
		t.Errorf("got %q", got)
	}
}

func TestUnescapeLiteral_CarriageReturn(t *testing.T) {
	if got := unescapeLiteral(`line1\rline2`); got != "line1\rline2" {
		t.Errorf("got %q", got)
	}
}

func TestUnescapeLiteral_Tab(t *testing.T) {
	if got := unescapeLiteral(`col1\tcol2`); got != "col1\tcol2" {
		t.Errorf("got %q", got)
	}
}

func TestUnescapeLiteral_UnknownEscape(t *testing.T) {
	// Unknown escape sequences should pass through the backslash
	if got := unescapeLiteral(`\x`); got != `\x` {
		t.Errorf("got %q, want %q", got, `\x`)
	}
}

func TestUnescapeLiteral_TrailingBackslash(t *testing.T) {
	// Trailing backslash without following char
	if got := unescapeLiteral(`end\`); got != `end\` {
		t.Errorf("got %q", got)
	}
}

func TestUnescapeLiteral_AllEscapes(t *testing.T) {
	input := `\"\\\n\r\t`
	want := "\"\\\n\r\t"
	if got := unescapeLiteral(input); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// =============================================================================
// decode.go — TermFromKey with TripleTerm
// =============================================================================

func TestTermFromKey_TripleTermN3(t *testing.T) {
	// TermFromKey with T: prefix expects N3 format, not the internal key format
	key := "T:<<( <http://example.org/s> <http://example.org/p> <http://example.org/o> )>>"
	decoded, err := TermFromKey(key)
	if err != nil {
		t.Fatalf("TermFromKey: %v", err)
	}
	tt, ok := decoded.(TripleTerm)
	if !ok {
		t.Fatalf("expected TripleTerm, got %T", decoded)
	}
	if tt.Subject().(URIRef).Value() != "http://example.org/s" {
		t.Errorf("subject: %v", tt.Subject())
	}
}

func TestTermFromKey_TripleTermWithLiteralObject(t *testing.T) {
	key := `T:<<( <http://example.org/s> <http://example.org/p> "hello"@en )>>`
	decoded, err := TermFromKey(key)
	if err != nil {
		t.Fatalf("TermFromKey: %v", err)
	}
	tt, ok := decoded.(TripleTerm)
	if !ok {
		t.Fatalf("expected TripleTerm, got %T", decoded)
	}
	lit, ok := tt.Object().(Literal)
	if !ok {
		t.Fatalf("expected Literal object, got %T", tt.Object())
	}
	if lit.Language() != "en" {
		t.Errorf("lang: %q", lit.Language())
	}
}

func TestTermFromKey_TripleTermInvalid(t *testing.T) {
	_, err := TermFromKey("T:not-valid")
	if err == nil {
		t.Error("expected error for invalid triple term key")
	}
}

// =============================================================================
// decode.go — literalFromN3 edge cases
// =============================================================================

func TestLiteralFromN3_DoubleShorthand(t *testing.T) {
	// "1.5e10" contains both "." and "eE", but literalFromN3 checks "." first → decimal
	lit, err := literalFromN3("1.5e10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lit.Datatype() != XSDDecimal {
		t.Errorf("expected xsd:decimal, got %v", lit.Datatype())
	}
	// Pure exponent form without dot → double
	lit2, err := literalFromN3("15e10")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lit2.Datatype() != XSDDouble {
		t.Errorf("expected xsd:double, got %v", lit2.Datatype())
	}
}

func TestLiteralFromN3_NegativeInteger(t *testing.T) {
	lit, err := literalFromN3("-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lit.Lexical() != "-42" {
		t.Errorf("lexical: %q", lit.Lexical())
	}
	if lit.Datatype() != XSDInteger {
		t.Errorf("datatype: %v", lit.Datatype())
	}
}

func TestLiteralFromN3_PositiveDecimal(t *testing.T) {
	lit, err := literalFromN3("+3.14")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lit.Datatype() != XSDDecimal {
		t.Errorf("datatype: %v", lit.Datatype())
	}
}

func TestLiteralFromN3_DirLangLiteral(t *testing.T) {
	lit, err := literalFromN3(`"hello"@en--ltr`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lit.Language() != "en" {
		t.Errorf("lang: %q", lit.Language())
	}
	if lit.Dir() != "ltr" {
		t.Errorf("dir: %q", lit.Dir())
	}
}

func TestLiteralFromN3_TripleQuoted(t *testing.T) {
	lit, err := literalFromN3(`"""hello\nworld"""`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lit.Lexical() != "hello\nworld" {
		t.Errorf("lexical: %q", lit.Lexical())
	}
}

func TestLiteralFromN3_TripleQuotedUnterminated(t *testing.T) {
	_, err := literalFromN3(`"""unterminated`)
	if err == nil {
		t.Error("expected error for unterminated triple-quoted literal")
	}
}

func TestLiteralFromN3_SingleQuotedUnterminated(t *testing.T) {
	_, err := literalFromN3(`"unterminated`)
	if err == nil {
		t.Error("expected error for unterminated literal")
	}
}

func TestLiteralFromN3_InvalidDatatypeSuffix(t *testing.T) {
	_, err := literalFromN3(`"x"^^invalid`)
	if err == nil {
		t.Error("expected error for invalid datatype suffix")
	}
}

func TestLiteralFromN3_PlainStringLiteral(t *testing.T) {
	lit, err := literalFromN3(`"hello"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if lit.Lexical() != "hello" {
		t.Errorf("lexical: %q", lit.Lexical())
	}
	if lit.Datatype() != XSDString {
		t.Errorf("datatype: %v", lit.Datatype())
	}
}

// =============================================================================
// key.go — TermKey default branch
// =============================================================================

// customTerm is a fake Term to hit the default branch of TermKey.
// This should never happen in practice because termType() is sealed,
// but we test the fallback for robustness.
type customTerm struct{}

func (c customTerm) N3(ns ...NamespaceManager) string { return "<custom>" }
func (c customTerm) String() string                    { return "custom" }
func (c customTerm) Equal(other Term) bool             { return false }
func (c customTerm) termType() string                  { return "Custom" }

func TestTermKey_DefaultBranch(t *testing.T) {
	ct := customTerm{}
	k := TermKey(ct)
	if k != "<custom>" {
		t.Errorf("default branch: %q", k)
	}
}

// =============================================================================
// order.go — termTypeOrder for Variable and default
// =============================================================================

func TestTermTypeOrder_Variable(t *testing.T) {
	v := NewVariable("x")
	order := termTypeOrder(v)
	if order != 4 {
		t.Errorf("Variable order: %d, want 4", order)
	}
}

func TestTermTypeOrder_Default(t *testing.T) {
	ct := customTerm{}
	order := termTypeOrder(ct)
	if order != 5 {
		t.Errorf("default order: %d, want 5", order)
	}
}

// =============================================================================
// triple_term.go — termType() and subject() via interface
// =============================================================================

func TestTripleTermTermType(t *testing.T) {
	s := NewURIRefUnsafe("http://example.org/s")
	p := NewURIRefUnsafe("http://example.org/p")
	o := NewURIRefUnsafe("http://example.org/o")
	tt := NewTripleTerm(s, p, o)
	if tt.termType() != "TripleTerm" {
		t.Errorf("termType: %q", tt.termType())
	}
}

func TestTripleTermSubjectMarker(t *testing.T) {
	s := NewURIRefUnsafe("http://example.org/s")
	p := NewURIRefUnsafe("http://example.org/p")
	o := NewURIRefUnsafe("http://example.org/o")
	tt := NewTripleTerm(s, p, o)
	// TripleTerm implements Subject; call subject() for coverage.
	tt.subject()
	// Also verify it satisfies the Subject interface.
	var subj Subject = tt
	_ = subj
}

// =============================================================================
// term.go — NewURIRefWithBase edge cases
// =============================================================================

func TestNewURIRefWithBase_RelativeResolution(t *testing.T) {
	u, err := NewURIRefWithBase("bar", "http://example.org/foo/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Value() != "http://example.org/foo/bar" {
		t.Errorf("resolved: %q", u.Value())
	}
}

func TestNewURIRefWithBase_AbsoluteValue(t *testing.T) {
	u, err := NewURIRefWithBase("http://other.org/x", "http://example.org/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Value() != "http://other.org/x" {
		t.Errorf("resolved: %q", u.Value())
	}
}

func TestNewURIRefWithBase_EmptyBase(t *testing.T) {
	u, err := NewURIRefWithBase("http://example.org/x", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Value() != "http://example.org/x" {
		t.Errorf("resolved: %q", u.Value())
	}
}

func TestNewURIRefWithBase_FragmentResolution(t *testing.T) {
	u, err := NewURIRefWithBase("#frag", "http://example.org/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Value() != "http://example.org/page#frag" {
		t.Errorf("resolved: %q", u.Value())
	}
}

func TestNewURIRefWithBase_QueryResolution(t *testing.T) {
	u, err := NewURIRefWithBase("?query=1", "http://example.org/page")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Value() != "http://example.org/page?query=1" {
		t.Errorf("resolved: %q", u.Value())
	}
}

// =============================================================================
// literal.go — valueEqual remaining branch (float64)
// =============================================================================

func TestValueEqual_Float64(t *testing.T) {
	a := NewLiteral(1.5)
	b := NewLiteral(1.5)
	if !a.ValueEqual(b) {
		t.Error("float64 1.5 should value-equal 1.5")
	}
	c := NewLiteral(2.5)
	if a.ValueEqual(c) {
		t.Error("float64 1.5 should not value-equal 2.5")
	}
}

// =============================================================================
// decode.go — findClosingQuote edge cases
// =============================================================================

func TestFindClosingQuote_Escaped(t *testing.T) {
	// Escaped quotes should be skipped
	idx := findClosingQuote(`hello\"world"`)
	if idx != 12 { // index of the unescaped quote
		t.Errorf("got %d, want 12", idx)
	}
}

func TestFindClosingQuote_NoClose(t *testing.T) {
	idx := findClosingQuote(`no closing quote here`)
	if idx != -1 {
		t.Errorf("got %d, want -1", idx)
	}
}

func TestFindClosingQuote_Immediate(t *testing.T) {
	idx := findClosingQuote(`"rest`)
	if idx != 0 {
		t.Errorf("got %d, want 0", idx)
	}
}

// =============================================================================
// Round-trip: TermKey → TermFromKey for all term types
// =============================================================================

func TestTermRoundTrip_AllTypes(t *testing.T) {
	terms := []Term{
		NewURIRefUnsafe("http://example.org/test"),
		NewBNode("node1"),
		NewLiteral("hello"),
		NewLiteral("bonjour", WithLang("fr")),
		NewLiteral("hello", WithLang("en"), WithDir("ltr")),
		NewLiteral(42),
		NewLiteral(3.14),
		NewLiteral(true),
		NewLiteral("text with \"quotes\""),
		NewLiteral("line1\nline2"),
		NewLiteral("tabs\there"),
	}
	for _, original := range terms {
		key := TermKey(original)
		decoded, err := TermFromKey(key)
		if err != nil {
			t.Errorf("TermFromKey(%q): %v", key, err)
			continue
		}
		if !original.Equal(decoded) {
			t.Errorf("round-trip failed: %s -> %q -> %s", original.N3(), key, decoded.N3())
		}
	}
}
