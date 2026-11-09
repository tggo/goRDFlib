package sparql

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Parser defects the DAWG SPARQL 1.0 syntax suites found (TestW3CSPARQL10).
// Each accepted query below failed to parse before, each rejected one parsed.
func TestParserDAWG10Syntax(t *testing.T) {
	const pre = "PREFIX : <http://example.org/ns#>\nPREFIX a: <http://example.org/ns2#>\n"
	accept := map[string]string{
		"dot after OPTIONAL":              `SELECT * { :p :q :r . OPTIONAL { :a :b :c } . }`,
		"dot after OPTIONAL, then triple": `SELECT * { OPTIONAL { :a :b :c } . ?x ?y ?z }`,
		"dot after MINUS":                 `SELECT * { ?x ?y ?z MINUS { :a :b :c } . ?u ?v ?w }`,
		"dot after a group":               `SELECT * { { ?x ?y ?z } . ?u ?v ?w }`,
		"dot after UNION":                 `SELECT * { { ?x ?y ?z } UNION { ?u ?v ?w } . }`,
		"dot after BIND":                  `SELECT * { ?x ?y ?z BIND (1 AS ?o) . ?u ?v ?w }`,
		"dot after GRAPH":                 `SELECT * { GRAPH ?g { ?x ?y ?z } . ?u ?v ?w }`,
		"trailing ; before OPTIONAL":      `SELECT * { :p :q :r ; OPTIONAL { :a :b :c } }`,
		"trailing ; before FILTER":        `SELECT * WHERE { :s :p :o ; FILTER(?x) }`,
		"repeated ;":                      `SELECT * { :s :p :o ;; :q :r }`,
		"triples after FILTER, no dot":    `SELECT * { ?s ?p ?o FILTER (?o) ?a ?b ?c }`,
		"prefix called a":                 `SELECT * { : a: :a . : : : . }`,
		"a as type, a: as prefix":         `SELECT * { ?s a a:T . ?s a:p ?o }`,
		"prefix beginning with a":         `PREFIX ab-c: <urn:x#> SELECT * { ?s ab-c:p ?o }`,
		"unary plus":                      `SELECT * { ?s ?p ?o FILTER (+?o > 1) }`,
		"unary plus literals":             `ASK { FILTER((+1 + -1 + +2.0) = 2) }`,
		"DESCRIBE IRI":                    `DESCRIBE <u>`,
		"DESCRIBE with WHERE":             `DESCRIBE <u> ?u WHERE { <x> <q> ?u . }`,
		"DESCRIBE *":                      `DESCRIBE * WHERE { ?s ?p ?o }`,
		"DESCRIBE var, modifiers":         `DESCRIBE ?s { ?s ?p ?o } LIMIT 3`,
		"DESCRIBE pname, no pattern":      `DESCRIBE :x ORDER BY ?x`,
		"non-ASCII prefix and local":      `PREFIX 食: <http://example.org/food#> SELECT * { 食:x 食:p 食:ほげ }`,
		"IRI-looking text with a space":   `SELECT * { FILTER (?x<?a && ?b>?y) }`,
	}
	for name, q := range accept {
		t.Run("accept/"+name, func(t *testing.T) {
			if _, err := Parse(pre + q); err != nil {
				t.Errorf("rejected: %v", err)
			}
		})
	}
	reject := map[string]string{
		"two triples without a dot":            `SELECT * { :s1 :p1 :o1 :s2 :p2 :o2 . }`,
		"two triples without a dot, variables": `SELECT * WHERE { ?x :q ?x ?z :p ?x }`,
		"IRIREF read greedily":                 `SELECT * { FILTER (?x<?a&&?b>?y) }`,
		"[] as predicate":                      `SELECT * { :x [] :q }`,
		"blank node as predicate":              `SELECT * { :x _:a :q }`,
		"literal as predicate":                 `SELECT * { :x "p" :q }`,
		"number as predicate":                  `SELECT * { :x 1 :q }`,
		"GRAPH []":                             `SELECT * { GRAPH [] { } }`,
		"GRAPH blank label":                    `SELECT * { GRAPH _:g { } }`,
		"GRAPH literal":                        `SELECT * { GRAPH "g" { } }`,
		"DESCRIBE with nothing":                `DESCRIBE`,
		"DESCRIBE a literal":                   `DESCRIBE "x"`,
		"two dots after OPTIONAL":              `SELECT * { OPTIONAL { :a :b :c } . . }`,
		"non-ASCII control as name":            "PREFIX ×: <urn:x#> SELECT * { ?s ?p ?o }",
	}
	for name, q := range reject {
		t.Run("reject/"+name, func(t *testing.T) {
			if _, err := Parse(pre + q); err == nil {
				t.Errorf("accepted")
			}
		})
	}
}

func TestParseDescribe(t *testing.T) {
	q, err := Parse(`DESCRIBE <u> ?v $w FROM <g> WHERE { ?v ?p ?w } LIMIT 2`)
	if err != nil {
		t.Fatal(err)
	}
	if q.Type != "DESCRIBE" || strings.Join(q.Describe, " ") != "<u> ?v ?w" || q.Limit != 2 || len(q.DatasetClause) != 1 {
		t.Errorf("parsed as %+v", q)
	}
	// Parsing DESCRIBE is not evaluating it.
	if _, err := Query(makeSPARQLGraph(t), `DESCRIBE <http://example.org/x>`); err == nil {
		t.Error("DESCRIBE evaluated")
	}
}

func TestUnaryPlus(t *testing.T) {
	for expr, want := range map[string]string{
		`+1`:         `1`,
		`+1.5`:       `1.5`,
		`+ -2`:       `-2`,
		`-(+3)`:      `-3`,
		`1 + +2`:     `3`,
		`+"x"`:       ``,
		`+(1 = 1)`:   ``,
		`2 * +3 - 1`: `5`,
	} {
		res, err := Query(makeSPARQLGraph(t), `SELECT ((`+expr+`) AS ?r) WHERE {}`)
		if err != nil {
			t.Errorf("%s: %v", expr, err)
			continue
		}
		got := ""
		if len(res.Bindings) == 1 && res.Bindings[0]["r"] != nil {
			got = res.Bindings[0]["r"].N3()
		}
		if got != want {
			t.Errorf("%s = %q, want %q", expr, got, want)
		}
	}
}

// Every query both reference engines reject must be a parse error for us too.
// disagreements.json holds the queries Oxigraph and rdflib disagree with this
// engine on; the ones where both error are syntax we used to accept
// (family "decorated": two triples with no '.' between them).
func TestParserRejectsWhatBothReferencesReject(t *testing.T) {
	raw, err := os.ReadFile("../testdata/sparql-diff/disagreements.json")
	if err != nil {
		t.Skip("no disagreements file")
	}
	var entries []struct {
		Query    string
		Oxigraph struct{ Error string }
		Rdflib   struct{ Error string }
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, e := range entries {
		if e.Oxigraph.Error == "" || e.Rdflib.Error == "" {
			continue
		}
		n++
		if _, err := Parse(e.Query); err == nil {
			t.Errorf("both references reject, we accept:\n%s", e.Query)
		}
	}
	t.Logf("%d queries both references reject", n)
}
