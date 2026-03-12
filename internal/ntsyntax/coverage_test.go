package ntsyntax

import (
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// --- LineParser tests ---

func TestLineParserReadSubjectIRI(t *testing.T) {
	p := &LineParser{Line: `<http://example.org/s> <http://example.org/p> "v" .`, LineNum: 1}
	s, err := p.ReadSubject()
	if err != nil {
		t.Fatal(err)
	}
	if s.N3() != "<http://example.org/s>" {
		t.Errorf("got %s", s.N3())
	}
}

func TestLineParserReadSubjectBNode(t *testing.T) {
	p := &LineParser{Line: `_:b1 <http://example.org/p> "v" .`, LineNum: 1}
	s, err := p.ReadSubject()
	if err != nil {
		t.Fatal(err)
	}
	if s.N3() != "_:b1" {
		t.Errorf("got %s", s.N3())
	}
}

func TestLineParserReadSubjectEOF(t *testing.T) {
	p := &LineParser{Line: "", LineNum: 1}
	_, err := p.ReadSubject()
	if err == nil {
		t.Error("expected error")
	}
}

func TestLineParserReadSubjectInvalid(t *testing.T) {
	p := &LineParser{Line: `"literal"`, LineNum: 1}
	_, err := p.ReadSubject()
	if err == nil {
		t.Error("expected error for literal as subject")
	}
}

func TestLineParserReadObjectIRI(t *testing.T) {
	p := &LineParser{Line: `<http://example.org/o> .`, LineNum: 1}
	o, err := p.ReadObject()
	if err != nil {
		t.Fatal(err)
	}
	if o.N3() != "<http://example.org/o>" {
		t.Errorf("got %s", o.N3())
	}
}

func TestLineParserReadObjectBNode(t *testing.T) {
	p := &LineParser{Line: `_:b1 .`, LineNum: 1}
	o, err := p.ReadObject()
	if err != nil {
		t.Fatal(err)
	}
	if o.N3() != "_:b1" {
		t.Errorf("got %s", o.N3())
	}
}

func TestLineParserReadObjectLiteral(t *testing.T) {
	p := &LineParser{Line: `"hello" .`, LineNum: 1}
	o, err := p.ReadObject()
	if err != nil {
		t.Fatal(err)
	}
	if o.String() != "hello" {
		t.Errorf("got %s", o.String())
	}
}

func TestLineParserReadObjectTripleTerm(t *testing.T) {
	p := &LineParser{Line: `<<( <http://example.org/s> <http://example.org/p> <http://example.org/o> )>> .`, LineNum: 1}
	o, err := p.ReadObject()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := o.(rdflibgo.TripleTerm); !ok {
		t.Errorf("expected TripleTerm, got %T", o)
	}
}

func TestLineParserReadObjectEOF(t *testing.T) {
	p := &LineParser{Line: "", LineNum: 1}
	_, err := p.ReadObject()
	if err == nil {
		t.Error("expected error")
	}
}

func TestLineParserReadObjectInvalid(t *testing.T) {
	p := &LineParser{Line: `@ .`, LineNum: 1}
	_, err := p.ReadObject()
	if err == nil {
		t.Error("expected error")
	}
}

func TestLineParserReadPredicate(t *testing.T) {
	p := &LineParser{Line: `<http://example.org/p> `, LineNum: 1}
	pred, err := p.ReadPredicate()
	if err != nil {
		t.Fatal(err)
	}
	if pred.Value() != "http://example.org/p" {
		t.Errorf("got %s", pred.Value())
	}
}

func TestLineParserReadGraphLabelIRI(t *testing.T) {
	p := &LineParser{Line: `<http://example.org/g> .`, LineNum: 1}
	g, err := p.ReadGraphLabel()
	if err != nil {
		t.Fatal(err)
	}
	if g == nil || g.N3() != "<http://example.org/g>" {
		t.Errorf("got %v", g)
	}
}

func TestLineParserReadGraphLabelBNode(t *testing.T) {
	p := &LineParser{Line: `_:g1 .`, LineNum: 1}
	g, err := p.ReadGraphLabel()
	if err != nil {
		t.Fatal(err)
	}
	if g == nil || g.N3() != "_:g1" {
		t.Errorf("got %v", g)
	}
}

func TestLineParserReadGraphLabelNone(t *testing.T) {
	p := &LineParser{Line: `.`, LineNum: 1}
	g, err := p.ReadGraphLabel()
	if err != nil {
		t.Fatal(err)
	}
	if g != nil {
		t.Errorf("expected nil, got %v", g)
	}
}

func TestLineParserReadGraphLabelInvalid(t *testing.T) {
	p := &LineParser{Line: `"lit" .`, LineNum: 1}
	_, err := p.ReadGraphLabel()
	if err == nil {
		t.Error("expected error")
	}
}

func TestLineParserReadIRI(t *testing.T) {
	p := &LineParser{Line: `<http://example.org/path>`, LineNum: 1}
	iri, err := p.ReadIRI()
	if err != nil {
		t.Fatal(err)
	}
	if iri != "http://example.org/path" {
		t.Errorf("got %s", iri)
	}
}

func TestLineParserReadIRIWithEscape(t *testing.T) {
	p := &LineParser{Line: `<http://example.org/\u0041>`, LineNum: 1}
	iri, err := p.ReadIRI()
	if err != nil {
		t.Fatal(err)
	}
	if iri != "http://example.org/A" {
		t.Errorf("got %s", iri)
	}
}

func TestLineParserReadIRIUnterminated(t *testing.T) {
	p := &LineParser{Line: `<http://example.org/path`, LineNum: 1}
	_, err := p.ReadIRI()
	if err == nil {
		t.Error("expected error")
	}
}

func TestLineParserReadIRINoAngle(t *testing.T) {
	p := &LineParser{Line: `http://example.org/`, LineNum: 1}
	_, err := p.ReadIRI()
	if err == nil {
		t.Error("expected error")
	}
}

func TestLineParserReadBNode(t *testing.T) {
	p := &LineParser{Line: `_:b1 `, Pos: 0, LineNum: 1}
	b, err := p.ReadBNode()
	if err != nil {
		t.Fatal(err)
	}
	if b.Value() != "b1" {
		t.Errorf("got %s", b.Value())
	}
}

func TestLineParserReadBNodeDotInMiddle(t *testing.T) {
	p := &LineParser{Line: `_:b.1 `, Pos: 0, LineNum: 1}
	b, err := p.ReadBNode()
	if err != nil {
		t.Fatal(err)
	}
	if b.Value() != "b.1" {
		t.Errorf("got %s", b.Value())
	}
}

func TestLineParserReadBNodeTrailingDot(t *testing.T) {
	p := &LineParser{Line: `_:b1. `, Pos: 0, LineNum: 1}
	b, err := p.ReadBNode()
	if err != nil {
		t.Fatal(err)
	}
	if b.Value() != "b1" {
		t.Errorf("trailing dot should be trimmed, got %s", b.Value())
	}
}

func TestLineParserReadBNodeEmpty(t *testing.T) {
	p := &LineParser{Line: `_:`, Pos: 0, LineNum: 1}
	_, err := p.ReadBNode()
	if err == nil {
		t.Error("expected error for empty bnode label")
	}
}

func TestLineParserReadBNodeInvalidStart(t *testing.T) {
	p := &LineParser{Line: `_:!bad`, Pos: 0, LineNum: 1}
	_, err := p.ReadBNode()
	if err == nil {
		t.Error("expected error for invalid bnode start char")
	}
}

func TestLineParserReadLiteralPlain(t *testing.T) {
	p := &LineParser{Line: `"hello" .`, Pos: 0, LineNum: 1}
	l, err := p.ReadLiteral()
	if err != nil {
		t.Fatal(err)
	}
	if l.Lexical() != "hello" {
		t.Errorf("got %s", l.Lexical())
	}
}

func TestLineParserReadLiteralLang(t *testing.T) {
	p := &LineParser{Line: `"hello"@en .`, Pos: 0, LineNum: 1}
	l, err := p.ReadLiteral()
	if err != nil {
		t.Fatal(err)
	}
	if l.Language() != "en" {
		t.Errorf("got lang %s", l.Language())
	}
}

func TestLineParserReadLiteralDirLang(t *testing.T) {
	p := &LineParser{Line: `"hello"@en--ltr .`, Pos: 0, LineNum: 1}
	l, err := p.ReadLiteral()
	if err != nil {
		t.Fatal(err)
	}
	if l.Language() != "en" || l.Dir() != "ltr" {
		t.Errorf("got lang=%s dir=%s", l.Language(), l.Dir())
	}
}

func TestLineParserReadLiteralDirInvalid(t *testing.T) {
	p := &LineParser{Line: `"hello"@en--up .`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error for invalid direction")
	}
}

func TestLineParserReadLiteralDatatype(t *testing.T) {
	p := &LineParser{Line: `"42"^^<http://www.w3.org/2001/XMLSchema#integer> .`, Pos: 0, LineNum: 1}
	l, err := p.ReadLiteral()
	if err != nil {
		t.Fatal(err)
	}
	if l.Datatype().Value() != "http://www.w3.org/2001/XMLSchema#integer" {
		t.Errorf("got dt %s", l.Datatype().Value())
	}
}

func TestLineParserReadLiteralEscapes(t *testing.T) {
	p := &LineParser{Line: `"a\nb\t\\\"" .`, Pos: 0, LineNum: 1}
	l, err := p.ReadLiteral()
	if err != nil {
		t.Fatal(err)
	}
	if l.Lexical() != "a\nb\t\\\"" {
		t.Errorf("got %q", l.Lexical())
	}
}

func TestLineParserReadLiteralUnicodeEscapes(t *testing.T) {
	p := &LineParser{Line: `"\u0041\U00000042" .`, Pos: 0, LineNum: 1}
	l, err := p.ReadLiteral()
	if err != nil {
		t.Fatal(err)
	}
	if l.Lexical() != "AB" {
		t.Errorf("got %q", l.Lexical())
	}
}

func TestLineParserReadLiteralUnterminated(t *testing.T) {
	p := &LineParser{Line: `"hello`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error")
	}
}

func TestLineParserReadLiteralUnknownEscape(t *testing.T) {
	p := &LineParser{Line: `"a\q" .`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error for unknown escape")
	}
}

func TestLineParserReadLiteralEscapeBackspace(t *testing.T) {
	p := &LineParser{Line: `"\b\f\r" .`, Pos: 0, LineNum: 1}
	l, err := p.ReadLiteral()
	if err != nil {
		t.Fatal(err)
	}
	if l.Lexical() != "\b\f\r" {
		t.Errorf("got %q", l.Lexical())
	}
}

func TestLineParserSkipSpaces(t *testing.T) {
	p := &LineParser{Line: "   \t\thello", LineNum: 1}
	p.SkipSpaces()
	if p.Pos != 5 {
		t.Errorf("expected pos 5, got %d", p.Pos)
	}
}

func TestLineParserExpect(t *testing.T) {
	p := &LineParser{Line: "abc", LineNum: 1}
	if !p.Expect('a') {
		t.Error("expected true")
	}
	if p.Expect('c') {
		t.Error("expected false for non-matching")
	}
}

func TestLineParserExpectEOF(t *testing.T) {
	p := &LineParser{Line: "", LineNum: 1}
	if p.Expect('a') {
		t.Error("expected false at EOF")
	}
}

// --- Serializer tests ---

func TestTermURIRef(t *testing.T) {
	u, _ := rdflibgo.NewURIRef("http://example.org/s")
	s, err := Term(u)
	if err != nil {
		t.Fatal(err)
	}
	if s != "<http://example.org/s>" {
		t.Errorf("got %s", s)
	}
}

func TestTermBNode(t *testing.T) {
	b := rdflibgo.NewBNode("b1")
	s, err := Term(b)
	if err != nil {
		t.Fatal(err)
	}
	if s != "_:b1" {
		t.Errorf("got %s", s)
	}
}

func TestTermLiteral(t *testing.T) {
	l := rdflibgo.NewLiteral("hello")
	s, err := Term(l)
	if err != nil {
		t.Fatal(err)
	}
	if s != `"hello"` {
		t.Errorf("got %s", s)
	}
}

func TestTermTripleTerm(t *testing.T) {
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	o, _ := rdflibgo.NewURIRef("http://example.org/o")
	tt := rdflibgo.NewTripleTerm(s, p, o)
	result, err := Term(tt)
	if err != nil {
		t.Fatal(err)
	}
	if result != "<<( <http://example.org/s> <http://example.org/p> <http://example.org/o> )>>" {
		t.Errorf("got %s", result)
	}
}

func TestLiteralWithLang(t *testing.T) {
	l := rdflibgo.NewLiteral("hello", rdflibgo.WithLang("en"))
	s := Literal(l)
	if s != `"hello"@en` {
		t.Errorf("got %s", s)
	}
}

func TestLiteralWithDirLang(t *testing.T) {
	l := rdflibgo.NewLiteral("hello", rdflibgo.WithLang("ar"), rdflibgo.WithDir("rtl"))
	s := Literal(l)
	if s != `"hello"@ar--rtl` {
		t.Errorf("got %s", s)
	}
}

func TestLiteralWithDatatype(t *testing.T) {
	l := rdflibgo.NewLiteral("42", rdflibgo.WithDatatype(rdflibgo.XSDInteger))
	s := Literal(l)
	expected := `"42"^^<http://www.w3.org/2001/XMLSchema#integer>`
	if s != expected {
		t.Errorf("got %s", s)
	}
}

func TestPadHex8(t *testing.T) {
	got := padHex8(0x1F600)
	if got != "0001F600" {
		t.Errorf("got %s", got)
	}
}

func TestEscapeStringSupplementary(t *testing.T) {
	// Supplementary character (emoji) should be \U escaped
	got := EscapeString("\U0001F600")
	if got != `\U0001F600` {
		t.Errorf("got %q", got)
	}
}

func TestEscapeIRISupplementary(t *testing.T) {
	got := EscapeIRI("http://example.org/\U0001F600")
	if got != `http://example.org/\U0001F600` {
		t.Errorf("got %q", got)
	}
}

// isValidLangTag
func TestLineParserReadLiteralInvalidLangEmpty(t *testing.T) {
	p := &LineParser{Line: `"hello"@ .`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error for empty lang tag")
	}
}

// isAbsoluteIRI
func TestLineParserRelativeSubjectIRI(t *testing.T) {
	p := &LineParser{Line: `<relative> <http://example.org/p> "v" .`, LineNum: 1}
	_, err := p.ReadSubject()
	if err == nil {
		t.Error("expected error for relative IRI")
	}
}

// ReadIRI invalid char
func TestLineParserReadIRIInvalidChar(t *testing.T) {
	p := &LineParser{Line: "<http://example.org/\x01> ", LineNum: 1}
	_, err := p.ReadIRI()
	if err == nil {
		t.Error("expected error for control char in IRI")
	}
}

// ReadIRI unterminated escape
func TestLineParserReadIRIUnterminatedEscape(t *testing.T) {
	p := &LineParser{Line: `<http://example.org/\`, LineNum: 1}
	_, err := p.ReadIRI()
	if err == nil {
		t.Error("expected error")
	}
}

// Literal: truncated \u
func TestLineParserReadLiteralTruncatedU(t *testing.T) {
	p := &LineParser{Line: `"\u00"`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error")
	}
}

// Literal: truncated \U
func TestLineParserReadLiteralTruncatedBigU(t *testing.T) {
	p := &LineParser{Line: `"\U0000"`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error")
	}
}

// Literal: invalid hex in \u
func TestLineParserReadLiteralInvalidHexU(t *testing.T) {
	p := &LineParser{Line: `"\uZZZZ"`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error")
	}
}

// Literal: surrogate in \u
func TestLineParserReadLiteralSurrogateU(t *testing.T) {
	p := &LineParser{Line: `"\uD800"`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error")
	}
}

// Literal: surrogate in \U
func TestLineParserReadLiteralSurrogateBigU(t *testing.T) {
	p := &LineParser{Line: `"\U0000D800"`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error")
	}
}

// Literal: unterminated escape at end
func TestLineParserReadLiteralUnterminatedEscape(t *testing.T) {
	p := &LineParser{Line: `"\`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error")
	}
}

