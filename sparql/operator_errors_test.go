package sparql

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/turtle"
)

// selectRow runs a query with an empty WHERE and returns the N3 of each
// projected variable in the single result row, "UNBOUND" for an unbound one.
func selectRow(t *testing.T, data, q string) map[string]string {
	t.Helper()
	g := rdflibgo.NewGraph()
	if data != "" {
		if err := turtle.Parse(g, strings.NewReader(data)); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Query(g, q)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(res.Bindings) != 1 {
		t.Fatalf("got %d rows, want 1", len(res.Bindings))
	}
	out := make(map[string]string, len(res.Vars))
	for _, v := range res.Vars {
		out[v] = "UNBOUND"
		if x, ok := res.Bindings[0][v]; ok && x != nil {
			out[v] = x.N3()
		}
	}
	return out
}

func checkRow(t *testing.T, got, want map[string]string) {
	t.Helper()
	for v, w := range want {
		if got[v] != w {
			t.Errorf("?%s = %s, want %s", v, got[v], w)
		}
	}
}

func countRows(t *testing.T, data, q string) int {
	t.Helper()
	g := rdflibgo.NewGraph()
	if err := turtle.Parse(g, strings.NewReader(data)); err != nil {
		t.Fatal(err)
	}
	res, err := Query(g, q)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	return len(res.Bindings)
}

// rdflib #737: a string never equals a number.
func TestEquality_StringIsNotNumber(t *testing.T) {
	data := `<urn:s> <urn:p> "1" .`
	if n := countRows(t, data, `SELECT ?s WHERE { ?s <urn:p> ?o FILTER(?o = 1) }`); n != 0 {
		t.Errorf(`FILTER("1" = 1) matched %d rows, want 0`, n)
	}
	if n := countRows(t, data, `SELECT ?s WHERE { ?s <urn:p> ?o FILTER(?o != 1) }`); n != 1 {
		t.Errorf(`FILTER("1" != 1) matched %d rows, want 1`, n)
	}
}

// §17.4.1.7 RDFterm-equal: two literals of unsupported datatypes that are not
// the same term are a type error.
func TestEquality_UnknownDatatypesAreTypeError(t *testing.T) {
	got := selectRow(t, ``, `SELECT
		("1"^^<urn:dt1> = "1"^^<urn:dt2> AS ?eq)
		("1"^^<urn:dt1> != "1"^^<urn:dt2> AS ?ne)
		("1"^^<urn:dt1> = "1"^^<urn:dt1> AS ?same)
		("a" = "a"@en AS ?lang)
		(<urn:x> = "urn:x" AS ?iri) {}`)
	checkRow(t, got, map[string]string{
		"eq":   "UNBOUND",
		"ne":   "UNBOUND",
		"same": "true",
		"lang": "false",
		"iri":  "false",
	})
}

// "abc"^^xsd:integer is ill-typed: arithmetic and comparison on it are type
// errors, not operations on 0.
func TestIllTypedNumeric_OperatorsRaiseErrors(t *testing.T) {
	got := selectRow(t, ``, `PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
SELECT ("abc"^^xsd:integer + 1 AS ?plus)
       ("abc"^^xsd:integer < 5 AS ?lt)
       ("abc"^^xsd:integer = 0 AS ?eq)
       (-"abc"^^xsd:integer AS ?neg)
       ("1.5"^^xsd:integer * 2 AS ?frac)
       ("inf"^^xsd:double > 0 AS ?inf)
       ("INF"^^xsd:double > 0 AS ?realInf)
       (COALESCE("abc"^^xsd:integer + 1, "OK") AS ?coal) {}`)
	checkRow(t, got, map[string]string{
		"plus":    "UNBOUND",
		"lt":      "UNBOUND",
		"eq":      "UNBOUND",
		"neg":     "UNBOUND",
		"frac":    "UNBOUND",
		"inf":     "UNBOUND",
		"realInf": "true",
		"coal":    `"OK"`,
	})
}

// §17.3: relational operators exist for numerics, strings, booleans and
// date/times only.
func TestRelational_NoOperatorMappingIsTypeError(t *testing.T) {
	got := selectRow(t, ``, `PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
SELECT (<urn:a> < <urn:b> AS ?iri)
       ("a" < 1 AS ?mixed)
       ("a"@en < "b"@en AS ?lang)
       ("a" < "b" AS ?str)
       (false < true AS ?bool)
       ("2006-08-23T09:00:00+01:00"^^xsd:dateTime > "2006-08-22"^^xsd:date AS ?dtDate)
       (10 > 9.5 AS ?num) {}`)
	checkRow(t, got, map[string]string{
		"iri":    "UNBOUND",
		"mixed":  "UNBOUND",
		"lang":   "UNBOUND",
		"str":    "true",
		"bool":   "true",
		"dtDate": "UNBOUND",
		"num":    "true",
	})
}

// §17.2: || and && mask an error only when the other operand decides.
func TestLogical_ErrorTruthTable(t *testing.T) {
	got := selectRow(t, ``, `SELECT
		(?u || true AS ?orTrue) (?u || false AS ?orFalse)
		(?u && false AS ?andFalse) (?u && true AS ?andTrue)
		(!(?u || false) AS ?notErr) (!<urn:x> AS ?notIRI) {}`)
	checkRow(t, got, map[string]string{
		"orTrue":   "true",
		"orFalse":  "UNBOUND",
		"andFalse": "false",
		"andTrue":  "UNBOUND",
		"notErr":   "UNBOUND",
		"notIRI":   "UNBOUND",
	})
}

// Integers compare exactly, beyond float64 precision.
func TestEquality_LargeIntegersExact(t *testing.T) {
	got := selectRow(t, ``, `SELECT (9007199254740993 = 9007199254740992 AS ?eq)
		(9007199254740993 > 9007199254740992 AS ?gt) {}`)
	checkRow(t, got, map[string]string{"eq": "false", "gt": "true"})
}
