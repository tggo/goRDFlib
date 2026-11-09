package sparql

import (
	"strings"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
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

// A pre-bound value is a constant, so MINUS evaluates its right side with it
// and does not count it as a variable the sides share: the result must be
// what the same query with the value written into the text gives.
func TestPreBoundMinusIsSubstitution(t *testing.T) {
	g := graph.NewGraph()
	data := `@prefix ex: <http://example.org/> .
ex:root ex:part ex:p1 . ex:x ex:part ex:p2 . ex:p1 ex:part ex:p3 .`
	if err := turtle.Parse(t.Context(), g, strings.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	root := map[string]term.Term{"this": term.NewURIRefUnsafe("http://example.org/root")}
	for _, q := range []string{
		`SELECT * WHERE { MINUS { ?this ex:part ?o } }`,
		`SELECT ?c WHERE { ?s ex:part ?c . { MINUS { ?this ex:part+ ?c } } }`,
		`SELECT ?c WHERE { ?s ex:part ?c MINUS { ?this ex:part ?c } }`,
		`SELECT ?c WHERE { ?s ex:part ?c MINUS { ?this ex:part+ ?c } }`,
	} {
		pq, err := Parse("PREFIX ex: <http://example.org/>\n" + q)
		if err != nil {
			t.Fatal(err)
		}
		res, err := EvalQuery(g, pq, root)
		if err != nil {
			t.Fatal(err)
		}
		got := renderRows(res.Bindings)
		want := repeatedVarRows(t, g, strings.ReplaceAll(q, "?this", "ex:root"))
		if len(got) != len(want) {
			t.Errorf("%s\npre-bound %d rows %v, textual %d rows %v", q, len(got), got, len(want), want)
		}
	}
}

// "The same blank node label cannot be used in two different basic graph
// patterns in the same query" (§4.1.4). A FILTER does not end a BGP; every
// other non-triples element and a nested group does.
func TestBlankLabelInTwoBGPsIsAnError(t *testing.T) {
	bad := []string{
		`SELECT * { _:a ?p ?v . { _:a ?q 1 } }`,
		`SELECT * { { _:a ?p ?v . } _:a ?q 1 }`,
		`SELECT * { { _:a ?p ?v . } UNION { _:a ?q 1 } }`,
		`SELECT * { _:a ?p ?v . OPTIONAL { _:a ?q 1 } }`,
		`SELECT * { _:a ?p ?v . GRAPH ?g { ?s ?p ?v } _:a ?q 1 }`,
		`SELECT * { _:a ?p ?v . OPTIONAL { ?s ?p ?v } _:a ?q 1 }`,
		`SELECT * { _:a ?p ?v . BIND (1 AS ?x) _:a ?q 1 }`,
		`SELECT * { _:a ?p ?v . MINUS { _:a ?q 1 } }`,
		`SELECT * { _:a ?p ?v FILTER EXISTS { _:a ?q 1 } }`,
		`SELECT * { ?s ?p <<( _:a ?q 1 )>> . { _:a ?r 2 } }`,
		`SELECT * { _:a ?p ?v . { SELECT * { _:a ?q 1 } } }`,
	}
	for _, q := range bad {
		if _, err := Parse(q); err == nil {
			t.Errorf("accepted: %s", q)
		}
	}
	good := []string{
		`SELECT * { _:a ?p ?v . _:a ?q 1 }`,
		`SELECT * { _:a ?p ?v FILTER (true) _:a ?q 1 }`,
		`SELECT * { _:a ?p ?v . ?s ?p <<( _:a ?q 1 )>> }`,
		`SELECT * { { _:a ?p ?v } { _:b ?q 1 } }`,
		`CONSTRUCT { _:a ?p ?v } WHERE { _:a ?p ?v }`,
	}
	for _, q := range good {
		if _, err := Parse(q); err != nil {
			t.Errorf("rejected: %s: %v", q, err)
		}
	}
}

// A DELETE template and DELETE WHERE hold no blank node, whatever syntax
// produced it (SPARQL 1.1 Update §3.1.3).
func TestDeleteTemplateRejectsEveryBlankNode(t *testing.T) {
	for _, u := range []string{
		`DELETE { ?s ex:r ?o {| ex:a 1 |} } WHERE { ?s ex:p ?o }`,
		`DELETE { ?s ex:r [] } WHERE { ?s ex:p ?o }`,
		`DELETE { ?s ex:r ( 1 2 ) } WHERE { ?s ex:p ?o }`,
		`DELETE { << ?s ex:r ?o >> ex:a 1 } WHERE { ?s ex:p ?o }`,
		`DELETE { ?s ex:r <<( _:b ex:q 1 )>> } WHERE { ?s ex:p ?o }`,
		`DELETE WHERE { ?s ex:r [] }`,
	} {
		if _, err := ParseUpdate("PREFIX ex: <http://example.org/>\n" + u); err == nil {
			t.Errorf("accepted: %s", u)
		}
	}
	for _, u := range []string{
		`DELETE { << ?s ex:r ?o ~ ex:id >> ex:a 1 } WHERE { ?s ex:p ?o }`,
		`INSERT { ?s ex:r ?o {| ex:a 1 |} } WHERE { ?s ex:p ?o }`,
		`DELETE { ?s ex:r ?o } WHERE { ?s ex:p ?o {| ex:q ?v |} }`,
	} {
		if _, err := ParseUpdate("PREFIX ex: <http://example.org/>\n" + u); err != nil {
			t.Errorf("rejected: %s: %v", u, err)
		}
	}
}
