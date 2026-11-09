package sparql

import (
	"errors"
	"testing"
)

func TestPreBindRestrictions(t *testing.T) {
	cases := []struct {
		name  string
		query string
		vars  []string
		bad   bool
	}{
		{"plain", `SELECT ?this WHERE { ?this ?p ?o }`, []string{"this"}, false},
		{"minus", `SELECT ?this WHERE { ?this ?p ?o MINUS { ?this ?p 1 } }`, []string{"this"}, true},
		{"minus nested in optional", `SELECT ?this WHERE { OPTIONAL { ?this ?p ?o MINUS { ?o ?p 1 } } }`, []string{"this"}, true},
		{"minus in exists", `SELECT ?this WHERE { FILTER EXISTS { ?this ?p ?o MINUS { ?o ?p 1 } } }`, []string{"this"}, true},
		{"minus in subquery", `SELECT ?this WHERE { { SELECT ?this WHERE { ?this ?p ?o MINUS { ?o ?p 1 } } } }`, []string{"this"}, true},
		{"values of other variable", `SELECT ?this WHERE { VALUES ?x { 1 } ?this ?p ?x }`, []string{"this"}, false},
		{"values of pre-bound variable", `SELECT ?this WHERE { VALUES ?this { <http://e/a> } }`, []string{"this"}, true},
		{"bind other", `SELECT ?this WHERE { ?this ?p ?o BIND (?o AS ?x) }`, []string{"this"}, false},
		{"bind pre-bound", `SELECT ?this WHERE { BIND (true AS ?this) }`, []string{"this"}, true},
		{"project expression pre-bound", `SELECT (1 AS ?this) WHERE { }`, []string{"this"}, true},
		{"project expression other", `SELECT ?this (1 AS ?x) WHERE { ?this ?p ?o }`, []string{"this"}, false},
		{"minus without pre-bound variables", `SELECT * WHERE { ?a ?p ?o MINUS { ?o ?p 1 } }`, nil, false},
	}
	for _, c := range cases {
		q, err := Parse(c.query)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		err = q.PreBindRestrictions(c.vars...)
		if c.bad != (err != nil) {
			t.Errorf("%s: PreBindRestrictions = %v, want violation=%v", c.name, err, c.bad)
		}
		if err != nil && !errors.Is(err, ErrPreBindRestriction) {
			t.Errorf("%s: error %v does not wrap ErrPreBindRestriction", c.name, err)
		}
	}
}
