package shacl

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The DASH test suite is the de-facto conformance suite for SHACL-AF: the W3C
// published AF as a Note without one, and both reference implementations —
// TopQuadrant's Java API and pySHACL — are tested against these files. The
// copies under testdata/dash-af are taken from TopQuadrant/shacl (Apache-2.0),
// with two files pySHACL carries that the Java tree does not.
//
// Each file is self-contained: the same graph holds the data, the shapes, and
// the expected result, declared as one of three test-case types.
//
//	dash:InferencingTestCase       rules must infer the expected triples
//	dash:GraphValidationTestCase   validation must produce the expected report
//	dash:FunctionTestCase          a SPARQL expression must evaluate to a value
//
// A file may declare several test cases, so each is run as its own subtest.

const dashNS = "http://datashapes.org/dash#"

func TestDASH_AdvancedFeatures(t *testing.T) {
	files := dashTestFiles(t)
	if len(files) == 0 {
		t.Fatal("no DASH test files found under testdata/dash-af")
	}

	var ran int
	for _, path := range files {
		path := path
		name := strings.TrimSuffix(strings.TrimPrefix(path, "../testdata/dash-af/"), ".ttl")
		t.Run(name, func(t *testing.T) {
			g, err := LoadTurtleFile(path)
			if err != nil {
				t.Fatalf("load %s: %v", path, err)
			}
			cases := dashTestCases(g)
			if len(cases) == 0 {
				t.Skip("no dash test case in this file (it is an import target)")
			}
			for _, tc := range cases {
				ran++
				t.Run(shortLabel(tc.node), func(t *testing.T) { tc.run(t, g) })
			}
		})
	}
	t.Logf("ran %d DASH advanced-features test cases from %d files", ran, len(files))
}

func dashTestFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	err := filepath.Walk("../testdata/dash-af", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".ttl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk testdata: %v", err)
	}
	sort.Strings(files)
	return files
}

// dashTestCase is one declared test case within a file.
type dashTestCase struct {
	node Term
	kind string
}

func dashTestCases(g *Graph) []dashTestCase {
	var cases []dashTestCase
	typePred := IRI(RDFType)
	for _, kind := range []string{"InferencingTestCase", "GraphValidationTestCase", "FunctionTestCase"} {
		for _, node := range g.Subjects(typePred, IRI(dashNS+kind)) {
			cases = append(cases, dashTestCase{node: node, kind: kind})
		}
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].node.String() < cases[j].node.String() })
	return cases
}

func (tc dashTestCase) run(t *testing.T, g *Graph) {
	switch tc.kind {
	case "InferencingTestCase":
		tc.runInferencing(t, g)
	case "GraphValidationTestCase":
		tc.runGraphValidation(t, g)
	case "FunctionTestCase":
		tc.runFunction(t, g)
	}
}

// runInferencing checks that applying the rules produces the expected triples.
//
// dash:expectedResult holds one or more reified triples. The test is that each
// is present afterwards; a rule producing extra triples is not a failure, since
// the files do not claim to enumerate everything a rule may infer.
func (tc dashTestCase) runInferencing(t *testing.T, g *Graph) {
	expected := tc.expectedTriples(g)
	if len(expected) == 0 {
		t.Fatal("dash:InferencingTestCase with no expected triples")
	}

	data := NewGraph()
	data.Merge(g)
	added, err := ApplyRules(data, g)
	if err != nil {
		t.Fatalf("ApplyRules: %v", err)
	}

	for _, want := range expected {
		s, p, o := want.Subject, want.Predicate, want.Object
		if !data.Has(&s, &p, &o) {
			t.Errorf("rules did not infer %s %s %s (%d triples were added)", s, p, o, added)
		}
	}
}

func (tc dashTestCase) expectedTriples(g *Graph) []Triple {
	var out []Triple
	for _, r := range g.Objects(tc.node, IRI(dashNS+"expectedResult")) {
		s := g.Objects(r, IRI(RDF+"subject"))
		p := g.Objects(r, IRI(RDF+"predicate"))
		o := g.Objects(r, IRI(RDF+"object"))
		if len(s) == 0 || len(p) == 0 || len(o) == 0 {
			continue
		}
		out = append(out, Triple{Subject: s[0], Predicate: p[0], Object: o[0]})
	}
	return out
}

// runGraphValidation checks conformance and the reported violations.
//
// The comparison is on the identifying fields of each result — focus node,
// path, value, source shape and constraint component. sh:message is not
// compared: it is implementation-defined text.
func (tc dashTestCase) runGraphValidation(t *testing.T, g *Graph) {
	reports := g.Objects(tc.node, IRI(dashNS+"expectedResult"))
	if len(reports) == 0 {
		t.Fatal("dash:GraphValidationTestCase with no expected report")
	}
	wantConforms := true
	if c := g.Objects(reports[0], IRI(SH+"conforms")); len(c) > 0 {
		wantConforms = c[0].Value() == "true"
	}
	var wantResults []string
	for _, r := range g.Objects(reports[0], IRI(SH+"result")) {
		wantResults = append(wantResults, dashResultKey(g, r))
	}
	sort.Strings(wantResults)

	var afErrs []error
	report := Validate(g, g,
		WithAdvancedFeatures(),
		WithErrorHandler(func(err error) { afErrs = append(afErrs, err) }))

	if report.Conforms != wantConforms {
		t.Errorf("conforms = %v, want %v (%d results, af errors: %v)",
			report.Conforms, wantConforms, len(report.Results), afErrs)
	}

	gotResults := make([]string, 0, len(report.Results))
	for _, r := range report.Results {
		gotResults = append(gotResults, validationResultKey(r))
	}
	sort.Strings(gotResults)

	if len(wantResults) > 0 && !equalStrings(gotResults, wantResults) {
		t.Errorf("results mismatch\n got: %s\nwant: %s\naf errors: %v",
			strings.Join(gotResults, "\n      "), strings.Join(wantResults, "\n      "), afErrs)
	}
}