// Literal: invalid lang tag
func TestLineParserReadLiteralInvalidLang(t *testing.T) {
	p := &LineParser{Line: `"hello"@en--invalid .`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error for invalid base direction")
	}
}

// ReadTripleTerm error paths
func TestReadTripleTermNoOpenParen(t *testing.T) {
	p := &LineParser{Line: `<< <http://example.org/s> `, LineNum: 1}
	_, err := p.ReadTripleTerm()
	if err == nil {
		t.Error("expected error: no '(' after '<<'")
	}
}

func TestReadTripleTermBadSubject(t *testing.T) {
	p := &LineParser{Line: `<<( "bad" `, LineNum: 1}
	_, err := p.ReadTripleTerm()
	if err == nil {
		t.Error("expected error for literal subject in triple term")
	}
}

func TestReadTripleTermBadPredicate(t *testing.T) {
	p := &LineParser{Line: `<<( <http://example.org/s> "bad" `, LineNum: 1}
	_, err := p.ReadTripleTerm()
	if err == nil {
		t.Error("expected error for literal predicate")
	}
}

func TestReadTripleTermBadObject(t *testing.T) {
	p := &LineParser{Line: `<<( <http://example.org/s> <http://example.org/p> @ `, LineNum: 1}
	_, err := p.ReadTripleTerm()
	if err == nil {
		t.Error("expected error for invalid object")
	}
}

