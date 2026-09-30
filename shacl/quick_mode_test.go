package shacl

import (
	"context"
	"reflect"
	"sync"
	"testing"
)

// quickShapesTTL exercises every nesting the quick mode has to get right:
// sh:class inside sh:or, sh:or inside sh:not, sh:not inside sh:and, sh:node
// inside sh:or, and a shape that reaches itself through sh:or (the recursion
// guard).
const quickShapesTTL = `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:OrClass a sh:NodeShape ; sh:targetClass ex:T ;
    sh:or ( [ sh:class ex:A ] [ sh:class ex:B ] ) .

ex:NotOr a sh:NodeShape ; sh:targetClass ex:T ;
    sh:not [ sh:or ( [ sh:class ex:A ] [ sh:class ex:B ] ) ] .

ex:AndNot a sh:NodeShape ; sh:targetClass ex:T ;
    sh:and ( [ sh:not [ sh:class ex:A ] ] [ sh:class ex:B ] ) .

ex:NodeInOr a sh:NodeShape ; sh:targetClass ex:T ;
    sh:or ( [ sh:node ex:Inner ] [ sh:class ex:B ] ) .

ex:Inner a sh:NodeShape ;
    sh:property [ sh:path ex:p ; sh:minCount 1 ; sh:class ex:A ] ;
    sh:property [ sh:path ex:q ; sh:datatype xsd:integer ] .

ex:Rec a sh:NodeShape ; sh:targetClass ex:R ;
    sh:or ( [ sh:property [ sh:path ex:next ; sh:minCount 1 ; sh:node ex:Rec ] ] [ sh:class ex:B ] ) .

ex:NotProp a sh:NodeShape ; sh:targetClass ex:T ;
    sh:not [ sh:property [ sh:path ex:p ; sh:class ex:A ] ] .

ex:Xone a sh:NodeShape ; sh:targetClass ex:T ;
    sh:xone ( [ sh:class ex:A ] [ sh:class ex:B ] ) .

ex:Direct a sh:NodeShape ; sh:targetClass ex:T ;
    sh:node ex:Inner .
`

const quickDataTTL = `
@prefix ex: <http://example.org/> .
ex:a a ex:T, ex:A .
ex:b a ex:T, ex:B .
ex:ab a ex:T, ex:A, ex:B .
ex:none a ex:T .
ex:good a ex:T ; ex:p ex:a ; ex:q 1 .
ex:badp a ex:T ; ex:p ex:none ; ex:q 1 .
ex:badq a ex:T ; ex:p ex:a ; ex:q "one" .
ex:multi a ex:T ; ex:p ex:none ; ex:q "x" .
ex:r1 a ex:R ; ex:next ex:r2 .
ex:r2 a ex:R ; ex:next ex:r1 .
ex:r3 a ex:R, ex:B ; ex:next ex:r4 .
ex:r4 a ex:R ; ex:next ex:r3 .
ex:r5 a ex:R ; ex:next ex:r5 .
ex:r6 a ex:R ; ex:next ex:none .
ex:r7 a ex:R .
`

