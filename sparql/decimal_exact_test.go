package sparql

import (
	"testing"

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
