package shacl_test

import (
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/provenance"
	"github.com/tggo/goRDFlib/shacl"
	"github.com/tggo/goRDFlib/turtle"
)

// FuzzSourceLines is the end-to-end property test: whatever Turtle goes in, no
// validation result may claim a line that does not exist, and the kind must
// always agree with whether there is a line at all.
//
// A line printed next to a file is only useful if it is never wrong, so the
// invariants worth fuzzing are about consistency rather than about any
// particular number.
func FuzzSourceLines(f *testing.F) {
	f.Add(sourceLineData)
	f.Add("@prefix ex: <http://example.org/> .\nex:a a ex:Person .\n")
	f.Add("@prefix ex: <http://example.org/> .\nex:a a ex:Person ; ex:name \"x\" ; ex:age 1 .\n")
	f.Add("@prefix ex: <http://example.org/> .\n[] a ex:Person .\n")
	f.Add("@prefix ex: <http://example.org/> .\nex:a a ex:Person ; ex:name \"x\"@en--rtl .\n")
	f.Add("")
	f.Add("@prefix")
	f.Add("<a> <b> <c> .")

	shapesSrc := sourceLineShapes

	f.Fuzz(func(t *testing.T, dataSrc string) {
		dataGraph := graph.NewGraph()
		idx := provenance.NewIndex()
		if err := turtle.Parse(dataGraph, strings.NewReader(dataSrc),
			turtle.WithProvenance(idx.Triple)); err != nil {
			return // a document that does not parse has nothing to validate
		}

		shapesGraph := graph.NewGraph()
		if err := turtle.Parse(shapesGraph, strings.NewReader(shapesSrc)); err != nil {
			t.Fatalf("the fixed shapes stopped parsing: %v", err)
		}

		report := shacl.Validate(
			shacl.NewGraphFromRDF(dataGraph, ""),
			shacl.NewGraphFromRDF(shapesGraph, ""),
			shacl.WithSourceLines(idx),
		)

		lineCount := 1 + strings.Count(dataSrc, "\n")

		var check func(rs []shacl.ValidationResult)
		check = func(rs []shacl.ValidationResult) {
			for _, r := range rs {
				if r.SourceLine < 0 {
					t.Fatalf("negative line %d", r.SourceLine)
				}
				if r.SourceLine > lineCount {
					t.Fatalf("line %d is outside the %d-line document", r.SourceLine, lineCount)
				}
				// The kind and the line must never disagree: a caller branches
				// on the kind to decide how to phrase the diagnostic.
				if (r.SourceLine == 0) != (r.SourceLineKind == shacl.SourceLineNone) {
					t.Fatalf("line %d with kind %s", r.SourceLine, r.SourceLineKind)
				}
				check(r.Details)
			}
		}
		check(report.Results)
	})
}

// FuzzSourceLinesNeverInvent checks the stronger claim behind SourceLineTriple:
// when a result says it is pointing at a triple, that triple must be one the
// parser recorded — not a line borrowed from somewhere else.
func FuzzSourceLinesNeverInvent(f *testing.F) {
	f.Add(sourceLineData)
	f.Add("@prefix ex: <http://example.org/> .\nex:a a ex:Person ; ex:age \"x\" .\n")
	f.Add("@prefix ex: <http://example.org/> .\nex:a a ex:Person ; ex:name 1 ; ex:name 2 .\n")

	f.Fuzz(func(t *testing.T, dataSrc string) {
		dataGraph := graph.NewGraph()
		idx := provenance.NewIndex()
		if err := turtle.Parse(dataGraph, strings.NewReader(dataSrc),
			turtle.WithProvenance(idx.Triple)); err != nil {
			return
		}

		shapesGraph := graph.NewGraph()
		if err := turtle.Parse(shapesGraph, strings.NewReader(sourceLineShapes)); err != nil {
			t.Fatal(err)
		}

		report := shacl.Validate(
			shacl.NewGraphFromRDF(dataGraph, ""),
			shacl.NewGraphFromRDF(shapesGraph, ""),
			shacl.WithSourceLines(idx),
		)

		var check func(rs []shacl.ValidationResult)
		check = func(rs []shacl.ValidationResult) {
			for _, r := range rs {
				switch r.SourceLineKind {
				case shacl.SourceLineTriple:
					// The result claims a specific triple, so the focus node,
					// path and value must be bound and the path an IRI.
					if r.FocusNode.IsNone() || !r.ResultPath.IsIRI() || r.Value.IsNone() {
						t.Fatalf("a triple line with focus=%s path=%s value=%s",
							r.FocusNode, r.ResultPath, r.Value)
					}
				case shacl.SourceLineFocusNode:
					if r.FocusNode.IsNone() {
						t.Fatal("a focus-node line with no focus node")
					}
				}
				check(r.Details)
			}
		}
		check(report.Results)
	})
}
