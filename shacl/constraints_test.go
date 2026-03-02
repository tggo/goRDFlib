package shacl

import (
	"testing"
)

// helpers

func testCtx(dataG *Graph) *evalContext {
	return &evalContext{
		dataGraph:   dataG,
		shapesGraph: NewGraph(),
		shapesMap:   map[string]*Shape{},
	}
}

func testShape() *Shape {
	return &Shape{
		ID:       IRI("http://example.org/TestShape"),
		Severity: SHViolation,
	}
}

func intLit(v string) Term        { return Literal(v, XSD+"integer", "") }
func langLit(v, lang string) Term { return Literal(v, "", lang) }

// --- ClassConstraint ---

func TestClassConstraint(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	node := IRI(ex + "n1")
	cls := IRI(ex + "Person")
	g.Add(node, IRI(RDFType), cls)

	ctx := testCtx(g)
	s := testShape()
	c := &ClassConstraint{Class: cls}

	// passes: node has type Person
	results := c.Evaluate(ctx, s, node, []Term{node})
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}

	// fails: node2 has no type
	node2 := IRI(ex + "n2")
	results = c.Evaluate(ctx, s, node2, []Term{node2})
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

// --- DatatypeConstraint ---

func TestDatatypeConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	dt := XSD + "integer"
	c := &DatatypeConstraint{Datatype: IRI(dt)}

	tests := []struct {
		name    string
		vn      Term
		wantErr bool
	}{
		{"valid integer", intLit("42"), false},
		{"wrong datatype", Literal("hello", "", ""), true},
		{"IRI not literal", IRI(ex + "x"), true},
		{"blank not literal", BlankNode("b1"), true},
		{"ill-formed integer", Literal("abc", XSD+"integer", ""), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := c.Evaluate(ctx, s, IRI(ex+"focus"), []Term{tc.vn})
			if tc.wantErr && len(results) == 0 {
				t.Error("expected violation")
			}
			if !tc.wantErr && len(results) != 0 {
				t.Errorf("expected no violation, got %d", len(results))
			}
		})
	}
}

// --- NodeKindConstraint ---

func TestNodeKindConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")

	tests := []struct {
		name     string
		nodeKind Term
		vn       Term
		wantErr  bool
	}{
		{"IRI pass", IRI(SH + "IRI"), IRI(ex + "x"), false},
		{"IRI fail on literal", IRI(SH + "IRI"), Literal("x", "", ""), true},
		{"IRI fail on blank", IRI(SH + "IRI"), BlankNode("b"), true},
		{"BlankNode pass", IRI(SH + "BlankNode"), BlankNode("b"), false},
		{"BlankNode fail on IRI", IRI(SH + "BlankNode"), IRI(ex + "x"), true},
		{"Literal pass", IRI(SH + "Literal"), Literal("x", "", ""), false},
		{"Literal fail on IRI", IRI(SH + "Literal"), IRI(ex + "x"), true},
		{"BlankNodeOrIRI pass IRI", IRI(SH + "BlankNodeOrIRI"), IRI(ex + "x"), false},
		{"BlankNodeOrIRI pass blank", IRI(SH + "BlankNodeOrIRI"), BlankNode("b"), false},
		{"BlankNodeOrIRI fail literal", IRI(SH + "BlankNodeOrIRI"), Literal("x", "", ""), true},
		{"BlankNodeOrLiteral pass blank", IRI(SH + "BlankNodeOrLiteral"), BlankNode("b"), false},
		{"BlankNodeOrLiteral pass literal", IRI(SH + "BlankNodeOrLiteral"), Literal("x", "", ""), false},
		{"BlankNodeOrLiteral fail IRI", IRI(SH + "BlankNodeOrLiteral"), IRI(ex + "x"), true},
		{"IRIOrLiteral pass IRI", IRI(SH + "IRIOrLiteral"), IRI(ex + "x"), false},
		{"IRIOrLiteral pass literal", IRI(SH + "IRIOrLiteral"), Literal("x", "", ""), false},
		{"IRIOrLiteral fail blank", IRI(SH + "IRIOrLiteral"), BlankNode("b"), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &NodeKindConstraint{NodeKind: tc.nodeKind}
			results := c.Evaluate(ctx, s, focus, []Term{tc.vn})
			if tc.wantErr && len(results) == 0 {
				t.Error("expected violation")
			}
			if !tc.wantErr && len(results) != 0 {
				t.Errorf("expected no violation, got %d", len(results))
			}
		})
	}
}

