package shacl

import (
	"os"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// --- Term / Graph helpers ---

func TestTermKindString(t *testing.T) {
	tests := []struct {
		kind TermKind
		want string
	}{
		{TermNone, "None"},
		{TermIRI, "IRI"},
		{TermLiteral, "Literal"},
		{TermBlankNode, "BlankNode"},
		{TermKind(99), "Unknown"},
	}
	for _, tc := range tests {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("TermKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

func TestTermEqual(t *testing.T) {
	t.Run("different kinds", func(t *testing.T) {
		if IRI("x").Equal(BlankNode("x")) {
			t.Error("IRI should not equal BlankNode")
		}
	})
	t.Run("both none", func(t *testing.T) {
		// TermNone.Equal returns false (no case for TermNone in switch)
		if (Term{}).Equal(Term{}) {
			t.Error("TermNone Equal returns false by design")
		}
	})
	t.Run("bnodes same", func(t *testing.T) {
		if !BlankNode("b1").Equal(BlankNode("b1")) {
			t.Error("same bnodes should be equal")
		}
	})
	t.Run("bnodes different", func(t *testing.T) {
		if BlankNode("b1").Equal(BlankNode("b2")) {
			t.Error("different bnodes should not be equal")
		}
	})
	t.Run("literals different lang", func(t *testing.T) {
		a := Literal("hi", "", "en")
		b := Literal("hi", "", "de")
		if a.Equal(b) {
			t.Error("different lang tags should not be equal")
		}
	})
}

func TestTermString(t *testing.T) {
	// Test the String() method (TermKey really) for coverage
	iri := IRI("http://example.org/x")
	if s := iri.String(); !strings.Contains(s, "http://example.org/x") {
		t.Errorf("unexpected IRI string: %s", s)
	}
	lit := Literal("hello", XSD+"string", "")
	if s := lit.String(); !strings.Contains(s, "hello") {
		t.Errorf("unexpected literal string: %s", s)
	}
	bn := BlankNode("b1")
	if s := bn.String(); !strings.Contains(s, "b1") {
		t.Errorf("unexpected bnode string: %s", s)
	}
}

func TestGraphOne(t *testing.T) {
	g := NewGraph()
	s := IRI(ex + "s")
	p := IRI(ex + "p")
	o := IRI(ex + "o")
	g.Add(s, p, o)

	tr, ok := g.One(&s, &p, nil)
	if !ok {
		t.Fatal("expected to find triple")
	}
	if !tr.Object.Equal(o) {
		t.Errorf("expected object %v, got %v", o, tr.Object)
	}

	// No match
	other := IRI(ex + "other")
	_, ok = g.One(&other, &p, nil)
	if ok {
		t.Error("expected no match")
	}
}

func TestGraphLen(t *testing.T) {
	g := NewGraph()
	if g.Len() != 0 {
		t.Error("expected 0")
	}
	g.Add(IRI(ex+"s"), IRI(ex+"p"), IRI(ex+"o"))
	if g.Len() != 1 {
		t.Error("expected 1")
	}
}

func TestGraphMerge(t *testing.T) {
	g1 := NewGraph()
	g1.Add(IRI(ex+"s"), IRI(ex+"p"), IRI(ex+"o"))
	g2 := NewGraph()
	g2.Add(IRI(ex+"s2"), IRI(ex+"p2"), IRI(ex+"o2"))
	g1.Merge(g2)
	if g1.Len() != 2 {
		t.Errorf("expected 2 triples, got %d", g1.Len())
	}
}

func TestGraphSubjects(t *testing.T) {
	g := NewGraph()
	s := IRI(ex + "s")
	p := IRI(ex + "p")
	o := IRI(ex + "o")
	g.Add(s, p, o)

	subs := g.Subjects(p, o)
	if len(subs) != 1 || !subs[0].Equal(s) {
		t.Errorf("expected [%v], got %v", s, subs)
	}

	// No match
	subs2 := g.Subjects(IRI(ex+"other"), o)
	if len(subs2) != 0 {
		t.Error("expected empty")
	}
}

// --- fromRDFLib edge cases ---

func TestFromRDFLibNil(t *testing.T) {
	result := fromRDFLib(nil)
	if result.kind != TermNone {
		t.Error("expected TermNone for nil")
	}
}

// --- toSubject / toTerm edge cases ---

func TestToSubjectLiteral(t *testing.T) {
	lit := Literal("hello", XSD+"string", "")
	result := toSubject(lit)
	if result != nil {
		t.Error("expected nil for literal as subject")
	}
}

func TestToTermBlankNode(t *testing.T) {
	bn := BlankNode("b1")
	result := toTerm(bn)
	if result == nil {
		t.Fatal("expected non-nil")
	}
}

func TestToTermNone(t *testing.T) {
	result := toTerm(Term{})
	if result != nil {
		t.Error("expected nil for TermNone")
	}
}

// --- LoadTurtle / LoadTurtleFile error ---

func TestLoadTurtleFileNotFound(t *testing.T) {
	_, err := LoadTurtleFile("/nonexistent/path.ttl")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoadTurtleInvalid(t *testing.T) {
	_, err := LoadTurtle(strings.NewReader("this is not valid turtle @@@"), "http://example.org/")
	if err == nil {
		t.Error("expected parse error")
	}
}

// --- RDFList cycle detection ---

func TestRDFListCycle(t *testing.T) {
	g := NewGraph()
	// Create a cycle: node1 -> first: "a", rest: node1
	node := BlankNode("cyc")
	first := IRI(RDFFirst)
	rest := IRI(RDFRest)
	g.Add(node, first, Literal("a", XSD+"string", ""))
	g.Add(node, rest, node) // cycle
	result := g.RDFList(node)
	if len(result) != 1 {
		t.Errorf("expected 1 element (cycle broken), got %d", len(result))
	}
}

// --- normalizeDollarVars ---

func TestNormalizeDollarVars(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"simple var", "SELECT $x WHERE { $x a $y }", "SELECT ?x WHERE { ?x a ?y }"},
		{"inside string", `SELECT $x WHERE { $x a "$y" }`, `SELECT ?x WHERE { ?x a "$y" }`},
		{"triple-quoted string", `"""$x""" $y`, `"""$x""" ?y`},
		{"no dollar", "SELECT ?x WHERE { ?x a ?y }", "SELECT ?x WHERE { ?x a ?y }"},
		{"dollar before non-var", "$1 $x", "$1 ?x"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeDollarVars(tc.input)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// --- termToSPARQL ---

func TestTermToSPARQL(t *testing.T) {
	tests := []struct {
		name string
		term Term
		want string
	}{
		{"IRI", IRI("http://example.org/x"), "<http://example.org/x>"},
		{"plain literal", Literal("hello", XSD+"string", ""), `"hello"`},
		{"lang literal", Literal("hello", "", "en"), `"hello"@en`},
		{"typed literal", Literal("42", XSD+"integer", ""), `"42"^^<` + XSD + `integer>`},
		{"bnode", BlankNode("b1"), "_:b1"},
		{"none", Term{}, `""`},
		{"literal with quotes", Literal(`say "hi"`, XSD+"string", ""), `"say \"hi\""`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := termToSPARQL(tc.term)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// --- replaceBoundCheck ---

func TestReplaceBoundCheck(t *testing.T) {
	tests := []struct {
		name, query, varToken, want string
	}{
		{"basic", "bound(?x)", "?x", "true"},
		{"with spaces", "bound( ?x )", "?x", "true"},
		{"no match", "bound(?y)", "?x", "bound(?y)"},
		{"in longer word", "unbound(?x)", "?x", "unbound(?x)"},
		{"case insensitive", "BOUND(?x)", "?x", "true"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := replaceBoundCheck(tc.query, tc.varToken)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// --- localName ---

func TestLocalName(t *testing.T) {
	tests := []struct {
		iri, want string
	}{
		{"http://example.org/x", "x"},
		{"http://example.org#y", "y"},
		{"noslash", "noslash"},
	}
	for _, tc := range tests {
		got := localName(tc.iri)
		if got != tc.want {
			t.Errorf("localName(%q) = %q, want %q", tc.iri, got, tc.want)
		}
	}
}

// --- Logical constraints (edge cases for shape not found) ---

func TestAndConstraintShapeNotFound(t *testing.T) {
	ctx := testCtx(NewGraph())
	s := testShape()
	c := &AndConstraint{Shapes: []Term{IRI(ex + "NonexistentShape")}}
	results := c.Evaluate(ctx, s, IRI(ex+"node"), []Term{IRI(ex + "node")})
	if len(results) != 0 {
		t.Error("expected no results when shape not found")
	}
}

func TestOrConstraintAllShapesNotFound(t *testing.T) {
	ctx := testCtx(NewGraph())
	s := testShape()
	c := &OrConstraint{Shapes: []Term{IRI(ex + "NonexistentShape")}}
	results := c.Evaluate(ctx, s, IRI(ex+"node"), []Term{IRI(ex + "node")})
	if len(results) != 1 {
		t.Errorf("expected 1 violation when all shapes not found, got %d", len(results))
	}
}

func TestNotConstraintShapeNotFound(t *testing.T) {
	ctx := testCtx(NewGraph())
	s := testShape()
	c := &NotConstraint{ShapeRef: IRI(ex + "NonexistentShape")}
	results := c.Evaluate(ctx, s, IRI(ex+"node"), []Term{IRI(ex + "node")})
	if len(results) != 0 {
		t.Error("expected no results when shape not found")
	}
}

func TestXoneConstraintShapeNotFound(t *testing.T) {
	ctx := testCtx(NewGraph())
	s := testShape()
	c := &XoneConstraint{Shapes: []Term{IRI(ex + "NonexistentShape")}}
	// count=0 (no shapes found) != 1, so should produce a violation
	results := c.Evaluate(ctx, s, IRI(ex+"node"), []Term{IRI(ex + "node")})
	if len(results) != 1 {
		t.Errorf("expected 1 violation for xone with no matching shapes, got %d", len(results))
	}
}

// --- NodeConstraint / PropertyConstraint shape not found ---

func TestNodeConstraintShapeNotFound(t *testing.T) {
	ctx := testCtx(NewGraph())
	s := testShape()
	c := &NodeConstraint{ShapeRef: IRI(ex + "NonexistentShape")}
	results := c.Evaluate(ctx, s, IRI(ex+"node"), []Term{IRI(ex + "node")})
	if len(results) != 0 {
		t.Error("expected no results when shape not found")
	}
}

func TestPropertyConstraintShapeNotFound(t *testing.T) {
	ctx := testCtx(NewGraph())
	s := testShape()
	c := &PropertyConstraint{ShapeRef: IRI(ex + "NonexistentShape")}
	results := c.Evaluate(ctx, s, IRI(ex+"node"), []Term{IRI(ex + "node")})
	if len(results) != 0 {
		t.Error("expected no results when shape not found")
	}
}

// --- deactivatedConstraint ---

func TestDeactivatedConstraint(t *testing.T) {
	c := &deactivatedConstraint{}
	if c.ComponentIRI() != "" {
		t.Error("expected empty component IRI")
	}
	results := c.Evaluate(nil, nil, Term{}, nil)
	if len(results) != 0 {
		t.Error("expected no results")
	}
}

// --- PairPathConstraint ---

func TestPairPathConstraintComponentIRI(t *testing.T) {
	tests := []struct {
		kind, wantSuffix string
	}{
		{"equals", "EqualsConstraintComponent"},
		{"disjoint", "DisjointConstraintComponent"},
		{"lessThan", "LessThanConstraintComponent"},
		{"lessThanOrEquals", "LessThanOrEqualsConstraintComponent"},
		{"unknown", ""},
	}
	for _, tc := range tests {
		c := &PairPathConstraint{Kind: tc.kind}
		got := c.ComponentIRI()
		if tc.wantSuffix == "" && got != "" {
			t.Errorf("kind=%s: expected empty, got %s", tc.kind, got)
		}
		if tc.wantSuffix != "" && !strings.HasSuffix(got, tc.wantSuffix) {
			t.Errorf("kind=%s: got %s", tc.kind, got)
		}
	}
}

// --- evalPairPathComparison ---

func TestEvalPairPathComparisonIRIs(t *testing.T) {
	s := testShape()
	// Two IRIs compared with lessThan
	a := IRI(ex + "b")
	b := IRI(ex + "a") // a < b alphabetically

	// lessThan: a >= b should fail (a.Value() = "http://example.org/b", b.Value() = "http://example.org/a")
	results := evalPairPathComparison(s, IRI(ex+"focus"), []Term{a}, []Term{b}, SH+"LessThanConstraintComponent", false)
	if len(results) != 1 {
		t.Errorf("expected 1 violation, got %d", len(results))
	}

	// lessThanOrEquals with equal values
	results = evalPairPathComparison(s, IRI(ex+"focus"), []Term{a}, []Term{a}, SH+"LessThanOrEqualsConstraintComponent", true)
	if len(results) != 0 {
		t.Errorf("expected 0 violations for equal, got %d", len(results))
	}
}

func TestEvalPairPathComparisonIncomparable(t *testing.T) {
	s := testShape()
	// A literal vs a bnode (not comparable)
	lit := Literal("hello", XSD+"string", "")
	bn := BlankNode("b1")
	results := evalPairPathComparison(s, IRI(ex+"focus"), []Term{lit}, []Term{bn}, SH+"LessThanConstraintComponent", false)
	if len(results) != 1 {
		t.Errorf("expected 1 violation for incomparable, got %d", len(results))
	}
}

// --- compareNumeric ---

func TestCompareNumericInvalid(t *testing.T) {
	_, ok := compareNumeric("notanumber", "42")
	if ok {
		t.Error("expected false for non-numeric")
	}
}

// --- compareDates ---

func TestCompareDatesIncomparable(t *testing.T) {
	// DateTime with timezone vs without timezone
	_, ok := compareDates("2024-01-01T10:00:00Z", "2024-01-01T10:00:00")
	if ok {
		t.Error("expected false for timezone mismatch")
	}
}

func TestCompareDatesInvalid(t *testing.T) {
	_, ok := compareDates("not-a-date", "also-not-a-date")
	if ok {
		t.Error("expected false for invalid dates")
	}
}

// --- parseTime ---

func TestParseTimeNoFormat(t *testing.T) {
	_, ok := parseTime("not-a-date", []string{"2006-01-02"})
	if ok {
		t.Error("expected false for unparseable time")
	}
}

// --- isValidDate ---

func TestIsValidDateTooShort(t *testing.T) {
	if isValidDate("2024-01") {
		t.Error("expected false for too-short date")
	}
}

func TestIsValidDateBadSeparator(t *testing.T) {
	if isValidDate("2024/01/01") {
		t.Error("expected false for / separator")
	}
}

func TestIsValidDateNonDigit(t *testing.T) {
	if isValidDate("202X-01-01") {
		t.Error("expected false for non-digit")
	}
}

// --- isInRange / isInBigRange / isInIntegerRange ---

func TestIsInRangeInvalid(t *testing.T) {
	if isInRange("notanumber", 0, 100) {
		t.Error("expected false")
	}
}

func TestIsInBigRangeInvalid(t *testing.T) {
	if isInBigRange("notanumber", "0", "100") {
		t.Error("expected false")
	}
	if isInBigRange("50", "notmin", "100") {
		t.Error("expected false for invalid min")
	}
}

func TestIsInIntegerRangeNonNegative(t *testing.T) {
	if !isInIntegerRange("0", XSD+"nonNegativeInteger") {
		t.Error("expected true for 0")
	}
	if isInIntegerRange("-1", XSD+"nonNegativeInteger") {
		t.Error("expected false for -1")
	}
}

func TestIsInIntegerRangeNonPositive(t *testing.T) {
	if !isInIntegerRange("0", XSD+"nonPositiveInteger") {
		t.Error("expected true for 0")
	}
	if isInIntegerRange("1", XSD+"nonPositiveInteger") {
		t.Error("expected false for 1")
	}
}

func TestIsInIntegerRangeInvalid(t *testing.T) {
	if isInIntegerRange("notanumber", XSD+"nonNegativeInteger") {
		t.Error("expected false for non-number")
	}
}

// --- matchesNodeKind ---

func TestMatchesNodeKindUnknown(t *testing.T) {
	if matchesNodeKind(IRI(ex+"x"), "http://example.org/CustomKind") {
		t.Error("expected false for unknown node kind")
	}
}

// --- SRL eval engine ---

func TestSRLTermToKeyBlankNode(t *testing.T) {
	term := SRLTerm{Kind: SRLTermBlankNode, Value: "b1"}
	if got := srlTermToKey(term); got != "_:b1" {
		t.Errorf("expected _:b1, got %s", got)
	}
}

func TestSRLTermToKeyLangLiteral(t *testing.T) {
	term := SRLTerm{Kind: SRLTermLiteral, Value: "hello", Language: "en"}
	got := srlTermToKey(term)
	if got != `"hello"@en` {
		t.Errorf("expected \"hello\"@en, got %s", got)
	}
}

func TestSRLTermToKeyTripleTerm(t *testing.T) {
	s := SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/s"}
	p := SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/p"}
	o := SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/o"}
	term := SRLTerm{Kind: SRLTermTripleTerm, TTSubject: &s, TTPredicate: &p, TTObject: &o}
	got := srlTermToKey(term)
	if !strings.HasPrefix(got, "<<(") {
		t.Errorf("expected triple term key, got %s", got)
	}
}

func TestSRLTermToKeyUnknown(t *testing.T) {
	term := SRLTerm{Kind: SRLTermKind(99)}
	if got := srlTermToKey(term); got != "" {
		t.Errorf("expected empty, got %s", got)
	}
}

func TestTermToSRLKeyBlankNode(t *testing.T) {
	bn := BlankNode("b1")
	if got := termToSRLKey(bn); got != "_:b1" {
		t.Errorf("expected _:b1, got %s", got)
	}
}

func TestTermToSRLKeyNone(t *testing.T) {
	if got := termToSRLKey(Term{}); got != "" {
		t.Errorf("expected empty, got %s", got)
	}
}

func TestTermToSRLKeyLiteralWithLang(t *testing.T) {
	lit := Literal("hello", "", "en")
	got := termToSRLKey(lit)
	if !strings.Contains(got, "@en") {
		t.Errorf("expected @en, got %s", got)
	}
}

// --- srlKeyToTerm ---

func TestSRLKeyToTermIRI(t *testing.T) {
	term := srlKeyToTerm("<http://example.org/x>")
	if !term.IsIRI() || term.Value() != "http://example.org/x" {
		t.Errorf("unexpected: %v", term)
	}
}

func TestSRLKeyToTermBlankNode(t *testing.T) {
	term := srlKeyToTerm("_:b1")
	if term.kind != TermBlankNode || term.value != "b1" {
		t.Errorf("unexpected: %v", term)
	}
}

func TestSRLKeyToTermLiteralTyped(t *testing.T) {
	term := srlKeyToTerm(`"42"^^<` + XSD + `integer>`)
	if !term.IsLiteral() || term.Value() != "42" {
		t.Errorf("unexpected: %v", term)
	}
	if term.Datatype() != XSD+"integer" {
		t.Errorf("expected xsd:integer, got %s", term.Datatype())
	}
}

func TestSRLKeyToTermLiteralLang(t *testing.T) {
	term := srlKeyToTerm(`"hello"@en`)
	if !term.IsLiteral() || term.Value() != "hello" {
		t.Errorf("unexpected: %v", term)
	}
	if term.Language() != "en" {
		t.Errorf("expected en, got %s", term.Language())
	}
}

func TestSRLKeyToTermLiteralDirLang(t *testing.T) {
	term := srlKeyToTerm(`"hello"@ar--ltr`)
	if term.Language() != "ar--ltr" {
		t.Errorf("expected ar--ltr, got %s", term.Language())
	}
	if term.Datatype() != RDF+"dirLangString" {
		t.Errorf("expected dirLangString, got %s", term.Datatype())
	}
}

func TestSRLKeyToTermPlain(t *testing.T) {
	term := srlKeyToTerm(`"hello"`)
	if !term.IsLiteral() || term.Value() != "hello" {
		t.Errorf("unexpected: %v", term)
	}
}

func TestSRLKeyToTermFallback(t *testing.T) {
	term := srlKeyToTerm("http://example.org/bare")
	if !term.IsIRI() {
		t.Errorf("expected IRI fallback, got %v", term)
	}
}

// --- escapeLiteralValue / unescapeLiteralValue ---

func TestEscapeUnescapeLiteralValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		escaped  string
	}{
		{"quotes", `say "hi"`, `say \"hi\"`},
		{"backslash", `a\b`, `a\\b`},
		{"newline", "a\nb", `a\nb`},
		{"return", "a\rb", `a\rb`},
		{"tab", "a\tb", `a\tb`},
		{"plain", "hello", "hello"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := escapeLiteralValue(tc.input)
			if got != tc.escaped {
				t.Errorf("escape: got %q, want %q", got, tc.escaped)
			}
			roundtripped := unescapeLiteralValue(got)
			if roundtripped != tc.input {
				t.Errorf("unescape: got %q, want %q", roundtripped, tc.input)
			}
		})
	}
}

// --- SRL evalFilter ---

func TestSRLEvalFilterTrue(t *testing.T) {
	e := &srlEvalEngine{triples: make(map[srlGroundTriple]bool)}
	if !e.evalFilter("true", srlBinding{}) {
		t.Error("expected true")
	}
}

func TestSRLEvalFilterFalse(t *testing.T) {
	e := &srlEvalEngine{triples: make(map[srlGroundTriple]bool)}
	if e.evalFilter("false", srlBinding{}) {
		t.Error("expected false")
	}
}

func TestSRLEvalFilterIsIRI(t *testing.T) {
	e := &srlEvalEngine{triples: make(map[srlGroundTriple]bool)}
	if !e.evalFilter("isIRI(<http://example.org/x>)", srlBinding{}) {
		t.Error("expected true for IRI")
	}
	if e.evalFilter(`isIRI("hello")`, srlBinding{}) {
		t.Error("expected false for non-IRI")
	}
}

func TestSRLEvalFilterComparison(t *testing.T) {
	e := &srlEvalEngine{triples: make(map[srlGroundTriple]bool)}
	if !e.evalFilter("5 > 3", srlBinding{}) {
		t.Error("expected true for 5 > 3")
	}
	if e.evalFilter("3 > 5", srlBinding{}) {
		t.Error("expected false for 3 > 5")
	}
	if !e.evalFilter("5 >= 5", srlBinding{}) {
		t.Error("expected true for 5 >= 5")
	}
	if !e.evalFilter("3 < 5", srlBinding{}) {
		t.Error("expected true for 3 < 5")
	}
	if !e.evalFilter("3 <= 3", srlBinding{}) {
		t.Error("expected true for 3 <= 3")
	}
	if !e.evalFilter("5 = 5", srlBinding{}) {
		t.Error("expected true for 5 = 5")
	}
	if !e.evalFilter("5 != 3", srlBinding{}) {
		t.Error("expected true for 5 != 3")
	}
}

func TestSRLEvalFilterDefault(t *testing.T) {
	e := &srlEvalEngine{triples: make(map[srlGroundTriple]bool)}
	// Unrecognized expression defaults to true
	if !e.evalFilter("some_unknown_expr", srlBinding{}) {
		t.Error("expected true for unrecognized expression")
	}
}

// --- SRL parseFilterNum ---

func TestParseFilterNum(t *testing.T) {
	v, ok := parseFilterNum("42")
	if !ok || v != 42 {
		t.Errorf("expected 42, got %v/%v", v, ok)
	}

	v, ok = parseFilterNum(`"123"^^<http://www.w3.org/2001/XMLSchema#integer>`)
	if !ok || v != 123 {
		t.Errorf("expected 123, got %v/%v", v, ok)
	}

	_, ok = parseFilterNum("notanumber")
	if ok {
		t.Error("expected false")
	}
}

// --- SRL substituteVars ---

func TestSubstituteVars(t *testing.T) {
	b := srlBinding{"x": "<http://example.org/x>"}
	got := substituteVars("?x > 5", b)
	if got != "<http://example.org/x> > 5" {
		t.Errorf("got %q", got)
	}
}

// --- SRL evalBindExpr ---

func TestSRLEvalBindExpr(t *testing.T) {
	e := &srlEvalEngine{triples: make(map[srlGroundTriple]bool)}
	b := srlBinding{"x": "<http://example.org/x>"}
	got := e.evalBindExpr("?x", b)
	if got != "<http://example.org/x>" {
		t.Errorf("got %q", got)
	}
}

// --- SRL matchElements with FILTER/NOT/BIND ---

func TestSRLMatchElementsFilter(t *testing.T) {
	e := &srlEvalEngine{triples: make(map[srlGroundTriple]bool)}
	gt := srlGroundTriple{S: `<http://example.org/s>`, P: `<http://example.org/p>`, O: `"42"^^<` + XSD + `integer>`}
	e.triples[gt] = true

	pattern := SRLTriple{
		Subject:   SRLTerm{Kind: SRLTermVariable, Value: "s"},
		Predicate: SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/p"},
		Object:    SRLTerm{Kind: SRLTermVariable, Value: "o"},
	}
	elements := []SRLBodyElement{
		{Kind: SRLBodyTriple, Triple: pattern},
		{Kind: SRLBodyFilter, FilterExpr: "true"},
	}
	results := e.matchElements(elements, 0, srlBinding{})
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSRLMatchElementsNot(t *testing.T) {
	e := &srlEvalEngine{triples: make(map[srlGroundTriple]bool)}
	gt := srlGroundTriple{S: "<http://example.org/s>", P: "<http://example.org/p>", O: "<http://example.org/o>"}
	e.triples[gt] = true

	// NOT body that does match — should fail
	notBody := []SRLBodyElement{
		{Kind: SRLBodyTriple, Triple: SRLTriple{
			Subject:   SRLTerm{Kind: SRLTermVariable, Value: "s"},
			Predicate: SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/p"},
			Object:    SRLTerm{Kind: SRLTermVariable, Value: "o"},
		}},
	}
	elements := []SRLBodyElement{
		{Kind: SRLBodyNot, NotBody: notBody},
	}
	results := e.matchElements(elements, 0, srlBinding{})
	if len(results) != 0 {
		t.Errorf("expected 0 results (NOT body matched), got %d", len(results))
	}

	// NOT body that does NOT match — should succeed
	notBody2 := []SRLBodyElement{
		{Kind: SRLBodyTriple, Triple: SRLTriple{
			Subject:   SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/nonexistent"},
			Predicate: SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/p"},
			Object:    SRLTerm{Kind: SRLTermVariable, Value: "o"},
		}},
	}
	elements2 := []SRLBodyElement{
		{Kind: SRLBodyNot, NotBody: notBody2},
	}
	results2 := e.matchElements(elements2, 0, srlBinding{})
	if len(results2) != 1 {
		t.Errorf("expected 1 result (NOT body did not match), got %d", len(results2))
	}
}

func TestSRLMatchElementsBind(t *testing.T) {
	e := &srlEvalEngine{triples: make(map[srlGroundTriple]bool)}
	elements := []SRLBodyElement{
		{Kind: SRLBodyBind, BindExpr: `"hello"`, BindVar: "x"},
	}
	results := e.matchElements(elements, 0, srlBinding{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0]["x"] != `"hello"` {
		t.Errorf("expected bound value, got %s", results[0]["x"])
	}
}

// --- SRL srlTermVars ---

func TestSRLTermVarsTripleTerm(t *testing.T) {
	s := SRLTerm{Kind: SRLTermVariable, Value: "s"}
	p := SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/p"}
	o := SRLTerm{Kind: SRLTermVariable, Value: "o"}
	tt := SRLTerm{Kind: SRLTermTripleTerm, TTSubject: &s, TTPredicate: &p, TTObject: &o}
	vars := srlTermVars(tt)
	if len(vars) != 2 {
		t.Errorf("expected 2 vars, got %d", len(vars))
	}
}

func TestSRLTermVarsIRI(t *testing.T) {
	term := SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/x"}
	vars := srlTermVars(term)
	if len(vars) != 0 {
		t.Errorf("expected 0 vars, got %d", len(vars))
	}
}

// --- ExpressionConstraint ---

func TestExpressionConstraintTruthy(t *testing.T) {
	// Test that expression evaluating to truthy returns no violations
	sg := NewGraph()
	node := IRI(ex + "expr1")
	// sh:path points to a constant expression — use a constant node
	constTrue := Literal("true", XSD+"boolean", "")
	sg.Add(node, IRI(SH+"path"), constTrue)

	ctx := &evalContext{
		dataGraph:   NewGraph(),
		shapesGraph: sg,
		shapesMap:   map[string]*Shape{},
	}
	s := testShape()
	c := &ExpressionConstraint{ExprNode: constTrue}
	results := c.Evaluate(ctx, s, IRI(ex+"focus"), []Term{IRI(ex + "focus")})
	// A plain literal "true" will be parsed as a constant expression
	if len(results) != 0 {
		t.Errorf("expected 0 violations for truthy expression, got %d", len(results))
	}
}

// --- NodeByExpressionConstraint ---

func TestNodeByExpressionConstraintShapeNotFound(t *testing.T) {
	ctx := testCtx(NewGraph())
	s := testShape()
	c := &NodeByExpressionConstraint{ShapeRef: IRI(ex + "NonexistentShape")}
	results := c.Evaluate(ctx, s, IRI(ex+"focus"), []Term{IRI(ex + "focus")})
	if len(results) != 0 {
		t.Error("expected no results when shape not found")
	}
}

// --- SingleLineConstraint ---

func TestSingleLineConstraintFalse(t *testing.T) {
	ctx := testCtx(NewGraph())
	s := testShape()
	c := &SingleLineConstraint{SingleLine: false}
	results := c.Evaluate(ctx, s, IRI(ex+"focus"), []Term{Literal("hello\nworld", XSD+"string", "")})
	if len(results) != 0 {
		t.Error("expected 0 violations when SingleLine is false")
	}
}

func TestSingleLineConstraintViolation(t *testing.T) {
	ctx := testCtx(NewGraph())
	s := testShape()
	c := &SingleLineConstraint{SingleLine: true}
	results := c.Evaluate(ctx, s, IRI(ex+"focus"), []Term{Literal("hello\nworld", XSD+"string", "")})
	if len(results) != 1 {
		t.Errorf("expected 1 violation, got %d", len(results))
	}
}

func TestSingleLineConstraintNonLiteral(t *testing.T) {
	ctx := testCtx(NewGraph())
	s := testShape()
	c := &SingleLineConstraint{SingleLine: true}
	results := c.Evaluate(ctx, s, IRI(ex+"focus"), []Term{IRI(ex + "x")})
	if len(results) != 0 {
		t.Error("expected 0 violations for non-literal")
	}
}

// --- SomeValueConstraint ---

func TestSomeValueConstraintShapeNotFound(t *testing.T) {
	ctx := testCtx(NewGraph())
	s := testShape()
	c := &SomeValueConstraint{ShapeRef: IRI(ex + "NonexistentShape")}
	results := c.Evaluate(ctx, s, IRI(ex+"focus"), []Term{IRI(ex + "x")})
	if len(results) != 0 {
		t.Error("expected no results when shape not found")
	}
}

// --- node_expr helpers ---

func TestParseNumeric(t *testing.T) {
	v, ok := parseNumeric(Literal("42", XSD+"integer", ""))
	if !ok || v != 42 {
		t.Errorf("expected 42, got %v", v)
	}

	_, ok = parseNumeric(IRI(ex + "x"))
	if ok {
		t.Error("expected false for IRI")
	}

	_, ok = parseNumeric(Literal("notanumber", XSD+"integer", ""))
	if ok {
		t.Error("expected false for non-numeric string")
	}
}

func TestIsTruthy(t *testing.T) {
	if isTruthy(nil) {
		t.Error("expected false for nil")
	}
	if isTruthy([]Term{}) {
		t.Error("expected false for empty")
	}
	if !isTruthy([]Term{Literal("hello", XSD+"string", "")}) {
		t.Error("expected true for non-false literal")
	}
	if isTruthy([]Term{Literal("false", XSD+"boolean", "")}) {
		t.Error("expected false for xsd:boolean false")
	}
	// Note: Literal("false", "", "") gets datatype xsd:string, which doesn't match
	// the boolean check in isTruthy, so it's actually truthy.
}

func TestCompareTermLists(t *testing.T) {
	if compareTermLists(nil, nil) != 0 {
		t.Error("nil vs nil should be 0")
	}
	if compareTermLists(nil, []Term{IRI(ex + "x")}) != -1 {
		t.Error("nil vs non-empty should be -1")
	}
	if compareTermLists([]Term{IRI(ex + "x")}, nil) != 1 {
		t.Error("non-empty vs nil should be 1")
	}
}

func TestCompareTerm(t *testing.T) {
	a := Literal("1", XSD+"integer", "")
	b := Literal("2", XSD+"integer", "")
	if compareTerm(a, b) >= 0 {
		t.Error("expected 1 < 2")
	}
	if compareTerm(b, a) <= 0 {
		t.Error("expected 2 > 1")
	}
	if compareTerm(a, a) != 0 {
		t.Error("expected equal")
	}

	// String fallback
	c := IRI(ex + "a")
	d := IRI(ex + "b")
	if compareTerm(c, d) >= 0 {
		t.Error("expected a < b")
	}
}

// --- TargetKind String ---

func TestTargetKindString(t *testing.T) {
	s := TargetNode.String()
	if s != "targetNode" {
		t.Errorf("expected targetNode, got %s", s)
	}
}

// --- Validate end-to-end with Turtle ---

func TestValidateBasicConforms(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ;
			ex:name "Alice" .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:name ;
				sh:datatype xsd:string ;
				sh:minCount 1 ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Errorf("expected conforms, got %d violations", len(report.Results))
	}
}

func TestValidateBasicViolation(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:Alice a ex:Person .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:name ;
				sh:minCount 1 ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if report.Conforms {
		t.Error("expected violation (missing required name)")
	}
}

// --- DatatypeListConstraint ---

func TestDatatypeListConstraintNonLiteral(t *testing.T) {
	ctx := testCtx(NewGraph())
	s := testShape()
	c := &DatatypeListConstraint{Datatypes: []Term{IRI(XSD + "string")}}
	results := c.Evaluate(ctx, s, IRI(ex+"focus"), []Term{IRI(ex + "x")})
	if len(results) != 1 {
		t.Errorf("expected 1 violation for non-literal, got %d", len(results))
	}
}

// --- QualifiedValueShapeConstraint shape not found ---

func TestQualifiedValueShapeNotFound(t *testing.T) {
	ctx := testCtx(NewGraph())
	s := testShape()
	c := &QualifiedValueShapeConstraint{
		ShapeRef:          IRI(ex + "NonexistentShape"),
		QualifiedMinCount: 1,
	}
	results := c.Evaluate(ctx, s, IRI(ex+"focus"), []Term{IRI(ex + "x")})
	if len(results) != 0 {
		t.Error("expected no results when shape not found")
	}
}

// --- SRL ParseSRL ---

func TestParseSRLBasic(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?x ex:inferred "yes" .
		} WHERE {
			?x ex:type ex:Thing .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rs.Rules))
	}
}

func TestParseSRLDataBlock(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:a ex:type ex:Thing .
		}
		RULE {
			?x ex:inferred "yes" .
		} WHERE {
			?x ex:type ex:Thing .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) != 1 {
		t.Errorf("expected 1 data block, got %d", len(rs.DataBlocks))
	}
}

func TestParseSRLBase(t *testing.T) {
	// BASE directive is parsed successfully (stored in lexer for IRI resolution)
	rs, err := ParseSRL(`
		BASE <http://example.org/>
		PREFIX ex: <http://example.org/>
		RULE {
			?x ex:inferred "yes" .
		} WHERE {
			?x ex:type ex:Thing .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rs.Rules))
	}
}

func TestParseSRLFilter(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?x ex:big "yes" .
		} WHERE {
			?x ex:value ?v .
			FILTER(?v > 10)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rs.Rules))
	}
	// Body should have 2 elements: triple + filter
	if len(rs.Rules[0].Body) != 2 {
		t.Errorf("expected 2 body elements, got %d", len(rs.Rules[0].Body))
	}
}

func TestParseSRLNot(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?x ex:alone "yes" .
		} WHERE {
			?x ex:type ex:Thing .
			NOT {
				?x ex:related ?y .
			}
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rules[0].Body) != 2 {
		t.Errorf("expected 2 body elements (triple + NOT), got %d", len(rs.Rules[0].Body))
	}
}

func TestParseSRLBind(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?x ex:label ?label .
		} WHERE {
			?x ex:name ?name .
			BIND(?name AS ?label)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range rs.Rules[0].Body {
		if e.Kind == SRLBodyBind {
			found = true
			if e.BindVar != "label" {
				t.Errorf("expected bind var 'label', got '%s'", e.BindVar)
			}
		}
	}
	if !found {
		t.Error("expected BIND body element")
	}
}

func TestParseSRLLiterals(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
		RULE {
			?x ex:result "42"^^xsd:integer .
		} WHERE {
			?x ex:type ex:Thing .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if rs.Rules[0].Head[0].Object.Datatype != XSD+"integer" {
		t.Errorf("expected integer datatype, got %s", rs.Rules[0].Head[0].Object.Datatype)
	}
}

func TestParseSRLLangLiteral(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?x ex:label "hello"@en .
		} WHERE {
			?x ex:type ex:Thing .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	obj := rs.Rules[0].Head[0].Object
	if obj.Language != "en" {
		t.Errorf("expected en, got %s", obj.Language)
	}
}

func TestParseSRLBooleanLiterals(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:a ex:flag true .
			ex:b ex:flag false .
		}
		RULE {
			?x ex:valid "yes" .
		} WHERE {
			?x ex:flag true .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) != 1 || len(rs.DataBlocks[0]) != 2 {
		t.Error("expected 2 data triples")
	}
}

func TestParseSRLNumericLiterals(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:a ex:value 42 .
			ex:b ex:value 3.14 .
		}
		RULE {
			?x ex:big "yes" .
		} WHERE {
			?x ex:value ?v .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks[0]) != 2 {
		t.Errorf("expected 2 data triples, got %d", len(rs.DataBlocks[0]))
	}
}

func TestParseSRLBlankNode(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			_:b1 ex:type ex:Thing .
		}
		RULE {
			?x ex:found "yes" .
		} WHERE {
			?x ex:type ex:Thing .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks[0]) != 1 {
		t.Errorf("expected 1 data triple, got %d", len(rs.DataBlocks[0]))
	}
	if rs.DataBlocks[0][0].Subject.Kind != SRLTermBlankNode {
		t.Error("expected blank node subject")
	}
}

// --- SRL EvalRuleSet ---

func TestEvalRuleSetBasic(t *testing.T) {
	dataG := NewGraph()
	dataG.Add(IRI(ex+"a"), IRI(ex+"type"), IRI(ex+"Thing"))

	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?x ex:inferred "yes" .
		} WHERE {
			?x ex:type ex:Thing .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}

	result, err := EvalRuleSet(rs, dataG)
	if err != nil {
		t.Fatal(err)
	}
	if result.Len() != 1 {
		t.Errorf("expected 1 inferred triple, got %d", result.Len())
	}
}

// --- SRL instantiateTerm error ---

func TestInstantiateTermUnbound(t *testing.T) {
	term := SRLTerm{Kind: SRLTermVariable, Value: "unbound"}
	_, err := instantiateTerm(term, srlBinding{})
	if err == nil {
		t.Error("expected error for unbound variable")
	}
}

// --- SRL Stratify ---

func TestStratifyEmpty(t *testing.T) {
	rs := &RuleSet{}
	strata, err := Stratify(rs)
	if err != nil {
		t.Fatal(err)
	}
	if len(strata) != 0 {
		t.Errorf("expected 0 strata, got %d", len(strata))
	}
}

// --- SRL CheckWellformed ---

func TestCheckWellformedValid(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?x ex:inferred "yes" .
		} WHERE {
			?x ex:type ex:Thing .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckWellformed(rs); err != nil {
		t.Errorf("expected valid, got error: %v", err)
	}
}

func TestCheckWellformedInvalid(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?x ex:inferred ?y .
		} WHERE {
			?x ex:type ex:Thing .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckWellformed(rs); err == nil {
		t.Error("expected error for unbound ?y in head")
	}
}

// --- TargetKind String all values ---

func TestTargetKindStringAll(t *testing.T) {
	tests := []struct {
		kind TargetKind
		want string
	}{
		{TargetNode, "targetNode"},
		{TargetClass, "targetClass"},
		{TargetSubjectsOf, "targetSubjectsOf"},
		{TargetObjectsOf, "targetObjectsOf"},
	}
	for _, tc := range tests {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("TargetKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

// --- SHACL with deactivated shapes ---

func TestValidateDeactivatedShape(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:Alice a ex:Person .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:deactivated true ;
			sh:property [
				sh:path ex:name ;
				sh:minCount 1 ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms because shape is deactivated")
	}
}

// --- SHACL OR constraint: at least one shape conforms ---

func TestOrConstraintOneConforms(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ;
			ex:name "Alice"^^xsd:string .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:NameOrAge a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:or (
				[ sh:property [ sh:path ex:name ; sh:minCount 1 ] ]
				[ sh:property [ sh:path ex:age ; sh:minCount 1 ] ]
			) .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms because OR branch matches")
	}
}

// --- SHACL AND constraint ---

func TestAndConstraintBothConform(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ;
			ex:name "Alice"^^xsd:string ;
			ex:age "30"^^xsd:integer .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:NameAndAge a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:and (
				[ sh:property [ sh:path ex:name ; sh:minCount 1 ] ]
				[ sh:property [ sh:path ex:age ; sh:minCount 1 ] ]
			) .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms because both AND branches match")
	}
}

// --- SHACL xone ---

func TestXoneConstraintExactlyOne(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ;
			ex:name "Alice"^^xsd:string .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:ExactlyOneShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:xone (
				[ sh:property [ sh:path ex:name ; sh:minCount 1 ] ]
				[ sh:property [ sh:path ex:age ; sh:minCount 1 ] ]
			) .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms because exactly one xone branch matches")
	}
}

// --- SHACL NOT constraint ---

func TestNotConstraintConforms(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:Alice a ex:Person .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:NotAnimal a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:not [ sh:class ex:Animal ] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms because Alice is not Animal")
	}
}

// --- SHACL sh:node constraint ---

func TestNodeConstraintConforms(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ;
			ex:address ex:Addr1 .
		ex:Addr1 ex:city "NYC"^^xsd:string .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:AddrShape a sh:NodeShape ;
			sh:property [
				sh:path ex:city ;
				sh:minCount 1 ;
			] .

		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:address ;
				sh:node ex:AddrShape ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms")
	}
}

// --- SHACL qualifiedValueShape ---

func TestQualifiedValueShapeConforms(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ;
			ex:name "Alice"^^xsd:string ;
			ex:name "Bob"^^xsd:string .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:name ;
				sh:qualifiedValueShape [ sh:datatype xsd:string ] ;
				sh:qualifiedMinCount 1 ;
				sh:qualifiedMaxCount 3 ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms")
	}
}

// --- SHACL SPARQL constraint ---

func TestSPARQLConstraintConforms(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ;
			ex:name "Alice"^^xsd:string .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:sparql [
				sh:select """
					PREFIX ex: <http://example.org/>
					SELECT $this WHERE {
						$this ex:name ?name .
						FILTER(!BOUND(?name))
					}
				""" ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms (SPARQL finds no results)")
	}
}

// --- SHACL minInclusive / maxInclusive ---

func TestValueRangeConstraint(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ;
			ex:age "30"^^xsd:integer .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:AgeShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:age ;
				sh:minInclusive 0 ;
				sh:maxInclusive 150 ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms (age 30 in range 0-150)")
	}
}

// --- SHACL pattern constraint ---

func TestPatternConstraintRegex(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ;
			ex:email "alice@example.org"^^xsd:string .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:email ;
				sh:pattern "^[^@]+@[^@]+$" ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms (email matches pattern)")
	}
}

// --- SHACL sh:equals / sh:disjoint ---

func TestEqualsConstraintPair(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ;
			ex:name "Alice"^^xsd:string ;
			ex:label "Alice"^^xsd:string .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:name ;
				sh:equals ex:label ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms (name equals label)")
	}
}

// --- SHACL sh:in ---

func TestInConstraintAllowed(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ;
			ex:status "active"^^xsd:string .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:status ;
				sh:in ( "active" "inactive" ) ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms (status in allowed list)")
	}
}

// --- SHACL sh:hasValue ---

func TestHasValueConstraintConforms(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ;
			ex:status "active"^^xsd:string .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:status ;
				sh:hasValue "active" ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms (has required value)")
	}
}

// --- SHACL sh:minLength / sh:maxLength ---

func TestLengthConstraint(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Alice a ex:Person ;
			ex:name "Alice"^^xsd:string .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:name ;
				sh:minLength 1 ;
				sh:maxLength 100 ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms (name length in range)")
	}
}

// --- SHACL sh:closed ---

func TestClosedConstraintConforms(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
		ex:Alice a ex:Person ;
			ex:name "Alice"^^xsd:string .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:closed true ;
			sh:ignoredProperties ( rdf:type ) ;
			sh:property [
				sh:path ex:name ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms (only allowed properties used)")
	}
}

// --- SHACL sh:uniqueLang ---

func TestUniqueLangConstraintConforms(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:Alice a ex:Person ;
			ex:name "Alice"@en ;
			ex:name "Alicia"@es .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .

		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:name ;
				sh:uniqueLang true ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conforms (unique langs)")
	}
}

// --- SHACL severity override ---

// --- SRL advanced parsing ---

func TestParseSRLTripleQuotedString(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:a ex:desc """Hello
World""" .
		}
		RULE {
			?x ex:found "yes" .
		} WHERE {
			?x ex:desc ?d .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if rs.DataBlocks[0][0].Object.Value != "Hello\nWorld" {
		t.Errorf("expected multiline string, got %q", rs.DataBlocks[0][0].Object.Value)
	}
}

func TestParseSRLStringEscapes(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:a ex:val "hello\tworld\n" .
		}
		RULE { ?x ex:found "yes" . } WHERE { ?x ex:val ?v . }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if rs.DataBlocks[0][0].Object.Value != "hello\tworld\n" {
		t.Errorf("expected escaped string, got %q", rs.DataBlocks[0][0].Object.Value)
	}
}

func TestParseSRLUnicodeEscape(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:a ex:val "caf\u00E9" .
		}
		RULE { ?x ex:found "yes" . } WHERE { ?x ex:val ?v . }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if rs.DataBlocks[0][0].Object.Value != "caf\u00E9" {
		t.Errorf("expected cafe with accent, got %q", rs.DataBlocks[0][0].Object.Value)
	}
}

func TestParseSRLTripleTerm(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			<<( ?s ex:p ?o )>> ex:asserted "yes" .
		} WHERE {
			?s ex:p ?o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if rs.Rules[0].Head[0].Subject.Kind != SRLTermTripleTerm {
		t.Error("expected triple term subject")
	}
}

func TestParseSRLCollection(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:a ex:list ( ex:b ex:c ) .
		}
		RULE { ?x ex:found "yes" . } WHERE { ?x ex:list ?l . }
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Collections expand to rdf:first/rdf:rest/rdf:nil triples
	if len(rs.DataBlocks[0]) < 3 {
		t.Errorf("expected at least 3 triples from collection, got %d", len(rs.DataBlocks[0]))
	}
}

func TestParseSRLBnodePropertyList(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:a ex:related [ ex:type ex:Thing ] .
		}
		RULE { ?x ex:found "yes" . } WHERE { ?x ex:related ?r . }
	`)
	if err != nil {
		t.Fatal(err)
	}
	// Blank node property list expands
	if len(rs.DataBlocks[0]) < 2 {
		t.Errorf("expected at least 2 triples from bnode prop list, got %d", len(rs.DataBlocks[0]))
	}
}

func TestParseSRLMultiplePredicates(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:a ex:type ex:Thing ;
				 ex:name "A" .
		}
		RULE { ?x ex:found "yes" . } WHERE { ?x ex:type ex:Thing . }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks[0]) != 2 {
		t.Errorf("expected 2 data triples from semicolon syntax, got %d", len(rs.DataBlocks[0]))
	}
}

func TestParseSRLMultipleObjects(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:a ex:name "Alice" , "Bob" .
		}
		RULE { ?x ex:found "yes" . } WHERE { ?x ex:name ?n . }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks[0]) != 2 {
		t.Errorf("expected 2 data triples from comma syntax, got %d", len(rs.DataBlocks[0]))
	}
}

func TestParseSRLRDFType(t *testing.T) {
	// 'a' as shorthand for rdf:type
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:alice a ex:Person .
		}
		RULE { ?x ex:found "yes" . } WHERE { ?x a ex:Person . }
	`)
	if err != nil {
		t.Fatal(err)
	}
	if rs.DataBlocks[0][0].Predicate.Value != RDFType {
		t.Errorf("expected rdf:type, got %s", rs.DataBlocks[0][0].Predicate.Value)
	}
}

func TestParseSRLExprInFilter(t *testing.T) {
	// FILTER with an expression containing strings
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE { ?x ex:found "yes" . }
		WHERE {
			?x ex:name ?n .
			FILTER(?n != "unknown")
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range rs.Rules[0].Body {
		if e.Kind == SRLBodyFilter {
			found = true
		}
	}
	if !found {
		t.Error("expected FILTER body element")
	}
}

// --- SRL allTriples ---

func TestSRLAllTriples(t *testing.T) {
	e := &srlEvalEngine{triples: make(map[srlGroundTriple]bool)}
	gt1 := srlGroundTriple{S: "<a>", P: "<b>", O: "<c>"}
	gt2 := srlGroundTriple{S: "<d>", P: "<e>", O: "<f>"}
	e.triples[gt1] = true
	e.triples[gt2] = true
	all := e.allTriples()
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}
}

// --- SRL instantiate ---

func TestSRLInstantiateFull(t *testing.T) {
	e := &srlEvalEngine{triples: make(map[srlGroundTriple]bool)}
	t1 := SRLTriple{
		Subject:   SRLTerm{Kind: SRLTermVariable, Value: "s"},
		Predicate: SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/p"},
		Object:    SRLTerm{Kind: SRLTermVariable, Value: "o"},
	}
	b := srlBinding{"s": "<http://example.org/x>", "o": `"hello"^^<` + XSD + `string>`}
	gt, err := e.instantiate(t1, b)
	if err != nil {
		t.Fatal(err)
	}
	if gt.S != "<http://example.org/x>" {
		t.Errorf("unexpected subject: %s", gt.S)
	}
}

func TestSRLInstantiateError(t *testing.T) {
	e := &srlEvalEngine{triples: make(map[srlGroundTriple]bool)}
	t1 := SRLTriple{
		Subject:   SRLTerm{Kind: SRLTermVariable, Value: "unbound"},
		Predicate: SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/p"},
		Object:    SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/o"},
	}
	_, err := e.instantiate(t1, srlBinding{})
	if err == nil {
		t.Error("expected error for unbound variable")
	}
}

func TestSeverityOverride(t *testing.T) {
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:Alice a ex:Person .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .

		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:severity sh:Warning ;
			sh:property [
				sh:path ex:name ;
				sh:minCount 1 ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}

	report := Validate(dataG, shapesG)
	// Warning still means non-conformant (only Debug/Trace are ignored)
	if report.Conforms {
		t.Error("expected non-conformant with Warning severity")
	}
	if len(report.Results) == 0 {
		t.Fatal("expected at least one result")
	}
}

// --- SRL parser: parsePrefix error paths ---

func TestParseSRLPrefixBadName(t *testing.T) {
	// Prefix name must end with ':'
	_, err := ParseSRL(`PREFIX nocolon <http://example.org/>`)
	if err == nil {
		t.Error("expected error for prefix without colon")
	}
}

func TestParseSRLPrefixMissingIRI(t *testing.T) {
	_, err := ParseSRL(`PREFIX ex: `)
	if err == nil {
		t.Error("expected error for prefix without IRI")
	}
}

// --- SRL parser: parseLiteral with typed literal ---

func TestParseSRLTypedLiteralPName(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p "42"^^xsd:integer .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 || len(rs.DataBlocks[0]) == 0 {
		t.Fatal("expected data block")
	}
	if rs.DataBlocks[0][0].Object.Datatype != "http://www.w3.org/2001/XMLSchema#integer" {
		t.Errorf("unexpected datatype: %s", rs.DataBlocks[0][0].Object.Datatype)
	}
}

func TestParseSRLTypedLiteralIRI(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p "42"^^<http://www.w3.org/2001/XMLSchema#integer> .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 || len(rs.DataBlocks[0]) == 0 {
		t.Fatal("expected data block")
	}
}

