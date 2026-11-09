package shacl

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/testutil"
)

// The SRL (SHACL Rules / SPARQL 1.2 RL) test suite moved: at data-shapes
// e0a20128 it lived in shacl12-test-suite/tests/rules with the
// shacl-rules-test# vocabulary; upstream now has it in
// shacl12-test-suite/tests/sparql-rl (sparql-rl-tests#), with the evaluation
// tests split over eval/, eval2/ and examples/. Both layouts are read, so the
// runner works at either submodule pin.
const (
	srlTestBaseNew = "../testdata/w3c/data-shapes/shacl12-test-suite/tests/sparql-rl"
	srlTestBaseOld = "../testdata/w3c/data-shapes/shacl12-test-suite/tests/rules"
)

var srtNamespaces = []string{
	"http://www.w3.org/ns/sparql-rl-tests#",
	"http://www.w3.org/ns/shacl-rules-test#",
}

func srlTestBase() string {
	if st, err := os.Stat(srlTestBaseNew); err == nil && st.IsDir() {
		return srlTestBaseNew
	}
	return srlTestBaseOld
}

// srlEvalDirs lists the directories holding RulesEvalTest manifests.
func srlEvalDirs() []string {
	base := srlTestBase()
	dirs := []string{filepath.Join(base, "eval")}
	for _, d := range []string{"eval2", "examples"} {
		p := filepath.Join(base, d)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			dirs = append(dirs, p)
		}
	}
	return dirs
}

type srlTestEntry struct {
	Name     string
	TestType string // e.g. "RulesPositiveSyntaxTest"
	Action   string // path to .srl file
	Ruleset  string // for eval tests: path to .srl file
	Data     string // for eval tests: path to .ttl file
	Result   string // for eval tests: path to expected .ttl file
}

func parseSRLManifest(t *testing.T, dir string) []srlTestEntry {
	t.Helper()
	manifestPath := filepath.Join(dir, "manifest.ttl")
	g, err := LoadTurtleFile(manifestPath)
	if err != nil {
		t.Fatalf("Failed to load manifest %s: %v", manifestPath, err)
	}

	typePred := IRI(RDFType)
	namePred := IRI(MF + "name")
	actionPred := IRI(MF + "action")
	resultPred := IRI(MF + "result")

	testTypes := []string{
		"RulesPositiveSyntaxTest",
		"RulesNegativeSyntaxTest",
		"RulesPositiveWellFormednessTest",
		"RulesNegativeWellFormednessTest",
		"RulesPositiveStratificationTest",
		"RulesNegativeStratificationTest",
		"RulesEvalTest",
	}

	var entries []srlTestEntry
	declared := 0
	for _, ns := range srtNamespaces {
		rulesetPred := IRI(ns + "ruleset")
		dataPred := IRI(ns + "data")
		for _, tt := range testTypes {
			typeIRI := IRI(ns + tt)
			tests := g.All(nil, &typePred, &typeIRI)
			declared += len(tests)
			for _, test := range tests {
				node := test.Subject
				entry := srlTestEntry{TestType: tt}

				if names := g.Objects(node, namePred); len(names) > 0 {
					entry.Name = names[0].Value()
				}

				actions := g.Objects(node, actionPred)
				if len(actions) == 0 {
					continue // reported by the declared/readable check below
				}

				if tt == "RulesEvalTest" {
					// Eval test: action is a blank node with srt:ruleset and srt:data
					actionNode := actions[0]
					if rs := g.Objects(actionNode, rulesetPred); len(rs) > 0 {
						entry.Ruleset = resolveSRLPath(dir, rs[0].Value(), g.BaseURI())
					}
					if d := g.Objects(actionNode, dataPred); len(d) > 0 {
						entry.Data = resolveSRLPath(dir, d[0].Value(), g.BaseURI())
					}
					if r := g.Objects(node, resultPred); len(r) > 0 {
						entry.Result = resolveSRLPath(dir, r[0].Value(), g.BaseURI())
					}
				} else {
					entry.Action = resolveSRLPath(dir, actions[0].Value(), g.BaseURI())
				}

				entries = append(entries, entry)
			}
		}
	}
	if n := len(g.All(nil, &actionPred, nil)); n != declared {
		t.Errorf("manifest %s has %d tests (mf:action) but %d of a type this runner knows: an unknown test type would be skipped silently", manifestPath, n, declared)
	}
	if len(entries) != declared {
		t.Errorf("manifest %s declares %d tests but %d were readable (missing mf:action?)", manifestPath, declared, len(entries))
	}
	return entries
}

