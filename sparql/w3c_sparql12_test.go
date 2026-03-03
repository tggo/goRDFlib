package sparql_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/testdata/w3c"
	"github.com/tggo/goRDFlib/turtle"
)

const sparql12ManifestPath = "../testdata/w3c/rdf-tests/sparql/sparql12/manifest.ttl"

func TestW3CSPARQL12(t *testing.T) {
	if _, err := os.Stat(sparql12ManifestPath); err != nil {
		t.Skip("W3C SPARQL 1.2 test suite not found (rdf-tests submodule)")
	}

	manifest, err := w3c.ParseIncludeManifest(sparql12ManifestPath)
	if err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}

	const (
		mfQueryEval     = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#QueryEvaluationTest"
		mfUpdateEval    = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#UpdateEvaluationTest"
		mfPosSyntax     = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#PositiveSyntaxTest"
		mfNegSyntax     = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#NegativeSyntaxTest"
		mfPosSyntax11   = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#PositiveSyntaxTest11"
		mfNegSyntax11   = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#NegativeSyntaxTest11"
	)

	for _, entry := range manifest.Entries {
		entry := entry
		switch entry.Type {
		case mfQueryEval:
			t.Run("eval/"+entry.Name, func(t *testing.T) {
				runSPARQL12QueryEvalTest(t, entry)
			})
		case mfUpdateEval:
			t.Run("update/"+entry.Name, func(t *testing.T) {
				runSPARQL12UpdateEvalTest(t, entry)
			})
		case mfPosSyntax, mfPosSyntax11:
			t.Run("syntax+/"+entry.Name, func(t *testing.T) {
				runSPARQL12PositiveSyntaxTest(t, entry)
			})
		case mfNegSyntax, mfNegSyntax11:
			t.Run("syntax-/"+entry.Name, func(t *testing.T) {
				runSPARQL12NegativeSyntaxTest(t, entry)
			})
		}
	}
}

func runSPARQL12QueryEvalTest(t *testing.T, entry w3c.TestEntry) {
	g := rdflibgo.NewGraph()
	var namedGraphs map[string]*rdflibgo.Graph

	if entry.Data != "" {
		ext := strings.ToLower(filepath.Ext(entry.Data))
		if ext == ".trig" {
			var ng map[string]*rdflibgo.Graph
			g, ng = loadTrigFile(t, entry.Data)
			if len(ng) > 0 {
				namedGraphs = ng
			}
		} else {
			loadDataFile(t, g, entry.Data)
		}
	}

	if len(entry.GraphData) > 0 {
		if namedGraphs == nil {
			namedGraphs = make(map[string]*rdflibgo.Graph)
		}
		for _, gd := range entry.GraphData {
			ng := rdflibgo.NewGraph()
			loadDataFile(t, ng, gd)
			graphName := "file://" + gd
			namedGraphs[graphName] = ng
		}
		if entry.Data == "" {
			for _, gd := range entry.GraphData {
				loadDataFile(t, g, gd)
			}
		}
	}

	queryPath := entry.Query
	if queryPath == "" {
		t.Skip("no query file")
	}
	queryBytes, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatalf("failed to read query file %s: %v", queryPath, err)
	}
	queryStr := string(queryBytes)

	var result *sparql.Result
	if namedGraphs != nil {
		pq, perr := sparql.Parse(queryStr)
		if perr != nil {
			t.Fatalf("query parse failed: %v", perr)
		}
		pq.NamedGraphs = namedGraphs
		if pq.BaseURI == "" {
			pq.BaseURI = "file://" + filepath.Dir(queryPath) + "/"
		}
		result, err = sparql.EvalQuery(g, pq, nil)
	} else {
		result, err = sparql.Query(g, queryStr)
	}
	if err != nil {
		t.Fatalf("query execution failed: %v", err)
	}

	if entry.Result == "" {
		t.Skip("no result file")
	}

	expected, err := loadExpectedResult(t, entry.Result, g)
	if err != nil {
		t.Fatalf("failed to load expected result %s: %v", entry.Result, err)
	}

	if !sparql.ResultsEqual(result, expected) {
		t.Errorf("result mismatch\nGot %d bindings, expected %d bindings", len(result.Bindings), len(expected.Bindings))
		if len(result.Bindings) <= 20 {
			t.Logf("Got:      %v", formatBindings(result.Bindings))
		}
		if len(expected.Bindings) <= 20 {
			t.Logf("Expected: %v", formatBindings(expected.Bindings))
		}
	}
}