func TestReadTripleTermNoCloseParen(t *testing.T) {
	p := &LineParser{Line: `<<( <http://example.org/s> <http://example.org/p> <http://example.org/o> >>`, LineNum: 1}
	_, err := p.ReadTripleTerm()
	if err == nil {
		t.Error("expected error: no ')'")
	}
}

func TestReadTripleTermNoCloseAngle(t *testing.T) {
	p := &LineParser{Line: `<<( <http://example.org/s> <http://example.org/p> <http://example.org/o> ) `, LineNum: 1}
	_, err := p.ReadTripleTerm()
	if err == nil {
		t.Error("expected error: no '>>'")
	}
}

func TestReadTripleTermNotStartingWithAngle(t *testing.T) {
	p := &LineParser{Line: `( <http://example.org/s> `, LineNum: 1}
	_, err := p.ReadTripleTerm()
	if err == nil {
		t.Error("expected error")
	}
}

// ReadPredicate error paths
func TestReadPredicateError(t *testing.T) {
	p := &LineParser{Line: `"literal"`, LineNum: 1}
	_, err := p.ReadPredicate()
	if err == nil {
		t.Error("expected error for non-IRI predicate")
	}
}

func TestReadPredicateRelative(t *testing.T) {
	p := &LineParser{Line: `<relative>`, LineNum: 1}
	_, err := p.ReadPredicate()
	if err == nil {
		t.Error("expected error for relative predicate IRI")
	}
}

