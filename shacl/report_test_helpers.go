package shacl

import (
	"sort"
	"strconv"
	"strings"
)

// ParseExpectedReport reads a validation report from the graph given a report node.
// This is intended for use in tests to parse expected results from W3C test files.
func ParseExpectedReport(g *Graph, reportNode Term) ValidationReport {
	report := ValidationReport{}

	conforms := g.Objects(reportNode, IRI(SH+"conforms"))
	if len(conforms) > 0 && conforms[0].Value() == "true" {
		report.Conforms = true
	}

	resultPred := IRI(SH + "result")
	resultNodes := g.Objects(reportNode, resultPred)
	for _, rn := range resultNodes {
		vr := parseValidationResult(g, rn)
		report.Results = append(report.Results, vr)
	}
	return report
}

func parseValidationResult(g *Graph, node Term) ValidationResult {
	vr := ValidationResult{}
	if vals := g.Objects(node, IRI(SH+"focusNode")); len(vals) > 0 {
		vr.FocusNode = vals[0]
	}
	if vals := g.Objects(node, IRI(SH+"resultPath")); len(vals) > 0 {
		vr.ResultPath = vals[0]
	}
	if vals := g.Objects(node, IRI(SH+"value")); len(vals) > 0 {
		vr.Value = vals[0]
	}
	if vals := g.Objects(node, IRI(SH+"sourceShape")); len(vals) > 0 {
		vr.SourceShape = vals[0]
	}
	if vals := g.Objects(node, IRI(SH+"sourceConstraintComponent")); len(vals) > 0 {
		vr.SourceConstraintComponent = vals[0]
	}
	if vals := g.Objects(node, IRI(SH+"resultSeverity")); len(vals) > 0 {
		vr.ResultSeverity = vals[0]
	}
	vr.ResultMessages = g.Objects(node, IRI(SH+"resultMessage"))
	return vr
}

// ResultKey produces a comparable string key for matching expected vs actual results.
func ResultKey(r ValidationResult) string {
	parts := []string{
		normalizeTerm(r.FocusNode),
		normalizeTerm(r.ResultPath),
		normalizeTerm(r.Value),
		normalizeTerm(r.SourceConstraintComponent),
		normalizeTerm(r.ResultSeverity),
		normalizeTerm(r.SourceShape),
	}
	return strings.Join(parts, "|")
}

func normalizeTerm(t Term) string {
	if t.IsBlank() {
		return "_:BLANK"
	}
	return t.String()
}

// SortResults sorts validation results by their keys.
func SortResults(results []ValidationResult) {
	sort.Slice(results, func(i, j int) bool {
		return ResultKey(results[i]) < ResultKey(results[j])
	})
}

// CompareReports checks whether actual matches expected (ignoring order, messages).
// It does not mutate the input slices.
func CompareReports(expected, actual ValidationReport) (match bool, details string) {
	if expected.Conforms != actual.Conforms {
		return false, "conforms mismatch: expected " + strconv.FormatBool(expected.Conforms) + " got " + strconv.FormatBool(actual.Conforms)
	}
	if len(expected.Results) != len(actual.Results) {
		var sb strings.Builder
		sb.WriteString("result count mismatch: expected ")
		sb.WriteString(strconv.Itoa(len(expected.Results)))
		sb.WriteString(" got ")
		sb.WriteString(strconv.Itoa(len(actual.Results)))
		sb.WriteString("\nExpected:\n")
		for _, r := range expected.Results {
			sb.WriteString("  " + ResultKey(r) + "\n")
		}
		sb.WriteString("Actual:\n")
		for _, r := range actual.Results {
			sb.WriteString("  " + ResultKey(r) + "\n")
		}
		return false, sb.String()
	}

	// Sort copies to avoid mutating input
	eCopy := make([]ValidationResult, len(expected.Results))
	copy(eCopy, expected.Results)
	aCopy := make([]ValidationResult, len(actual.Results))
	copy(aCopy, actual.Results)
	SortResults(eCopy)
	SortResults(aCopy)

	for i := range eCopy {
		ek := ResultKey(eCopy[i])
		ak := ResultKey(aCopy[i])
		if ek != ak {
			return false, "result mismatch at " + strconv.Itoa(i) + ":\n  expected: " + ek + "\n  actual:   " + ak
		}
	}
	return true, ""
}
