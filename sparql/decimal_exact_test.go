package sparql

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tggo/goRDFlib/graph"
)

// xsd:decimal is exact (XSD 1.1 §3.3.3), and results are written in its
// canonical form. Arithmetic used to go through float64 and was cut to ten
// fractional digits: the first two cases lost value. Expected values are
// Oxigraph's (testdata/sparql-diff found the discrepancy).
func TestDecimalArithmeticIsExact(t *testing.T) {
	for expr, want := range map[string]string{
		"12345678901234567.5 + 1":               "12345678901234568.5",
		"1.0000000000001 + 0":                   "1.0000000000001",
		"0.1 + 0.2":                             "0.3",
		"1/3":                                   "0.333333333333333333",
		"10/4":                                  "2.5",
		"178.0 + 0":                             "178",
		"-0.5 * 2":                              "-1",
		"ABS(-123456789012345678.25)":           "123456789012345678.25",
		"ROUND(2.5)":                            "3",
		"ROUND(-2.5)":                           "-2",
		"CEIL(-0.5)":                            "0",
		"FLOOR(-0.5)":                           "-1",
		"xsd:decimal(\"0.10\")":                 "0.1",
		"xsd:decimal(\"98765432109876543.21\")": "98765432109876543.21",
	} {
		q := `PREFIX xsd: <http://www.w3.org/2001/XMLSchema#> SELECT ((` + expr + `) AS ?x) WHERE {}`
		r, err := Query(graph.NewGraph(), q)
		if err != nil {
			t.Errorf("%s: %v", expr, err)
			continue
		}
		x := r.Bindings[0]["x"]
		if x == nil || x.String() != want {
			t.Errorf("%s = %v, want %s", expr, x, want)
		}
	}
	for q, want := range map[string]bool{
		`ASK { FILTER (0.1 + 0.2 = 0.3) }`:                           true,
		`ASK { FILTER (12345678901234567.5 < 12345678901234567.6) }`: true,
		`ASK { FILTER (12345678901234567.5 = 12345678901234567.6) }`: false,
		`ASK { FILTER (0.00000000000000000001) }`:                    true,
	} {
		r, err := Query(graph.NewGraph(), q)
		if err != nil || r.AskResult != want {
			t.Errorf("%s = %v (%v), want %v", q, r.AskResult, err, want)
		}
	}
}

// Review findings on the exact decimal change: ordering and extrema, the
// string cast, NaN/INF casts, and growth bounds.
func TestDecimalEdges(t *testing.T) {
	g := graph.NewGraph()
	one := func(q string) string {
		t.Helper()
		r, err := Query(g, `PREFIX xsd: <http://www.w3.org/2001/XMLSchema#> `+q)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		if len(r.Bindings) == 0 || r.Bindings[0]["x"] == nil {
			return "unbound"
		}
		return r.Bindings[0]["x"].String()
	}
	for q, want := range map[string]string{
		`SELECT (MAX(?v) AS ?x) WHERE { VALUES ?v { 12345678901234567.4 12345678901234567.6 12345678901234567.5 } }`: "12345678901234567.6",
		`SELECT (MIN(?v) AS ?x) WHERE { VALUES ?v { 12345678901234567.4 12345678901234567.6 12345678901234567.5 } }`: "12345678901234567.4",
		`SELECT ?x WHERE { VALUES ?x { 12345678901234567.6 12345678901234567.4 } } ORDER BY ?x`:                      "12345678901234567.4",
		`SELECT (MAX(?v) AS ?x) WHERE { VALUES ?v { 9007199254740993 9007199254740992 } }`:                           "9007199254740993",
		`SELECT (xsd:string(12345678901234567.5) AS ?x) WHERE {}`:                                                    "12345678901234567.5",
		`SELECT (xsd:string(1/3) AS ?x) WHERE {}`:                                                                    "0.333333333333333333",
		`SELECT (xsd:string(123456789012345678901234567890) AS ?x) WHERE {}`:                                         "123456789012345678901234567890",
		`SELECT (xsd:decimal("NaN") AS ?x) WHERE {}`:                                                                 "unbound",
		`SELECT (xsd:decimal(xsd:double("INF")) AS ?x) WHERE {}`:                                                     "unbound",
		`SELECT (xsd:decimal("1_0") AS ?x) WHERE {}`:                                                                 "unbound",
		`SELECT (xsd:decimal(1.5e0) AS ?x) WHERE {}`:                                                                 "1.5",
		`SELECT (1/3*3 AS ?x) WHERE {}`:                                                                              "0.999999999999999999",
	} {
		if got := one(q); got != want {
			t.Errorf("%s\n got %s, want %s", q, got, want)
		}
	}
}

// Exact arithmetic must not let a query build an unbounded number: repeated
// squaring doubles the digits each step. Over the bound the result is an
// error (unbound), quickly.
func TestDecimalGrowthIsBounded(t *testing.T) {
	q := `SELECT ?x WHERE { BIND (1.123456789 AS ?v0)`
	for i := 1; i <= 20; i++ {
		q += fmt.Sprintf(` BIND (?v%d * ?v%d AS ?v%d)`, i-1, i-1, i)
	}
	q += ` BIND (?v20 AS ?x) }`
	start := time.Now()
	r, err := Query(graph.NewGraph(), q)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
	if x := r.Bindings[0]["x"]; x != nil {
		t.Errorf("?x bound to a %d-character decimal, want unbound past the bound", len(x.String()))
	}
	tiny := func(d string) string {
		return `"0.` + strings.Repeat("0", 400) + d + `"^^<http://www.w3.org/2001/XMLSchema#decimal>`
	}
	for q, want := range map[string]bool{
		`ASK { FILTER (` + tiny("1") + ` = 0) }`:                 false,
		`ASK { FILTER (` + tiny("1") + ` = ` + tiny("2") + `) }`: false,
		`ASK { FILTER (` + tiny("1") + ` < ` + tiny("2") + `) }`: true,
	} {
		r, err := Query(graph.NewGraph(), q)
		if err != nil || r.AskResult != want {
			t.Errorf("below float64 range: %v (%v), want %v", r.AskResult, err, want)
		}
	}
	huge := `"0.` + strings.Repeat("0", 200000) + `1"^^<http://www.w3.org/2001/XMLSchema#decimal>`
	for _, e := range []string{huge + " / 3", "ABS(" + huge + ")", "-" + huge, "<http://www.w3.org/2001/XMLSchema#string>(" + huge + ")"} {
		start := time.Now()
		if _, err := Query(graph.NewGraph(), `SELECT ((`+e+`) AS ?x) WHERE {}`); err != nil {
			t.Fatal(err)
		}
		if d := time.Since(start); d > 2*time.Second {
			t.Errorf("%.40s… took %v", e, d)
		}
	}
	start = time.Now()
	big := `"0.` + strings.Repeat("0", 200000) + `1"^^<http://www.w3.org/2001/XMLSchema#decimal>`
	if _, err := Query(graph.NewGraph(), `SELECT ((`+big+` * 1) AS ?x) WHERE {}`); err != nil {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("a 200,000-digit literal took %v", d)
	}
}
