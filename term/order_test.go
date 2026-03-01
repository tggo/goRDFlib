package term

import (
	"sort"
	"strings"
	"testing"
)

// --- Fix 1: Triple-quote escaping bug in Literal.N3() ---

func TestLiteralN3TripleQuoteInLexical(t *testing.T) {
	// A lexical value containing """ should produce valid N3.
	l := NewLiteral("before\"\"\"after\nwith newline")
	n3 := l.N3()
	// The result must be parseable: the inner """ must be escaped so it
	// doesn't prematurely close the triple-quoted string.
	if strings.Count(n3, `"""`) != 2 {
		// Exactly two occurrences: opening and closing delimiters.
		t.Errorf("expected exactly 2 triple-quote delimiters, got N3: %s", n3)
	}
	// Must start with """ and end with """
	if !strings.HasPrefix(n3, `"""`) || !strings.HasSuffix(n3, `"""`) {
		t.Errorf("N3 should be triple-quoted, got: %s", n3)
	}
}

func TestLiteralN3ConsecutiveQuotesNoNewline(t *testing.T) {
	// Without newline, single-quoting is used: quotes get escaped normally.
	l := NewLiteral(`he said """hello"""`)
	n3 := l.N3()
	if !strings.HasPrefix(n3, `"`) {
		t.Errorf("expected single-quoted, got: %s", n3)
	}
}

func TestLiteralN3TripleQuoteOnly(t *testing.T) {
	// Edge case: lexical is exactly """
	l := NewLiteral("\"\"\"\n")
	n3 := l.N3()
	// Must produce valid triple-quoted output.
	if !strings.HasPrefix(n3, `"""`) || !strings.HasSuffix(n3, `"""`) {
		t.Errorf("expected triple-quoted output, got: %s", n3)
	}
	// The inner content must not contain an unescaped """ sequence.
	inner := n3[3 : len(n3)-3]
	if strings.Contains(inner, `"""`) {
		t.Errorf("inner content has unescaped triple-quote: %s", inner)
	}
}

// --- Fix 2: CompareTerm type-aware for Literals ---

func TestCompareTermNumericLiterals(t *testing.T) {
	l1 := NewLiteral("1", WithDatatype(XSDInteger))
	l2 := NewLiteral("2", WithDatatype(XSDInteger))
	l10 := NewLiteral("10", WithDatatype(XSDInteger))

	if CompareTerm(l1, l2) >= 0 {
		t.Error("1 should sort before 2")
	}
	if CompareTerm(l2, l10) >= 0 {
		t.Error("2 should sort before 10 numerically")
	}
	if CompareTerm(l1, l1) != 0 {
		t.Error("same literal should compare equal")
	}
}

func TestCompareTermFloatLiterals(t *testing.T) {
	a := NewLiteral("1.5", WithDatatype(XSDDouble))
	b := NewLiteral("2.3", WithDatatype(XSDDouble))
	if CompareTerm(a, b) >= 0 {
		t.Error("1.5 should sort before 2.3")
	}
}

func TestCompareTermStringLiterals(t *testing.T) {
	a := NewLiteral("apple")
	b := NewLiteral("banana")
	if CompareTerm(a, b) >= 0 {
		t.Error("apple should sort before banana")
	}
}

// --- Fix 3: Deduplicated GoToLexical / NewLiteral ---

func TestGoToLexicalAndNewLiteralConsistency(t *testing.T) {
	// Verify that GoToLexical and NewLiteral produce the same lexical form.
	values := []any{42, int64(99), float32(1.5), 3.14, true, false, "hello"}
	for _, v := range values {
		lex, dt := GoToLexical(v)
		lit := NewLiteral(v)
		if lit.Lexical() != lex {
			t.Errorf("GoToLexical(%v) lexical=%q but NewLiteral lexical=%q", v, lex, lit.Lexical())
		}
		if lit.Datatype() != dt {
			t.Errorf("GoToLexical(%v) dt=%v but NewLiteral dt=%v", v, dt, lit.Datatype())
		}
	}
}

// --- Fix 4: MustURIRef ---

func TestMustURIRef(t *testing.T) {
	u := MustURIRef("http://example.org/test")
	if u.Value() != "http://example.org/test" {
		t.Errorf("got %q", u.Value())
	}
	// Should produce the same result as NewURIRefUnsafe.
	u2 := NewURIRefUnsafe("http://example.org/test")
	if u != u2 {
		t.Error("MustURIRef and NewURIRefUnsafe should produce identical results")
	}
}

