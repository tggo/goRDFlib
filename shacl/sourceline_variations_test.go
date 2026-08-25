package shacl_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/shacl"
)

// TestSourceLineKindString covers the rendering, which a caller printing a
// diagnostic relies on.
func TestSourceLineKindString(t *testing.T) {
	cases := []struct {
		kind shacl.SourceLineKind
		want string
	}{
		{shacl.SourceLineNone, "none"},
		{shacl.SourceLineTriple, "triple"},
		{shacl.SourceLineFocusNode, "focus node"},
		{shacl.SourceLineKind(99), "none"},
	}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.want {
			t.Errorf("SourceLineKind(%d).String() = %q, want %q", tc.kind, got, tc.want)
		}
	}
}

// TestSourceLineNonIRIPathFallsBack covers a sequence path. The value it
// reached is several hops from the focus node, so no single triple is the
// offender and the result must fall back to the focus node rather than pick one
// of the hops.
func TestSourceLineNonIRIPathFallsBack(t *testing.T) {
	data, idx := loadWithProvenance(t, `@prefix ex: <http://example.org/> .

ex:alice
    ex:address ex:addr1 .

ex:addr1
    ex:zip "not a number" .
`)
	shapes := loadShapes(t, `@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:AliceShape a sh:NodeShape ;
    sh:targetNode ex:alice ;
    sh:property [
        sh:path ( ex:address ex:zip ) ;
        sh:datatype xsd:integer ;
    ] .
`)

	report := shacl.Validate(data, shapes, shacl.WithSourceLines(idx))
	if len(report.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(report.Results))
	}
	r := report.Results[0]
	if r.SourceLineKind != shacl.SourceLineFocusNode {
		t.Errorf("got %s, want the focus node — a sequence path has no single offending triple",
			r.SourceLineKind)
	}
	// The subject's line is the earliest line one of its triples *completes*
	// on, which for a statement written across lines 3-4 is line 4 — the same
	// rule every parser here follows.
	if r.SourceLine != 4 {
		t.Errorf("got line %d, want 4", r.SourceLine)
	}
}

// TestSourceLineInversePathFallsBack is the same argument for `^ex:p`: the
// value is the subject of the triple, not its object, so the focus/path/value
// triple the lookup builds does not exist.
func TestSourceLineInversePathFallsBack(t *testing.T) {
	data, idx := loadWithProvenance(t, `@prefix ex: <http://example.org/> .

ex:bob ex:knows ex:target .
`)
	shapes := loadShapes(t, `@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:TargetShape a sh:NodeShape ;
    sh:targetNode ex:target ;
    sh:property [
        sh:path [ sh:inversePath ex:knows ] ;
        sh:maxCount 0 ;
    ] .
`)

	report := shacl.Validate(data, shapes, shacl.WithSourceLines(idx))
	if len(report.Results) == 0 {
		t.Fatal("expected a violation")
	}
	for _, r := range report.Results {
		if r.SourceLineKind == shacl.SourceLineTriple {
			t.Errorf("an inverse path produced a triple line; the (focus, path, value) triple does not exist")
		}
	}
}

// TestSourceLineInNestedResults covers sh:node, whose failures arrive as
// Details on an outer result. Annotation recurses, so the nested result names
// the line of the nested value rather than of the outer focus node.
func TestSourceLineInNestedResults(t *testing.T) {
	data, idx := loadWithProvenance(t, `@prefix ex: <http://example.org/> .

ex:order
    ex:customer ex:cust1 .

ex:cust1
    ex:age "young" .
`)
	shapes := loadShapes(t, `@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:CustomerShape a sh:NodeShape ;
    sh:property [
        sh:path ex:age ;
        sh:datatype xsd:integer ;
    ] .

ex:OrderShape a sh:NodeShape ;
    sh:targetNode ex:order ;
    sh:property [
        sh:path ex:customer ;
        sh:node ex:CustomerShape ;
    ] .
`)

	report := shacl.Validate(data, shapes, shacl.WithSourceLines(idx))
	if len(report.Results) == 0 {
		t.Fatal("expected a violation")
	}

	// Whether the nested failure surfaces as Details or as its own top-level
	// result depends on the shape; either way every result must carry a line,
	// and any nested one must point at the customer's own bad value on line 7.
	var seenNested bool
	var walk func(rs []shacl.ValidationResult)
	walk = func(rs []shacl.ValidationResult) {
		for _, r := range rs {
			if r.SourceLine == 0 {
				t.Errorf("result with no line: focus=%s path=%s value=%s",
					r.FocusNode, r.ResultPath, r.Value)
			}
			if r.FocusNode.Value() == "http://example.org/cust1" {
				seenNested = true
				if r.SourceLine != 7 {
					t.Errorf("the nested result reported line %d, want 7", r.SourceLine)
				}
			}
			walk(r.Details)
		}
	}
	walk(report.Results)

	if !seenNested {
		t.Log("no nested result surfaced; sh:node reported only the outer failure")
	}
}