// --- MinCountConstraint / MaxCountConstraint ---

func TestMinCountConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")

	tests := []struct {
		name    string
		min     int
		nodes   []Term
		wantErr bool
	}{
		{"exact", 2, []Term{Literal("a", "", ""), Literal("b", "", "")}, false},
		{"more", 1, []Term{Literal("a", "", ""), Literal("b", "", "")}, false},
		{"fewer", 3, []Term{Literal("a", "", "")}, true},
		{"zero min always passes", 0, nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &MinCountConstraint{MinCount: tc.min}
			results := c.Evaluate(ctx, s, focus, tc.nodes)
			if tc.wantErr && len(results) == 0 {
				t.Error("expected violation")
			}
			if !tc.wantErr && len(results) != 0 {
				t.Error("expected no violation")
			}
		})
	}
}

func TestMaxCountConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")

	tests := []struct {
		name    string
		max     int
		nodes   []Term
		wantErr bool
	}{
		{"exact", 2, []Term{Literal("a", "", ""), Literal("b", "", "")}, false},
		{"fewer", 3, []Term{Literal("a", "", "")}, false},
		{"more", 1, []Term{Literal("a", "", ""), Literal("b", "", "")}, true},
		{"zero max empty passes", 0, nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &MaxCountConstraint{MaxCount: tc.max}
			results := c.Evaluate(ctx, s, focus, tc.nodes)
			if tc.wantErr && len(results) == 0 {
				t.Error("expected violation")
			}
			if !tc.wantErr && len(results) != 0 {
				t.Error("expected no violation")
			}
		})
	}
}

// --- Value Range Constraints ---

func TestMinExclusiveConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")
	c := &MinExclusiveConstraint{Value: intLit("5")}

	tests := []struct {
		name    string
		vn      Term
		wantErr bool
	}{
		{"greater", intLit("6"), false},
		{"equal", intLit("5"), true},
		{"less", intLit("4"), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := c.Evaluate(ctx, s, focus, []Term{tc.vn})
			if tc.wantErr && len(results) == 0 {
				t.Error("expected violation")
			}
			if !tc.wantErr && len(results) != 0 {
				t.Error("expected no violation")
			}
		})
	}
}

func TestMinInclusiveConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")
	c := &MinInclusiveConstraint{Value: intLit("5")}

	if r := c.Evaluate(ctx, s, focus, []Term{intLit("5")}); len(r) != 0 {
		t.Error("equal should pass")
	}
	if r := c.Evaluate(ctx, s, focus, []Term{intLit("6")}); len(r) != 0 {
		t.Error("greater should pass")
	}
	if r := c.Evaluate(ctx, s, focus, []Term{intLit("4")}); len(r) == 0 {
		t.Error("less should fail")
	}
}

func TestMaxExclusiveConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")
	c := &MaxExclusiveConstraint{Value: intLit("5")}

	if r := c.Evaluate(ctx, s, focus, []Term{intLit("4")}); len(r) != 0 {
		t.Error("less should pass")
	}
	if r := c.Evaluate(ctx, s, focus, []Term{intLit("5")}); len(r) == 0 {
		t.Error("equal should fail")
	}
	if r := c.Evaluate(ctx, s, focus, []Term{intLit("6")}); len(r) == 0 {
		t.Error("greater should fail")
	}
}

func TestMaxInclusiveConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")
	c := &MaxInclusiveConstraint{Value: intLit("5")}

	if r := c.Evaluate(ctx, s, focus, []Term{intLit("5")}); len(r) != 0 {
		t.Error("equal should pass")
	}
	if r := c.Evaluate(ctx, s, focus, []Term{intLit("4")}); len(r) != 0 {
		t.Error("less should pass")
	}
	if r := c.Evaluate(ctx, s, focus, []Term{intLit("6")}); len(r) == 0 {
		t.Error("greater should fail")
	}
}