// ReadGraphLabel: relative IRI
func TestReadGraphLabelRelative(t *testing.T) {
	p := &LineParser{Line: `<relative> .`, LineNum: 1}
	_, err := p.ReadGraphLabel()
	if err == nil {
		t.Error("expected error for relative graph IRI")
	}
}

// isValidLangTag edge cases
func TestLineParserLangTagEmptySubtag(t *testing.T) {
	p := &LineParser{Line: `"hello"@en- .`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	// en- has empty subtag after dash — should fail validation
	if err == nil {
		t.Error("expected error for invalid lang tag")
	}
}

func TestLineParserLangTagTooLong(t *testing.T) {
	p := &LineParser{Line: `"hello"@abcdefghi .`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error for lang subtag >8 chars")
	}
}

func TestLineParserLangTagDigitInPrimary(t *testing.T) {
	p := &LineParser{Line: `"hello"@1en .`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error for digit in primary subtag")
	}
}

func TestLineParserLangTagDigitInSecondary(t *testing.T) {
	p := &LineParser{Line: `"hello"@en-123 .`, Pos: 0, LineNum: 1}
	l, err := p.ReadLiteral()
	if err != nil {
		t.Fatal(err)
	}
	if l.Language() != "en-123" {
		t.Errorf("got lang %s", l.Language())
	}
}

// isAbsoluteIRI edge: no colon
func TestReadObjectRelativeIRI(t *testing.T) {
	p := &LineParser{Line: `<relative> .`, LineNum: 1}
	_, err := p.ReadObject()
	if err == nil {
		t.Error("expected error for relative object IRI")
	}
}

// ReadLiteral: invalid hex in \U
func TestLineParserReadLiteralInvalidHexBigU(t *testing.T) {
	p := &LineParser{Line: `"\UZZZZZZZZ"`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error")
	}
}

// TripleTermStr error propagation
func TestTripleTermStrError(t *testing.T) {
	// Can't easily trigger error from Term() on a well-formed TripleTerm,
	// but we can exercise the function path
	s, _ := rdflibgo.NewURIRef("http://example.org/s")
	p, _ := rdflibgo.NewURIRef("http://example.org/p")
	o := rdflibgo.NewLiteral("val")
	tt := rdflibgo.NewTripleTerm(s, p, o)
	result, err := TripleTermStr(tt)
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

// Literal: invalid lang in dir lang
func TestLineParserReadLiteralInvalidDirLang(t *testing.T) {
	p := &LineParser{Line: `"hello"@123--ltr .`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error for invalid lang in dir-lang")
	}
}

// TestTermNil covers the unsupported type error in Term.
func TestTermNilError(t *testing.T) {
	_, err := Term(nil)
	if err == nil {
		t.Error("expected error for nil term")
	}
}

// TestUnescapeIRIVariousErrors covers UnescapeIRI error paths.
func TestUnescapeIRIVariousErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"truncated_u", `http://x/\u00`},
		{"invalid_u", `http://x/\uGGGG`},
		{"surrogate_u", `http://x/\uD800`},
		{"truncated_U", `http://x/\U0000`},
		{"invalid_U", `http://x/\UGGGGGGGG`},
		{"surrogate_U", `http://x/\U0000D800`},
		{"unknown_escape", `http://x/\x`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := UnescapeIRI(tc.input)
			if err == nil {
				t.Error("expected error")
			}
		})
	}
}