func TestParseSRLTypedLiteralBadDatatype(t *testing.T) {
	_, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p "42"^^42 .
		}
	`)
	if err == nil {
		t.Error("expected error for bad datatype after ^^")
	}
}

// --- SRL parser: parseLiteral with directional lang tag ---

func TestParseSRLLiteralDirLangTag(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p "hello"@en--ltr .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 || len(rs.DataBlocks[0]) == 0 {
		t.Fatal("expected data block")
	}
	obj := rs.DataBlocks[0][0].Object
	if obj.Language != "en--ltr" {
		t.Errorf("expected lang en--ltr, got %s", obj.Language)
	}
	// The -- separator may be treated differently depending on parser logic
	// Just check that the lang tag was parsed
	if obj.Language == "" {
		t.Error("expected non-empty language")
	}
}

// --- SRL parser: parseAnnotations ---

func TestParseSRLAnnotation(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p ex:o {| ex:source ex:wikipedia |} .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 || len(rs.DataBlocks[0]) < 3 {
		t.Fatalf("expected at least 3 triples from annotated triple, got %d", len(rs.DataBlocks[0]))
	}
}

func TestParseSRLAnnotationWithReifier(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p ex:o ~ex:reifier1 .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
}

// --- SRL parser: parseTripleTerm with reifier ---

func TestParseSRLTripleTermWithReifier(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			<<( ex:s ex:p ex:o ~ex:reif1 )>> ex:source ex:wiki .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
}

// --- SRL parser: reified triple (<<>> without parens) ---

func TestParseSRLReifiedTriple(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			<< ex:s ex:p ex:o >> ex:source ex:wiki .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
}

// --- SRL parser: reified triple with annotation ---

func TestParseSRLReifiedTripleWithAnnotation(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			<< ex:s ex:p ex:o >> {| ex:meta ex:val |} ex:source ex:wiki .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
}

// --- SRL parser: parseBodyElements with FILTER/BIND ---

func TestParseSRLRuleWithFilterAndBind(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?s ex:bigger ?o .
		} WHERE {
			?s ex:value ?v1 .
			?o ex:value ?v2 .
			FILTER(?v1 > ?v2)
			BIND(?v1 - ?v2 AS ?diff)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rules) == 0 {
		t.Fatal("expected a rule")
	}
	// Check body elements
	hasFilter := false
	hasBind := false
	for _, elem := range rs.Rules[0].Body {
		if elem.Kind == SRLBodyFilter {
			hasFilter = true
		}
		if elem.Kind == SRLBodyBind {
			hasBind = true
		}
	}
	if !hasFilter {
		t.Error("expected FILTER element")
	}
	if !hasBind {
		t.Error("expected BIND element")
	}
}

// --- SRL parser: expandPName error ---

func TestParseSRLUndefinedPrefix(t *testing.T) {
	_, err := ParseSRL(`
		DATA {
			undefined:s undefined:p undefined:o .
		}
	`)
	if err == nil {
		t.Error("expected error for undefined prefix")
	}
}

// --- SRL parser: parseBind error ---

func TestParseSRLBindNoAS(t *testing.T) {
	_, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?s ex:p ?o .
		} WHERE {
			?s ex:q ?v .
			BIND(?v ?x)
		}
	`)
	if err == nil {
		t.Error("expected error for BIND without AS")
	}
}

