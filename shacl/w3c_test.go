package shacl

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const testSuiteBase = "../testdata/w3c/data-shapes/data-shapes-test-suite/tests"

func TestW3CCoreTests(t *testing.T) {
	runShaclManifestTree(t, testSuiteBase, "core")
}

// shacl10OnlyFailures lists the SHACL 1.0 test-suite cases that expect a
// failure for a restriction SHACL 1.2 lifted. The 1.0 specification forbade
// any VALUES clause and any sub-SELECT that does not project the pre-bound
// variables; SHACL-SPARQL 1.2 restricts only a VALUES that mentions a
// pre-bound variable, and substitutes pre-bound values into sub-SELECTs
// (the 1.2 suite's scope-interactions/projected-subquery-* cases need that).
// The engine implements the 1.2 rules, so these queries are legal and run.
var shacl10OnlyFailures = map[string]string{
	"data-shapes-test-suite/tests/sparql/pre-binding/pre-binding-006.ttl":        "1.0: a sub-SELECT * is unsupported; 1.2 pre-binds into sub-SELECTs",
	"data-shapes-test-suite/tests/sparql/pre-binding/unsupported-sparql-002.ttl": "1.0: any VALUES is unsupported; 1.2 only VALUES mentioning a pre-bound variable",
	"data-shapes-test-suite/tests/sparql/pre-binding/unsupported-sparql-004.ttl": "1.0: a sub-SELECT must project $this; 1.2 pre-binds into sub-SELECTs",
}

func shacl10OnlyFailureReason(testFilePath string) (string, bool) {
	p := filepath.ToSlash(testFilePath)
	i := strings.LastIndex(p, "data-shapes-test-suite/")
	if i < 0 {
		return "", false
	}
	reason, ok := shacl10OnlyFailures[p[i:]]
	return reason, ok
}

func runSingleTest(t *testing.T, testFilePath string) bool {
	t.Helper()

	g, err := LoadTurtleFile(testFilePath)
	if err != nil {
		t.Fatalf("Failed to load test file %s: %v", testFilePath, err)
	}

	typePred := IRI(RDFType)
	validateType := IRI(SHT + "Validate")
	testEntries := g.All(nil, &typePred, &validateType)
	if len(testEntries) == 0 {
		t.Skip("No sht:Validate entry found")
	}

	testNode := testEntries[0].Subject

	actionPred := IRI(MF + "action")
	actions := g.Objects(testNode, actionPred)
	if len(actions) == 0 {
		t.Fatal("No mf:action found")
	}
	actionNode := actions[0]

	dataGraphURI := ""
	shapesGraphURI := ""
	if dg := g.Objects(actionNode, IRI(SHT+"dataGraph")); len(dg) > 0 {
		dataGraphURI = dg[0].Value()
	}
	if sg := g.Objects(actionNode, IRI(SHT+"shapesGraph")); len(sg) > 0 {
		shapesGraphURI = sg[0].Value()
	}

	var dataGraph, shapesGraph *Graph

	if dataGraphURI == "" || dataGraphURI == g.BaseURI() {
		dataGraph = g
	} else {
		resolvedData := resolveURI(g.BaseURI(), dataGraphURI)
		dataPath := uriToPath(resolvedData)
		if dataPath == "" {
			dataPath = resolveRelativePath(testFilePath, dataGraphURI)
		}
		dataGraph, err = LoadTurtleFile(dataPath)
		if err != nil {
			t.Fatalf("Failed to load data graph %s: %v", dataPath, err)
		}
	}

	if shapesGraphURI == "" || shapesGraphURI == g.BaseURI() {
		shapesGraph = g
	} else if shapesGraphURI == dataGraphURI {
		shapesGraph = dataGraph
	} else {
		resolvedShapes := resolveURI(g.BaseURI(), shapesGraphURI)
		shapesPath := uriToPath(resolvedShapes)
		if shapesPath == "" {
			shapesPath = resolveRelativePath(testFilePath, shapesGraphURI)
		}
		shapesGraph, err = LoadTurtleFile(shapesPath)
		if err != nil {
			t.Fatalf("Failed to load shapes graph %s: %v", shapesPath, err)
		}
	}

	resultPred := IRI(MF + "result")
	results := g.Objects(testNode, resultPred)
	if len(results) == 0 {
		t.Fatal("No mf:result found")
	}

	// sht:Failure means the test expects a processing failure for implementations
	// that don't support certain SPARQL features. Our implementation supports them,
	// so we verify that validation completes without panicking.
	if results[0].IsIRI() && results[0].Value() == SHT+"Failure" {
		var reported []error
		actual, runErr := ValidateContext(context.Background(), dataGraph, shapesGraph,
			WithErrorHandler(func(err error) { reported = append(reported, err) }))
		failed := runErr != nil || len(reported) > 0
		reason, exempt := shacl10OnlyFailureReason(testFilePath)
		if exempt {
			if failed {
				t.Errorf("sht:Failure: this test is exempt (%s) but the run now reports a failure; remove it from shacl10OnlyFailures", reason)
				return false
			}
			t.Logf("sht:Failure exempt (%s): conforms=%v results=%d", reason, actual.Conforms, len(actual.Results))
			return true
		}
		if !failed {
			t.Errorf("sht:Failure: the run produced a report (conforms=%v, %d results) and reported no failure",
				actual.Conforms, len(actual.Results))
			return false
		}
		t.Logf("sht:Failure: run error %v, reported %v", runErr, reported)
		return true
	}

	expected := ParseExpectedReport(g, results[0])

	actual := Validate(dataGraph, shapesGraph)
	checkCompiledShapesAgree(t, dataGraph, shapesGraph, actual)

	// SHACL 1.2: sh:conformanceDisallows — recompute conforms using only listed severities
	conformanceDisallows := g.Objects(results[0], IRI(SH+"conformanceDisallows"))
	if len(conformanceDisallows) > 0 {
		disallowed := make(map[string]bool, len(conformanceDisallows))
		for _, cd := range conformanceDisallows {
			disallowed[cd.Value()] = true
		}
		actual.Conforms = true
		for _, r := range actual.Results {
			if disallowed[r.ResultSeverity.Value()] {
				actual.Conforms = false
				break
			}
		}
	}

	match, details := CompareReports(expected, actual)
	if !match {
		t.Errorf("Report mismatch:\n%s", details)
		return false
	}
	return true
}

