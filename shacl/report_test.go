package shacl

import (
	"strings"
	"testing"
)

func TestResultKey_BlankNormalization(t *testing.T) {
	t.Parallel()
	r := ValidationResult{
		FocusNode:                 BlankNode("b0"),
		SourceConstraintComponent: IRI(SH + "NodeKindConstraintComponent"),
		ResultSeverity:            SHViolation,
	}
	key := ResultKey(r)
	if got := key; got == "" {
		t.Fatal("expected non-empty key")
	}
	// Blank node should be normalized
	if !strings.Contains(key, "_:BLANK") {
		t.Errorf("expected blank node normalized to _:BLANK in key, got %q", key)
	}
}

func TestResultKey_IRIPreserved(t *testing.T) {
	t.Parallel()
	r := ValidationResult{
		FocusNode:                 IRI("http://example.org/Alice"),
		SourceConstraintComponent: IRI(SH + "NodeKindConstraintComponent"),
		ResultSeverity:            SHViolation,
	}
	key := ResultKey(r)
	if !strings.Contains(key, "example.org/Alice") {
		t.Errorf("expected IRI preserved in key, got %q", key)
	}
}

func TestCompareReports_Matching(t *testing.T) {
	t.Parallel()
	r1 := ValidationReport{
		Conforms: false,
		Results: []ValidationResult{
			{FocusNode: IRI("http://example.org/A"), ResultSeverity: SHViolation, SourceConstraintComponent: IRI(SH + "X")},
			{FocusNode: IRI("http://example.org/B"), ResultSeverity: SHViolation, SourceConstraintComponent: IRI(SH + "X")},
		},
	}
	r2 := ValidationReport{
		Conforms: false,
		Results: []ValidationResult{
			{FocusNode: IRI("http://example.org/B"), ResultSeverity: SHViolation, SourceConstraintComponent: IRI(SH + "X")},
			{FocusNode: IRI("http://example.org/A"), ResultSeverity: SHViolation, SourceConstraintComponent: IRI(SH + "X")},
		},
	}
	match, details := CompareReports(r1, r2)
	if !match {
		t.Errorf("expected match, got: %s", details)
	}
}

func TestCompareReports_ConformsMismatch(t *testing.T) {
	t.Parallel()
	r1 := ValidationReport{Conforms: true}
	r2 := ValidationReport{Conforms: false}
	match, details := CompareReports(r1, r2)
	if match {
		t.Error("expected mismatch")
	}
	if !strings.Contains(details, "conforms mismatch") {
		t.Errorf("expected conforms mismatch detail, got %q", details)
	}
}

func TestCompareReports_CountMismatch(t *testing.T) {
	t.Parallel()
	r1 := ValidationReport{
		Conforms: false,
		Results:  []ValidationResult{{FocusNode: IRI("http://example.org/A")}},
	}
	r2 := ValidationReport{Conforms: false}
	match, details := CompareReports(r1, r2)
	if match {
		t.Error("expected mismatch")
	}
	if !strings.Contains(details, "result count mismatch") {
		t.Errorf("expected count mismatch detail, got %q", details)
	}
}

func TestCompareReports_ResultMismatch(t *testing.T) {
	t.Parallel()
	r1 := ValidationReport{
		Conforms: false,
		Results:  []ValidationResult{{FocusNode: IRI("http://example.org/A"), ResultSeverity: SHViolation}},
	}
	r2 := ValidationReport{
		Conforms: false,
		Results:  []ValidationResult{{FocusNode: IRI("http://example.org/B"), ResultSeverity: SHViolation}},
	}
	match, _ := CompareReports(r1, r2)
	if match {
		t.Error("expected mismatch for different focus nodes")
	}
}

func TestCompareReports_DoesNotMutateInput(t *testing.T) {
	t.Parallel()
	results := []ValidationResult{
		{FocusNode: IRI("http://example.org/B"), ResultSeverity: SHViolation, SourceConstraintComponent: IRI(SH + "X")},
		{FocusNode: IRI("http://example.org/A"), ResultSeverity: SHViolation, SourceConstraintComponent: IRI(SH + "X")},
	}
	// Save original order
	first := results[0].FocusNode.Value()
	second := results[1].FocusNode.Value()

	r := ValidationReport{Conforms: false, Results: results}
	CompareReports(r, r)

	if results[0].FocusNode.Value() != first || results[1].FocusNode.Value() != second {
		t.Error("CompareReports mutated input slice order")
	}
}
