package shacl

import (
	"context"
	"errors"
	"strings"
	"testing"
)

const illFormedShapesTTL = `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
ex:Person a sh:NodeShape ;
	sh:targetClass ex:Person ;
	sh:property [ sh:minCount 1 ] .
ex:Typed a sh:PropertyShape ; sh:datatype ex:T .
ex:Two a sh:PropertyShape ; sh:path ex:a, ex:b .
ex:Lit a sh:PropertyShape ; sh:path "ex:name" .
ex:Fine a sh:PropertyShape ; sh:path ex:name ; sh:minCount 1 .
`

const illFormedDataTTL = `
@prefix ex: <http://example.org/> .
ex:alice a ex:Person .
`

func loadIllFormed(t *testing.T) (data, shapes *Graph) {
	t.Helper()
	shapes, err := LoadTurtleString(illFormedShapesTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	data, err = LoadTurtleString(illFormedDataTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	return data, shapes
}

// Issue #46: a property shape without sh:path used to load silently.
func TestIllFormedShapesReported(t *testing.T) {
	data, shapes := loadIllFormed(t)
	var got []string
	Validate(data, shapes, WithErrorHandler(func(err error) {
		if !errors.Is(err, ErrIllFormedShape) {
			t.Errorf("unexpected error: %v", err)
		}
		got = append(got, err.Error())
	}))
	want := []string{
		"(the sh:property of <http://example.org/Person>) has no sh:path",
		"<http://example.org/Lit> has the literal",
		"<http://example.org/Two> has 2 values for sh:path",
		"<http://example.org/Typed> has no sh:path",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d reports, want %d:\n%s", len(got), len(want), strings.Join(got, "\n"))
	}
	for _, w := range want {
		if !strings.Contains(strings.Join(got, "\n"), w) {
			t.Errorf("no report containing %q in:\n%s", w, strings.Join(got, "\n"))
		}
	}
}

func TestIllFormedShapesStrict(t *testing.T) {
	data, shapes := loadIllFormed(t)
	ctx := context.Background()

	if _, err := CompileShapes(shapes, WithStrictShapes()); !errors.Is(err, ErrIllFormedShape) {
		t.Errorf("CompileShapes: err = %v, want ErrIllFormedShape", err)
	}
	if _, err := ValidateContext(ctx, data, shapes, WithStrictShapes()); !errors.Is(err, ErrIllFormedShape) {
		t.Errorf("ValidateContext: err = %v, want ErrIllFormedShape", err)
	}
	if _, err := PrepareContext(ctx, data, shapes, WithStrictShapes()); !errors.Is(err, ErrIllFormedShape) {
		t.Errorf("PrepareContext: err = %v, want ErrIllFormedShape", err)
	}

	compiled, err := CompileShapes(shapes)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := compiled.ValidateContext(ctx, data, WithStrictShapes()); !errors.Is(err, ErrIllFormedShape) {
		t.Errorf("CompiledShapes.ValidateContext: err = %v, want ErrIllFormedShape", err)
	}

	// No error result: strict degrades to reporting, and validation still runs.
	for name, validate := range map[string]func(...Option) ValidationReport{
		"Validate":                func(o ...Option) ValidationReport { return Validate(data, shapes, o...) },
		"Prepare":                 func(o ...Option) ValidationReport { return Prepare(data, shapes, o...).Validate() },
		"CompiledShapes.Validate": func(o ...Option) ValidationReport { return compiled.Validate(data, o...) },
	} {
		n := 0
		r := validate(WithStrictShapes(), WithErrorHandler(func(error) { n++ }))
		if n != 4 || len(r.Results) == 0 {
			t.Errorf("%s: %d reports, %d results; want 4 reports and a full report", name, n, len(r.Results))
		}
	}
}

// Every run reports, so a handler given per call to CompiledShapes sees it.
func TestIllFormedShapesCompiledReportsPerRun(t *testing.T) {
	data, shapes := loadIllFormed(t)
	compiled, err := CompileShapes(shapes)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		n := 0
		compiled.Validate(data, WithErrorHandler(func(error) { n++ }))
		if n != 4 {
			t.Errorf("got %d reports, want 4", n)
		}
	}
}

func TestWellFormedShapesNotReported(t *testing.T) {
	shapes, err := LoadTurtleString(`
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
ex:S a sh:NodeShape ; sh:targetClass ex:C ;
	sh:property [ sh:path [ sh:inversePath ex:p ] ; sh:minCount 1 ] ;
	sh:property [ sh:path ( ex:a ex:b ) ] ;
	sh:node [ sh:class ex:D ] .
`, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CompileShapes(shapes, WithStrictShapes()); err != nil {
		t.Errorf("CompileShapes: %v", err)
	}
}