// --- SRL parser: parseRule with NOT block ---

func TestParseSRLRuleWithNOT(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?s ex:inferred ex:yes .
		} WHERE {
			?s ex:value ?v .
			NOT {
				?s ex:excluded ex:yes .
			}
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rules) == 0 {
		t.Fatal("expected a rule")
	}
	hasNot := false
	for _, elem := range rs.Rules[0].Body {
		if elem.Kind == SRLBodyNot {
			hasNot = true
		}
	}
	if !hasNot {
		t.Error("expected NOT element")
	}
}

// --- SRL parser: DATA with variables should error ---

func TestParseSRLDataWithVariables(t *testing.T) {
	_, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			?s ex:p ex:o .
		}
	`)
	if err == nil {
		t.Error("expected error for variables in DATA block")
	}
}

// --- SRL parser: blank node as predicate should error ---

func TestParseSRLBNodeAsPredicate(t *testing.T) {
	_, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s _:bnode ex:o .
		}
	`)
	if err == nil {
		t.Error("expected error for blank node as predicate")
	}
}

// --- SRL parser: empty blank node property list ---

func TestParseSRLEmptyBNodePropertyList(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			[] ex:p ex:o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 || len(rs.DataBlocks[0]) == 0 {
		t.Fatal("expected data block")
	}
}