// TestQuickModeVerdictEqualsFullMode: for every shape, nested or not, and every
// node of the data, nodeConforms says what len(results) == 0 says of a full
// run, and it leaves quick mode switched off.
func TestQuickModeVerdictEqualsFullMode(t *testing.T) {
	shapes, err := LoadTurtleString(quickShapesTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := LoadTurtleString(quickDataTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	prepared := Prepare(data, shapes)
	ctx := prepared.evaluation(context.Background())
	nodes := allNodes(data)
	checked, conforming, violating := 0, 0, 0
	for id, s := range ctx.shapesMap {
		for _, node := range nodes {
			full := len(validateNodeAgainstShape(ctx, s, node)) == 0
			quick := nodeConforms(ctx, s, node)
			if quick != full {
				t.Errorf("shape %s on %s: quick = %v, full = %v", id, node, quick, full)
			}
			if ctx.guard.quick {
				t.Fatalf("shape %s on %s: quick mode left switched on", id, node)
			}
			checked++
			if full {
				conforming++
			} else {
				violating++
			}
		}
	}
	// The table is only a test if it has both verdicts in quantity.
	if conforming < 20 || violating < 20 {
		t.Fatalf("degenerate table: %d checks, %d conforming, %d violating", checked, conforming, violating)
	}
}

// TestQuickModeNeverReachesAReport: the reports of a run whose sh:or, sh:and and
// sh:not all evaluate in quick mode carry only real results, and the nested
// Details of sh:node are complete.
func TestQuickModeNeverReachesAReport(t *testing.T) {
	shapes, err := LoadTurtleString(quickShapesTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := LoadTurtleString(quickDataTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	report := Validate(data, shapes)
	if report.Conforms {
		t.Fatal("data violates the shapes, report conforms")
	}
	var check func(results []ValidationResult)
	check = func(results []ValidationResult) {
		for _, r := range results {
			if r.SourceConstraintComponent.IsNone() || r.FocusNode.IsNone() || r.SourceShape.IsNone() {
				t.Errorf("result with no component, focus node or shape (a quickFailure?): %+v", r)
			}
			check(r.Details)
		}
	}
	check(report.Results)

	// ex:Direct is sh:node ex:Inner: its violations are inner sh:class / minCount
	// / datatype results, and they must be there in full.
	details := map[string][]string{}
	for _, r := range report.Results {
		if r.SourceShape.Value() == "http://example.org/Direct" {
			for _, d := range r.Details {
				details[r.FocusNode.Value()] = append(details[r.FocusNode.Value()], d.SourceConstraintComponent.Value())
			}
		}
	}
	if got := details["http://example.org/badp"]; len(got) != 1 || got[0] != SH+"ClassConstraintComponent" {
		t.Errorf("Details of sh:node for badp = %v, want one sh:class result", got)
	}
	if got := details["http://example.org/badq"]; len(got) != 1 || got[0] != SH+"DatatypeConstraintComponent" {
		t.Errorf("Details of sh:node for badq = %v, want one sh:datatype result", got)
	}
	// Two violations in one node shape are both reported: stopping at the first
	// is for quick mode only.
	if got := details["http://example.org/multi"]; len(got) != 2 {
		t.Errorf("Details of sh:node for multi = %v, want the sh:class and the sh:datatype result", got)
	}
	if got := details["http://example.org/none"]; len(got) != 1 || got[0] != SH+"MinCountConstraintComponent" {
		t.Errorf("Details of sh:node for none = %v, want one sh:minCount result", got)
	}
}

// TestQuickModeVerdictsOfLogicalConstraints pins the conformance of the
// logical shapes by hand, so the table above cannot agree with itself on a
// shared mistake.
func TestQuickModeVerdictsOfLogicalConstraints(t *testing.T) {
	shapes, err := LoadTurtleString(quickShapesTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := LoadTurtleString(quickDataTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	ctx := Prepare(data, shapes).evaluation(context.Background())
	ex := func(n string) Term { return IRI("http://example.org/" + n) }
	want := []struct {
		shape, node string
		conforms    bool
	}{
		{"OrClass", "a", true}, {"OrClass", "b", true}, {"OrClass", "none", false},
		{"NotOr", "a", false}, {"NotOr", "none", true},
		{"AndNot", "b", true}, {"AndNot", "ab", false}, {"AndNot", "none", false},
		{"NodeInOr", "good", true}, {"NodeInOr", "b", true}, {"NodeInOr", "badp", false},
		{"NodeInOr", "badq", false},
		{"Rec", "r1", true}, {"Rec", "r3", true}, {"Rec", "r5", true}, {"Rec", "r6", false}, {"Rec", "r7", false},
		{"NotProp", "good", false}, {"NotProp", "badp", true},
		{"Xone", "a", true}, {"Xone", "ab", false},
	}
	for _, w := range want {
		s := ctx.shapesMap[ex(w.shape).String()]
		if s == nil {
			t.Fatalf("no shape %s", w.shape)
		}
		if got := nodeConforms(ctx, s, ex(w.node)); got != w.conforms {
			t.Errorf("nodeConforms(%s, %s) = %v, want %v", w.shape, w.node, got, w.conforms)
		}
	}
}

// A per-constraint sh:severity annotation wraps the constraint in
// severityOverrideConstraint, which rewrites the severity of the results it
// gets back. Under sh:or those results can be the shared quickFailure
// placeholder; writing to it is a data race between concurrent validations
// and leaves the placeholder non-zero. Run with -race.
func TestQuickModeSeverityOverrideLeavesPlaceholderAlone(t *testing.T) {
	shapes, err := LoadTurtleString(`
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
ex:IsC a sh:NodeShape ; sh:class ex:C {| sh:severity sh:Warning |} .
ex:IsD a sh:NodeShape ; sh:class ex:D .
ex:Root a sh:NodeShape ; sh:targetNode ex:n ; sh:or ( ex:IsC ex:IsD ) .
`, "")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := LoadTurtleString(`@prefix ex: <http://example.org/> . ex:n a ex:E .`, "")
	compiled, err := CompileShapes(shapes)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if r := compiled.Validate(data); r.Conforms || len(r.Results) != 1 {
				t.Errorf("conforms=%v results=%d, want one sh:or violation", r.Conforms, len(r.Results))
			}
		}()
	}
	wg.Wait()
	if !reflect.DeepEqual(quickFailure[0], ValidationResult{}) {
		t.Fatalf("quickFailure was written to: %+v", quickFailure[0])
	}
}