// TestIsValidLangTagEdgeCases covers edge cases in language tag validation.
func TestIsValidLangTagEdgeCases(t *testing.T) {
	tests := []struct {
		tag  string
		want bool
	}{
		{"", false},
		{"en", true},
		{"en-US", true},
		{"en-123456789", false},
		{"en-", false},
		{"123", false},
		{"en-US-variant", true},
		{"a-b-c-d-e-f-g-h", true},
	}
	for _, tc := range tests {
		got := isValidLangTag(tc.tag)
		if got != tc.want {
			t.Errorf("isValidLangTag(%q) = %v, want %v", tc.tag, got, tc.want)
		}
	}
}

// TestIsAbsoluteIRIEdgeCases covers edge cases in absolute IRI detection.
func TestIsAbsoluteIRIEdgeCases(t *testing.T) {
	tests := []struct {
		iri  string
		want bool
	}{
		{"", false},
		{":", false},
		{"1http://x", false},
		{"http://x", true},
		{"h+t://x", true},
		{"h-t://x", true},
		{"h.t://x", true},
		{"h!t://x", false},
	}
	for _, tc := range tests {
		got := isAbsoluteIRI(tc.iri)
		if got != tc.want {
			t.Errorf("isAbsoluteIRI(%q) = %v, want %v", tc.iri, got, tc.want)
		}
	}
}