// --- String-Based Constraints ---

func TestMinLengthConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")

	tests := []struct {
		name    string
		minLen  int
		vn      Term
		wantErr bool
	}{
		{"literal pass", 3, Literal("abc", "", ""), false},
		{"literal fail", 4, Literal("abc", "", ""), true},
		{"IRI uses IRI string", 5, IRI("http://x"), false},
		{"blank fails", 1, BlankNode("b1"), true},
		{"unicode rune count", 2, Literal("\u00e9\u00e8", "", ""), false}, // 2 runes
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := &MinLengthConstraint{MinLength: tc.minLen}
			results := c.Evaluate(ctx, s, focus, []Term{tc.vn})
			if tc.wantErr && len(results) == 0 {
				t.Error("expected violation")
			}
			if !tc.wantErr && len(results) != 0 {
				t.Errorf("expected no violation, got %d", len(results))
			}
		})
	}
}

func TestMaxLengthConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")

	c := &MaxLengthConstraint{MaxLength: 3}
	if r := c.Evaluate(ctx, s, focus, []Term{Literal("abc", "", "")}); len(r) != 0 {
		t.Error("exact length should pass")
	}
	if r := c.Evaluate(ctx, s, focus, []Term{Literal("abcd", "", "")}); len(r) == 0 {
		t.Error("too long should fail")
	}
	if r := c.Evaluate(ctx, s, focus, []Term{Literal("ab", "", "")}); len(r) != 0 {
		t.Error("shorter should pass")
	}
}

func TestPatternConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")

	tests := []struct {
		name    string
		pattern string
		flags   string
		vn      Term
		wantErr bool
	}{
		{"match", "^abc$", "", Literal("abc", "", ""), false},
		{"no match", "^abc$", "", Literal("xyz", "", ""), true},
		{"case insensitive", "^ABC$", "i", Literal("abc", "", ""), false},
		{"IRI value", "example", "", IRI(ex + "test"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := NewPatternConstraint(tc.pattern, tc.flags)
			results := c.Evaluate(ctx, s, focus, []Term{tc.vn})
			if tc.wantErr && len(results) == 0 {
				t.Error("expected violation")
			}
			if !tc.wantErr && len(results) != 0 {
				t.Error("expected no violation")
			}
		})
	}
}

func TestLanguageInConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")

	c := &LanguageInConstraint{Languages: []string{"en", "fr"}}

	tests := []struct {
		name    string
		vn      Term
		wantErr bool
	}{
		{"exact match en", langLit("hello", "en"), false},
		{"prefix match en-US", langLit("hello", "en-US"), false},
		{"exact match fr", langLit("bonjour", "fr"), false},
		{"no match de", langLit("hallo", "de"), true},
		{"not a lang literal", Literal("hello", "", ""), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			results := c.Evaluate(ctx, s, focus, []Term{tc.vn})
			if tc.wantErr && len(results) == 0 {
				t.Error("expected violation")
			}
			if !tc.wantErr && len(results) != 0 {
				t.Error("expected no violation")
			}
		})
	}
}

func TestUniqueLangConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")
	c := &UniqueLangConstraint{UniqueLang: true}

	// no duplicates
	nodes := []Term{langLit("hello", "en"), langLit("bonjour", "fr")}
	if r := c.Evaluate(ctx, s, focus, nodes); len(r) != 0 {
		t.Error("unique langs should pass")
	}

	// duplicate "en"
	nodes = []Term{langLit("hello", "en"), langLit("hi", "en")}
	if r := c.Evaluate(ctx, s, focus, nodes); len(r) == 0 {
		t.Error("duplicate lang should fail")
	}

	// disabled
	c2 := &UniqueLangConstraint{UniqueLang: false}
	if r := c2.Evaluate(ctx, s, focus, nodes); len(r) != 0 {
		t.Error("disabled uniqueLang should pass")
	}
}

