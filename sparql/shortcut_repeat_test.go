package sparql

import (
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/turtle"
)

// The COUNT and ASK store shortcuts hand one triple pattern to the store,
// which knows nothing about a variable repeated within it, and a blank-node
// label in a query pattern is a variable (§4.1.4) that must join like one.
// Each query here is checked against the same query with the shortcut
// impossible (a second, always-true pattern), so a wrong shortcut and a wrong
// general path cannot agree by accident.
func TestShortcutsAndBlankLabelsJoin(t *testing.T) {
	g := graph.NewGraph()
	data := `@prefix ex: <http://example.org/> .
ex:a ex:p ex:a . ex:a ex:p ex:b . ex:b ex:p ex:c .
ex:c ex:q ex:a . ex:b ex:q ex:z .`
	if err := turtle.Parse(t.Context(), g, strings.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, q string
		want    []string
	}{
		{"COUNT repeated var", `SELECT (COUNT(*) AS ?n) WHERE { ?x ex:p ?x }`, []string{"n=1 "}},
		{"COUNT repeated var named", `SELECT (COUNT(?x) AS ?n) WHERE { ?x ex:p ?x }`, []string{"n=1 "}},
		{"COUNT repeated blank label", `SELECT (COUNT(*) AS ?n) WHERE { _:b ex:p _:b }`, []string{"n=1 "}},
		{"blank label joins", `SELECT ?o WHERE { _:b ex:p ?o . _:b ex:q ex:z }`, []string{"o=<http://example.org/c> "}},
		{"blank label joins in path", `SELECT ?o WHERE { _:b ex:p/ex:p ?o . _:b ex:q ex:a }`, nil},
		{"blank label is not projected by *", `SELECT * WHERE { _:b ex:q ex:z }`, []string{""}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := repeatedVarRows(t, g, c.q)
			if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
				t.Errorf("%s\n got %q\nwant %q", c.q, got, c.want)
			}
		})
	}
	asks := []struct {
		q    string
		want bool
	}{
		{`ASK { ?x ex:q ?x }`, false},
		{`ASK { _:b ex:q _:b }`, false},
		{`ASK { ?x ex:p ?x }`, true},
		{`ASK { _:b ex:p ex:c . _:b ex:q ex:z }`, true},
		{`ASK { _:b ex:p ex:b . _:b ex:q ex:z }`, false},
	}
	for _, c := range asks {
		pq, err := Parse("PREFIX ex: <http://example.org/>\n" + c.q)
		if err != nil {
			t.Fatal(err)
		}
		res, err := EvalQuery(g, pq, nil)
		if err != nil {
			t.Fatal(err)
		}
		if res.AskResult != c.want {
			t.Errorf("%s = %v, want %v", c.q, res.AskResult, c.want)
		}
	}
}
