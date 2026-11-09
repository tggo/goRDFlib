package shacl

import (
	"errors"
	"math/big"
	"path/filepath"
	"strings"
	"testing"
)

// The SHACL 1.2 Node Expressions suite (shacl12-test-suite/tests/node-expr).
//
// The runner walks the manifest tree from node-expr/manifest.ttl, following
// every mf:include, instead of listing directories: a manifest added upstream
// is run, and a test it cannot run is a failure, never a skip. Each leaf file
// is a manifest of its own, whose mf:entries are the tests:
//
//   - sht:EvalNodeExpr evaluates sht:nodeExpr with the sht:focusNode and the
//     scope variables sht:scope-NAME of the action, against the file itself
//     as data and shapes graph, and compares the output nodes with mf:result
//     (a list, or sht:Failure for an evaluation failure);
//   - sht:Validate validates the file against itself, like the Core suite.

const nodeExprSuite = testSuite12Base + "/node-expr"

// nodeExprManifestFiles returns the leaf manifests below manifest, in order.
func nodeExprManifestFiles(t *testing.T, manifest string) []string {
	t.Helper()
	g, err := LoadTurtleFile(manifest)
	if err != nil {
		t.Fatalf("cannot load manifest %s: %v", manifest, err)
	}
	var leaves []string
	include := IRI(MF + "include")
	subs := g.All(nil, &include, nil)
	for _, inc := range subs {
		path := uriToPath(resolveURI(g.BaseURI(), inc.Object.Value()))
		if path == "" || !strings.HasSuffix(path, ".ttl") {
			t.Errorf("%s includes %s, which is not a local Turtle file", manifest, inc.Object)
			continue
		}
		child, err := LoadTurtleFile(path)
		if err != nil {
			t.Errorf("cannot load %s: %v", path, err)
			continue
		}
		if child.Has(nil, &include, nil) {
			leaves = append(leaves, nodeExprManifestFiles(t, path)...)
		} else {
			leaves = append(leaves, path)
		}
	}
	return leaves
}

func TestW3CSHACL12NodeExpr(t *testing.T) {
	files := nodeExprManifestFiles(t, filepath.Join(nodeExprSuite, "manifest.ttl"))
	if len(files) == 0 {
		t.Fatal("no node expression test files found: is the data-shapes submodule checked out?")
	}
	var total, passed int
	for _, file := range files {
		rel, _ := filepath.Rel(nodeExprSuite, file)
		g, err := LoadTurtleFile(file)
		if err != nil {
			t.Errorf("cannot load %s: %v", rel, err)
			continue
		}
		entriesHead := g.Objects(IRI(g.BaseURI()), IRI(MF+"entries"))
		if len(entriesHead) == 0 {
			t.Errorf("%s has no mf:entries", rel)
			continue
		}
		entries := g.RDFList(entriesHead[0])

		// Every test node of the file must be in the list: a test outside
		// mf:entries would never run.
		typed := 0
		for _, tt := range []string{"EvalNodeExpr", "Validate"} {
			ty := IRI(SHT + tt)
			typed += len(g.Subjects(IRI(RDFType), ty))
		}
		if typed != len(entries) {
			t.Errorf("%s declares %d tests but lists %d in mf:entries", rel, typed, len(entries))
		}

		for _, entry := range entries {
			total++
			name := rel + "/" + labelOf(g, entry)
			ok := t.Run(name, func(t *testing.T) {
				switch {
				case g.Has(&entry, &Term{kind: TermIRI, value: RDFType}, &Term{kind: TermIRI, value: SHT + "EvalNodeExpr"}):
					runEvalNodeExprTest(t, g, entry)
				case g.Has(&entry, &Term{kind: TermIRI, value: RDFType}, &Term{kind: TermIRI, value: SHT + "Validate"}):
					if !runSingleTest(t, file) {
						t.Fail()
					}
				default:
					t.Fatalf("%s has a test type this runner does not know", entry)
				}
			})
			if ok {
				passed++
			}
		}
	}
	t.Logf("node expression tests: %d, passed: %d", total, passed)
}

func labelOf(g *Graph, node Term) string {
	if labs := g.Objects(node, IRI(RDFS+"label")); len(labs) > 0 {
		return labs[0].Value()
	}
	return node.Value()
}

// runEvalNodeExprTest runs one sht:EvalNodeExpr test.
func runEvalNodeExprTest(t *testing.T, g *Graph, testNode Term) {
	t.Helper()
	actions := g.Objects(testNode, IRI(MF+"action"))
	if len(actions) == 0 {
		t.Fatal("no mf:action")
	}
	action := actions[0]
	exprNodes := g.Objects(action, IRI(SHT+"nodeExpr"))
	if len(exprNodes) == 0 {
		t.Fatal("no sht:nodeExpr")
	}
	var focus Term
	if fn := g.Objects(action, IRI(SHT+"focusNode")); len(fn) > 0 {
		focus = fn[0]
	}
	ignoreOrder := false
	if io := g.Objects(action, IRI(SHT+"ignoreOrder")); len(io) > 0 {
		ignoreOrder = io[0].Value() == "true"
	}
	vars := make(map[string]Term)
	for _, triple := range g.All(&action, nil, nil) {
		if name, ok := strings.CutPrefix(triple.Predicate.Value(), SHT+"scope-"); ok {
			vars[name] = triple.Object
		}
	}
	results := g.Objects(testNode, IRI(MF+"result"))
	if len(results) == 0 {
		t.Fatal("no mf:result")
	}
	wantFailure := results[0].IsIRI() && results[0].Value() == SHT+"Failure"

	expr := parseNodeExpr(g, exprNodes[0])
	ctx := &nodeExprContext{
		dataGraph: g,
		shapesMap: parseShapes(g),
		focusNode: focus,
		vars:      vars,
	}
	actual, err := evalNodeExpr(ctx, expr)

	if wantFailure {
		if err == nil {
			t.Fatalf("expected an evaluation failure, got %v", termsToStrings(actual))
		}
		if !errors.Is(err, ErrNodeExprFailure) {
			t.Fatalf("the failure is %v, which does not wrap ErrNodeExprFailure", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("unexpected evaluation failure: %v", err)
	}
	expected := g.RDFList(results[0])
	want, got := canonicalDecimals(expected), canonicalDecimals(actual)
	if ignoreOrder {
		if !termSetsEqual(want, got) {
			t.Errorf("result mismatch (ignoring order)\nexpected: %v\nactual:   %v", termsToStrings(expected), termsToStrings(actual))
		}
	} else if !termListsEqual(want, got) {
		t.Errorf("result mismatch\nexpected: %v\nactual:   %v", termsToStrings(expected), termsToStrings(actual))
	}
}

// canonicalDecimals rewrites every xsd:decimal in terms to one lexical form,
// so that "42.0" and "42" (the result of sparql:divide(84, 2)) compare equal.
// The suite's expected values spell decimals with a trailing ".0" (and "00"
// for sparql:seconds), the spelling of the implementation they were written
// against; SPARQL defines the value of these functions, not its lexical form,
// and the W3C SPARQL result comparison matches numbers by value for the same
// reason. Every other term, integers and doubles included, is compared
// exactly.
func canonicalDecimals(terms []Term) []Term {
	out := make([]Term, len(terms))
	for i, t := range terms {
		out[i] = t
		if t.IsLiteral() && t.Datatype() == XSD+"decimal" {
			if r, ok := new(big.Rat).SetString(t.Value()); ok {
				out[i] = Literal(r.FloatString(18), XSD+"decimal", "")
			}
		}
	}
	return out
}
