package sparql

import (
	"slices"
	"testing"
)

func starVars(t *testing.T, data, q string) []string {
	t.Helper()
	g := updateTestDataset(t, data).Default
	res, err := Query(g, q)
	if err != nil {
		t.Fatal(err)
	}
	return res.Vars
}

// §18.2.1 / §16.1.1: SELECT * projects every in-scope variable, bound or not,
// in the order the variables first appear in the query (what the W3C expected
// results use). A blank node property list is expanded into triples ahead of
// the triple that contains it, so ?deep comes before ?s there.
func TestSelectStar_InScopeVariables(t *testing.T) {
	data := `<urn:s> <urn:p> 1 .`
	for _, c := range []struct {
		q    string
		want []string
	}{
		{`SELECT * WHERE { ?s <urn:p> ?o OPTIONAL { ?s <urn:q> ?z } }`, []string{"s", "o", "z"}},
		{`SELECT * WHERE { ?s <urn:nothing> ?o }`, []string{"s", "o"}},
		{`SELECT * WHERE { ?_x <urn:p> ?o }`, []string{"_x", "o"}},
		{`SELECT * WHERE { ?s <urn:p> [ <urn:q> ?deep ] }`, []string{"deep", "s"}},
		{`SELECT * WHERE { ?s <urn:p> ?o MINUS { ?s <urn:q> ?m } }`, []string{"s", "o"}},
		{`SELECT * WHERE { { ?a <urn:p> ?o } UNION { ?b <urn:p> ?o } }`, []string{"a", "o", "b"}},
		{`SELECT * WHERE { ?s <urn:p> ?o BIND(?o + 1 AS ?next) VALUES ?v { 1 } }`, []string{"s", "o", "next", "v"}},
		{`SELECT * WHERE { GRAPH ?g { ?s <urn:p> ?o } }`, []string{"g", "s", "o"}},
		{`SELECT * WHERE { { SELECT ?s (1 AS ?one) WHERE { ?s <urn:p> ?hidden } } }`, []string{"s", "one"}},
		{`SELECT * WHERE { ?s <urn:p> ?o FILTER NOT EXISTS { ?s <urn:q> ?inner } }`, []string{"s", "o"}},
	} {
		if got := starVars(t, data, c.q); !slices.Equal(got, c.want) {
			t.Errorf("%s\n  Vars = %v, want %v", c.q, got, c.want)
		}
	}
}

// A user variable starting with '_' is returned with its values, and parser
// variables for [] never are.
func TestSelectStar_UnderscoreVariableBindings(t *testing.T) {
	g := updateTestDataset(t, `<urn:s> <urn:p> [ <urn:q> 1 ] .`).Default
	res, err := Query(g, `SELECT * WHERE { ?_x <urn:p> [ <urn:q> ?v ] }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 1 {
		t.Fatalf("got %d rows, want 1", len(res.Bindings))
	}
	row := res.Bindings[0]
	if x, ok := row["_x"]; !ok || x.N3() != "<urn:s>" {
		t.Errorf("?_x = %v, want <urn:s>", x)
	}
	if len(row) != 2 {
		t.Errorf("row has %d bindings (%v), want ?_x and ?v only", len(row), row)
	}
}