func resolveSRLPath(dir, ref, base string) string {
	// If it's a file URI, convert to path.
	if strings.HasPrefix(ref, "file://") {
		return strings.TrimPrefix(ref, "file://")
	}
	// If it starts with the base URI, strip it.
	if base != "" && strings.HasPrefix(ref, base) {
		ref = strings.TrimPrefix(ref, base)
	}
	// If it's still a full URI, try to extract just the filename.
	if strings.Contains(ref, "://") {
		// Extract last path component.
		parts := strings.Split(ref, "/")
		ref = parts[len(parts)-1]
	}
	return filepath.Join(dir, ref)
}

func TestW3CSRLSyntaxTests(t *testing.T) {
	dir := filepath.Join(srlTestBase(), "syntax")
	entries := parseSRLManifest(t, dir)

	var total, passed int
	for _, entry := range entries {
		if entry.TestType != "RulesPositiveSyntaxTest" && entry.TestType != "RulesNegativeSyntaxTest" {
			continue
		}
		total++
		name := entry.Name
		if name == "" {
			name = filepath.Base(entry.Action)
		}
		isPositive := entry.TestType == "RulesPositiveSyntaxTest"
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(entry.Action)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", entry.Action, err)
			}
			_, parseErr := ParseSRL(string(data))
			if isPositive {
				if parseErr != nil {
					t.Errorf("Expected positive syntax test to pass, got error: %v", parseErr)
				} else {
					passed++
				}
			} else {
				if parseErr == nil {
					t.Errorf("Expected negative syntax test to fail, but it parsed successfully")
				} else {
					passed++
				}
			}
		})
	}
	t.Logf("SRL Syntax: %d/%d passed", passed, total)
}

func TestW3CSRLWellformedTests(t *testing.T) {
	dir := filepath.Join(srlTestBase(), "wellformed")
	entries := parseSRLManifest(t, dir)

	var total, passed int
	for _, entry := range entries {
		if entry.TestType != "RulesPositiveWellFormednessTest" && entry.TestType != "RulesNegativeWellFormednessTest" {
			continue
		}
		total++
		name := entry.Name
		if name == "" {
			name = filepath.Base(entry.Action)
		}
		isPositive := entry.TestType == "RulesPositiveWellFormednessTest"
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(entry.Action)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", entry.Action, err)
			}
			rs, parseErr := ParseSRL(string(data))
			if parseErr != nil {
				if isPositive {
					t.Errorf("Failed to parse: %v", parseErr)
				} else {
					passed++ // negative test passed by parse failure
				}
				return
			}
			wfErr := CheckWellformed(rs)
			if isPositive {
				if wfErr != nil {
					t.Errorf("Expected well-formed, got error: %v", wfErr)
				} else {
					passed++
				}
			} else {
				if wfErr == nil {
					t.Errorf("Expected not well-formed, but check passed")
				} else {
					passed++
				}
			}
		})
	}
	t.Logf("SRL Well-formedness: %d/%d passed", passed, total)
}

func TestW3CSRLStratificationTests(t *testing.T) {
	dir := filepath.Join(srlTestBase(), "stratification")
	entries := parseSRLManifest(t, dir)

	var total, passed int
	for _, entry := range entries {
		if entry.TestType != "RulesPositiveStratificationTest" && entry.TestType != "RulesNegativeStratificationTest" {
			continue
		}
		total++
		name := entry.Name
		if name == "" {
			name = filepath.Base(entry.Action)
		}
		isPositive := entry.TestType == "RulesPositiveStratificationTest"
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(entry.Action)
			if err != nil {
				t.Fatalf("Failed to read %s: %v", entry.Action, err)
			}
			rs, parseErr := ParseSRL(string(data))
			if parseErr != nil {
				if isPositive {
					t.Errorf("Failed to parse: %v", parseErr)
				} else {
					passed++
				}
				return
			}
			_, stratErr := Stratify(rs)
			if isPositive {
				if stratErr != nil {
					t.Errorf("Expected stratifiable, got error: %v", stratErr)
				} else {
					passed++
				}
			} else {
				if stratErr == nil {
					t.Errorf("Expected not stratifiable, but stratification succeeded")
				} else {
					passed++
				}
			}
		})
	}
	t.Logf("SRL Stratification: %d/%d passed", passed, total)
}

// srlEvalBroken lists evaluation tests whose files the suite itself lacks.
// Each is checked: if its ruleset appears, the run fails and the entry must go.
var srlEvalBroken = map[string]string{
	"Eval-assign-01": "the eval2 manifest names eval-assign-01.srl, which exists only in eval/ " +
		"(the same test runs there as eval-assign-01): a manifest defect upstream",
}

