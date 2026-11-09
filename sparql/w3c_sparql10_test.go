package sparql_test

import (
	"os"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/testdata/w3c"
)

const sparql10ManifestPath = "../testdata/w3c/rdf-tests/sparql/sparql10/manifest.ttl"

// TestW3CSPARQL10 runs the DAWG SPARQL 1.0 suites (syntax-sparql1..5 and
// every evaluation directory). Only dataset/ used to run (TestW3CDataset);
// the rest was never wired in, and its syntax tests held parser bugs that
// the 1.1 suites do not reach. dataset/ is left to TestW3CDataset, which
// builds the dataset from the query's FROM clauses.
func TestW3CSPARQL10(t *testing.T) {
	if _, err := os.Stat(sparql10ManifestPath); err != nil {
		t.Skip("W3C SPARQL test suite not found (rdf-tests submodule)")
	}
	manifest, err := w3c.ParseIncludeManifest(sparql10ManifestPath)
	if err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}
	const (
		mf        = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#"
		queryEval = mf + "QueryEvaluationTest"
		posSyntax = mf + "PositiveSyntaxTest"
		negSyntax = mf + "NegativeSyntaxTest"
	)
	n := 0
	for _, entry := range manifest.Entries {
		if isDatasetEntry(entry) {
			continue
		}
		switch entry.Type {
		case queryEval:
			n++
			t.Run("eval/"+entry.Name, func(t *testing.T) { runQueryEvalTest(t, entry) })
		case posSyntax:
			n++
			t.Run("syntax+/"+entry.Name, func(t *testing.T) { runPositiveSyntaxTest(t, entry) })
		case negSyntax:
			n++
			t.Run("syntax-/"+entry.Name, func(t *testing.T) { runNegativeSyntaxTest(t, entry) })
		}
	}
	if n < 400 {
		t.Fatalf("only %d SPARQL 1.0 tests found: the manifest did not load", n)
	}
	t.Logf("%d SPARQL 1.0 tests", n)
}

func isDatasetEntry(e w3c.TestEntry) bool {
	return strings.Contains(e.Query+e.Action, "/sparql10/dataset/")
}
