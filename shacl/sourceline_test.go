package shacl_test

import (
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/provenance"
	"github.com/tggo/goRDFlib/shacl"
	"github.com/tggo/goRDFlib/turtle"
)

// The data is laid out so that every interesting line number is distinct and
// obvious from reading it. Line 1 is the first @prefix.
//
//	 1  @prefix ex: ...
//	 2  @prefix xsd: ...
//	 3  (blank)
//	 4  ex:alice a ex:Person ;
//	 5      ex:name "Alice" ;
//	 6      ex:age 34 .
//	 7  (blank)
//	 8  ex:bob a ex:Person ;
//	 9      ex:name "Bob" ;
//	10      ex:age "not a number" .
//	11  (blank)
//	12  ex:carol a ex:Person ;
//	13      ex:age 41 .
const sourceLineData = `@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:alice a ex:Person ;
    ex:name "Alice" ;
    ex:age 34 .

ex:bob a ex:Person ;
    ex:name "Bob" ;
    ex:age "not a number" .

ex:carol a ex:Person ;
    ex:age 41 .
`

const sourceLineShapes = `@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
        sh:datatype xsd:string ;
    ] ;
    sh:property [
        sh:path ex:age ;
        sh:datatype xsd:integer ;
    ] .
`

func loadWithProvenance(t *testing.T, ttl string) (*shacl.Graph, *provenance.Index) {
	t.Helper()

	g := graph.NewGraph()
	idx := provenance.NewIndex()
	if err := turtle.Parse(g, strings.NewReader(ttl), turtle.WithProvenance(idx.Triple)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if idx.Len() == 0 {
		t.Fatal("provenance index is empty; the option never reached the parser")
	}
	return shacl.NewGraphFromRDF(g, ""), idx
}

func loadShapes(t *testing.T, ttl string) *shacl.Graph {
	t.Helper()
	g := graph.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(ttl)); err != nil {
		t.Fatalf("parse shapes: %v", err)
	}
	return shacl.NewGraphFromRDF(g, "")
}

// TestValidationResultsCarrySourceLines is the end-to-end case: parse with
// provenance, validate with the index, and get back results that name a line in
// the file the reader can go and look at.
func TestValidationResultsCarrySourceLines(t *testing.T) {
	data, idx := loadWithProvenance(t, sourceLineData)
	shapes := loadShapes(t, sourceLineShapes)

	report := shacl.Validate(data, shapes, shacl.WithSourceLines(idx))
	if report.Conforms {
		t.Fatal("expected the data to violate the shapes")
	}
	if len(report.Results) != 2 {
		for _, r := range report.Results {
			t.Logf("result: focus=%s path=%s value=%s line=%d (%s)",
				r.FocusNode, r.ResultPath, r.Value, r.SourceLine, r.SourceLineKind)
		}
		t.Fatalf("got %d results, want 2", len(report.Results))
	}

	byFocus := map[string]shacl.ValidationResult{}
	for _, r := range report.Results {
		byFocus[r.FocusNode.Value()] = r
	}

	// Bob's age is a string where the shape wants an integer. The offending
	// triple is written on line 10, and that is the line to report: the value
	// that is wrong is on it.
	bob, ok := byFocus["http://example.org/bob"]
	if !ok {
		t.Fatal("no result for ex:bob")
	}
	if bob.SourceLine != 10 {
		t.Errorf("ex:bob datatype violation reported line %d, want 10", bob.SourceLine)
	}
	if bob.SourceLineKind != shacl.SourceLineTriple {
		t.Errorf("ex:bob reported %s, want a triple line — the bad value is written down",
			bob.SourceLineKind)
	}

	// Carol has no ex:name at all. There is no triple to point at, because the
	// problem is a triple that was never written, so the answer is where carol
	// is defined: line 12.
	carol, ok := byFocus["http://example.org/carol"]
	if !ok {
		t.Fatal("no result for ex:carol")
	}
	if carol.SourceLine != 12 {
		t.Errorf("ex:carol minCount violation reported line %d, want 12", carol.SourceLine)
	}
	if carol.SourceLineKind != shacl.SourceLineFocusNode {
		t.Errorf("ex:carol reported %s, want the focus node's line — nothing is written to blame",
			carol.SourceLineKind)
	}
}

