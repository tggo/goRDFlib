package sparql

import (
	"sort"
	"strings"
	"testing"
)

// §17.4.1.3: an unbound argument is an error, so BIND leaves the variable
// unbound. Every built-in that evaluates its arguments is checked.
func TestBuiltins_UnboundArgumentIsError(t *testing.T) {
	calls := map[string]string{
		"isiri": "isIRI(?u)", "isblank": "isBlank(?u)", "isliteral": "isLiteral(?u)",
		"isnumeric": "isNumeric(?u)", "sameterm": "sameTerm(?u, 1)", "str": "STR(?u)",
		"lang": "LANG(?u)", "datatype": "DATATYPE(?u)", "iri": "IRI(?u)", "bnode": "BNODE(?u)",
		"strdt": "STRDT(?u, <urn:t>)", "strlang": `STRLANG(?u, "en")`, "strlen": "STRLEN(?u)",
		"substr": "SUBSTR(?u, 1)", "ucase": "UCASE(?u)", "lcase": "LCASE(?u)",
		"strstarts": `STRSTARTS(?u, "")`, "strends": `STRENDS(?u, "")`, "contains": `CONTAINS(?u, "")`,
		"strbefore": `STRBEFORE(?u, "")`, "strafter": `STRAFTER(?u, "")`,
		"encode": "ENCODE_FOR_URI(?u)", "concat": `CONCAT("a", ?u)`,
		"langmatches": `langMatches(?u, "*")`, "regex": `REGEX(?u, "")`, "replace": `REPLACE(?u, "a", "b")`,
		"abs": "ABS(?u)", "round": "ROUND(?u)", "ceil": "CEIL(?u)", "floor": "FLOOR(?u)",
		"year": "YEAR(?u)", "tz": "TZ(?u)", "timezone": "TIMEZONE(?u)",
		"md5": "MD5(?u)", "sha1": "SHA1(?u)", "sha256": "SHA256(?u)", "sha384": "SHA384(?u)", "sha512": "SHA512(?u)",
		"istriple": "isTRIPLE(?u)", "haslang": "hasLANG(?u)", "langdir": "LANGDIR(?u)",
		"cast": "<http://www.w3.org/2001/XMLSchema#string>(?u)",
	}
	names := make([]string, 0, len(calls))
	for n := range calls {
		names = append(names, n)
	}
	sort.Strings(names)
	var sb strings.Builder
	sb.WriteString("SELECT")
	want := make(map[string]string, len(calls))
	for _, n := range names {
		sb.WriteString(" (" + calls[n] + " AS ?" + n + ")")
		want[n] = "UNBOUND"
	}
	sb.WriteString(" {}")
	checkRow(t, selectRow(t, ``, sb.String()), want)
}

// rdflib #2050: an unbound OPTIONAL variable inside CONCAT makes the whole
// chain an error instead of hashing the empty string.
func TestBuiltins_ErrorPropagatesThroughNestedCalls(t *testing.T) {
	got := selectRow(t, `<urn:r1> <urn:id> "1" .`, `SELECT ?post WHERE {
		?ru <urn:id> ?amid . OPTIONAL { ?ru <urn:function> ?function }
		BIND(URI(CONCAT("urn:ns/", MD5(CONCAT(?amid, ?function)))) AS ?post) }`)
	checkRow(t, got, map[string]string{"post": "UNBOUND"})
}

// §17.4: arguments of the wrong type are errors.
func TestBuiltins_WrongArgumentTypeIsError(t *testing.T) {
	got := selectRow(t, ``, `PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
SELECT (STRLEN(<urn:x>) AS ?strlenIRI)
       (UCASE(<urn:x>) AS ?ucaseIRI)
       (MD5(1) AS ?md5Int)
       (MD5("a"@en) AS ?md5Lang)
       (CONTAINS(1, "1") AS ?containsInt)
       (STRSTARTS("abc"@en, "a"@fr) AS ?incompatible)
       (ABS("1") AS ?absStr)
       (ROUND(<urn:x>) AS ?roundIRI)
       (SUBSTR("abc", "1") AS ?substrStart)
       (STR(BNODE()) AS ?strBnode)
       (IRI(1) AS ?iriInt)
       (STRDT("1", "t") AS ?strdtLit)
       (YEAR("2011-01-10T14:45:13Z") AS ?yearString)
       (REGEX("abc", "[") AS ?badPattern)
       (REGEX("abc", "a", "z") AS ?badFlag)
       (REPLACE("abc", "x*", "y") AS ?emptyMatch)
       (LANGMATCHES(1, "*") AS ?langmatchesInt) {}`)
	want := map[string]string{}
	for v := range got {
		want[v] = "UNBOUND"
	}
	checkRow(t, got, want)
}

// Well-typed calls still work, with XPath semantics where they differ from Go.
func TestBuiltins_WellTypedResults(t *testing.T) {
	got := selectRow(t, ``, `PREFIX xsd: <http://www.w3.org/2001/XMLSchema#>
SELECT (ROUND(-2.5) AS ?roundHalf)
       (DATATYPE(ABS(-2)) AS ?absType)
       (ABS(-2) AS ?abs)
       (DATATYPE(CEIL("1.5"^^xsd:float)) AS ?ceilType)
       (SUBSTR("12345", 1.5, 2.6) AS ?substr)
       (SUBSTR("motor"@en, 2) AS ?substrLang)
       (DATATYPE("a") AS ?simpleType)
       (REGEX("ABC", "b", "i") AS ?regexI)
       (REGEX("a.c", ".", "q") AS ?regexQ)
       (REPLACE("abcd", "(b)(c)", "$2$1\\$") AS ?replace)
       (YEAR("2011-01-10T14:45:13Z"^^xsd:dateTime) AS ?year)
       (isIRI(<urn:x>) AS ?isiri) {}`)
	checkRow(t, got, map[string]string{
		"roundHalf":  "-2.0",
		"absType":    "<http://www.w3.org/2001/XMLSchema#integer>",
		"abs":        "2",
		"ceilType":   "<http://www.w3.org/2001/XMLSchema#float>",
		"substr":     `"234"`,
		"substrLang": `"otor"@en`,
		"simpleType": "<http://www.w3.org/2001/XMLSchema#string>",
		"regexI":     "true",
		"regexQ":     "true",
		"replace":    `"acb$d"`,
		"year":       "2011",
		"isiri":      "true",
	})
}