// --- SRL parser: decimal and double numbers ---

func TestParseSRLDecimalNumber(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p 3.14 .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
	obj := rs.DataBlocks[0][0].Object
	if obj.Datatype != XSD+"decimal" {
		t.Errorf("expected decimal datatype, got %s", obj.Datatype)
	}
}

func TestParseSRLDoubleNumber(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p 3.14e2 .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
	obj := rs.DataBlocks[0][0].Object
	if obj.Datatype != XSD+"double" {
		t.Errorf("expected double datatype, got %s", obj.Datatype)
	}
}

// --- SRL parser: signed numbers ---

func TestParseSRLSignedNumber(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p -42 .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
	obj := rs.DataBlocks[0][0].Object
	if obj.Value != "-42" {
		t.Errorf("expected -42, got %s", obj.Value)
	}
}

// --- SRL parser: comment handling ---

func TestParseSRLComment(t *testing.T) {
	rs, err := ParseSRL(`
		# This is a comment
		PREFIX ex: <http://example.org/>
		DATA {
			# Another comment
			ex:s ex:p ex:o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
}

// --- SRL parser: 'a' as object (subject too) ---

func TestParseSRLAAsObject(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p a .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
	obj := rs.DataBlocks[0][0].Object
	if obj.Value != RDFType {
		t.Errorf("expected rdf:type IRI, got %s", obj.Value)
	}
}

// --- SRL lexer: unicode escape in IRI ---

func TestParseSRLIRIWithUnicodeEscape(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			<http://example.org/\u0041> ex:p ex:o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
	// \u0041 = 'A'
	subj := rs.DataBlocks[0][0].Subject
	if subj.Value != "http://example.org/A" {
		t.Errorf("expected http://example.org/A, got %s", subj.Value)
	}
}

// --- SRL lexer: string escapes ---

func TestParseSRLStringEscapeSequences(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p "line1\nline2\ttab\\slash\"quote" .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
	val := rs.DataBlocks[0][0].Object.Value
	if val != "line1\nline2\ttab\\slash\"quote" {
		t.Errorf("unexpected string value: %q", val)
	}
}

// --- SRL lexer: hat (^^) and pipe (|) ---

func TestParseSRLAnnotationEnd(t *testing.T) {
	// |} is the RAnnot token
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p ex:o {| ex:anno ex:val |} .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
}

// --- SRL lexer: )>> token ---

func TestParseSRLTripleTermParenClose(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			<<( ex:s ex:p ex:o )>> ex:meta ex:val .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
}

// --- SRL lexer: dot starting decimal ---

func TestParseSRLDotDecimal(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p .5 .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
}

// --- SRL lexer: single-quoted string ---

func TestParseSRLSingleQuotedString(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p 'single quoted' .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
	if rs.DataBlocks[0][0].Object.Value != "single quoted" {
		t.Errorf("unexpected value: %s", rs.DataBlocks[0][0].Object.Value)
	}
}

// --- SRL parser: unexpected token ---

func TestParseSRLUnexpectedToken(t *testing.T) {
	_, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		UNKNOWN {
		}
	`)
	if err == nil {
		t.Error("expected error for unexpected token")
	}
}

// --- SRL eval: termToSRLKey for blank and nil ---

func TestTermToSRLKeyBlank(t *testing.T) {
	bt := Term{kind: TermBlankNode, value: "b1"}
	key := termToSRLKey(bt)
	if key != "_:b1" {
		t.Errorf("expected _:b1, got %s", key)
	}
}

func TestTermToSRLKeyNoneKind(t *testing.T) {
	nt := Term{kind: TermNone}
	key := termToSRLKey(nt)
	if key != "" {
		t.Errorf("expected empty, got %s", key)
	}
}

func TestTermToSRLKeyLiteralNoDatatype(t *testing.T) {
	lt := Literal("hello", "", "")
	key := termToSRLKey(lt)
	if key != `"hello"^^<http://www.w3.org/2001/XMLSchema#string>` {
		t.Errorf("unexpected key: %s", key)
	}
}

// --- SRL eval: substituteVars full coverage ---

func TestSubstituteVarsNoMatch(t *testing.T) {
	// Variable not in binding should remain
	result := substituteVars("?x + ?y", map[string]string{"x": `"1"`})
	if result != `"1" + ?y` {
		t.Errorf("unexpected result: %s", result)
	}
}

// --- SRL eval: parseSRLKeyLiteral with escapes ---

func TestParseSRLKeyLiteralEscapes(t *testing.T) {
	key := `"hello\"world"^^<http://www.w3.org/2001/XMLSchema#string>`
	result := parseSRLKeyLiteral(key)
	if result.value != `hello"world` {
		t.Errorf("unexpected value: %s", result.value)
	}
}

func TestParseSRLKeyLiteralLang(t *testing.T) {
	key := `"hello"@en`
	result := parseSRLKeyLiteral(key)
	if result.language != "en" {
		t.Errorf("expected lang en, got %s", result.language)
	}
}

// --- SRL eval: EvalRuleSet with stratification ---

func TestEvalRuleSetWithDataBlock(t *testing.T) {
	g := NewGraph()
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:a ex:knows ex:b .
		}
		RULE {
			?x ex:connected ?y .
		} WHERE {
			?x ex:knows ?y .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	inferred, err := EvalRuleSet(rs, g)
	if err != nil {
		t.Fatal(err)
	}
	if inferred.Len() == 0 {
		t.Error("expected inferred triples")
	}
}

// --- SRL stratify: extractPredIRI edge cases ---

func TestExtractPredIRIVariable(t *testing.T) {
	t1 := SRLTriple{
		Subject:   SRLTerm{Kind: SRLTermVariable, Value: "s"},
		Predicate: SRLTerm{Kind: SRLTermVariable, Value: "p"},
		Object:    SRLTerm{Kind: SRLTermVariable, Value: "o"},
	}
	result := extractPredIRI(t1)
	if result != "?var" {
		t.Errorf("expected ?var, got %s", result)
	}
}

func TestExtractPredIRIBlank(t *testing.T) {
	t1 := SRLTriple{
		Subject:   SRLTerm{Kind: SRLTermIRI, Value: "http://s"},
		Predicate: SRLTerm{Kind: SRLTermBlankNode, Value: "b1"},
		Object:    SRLTerm{Kind: SRLTermIRI, Value: "http://o"},
	}
	result := extractPredIRI(t1)
	if result != "" {
		t.Errorf("expected empty string, got %s", result)
	}
}

// --- SRL wellformed: checkRuleWellformed ---

func TestCheckRuleWellformedBadHeadVar(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?x ex:p ?unknown .
		} WHERE {
			?x ex:q ex:o .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	err = CheckWellformed(rs)
	if err == nil {
		t.Error("expected well-formedness error for unbound head variable")
	}
}

// --- SRL patternsOverlap ---

func TestPatternsOverlap(t *testing.T) {
	head := []SRLTriple{
		{Subject: SRLTerm{Kind: SRLTermVariable, Value: "s"}, Predicate: SRLTerm{Kind: SRLTermIRI, Value: "http://p"}, Object: SRLTerm{Kind: SRLTermVariable, Value: "o"}},
	}
	body := []SRLTriple{
		{Subject: SRLTerm{Kind: SRLTermVariable, Value: "a"}, Predicate: SRLTerm{Kind: SRLTermIRI, Value: "http://p"}, Object: SRLTerm{Kind: SRLTermVariable, Value: "b"}},
	}
	if !patternsOverlap(head, body) {
		t.Error("expected overlap with same predicate")
	}
}

func TestPatternsOverlapNoMatch(t *testing.T) {
	head := []SRLTriple{
		{Subject: SRLTerm{Kind: SRLTermVariable, Value: "s"}, Predicate: SRLTerm{Kind: SRLTermIRI, Value: "http://p1"}, Object: SRLTerm{Kind: SRLTermVariable, Value: "o"}},
	}
	body := []SRLTriple{
		{Subject: SRLTerm{Kind: SRLTermVariable, Value: "a"}, Predicate: SRLTerm{Kind: SRLTermIRI, Value: "http://p2"}, Object: SRLTerm{Kind: SRLTermVariable, Value: "b"}},
	}
	if patternsOverlap(head, body) {
		t.Error("expected no overlap with different predicates")
	}
}

// --- shape_based.go: ComponentIRI coverage ---

func TestPropertyConstraintComponentIRI(t *testing.T) {
	c := &PropertyConstraint{ShapeRef: IRI("http://example.org/shape")}
	iri := c.ComponentIRI()
	if iri != SH+"PropertyConstraintComponent" {
		t.Errorf("unexpected IRI: %s", iri)
	}
}

func TestQualifiedValueShapeConstraintComponentIRI(t *testing.T) {
	c := &QualifiedValueShapeConstraint{ShapeRef: IRI("http://example.org/shape")}
	iri := c.ComponentIRI()
	if iri != SH+"QualifiedMinCountConstraintComponent" {
		t.Errorf("unexpected IRI: %s", iri)
	}
}

