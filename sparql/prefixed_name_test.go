package sparql

import (
	"strings"
	"testing"
)

const pnData = `<urn:s> <urn:p> <http://example.org/a-b> .
<urn:s> <urn:p> <http://example.org/a.b> .
<urn:s> <urn:p> <http://example.org/a/b> .
<urn:s> <urn:p> <http://example.org/a%20b> .
<urn:s> <urn:p> "v"^^<http://example.org/my-type> .`

// rdflib #1871: SPARQL 1.1 grammar [169]-[173] allows '-', '.', PERCENT and
// PN_LOCAL_ESC inside PN_LOCAL, in expressions as in triple patterns.
func TestPrefixedName_InFilter(t *testing.T) {
	for local, iri := range map[string]string{
		`a-b`:   "a-b",
		`a.b`:   "a.b",
		`a\/b`:  "a/b",
		`a%20b`: "a%20b",
	} {
		q := `PREFIX ex: <http://example.org/> SELECT ?o WHERE { ?s <urn:p> ?o FILTER(?o = ex:` + local + `) }`
		if n := countRows(t, pnData, q); n != 1 {
			t.Errorf("FILTER(?o = ex:%s): %d rows, want 1 (<http://example.org/%s>)", local, n, iri)
		}
	}
}

func TestPrefixedName_InBind(t *testing.T) {
	got := selectRow(t, ``, `PREFIX ex: <http://example.org/> PREFIX my-ns: <urn:ns:>
SELECT ?dash ?esc ?pfx WHERE { BIND(ex:a-b AS ?dash) BIND(ex:a\/b AS ?esc) BIND(my-ns:x AS ?pfx) }`)
	checkRow(t, got, map[string]string{
		"dash": "<http://example.org/a-b>",
		"esc":  "<http://example.org/a/b>",
		"pfx":  "<urn:ns:x>",
	})
}

// The backslash of PN_LOCAL_ESC is not part of the IRI in a triple pattern.
func TestPrefixedName_EscapeInTriplePattern(t *testing.T) {
	q := `PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s <urn:p> ex:a\/b }`
	if n := countRows(t, pnData, q); n != 1 {
		t.Errorf("BGP object ex:a\\/b: %d rows, want 1", n)
	}
}

// A trailing '.' ends the triple, it is not part of the local name.
func TestPrefixedName_TrailingDot(t *testing.T) {
	q := `PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s <urn:p> ex:a.b. }`
	if n := countRows(t, pnData, q); n != 1 {
		t.Errorf("ex:a.b. : %d rows, want 1", n)
	}
}

// A prefixed datatype in a triple pattern resolves like one in an expression
// (DAWG open-eq-02).
func TestPrefixedName_DatatypeInTriplePattern(t *testing.T) {
	q := `PREFIX ex: <http://example.org/> SELECT ?s WHERE { ?s <urn:p> "v"^^ex:my-type }`
	if n := countRows(t, pnData, q); n != 1 {
		t.Errorf(`BGP object "v"^^ex:my-type: %d rows, want 1`, n)
	}
}

// Function calls through a prefixed name with PN_LOCAL characters still parse.
func TestPrefixedName_FunctionCall(t *testing.T) {
	got := selectRow(t, ``, `PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
SELECT (xsd:integer("5") + 1 AS ?n) {}`)
	checkRow(t, got, map[string]string{"n": "6"})
	if _, err := Parse(`PREFIX ex: <http://example.org/> SELECT * WHERE { ?s ?p ?o FILTER(ex:is-ok(?o)) }`); err != nil && strings.Contains(err.Error(), "parse") {
		t.Errorf("ex:is-ok(?o) rejected: %v", err)
	}
}
