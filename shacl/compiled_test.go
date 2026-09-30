package shacl

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// checkCompiledShapesAgree validates dataGraph through one CompiledShapes from
// several goroutines at once and requires every report to equal want, the
// report of the package-level Validate. The W3C runners call it on every
// test, so the whole suite checks that compiled shapes are shareable; run
// with -race for the concurrency half of that claim.
func checkCompiledShapesAgree(t *testing.T, dataGraph, shapesGraph *Graph, want ValidationReport) {
	t.Helper()
	compiled, err := CompileShapes(shapesGraph)
	if err != nil {
		t.Fatalf("CompileShapes: %v", err)
	}
	const workers = 8
	reports := make([]ValidationReport, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			reports[i] = compiled.Validate(dataGraph)
		}()
	}
	wg.Wait()
	for i, got := range reports {
		if match, details := CompareReports(want, got); !match {
			t.Errorf("CompiledShapes report %d differs from Validate:\n%s", i, details)
			return
		}
	}
}

func TestCompileShapesNil(t *testing.T) {
	if _, err := CompileShapes(nil); !errors.Is(err, ErrNilShapesGraph) {
		t.Fatalf("err = %v, want ErrNilShapesGraph", err)
	}
}

const compiledShapesTTL = `
@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [ sh:path ex:name ; sh:minCount 1 ] .
`

func TestCompiledShapesManyDocuments(t *testing.T) {
	shapes, err := LoadTurtleString(compiledShapesTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := CompileShapes(shapes)
	if err != nil {
		t.Fatal(err)
	}
	good, _ := LoadTurtleString(`@prefix ex: <http://example.org/> . ex:a a ex:Person ; ex:name "A" .`, "")
	bad, _ := LoadTurtleString(`@prefix ex: <http://example.org/> . ex:b a ex:Person .`, "")
	for range 3 {
		if r := compiled.Validate(good); !r.Conforms {
			t.Fatalf("good document does not conform: %+v", r.Results)
		}
		if r := compiled.Validate(bad); r.Conforms || len(r.Results) != 1 {
			t.Fatalf("bad document: conforms=%v results=%d, want one violation", r.Conforms, len(r.Results))
		}
	}
}

// An ad-hoc shape found during one run (an anonymous sh:filterShape) is added
// to the shape map. It must land in that run's copy, not the compiled map, or
// concurrent runs write the same map.
func TestCompiledShapesDoNotGrow(t *testing.T) {
	shapes, err := LoadTurtleString(`
@prefix ex: <http://example.org/> .
@prefix sh: <http://www.w3.org/ns/shacl#> .
ex:Root a sh:NodeShape ; sh:targetNode ex:product ;
    sh:expression [ sh:filterShape [ sh:nodeKind sh:IRI ] ; sh:nodes sh:this ] .
`, "")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := CompileShapes(shapes, WithAdvancedFeatures())
	if err != nil {
		t.Fatal(err)
	}
	before := len(compiled.shapes)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			compiled.Validate(NewGraph())
		}()
	}
	wg.Wait()
	if after := len(compiled.shapes); after != before {
		t.Fatalf("compiled shape map grew from %d to %d", before, after)
	}
}

func TestCompiledShapesPerCallOptions(t *testing.T) {
	shapes, err := LoadTurtleString(compiledShapesTTL, "")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := CompileShapes(shapes)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := LoadTurtleString(`@prefix ex: <http://example.org/> . ex:b a ex:Person .`, "")

	// Switching advanced features on for one call reparses rather than using
	// shapes compiled without their targets.
	if r := compiled.Validate(data, WithAdvancedFeatures()); r.Conforms {
		t.Fatal("advanced-features call lost the violation")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := compiled.ValidateContext(ctx, data); !errors.Is(err, ErrCancelled) {
		t.Fatalf("cancelled run: err = %v, want ErrCancelled", err)
	}
}

// BenchmarkCompiledShapes is the issue #39 workload: one shapes graph, many
// small documents. "validate" parses the shapes on every call, "compiled"
// parses them once.
func BenchmarkCompiledShapes(b *testing.B) {
	var ttl strings.Builder
	ttl.WriteString("@prefix sh: <http://www.w3.org/ns/shacl#> .\n@prefix ex: <http://example.org/> .\n@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .\n")
	for i := range 30 {
		fmt.Fprintf(&ttl, `ex:Shape%d a sh:NodeShape ; sh:targetClass ex:Type%d ;
    sh:property [ sh:path ex:name ; sh:minCount 1 ; sh:datatype xsd:string ] ;
    sh:property [ sh:path ex:url ; sh:nodeKind sh:IRI ; sh:maxCount 3 ] ;
    sh:property [ sh:path ex:geo ; sh:node ex:Shape%d ] .
`, i, i, (i+1)%30)
	}
	shapes, err := LoadTurtleString(ttl.String(), "")
	if err != nil {
		b.Fatal(err)
	}
	data, err := LoadTurtleString(`@prefix ex: <http://example.org/> .
ex:doc a ex:Type0 ; ex:name "a place" ; ex:url ex:page .`, "")
	if err != nil {
		b.Fatal(err)
	}
	b.Run("validate", func(b *testing.B) {
		for b.Loop() {
			benchReport = Validate(data, shapes)
		}
	})
	b.Run("compiled", func(b *testing.B) {
		compiled, err := CompileShapes(shapes)
		if err != nil {
			b.Fatal(err)
		}
		for b.Loop() {
			benchReport = compiled.Validate(data)
		}
	})
}
