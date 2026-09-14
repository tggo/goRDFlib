package sparql

import (
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// rdflib #1884: every ECHAR of grammar [160] is decoded — \t \b \n \r \f \" \' \\.
func TestStringEscapes_AllECHARs(t *testing.T) {
	res, err := Query(rdflibgo.NewGraph(), `SELECT ("a\fb" AS ?f) ("a\bb" AS ?b) ('it\'s' AS ?q) ("x\"y" AS ?dq)
		("t\tn\nr\r" AS ?tnr) ("back\\slash" AS ?bs) ('''long\'s''' AS ?long) {}`)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"f": "a\fb", "b": "a\bb", "q": "it's", "dq": `x"y`,
		"tnr": "t\tn\nr\r", "bs": `back\slash`, "long": "long's",
	}
	for v, w := range want {
		l, ok := res.Bindings[0][v].(rdflibgo.Literal)
		if !ok || l.Lexical() != w {
			t.Errorf("?%s = %v, want lexical %q", v, res.Bindings[0][v], w)
		}
	}
}

// The same escapes in a triple pattern and in INSERT DATA.
func TestStringEscapes_InPatternsAndUpdates(t *testing.T) {
	ds := updateTestDataset(t, ``)
	if err := Update(ds, `INSERT DATA { <urn:s> <urn:p> 'it\'s\f' }`); err != nil {
		t.Fatal(err)
	}
	if !ds.Default.Contains(rdflibgo.NewURIRefUnsafe("urn:s"), rdflibgo.NewURIRefUnsafe("urn:p"), rdflibgo.NewLiteral("it's\f")) {
		t.Error(`INSERT DATA 'it\'s\f' did not store "it's\f"`)
	}
	res, err := Query(ds.Default, `SELECT ?s WHERE { ?s <urn:p> "it's\f" }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Bindings) != 1 {
		t.Errorf(`pattern "it's\f": %d rows, want 1`, len(res.Bindings))
	}
}

// An escape outside ECHAR, or a malformed codepoint escape, is a syntax error
// wherever the literal appears (§19.2, grammar [160]).
func TestStringEscapes_InvalidAreSyntaxErrors(t *testing.T) {
	for _, q := range []string{
		`SELECT ("a\qb" AS ?x) {}`,
		`SELECT ("\u00" AS ?x) {}`,
		`SELECT ("\U0001HHHH" AS ?x) {}`,
		`SELECT ("\uD800" AS ?x) {}`,
		`SELECT * WHERE { FILTER(?o = "a\qb") }`,
		`SELECT * WHERE { ?s ?p "\u00" }`,
		`SELECT * WHERE { ?s ?p "x\"y\qz" }`,
		`SELECT * WHERE { VALUES ?x { "a\qb" } }`,
	} {
		if _, err := Parse(q); err == nil {
			t.Errorf("invalid escape accepted: %s", q)
		}
	}
	if _, err := ParseUpdate(`INSERT DATA { <urn:s> <urn:p> "a\qb" }`); err == nil {
		t.Error(`INSERT DATA with "a\qb" accepted`)
	}
}