// TestW3CSRLManifestsAllRun checks the runner reads every manifest the root
// manifest includes, so a suite that grows a directory is not skipped.
func TestW3CSRLManifestsAllRun(t *testing.T) {
	base := srlTestBase()
	root := filepath.Join(base, "manifest-sparql-rl.ttl")
	if _, err := os.Stat(root); err != nil {
		t.Skipf("no root manifest at this submodule pin (%v)", err)
	}
	g, err := LoadTurtleFile(root)
	if err != nil {
		t.Fatal(err)
	}
	include := IRI(MF + "include")
	var included []string
	for _, tr := range g.All(nil, &include, nil) {
		for _, m := range g.RDFList(tr.Object) {
			p := resolveSRLPath(base, m.Value(), g.BaseURI())
			included = append(included, filepath.Dir(p))
		}
	}
	if len(included) == 0 {
		t.Fatalf("%s lists no sub-manifests", root)
	}
	run := map[string]bool{
		filepath.Join(base, "syntax"):         true,
		filepath.Join(base, "wellformed"):     true,
		filepath.Join(base, "stratification"): true,
	}
	for _, d := range srlEvalDirs() {
		run[d] = true
	}
	for _, d := range included {
		if !run[d] {
			t.Errorf("the root manifest includes %s, which no SRL runner reads", d)
		}
	}
	if len(included) != len(run) {
		t.Errorf("the root manifest includes %d manifests, the runner reads %d", len(included), len(run))
	}
}

func TestW3CSRLEvalTests(t *testing.T) {
	var entries []srlTestEntry
	for _, dir := range srlEvalDirs() {
		entries = append(entries, parseSRLManifest(t, dir)...)
	}

	var total, passed int
	for _, entry := range entries {
		if entry.TestType != "RulesEvalTest" {
			continue
		}
		name := entry.Name
		if reason, ok := srlEvalBroken[name]; ok {
			if _, err := os.Stat(entry.Ruleset); err == nil {
				t.Errorf("%s is listed as broken (%s) but its ruleset exists: remove the exemption", name, reason)
			}
			t.Logf("SRL eval %s not run: %s", name, reason)
			continue
		}
		total++
		t.Run(name, func(t *testing.T) {
			// Load ruleset.
			srlData, err := os.ReadFile(entry.Ruleset)
			if err != nil {
				t.Fatalf("Failed to read ruleset %s: %v", entry.Ruleset, err)
			}
			rs, parseErr := ParseSRL(string(srlData))
			if parseErr != nil {
				t.Fatalf("Failed to parse ruleset: %v", parseErr)
			}

			// Load data graph.
			dataGraph, err := LoadTurtleFile(entry.Data)
			if err != nil {
				t.Fatalf("Failed to load data %s: %v", entry.Data, err)
			}

			// Evaluate.
			resultGraph, err := EvalRuleSet(rs, dataGraph)
			if err != nil {
				t.Fatalf("Evaluation failed: %v", err)
			}

			// Load expected result.
			expectedGraph, err := LoadTurtleFile(entry.Result)
			if err != nil {
				t.Fatalf("Failed to load expected result %s: %v", entry.Result, err)
			}

			// Compare graphs.
			if !srlGraphsEqual(t, resultGraph, expectedGraph) {
				t.Errorf("Result graph does not match expected")
				t.Logf("Got triples:")
				for _, tr := range resultGraph.Triples() {
					t.Logf("  %s %s %s", tr.Subject.TermKey(), tr.Predicate.TermKey(), tr.Object.TermKey())
				}
				t.Logf("Expected triples:")
				for _, tr := range expectedGraph.Triples() {
					t.Logf("  %s %s %s", tr.Subject.TermKey(), tr.Predicate.TermKey(), tr.Object.TermKey())
				}
			} else {
				passed++
			}
		})
	}
	t.Logf("SRL Eval: %d/%d passed", passed, total)
}

// srlGraphsEqual compares two graphs for isomorphism, so a blank node of the
// inference graph only has to correspond to one of the expected graph.
func srlGraphsEqual(t *testing.T, got, expected *Graph) bool {
	t.Helper()
	ctx := context.Background()
	gr, err := got.rdf(ctx)
	if err != nil {
		t.Fatalf("reading the result graph: %v", err)
	}
	er, err := expected.rdf(ctx)
	if err != nil {
		t.Fatalf("reading the expected graph: %v", err)
	}
	ok, err := testutil.Isomorphic(ctx, gr, er)
	if err != nil {
		t.Fatalf("comparing graphs: %v", err)
	}
	return ok
}