// --- Fix 5: sort.Interface helper (TermSlice, SortTerms) ---

func TestTermSliceSortInterface(t *testing.T) {
	u1, _ := NewURIRef("http://example.org/b")
	u2, _ := NewURIRef("http://example.org/a")
	b := NewBNode("x")
	l := NewLiteral("hello")

	terms := TermSlice{l, u1, b, u2}
	sort.Sort(terms)

	// Expected order: BNode < URIRef(a) < URIRef(b) < Literal
	if _, ok := terms[0].(BNode); !ok {
		t.Errorf("index 0 should be BNode, got %T", terms[0])
	}
	if terms[1].(URIRef).Value() != "http://example.org/a" {
		t.Errorf("index 1 should be URIRef(a), got %s", terms[1])
	}
	if terms[2].(URIRef).Value() != "http://example.org/b" {
		t.Errorf("index 2 should be URIRef(b), got %s", terms[2])
	}
	if _, ok := terms[3].(Literal); !ok {
		t.Errorf("index 3 should be Literal, got %T", terms[3])
	}
}

func TestSortTerms(t *testing.T) {
	u, _ := NewURIRef("http://example.org/a")
	b := NewBNode("x")
	terms := []Term{u, b}
	SortTerms(terms)
	if _, ok := terms[0].(BNode); !ok {
		t.Error("BNode should sort first")
	}
}

func TestSortTermsNumeric(t *testing.T) {
	l1 := NewLiteral("10", WithDatatype(XSDInteger))
	l2 := NewLiteral("2", WithDatatype(XSDInteger))
	l3 := NewLiteral("1", WithDatatype(XSDInteger))
	terms := []Term{l1, l2, l3}
	SortTerms(terms)
	// Should be: 1, 2, 10 (numeric order, not lexicographic)
	expected := []string{"1", "2", "10"}
	for i, e := range expected {
		if terms[i].(Literal).Lexical() != e {
			t.Errorf("index %d: expected %s, got %s", i, e, terms[i].(Literal).Lexical())
		}
	}
}

// --- Fix 6: ValueEqual (renamed from Eq) ---

func TestLiteralValueEqual(t *testing.T) {
	a := NewLiteral("1", WithDatatype(XSDInteger))
	b := NewLiteral("01", WithDatatype(XSDInteger))
	if !a.ValueEqual(b) {
		t.Error("ValueEqual should match '1' and '01' as integers")
	}
}

func TestLiteralValueEqualFloat(t *testing.T) {
	a := NewLiteral("1.0", WithDatatype(XSDDouble))
	b := NewLiteral("1.00", WithDatatype(XSDDouble))
	if !a.ValueEqual(b) {
		t.Error("ValueEqual should match 1.0 and 1.00")
	}
}

func TestLiteralValueEqualDifferentDatatype(t *testing.T) {
	a := NewLiteral("1", WithDatatype(XSDInteger))
	b := NewLiteral("1", WithDatatype(XSDString))
	if a.ValueEqual(b) {
		t.Error("different datatypes should not ValueEqual")
	}
}

func TestLiteralEqStillWorks(t *testing.T) {
	// Verify backward compatibility of deprecated Eq.
	a := NewLiteral("1", WithDatatype(XSDInteger))
	b := NewLiteral("01", WithDatatype(XSDInteger))
	if !a.Eq(b) {
		t.Error("Eq (deprecated) should still work")
	}
}

// --- Fix 7: BNode.Skolemize with basepath ---

func TestBNodeSkolemizeWithBasepath(t *testing.T) {
	b := NewBNode("abc")
	s := b.Skolemize("http://example.org", "custom/path")
	if s.Value() != "http://example.org/custom/path/abc" {
		t.Errorf("got %q", s.Value())
	}
}

func TestBNodeSkolemizeWithBasepathTrailingSlash(t *testing.T) {
	b := NewBNode("abc")
	s := b.Skolemize("http://example.org/", "custom/path/")
	if s.Value() != "http://example.org/custom/path/abc" {
		t.Errorf("got %q", s.Value())
	}
}

func TestBNodeSkolemizeDefaultBasepath(t *testing.T) {
	// Without basepath, should use default .well-known/genid/
	b := NewBNode("abc")
	s := b.Skolemize("http://example.org")
	if s.Value() != "http://example.org/.well-known/genid/abc" {
		t.Errorf("got %q", s.Value())
	}
}