// TestSourceLineTaggedLiterals covers values that are not plain strings. The
// lookup rebuilds a term from the validation result and keys it the same way
// the parser did, so a language tag, a base direction or a datatype has to
// survive the round trip — otherwise the lookup silently misses and every such
// violation degrades to the focus node.
func TestSourceLineTaggedLiterals(t *testing.T) {
	cases := []struct {
		name   string
		object string
		line   int
	}{
		{"language tag", `"bonjour"@fr`, 4},
		{"directional language tag", `"שלום"@he--rtl`, 4},
		{"explicit datatype", `"42"^^xsd:integer`, 4},
		{"decimal", `4.2`, 4},
		{"boolean", `true`, 4},
		{"escaped quote", `"he said \"hi\""`, 4},
		{"newline", `"one\ntwo"`, 4},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data, idx := loadWithProvenance(t, fmt.Sprintf(`@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:s ex:p %s .
`, tc.object))

			// A shape that rejects everything on ex:p, so the value itself is
			// what the result blames.
			shapes := loadShapes(t, `@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:Shape a sh:NodeShape ;
    sh:targetNode ex:s ;
    sh:property [
        sh:path ex:p ;
        sh:maxCount 0 ;
    ] .
`)

			report := shacl.Validate(data, shapes, shacl.WithSourceLines(idx))
			if len(report.Results) == 0 {
				t.Fatalf("expected a violation for %s", tc.object)
			}
			r := report.Results[0]
			if r.SourceLine != tc.line {
				t.Errorf("%s: line %d, want %d", tc.object, r.SourceLine, tc.line)
			}
		})
	}
}

// TestSourceLineValueLookupSurvivesTagging is the sharper form of the previous
// test: it checks the *triple* lookup rather than settling for any line. A
// value-level constraint names the value, so the result must resolve to the
// triple, not fall back.
func TestSourceLineValueLookupSurvivesTagging(t *testing.T) {
	cases := []struct{ name, object string }{
		{"plain", `"plain"`},
		{"language tag", `"bonjour"@fr`},
		{"directional language tag", `"שלום"@he--rtl`},
		{"explicit datatype", `"42"^^xsd:integer`},
		{"IRI", `<http://example.org/other>`},
		{"escaped quote", `"he said \"hi\""`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Two lines carrying ex:p, so a fallback to the focus node would
			// give line 4 while the real triple is on line 5.
			data, idx := loadWithProvenance(t, fmt.Sprintf(`@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:s ex:other "filler" ;
     ex:p %s .
`, tc.object))

			shapes := loadShapes(t, `@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .

ex:Shape a sh:NodeShape ;
    sh:targetNode ex:s ;
    sh:property [
        sh:path ex:p ;
        sh:nodeKind sh:BlankNode ;
    ] .
`)

			report := shacl.Validate(data, shapes, shacl.WithSourceLines(idx))
			if len(report.Results) != 1 {
				t.Fatalf("got %d results, want 1", len(report.Results))
			}
			r := report.Results[0]
			if r.SourceLineKind != shacl.SourceLineTriple {
				t.Fatalf("%s: got %s, want a triple line — the value is named by the result",
					tc.object, r.SourceLineKind)
			}
			if r.SourceLine != 5 {
				t.Errorf("%s: line %d, want 5", tc.object, r.SourceLine)
			}
		})
	}
}

// TestSourceLineBlankNodeFocus covers a focus node that is a blank node. Its
// label is minted by the parser, so the index and the validation result have to
// agree on it or the lookup misses.
func TestSourceLineBlankNodeFocus(t *testing.T) {
	data, idx := loadWithProvenance(t, `@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:container ex:item [
    a ex:Thing ;
    ex:size "big"
] .
`)
	shapes := loadShapes(t, `@prefix sh: <http://www.w3.org/ns/shacl#> .
@prefix ex: <http://example.org/> .
@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .

ex:ThingShape a sh:NodeShape ;
    sh:targetClass ex:Thing ;
    sh:property [
        sh:path ex:size ;
        sh:datatype xsd:integer ;
    ] .
`)

	report := shacl.Validate(data, shapes, shacl.WithSourceLines(idx))
	if len(report.Results) != 1 {
		t.Fatalf("got %d results, want 1", len(report.Results))
	}
	if report.Results[0].SourceLine == 0 {
		t.Error("a blank-node focus produced no line; the parser's label did not survive into the index")
	}
}

// TestSourceLineNeverExceedsTheSource is the invariant that matters most when a
// line is printed next to a file: a reported line must exist.
func TestSourceLineNeverExceedsTheSource(t *testing.T) {
	data, idx := loadWithProvenance(t, sourceLineData)
	shapes := loadShapes(t, sourceLineShapes)

	lineCount := len(strings.Split(sourceLineData, "\n"))
	report := shacl.Validate(data, shapes, shacl.WithSourceLines(idx))

	for _, r := range report.Results {
		if r.SourceLine < 0 || r.SourceLine > lineCount {
			t.Errorf("line %d is outside the %d-line document", r.SourceLine, lineCount)
		}
		if r.SourceLine == 0 && r.SourceLineKind != shacl.SourceLineNone {
			t.Errorf("kind %s with no line", r.SourceLineKind)
		}
		if r.SourceLine > 0 && r.SourceLineKind == shacl.SourceLineNone {
			t.Errorf("line %d reported as kind none", r.SourceLine)
		}
	}
}
