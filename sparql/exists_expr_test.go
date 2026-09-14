package sparql

import (
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

const existsData = `<urn:a> <urn:b> <urn:c> . <urn:x> <urn:b> <urn:y> .`

// rdflib #2124: EXISTS is a built-in call (§17.4.1.4) and works in every
// expression position, not only in FILTER and BIND.
func TestExists_InSelectExpression(t *testing.T) {
	got := selectRow(t, existsData, `SELECT
		(EXISTS { <urn:a> <urn:b> <urn:c> } AS ?yes)
		(NOT EXISTS { <urn:a> <urn:b> <urn:c> } AS ?no)
		(EXISTS { <http://example.com/a> <http://example.com/b> <http://example.com/c> } AS ?missing) {}`)
	checkRow(t, got, map[string]string{"yes": "true", "no": "false", "missing": "false"})
}

func TestExists_InFunctionArgument(t *testing.T) {
	got := selectRow(t, existsData, `SELECT
		(IF(EXISTS { <urn:a> <urn:b> ?o }, "has", "none") AS ?if)
		(COALESCE(EXISTS { <urn:nope> ?p ?o }, "x") AS ?coalesce) {}`)
	checkRow(t, got, map[string]string{"if": `"has"`, "coalesce": "false"})
}

func TestExists_InHaving(t *testing.T) {
	q := `SELECT ?s WHERE { ?s <urn:b> ?o } GROUP BY ?s HAVING (EXISTS { ?s <urn:b> <urn:c> })`
	if n := countRows(t, existsData, q); n != 1 {
		t.Errorf("HAVING(EXISTS): %d rows, want 1", n)
	}
}

func TestExists_InOrderBy(t *testing.T) {
	g := updateTestDataset(t, existsData).Default
	// Both directions, so an ORDER BY that ignores the key cannot pass by
	// keeping the input order.
	for dir, first := range map[string]string{"ASC": "urn:x", "DESC": "urn:a"} {
		res, err := Query(g, `SELECT ?s WHERE { ?s <urn:b> ?o } ORDER BY `+dir+`(EXISTS { ?s <urn:b> <urn:c> })`)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Bindings) != 2 || res.Bindings[0]["s"].N3() != "<"+first+">" {
			t.Errorf("SELECT ORDER BY %s(EXISTS): got %v, want <%s> first", dir, res.Bindings, first)
		}
		res, err = Query(g, `CONSTRUCT { ?s <urn:seen> 1 } WHERE { ?s <urn:b> ?o } ORDER BY `+dir+`(EXISTS { ?s <urn:b> <urn:c> }) LIMIT 1`)
		if err != nil {
			t.Fatal(err)
		}
		if !res.Graph.Contains(rdflibgo.NewURIRefUnsafe(first), rdflibgo.NewURIRefUnsafe("urn:seen"), rdflibgo.NewLiteral(1)) {
			t.Errorf("CONSTRUCT ORDER BY %s(EXISTS) LIMIT 1 did not keep <%s>", dir, first)
		}
	}
}

func TestExists_InGroupByAndAggregate(t *testing.T) {
	got := selectRow(t, existsData, `SELECT (SUM(IF(EXISTS { ?s <urn:b> <urn:c> }, 1, 0)) AS ?n)
		WHERE { ?s <urn:b> ?o }`)
	checkRow(t, got, map[string]string{"n": "1"})
}
