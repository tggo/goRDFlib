// Command w3cstat reads `go test -json` output from the W3C conformance tests
// and prints one row per suite. It counts leaf subtests only: a parent test,
// or an intermediate group such as a SHACL manifest section, passes whenever
// its children do and is not a test case of its own.
//
// Usage: go test ./... -run TestW3C -json | go run ./internal/w3cstat
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type event struct {
	Action  string
	Package string
	Test    string
}

// suites maps a top-level test function to the suite row it belongs to.
// Several functions can feed one row; the row order is the slice order.
var suites = []struct {
	name  string
	tests []string
}{
	{"SPARQL 1.1 Query", []string{"sparql.TestW3C"}},
	{"SPARQL 1.0 Dataset (FROM / FROM NAMED)", []string{"sparql.TestW3CDataset"}},
	{"SPARQL 1.1 Update", []string{"sparql.TestW3CUpdate"}},
	{"SPARQL 1.2", []string{"sparql.TestW3CSPARQL12"}},
	{"SPARQL 1.1 Protocol", []string{"endpoint.TestW3CProtocol"}},
	{"SPARQL 1.1 Graph Store Protocol", []string{"endpoint.TestW3CGraphStore"}},
	{"SPARQL 1.1 Results CSV/TSV", []string{"results.TestW3CCSVTSV", "results.TestW3CResultFilesRoundTrip"}},
	{"Turtle 1.1", []string{"turtle.TestW3CTurtle"}},
	{"Turtle 1.2", []string{"turtle.TestW3CTurtle12Syntax", "turtle.TestW3CTurtle12Eval"}},
	{"TriG 1.1", []string{"trig.TestW3CTrig"}},
	{"TriG 1.2", []string{"trig.TestW3CTrigRDF12"}},
	{"N-Triples 1.1", []string{"nt.TestW3CNTriples"}},
	{"N-Triples 1.2", []string{"nt.TestW3CNTriples12"}},
	{"N-Quads 1.1", []string{"nq.TestW3CNQuads"}},
	{"N-Quads 1.2", []string{"nq.TestW3CNQuads12"}},
	{"RDF/XML 1.1", []string{"rdfxml.TestW3CRDFXML"}},
	{"RDF/XML 1.2", []string{"rdfxml.TestW3CRDFXML12"}},
	{"SHACL Core", []string{"shacl.TestW3CCoreTests"}},
	{"SHACL-SPARQL", []string{"shacl.TestW3CSPARQLTests"}},
	{"SHACL 1.2 Core", []string{"shacl.TestW3CSHACL12CoreTests"}},
	{"SHACL 1.2 SPARQL", []string{"shacl.TestW3CSHACL12SPARQLTests"}},
	{"SHACL 1.2 Node Expressions", []string{"shacl.TestW3CSHACL12NodeExprTests", "shacl.TestW3CSHACL12NodeExprConstraintTests"}},
	{"SHACL 1.2 Rules (SRL)", []string{"shacl.TestW3CSRLSyntaxTests", "shacl.TestW3CSRLWellformedTests", "shacl.TestW3CSRLStratificationTests", "shacl.TestW3CSRLEvalTests"}},
	{"RDF 1.1 Semantics (RDFS entailment)", []string{"reasoning.TestW3C_RDFS_Entailment"}},
}

type counts struct{ pass, fail, skip int }

func main() {
	result := map[string]string{} // "pkg.Test/sub" -> last action
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var e event
		if json.Unmarshal(sc.Bytes(), &e) != nil || e.Test == "" {
			continue
		}
		switch e.Action {
		case "pass", "fail", "skip":
			pkg := e.Package[strings.LastIndex(e.Package, "/")+1:]
			result[pkg+"."+e.Test] = e.Action
		}
	}

	parents := make(map[string]bool, len(result))
	for name := range result {
		for i := strings.LastIndex(name, "/"); i > 0; i = strings.LastIndex(name[:i], "/") {
			parents[name[:i]] = true
		}
	}
	byTop := map[string]*counts{}
	for name, action := range result {
		if parents[name] {
			continue
		}
		top, _, _ := strings.Cut(name, "/")
		c := byTop[top]
		if c == nil {
			c = &counts{}
			byTop[top] = c
		}
		switch action {
		case "pass":
			c.pass++
		case "fail":
			c.fail++
		case "skip":
			c.skip++
		}
	}

	var total counts
	failed := false
	fmt.Printf("  %-40s %6s %6s %6s\n", "Suite", "pass", "fail", "skip")
	for _, s := range suites {
		var c counts
		for _, t := range s.tests {
			if x := byTop[t]; x != nil {
				c.pass += x.pass
				c.fail += x.fail
				c.skip += x.skip
			}
		}
		if c == (counts{}) {
			fmt.Printf("  %-40s %6s\n", s.name, "not run")
			continue
		}
		fmt.Printf("  %-40s %6d %6d %6d\n", s.name, c.pass, c.fail, c.skip)
		total.pass += c.pass
		total.fail += c.fail
		total.skip += c.skip
		failed = failed || c.fail > 0
	}
	fmt.Println("  " + strings.Repeat("-", 62))
	fmt.Printf("  %-40s %6d %6d %6d\n", "TOTAL", total.pass, total.fail, total.skip)
	if failed {
		os.Exit(1)
	}
}