func runSPARQL12UpdateEvalTest(t *testing.T, entry w3c.TestEntry) {
	if entry.Request == "" {
		t.Skip("no request file")
	}

	reqBytes, err := os.ReadFile(entry.Request)
	if err != nil {
		t.Fatalf("failed to read request file: %v", err)
	}
	reqStr := string(reqBytes)

	ds := &sparql.Dataset{
		Default:     rdflibgo.NewGraph(),
		NamedGraphs: make(map[string]*rdflibgo.Graph),
	}

	if entry.ActionData != "" {
		ext := strings.ToLower(filepath.Ext(entry.ActionData))
		if ext == ".trig" {
			defG, ngs := loadTrigFile(t, entry.ActionData)
			ds.Default = defG
			for k, v := range ngs {
				ds.NamedGraphs[k] = v
			}
		} else {
			loadDataFile(t, ds.Default, entry.ActionData)
		}
	}
	for _, gd := range entry.ActionGraphData {
		ng := rdflibgo.NewGraph()
		loadDataFile(t, ng, gd.Graph)
		ds.NamedGraphs[gd.Label] = ng
	}

	if err := sparql.Update(ds, reqStr); err != nil {
		t.Fatalf("update execution failed: %v", err)
	}

	// Check result
	if entry.ResultData != "" {
		ext := strings.ToLower(filepath.Ext(entry.ResultData))
		if ext == ".trig" {
			expectedDefault, expectedNGs := loadTrigFile(t, entry.ResultData)
			if !graphsIsomorphic(t, ds.Default, expectedDefault) {
				t.Errorf("default graph mismatch after update")
				t.Logf("Got %d triples, expected %d", len(collectTriples(ds.Default)), len(collectTriples(expectedDefault)))
			}
			for name, expectedNG := range expectedNGs {
				actualNG := ds.NamedGraphs[name]
				if actualNG == nil {
					actualNG = rdflibgo.NewGraph()
				}
				if !graphsIsomorphic(t, actualNG, expectedNG) {
					t.Errorf("named graph %s mismatch after update", name)
				}
			}
		} else {
			expectedDefault := rdflibgo.NewGraph()
			loadDataFile(t, expectedDefault, entry.ResultData)
			if !graphsIsomorphic(t, ds.Default, expectedDefault) {
				t.Errorf("default graph mismatch after update")
			}
		}
	}
	for _, gd := range entry.ResultGraphData {
		expectedNG := rdflibgo.NewGraph()
		loadDataFile(t, expectedNG, gd.Graph)
		actualNG := ds.NamedGraphs[gd.Label]
		if actualNG == nil {
			actualNG = rdflibgo.NewGraph()
		}
		if !graphsIsomorphic(t, actualNG, expectedNG) {
			t.Errorf("named graph %s mismatch after update", gd.Label)
		}
	}
}

func runSPARQL12PositiveSyntaxTest(t *testing.T, entry w3c.TestEntry) {
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

	input := string(queryBytes)

	// Try as query first, then as update
	_, err = sparql.Parse(input)
	if err != nil {
		// Some positive syntax tests might be update queries
		_, err2 := sparql.ParseUpdate(input)
		if err2 != nil {
			t.Errorf("expected valid syntax but got parse error: %v (also tried as update: %v)", err, err2)
		}
	}
}

func runSPARQL12NegativeSyntaxTest(t *testing.T, entry w3c.TestEntry) {
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

	input := string(queryBytes)

	_, err1 := sparql.Parse(input)
	_, err2 := sparql.ParseUpdate(input)
	if err1 == nil || err2 == nil {
		t.Errorf("expected parse error but query parsed successfully")
	}
}