// TestEscapeIRIControlChar covers \u escape for control chars in IRI.
func TestEscapeIRIControlChar(t *testing.T) {
	got := EscapeIRI("http://x/\x01")
	if got != `http://x/\u0001` {
		t.Errorf("got %q", got)
	}
}

// TestEscapeIRIAngleBrackets covers < and > escaping in IRI.
func TestEscapeIRIAngleBrackets(t *testing.T) {
	got := EscapeIRI("http://x/<test>")
	if got != `http://x/\u003Ctest\u003E` {
		t.Errorf("got %q", got)
	}
}

// TestEscapeStringCarriageReturn covers \r escaping.
func TestEscapeStringCarriageReturn(t *testing.T) {
	got := EscapeString("a\rb")
	if got != `a\rb` {
		t.Errorf("got %q", got)
	}
}

// TestLiteralRDFLangStringNoLang covers the rdf:langString validation.
func TestLiteralRDFLangStringNoLang(t *testing.T) {
	p := &LineParser{
		Line:    `"hello"^^<http://www.w3.org/1999/02/22-rdf-syntax-ns#langString>`,
		Pos:     0,
		LineNum: 1,
	}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error for rdf:langString without language tag")
	}
}

// TestLiteralRDFDirLangStringNoLang covers the rdf:dirLangString validation.
func TestLiteralRDFDirLangStringNoLang(t *testing.T) {
	p := &LineParser{
		Line:    `"hello"^^<http://www.w3.org/1999/02/22-rdf-syntax-ns#dirLangString>`,
		Pos:     0,
		LineNum: 1,
	}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error for rdf:dirLangString without language tag")
	}
}

// TestReadLiteralDatatypeRelative covers the relative datatype IRI error.
func TestReadLiteralDatatypeRelative(t *testing.T) {
	p := &LineParser{Line: `"hello"^^<relative>`, Pos: 0, LineNum: 1}
	_, err := p.ReadLiteral()
	if err == nil {
		t.Error("expected error for relative datatype IRI")
	}
}