// --- Property Pair Constraints ---

func TestEqualsConstraint(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	focus := IRI(ex + "n1")
	pred := IRI(ex + "name")
	g.Add(focus, pred, Literal("Alice", "", ""))

	ctx := testCtx(g)
	s := testShape()
	c := &EqualsConstraint{Path: pred}

	// equal sets
	if r := c.Evaluate(ctx, s, focus, []Term{Literal("Alice", "", "")}); len(r) != 0 {
		t.Error("equal sets should pass")
	}

	// value nodes have extra
	if r := c.Evaluate(ctx, s, focus, []Term{Literal("Alice", "", ""), Literal("Bob", "", "")}); len(r) == 0 {
		t.Error("extra value should fail")
	}

	// value nodes missing
	if r := c.Evaluate(ctx, s, focus, nil); len(r) == 0 {
		t.Error("missing value should fail")
	}
}

func TestDisjointConstraint(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	focus := IRI(ex + "n1")
	pred := IRI(ex + "name")
	g.Add(focus, pred, Literal("Alice", "", ""))

	ctx := testCtx(g)
	s := testShape()
	c := &DisjointConstraint{Path: pred}

	// disjoint
	if r := c.Evaluate(ctx, s, focus, []Term{Literal("Bob", "", "")}); len(r) != 0 {
		t.Error("disjoint sets should pass")
	}

	// overlap
	if r := c.Evaluate(ctx, s, focus, []Term{Literal("Alice", "", "")}); len(r) == 0 {
		t.Error("overlapping should fail")
	}
}

func TestLessThanConstraint(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	focus := IRI(ex + "n1")
	pred := IRI(ex + "maxAge")
	g.Add(focus, pred, intLit("30"))

	ctx := testCtx(g)
	s := testShape()
	c := &LessThanConstraint{Path: pred}

	// value < all objects
	if r := c.Evaluate(ctx, s, focus, []Term{intLit("20")}); len(r) != 0 {
		t.Error("20 < 30 should pass")
	}

	// value >= some object
	if r := c.Evaluate(ctx, s, focus, []Term{intLit("30")}); len(r) == 0 {
		t.Error("30 not < 30 should fail")
	}
	if r := c.Evaluate(ctx, s, focus, []Term{intLit("40")}); len(r) == 0 {
		t.Error("40 not < 30 should fail")
	}
}

func TestLessThanOrEqualsConstraint(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	focus := IRI(ex + "n1")
	pred := IRI(ex + "maxAge")
	g.Add(focus, pred, intLit("30"))

	ctx := testCtx(g)
	s := testShape()
	c := &LessThanOrEqualsConstraint{Path: pred}

	if r := c.Evaluate(ctx, s, focus, []Term{intLit("30")}); len(r) != 0 {
		t.Error("30 <= 30 should pass")
	}
	if r := c.Evaluate(ctx, s, focus, []Term{intLit("20")}); len(r) != 0 {
		t.Error("20 <= 30 should pass")
	}
	if r := c.Evaluate(ctx, s, focus, []Term{intLit("40")}); len(r) == 0 {
		t.Error("40 not <= 30 should fail")
	}
}

// --- Logical Constraints ---

func TestAndConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	inner1 := &Shape{ID: IRI(ex + "S1"), Severity: SHViolation, Constraints: []Constraint{
		&NodeKindConstraint{NodeKind: IRI(SH + "IRI")},
	}}
	inner2 := &Shape{ID: IRI(ex + "S2"), Severity: SHViolation, Constraints: []Constraint{
		&MinLengthConstraint{MinLength: 5},
	}}
	ctx.shapesMap[inner1.ID.String()] = inner1
	ctx.shapesMap[inner2.ID.String()] = inner2

	c := &AndConstraint{Shapes: []Term{inner1.ID, inner2.ID}}

	// IRI with length > 5: pass
	node := IRI(ex + "longname")
	if r := c.Evaluate(ctx, s, node, []Term{node}); len(r) != 0 {
		t.Error("should pass both shapes")
	}

	// literal: fails first shape
	litNode := Literal("longvalue", "", "")
	if r := c.Evaluate(ctx, s, litNode, []Term{litNode}); len(r) == 0 {
		t.Error("literal should fail IRI constraint")
	}
}

func TestOrConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()

	iriShape := &Shape{ID: IRI(ex + "IriShape"), Severity: SHViolation, Constraints: []Constraint{
		&NodeKindConstraint{NodeKind: IRI(SH + "IRI")},
	}}
	litShape := &Shape{ID: IRI(ex + "LitShape"), Severity: SHViolation, Constraints: []Constraint{
		&NodeKindConstraint{NodeKind: IRI(SH + "Literal")},
	}}
	ctx.shapesMap[iriShape.ID.String()] = iriShape
	ctx.shapesMap[litShape.ID.String()] = litShape

	c := &OrConstraint{Shapes: []Term{iriShape.ID, litShape.ID}}

	node := IRI(ex + "x")
	if r := c.Evaluate(ctx, s, node, []Term{node}); len(r) != 0 {
		t.Error("IRI should match one of the shapes")
	}

	bn := BlankNode("b1")
	if r := c.Evaluate(ctx, s, bn, []Term{bn}); len(r) == 0 {
		t.Error("blank node should match neither")
	}
}

func TestNotConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()

	inner := &Shape{ID: IRI(ex + "LitShape"), Severity: SHViolation, Constraints: []Constraint{
		&NodeKindConstraint{NodeKind: IRI(SH + "Literal")},
	}}
	ctx.shapesMap[inner.ID.String()] = inner

	c := &NotConstraint{ShapeRef: inner.ID}

	// IRI should pass (not a literal)
	node := IRI(ex + "x")
	if r := c.Evaluate(ctx, s, node, []Term{node}); len(r) != 0 {
		t.Error("IRI should pass not-literal")
	}

	// literal should fail
	litNode := Literal("hello", "", "")
	if r := c.Evaluate(ctx, s, litNode, []Term{litNode}); len(r) == 0 {
		t.Error("literal should fail not-literal")
	}
}

func TestXoneConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()

	iriShape := &Shape{ID: IRI(ex + "IriShape"), Severity: SHViolation, Constraints: []Constraint{
		&NodeKindConstraint{NodeKind: IRI(SH + "IRI")},
	}}
	iriOrLitShape := &Shape{ID: IRI(ex + "IriOrLitShape"), Severity: SHViolation, Constraints: []Constraint{
		&NodeKindConstraint{NodeKind: IRI(SH + "IRIOrLiteral")},
	}}
	litShape := &Shape{ID: IRI(ex + "LitShape"), Severity: SHViolation, Constraints: []Constraint{
		&NodeKindConstraint{NodeKind: IRI(SH + "Literal")},
	}}
	ctx.shapesMap[iriShape.ID.String()] = iriShape
	ctx.shapesMap[iriOrLitShape.ID.String()] = iriOrLitShape
	ctx.shapesMap[litShape.ID.String()] = litShape

	// exactly one: literal matches LitShape and IriOrLitShape = 2 matches -> fail
	c := &XoneConstraint{Shapes: []Term{iriShape.ID, iriOrLitShape.ID, litShape.ID}}
	litNode := Literal("hello", "", "")
	if r := c.Evaluate(ctx, s, litNode, []Term{litNode}); len(r) == 0 {
		t.Error("literal matches 2 shapes, xone should fail")
	}

	// blank node matches none -> fail
	bn := BlankNode("b1")
	if r := c.Evaluate(ctx, s, bn, []Term{bn}); len(r) == 0 {
		t.Error("blank matches 0 shapes, xone should fail")
	}

	// exactly one: use only IriShape and LitShape, IRI matches exactly one
	c2 := &XoneConstraint{Shapes: []Term{iriShape.ID, litShape.ID}}
	node := IRI(ex + "x")
	if r := c2.Evaluate(ctx, s, node, []Term{node}); len(r) != 0 {
		t.Error("IRI matches exactly 1 of 2 shapes, xone should pass")
	}
}

// --- NodeConstraint ---

func TestNodeConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()

	inner := &Shape{ID: IRI(ex + "IriShape"), Severity: SHViolation, Constraints: []Constraint{
		&NodeKindConstraint{NodeKind: IRI(SH + "IRI")},
	}}
	ctx.shapesMap[inner.ID.String()] = inner

	c := &NodeConstraint{ShapeRef: inner.ID}

	node := IRI(ex + "x")
	if r := c.Evaluate(ctx, s, node, []Term{node}); len(r) != 0 {
		t.Error("IRI should conform to IRI shape")
	}

	litNode := Literal("hello", "", "")
	if r := c.Evaluate(ctx, s, litNode, []Term{litNode}); len(r) == 0 {
		t.Error("literal should not conform to IRI shape")
	}
}

// --- HasValueConstraint ---

func TestHasValueConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")

	c := &HasValueConstraint{Value: Literal("target", "", "")}

	if r := c.Evaluate(ctx, s, focus, []Term{Literal("target", "", ""), Literal("other", "", "")}); len(r) != 0 {
		t.Error("value present should pass")
	}
	if r := c.Evaluate(ctx, s, focus, []Term{Literal("other", "", "")}); len(r) == 0 {
		t.Error("value absent should fail")
	}
	if r := c.Evaluate(ctx, s, focus, nil); len(r) == 0 {
		t.Error("empty should fail")
	}
}

// --- InConstraint ---

func TestInConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := testShape()
	focus := IRI(ex + "focus")

	c := &InConstraint{Values: []Term{Literal("a", "", ""), Literal("b", "", ""), Literal("c", "", "")}}

	if r := c.Evaluate(ctx, s, focus, []Term{Literal("a", "", ""), Literal("c", "", "")}); len(r) != 0 {
		t.Error("all in set should pass")
	}
	if r := c.Evaluate(ctx, s, focus, []Term{Literal("a", "", ""), Literal("d", "", "")}); len(r) == 0 {
		t.Error("d not in set should fail")
	}
	// empty value nodes should pass
	if r := c.Evaluate(ctx, s, focus, nil); len(r) != 0 {
		t.Error("no value nodes should pass")
	}
}

// --- ClosedConstraint ---

func TestClosedConstraint(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	focus := IRI(ex + "n1")
	namePred := IRI(ex + "name")
	agePred := IRI(ex + "age")
	secretPred := IRI(ex + "secret")

	g.Add(focus, namePred, Literal("Alice", "", ""))
	g.Add(focus, agePred, intLit("30"))
	g.Add(focus, IRI(RDFType), IRI(ex+"Person"))

	ctx := testCtx(g)
	s := testShape()

	// allowed: name, age; ignored: rdf:type
	c := &ClosedConstraint{
		AllowedProperties: []Term{namePred, agePred},
		IgnoredProperties: []Term{IRI(RDFType)},
	}

	if r := c.Evaluate(ctx, s, focus, []Term{focus}); len(r) != 0 {
		t.Errorf("all predicates allowed/ignored, should pass, got %d results", len(r))
	}

	// add disallowed predicate
	g.Add(focus, secretPred, Literal("hidden", "", ""))
	if r := c.Evaluate(ctx, s, focus, []Term{focus}); len(r) == 0 {
		t.Error("disallowed predicate should fail")
	}
}

// --- PropertyConstraint ---

func TestPropertyConstraint(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	focus := IRI(ex + "alice")
	g.Add(focus, IRI(ex+"name"), Literal("Alice", "", ""))

	ctx := testCtx(g)
	s := testShape()

	inner := &Shape{
		ID:         IRI(ex + "NameShape"),
		IsProperty: true,
		Path:       &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "name")},
		Severity:   SHViolation,
		Constraints: []Constraint{
			&NodeKindConstraint{NodeKind: IRI(SH + "IRI")},
		},
	}
	ctx.shapesMap[inner.ID.String()] = inner

	c := &PropertyConstraint{ShapeRef: inner.ID}

	// "Alice" is a literal, not IRI — should fail
	if r := c.Evaluate(ctx, s, focus, []Term{focus}); len(r) == 0 {
		t.Error("literal name should fail IRI nodeKind")
	}

	// Change to IRI value
	g2 := NewGraph()
	g2.Add(focus, IRI(ex+"name"), IRI(ex+"AliceName"))
	ctx2 := testCtx(g2)
	ctx2.shapesMap[inner.ID.String()] = inner

	if r := c.Evaluate(ctx2, s, focus, []Term{focus}); len(r) != 0 {
		t.Error("IRI name should pass")
	}
}