// dashResultKey renders an expected sh:ValidationResult as a comparable string.
func dashResultKey(g *Graph, r Term) string {
	get := func(pred string) string {
		if v := g.Objects(r, IRI(SH+pred)); len(v) > 0 {
			return v[0].String()
		}
		return ""
	}
	return fmt.Sprintf("focus=%s path=%s value=%s shape=%s component=%s",
		get("focusNode"), get("resultPath"), get("value"), get("sourceShape"), get("sourceConstraintComponent"))
}

func validationResultKey(r ValidationResult) string {
	str := func(t Term) string {
		if t.IsNone() {
			return ""
		}
		return t.String()
	}
	return fmt.Sprintf("focus=%s path=%s value=%s shape=%s component=%s",
		str(r.FocusNode), str(r.ResultPath), str(r.Value), str(r.SourceShape), str(r.SourceConstraintComponent))
}

// runFunction evaluates dash:expression as a SPARQL expression and compares the
// value with dash:expectedResult.
//
// This is the path that matters most for the design: the expression calls a
// sh:SPARQLFunction from inside an ordinary SPARQL query, which only works
// because the functions are bound to the query rather than registered globally.
func (tc dashTestCase) runFunction(t *testing.T, g *Graph) {
	exprs := g.Objects(tc.node, IRI(dashNS+"expression"))
	if len(exprs) == 0 {
		t.Fatal("dash:FunctionTestCase with no dash:expression")
	}
	want := g.Objects(tc.node, IRI(dashNS+"expectedResult"))
	if len(want) == 0 {
		t.Fatal("dash:FunctionTestCase with no dash:expectedResult")
	}

	af, err := newAFContext(g, g, newConfig([]Option{WithAdvancedFeatures()}))
	if err != nil {
		t.Fatalf("newAFContext: %v", err)
	}
	if af.loadErr != nil {
		t.Fatalf("loading functions: %v", af.loadErr)
	}

	query := dashPrefixes(g) + "SELECT (" + exprs[0].Value() + " AS ?result) WHERE { }"
	rows, err := executeSPARQL(g, query, nil, nil, af.functionsAtDepth(1))
	if err != nil {
		t.Fatalf("evaluating %q: %v", exprs[0].Value(), err)
	}
	if len(rows) == 0 {
		t.Fatalf("evaluating %q produced no result, want %s", exprs[0].Value(), want[0])
	}
	got, ok := rows[0]["result"]
	if !ok {
		t.Fatalf("evaluating %q left ?result unbound, want %s", exprs[0].Value(), want[0])
	}
	if !sameValue(got, want[0]) {
		t.Errorf("evaluating %q = %s, want %s", exprs[0].Value(), got, want[0])
	}
}

// sameValue compares two literals by lexical value, tolerating a difference in
// datatype. A DASH file writes an expected xsd:string as a plain literal, and
// SPARQL's CONCAT returns a plain literal too, but an implementation is free to
// type either; the value is what the test is about.
func sameValue(got, want Term) bool {
	if got.Equal(want) {
		return true
	}
	return got.Kind() == want.Kind() && got.Value() == want.Value()
}

// dashPrefixes rebuilds the prefix declarations a DASH file relies on. The
// expression strings use ex: and the standard prefixes without declaring them.
func dashPrefixes(g *Graph) string {
	var sb strings.Builder
	sb.WriteString("PREFIX sh: <" + SH + ">\n")
	sb.WriteString("PREFIX xsd: <" + XSD + ">\n")
	sb.WriteString("PREFIX rdf: <" + RDF + ">\n")
	sb.WriteString("PREFIX rdfs: <" + RDFS + ">\n")
	sb.WriteString("PREFIX dash: <" + dashNS + ">\n")
	if ns := exNamespaceOf(g); ns != "" {
		sb.WriteString("PREFIX ex: <" + ns + ">\n")
	}
	return sb.String()
}

// exNamespaceOf recovers the ex: namespace of a DASH file from any subject it
// declares, since the parsed graph no longer carries the prefix map.
func exNamespaceOf(g *Graph) string {
	typePred := IRI(RDFType)
	fnType := IRI(SHSPARQLFunction)
	for _, n := range g.Subjects(typePred, fnType) {
		if i := strings.LastIndex(n.Value(), "#"); i >= 0 {
			return n.Value()[:i+1]
		}
	}
	return ""
}

func shortLabel(t Term) string {
	s := t.Value()
	if i := strings.LastIndexAny(s, "#/"); i >= 0 && i+1 < len(s) {
		s = s[i+1:]
	}
	if s == "" {
		return "case"
	}
	return s
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
