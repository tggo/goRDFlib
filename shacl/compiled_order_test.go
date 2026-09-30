package shacl

import (
	"slices"
	"sort"
	"testing"
)

// orderShapesTTL names shapes out of alphabetical order, each with a SPARQL
// target that cannot be run. What the error handler hears, and in what order,
// shows the order the shapes were visited in.
const orderShapesTTL = `
ex:SQ a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ; sh:select """SELECT ?this WHERE { broken Q }""" ] .
ex:SC a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ; sh:select """SELECT ?this WHERE { broken C }""" ] .
ex:SX a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ; sh:select """SELECT ?this WHERE { broken X }""" ] .
ex:SB a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ; sh:select """SELECT ?this WHERE { broken B }""" ] .
ex:SM a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ; sh:select """SELECT ?this WHERE { broken M }""" ] .
ex:SZ a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ; sh:select """SELECT ?this WHERE { broken Z }""" ] .
ex:SA a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ; sh:select """SELECT ?this WHERE { broken A }""" ] .
ex:SK a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ; sh:select """SELECT ?this WHERE { broken K }""" ] .
ex:ST a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ; sh:select """SELECT ?this WHERE { broken T }""" ] .
ex:SF a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ; sh:select """SELECT ?this WHERE { broken F }""" ] .
ex:SR a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ; sh:select """SELECT ?this WHERE { broken R }""" ] .
ex:SD a sh:NodeShape ;
    sh:target [ a sh:SPARQLTarget ; sh:select """SELECT ?this WHERE { broken D }""" ] .
`

func reportedTargets(t *testing.T, run func(opts ...Option) ValidationReport) []string {
	t.Helper()
	var got []string
	run(WithAdvancedFeatures(), WithErrorHandler(func(err error) { got = append(got, err.Error()) }))
	return got
}

// TestCompiledShapesVisitInKeyOrder: shapes are visited sorted by key, on every
// run, whether the order is sorted per run (Validate) or once (CompiledShapes).
func TestCompiledShapesVisitInKeyOrder(t *testing.T) {
	shapes := afGraph(t, orderShapesTTL)
	data := afGraph(t, "ex:x a ex:Thing .")

	want := reportedTargets(t, func(opts ...Option) ValidationReport { return Validate(data, shapes, opts...) })
	if len(want) != 12 {
		t.Fatalf("Validate reported %d broken targets, want 12", len(want))
	}
	if !sort.StringsAreSorted(want) {
		t.Fatalf("Validate does not visit shapes in key order:\n%v", want)
	}

	compiled, err := CompileShapes(shapes, WithAdvancedFeatures())
	if err != nil {
		t.Fatal(err)
	}
	for i := range 5 {
		got := reportedTargets(t, func(opts ...Option) ValidationReport { return compiled.Validate(data, opts...) })
		if !slices.Equal(got, want) {
			t.Fatalf("run %d: compiled order\n%v\nwant\n%v", i, got, want)
		}
	}

	// Compiled without advanced features and switched on per call: the compiled
	// shapes are not used, the run parses its own, and the order is still the same.
	plain, err := CompileShapes(shapes)
	if err != nil {
		t.Fatal(err)
	}
	got := reportedTargets(t, func(opts ...Option) ValidationReport { return plain.Validate(data, opts...) })
	if !slices.Equal(got, want) {
		t.Fatalf("per-call advanced features: order\n%v\nwant\n%v", got, want)
	}
}

// TestCompiledShapesOrderedIsSorted: the order kept at compile time is the one
// shapesInOrder computes.
func TestCompiledShapesOrderedIsSorted(t *testing.T) {
	compiled, err := CompileShapes(afGraph(t, orderShapesTTL))
	if err != nil {
		t.Fatal(err)
	}
	want := shapesInOrder(compiled.shapes)
	if len(want) == 0 || !slices.Equal(compiled.ordered, want) {
		t.Fatalf("ordered = %v, want %v", compiled.ordered, want)
	}
}