// --- QualifiedValueShapeConstraint ---

func TestQualifiedValueShapeConstraint(t *testing.T) {
	t.Parallel()
	ctx := testCtx(NewGraph())
	s := &Shape{
		ID:       IRI(ex + "PropShape"),
		Severity: SHViolation,
		Path:     &PropertyPath{Kind: PathPredicate, Pred: IRI(ex + "value")},
	}

	iriShape := &Shape{ID: IRI(ex + "IriShape"), Severity: SHViolation, Constraints: []Constraint{
		&NodeKindConstraint{NodeKind: IRI(SH + "IRI")},
	}}
	ctx.shapesMap[iriShape.ID.String()] = iriShape

	// qualifiedMinCount=2: need at least 2 IRIs
	c := &QualifiedValueShapeConstraint{
		ShapeRef:          iriShape.ID,
		QualifiedMinCount: 2,
		QualifiedMaxCount: -1,
	}

	nodes := []Term{IRI(ex + "a"), IRI(ex + "b"), Literal("c", "", "")}
	if r := c.Evaluate(ctx, s, IRI(ex+"focus"), nodes); len(r) != 0 {
		t.Error("2 IRIs >= minCount 2, should pass")
	}

	nodes = []Term{IRI(ex + "a"), Literal("b", "", "")}
	if r := c.Evaluate(ctx, s, IRI(ex+"focus"), nodes); len(r) == 0 {
		t.Error("1 IRI < minCount 2, should fail")
	}

	// qualifiedMaxCount=1: at most 1 IRI
	c2 := &QualifiedValueShapeConstraint{
		ShapeRef:          iriShape.ID,
		QualifiedMinCount: 0,
		QualifiedMaxCount: 1,
	}
	nodes = []Term{IRI(ex + "a"), IRI(ex + "b")}
	if r := c2.Evaluate(ctx, s, IRI(ex+"focus"), nodes); len(r) == 0 {
		t.Error("2 IRIs > maxCount 1, should fail")
	}
}

// --- NewPatternConstraint panic ---

func TestNewPatternConstraint_InvalidReturnsNil(t *testing.T) {
	t.Parallel()
	c := NewPatternConstraint("[invalid", "")
	if c != nil {
		t.Fatal("expected nil for invalid regex")
	}
}

// --- parseInt panic ---

func TestParseInt_InvalidReturnsZero(t *testing.T) {
	t.Parallel()
	v := parseInt(Literal("not-a-number", "", ""))
	if v != 0 {
		t.Fatalf("expected 0 for invalid integer, got %d", v)
	}
}

// --- LessThan IRI fallback ---

func TestLessThanConstraint_IRIFallback(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	focus := IRI(ex + "n1")
	pred := IRI(ex + "other")
	g.Add(focus, pred, IRI(ex+"bbb"))

	ctx := testCtx(g)
	s := testShape()
	c := &LessThanConstraint{Path: pred}

	// IRI "aaa" < "bbb" should pass
	if r := c.Evaluate(ctx, s, focus, []Term{IRI(ex + "aaa")}); len(r) != 0 {
		t.Error("aaa < bbb should pass")
	}

	// IRI "ccc" >= "bbb" should fail
	if r := c.Evaluate(ctx, s, focus, []Term{IRI(ex + "ccc")}); len(r) == 0 {
		t.Error("ccc not < bbb should fail")
	}

	// IRI == IRI should fail for strict less than
	if r := c.Evaluate(ctx, s, focus, []Term{IRI(ex + "bbb")}); len(r) == 0 {
		t.Error("bbb not < bbb should fail")
	}
}
