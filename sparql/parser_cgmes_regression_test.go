package sparql

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// These are standard SPARQL forms encountered in SHACL constraint queries.
func TestParseNumericTripleTerminatorAndHavingSubquery(t *testing.T) {
	for name, query := range map[string]string{
		"integer followed by dot":        `SELECT * WHERE { ?s <urn:p> 1. ?s <urn:q> 2. }`,
		"signed integer followed by dot": `SELECT * WHERE { ?s <urn:p> -1. ?s <urn:q> +2. }`,
		"decimal and double":             `SELECT * WHERE { ?s <urn:p> 1.2. ?s <urn:q> 1.e3. }`,
		"dot followed by prefixed name":  `PREFIX ex: <urn:> SELECT * WHERE { ?s ex:p 1.ex:s ex:q 2. }`,
		"subquery having":                `SELECT ?s WHERE { { SELECT ?s (COUNT(?o) AS ?n) WHERE { ?s <urn:p> ?o } GROUP BY ?s HAVING (COUNT(?o) > 1) } }`,
		"multiple having constraints":    `SELECT ?s WHERE { { SELECT ?s WHERE { ?s <urn:p> ?o } GROUP BY ?s HAVING (COUNT(?o) > 1) (COUNT(?o) < 5) } }`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse(query); err != nil {
				t.Fatalf("Parse(%q): %v", query, err)
			}
		})
	}
}

// A number followed by '.' is the number, then the end of the triple: the
// dot is part of a DECIMAL only before a digit and of a DOUBLE only before
// an exponent (SPARQL 1.1 grammar, [146]-[148]). Parsing is not enough:
// the terms must keep their datatype and lexical form.
func TestParseNumericBeforeDotKeepsItsType(t *testing.T) {
	const xsd = "http://www.w3.org/2001/XMLSchema#"
	res, err := Query(graph.NewGraph(), `CONSTRUCT {
		<urn:s> <urn:a> 1. <urn:s> <urn:b> -2. <urn:s> <urn:c> 1.5.
		<urn:s> <urn:d> 1.e3. <urn:s> <urn:e> 2.E-1. <urn:s> <urn:f> 3 .
	} WHERE {}`)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][2]string{
		"urn:a": {"1", xsd + "integer"},
		"urn:b": {"-2", xsd + "integer"},
		"urn:c": {"1.5", xsd + "decimal"},
		"urn:d": {"1.e3", xsd + "double"},
		"urn:e": {"2.E-1", xsd + "double"},
		"urn:f": {"3", xsd + "integer"},
	}
	got := 0
	for tr, err := range res.Graph.Triples(t.Context(), nil, nil, nil) {
		if err != nil {
			t.Fatal(err)
		}
		w, ok := want[tr.Predicate.Value()]
		l, isLit := tr.Object.(term.Literal)
		if !ok || !isLit {
			t.Errorf("unexpected triple %v", tr)
			continue
		}
		got++
		if l.Lexical() != w[0] || l.Datatype().Value() != w[1] {
			t.Errorf("%s: got %q^^%s, want %q^^%s", tr.Predicate.Value(), l.Lexical(), l.Datatype().Value(), w[0], w[1])
		}
	}
	if got != len(want) {
		t.Errorf("%d triples, want %d", got, len(want))
	}
}
