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

// §18.2.1 / §16.1.1: SELECT * projects every in-scope variable, bound or not.
func TestSelectStar_InScopeVariables(t *testing.T) {
	data := `<urn:s> <urn:p> 1 .`
	for _, c := range []struct {
		q    string
		want []string
	}{
		{`SELECT * WHERE { ?s <urn:p> ?o OPTIONAL { ?s <urn:q> ?z } }`, []string{"o", "s", "z"}},
		{`SELECT * WHERE { ?s <urn:nothing> ?o }`, []string{"o", "s"}},
		{`SELECT * WHERE { ?_x <urn:p> ?o }`, []string{"_x", "o"}},
		{`SELECT * WHERE { ?s <urn:p> [ <urn:q> ?deep ] }`, []string{"deep", "s"}},
		{`SELECT * WHERE { ?s <urn:p> ?o MINUS { ?s <urn:q> ?m } }`, []string{"o", "s"}},
		{`SELECT * WHERE { { ?a <urn:p> ?o } UNION { ?b <urn:p> ?o } }`, []string{"a", "b", "o"}},
		{`SELECT * WHERE { ?s <urn:p> ?o BIND(?o + 1 AS ?next) VALUES ?v { 1 } }`, []string{"next", "o", "s", "v"}},
		{`SELECT * WHERE { GRAPH ?g { ?s <urn:p> ?o } }`, []string{"g", "o", "s"}},
		{`SELECT * WHERE { { SELECT ?s (1 AS ?one) WHERE { ?s <urn:p> ?hidden } } }`, []string{"one", "s"}},
		{`SELECT * WHERE { ?s <urn:p> ?o FILTER NOT EXISTS { ?s <urn:q> ?inner } }`, []string{"o", "s"}},
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
