package shacl

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
)

// nestedShapesPrefixes is prepended to generated shapes and data.
const nestedShapesPrefixes = "@prefix sh: <http://www.w3.org/ns/shacl#> .\n" +
	"@prefix ex: <http://example.org/> .\n" +
	"@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .\n"

// nestedShapesMaxDepth bounds how deep logical constraints nest. sh:or has
// three arms here, so the work per focus node grows as 3^depth.
const nestedShapesMaxDepth = 5

// genLeaf returns one non-recursive constraint, occasionally with a
// per-constraint severity annotation (RDF 1.2), which wraps it in
// severityOverrideConstraint.
func genLeaf(r *rand.Rand) string {
	var c string
	switch r.IntN(7) {
	case 0:
		c = fmt.Sprintf("sh:class ex:C%d", r.IntN(3))
	case 1:
		c = "sh:nodeKind sh:IRI"
	case 2:
		return fmt.Sprintf("sh:property [ sh:path ex:p%d ; sh:minCount %d ]", r.IntN(3), r.IntN(2))
	case 3:
		return fmt.Sprintf("sh:property [ sh:path ex:p%d ; sh:datatype xsd:string ]", r.IntN(3))
	case 4:
		return fmt.Sprintf("sh:in ( ex:n%d ex:n%d )", r.IntN(6), r.IntN(6))
	case 5:
		c = fmt.Sprintf("sh:hasValue ex:n%d", r.IntN(6))
	default:
		return fmt.Sprintf("sh:property [ sh:path ex:p%d ; sh:qualifiedValueShape [ sh:class ex:C%d ] ; sh:qualifiedMinCount 1 ]",
			r.IntN(3), r.IntN(3))
	}
	if r.IntN(3) == 0 {
		c += " {| sh:severity sh:Warning |}"
	}
	return c
}

// genBody returns a shape body nesting logical constraints and references to
// other named shapes (which may form cycles) up to depth levels.
func genBody(r *rand.Rand, depth, nShapes int) string {
	if depth == 0 {
		return genLeaf(r)
	}
	ref := func() string { return fmt.Sprintf("ex:S%d", r.IntN(nShapes)) }
	inner := func() string { return "[ " + genBody(r, depth-1, nShapes) + " ]" }
	switch r.IntN(7) {
	case 0:
		return "sh:or ( " + inner() + " " + inner() + " " + ref() + " )"
	case 1:
		return "sh:and ( " + inner() + " " + ref() + " )"
	case 2:
		return "sh:not " + inner()
	case 3:
		return "sh:xone ( " + inner() + " " + inner() + " )"
	case 4:
		return "sh:node " + ref()
	case 5:
		return genLeaf(r) + " ; " + genBody(r, depth-1, nShapes)
	default:
		return fmt.Sprintf("sh:property [ sh:path ex:p%d ; sh:node %s ]", r.IntN(3), ref())
	}
}

// genNestedShapes builds a shapes graph and a data graph from seed.
func genNestedShapes(seed uint64) (shapesTTL, dataTTL string) {
	r := rand.New(rand.NewPCG(seed, 0x5eed))
	nShapes := 2 + r.IntN(8)
	var sh strings.Builder
	sh.WriteString(nestedShapesPrefixes)
	for i := range nShapes {
		fmt.Fprintf(&sh, "ex:S%d a sh:NodeShape ; %s .\n", i, genBody(r, r.IntN(nestedShapesMaxDepth+1), nShapes))
	}
	sh.WriteString("ex:S0 sh:targetClass ex:C0 .\nex:S1 sh:targetSubjectsOf ex:p0 .\n")

	var d strings.Builder
	d.WriteString(nestedShapesPrefixes)
	for n := range 6 {
		fmt.Fprintf(&d, "ex:n%d a ex:C%d ; ex:p%d ex:n%d ; ex:p%d \"v%d\" .\n",
			n, r.IntN(3), r.IntN(3), r.IntN(6), r.IntN(3), n)
	}
	return sh.String(), d.String()
}

// FuzzNestedShapes validates randomly nested shapes and checks what must hold
// whatever the shapes are:
//
//   - CompiledShapes from several goroutines gives exactly Validate's report
//     (checkCompiledShapesAgree), so nothing a run keeps on its recursion
//     guard (quick mode, the value-node slot stack) leaks between runs;
//   - a second Validate gives the same report;
//   - for every shape and every node, the quick-mode verdict (nodeConforms)
//     equals the full one, and the quick-mode placeholder stays zero.
//
// The seed corpus runs on every go test; `make test-fuzz` searches further.
func FuzzNestedShapes(f *testing.F) {
	for seed := range uint64(40) {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, seed uint64) {
		shapesTTL, dataTTL := genNestedShapes(seed)
		shapes, err := LoadTurtleString(shapesTTL, "")
		if err != nil {
			t.Fatalf("generated shapes do not parse: %v\n%s", err, shapesTTL)
		}
		data, err := LoadTurtleString(dataTTL, "")
		if err != nil {
			t.Fatal(err)
		}

		want := Validate(data, shapes)
		if again := Validate(data, shapes); !sameReport(want, again) {
			t.Fatalf("seed %d: two Validate runs differ", seed)
		}
		checkCompiledShapesAgree(t, data, shapes, want)

		p := Prepare(data, shapes)
		ctx := p.evaluation(nil)
		nodes := allNodes(data)
		for _, s := range shapesInOrder(ctx.shapesMap) {
			for _, n := range nodes {
				full := len(validateShapeOnNode(ctx, s, n)) == 0
				if quick := nodeConforms(ctx, s, n); quick != full {
					t.Fatalf("seed %d: shape %s on %s: quick verdict %v, full %v\n%s",
						seed, s.ID, n, quick, full, shapesTTL)
				}
				if ctx.guard.quick {
					t.Fatalf("seed %d: quick flag left on", seed)
				}
				if len(ctx.guard.values) != 0 {
					t.Fatalf("seed %d: %d value-node slots not released", seed, len(ctx.guard.values))
				}
			}
		}
		if quickFailure[0].FocusNode != (Term{}) || quickFailure[0].ResultSeverity != (Term{}) {
			t.Fatalf("seed %d: quickFailure was written to: %+v", seed, quickFailure[0])
		}
	})
}

// sameReport compares two reports with CompareReports, which is what the W3C
// runners trust.
func sameReport(a, b ValidationReport) bool {
	ok, _ := CompareReports(a, b)
	return ok
}
