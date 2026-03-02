package shacl

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const testSuiteBase = "../testdata/w3c/data-shapes/data-shapes-test-suite/tests"

func TestW3CCoreTests(t *testing.T) {
	categories := []string{
		"core/complex",
		"core/misc",
		"core/node",
		"core/path",
		"core/property",
		"core/targets",
		"core/validation-reports",
	}

	var total, passed int64

	for _, cat := range categories {
		catDir := filepath.Join(testSuiteBase, cat)
		manifestPath := filepath.Join(catDir, "manifest.ttl")

		manifest, err := LoadTurtleFile(manifestPath)
		if err != nil {
			t.Errorf("Failed to load manifest %s: %v", cat, err)
			continue
		}

		includePred := IRI(MF + "include")
		includes := manifest.All(nil, &includePred, nil)

		for _, inc := range includes {
			testFile := inc.Object.Value()
			testFile = resolveURI(manifest.BaseURI(), testFile)
			testFilePath := uriToPath(testFile)

			if testFilePath == "" || !strings.HasSuffix(testFilePath, ".ttl") {
				continue
			}

			atomic.AddInt64(&total, 1)
			testName := cat + "/" + filepath.Base(testFilePath)

			t.Run(testName, func(t *testing.T) {
				if runSingleTest(t, testFilePath) {
					atomic.AddInt64(&passed, 1)
				}
			})
		}
	}

	t.Logf("Total: %d, Passed: %d", atomic.LoadInt64(&total), atomic.LoadInt64(&passed))
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

	expected := ParseExpectedReport(g, results[0])

	actual := Validate(dataGraph, shapesGraph)

	match, details := CompareReports(expected, actual)
	if !match {
		t.Errorf("Report mismatch:\n%s", details)
		return false
	}
	return true
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
