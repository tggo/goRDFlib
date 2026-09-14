package sparql

import "testing"

// SPARQL 1.1 §17.1: the types derived from xsd:integer are numeric.
func TestNumeric_DerivedIntegerTypes(t *testing.T) {
	got := selectRow(t, ``, `PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
SELECT ("5"^^xsd:short + 1 AS ?sum)
       ("01"^^xsd:byte = 1 AS ?eq)
       ("10"^^xsd:nonNegativeInteger > "9"^^xsd:nonNegativeInteger AS ?gt)
       ("7"^^xsd:unsignedLong * "6"^^xsd:positiveInteger AS ?mul)
       (-"3"^^xsd:unsignedByte AS ?neg)
       (DATATYPE("5"^^xsd:short + "1"^^xsd:short) AS ?dt) {}`)
	checkRow(t, got, map[string]string{
		"sum": "6",
		"eq":  "true",
		"gt":  "true",
		"mul": "42",
		"neg": "-3",
		"dt":  "<http://www.w3.org/2001/XMLSchema#integer>",
	})
}

// §17.4.2.5: isNumeric examples, and a value outside the type's range is
// ill-typed.
func TestNumeric_IsNumericAndRanges(t *testing.T) {
	got := selectRow(t, ``, `PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
SELECT (isNumeric(12) AS ?int)
       (isNumeric("12") AS ?str)
       (isNumeric("12"^^xsd:nonNegativeInteger) AS ?nni)
       (isNumeric("1200"^^xsd:byte) AS ?byteRange)
       (isNumeric("-1"^^xsd:unsignedInt) AS ?unsigned)
       (isNumeric("0"^^xsd:positiveInteger) AS ?positive)
       (isNumeric("12"^^<urn:x>) AS ?unknown)
       (COALESCE("999"^^xsd:byte > 0, "OK") AS ?coal) {}`)
	checkRow(t, got, map[string]string{
		"int":       "true",
		"str":       "false",
		"nni":       "true",
		"byteRange": "false",
		"unsigned":  "false",
		"positive":  "false",
		"unknown":   "false",
		"coal":      `"OK"`,
	})
}

// §17.3 numeric type promotion: integer → decimal → float → double, and
// integer / integer is a decimal.
func TestNumeric_TypePromotion(t *testing.T) {
	got := selectRow(t, ``, `PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
SELECT (DATATYPE(1 + 2) AS ?ii)
       (DATATYPE(1 / 2) AS ?idiv)
       (1 / 2 AS ?half)
       (DATATYPE(1 + 2.5) AS ?id)
       (DATATYPE(1 + "2"^^xsd:float) AS ?if)
       (DATATYPE(2.5 + "2"^^xsd:float) AS ?df)
       (DATATYPE("2"^^xsd:float + 1e0) AS ?fd)
       (DATATYPE(-"2"^^xsd:float) AS ?negf)
       (1 / 0 AS ?divZeroInt)
       (1.0 / 0 AS ?divZeroDec)
       (1e0 / 0 AS ?divZeroDouble)
       (9223372036854775807 + 1 AS ?big) {}`)
	checkRow(t, got, map[string]string{
		"ii":            "<http://www.w3.org/2001/XMLSchema#integer>",
		"idiv":          "<http://www.w3.org/2001/XMLSchema#decimal>",
		"half":          "0.5",
		"id":            "<http://www.w3.org/2001/XMLSchema#decimal>",
		"if":            "<http://www.w3.org/2001/XMLSchema#float>",
		"df":            "<http://www.w3.org/2001/XMLSchema#float>",
		"fd":            "<http://www.w3.org/2001/XMLSchema#double>",
		"negf":          "<http://www.w3.org/2001/XMLSchema#float>",
		"divZeroInt":    "UNBOUND",
		"divZeroDec":    "UNBOUND",
		"divZeroDouble": `"INF"^^<http://www.w3.org/2001/XMLSchema#double>`,
		"big":           `"9223372036854775808"^^<http://www.w3.org/2001/XMLSchema#integer>`,
	})
}

// SUM and AVG promote like +, and accept derived integer types.
func TestNumeric_AggregatesOverDerivedTypes(t *testing.T) {
	data := `@prefix xsd: <http://www.w3.org/2001/XMLSchema#> .
<urn:a> <urn:v> "1"^^xsd:short . <urn:b> <urn:v> "2"^^xsd:unsignedByte .`
	got := selectRow(t, data, `SELECT (SUM(?v) AS ?sum) (AVG(?v) AS ?avg)
		WHERE { ?s <urn:v> ?v }`)
	checkRow(t, got, map[string]string{
		"sum": "3",
		"avg": "1.5", // N3 shorthand: an xsd:decimal
	})
}