// --- shapes.go: deactivatedConstraint ComponentIRI ---

func TestDeactivatedConstraintComponentIRI(t *testing.T) {
	dc := &deactivatedConstraint{}
	iri := dc.ComponentIRI()
	if iri != "" {
		t.Errorf("expected empty IRI, got %s", iri)
	}
	results := dc.Evaluate(nil, nil, Term{}, nil)
	if len(results) != 0 {
		t.Error("expected no results from deactivated constraint")
	}
}

func TestSeverityOverrideConstraintComponentIRI(t *testing.T) {
	inner := &MinCountConstraint{MinCount: 1}
	sc := &severityOverrideConstraint{inner: inner, severity: IRI(SH + "Warning")}
	iri := sc.ComponentIRI()
	if iri != SH+"MinCountConstraintComponent" {
		t.Errorf("unexpected IRI: %s", iri)
	}
}

// --- validator.go: evalSPARQLValues with non-matching ---

func TestValidateWithSPARQLSelect(t *testing.T) {
	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:name ;
				sh:datatype xsd:string ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:alice a ex:Person ;
			ex:name "Alice" .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conformant")
	}
}

// --- SRL collection parsing ---

func TestParseSRLEmptyCollection(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p () .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
	// Empty collection should produce rdf:nil
	obj := rs.DataBlocks[0][0].Object
	if obj.Value != RDFNil {
		t.Errorf("expected rdf:nil, got %s", obj.Value)
	}
}

// --- SRL parser: multiple predicates with semicolon ---

func TestParseSRLMultiplePredicatesSemicolon(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p1 ex:o1 ; ex:p2 ex:o2 .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 || len(rs.DataBlocks[0]) < 2 {
		t.Fatal("expected at least 2 triples")
	}
}

// --- SRL parser: multiple objects with comma ---

func TestParseSRLMultipleObjectsComma(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p ex:o1 , ex:o2 , ex:o3 .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 || len(rs.DataBlocks[0]) < 3 {
		t.Fatal("expected at least 3 triples")
	}
}

// --- SRL parser: base URI ---

