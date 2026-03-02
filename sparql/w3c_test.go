package sparql_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/nt"
	"github.com/tggo/goRDFlib/rdfxml"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/testdata/w3c"
	"github.com/tggo/goRDFlib/turtle"
)

const sparql11ManifestPath = "../testdata/w3c/rdf-tests/sparql/sparql11/manifest-sparql11-query.ttl"

func TestW3C(t *testing.T) {
	if _, err := os.Stat(sparql11ManifestPath); err != nil {
		t.Skip("W3C SPARQL test suite not found (rdf-tests submodule)")
	}

	manifest, err := w3c.ParseIncludeManifest(sparql11ManifestPath)
	if err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	// Categorize by test type
	const (
		mfQueryEval    = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#QueryEvaluationTest"
		mfPosSyntax11  = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#PositiveSyntaxTest11"
		mfNegSyntax11  = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#NegativeSyntaxTest11"
	)

	for _, entry := range manifest.Entries {
		entry := entry
		switch entry.Type {
		case mfQueryEval:
			t.Run("eval/"+entry.Name, func(t *testing.T) {
				runQueryEvalTest(t, entry)
			})
		case mfPosSyntax11:
			t.Run("syntax+/"+entry.Name, func(t *testing.T) {
				runPositiveSyntaxTest(t, entry)
			})
		case mfNegSyntax11:
			t.Run("syntax-/"+entry.Name, func(t *testing.T) {
				runNegativeSyntaxTest(t, entry)
			})
		}
	}
}

func runQueryEvalTest(t *testing.T, entry w3c.TestEntry) {
	// Load default graph data
	g := rdflibgo.NewGraph()
	if entry.Data != "" {
		loadDataFile(t, g, entry.Data)
	}

	// Load named graphs
	var namedGraphs map[string]*rdflibgo.Graph
	if len(entry.GraphData) > 0 {
		namedGraphs = make(map[string]*rdflibgo.Graph)
		for _, gd := range entry.GraphData {
			ng := rdflibgo.NewGraph()
			loadDataFile(t, ng, gd)
			graphName := "file://" + gd
			namedGraphs[graphName] = ng
		}
		// If no explicit default graph data, add named graph data to default
		// (only for queries that don't use GRAPH patterns)
		if entry.Data == "" {
			for _, gd := range entry.GraphData {
				loadDataFile(t, g, gd)
			}
		}
	}

	// Read query
	queryPath := entry.Query
	if queryPath == "" {
		t.Skip("no query file")
	}
	queryBytes, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("failed to read query file %s: %v", queryPath, err)
	}
	queryStr := string(queryBytes)

	// Execute query
	var result *sparql.Result
	if namedGraphs != nil {
		pq, perr := sparql.Parse(queryStr)
		if perr != nil {
			t.Fatalf("query parse failed: %v", perr)
		}
		pq.NamedGraphs = namedGraphs
		var evalErr error
		result, evalErr = sparql.EvalQuery(g, pq, nil)
		err = evalErr
	} else {
		result, err = sparql.Query(g, queryStr)
	}
	if err != nil {
		t.Fatalf("query execution failed: %v", err)
	}

	// Load expected result
	if entry.Result == "" {
		t.Skip("no result file")
	}

	expected, err := loadExpectedResult(t, entry.Result, g)
	if err != nil {
		t.Fatalf("failed to load expected result %s: %v", entry.Result, err)
	}

	// Compare
	if !sparql.ResultsEqual(result, expected) {
		t.Errorf("result mismatch\nGot %d bindings, expected %d bindings", len(result.Bindings), len(expected.Bindings))
		if len(result.Bindings) <= 10 {
			t.Logf("Got:      %v", formatBindings(result.Bindings))
		}
		if len(expected.Bindings) <= 10 {
			t.Logf("Expected: %v", formatBindings(expected.Bindings))
		}
	}
}

func runPositiveSyntaxTest(t *testing.T, entry w3c.TestEntry) {
	queryPath := entry.Action
	if queryPath == "" {
		queryPath = entry.Query
	}
	if queryPath == "" {
		t.Skip("no query file")
	}

	queryBytes, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("failed to read query file: %v", err)
	}

	_, err = sparql.Parse(string(queryBytes))
	if err != nil {
		t.Errorf("expected valid syntax but got parse error: %v", err)
	}
}

func runNegativeSyntaxTest(t *testing.T, entry w3c.TestEntry) {
	queryPath := entry.Action
	if queryPath == "" {
		queryPath = entry.Query
	}
	if queryPath == "" {
		t.Skip("no query file")
	}

	queryBytes, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("failed to read query file: %v", err)
	}

	_, err = sparql.Parse(string(queryBytes))
	if err == nil {
		t.Errorf("expected parse error but query parsed successfully")
	}
}

func loadDataFile(t *testing.T, g *rdflibgo.Graph, path string) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("failed to open data file %s: %v", path, err)
	}
	defer f.Close()
	base := "file://" + path
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".ttl":
		if err := turtle.Parse(g, f, turtle.WithBase(base)); err != nil {
			t.Fatalf("failed to parse turtle data %s: %v", path, err)
		}
	case ".rdf", ".xml":
		if err := rdfxml.Parse(g, f, rdfxml.WithBase(base)); err != nil {
			t.Fatalf("failed to parse RDF/XML data %s: %v", path, err)
		}
	case ".nt":
		if err := nt.Parse(g, f, nt.WithBase(base)); err != nil {
			t.Fatalf("failed to parse N-Triples data %s: %v", path, err)
		}
	default:
		t.Fatalf("unsupported data format: %s", ext)
	}
}

func loadExpectedResult(t *testing.T, path string, g *rdflibgo.Graph) (*sparql.Result, error) {
	t.Helper()
	ext := strings.ToLower(filepath.Ext(path))

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	switch ext {
	case ".srx":
		return sparql.ParseSRX(f)
	case ".srj":
		return sparql.ParseSRJ(f)
	case ".ttl":
		// CONSTRUCT result: load as graph and compare
		expected := rdflibgo.NewGraph()
		base := "file://" + path
		if err := turtle.Parse(expected, f, turtle.WithBase(base)); err != nil {
			return nil, err
		}
		return &sparql.Result{Type: "CONSTRUCT", Graph: expected}, nil
	default:
		t.Skipf("unsupported result format: %s", ext)
		return nil, nil
	}
}

func formatBindings(bindings []map[string]rdflibgo.Term) string {
	var rows []string
	for _, b := range bindings {
		var parts []string
		for k, v := range b {
			if v != nil {
				parts = append(parts, k+"="+v.N3())
			}
		}
		rows = append(rows, "{"+strings.Join(parts, ", ")+"}")
	}
	return strings.Join(rows, "\n")
}
