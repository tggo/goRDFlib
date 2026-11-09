package sparql_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/sparql"
	"github.com/tggo/goRDFlib/testdata/w3c"
)

const sparql11FedManifestPath = "../testdata/w3c/rdf-tests/sparql/sparql11/manifest-sparql11-fed.ttl"

// TestW3CFederated runs the SPARQL 1.1 Federated Query suites: syntax-fed (3)
// and service (7). The engine makes no network call, so the evaluation tests
// run the way other implementations run them: a ServiceHandler serves each
// endpoint of the manifest's qt:serviceData from the data file the manifest
// names. A test without qt:serviceData (service07) installs no handler and
// relies on the default, where SERVICE SILENT yields one empty solution.
func TestW3CFederated(t *testing.T) {
	if _, err := os.Stat(sparql11FedManifestPath); err != nil {
		t.Skip("W3C SPARQL test suite not found (rdf-tests submodule)")
	}
	manifest, err := w3c.ParseIncludeManifest(sparql11FedManifestPath)
	if err != nil {
		t.Fatalf("failed to parse manifest: %v", err)
	}
	const (
		mfQueryEval   = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#QueryEvaluationTest"
		mfPosSyntax11 = "http://www.w3.org/2001/sw/DataAccess/tests/test-manifest#PositiveSyntaxTest11"
	)
	var eval, syntax int
	for _, entry := range manifest.Entries {
		switch entry.Type {
		case mfQueryEval:
			eval++
			t.Run("eval/"+entry.Name, func(t *testing.T) { runQueryEvalTest(t, entry) })
		case mfPosSyntax11:
			syntax++
			t.Run("syntax+/"+entry.Name, func(t *testing.T) { runPositiveSyntaxTest(t, entry) })
		default:
			t.Errorf("manifest entry %q has a test type the runner does not dispatch: %s", entry.Name, entry.Type)
		}
	}
	if eval != 7 || syntax != 3 {
		t.Fatalf("ran %d evaluation and %d syntax tests, want 7 and 3", eval, syntax)
	}
}

// manifestServices is the ServiceHandler for one test: the endpoints of its
// qt:serviceData, each answering from its data file. An endpoint the manifest
// does not list is an error, as an unreachable one would be.
func manifestServices(t *testing.T, entry w3c.TestEntry) sparql.ServiceHandler {
	t.Helper()
	endpoints := make(map[string]*rdflibgo.Graph, len(entry.ServiceData))
	for _, sd := range entry.ServiceData {
		g := rdflibgo.NewGraph()
		loadDataFile(t, g, sd.Data)
		endpoints[sd.Endpoint] = g
	}
	return func(ctx context.Context, call *sparql.ServiceCall) ([]map[string]rdflibgo.Term, error) {
		g, ok := endpoints[call.Endpoint.Value()]
		if !ok {
			return nil, fmt.Errorf("no such endpoint %s", call.Endpoint)
		}
		// The same ctx: a SERVICE inside the pattern reaches this handler too.
		res, err := sparql.EvalQueryContext(ctx, g, call.Select(), nil)
		if err != nil {
			return nil, err
		}
		return res.Bindings, nil
	}
}
