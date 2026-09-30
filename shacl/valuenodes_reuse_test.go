package shacl

import "testing"

// probeConstraint checks the contract validateShapeOnNode gives a node shape's
// constraints: valueNodes is exactly the focus node, has capacity 1, and keeps
// that value while a nested evaluation (which reuses the run's slot stack)
// runs inside Evaluate.
type probeConstraint struct {
	t      *testing.T
	nested *Shape // evaluated on node from inside Evaluate, if set
	node   Term
	calls  int
}

func (p *probeConstraint) ComponentIRI() string { return SH + "ProbeConstraintComponent" }

func (p *probeConstraint) Evaluate(ctx *evalContext, shape *Shape, focus Term, vn []Term) []ValidationResult {
	p.t.Helper()
	p.calls++
	check := func(when string) {
		if len(vn) != 1 || cap(vn) != 1 || vn[0] != focus {
			p.t.Errorf("%s: valueNodes = %v (cap %d), want exactly [%v]", when, vn, cap(vn), focus)
		}
	}
	check("on entry")
	if p.nested != nil {
		validateShapeOnNode(ctx, p.nested, p.node)
		check("after a nested evaluation")
	}
	// An append by a constraint must not write into a neighbouring slot.
	_ = append(vn, IRI("http://example.org/intruder"))
	check("after append")
	return nil
}

func TestNodeShapeValueNodesSurviveNestedEvaluation(t *testing.T) {
	t.Parallel()
	g := NewGraph()
	ex := func(s string) Term { return IRI("http://example.org/" + s) }

	leaf := &Shape{ID: ex("leaf"), Severity: SHViolation}
	leaf.Constraints = []Constraint{&probeConstraint{t: t}}
	mid := &Shape{ID: ex("mid"), Severity: SHViolation}
	mid.Constraints = []Constraint{
		&probeConstraint{t: t, nested: leaf, node: ex("c")},
		&probeConstraint{t: t}, // runs after the nested call on the same focus node
	}
	top := &Shape{ID: ex("top"), Severity: SHViolation}
	top.Constraints = []Constraint{&probeConstraint{t: t, nested: mid, node: ex("b")}}

	ctx := testCtx(g)
	for i := 0; i < 50; i++ {
		validateShapeOnNode(ctx, top, ex("a"))
	}
	if n := len(ctx.guard.values); n != 0 {
		t.Errorf("slot stack not released: %d left", n)
	}
	for _, s := range []*Shape{top, mid, leaf} {
		for _, c := range s.Constraints {
			if c.(*probeConstraint).calls == 0 {
				t.Errorf("probe on %v never ran", s.ID)
			}
		}
	}
}

func TestValueNodesStackGrowthKeepsOldSlices(t *testing.T) {
	t.Parallel()
	var g recursionGuard
	var held [][]Term
	for i := 0; i < 100; i++ {
		held = append(held, g.pushSingle(Literal(string(rune('a'+i%26)), "", "")))
	}
	for i, s := range held {
		if want := string(rune('a' + i%26)); s[0].Value() != want {
			t.Fatalf("slot %d changed to %q after the stack grew, want %q", i, s[0].Value(), want)
		}
	}
}
