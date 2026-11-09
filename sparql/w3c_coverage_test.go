package sparql_test

import (
	"os"
	"path"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/tggo/goRDFlib/testdata/w3c"
)

const w3cSPARQLRoot = "../testdata/w3c/rdf-tests/sparql"

// TestW3CJSONResults runs sparql11/json-res: queries whose expected answer is
// a SPARQL Results JSON document. It sits outside manifest-sparql11-query.ttl
// (it belongs to the results manifest), so TestW3C never reached it.
func TestW3CJSONResults(t *testing.T) {
	manifestPath := w3cSPARQLRoot + "/sparql11/json-res/manifest.ttl"
	if _, err := os.Stat(manifestPath); err != nil {
		t.Skip("W3C SPARQL test suite not found (rdf-tests submodule)")
	}
	manifest, err := w3c.ParseManifest(manifestPath)
	if err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}
	const queryEval = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#QueryEvaluationTest"
	n := 0
	for _, entry := range manifest.Entries {
		if entry.Type != queryEval {
			t.Errorf("%s: unexpected test type %s", entry.Name, entry.Type)
			continue
		}
		n++
		t.Run("eval/"+entry.Name, func(t *testing.T) { runQueryEvalTest(t, entry) })
	}
	if n != 4 {
		t.Fatalf("got %d json-res tests, want 4", n)
	}
}

// w3cSPARQLElsewhere lists the leaf manifests under rdf-tests/sparql that no
// runner in this package executes, and why. Every other manifest has to be
// reachable from one of the roots the runners load; TestW3CEveryManifestIsRun
// fails on a directory that is neither, so a new upstream suite cannot be
// missed silently.
var w3cSPARQLElsewhere = map[string]string{
	// Run by other packages.
	"sparql11/csv-tsv-res":          "sparql/results TestW3CCSVTSV",
	"sparql11/protocol":             "endpoint (SPARQL Protocol)",
	"sparql11/http-rdf-update":      "endpoint (Graph Store HTTP, update)",
	"sparql11/graph-store-protocol": "endpoint (Graph Store HTTP Protocol)",
	// Features the engine does not have.
	"sparql11/syntax-fed":          "SERVICE is not supported (parse error): federated query is not implemented",
	"sparql11/service":             "SERVICE is not supported: federated query is not implemented",
	"sparql11/entailment":          "SPARQL entailment regimes (RDFS, OWL, RIF) are not implemented",
	"sparql11/service-description": "service description is a protocol concern of the binary, not the library",
}

var manifestRef = regexp.MustCompile(`<([^<>\s]*manifest[^<>\s]*\.ttl)>`)

// reachableManifests follows every manifest reference of root, recursively,
// and adds the directories of those that are found to dirs.
func reachableManifests(t *testing.T, root string, dirs map[string]bool, seen map[string]bool) {
	t.Helper()
	if seen[root] {
		return
	}
	seen[root] = true
	data, err := os.ReadFile(root)
	if err != nil {
		t.Fatalf("manifest %s: %v", root, err)
	}
	dirs[filepath.Dir(root)] = true
	for _, m := range manifestRef.FindAllStringSubmatch(string(data), -1) {
		reachableManifests(t, path.Join(filepath.Dir(root), m[1]), dirs, seen)
	}
}

// TestW3CEveryManifestIsRun walks rdf-tests/sparql and requires every manifest
// to be either reached from a root a test in this package loads or listed in
// w3cSPARQLElsewhere with a reason.
func TestW3CEveryManifestIsRun(t *testing.T) {
	if _, err := os.Stat(w3cSPARQLRoot + "/sparql12/manifest.ttl"); err != nil {
		t.Skip("W3C SPARQL test suite not found (rdf-tests submodule)")
	}
	roots := []string{
		sparql10ManifestPath,
		sparql11ManifestPath,
		sparql11UpdateManifestPath,
		sparql12ManifestPath,
		w3cSPARQLRoot + "/sparql11/json-res/manifest.ttl", // TestW3CJSONResults
	}
	run := map[string]bool{}
	seen := map[string]bool{}
	for _, r := range roots {
		reachableManifests(t, r, run, seen)
	}

	found := 0
	err := filepath.WalkDir(w3cSPARQLRoot, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "manifest.ttl" {
			return err
		}
		found++
		dir := filepath.Dir(p)
		rel, err := filepath.Rel(w3cSPARQLRoot, dir)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if _, ok := w3cSPARQLElsewhere[rel]; ok {
			if run[dir] {
				t.Errorf("%s is listed in w3cSPARQLElsewhere but a runner here now reaches it: remove the entry", rel)
			}
			return nil
		}
		if !run[dir] {
			t.Errorf("%s/manifest.ttl is reached by no runner and has no entry in w3cSPARQLElsewhere", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found < 60 {
		t.Fatalf("only %d manifests found under %s", found, w3cSPARQLRoot)
	}
	for rel, why := range w3cSPARQLElsewhere {
		if why == "" {
			t.Errorf("w3cSPARQLElsewhere[%q] has no reason", rel)
		}
	}
}