func TestW3CSPARQLTests(t *testing.T) {
	runShaclManifestTree(t, testSuiteBase, "sparql")
}

const testSuite12Base = "../testdata/w3c/data-shapes/shacl12-test-suite/tests"

func TestW3CSHACL12CoreTests(t *testing.T) {
	runShaclManifestTree(t, testSuite12Base, "core")
}

func TestW3CSHACL12SPARQLTests(t *testing.T) {
	runShaclManifestTree(t, testSuite12Base, "sparql")
}

func termListsEqual(a, b []Term) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].TermKey() != b[i].TermKey() {
			return false
		}
	}
	return true
}

func termSetsEqual(a, b []Term) bool {
	if len(a) != len(b) {
		return false
	}
	counts := make(map[string]int, len(a))
	for _, t := range a {
		counts[t.TermKey()]++
	}
	for _, t := range b {
		k := t.TermKey()
		counts[k]--
		if counts[k] < 0 {
			return false
		}
	}
	return true
}

func termsToStrings(terms []Term) []string {
	result := make([]string, len(terms))
	for i, t := range terms {
		result[i] = t.String()
	}
	return result
}

func resolveURI(base, ref string) string {
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") || strings.HasPrefix(ref, "file://") {
		return ref
	}
	if strings.HasPrefix(base, "file://") {
		basePath := strings.TrimPrefix(base, "file://")
		dir := filepath.Dir(basePath)
		return "file://" + filepath.Join(dir, ref)
	}
	return ref
}

func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return ""
}

func resolveRelativePath(testFilePath, relativeURI string) string {
	dir := filepath.Dir(testFilePath)

	base := filepath.Base(testFilePath)
	baseName := strings.TrimSuffix(base, filepath.Ext(base))
	if strings.HasPrefix(relativeURI, baseName) {
		return filepath.Join(dir, relativeURI)
	}

	if _, err := os.Stat(filepath.Join(dir, relativeURI)); err == nil {
		return filepath.Join(dir, relativeURI)
	}

	withExt := relativeURI + ".ttl"
	if _, err := os.Stat(filepath.Join(dir, withExt)); err == nil {
		return filepath.Join(dir, withExt)
	}

	return filepath.Join(dir, relativeURI)
}
