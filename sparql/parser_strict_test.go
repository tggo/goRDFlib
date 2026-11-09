package sparql

import (
	"strings"
	"testing"
)

// The parser used to accept text that is no query: a SELECT with nothing
// selected, a variable whose first character was not ? or $ (the character
// was skipped unseen), and a bare word where a term belongs. Found by
// FuzzOptimisationsAgree with
//
//	PREFIX ex:<0000000000000000000>SELECT{VALUES!d{00000000000}A A 0 A0 ex:000*$d  }
//
// which parsed into VALUES ?d joined with {A A 0 . A0 ex:000* ?d}. Each case
// must fail, with a position, and the legal near neighbours must still parse.
func TestParserRejectsWhatIsNotSPARQL(t *testing.T) {
	for _, c := range []struct{ name, query, want string }{
		{"the fuzz input", `PREFIX ex:<0000000000000000000>SELECT{VALUES!d{00000000000}A A 0 A0 ex:000*$d  }`, ""},
		{"SELECT with nothing selected", `SELECT WHERE { ?s ?p ?o }`, "SELECT needs"},
		{"SELECT with nothing selected, braces", `SELECT { ?s ?p ?o }`, "SELECT needs"},
		{"subquery with nothing selected", `SELECT * WHERE { { SELECT WHERE { ?s ?p ?o } } }`, "SELECT needs"},
		{"VALUES variable without ? or $", `SELECT * WHERE { VALUES !d { 1 } }`, "expected a variable"},
		{"VALUES variable without ? or $, letter", `SELECT * WHERE { VALUES d { 1 } }`, "expected a variable"},
		{"bare word as subject", `SELECT * WHERE { A <urn:p> <urn:o> }`, "not a term"},
		{"bare word as object", `SELECT * WHERE { ?s <urn:p> word }`, "not a term"},
		{"bare word in a path pattern", `PREFIX ex: <urn:> SELECT * WHERE { A0 ex:p* ?d }`, "not a term"},
		{"upper case A as a predicate", `SELECT * WHERE { ?s A ?o }`, "not a term"},
		{"FILTER with a bare word", `SELECT * WHERE { ?s ?p ?o FILTER A00000 }`, "a constraint is"},
		{"FILTER with a variable", `SELECT * WHERE { ?s ?p ?o FILTER ?s }`, "a constraint is"},
		{"FILTER with a literal", `SELECT * WHERE { ?s ?p ?o FILTER 1 }`, "a constraint is"},
		{"FILTER with a negated variable", `SELECT * WHERE { ?s ?p ?o FILTER !?s }`, "a constraint is"},
		{"FILTER with a negated call", `SELECT * WHERE { ?s ?p ?o FILTER !bound(?s) }`, "write it in brackets"},
		{"FILTER with a negated EXISTS", `SELECT * WHERE { ?s ?p ?o FILTER !EXISTS { ?s ?p ?o } }`, "write it in brackets"},
		{"HAVING with a variable", `SELECT ?s WHERE { ?s ?p ?o } GROUP BY ?s HAVING ?s`, "a constraint is"},
		{"HAVING with an unbracketed comparison", `SELECT ?s WHERE { ?s ?p ?o } GROUP BY ?s HAVING COUNT(?o) > 1`, ""},
		{"HAVING with a negated call", `SELECT ?s WHERE { ?s ?p ?o } GROUP BY ?s HAVING !bound(?s)`, "a constraint is"},
		{"FILTER with an IRI and no call", `PREFIX ex: <urn:> SELECT * WHERE { ?s ?p ?o FILTER ex:f }`, "a constraint is"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := Parse(c.query)
			if err == nil {
				t.Fatalf("accepted %q", c.query)
			}
			if !strings.Contains(err.Error(), "pos ") && !strings.Contains(err.Error(), "position") && !strings.Contains(err.Error(), "at ") {
				t.Errorf("error has no position: %v", err)
			}
			if c.want != "" && !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not mention %q", err, c.want)
			}
		})
	}
	for _, q := range []string{
		`SELECT * WHERE { ?s a ?o }`,
		`SELECT ?s WHERE { ?s ?p true }`,
		`SELECT ?s WHERE { ?s ?p TRUE . ?s ?p false }`,
		`PREFIX ex: <urn:> SELECT $s WHERE { $s ex:p 1.5e3 , -2 , "x"@en , _:b }`,
		`SELECT (1 AS ?x) WHERE { VALUES ?d { 1 } }`,
		`SELECT DISTINCT ?s WHERE { VALUES (?a ?b) { (1 2) } }`,
		`SELECT ?s WHERE { ?s <urn:p> ( 1 2 ) }`,
		`SELECT ?s WHERE { ?s <urn:p> [ <urn:q> 1 ] }`,
		`PREFIX : <urn:> SELECT ?s WHERE { ?s :p :o }`,
		`SELECT * WHERE { ?s ?p ?o FILTER (?s) }`,
		`SELECT * WHERE { ?s ?p ?o FILTER bound(?s) }`,
		`SELECT * WHERE { ?s ?p ?o FILTER (!bound(?s)) }`,
		`SELECT * WHERE { ?s ?p ?o FILTER regex(?o, "a") }`,
		`SELECT * WHERE { ?s ?p ?o FILTER EXISTS { ?s ?p ?o } }`,
		`SELECT * WHERE { ?s ?p ?o FILTER NOT EXISTS { ?s ?p ?o } }`,
		`PREFIX ex: <urn:> SELECT * WHERE { ?s ?p ?o FILTER ex:f(?s) }`,
		`SELECT ?s WHERE { ?s ?p ?o } GROUP BY ?s HAVING (COUNT(?o) > 1)`,
		`SELECT ?s WHERE { ?s ?p ?o } GROUP BY ?s HAVING (COUNT(?o) > 1) (SUM(?o) < 9)`,
		`SELECT ?s WHERE { ?s ?p ?o } GROUP BY ?s HAVING BOUND(?s)`,
	} {
		if _, err := Parse(q); err != nil {
			t.Errorf("rejected %q: %v", q, err)
		}
	}
}