// TestSourceLinesAreOptIn checks that nothing changes when the option is not
// passed. The field stays zero, and every other part of the report is identical.
func TestSourceLinesAreOptIn(t *testing.T) {
	data, idx := loadWithProvenance(t, sourceLineData)
	shapes := loadShapes(t, sourceLineShapes)

	plain := shacl.Validate(data, shapes)
	annotated := shacl.Validate(data, shapes, shacl.WithSourceLines(idx))

	if plain.Conforms != annotated.Conforms || len(plain.Results) != len(annotated.Results) {
		t.Fatalf("the option changed the outcome: %v/%d vs %v/%d",
			plain.Conforms, len(plain.Results), annotated.Conforms, len(annotated.Results))
	}
	for _, r := range plain.Results {
		if r.SourceLine != 0 || r.SourceLineKind != shacl.SourceLineNone {
			t.Errorf("result carries line %d without WithSourceLines", r.SourceLine)
		}
	}
	for _, r := range annotated.Results {
		if r.SourceLine == 0 {
			t.Errorf("result for %s has no line: focus=%s path=%s value=%s",
				r.SourceConstraintComponent, r.FocusNode, r.ResultPath, r.Value)
		}
	}
}

// TestSourceLinesWithNilIndex covers passing the option with nothing in it,
// which a caller does when provenance is conditional on a flag.
func TestSourceLinesWithNilIndex(t *testing.T) {
	data, _ := loadWithProvenance(t, sourceLineData)
	shapes := loadShapes(t, sourceLineShapes)

	report := shacl.Validate(data, shapes, shacl.WithSourceLines(nil))
	if report.Conforms {
		t.Fatal("expected violations")
	}
	for _, r := range report.Results {
		if r.SourceLine != 0 {
			t.Errorf("a nil index produced line %d", r.SourceLine)
		}
	}
}

// TestSourceLinesForInferredTriples pins what happens to a triple that was
// never in the file. SHACL-AF rules and the reasoner both mint triples, and
// those have no line — the result must fall back to the focus node rather than
// invent one or borrow an unrelated line.
func TestSourceLinesForInferredTriples(t *testing.T) {
	data, idx := loadWithProvenance(t, `@prefix ex: <http://example.org/> .

ex:dave a ex:Person .
`)
	shapes := loadShapes(t, `@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [
        sh:path ex:name ;
        sh:minCount 1 ;
    ] .
`)

	report := shacl.Validate(data, shapes, shacl.WithSourceLines(idx))
	if len(report.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(report.Results))
	}
	r := report.Results[0]
	if r.SourceLine != 3 || r.SourceLineKind != shacl.SourceLineFocusNode {
		t.Errorf("got line %d (%s), want 3 (focus node)", r.SourceLine, r.SourceLineKind)
	}
}

// TestSourceLinesAcrossMultilineTriple checks a triple whose object sits on a
// different line from its subject. The parser reports the line the triple was
// completed on, which is where the offending value is written — not where the
// subject was introduced.
func TestSourceLinesAcrossMultilineTriple(t *testing.T) {
	data, idx := loadWithProvenance(t, `@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:erin
    a
        ex:Person ;
    ex:age
        "nope" .
`)
	shapes := loadShapes(t, `@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:PersonShape a sh:NodeShape ;
    sh:targetClass ex:Person ;
    sh:property [
        sh:path ex:age ;
        sh:datatype xsd:integer ;
    ] .
`)

	report := shacl.Validate(data, shapes, shacl.WithSourceLines(idx))
	if len(report.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(report.Results))
	}
	r := report.Results[0]
	if r.SourceLineKind != shacl.SourceLineTriple {
		t.Fatalf("got %s, want a triple line", r.SourceLineKind)
	}
	// "nope" is on line 8. Pointing at line 4, where ex:erin starts, would send
	// the reader to the wrong place in a long block.
	if r.SourceLine != 8 {
		t.Errorf("got line %d, want 8 — the line the offending value is on", r.SourceLine)
	}
}
