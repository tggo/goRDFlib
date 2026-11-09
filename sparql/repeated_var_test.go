package sparql

import (
	"slices"
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/turtle"
)

// repeatedVarRows returns the rows of a SELECT query as sorted strings, each
// with its variables in name order.
func repeatedVarRows(t *testing.T, g *graph.Graph, q string) []string {
	t.Helper()
	pq, err := Parse("PREFIX ex: <http://example.org/>\n" + q)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	res, err := EvalQuery(g, pq, nil)
	if err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	var rows []string
	for _, b := range res.Bindings {
		keys := make([]string, 0, len(b))
		for k := range b {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		var sb strings.Builder
		for _, k := range keys {
			sb.WriteString(k + "=" + b[k].N3() + " ")
		}
		rows = append(rows, sb.String())
	}
	slices.Sort(rows)
	return rows
}

// A variable that stands in two positions of one pattern names one value
// (SPARQL 1.1 §18.2.2.1): ?x ex:p ?x matches only a triple whose subject and
// object are the same term. The BGP loop bound the variable at its first
// position and never looked at it again, so every triple of ex:p matched and
// the row carried the subject only. Found by the differential test of the
// optimisations: a plan that happened to run another pattern first hid it.
func TestRepeatedVariableInOnePattern(t *testing.T) {
	g := graph.NewGraph()
	if err := turtle.Parse(t.Context(), g, strings.NewReader(`@prefix ex: <http://example.org/> .
ex:a ex:p ex:b . ex:b ex:p ex:c . ex:c ex:p ex:c . ex:d ex:p ex:d .
ex:a ex:k ex:a . ex:a ex:k ex:b .
ex:a ex:q ex:b . ex:b ex:q ex:a . ex:c ex:q ex:d . ex:e ex:q ex:e .
ex:s ex:s ex:o . ex:t ex:t ex:t . ex:u ex:r ex:u .
`)); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		query string
		want  []string
	}{
		{`SELECT ?x WHERE { ?x ex:p ?x }`, []string{"x=<http://example.org/c> ", "x=<http://example.org/d> "}},
		{`SELECT ?x WHERE { ?x ex:p ?x . ?x ex:k ?y }`, nil},
		{`SELECT ?x WHERE { ?x ex:k ?y . ?x ex:p ?x }`, nil},
		{`SELECT ?x WHERE { ?x ex:p+ ?x }`, []string{"x=<http://example.org/c> ", "x=<http://example.org/d> "}},
		{`SELECT ?x WHERE { ?x ex:q+ ?x }`, []string{"x=<http://example.org/a> ", "x=<http://example.org/b> ", "x=<http://example.org/e> "}},
		{`SELECT ?x WHERE { ?x ex:q/ex:q ?x }`, []string{"x=<http://example.org/a> ", "x=<http://example.org/b> ", "x=<http://example.org/e> "}},
		{`SELECT ?s WHERE { ?s ?s ?o }`, []string{"s=<http://example.org/s> ", "s=<http://example.org/t> "}},
		{`SELECT ?s WHERE { ?s ?p ?s . ?s ?s ?s }`, []string{"s=<http://example.org/t> "}},
		{`SELECT ?o WHERE { ?s ?o ?o }`, []string{"o=<http://example.org/t> "}},
		{`SELECT ?x WHERE { ?x ex:p ?y . ?y ex:p ?y }`, []string{"x=<http://example.org/b> ", "x=<http://example.org/c> ", "x=<http://example.org/d> "}},
	} {
		got := repeatedVarRows(t, g, c.query)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s\ngot  %v\nwant %v", c.query, got, c.want)
		}
	}
}