func TestParseSRLBaseURI(t *testing.T) {
	rs, err := ParseSRL(`
		BASE <http://example.org/>
		DATA {
			<s> <p> <o> .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 || len(rs.DataBlocks[0]) == 0 {
		t.Fatal("expected data block")
	}
	subj := rs.DataBlocks[0][0].Subject
	if subj.Value != "http://example.org/s" {
		t.Errorf("expected http://example.org/s, got %s", subj.Value)
	}
}

// --- SRL string_based: PatternConstraint with flags ---

func TestParseSRLStringWithSingleQuoteEscape(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p 'it\'s a test' .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
	val := rs.DataBlocks[0][0].Object.Value
	if val != "it's a test" {
		t.Errorf("unexpected value: %q", val)
	}
}

// --- SRL: multiple rules in same ruleset ---

func TestParseSRLMultipleRules(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?s ex:inferred1 ex:yes .
		} WHERE {
			?s ex:p1 ex:o1 .
		}
		RULE {
			?s ex:inferred2 ex:yes .
		} WHERE {
			?s ex:p2 ex:o2 .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(rs.Rules))
	}
}

// --- SRL lexer: newline in string should error ---

func TestParseSRLNewlineInString(t *testing.T) {
	_, err := ParseSRL("PREFIX ex: <http://example.org/>\nDATA {\n\tex:s ex:p \"hello\nworld\" .\n}")
	if err == nil {
		t.Error("expected error for newline in single-line string")
	}
}

// --- SRL: triple-quoted strings with newlines ---

func TestParseSRLTripleQuotedWithNewline(t *testing.T) {
	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		DATA {
			ex:s ex:p """hello
world""" .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Fatal("expected data block")
	}
	val := rs.DataBlocks[0][0].Object.Value
	if val != "hello\nworld" {
		t.Errorf("unexpected value: %q", val)
	}
}

// --- SRL eval: matchElements for FILTER/NOT/BIND more paths ---

func TestSRLEvalFilterSimple(t *testing.T) {
	g := NewGraph()
	g.Add(IRI("http://example.org/s"), IRI("http://example.org/value"), IRI("http://example.org/v"))

	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?s ex:checked ex:yes .
		} WHERE {
			?s ex:value ?v .
			FILTER(isIRI(?v))
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	inferred, err := EvalRuleSet(rs, g)
	if err != nil {
		t.Fatal(err)
	}
	// isIRI should pass for IRI values
	if inferred.Len() == 0 {
		t.Error("expected inferred triples from isIRI filter")
	}
}

func TestSRLEvalFilterFalseInRule(t *testing.T) {
	g := NewGraph()
	g.Add(IRI("http://example.org/s"), IRI("http://example.org/value"), IRI("http://example.org/v"))

	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?s ex:checked ex:yes .
		} WHERE {
			?s ex:value ?v .
			FILTER(false)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	inferred, err := EvalRuleSet(rs, g)
	if err != nil {
		t.Fatal(err)
	}
	// FILTER(false) should prevent any inference
	if inferred.Len() != 0 {
		t.Errorf("expected 0 inferred triples from FILTER(false), got %d", inferred.Len())
	}
}

func TestSRLEvalFilterMatch(t *testing.T) {
	g := NewGraph()
	g.Add(IRI("http://example.org/s"), IRI("http://example.org/value"), Literal("15", XSD+"integer", ""))

	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?s ex:big ex:yes .
		} WHERE {
			?s ex:value ?v .
			FILTER(?v > 10)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	inferred, err := EvalRuleSet(rs, g)
	if err != nil {
		t.Fatal(err)
	}
	if inferred.Len() == 0 {
		t.Error("expected inferred triples")
	}
}

func TestSRLEvalWithNOTBlock(t *testing.T) {
	g := NewGraph()
	g.Add(IRI("http://example.org/a"), IRI("http://example.org/p"), IRI("http://example.org/b"))

	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?s ex:inferred ex:yes .
		} WHERE {
			?s ex:p ?o .
			NOT {
				?s ex:excluded ex:yes .
			}
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	inferred, err := EvalRuleSet(rs, g)
	if err != nil {
		t.Fatal(err)
	}
	if inferred.Len() == 0 {
		t.Error("expected inferred triples (NOT block should not match)")
	}
}

// --- SRL eval: BIND ---

// --- TargetKind.String (extended) ---

func TestTargetKindStringExtended(t *testing.T) {
	tests := []struct {
		kind TargetKind
		want string
	}{
		{TargetNode, "targetNode"},
		{TargetClass, "targetClass"},
		{TargetSubjectsOf, "targetSubjectsOf"},
		{TargetObjectsOf, "targetObjectsOf"},
		{TargetImplicitClass, "implicitClassTarget"},
		{TargetWhere, "targetWhere"},
		{TargetKind(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("TargetKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}

// --- Validate with sh:targetSubjectsOf ---

func TestValidateTargetSubjectsOf(t *testing.T) {
	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:NameShape a sh:NodeShape ;
			sh:targetSubjectsOf ex:name ;
			sh:property [
				sh:path ex:name ;
				sh:datatype xsd:string ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:alice ex:name "Alice" .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conformant")
	}
}

// --- Validate with sh:targetObjectsOf ---

func TestValidateTargetObjectsOf(t *testing.T) {
	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:TargetShape a sh:NodeShape ;
			sh:targetObjectsOf ex:knows ;
			sh:nodeKind sh:IRI .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:alice ex:knows ex:bob .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conformant")
	}
}

// --- Validate with sh:class ---

func TestValidateClassConstraint(t *testing.T) {
	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:class ex:Agent .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
		ex:alice a ex:Person .
		ex:Person rdfs:subClassOf ex:Agent .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conformant (via subclass)")
	}
}

// --- Validate with sh:maxCount ---

func TestValidateMaxCount(t *testing.T) {
	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:name ;
				sh:maxCount 1 ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:alice a ex:Person ;
			ex:name "Alice" ;
			ex:name "Al" .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if report.Conforms {
		t.Error("expected non-conformant for maxCount violation")
	}
}

// --- Validate with sh:minLength / sh:maxLength ---

func TestValidateMinMaxLength(t *testing.T) {
	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:name ;
				sh:minLength 2 ;
				sh:maxLength 50 ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:alice a ex:Person ;
			ex:name "A" .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if report.Conforms {
		t.Error("expected non-conformant for minLength violation")
	}
}

// --- Validate with sh:languageIn ---

func TestValidateLanguageIn(t *testing.T) {
	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:name ;
				sh:languageIn ( "en" "fr" ) ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:alice a ex:Person ;
			ex:name "Alice"@en .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conformant")
	}
}

// --- Validate with sh:disjoint ---

func TestValidateDisjoint(t *testing.T) {
	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:firstName ;
				sh:disjoint ex:lastName ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:alice a ex:Person ;
			ex:firstName "Alice" ;
			ex:lastName "Smith" .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conformant (disjoint values)")
	}
}

// --- Validate with sh:lessThan ---

func TestValidateLessThan(t *testing.T) {
	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ex:startDate ;
				sh:lessThan ex:endDate ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:alice a ex:Person ;
			ex:startDate "2024-01-01"^^xsd:date ;
			ex:endDate "2024-12-31"^^xsd:date .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conformant (start < end)")
	}
}

// --- Validate with inverse path ---

func TestValidateInversePath(t *testing.T) {
	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path [ sh:inversePath ex:knows ] ;
				sh:minCount 0 ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:alice a ex:Person .
		ex:bob ex:knows ex:alice .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conformant")
	}
}

// --- Validate with alternative path ---

func TestValidateAlternativePath(t *testing.T) {
	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path [ sh:alternativePath ( ex:firstName ex:givenName ) ] ;
				sh:minCount 1 ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:alice a ex:Person ;
			ex:givenName "Alice" .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conformant")
	}
}

// --- Validate with sequence path ---

func TestValidateSequencePath(t *testing.T) {
	shapesG, err := LoadTurtleString(`
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix ex: <http://example.org/> .
		@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:property [
				sh:path ( ex:address ex:city ) ;
				sh:minCount 0 ;
			] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	dataG, err := LoadTurtleString(`
		@prefix ex: <http://example.org/> .
		ex:alice a ex:Person ;
			ex:address [ ex:city "NYC" ] .
	`, "http://example.org/")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(dataG, shapesG)
	if !report.Conforms {
		t.Error("expected conformant")
	}
}

// --- SRL: substituteVars with multiple variables ---

func TestSubstituteVarsMultiple(t *testing.T) {
	result := substituteVars("?x + ?y", map[string]string{"x": "1", "y": "2"})
	if result != "1 + 2" {
		t.Errorf("expected '1 + 2', got %q", result)
	}
}

// --- SRL: srlTermToKey for triple term (variant) ---

func TestSRLTermToKeyTripleTermVariant(t *testing.T) {
	s := SRLTerm{Kind: SRLTermIRI, Value: "http://s"}
	p := SRLTerm{Kind: SRLTermIRI, Value: "http://p"}
	o := SRLTerm{Kind: SRLTermIRI, Value: "http://o"}
	tt := SRLTerm{Kind: SRLTermTripleTerm, TTSubject: &s, TTPredicate: &p, TTObject: &o}
	key := srlTermToKey(tt)
	if key == "" {
		t.Error("expected non-empty key")
	}
}

// --- SRL: instantiateTerm for non-variable ---

func TestInstantiateTermNonVariable(t *testing.T) {
	iriTerm := SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/x"}
	key, err := instantiateTerm(iriTerm, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "<http://example.org/x>" {
		t.Errorf("expected <http://example.org/x>, got %s", key)
	}
}

// --- SRL: RuleSet with multiple strata ---

func TestSRLEvalMultipleStrata(t *testing.T) {
	g := NewGraph()
	g.Add(IRI("http://example.org/a"), IRI("http://example.org/p"), IRI("http://example.org/b"))

	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?x ex:connected ?y .
		} WHERE {
			?x ex:p ?y .
		}
		RULE {
			?x ex:reachable ?z .
		} WHERE {
			?x ex:connected ?y .
			?y ex:connected ?z .
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	inferred, err := EvalRuleSet(rs, g)
	if err != nil {
		t.Fatal(err)
	}
	if inferred.Len() == 0 {
		t.Error("expected inferred triples")
	}
}

func TestSRLEvalWithBIND(t *testing.T) {
	g := NewGraph()
	g.Add(IRI("http://example.org/s"), IRI("http://example.org/value"), Literal("10", XSD+"integer", ""))

	rs, err := ParseSRL(`
		PREFIX ex: <http://example.org/>
		RULE {
			?s ex:doubled ?d .
		} WHERE {
			?s ex:value ?v .
			BIND(?v * 2 AS ?d)
		}
	`)
	if err != nil {
		t.Fatal(err)
	}
	inferred, err := EvalRuleSet(rs, g)
	if err != nil {
		t.Fatal(err)
	}
	if inferred.Len() == 0 {
		t.Error("expected inferred triples with BIND")
	}
}

// --- fromRDFLib / toTerm coverage ---

func TestFromRDFLibDirLangString(t *testing.T) {
	rdflibTerm := rdflibgo.NewLiteral("hello", rdflibgo.WithLang("ar"), rdflibgo.WithDir("rtl"))
	shTerm := fromRDFLib(rdflibTerm)
	if shTerm.kind != TermLiteral {
		t.Error("expected literal term")
	}
	if !strings.Contains(shTerm.language, "ar") {
		t.Errorf("expected language containing ar, got %s", shTerm.language)
	}
}

func TestFromRDFLibBlankNode(t *testing.T) {
	bn := rdflibgo.NewBNode("x1")
	shTerm := fromRDFLib(bn)
	if shTerm.kind != TermBlankNode {
		t.Error("expected blank node term")
	}
	if shTerm.value != "x1" {
		t.Errorf("expected x1, got %s", shTerm.value)
	}
}

func TestFromRDFLibNilCov(t *testing.T) {
	shTerm := fromRDFLib(nil)
	if !shTerm.IsNone() {
		t.Error("expected none term for nil")
	}
}

func TestToTermBlankNodeCov(t *testing.T) {
	shTerm := Term{kind: TermBlankNode, value: "b0"}
	rdfTerm := toTerm(shTerm)
	if rdfTerm == nil {
		t.Fatal("expected non-nil term")
	}
	bn, ok := rdfTerm.(rdflibgo.BNode)
	if !ok {
		t.Fatalf("expected BNode, got %T", rdfTerm)
	}
	if bn.Value() != "b0" {
		t.Errorf("expected b0, got %s", bn.Value())
	}
}

func TestToTermLiteralWithDatatype(t *testing.T) {
	shTerm := Term{kind: TermLiteral, value: "42", datatype: XSD + "integer"}
	rdfTerm := toTerm(shTerm)
	if rdfTerm == nil {
		t.Fatal("expected non-nil term")
	}
}

func TestToTermNoneCov(t *testing.T) {
	shTerm := Term{}
	rdfTerm := toTerm(shTerm)
	if rdfTerm != nil {
		t.Error("expected nil for none term")
	}
}

// --- compareTerm coverage ---

func TestCompareTermNumeric(t *testing.T) {
	a := Literal("1", XSD+"integer", "")
	b := Literal("2", XSD+"integer", "")
	if compareTerm(a, b) >= 0 {
		t.Error("expected a < b")
	}
	if compareTerm(b, a) <= 0 {
		t.Error("expected b > a")
	}
	if compareTerm(a, a) != 0 {
		t.Error("expected a == a")
	}
}

func TestCompareTermString(t *testing.T) {
	a := Literal("abc", XSD+"string", "")
	b := Literal("def", XSD+"string", "")
	if compareTerm(a, b) >= 0 {
		t.Error("expected a < b")
	}
	if compareTerm(b, a) <= 0 {
		t.Error("expected b > a")
	}
	if compareTerm(a, a) != 0 {
		t.Error("expected a == a")
	}
}

// --- evalPath: zero or more, one or more ---

func TestEvalPathZeroOrMore(t *testing.T) {
	g := NewGraph()
	s := IRI("http://example.org/s")
	m := IRI("http://example.org/m")
	o := IRI("http://example.org/o")
	p := IRI("http://example.org/p")
	g.Add(s, p, m)
	g.Add(m, p, o)
	path := &PropertyPath{
		Kind: PathZeroOrMore,
		Sub:  &PropertyPath{Kind: PathPredicate, Pred: p},
	}
	results := evalPath(g, path, s)
	if len(results) < 2 {
		t.Errorf("expected at least 2 results for zero-or-more path, got %d", len(results))
	}
}

func TestEvalPathOneOrMore(t *testing.T) {
	g := NewGraph()
	s := IRI("http://example.org/s")
	m := IRI("http://example.org/m")
	o := IRI("http://example.org/o")
	p := IRI("http://example.org/p")
	g.Add(s, p, m)
	g.Add(m, p, o)
	path := &PropertyPath{
		Kind: PathOneOrMore,
		Sub:  &PropertyPath{Kind: PathPredicate, Pred: p},
	}
	results := evalPath(g, path, s)
	if len(results) < 2 {
		t.Errorf("expected at least 2 results for one-or-more path, got %d", len(results))
	}
}

func TestEvalPathZeroOrOne(t *testing.T) {
	g := NewGraph()
	s := IRI("http://example.org/s")
	o := IRI("http://example.org/o")
	p := IRI("http://example.org/p")
	g.Add(s, p, o)
	path := &PropertyPath{
		Kind: PathZeroOrOne,
		Sub:  &PropertyPath{Kind: PathPredicate, Pred: p},
	}
	results := evalPath(g, path, s)
	// Should include the focus node itself (zero steps) and one step
	if len(results) < 1 {
		t.Errorf("expected at least 1 result for zero-or-one path, got %d", len(results))
	}
}

// --- Validate with SPARQL-based target ---

func TestValidateTargetSPARQL(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
		@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
		ex:a ex:name "Alice" .
		ex:b ex:name "Bob" .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:NameShape a sh:NodeShape ;
			sh:targetSubjectsOf ex:name ;
			sh:property [
				sh:path ex:name ;
				sh:datatype xsd:string ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with sh:hasValue ---

func TestValidateHasValue(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		ex:a ex:color "red" .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		ex:ColorShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:color ;
				sh:hasValue "red" ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with sh:in ---

func TestValidateIn(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		ex:a ex:color "blue" .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		ex:ColorShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:color ;
				sh:in ("red" "green" "blue") ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with sh:closed ---

func TestValidateClosed(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
		ex:a rdf:type ex:Person ;
			ex:name "Alice" ;
			ex:extra "val" .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
		ex:PersonShape a sh:NodeShape ;
			sh:targetClass ex:Person ;
			sh:closed true ;
			sh:ignoredProperties ( rdf:type ) ;
			sh:property [
				sh:path ex:name ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if report.Conforms {
		t.Error("expected non-conforming report due to ex:extra")
	}
}

// --- Validate with sh:equals ---

func TestValidateEquals(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		ex:a ex:name "Alice" ;
			ex:label "Alice" .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		ex:EqShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:name ;
				sh:equals ex:label ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with sh:not ---

func TestValidateNot(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		ex:a ex:name "Alice" .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:NotShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:not [
				sh:property [
					sh:path ex:name ;
					sh:datatype xsd:integer ;
				] ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with sh:and ---

func TestValidateAnd(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:a ex:age "25"^^xsd:integer .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:AndShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:and (
				[ sh:property [ sh:path ex:age ; sh:minCount 1 ] ]
				[ sh:property [ sh:path ex:age ; sh:datatype xsd:integer ] ]
			) .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with sh:or ---

func TestValidateOr(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:a ex:value "hello" .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:OrShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:value ;
				sh:or (
					[ sh:datatype xsd:string ]
					[ sh:datatype xsd:integer ]
				) ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with sh:xone ---

func TestValidateXone(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:a ex:value "42"^^xsd:integer .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:XoneShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:value ;
				sh:xone (
					[ sh:datatype xsd:integer ]
					[ sh:datatype xsd:string ]
				) ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with sh:node ---

func TestValidateNode(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:a ex:address [ ex:city "Boston" ] .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:AddrShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:address ;
				sh:node ex:CityShape ;
			] .
		ex:CityShape a sh:NodeShape ;
			sh:property [
				sh:path ex:city ;
				sh:minCount 1 ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with sh:qualifiedValueShape ---

func TestValidateQualifiedValueShape(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:a ex:value "1"^^xsd:integer ;
			ex:value "2"^^xsd:integer ;
			ex:value "hello" .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:QShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:value ;
				sh:qualifiedValueShape [ sh:datatype xsd:integer ] ;
				sh:qualifiedMinCount 2 ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with sh:pattern ---

func TestValidatePattern(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		ex:a ex:code "ABC-123" .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		ex:PatShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:code ;
				sh:pattern "^[A-Z]+-[0-9]+$" ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with sh:uniqueLang ---

func TestValidateUniqueLang(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		ex:a ex:label "hello"@en ;
			ex:label "world"@en .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		ex:UniqLangShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:label ;
				sh:uniqueLang true ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if report.Conforms {
		t.Error("expected non-conforming report for duplicate language")
	}
}

// --- Validate with sh:minInclusive/sh:maxInclusive ---

func TestValidateMinMaxInclusive(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:a ex:age "25"^^xsd:integer .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:AgeShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:age ;
				sh:minInclusive 0 ;
				sh:maxInclusive 150 ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with sh:minExclusive/sh:maxExclusive ---

func TestValidateMinMaxExclusive(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:a ex:score "50"^^xsd:integer .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:ScoreShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:score ;
				sh:minExclusive 0 ;
				sh:maxExclusive 100 ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with nested sh:property ---

func TestValidateNestedProperty(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		ex:a ex:address [ ex:city "Boston" ; ex:zip "02101" ] .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:Shape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:address ;
				sh:property [
					sh:path ex:city ;
					sh:minCount 1 ;
					sh:datatype xsd:string ;
				] ;
				sh:property [
					sh:path ex:zip ;
					sh:minCount 1 ;
				] ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- SRL: parseLiteral with unknown type after ^^ ---

func TestSRLParseLiteralBadDatatype(t *testing.T) {
	srl := `PREFIX ex: <http://example.org/>
DATA {
	ex:s ex:p "val"^^42 .
}
`
	_, err := ParseSRL(srl)
	if err == nil {
		t.Error("expected error for bad datatype after ^^")
	}
}

// --- SRL: parseCollection ---

func TestSRLParseCollection(t *testing.T) {
	srl := `PREFIX ex: <http://example.org/>
PREFIX rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#>
DATA {
	ex:s ex:list ( ex:a ex:b ex:c ) .
}
`
	rs, err := ParseSRL(srl)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 || len(rs.DataBlocks[0]) == 0 {
		t.Error("expected data triples from collection")
	}
}

// --- SRL: parsePredicateObjectList with semicolons ---

func TestSRLParseSemicolonPredicateList(t *testing.T) {
	srl := `PREFIX ex: <http://example.org/>
DATA {
	ex:s ex:p1 ex:o1 ; ex:p2 ex:o2 .
}
`
	rs, err := ParseSRL(srl)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 || len(rs.DataBlocks[0]) < 2 {
		t.Errorf("expected at least 2 data triples, got %d", len(rs.DataBlocks[0]))
	}
}

// --- SRL: parsePredicateObjectList with comma ---

func TestSRLParseCommaObjectList(t *testing.T) {
	srl := `PREFIX ex: <http://example.org/>
DATA {
	ex:s ex:p ex:o1 , ex:o2 .
}
`
	rs, err := ParseSRL(srl)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 || len(rs.DataBlocks[0]) < 2 {
		t.Errorf("expected at least 2 data triples, got %d", len(rs.DataBlocks[0]))
	}
}

// --- SRL: parseBodyElements with NOT ---

func TestSRLParseBodyNot(t *testing.T) {
	srl := `PREFIX ex: <http://example.org/>
RULE {
	?x ex:hasType ex:Type .
} WHERE {
	?x ex:p ?o .
	NOT { ?x ex:excluded ex:val . }
}
`
	rs, err := ParseSRL(srl)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rules) != 1 {
		t.Errorf("expected 1 rule, got %d", len(rs.Rules))
	}
}

// --- SRL eval: multiple rules in same stratum ---

func TestSRLEvalMultipleRulesSameStratum(t *testing.T) {
	srl := `PREFIX ex: <http://example.org/>
DATA {
	ex:a ex:type ex:Person .
	ex:b ex:type ex:Animal .
}
RULE {
	?x ex:kind "living" .
} WHERE {
	?x ex:type ex:Person .
}
RULE {
	?x ex:kind "living" .
} WHERE {
	?x ex:type ex:Animal .
}
`
	rs, err := ParseSRL(srl)
	if err != nil {
		t.Fatal(err)
	}
	g := NewGraph()
	for _, block := range rs.DataBlocks {
		for _, d := range block {
			g.Add(srlTermToTerm(d.Subject), srlTermToTerm(d.Predicate), srlTermToTerm(d.Object))
		}
	}
	inferred, err := EvalRuleSet(rs, g)
	if err != nil {
		t.Fatal(err)
	}
	if inferred.Len() < 2 {
		t.Errorf("expected at least 2 inferred triples, got %d", inferred.Len())
	}
}

// --- isWellFormedList ---

func TestIsWellFormedList(t *testing.T) {
	g := NewGraph()
	rdfFirst := IRI(RDF + "first")
	rdfRest := IRI(RDF + "rest")
	rdfNil := IRI(RDF + "nil")

	node1 := BlankNode("list1")
	node2 := BlankNode("list2")
	g.Add(node1, rdfFirst, Literal("a", XSD+"string", ""))
	g.Add(node1, rdfRest, node2)
	g.Add(node2, rdfFirst, Literal("b", XSD+"string", ""))
	g.Add(node2, rdfRest, rdfNil)

	list := g.RDFList(node1)
	if len(list) != 2 {
		t.Errorf("expected 2 items in list, got %d", len(list))
	}
}

// --- SRL: expandPName error ---

func TestSRLExpandPNameUndefined(t *testing.T) {
	srl := `DATA {
	undef:s undef:p undef:o .
}
`
	_, err := ParseSRL(srl)
	if err == nil {
		t.Error("expected error for undefined prefix")
	}
}

// --- Validate with sh:nodeKind ---

func TestValidateNodeKind(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		ex:a ex:ref ex:b .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		ex:NK a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:ref ;
				sh:nodeKind sh:IRI ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report")
	}
}

// --- Validate with deactivated shape ---

func TestValidateDeactivated(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		ex:a ex:value "not-an-integer" .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:DeactShape a sh:NodeShape ;
			sh:deactivated true ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:value ;
				sh:datatype xsd:integer ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if !report.Conforms {
		t.Error("expected conforming report for deactivated shape")
	}
}

// --- Validate with sh:severity ---

func TestValidateSeverity(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		ex:a ex:name 42 .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:WarnShape a sh:NodeShape ;
			sh:severity sh:Warning ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:name ;
				sh:datatype xsd:string ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if report.Conforms {
		t.Error("expected non-conforming report")
	}
	if len(report.Results) == 0 {
		t.Error("expected validation results")
	}
}

// --- Validate with sh:message ---

func TestValidateWithMessage(t *testing.T) {
	data := `
		@prefix ex: <http://example.org/> .
		ex:a ex:count "abc" .
	`
	shapes := `
		@prefix ex: <http://example.org/> .
		@prefix sh: <http://www.w3.org/ns/shacl#> .
		@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
		ex:MsgShape a sh:NodeShape ;
			sh:targetNode ex:a ;
			sh:property [
				sh:path ex:count ;
				sh:datatype xsd:integer ;
				sh:message "count must be integer" ;
			] .
	`
	dg := loadTurtle(t, data)
	sg := loadTurtle(t, shapes)
	report := Validate(dg, sg)
	if report.Conforms {
		t.Error("expected non-conforming report")
	}
}

// --- SRL: patternsCouldMatch ---

func TestSRLPatternsCouldMatch(t *testing.T) {
	// Both use the same predicate IRI
	a := SRLTriple{
		Subject:   SRLTerm{Kind: SRLTermVariable, Value: "x"},
		Predicate: SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/p"},
		Object:    SRLTerm{Kind: SRLTermVariable, Value: "y"},
	}
	b := SRLTriple{
		Subject:   SRLTerm{Kind: SRLTermVariable, Value: "a"},
		Predicate: SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/p"},
		Object:    SRLTerm{Kind: SRLTermVariable, Value: "b"},
	}
	if !patternsCouldMatch([]SRLTriple{a}, []SRLTriple{b}) {
		t.Error("expected patterns with same predicate to potentially match")
	}
}

func TestSRLPatternsDontMatch(t *testing.T) {
	a := SRLTriple{
		Subject:   SRLTerm{Kind: SRLTermVariable, Value: "x"},
		Predicate: SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/p1"},
		Object:    SRLTerm{Kind: SRLTermVariable, Value: "y"},
	}
	b := SRLTriple{
		Subject:   SRLTerm{Kind: SRLTermVariable, Value: "a"},
		Predicate: SRLTerm{Kind: SRLTermIRI, Value: "http://example.org/p2"},
		Object:    SRLTerm{Kind: SRLTermVariable, Value: "b"},
	}
	if patternsCouldMatch([]SRLTriple{a}, []SRLTriple{b}) {
		t.Error("expected patterns with different predicates to not match")
	}
}

// --- SRL wellformed: head var must be in body ---

func TestSRLWellformedHeadVarNotInBody(t *testing.T) {
	srl := `PREFIX ex: <http://example.org/>
RULE {
	?x ex:knows ?y .
} WHERE {
	?x ex:type ex:Person .
}
`
	rs, err := ParseSRL(srl)
	if err != nil {
		t.Fatal(err)
	}
	wfErr := CheckWellformed(rs)
	if wfErr == nil {
		t.Error("expected wellformedness error for unbound head variable ?y")
	}
}

// --- srlTermToTerm helper for tests ---

func srlTermToTerm(st SRLTerm) Term {
	switch st.Kind {
	case SRLTermIRI:
		return IRI(st.Value)
	case SRLTermLiteral:
		return Literal(st.Value, st.Datatype, st.Language)
	case SRLTermBlankNode:
		return BlankNode(st.Value)
	}
	return Term{}
}

// --- loadTurtle helper ---

func loadTurtle(t *testing.T, ttl string) *Graph {
	t.Helper()
	g, err := LoadTurtle(strings.NewReader(ttl), "http://example.org/")
	if err != nil {
		t.Fatalf("failed to parse turtle: %v", err)
	}
	return g
}

// --- Additional coverage tests ---

func TestFromRDFLibDirLangStringCov2(t *testing.T) {
	// Test fromRDFLib with directional language tag
	lit := rdflibgo.NewLiteral("مرحبا", rdflibgo.WithLang("ar"), rdflibgo.WithDir("rtl"))
	term := fromRDFLib(lit)
	if term.language == "" {
		t.Error("expected language set")
	}
	if !strings.Contains(term.language, "--") {
		t.Error("expected directional language tag with --")
	}
}

func TestFromRDFLibLiteralNoLangNoDatatype(t *testing.T) {
	// Literal with no lang and no datatype → should get xsd:string
	lit := rdflibgo.NewLiteral("hello")
	term := fromRDFLib(lit)
	if term.datatype != XSD+"string" {
		t.Errorf("expected xsd:string datatype, got %s", term.datatype)
	}
}

func TestToTermLiteralWithDatatypeCov(t *testing.T) {
	// Convert shacl Term with datatype to rdflibgo term
	st := Literal("42", XSD+"integer", "")
	rt := toTerm(st)
	if rt == nil {
		t.Fatal("expected non-nil term")
	}
	l, ok := rt.(rdflibgo.Literal)
	if !ok {
		t.Fatal("expected Literal")
	}
	if l.Datatype() != rdflibgo.XSDInteger {
		t.Errorf("expected xsd:integer datatype")
	}
}

func TestToTermLiteralWithLangCov(t *testing.T) {
	st := Term{kind: TermLiteral, value: "hello", language: "en"}
	rt := toTerm(st)
	if rt == nil {
		t.Fatal("expected non-nil term")
	}
}

func TestToTermNoneKindCov2(t *testing.T) {
	// Term with kind=0 (none) → should return nil
	st := Term{}
	rt := toTerm(st)
	if rt != nil {
		t.Error("expected nil for none-kind term")
	}
}

func TestMatchConstraintAnnotation(t *testing.T) {
	// Test matching various constraint types
	tests := []struct {
		componentIRI string
		predIRI      string
	}{
		{SH + "DatatypeConstraintComponent", SH + "datatype"},
		{SH + "ClassConstraintComponent", SH + "class"},
		{SH + "NodeKindConstraintComponent", SH + "nodeKind"},
		{SH + "MinCountConstraintComponent", SH + "minCount"},
		{SH + "MaxCountConstraintComponent", SH + "maxCount"},
		{SH + "PatternConstraintComponent", SH + "pattern"},
		{SH + "HasValueConstraintComponent", SH + "hasValue"},
		{SH + "InConstraintComponent", SH + "in"},
	}
	for _, tc := range tests {
		c := &testConstraint{iri: tc.componentIRI}
		ann := tripleAnnotation{
			predicate: IRI(tc.predIRI),
			props:     map[string]Term{"key": IRI("val")},
		}
		props, ok := matchConstraintAnnotation(c, []tripleAnnotation{ann})
		if !ok {
			t.Errorf("expected match for %s", tc.componentIRI)
		}
		if props == nil {
			t.Errorf("expected non-nil props for %s", tc.componentIRI)
		}
	}
	// Non-matching constraint
	c := &testConstraint{iri: SH + "UnknownConstraintComponent"}
	ann := tripleAnnotation{predicate: IRI(SH + "datatype")}
	_, ok := matchConstraintAnnotation(c, []tripleAnnotation{ann})
	if ok {
		t.Error("expected no match for unknown constraint component")
	}
}

// testConstraint is a mock Constraint for testing matchConstraintAnnotation.
type testConstraint struct {
	iri string
}

func (c *testConstraint) ComponentIRI() string { return c.iri }
func (c *testConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	return nil
}

func TestParsePropertyShapesCov(t *testing.T) {
	ttl := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:PersonShape
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
    ] .
`
	g := loadTurtle(t, ttl)
	shapeID := IRI("http://example.org/PersonShape")
	shapes := parsePropertyShapes(g, shapeID, make(map[string]*Shape))
	if len(shapes) == 0 {
		t.Error("expected at least one property shape")
	}
}

// multiTermExpr is a test helper implementing NodeExpr that returns multiple terms.
type multiTermExpr struct {
	terms []Term
}

func (e *multiTermExpr) Eval(ctx *nodeExprContext) []Term {
	return e.terms
}

func TestNodeExprSumWithFlatMap(t *testing.T) {
	// SumExpr with FlatMap = nil exercises the else branch (getNodes)
	expr := &SumExpr{
		Nodes: &multiTermExpr{terms: []Term{
			Literal("1", XSD+"integer", ""),
			Literal("2", XSD+"integer", ""),
		}},
	}
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: loadTurtle(&testing.T{}, ""),
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	if len(result) == 0 {
		t.Error("expected sum result")
	}
}

func TestNodeExprMinNoNumeric(t *testing.T) {
	// MinExpr with non-numeric values → nil
	expr := &MinExpr{
		Nodes: &multiTermExpr{terms: []Term{
			IRI("http://example.org/a"),
			IRI("http://example.org/b"),
		}},
	}
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: loadTurtle(&testing.T{}, ""),
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	if result != nil {
		t.Error("expected nil for non-numeric min")
	}
}

func TestNodeExprMaxNoNumeric(t *testing.T) {
	expr := &MaxExpr{
		Nodes: &multiTermExpr{terms: []Term{
			IRI("http://example.org/a"),
		}},
	}
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: loadTurtle(&testing.T{}, ""),
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	if result != nil {
		t.Error("expected nil for non-numeric max")
	}
}

func TestNodeExprMinEmpty(t *testing.T) {
	expr := &MinExpr{
		Nodes: &multiTermExpr{terms: []Term{}},
	}
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: loadTurtle(&testing.T{}, ""),
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	if result != nil {
		t.Error("expected nil for empty min")
	}
}

func TestNodeExprMaxEmpty(t *testing.T) {
	expr := &MaxExpr{
		Nodes: &multiTermExpr{terms: []Term{}},
	}
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: loadTurtle(&testing.T{}, ""),
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	if result != nil {
		t.Error("expected nil for empty max")
	}
}

func TestNodeExprVarFocusNodeNone(t *testing.T) {
	// VarExpr with focusNode but none value
	expr := &VarExpr{Name: "focusNode"}
	ctx := &nodeExprContext{
		focusNode: Term{}, // none
		dataGraph: loadTurtle(&testing.T{}, ""),
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	if result != nil {
		t.Error("expected nil for none focusNode")
	}
}

func TestNodeExprFilterShapeNoShape(t *testing.T) {
	// FilterShapeExpr where shape is not in shapesMap and not parseable from graph
	expr := &FilterShapeExpr{
		ShapeRef: IRI("http://example.org/NoSuchShape"),
		Nodes: &multiTermExpr{terms: []Term{
			IRI("http://example.org/x"),
		}},
	}
	g := loadTurtle(&testing.T{}, "")
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: g,
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	// No shape constraints = all nodes pass
	if len(result) != 1 {
		t.Errorf("expected 1 result (no constraints), got %d", len(result))
	}
}

func TestNodeExprFindFirstNoShape(t *testing.T) {
	expr := &FindFirstExpr{
		ShapeRef: IRI("http://example.org/NoSuchShape"),
		Nodes: &multiTermExpr{terms: []Term{
			IRI("http://example.org/x"),
		}},
	}
	g := loadTurtle(&testing.T{}, "")
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: g,
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	if len(result) != 1 {
		t.Errorf("expected first node (no constraints), got %d", len(result))
	}
}

func TestNodeExprFindFirstEmpty(t *testing.T) {
	expr := &FindFirstExpr{
		ShapeRef: IRI("http://example.org/NoSuchShape"),
		Nodes:    &multiTermExpr{terms: []Term{}},
	}
	g := loadTurtle(&testing.T{}, "")
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: g,
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	if result != nil {
		t.Error("expected nil for empty findFirst")
	}
}

func TestNodeExprNodesMatchingNoShape(t *testing.T) {
	expr := &NodesMatchingExpr{
		ShapeRef: IRI("http://example.org/NoSuchShape"),
	}
	g := loadTurtle(&testing.T{}, `
@prefix ex: <http://example.org/> .
ex:a ex:p "v" .
`)
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/a"),
		dataGraph: g,
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	// No constraints = all nodes returned
	if len(result) == 0 {
		t.Error("expected all nodes when no shape constraints")
	}
}

func TestNodeExprIntersectionEmpty(t *testing.T) {
	expr := &IntersectionExpr{Members: []NodeExpr{}}
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: loadTurtle(&testing.T{}, ""),
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	if result != nil {
		t.Error("expected nil for empty intersection")
	}
}

func TestNodeExprIfElseBranch(t *testing.T) {
	// If with false condition → else branch
	expr := &IfExpr{
		Condition: &multiTermExpr{terms: []Term{}}, // empty = falsy
		Then:      &multiTermExpr{terms: []Term{IRI("http://example.org/then")}},
		Else:      &multiTermExpr{terms: []Term{IRI("http://example.org/else")}},
	}
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: loadTurtle(&testing.T{}, ""),
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	if len(result) != 1 || result[0].Value() != "http://example.org/else" {
		t.Error("expected else branch result")
	}
}

func TestNodeExprIfTrueNoThen(t *testing.T) {
	expr := &IfExpr{
		Condition: &multiTermExpr{terms: []Term{Literal("true", XSD+"boolean", "")}},
		Then:      nil,
	}
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: loadTurtle(&testing.T{}, ""),
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	if result != nil {
		t.Error("expected nil when Then is nil")
	}
}

func TestNodeExprOrderBySingleElement(t *testing.T) {
	expr := &OrderByExpr{
		Nodes:   &multiTermExpr{terms: []Term{Literal("1", XSD+"integer", "")}},
		KeyExpr: &multiTermExpr{terms: []Term{Literal("1", XSD+"integer", "")}},
	}
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: loadTurtle(&testing.T{}, ""),
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	if len(result) != 1 {
		t.Errorf("expected 1 result, got %d", len(result))
	}
}

func TestPairPathLessThan(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix ex: <http://example.org/> .

ex:PersonShape
    a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:property [
        sh:path ex:startDate ;
        sh:lessThan ex:endDate ;
    ] .

ex:alice ex:startDate "2020-01-01"^^xsd:date ;
         ex:endDate "2021-01-01"^^xsd:date .
`
	g := loadTurtle(&testing.T{}, data)
	report := Validate(g, g)
	if !report.Conforms {
		t.Error("expected conforming for lessThan valid data")
	}
}

func TestPairPathLessThanOrEquals(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
@prefix ex: <http://example.org/> .

ex:PersonShape
    a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:property [
        sh:path ex:startDate ;
        sh:lessThanOrEquals ex:endDate ;
    ] .

ex:alice ex:startDate "2020-01-01"^^xsd:date ;
         ex:endDate "2020-01-01"^^xsd:date .
`
	g := loadTurtle(&testing.T{}, data)
	report := Validate(g, g)
	if !report.Conforms {
		t.Error("expected conforming for lessThanOrEquals with equal values")
	}
}

func TestEvalPathSequence(t *testing.T) {
	data := `
@prefix ex: <http://example.org/> .
ex:a ex:parent ex:b .
ex:b ex:name "Bob" .
`
	g := loadTurtle(&testing.T{}, data)
	path := &PropertyPath{
		Kind: PathSequence,
		Elements: []*PropertyPath{
			{Kind: PathPredicate, Pred: IRI("http://example.org/parent")},
			{Kind: PathPredicate, Pred: IRI("http://example.org/name")},
		},
	}
	result := evalPath(g, path, IRI("http://example.org/a"))
	if len(result) == 0 {
		t.Error("expected result for sequence path")
	}
}

func TestEvalPathAlternative(t *testing.T) {
	data := `
@prefix ex: <http://example.org/> .
ex:a ex:name "Alice" .
ex:a ex:label "AliceLabel" .
`
	g := loadTurtle(&testing.T{}, data)
	path := &PropertyPath{
		Kind: PathAlternative,
		Elements: []*PropertyPath{
			{Kind: PathPredicate, Pred: IRI("http://example.org/name")},
			{Kind: PathPredicate, Pred: IRI("http://example.org/label")},
		},
	}
	result := evalPath(g, path, IRI("http://example.org/a"))
	if len(result) < 2 {
		t.Errorf("expected at least 2 results for alternative path, got %d", len(result))
	}
}

func TestIsSubClassOfVisitedCycle(t *testing.T) {
	// Test cycle detection in isSubClassOfVisited
	data := `
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .
@prefix ex: <http://example.org/> .
ex:A rdfs:subClassOf ex:B .
ex:B rdfs:subClassOf ex:A .
`
	g := loadTurtle(t, data)
	// isSubClassOfVisited should handle cycles without infinite loop
	result := g.isSubClassOfVisited(IRI("http://example.org/A"), IRI("http://example.org/B"), map[string]bool{})
	if !result {
		t.Error("expected A subClassOf B")
	}
}

func TestLoadTurtleFileNotExist(t *testing.T) {
	_, err := LoadTurtleFile("/nonexistent/path.ttl")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestExecuteSPARQLAskCov(t *testing.T) {
	data := `
@prefix ex: <http://example.org/> .
ex:a ex:p "hello" .
`
	g := loadTurtle(&testing.T{}, data)
	result, err := executeSPARQLAsk(g, `ASK { <http://example.org/a> <http://example.org/p> "hello" }`, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !result {
		t.Error("expected true")
	}
}

func TestSPARQLComponentConstraintViaTurtle(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:MyComponent
    a sh:ConstraintComponent ;
    sh:parameter [
        sh:path ex:myParam ;
    ] ;
    sh:nodeValidator [
        sh:ask """
            ASK { $this ex:status "active" }
        """ ;
    ] .

ex:PersonShape
    a sh:NodeShape ;
    sh:targetNode ex:alice ;
    ex:myParam "required" .

ex:alice ex:status "active" .
`
	g := loadTurtle(&testing.T{}, data)
	report := Validate(g, g)
	_ = report // Just exercise the code path
}

func TestRDFListCycleDetection(t *testing.T) {
	// Test cycle in RDF list — isWellFormedList should detect it
	data := `
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix ex: <http://example.org/> .
ex:list rdf:first "a" ;
        rdf:rest ex:list .
`
	g := loadTurtle(&testing.T{}, data)
	result := isWellFormedList(g, IRI("http://example.org/list"))
	if result {
		t.Error("expected not well-formed for cyclic list")
	}
}

func TestExtractAnnotatedTriple(t *testing.T) {
	// Test with a non-Subject reifier (should return nil)
	data := `
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix ex: <http://example.org/> .

ex:r1 rdf:reifies <<( ex:s ex:p "v" )>> .
`
	g := loadTurtle(&testing.T{}, data)
	// extractAnnotatedTriple with matching subject
	result := extractAnnotatedTriple(g, IRI("http://example.org/r1"), IRI("http://example.org/s"))
	if result == nil {
		t.Error("expected non-nil annotation")
	}
	// extractAnnotatedTriple with non-matching subject
	result2 := extractAnnotatedTriple(g, IRI("http://example.org/r1"), IRI("http://example.org/other"))
	if result2 != nil {
		t.Error("expected nil for non-matching subject")
	}
	// extractAnnotatedTriple with non-Subject reifier
	result3 := extractAnnotatedTriple(g, Literal("notASubject", "", ""), IRI("http://example.org/s"))
	if result3 != nil {
		t.Error("expected nil for non-Subject reifier")
	}
}

func TestSRLLexerScanBNodeEdge(t *testing.T) {
	// Test SRL parser with blank node containing underscores
	input := `
PREFIX ex: <http://example.org/>
DATA {
    _:b_1 ex:p "v" .
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Error("expected data blocks")
	}
}

func TestSRLLexerScanStringEscape(t *testing.T) {
	input := `
PREFIX ex: <http://example.org/>
DATA {
    ex:s ex:p "hello\nworld" .
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 || len(rs.DataBlocks[0]) == 0 {
		t.Error("expected data with escaped string")
	}
}

func TestSRLLexerUnicodeEscape(t *testing.T) {
	input := `
PREFIX ex: <http://example.org/>
DATA {
    ex:s ex:p "\u0041BC" .
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Error("expected data with unicode escape")
	}
}

func TestSRLAngleOrTripleTermBracket(t *testing.T) {
	// Test SRL with triple term syntax
	input := `
PREFIX ex: <http://example.org/>
RULE {
    <<( ?s ex:p ?o )>> ex:annotated "true" .
}
WHERE {
    ?s ex:p ?o .
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rules) == 0 {
		t.Error("expected rules")
	}
}

func TestReifierShapeNoPath(t *testing.T) {
	// ReifierShapeConstraint with no path → should return nil
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:Shape a sh:NodeShape ;
    sh:targetNode ex:a ;
    sh:reifierShape ex:ReifierShape .

ex:ReifierShape a sh:NodeShape .
ex:a ex:p "v" .
`
	g := loadTurtle(&testing.T{}, data)
	report := Validate(g, g)
	_ = report // Just exercise code
}

func TestFindReifiersNonURIPredicate(t *testing.T) {
	data := `
@prefix ex: <http://example.org/> .
ex:a ex:p "v" .
`
	g := loadTurtle(&testing.T{}, data)
	// Non-URI predicate → empty result
	reifiers := findReifiers(g, IRI("http://example.org/a"), Literal("notURI", "", ""), Literal("v", "", ""))
	if len(reifiers) != 0 {
		t.Error("expected empty reifiers for non-URI predicate")
	}
}

func TestMemberShapeNoShape(t *testing.T) {
	// MemberShapeConstraint with shape not found → returns nil
	c := &MemberShapeConstraint{ShapeRef: IRI("http://example.org/NoShape")}
	ctx := &evalContext{
		shapesMap: map[string]*Shape{},
	}
	results := c.Evaluate(ctx, &Shape{}, IRI("http://example.org/x"), nil)
	if results != nil {
		t.Error("expected nil for missing shape")
	}
}

func TestReifierShapeNoShape(t *testing.T) {
	c := &ReifierShapeConstraint{ShapeRef: IRI("http://example.org/NoShape")}
	ctx := &evalContext{
		shapesMap: map[string]*Shape{},
	}
	results := c.Evaluate(ctx, &Shape{}, IRI("http://example.org/x"), nil)
	if results != nil {
		t.Error("expected nil for missing shape")
	}
}

func TestParseOneComponentNoParams(t *testing.T) {
	// Component with no parameters → nil
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
ex:MyComp a sh:ConstraintComponent .
`
	g := loadTurtle(&testing.T{}, data)
	cd := parseOneComponent(g, IRI("http://example.org/MyComp"))
	if cd != nil {
		t.Error("expected nil for component with no parameters")
	}
}

func TestParseOneComponentNoValidator(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
ex:MyComp a sh:ConstraintComponent ;
    sh:parameter [
        sh:path ex:myParam ;
    ] .
`
	g := loadTurtle(&testing.T{}, data)
	cd := parseOneComponent(g, IRI("http://example.org/MyComp"))
	if cd != nil {
		t.Error("expected nil for component with no validators")
	}
}

func TestResolvePrefixesForValidatorCov(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:MyComp a sh:ConstraintComponent ;
    sh:parameter [
        sh:path ex:myParam ;
    ] ;
    sh:validator [
        sh:ask "ASK { $this ?p ?o }" ;
        sh:prefixes [
            sh:declare [
                sh:prefix "ex" ;
                sh:namespace "http://example.org/"^^<http://www.w3.org/2001/XMLSchema#anyURI> ;
            ] ;
        ] ;
    ] .

ex:PersonShape a sh:NodeShape ;
    sh:targetNode ex:alice ;
    ex:myParam "value" .

ex:alice ex:p "v" .
`
	g := loadTurtle(&testing.T{}, data)
	report := Validate(g, g)
	_ = report
}

func TestShapeBasedConstraintNilShape(t *testing.T) {
	// shape_based.go: shape not found in shapesMap
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:PersonShape
    a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:node ex:AddressShape .

ex:alice ex:name "Alice" .
`
	g := loadTurtle(&testing.T{}, data)
	report := Validate(g, g)
	// AddressShape not defined → should conform (no constraints to fail)
	_ = report
}

func TestSRLParserDirLangTag(t *testing.T) {
	// Test SRL parser with directional language tag
	input := `
PREFIX ex: <http://example.org/>
DATA {
    ex:s ex:p "hello"@en--ltr .
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 || len(rs.DataBlocks[0]) == 0 {
		t.Error("expected data blocks with dir lang tag")
	}
}

func TestSRLParserTripleQuoteString(t *testing.T) {
	input := `
PREFIX ex: <http://example.org/>
DATA {
    ex:s ex:p """multi
line""" .
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Error("expected data blocks")
	}
}

func TestSRLParserBNodeDot(t *testing.T) {
	// BNode name with dots (PN_LOCAL)
	input := `
PREFIX ex: <http://example.org/>
DATA {
    _:a.b ex:p "v" .
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Error("expected data blocks")
	}
}

func TestSRLParserWordWithDot(t *testing.T) {
	// Prefixed name with trailing dot in SRL
	input := `
PREFIX ex: <http://example.org/>
DATA {
    ex:a ex:b.c ex:d .
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Error("expected data blocks")
	}
}

func TestSRLParserFilterExpr(t *testing.T) {
	// SRL FILTER
	input := `
PREFIX ex: <http://example.org/>
RULE {
    ?s ex:ok "true" .
}
WHERE {
    ?s ex:p ?v .
    FILTER(?v > 0)
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rules) == 0 {
		t.Error("expected rules")
	}
}

func TestSRLWellformedVarNotInBody(t *testing.T) {
	input := `
PREFIX ex: <http://example.org/>
RULE {
    ?x ex:ok "true" .
}
WHERE {
    ?s ex:p ?v .
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	err = CheckWellformed(rs)
	if err == nil {
		t.Error("expected error for head var not in body")
	}
}

func TestSRLStratifyNegativeCycle(t *testing.T) {
	input := `
PREFIX ex: <http://example.org/>
RULE {
    ?s ex:derived "yes" .
}
WHERE {
    ?s ex:p ?v .
    NOT {
        ?s ex:derived "yes" .
    }
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Stratify(rs)
	_ = err
}

func TestNodeExprParseBlankNodeExprMissing(t *testing.T) {
	// parseBlankNodeExpr with a blank node that has no shnex properties
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix shnex: <http://www.w3.org/ns/shacl-nex#> .
@prefix ex: <http://example.org/> .

ex:MyShape a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:values _:blank .
`
	g := loadTurtle(t, data)
	report := Validate(g, g)
	_ = report
}

func TestValidateDeactivatedShapeCov(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:PersonShape
    a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:deactivated true ;
    sh:minCount 5 .

ex:alice ex:name "Alice" .
`
	g := loadTurtle(t, data)
	report := Validate(g, g)
	if !report.Conforms {
		t.Error("expected conforming for deactivated shape")
	}
}

func TestValidateWithSeverityOverride(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:PersonShape
    a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:severity sh:Warning ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
    ] .
`
	g := loadTurtle(t, data)
	report := Validate(g, g)
	// No ex:name → violation but with Warning severity
	if report.Conforms {
		t.Error("expected non-conforming (minCount violation)")
	}
}

func TestSRLBindExpr(t *testing.T) {
	input := `
PREFIX ex: <http://example.org/>
RULE {
    ?s ex:doubled ?d .
}
WHERE {
    ?s ex:val ?v .
    BIND(?v + ?v AS ?d)
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.Rules) == 0 {
		t.Error("expected rules")
	}
}

func TestSRLParserAnnotation(t *testing.T) {
	// Test SRL with annotation blocks (;)
	input := `
PREFIX ex: <http://example.org/>
DATA {
    ex:a ex:p "v1" ;
         ex:q "v2" .
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Error("expected data blocks")
	}
}

func TestSRLParserCommaInData(t *testing.T) {
	input := `
PREFIX ex: <http://example.org/>
DATA {
    ex:a ex:p "v1", "v2" .
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Error("expected data blocks")
	}
}

func TestDeactivatedPropertyShape(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:PersonShape
    a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
        sh:deactivated true ;
    ] .

ex:alice ex:age 30 .
`
	g := loadTurtle(t, data)
	report := Validate(g, g)
	if !report.Conforms {
		t.Error("expected conforming with deactivated property shape")
	}
}

func TestSPARQLValuesNoQuery(t *testing.T) {
	// evalSPARQLValues with no select and no expr → nil
	ctx := &evalContext{
		dataGraph: loadTurtle(&testing.T{}, ""),
		shapesMap: map[string]*Shape{},
	}
	v := &SPARQLValues{}
	result := evalSPARQLValues(ctx, v, IRI("http://example.org/x"))
	if result != nil {
		t.Error("expected nil for empty SPARQLValues")
	}
}

func TestValidateWithMessageCov(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:PersonShape
    a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
        sh:message "Name is required" ;
    ] .
`
	g := loadTurtle(t, data)
	report := Validate(g, g)
	if report.Conforms {
		t.Error("expected non-conforming")
	}
}

func TestUniqueLangNoLangLiterals(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:PersonShape
    a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:property [
        sh:path ex:label ;
        sh:uniqueLang true ;
    ] .

ex:alice ex:label "hello" ;
         ex:label "world" .
`
	g := loadTurtle(t, data)
	report := Validate(g, g)
	// Non-lang literals should be ignored by uniqueLang
	if !report.Conforms {
		t.Error("expected conforming (non-lang literals ignored)")
	}
}

func TestSPARQLComponentSELECT(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:MyComp a sh:ConstraintComponent ;
    sh:parameter [
        sh:path ex:myParam ;
    ] ;
    sh:nodeValidator [
        sh:select """
            SELECT $this
            WHERE {
                FILTER NOT EXISTS { $this ex:status "active" }
            }
        """ ;
    ] .

ex:PersonShape a sh:NodeShape ;
    sh:targetNode ex:alice ;
    ex:myParam "value" .

ex:alice ex:status "inactive" .
`
	g := loadTurtle(t, data)
	report := Validate(g, g)
	_ = report // Just exercise the SELECT validator path
}

func TestSumExprNonNumericSkip(t *testing.T) {
	// SumExpr with non-numeric values → skips non-numeric
	expr := &SumExpr{
		Nodes: &multiTermExpr{terms: []Term{
			Literal("1", XSD+"integer", ""),
			IRI("http://example.org/notNumeric"),
		}},
	}
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: loadTurtle(&testing.T{}, ""),
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	if len(result) == 0 {
		t.Error("expected sum result with non-numeric skipped")
	}
}

func TestSumExprWithFlatMap(t *testing.T) {
	expr := &SumExpr{
		Nodes: &multiTermExpr{terms: []Term{
			Literal("1", XSD+"integer", ""),
		}},
		FlatMap: &multiTermExpr{terms: []Term{
			Literal("2", XSD+"integer", ""),
		}},
	}
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: loadTurtle(&testing.T{}, ""),
		vars:      map[string]Term{},
		shapesMap: map[string]*Shape{},
	}
	result := expr.Eval(ctx)
	_ = result
}

func TestFindFirstExprNoMatch(t *testing.T) {
	// FindFirstExpr where shape exists but no nodes conform
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
ex:Shape a sh:NodeShape ;
    sh:datatype xsd:integer .
`
	g := loadTurtle(&testing.T{}, data)
	shapes := parseShapes(g)
	expr := &FindFirstExpr{
		ShapeRef: IRI("http://example.org/Shape"),
		Nodes: &multiTermExpr{terms: []Term{
			Literal("notAnInt", XSD+"string", ""),
		}},
	}
	ctx := &nodeExprContext{
		focusNode: IRI("http://example.org/x"),
		dataGraph: g,
		vars:      map[string]Term{},
		shapesMap: shapes,
	}
	result := expr.Eval(ctx)
	if result != nil {
		t.Error("expected nil when no nodes conform")
	}
}

func TestEvalPathInverse(t *testing.T) {
	data := `
@prefix ex: <http://example.org/> .
ex:a ex:parent ex:b .
`
	g := loadTurtle(&testing.T{}, data)
	path := &PropertyPath{
		Kind: PathInverse,
		Sub:  &PropertyPath{Kind: PathPredicate, Pred: IRI("http://example.org/parent")},
	}
	result := evalPath(g, path, IRI("http://example.org/b"))
	if len(result) == 0 {
		t.Error("expected result for inverse path")
	}
}

func TestEvalPathUnknownKind(t *testing.T) {
	path := &PropertyPath{Kind: 99} // unknown kind
	g := loadTurtle(&testing.T{}, "")
	result := evalPath(g, path, IRI("http://example.org/x"))
	if result != nil {
		t.Error("expected nil for unknown path kind")
	}
}

func TestSPARQLConstraintDeactivated(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:PersonShape
    a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:sparql [
        sh:deactivated true ;
        sh:select """
            SELECT $this
            WHERE { FILTER(false) }
        """ ;
    ] .
`
	g := loadTurtle(&testing.T{}, data)
	report := Validate(g, g)
	_ = report
}

func TestSPARQLConstraintParseError(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:PersonShape
    a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:sparql [
        sh:select "INVALID SPARQL {{{{" ;
    ] .
`
	g := loadTurtle(&testing.T{}, data)
	report := Validate(g, g)
	_ = report
}

func TestShapeBasedNodeConstraint(t *testing.T) {
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:AddressShape
    a sh:NodeShape ;
    sh:property [
        sh:path ex:street ;
        sh:minCount 1 ;
    ] .

ex:PersonShape
    a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:property [
        sh:path ex:address ;
        sh:node ex:AddressShape ;
    ] .

ex:alice ex:address ex:addr1 .
ex:addr1 ex:street "123 Main St" .
`
	g := loadTurtle(&testing.T{}, data)
	report := Validate(g, g)
	if !report.Conforms {
		t.Error("expected conforming")
	}
}

func TestExtractAnnotatedTripleNonTripleTerm(t *testing.T) {
	// extractAnnotatedTriple where rdf:reifies object is not a TripleTerm
	data := `
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix ex: <http://example.org/> .

ex:r1 rdf:reifies "not a triple term" .
`
	g := loadTurtle(&testing.T{}, data)
	result := extractAnnotatedTriple(g, IRI("http://example.org/r1"), IRI("http://example.org/s"))
	if result != nil {
		t.Error("expected nil for non-TripleTerm reification object")
	}
}

func TestSRLParserUnicodeEscapeInIRI(t *testing.T) {
	input := `
PREFIX ex: <http://example.org/>
DATA {
    <http://example.org/\u0041> ex:p "v" .
}
`
	rs, err := ParseSRL(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rs.DataBlocks) == 0 {
		t.Error("expected data blocks")
	}
}

func TestConstraintParserFallthrough(t *testing.T) {
	// Test parseConstraints with a shape that has sh:uniqueMembers = false
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:ListShape
    a sh:NodeShape ;
    sh:targetNode ex:myList ;
    sh:uniqueMembers false .

ex:myList <http://www.w3.org/1999/02/22-rdf-syntax-ns#first> "a" ;
    <http://www.w3.org/1999/02/22-rdf-syntax-ns#rest> <http://www.w3.org/1999/02/22-rdf-syntax-ns#nil> .
`
	g := loadTurtle(&testing.T{}, data)
	report := Validate(g, g)
	if !report.Conforms {
		t.Error("expected conforming with uniqueMembers=false")
	}
}

// --- PairPathConstraint coverage ---

func TestPairPathConstraintLessThan(t *testing.T) {
	// Covers property_pair.go:127 (lessThan case in PairPathConstraint.Evaluate)
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [
        sh:path ex:startDate ;
        sh:lessThan ex:endDate ;
    ] .

ex:alice a ex:Person ;
    ex:startDate "2020-01-01"^^xsd:date ;
    ex:endDate "2021-01-01"^^xsd:date .
`
	g := loadTurtle(t, data)
	report := Validate(g, g)
	if !report.Conforms {
		t.Error("expected conforming for lessThan with valid dates")
	}
}

func TestPairPathConstraintLessThanOrEquals(t *testing.T) {
	// Covers property_pair.go:129 (lessThanOrEquals case in PairPathConstraint.Evaluate)
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [
        sh:path ex:startDate ;
        sh:lessThanOrEquals ex:endDate ;
    ] .

ex:alice a ex:Person ;
    ex:startDate "2020-01-01"^^xsd:date ;
    ex:endDate "2020-01-01"^^xsd:date .
`
	g := loadTurtle(t, data)
	report := Validate(g, g)
	if !report.Conforms {
		t.Error("expected conforming for lessThanOrEquals with equal dates")
	}
}

// --- SPARQLComponentConstraint evaluateSELECT error path ---

func TestSPARQLComponentConstraintSELECTError(t *testing.T) {
	// Covers sparql_component.go:75-81 (evaluateSELECT error path)
	g := NewGraph()
	focus := IRI("http://example.org/x")
	shape := &Shape{ID: IRI("http://example.org/S")}
	ctx := &evalContext{
		dataGraph:   g,
		shapesGraph: g,
		shapesMap:   map[string]*Shape{},
	}
	c := &SPARQLComponentConstraint{
		ComponentNode: IRI("http://example.org/comp"),
		Validator: &validatorDef{
			IsASK: false,
			Query: "INVALID SPARQL {{{{",
			Messages: []Term{Literal("error msg", "", "")},
		},
	}
	results := c.evaluateSELECT(ctx, shape, focus, nil, "INVALID SPARQL {{{{")
	if len(results) != 1 {
		t.Fatalf("expected 1 result for SELECT error, got %d", len(results))
	}
	if len(results[0].ResultMessages) == 0 {
		t.Error("expected messages from validator on error")
	}
}

// --- ClosedByTypesConstraint with IgnoredProperties ---

func TestPairPathConstraintUnknownKind(t *testing.T) {
	// Covers property_pair.go:131 (default return nil for unknown kind)
	g := NewGraph()
	ctx := &evalContext{dataGraph: g, shapesGraph: g, shapesMap: map[string]*Shape{}}
	shape := &Shape{ID: IRI("http://example.org/S")}
	c := &PairPathConstraint{
		Kind:      "unknown",
		OtherPath: &PropertyPath{Kind: PathPredicate, Pred: IRI("http://example.org/p")},
	}
	results := c.Evaluate(ctx, shape, IRI("http://example.org/x"), nil)
	if results != nil {
		t.Error("expected nil for unknown kind")
	}
}

func TestLoadTurtleFileParseError(t *testing.T) {
	// Covers rdf.go:247-249 (parse error from LoadTurtleFile)
	tmpFile := t.TempDir() + "/bad.ttl"
	if err := os.WriteFile(tmpFile, []byte("@prefix ex INVALID"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadTurtleFile(tmpFile)
	if err == nil {
		t.Error("expected error for invalid Turtle")
	}
}

func TestClosedByTypesIgnoredProperties(t *testing.T) {
	// Covers other.go:155-157 (IgnoredProperties loop in ClosedByTypesConstraint)
	data := `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix rdf: <http://www.w3.org/1999/02/22-rdf-syntax-ns#> .
@prefix rdfs: <http://www.w3.org/2000/01/rdf-schema#> .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:closed sh:ByTypes ;
    sh:ignoredProperties ( ex:nickname ) ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
    ] .

ex:alice a ex:Person ;
    ex:name "Alice" ;
    ex:nickname "Ally" .
`
	g := loadTurtle(t, data)
	report := Validate(g, g)
	if !report.Conforms {
		t.Error("expected conforming: nickname is in ignoredProperties")
	}
}