// loadTrigFile parses a TriG file into default graph and named graphs.
// TriG is Turtle + GRAPH blocks. This is a simplified parser.
func loadTrigFile(t *testing.T, path string) (*rdflibgo.Graph, map[string]*rdflibgo.Graph) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read trig file: %v", err)
	}
	content := string(data)
	defaultG := rdflibgo.NewGraph()
	namedGraphs := make(map[string]*rdflibgo.Graph)
	base := "file://" + path

	// Extract prefixes and GRAPH blocks
	// Simple approach: split on GRAPH keyword, handle each block
	prefixes := ""
	remaining := content

	// First extract all PREFIX/BASE declarations
	for {
		idx := strings.Index(strings.ToUpper(remaining), "PREFIX")
		if idx < 0 {
			break
		}
		// Check it's at start or after whitespace
		if idx > 0 && remaining[idx-1] != '\n' && remaining[idx-1] != '\r' && remaining[idx-1] != ' ' && remaining[idx-1] != '\t' {
			break
		}
		endIdx := strings.Index(remaining[idx:], ">")
		if endIdx < 0 {
			break
		}
		endIdx += idx + 1
		// Skip whitespace after >
		for endIdx < len(remaining) && (remaining[endIdx] == ' ' || remaining[endIdx] == '\t' || remaining[endIdx] == '\n' || remaining[endIdx] == '\r') {
			endIdx++
		}
		prefixes += remaining[idx:endIdx] + "\n"
		remaining = remaining[:idx] + remaining[endIdx:]
	}

	// Now parse GRAPH blocks and default triples
	for len(remaining) > 0 {
		remaining = strings.TrimSpace(remaining)
		if remaining == "" {
			break
		}

		upper := strings.ToUpper(remaining)
		if strings.HasPrefix(upper, "GRAPH") {
			remaining = strings.TrimSpace(remaining[5:])
			// Read graph name
			nameEnd := strings.IndexByte(remaining, '{')
			if nameEnd < 0 {
				break
			}
			graphName := strings.TrimSpace(remaining[:nameEnd])
			remaining = remaining[nameEnd+1:]

			// Find matching }
			depth := 1
			blockEnd := 0
			for i := 0; i < len(remaining); i++ {
				if remaining[i] == '{' {
					depth++
				} else if remaining[i] == '}' {
					depth--
					if depth == 0 {
						blockEnd = i
						break
					}
				}
			}
			block := remaining[:blockEnd]
			remaining = remaining[blockEnd+1:]

			// Resolve graph name using Turtle parser to handle prefixed names
			resolvedName := graphName
			if strings.HasPrefix(graphName, "<") && strings.HasSuffix(graphName, ">") {
				resolvedName = graphName[1 : len(graphName)-1]
			} else {
				// Prefixed name or other — resolve using prefixes
				rdfType := "<http://www.w3.org/1999/02/22-rdf-syntax-ns#type>"
				turtleStr := prefixes + graphName + " " + rdfType + " " + rdfType + " ."
				tempG := rdflibgo.NewGraph()
				if err := turtle.Parse(tempG, strings.NewReader(turtleStr), turtle.WithBase(base)); err == nil {
					for tr := range tempG.Triples(nil, nil, nil) {
						resolvedName = tr.Subject.String()
						break
					}
				}
			}

			// Parse block as turtle
			ng := namedGraphs[resolvedName]
			if ng == nil {
				ng = rdflibgo.NewGraph()
				namedGraphs[resolvedName] = ng
			}
			// Ensure block ends with . (TriG allows omitting it inside GRAPH)
			trimBlock := strings.TrimSpace(block)
			if trimBlock != "" && !strings.HasSuffix(trimBlock, ".") && !strings.HasSuffix(trimBlock, "}") {
				block = trimBlock + " ."
			}
			turtleStr := prefixes + block
			if err := turtle.Parse(ng, strings.NewReader(turtleStr), turtle.WithBase(base)); err != nil {
				t.Fatalf("failed to parse GRAPH block in trig: %v", err)
			}
		} else {
			// Default graph triples — find next GRAPH or end
			nextGraph := strings.Index(strings.ToUpper(remaining), "\nGRAPH ")
			if nextGraph < 0 {
				nextGraph = len(remaining)
			}
			block := remaining[:nextGraph]
			remaining = remaining[nextGraph:]

			if strings.TrimSpace(block) != "" {
				turtleStr := prefixes + block
				if err := turtle.Parse(defaultG, strings.NewReader(turtleStr), turtle.WithBase(base)); err != nil {
					t.Fatalf("failed to parse default graph in trig: %v", err)
				}
			}
		}
	}

	return defaultG, namedGraphs
}

func graphsIsomorphic(t *testing.T, a, b *rdflibgo.Graph) bool {
	t.Helper()
	// Simple comparison: same number of triples and each triple in a is in b
	aTriples := collectTriples(a)
	bTriples := collectTriples(b)
	if len(aTriples) != len(bTriples) {
		t.Logf("triple count mismatch: got %d, expected %d", len(aTriples), len(bTriples))
		return false
	}
	// Use N3 keys for comparison (with bnode normalization)
	aKeys := make(map[string]int)
	for _, tr := range aTriples {
		aKeys[tripleN3(tr)]++
	}
	bKeys := make(map[string]int)
	for _, tr := range bTriples {
		bKeys[tripleN3(tr)]++
	}
	// Try exact match first
	for k, v := range aKeys {
		if bKeys[k] != v {
			// Fall back to bnode-normalized comparison
			return graphsIsomorphicBnodes(aTriples, bTriples)
		}
	}
	return len(aKeys) == len(bKeys)
}

func graphsIsomorphicBnodes(a, b []rdflibgo.Triple) bool {
	normalize := func(s string) string {
		if strings.HasPrefix(s, "_:") {
			return "_:BNODE"
		}
		return s
	}
	aKeys := make(map[string]int)
	for _, tr := range a {
		k := normalize(tr.Subject.N3()) + " " + normalize(tr.Predicate.N3()) + " " + normalize(tr.Object.N3())
		aKeys[k]++
	}
	bKeys := make(map[string]int)
	for _, tr := range b {
		k := normalize(tr.Subject.N3()) + " " + normalize(tr.Predicate.N3()) + " " + normalize(tr.Object.N3())
		bKeys[k]++
	}
	if len(aKeys) != len(bKeys) {
		return false
	}
	for k, v := range aKeys {
		if bKeys[k] != v {
			return false
		}
	}
	return true
}

func collectTriples(g *rdflibgo.Graph) []rdflibgo.Triple {
	var result []rdflibgo.Triple
	for t := range g.Triples(nil, nil, nil) {
		result = append(result, t)
	}
	return result
}

func tripleN3(t rdflibgo.Triple) string {
	return t.Subject.N3() + " " + t.Predicate.N3() + " " + t.Object.N3()
}
