package sparql

import (
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

func typed(s string, dt rdflibgo.URIRef) rdflibgo.Literal {
	return rdflibgo.NewLiteral(s, rdflibgo.WithDatatype(dt))
}

// A value with a timezone and one without are not equal, and not different
// either unless more than 14 hours apart: the order is indeterminate (XSD 1.1
// section 3.3.7.2), so "=" and "!=" are type errors. DAWG open-world/date-1 and
// expr-equals/Equality with dateTime.
func TestTemporalTimezoneIsIndeterminate(t *testing.T) {
	d, dt := rdflibgo.XSDDate, rdflibgo.XSDDateTime
	for _, c := range []struct {
		a, b        rdflibgo.Literal
		equal       bool
		determinate bool
	}{
		{typed("2006-08-23", d), typed("2006-08-23", d), true, true},
		{typed("2006-08-23Z", d), typed("2006-08-23+00:00", d), true, true},
		{typed("2006-08-23", d), typed("2006-08-23Z", d), false, false},
		{typed("2006-08-23", d), typed("2006-08-23+00:00", d), false, false},
		{typed("2006-08-23", d), typed("2006-08-25Z", d), false, true},
		{typed("2002-04-02T23:00:00", dt), typed("2002-04-02T23:00:00+06:00", dt), false, false},
		{typed("2002-04-02T23:00:00", dt), typed("2003-04-02T23:00:00+06:00", dt), false, true},
		{typed("2002-04-02T23:00:00-04:00", dt), typed("2002-04-03T02:00:00-01:00", dt), true, true},
		// 24:00:00 is the first instant of the next day.
		{typed("1999-12-31T24:00:00", dt), typed("2000-01-01T00:00:00", dt), true, true},
		{typed("2005-04-04T24:00:00", dt), typed("2005-04-04T00:00:00", dt), false, true},
		{typed("1999-12-31T24:00:00Z", dt), typed("2000-01-01T00:00:00Z", dt), true, true},
	} {
		eq, ok := literalsEqual(c.a, c.b)
		if ok != c.determinate || (ok && eq != c.equal) {
			t.Errorf("%s = %s: got (%v, %v), want (%v, %v)", c.a.N3(), c.b.N3(), eq, ok, c.equal, c.determinate)
		}
		// Symmetric.
		eq, ok = literalsEqual(c.b, c.a)
		if ok != c.determinate || (ok && eq != c.equal) {
			t.Errorf("%s = %s: got (%v, %v), want (%v, %v)", c.b.N3(), c.a.N3(), eq, ok, c.equal, c.determinate)
		}
	}
	// "<" agrees: indeterminate is an error, determinate orders.
	if _, ok := valueCompare(typed("2006-08-23", d), typed("2006-08-23Z", d)); ok {
		t.Error("untimezoned < timezoned same day should be an error")
	}
	if c, ok := valueCompare(typed("2006-08-23", d), typed("2006-08-25Z", d)); !ok || c >= 0 {
		t.Errorf("got (%d, %v)", c, ok)
	}
}

func TestCastToDateTime(t *testing.T) {
	g := makeSPARQLGraph(t)
	for expr, want := range map[string]string{
		`xsd:dateTime("2002-10-10T17:00:00Z")`:                                `"2002-10-10T17:00:00Z"^^<http://www.w3.org/2001/XMLSchema#dateTime>`,
		`xsd:dateTime("2002-10-10T17:00:00Z"^^xsd:dateTime)`:                  `"2002-10-10T17:00:00Z"^^<http://www.w3.org/2001/XMLSchema#dateTime>`,
		`xsd:dateTime("2002-10-10T17:00:00"^^xsd:string)`:                     `"2002-10-10T17:00:00"^^<http://www.w3.org/2001/XMLSchema#dateTime>`,
		`xsd:dateTime("yesterday")`:                                           ``,
		`xsd:dateTime(1)`:                                                     ``,
		`xsd:dateTime(<http://example.org/x>)`:                                ``,
		`datatype(xsd:dateTime("2002-10-10T17:00:00Z"))`:                      `<http://www.w3.org/2001/XMLSchema#dateTime>`,
		`xsd:dateTime("2002-10-10T17:00:00Z"^^xsd:date)`:                      ``,
		`<http://www.w3.org/2001/XMLSchema#dateTime>("2002-10-10T17:00:00Z")`: `"2002-10-10T17:00:00Z"^^<http://www.w3.org/2001/XMLSchema#dateTime>`,
	} {
		res, err := Query(g, `PREFIX xsd: <http://www.w3.org/2001/XMLSchema#> SELECT (`+expr+` AS ?r) WHERE {}`)
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

// A relative namespace in PREFIX is resolved against the BASE declared before
// it (DAWG basic/Basic - Prefix/Base 1 and 2).
func TestPrefixResolvedAgainstBase(t *testing.T) {
	for _, c := range []struct{ decl, want string }{
		{`BASE <http://example.org/x/> PREFIX : <>`, "http://example.org/x/x"},
		{`BASE <http://example.org/x/> PREFIX : <#>`, "http://example.org/x/#x"},
		{`BASE <http://example.org/x/> PREFIX : <sub/>`, "http://example.org/x/sub/x"},
		{`BASE <http://example.org/x/> PREFIX : <http://other/>`, "http://other/x"},
		{`PREFIX : <http://other/> BASE <http://example.org/x/>`, "http://other/x"},
	} {
		q, err := Parse(c.decl + ` SELECT * { :x ?p ?o }`)
		if err != nil {
			t.Fatal(err)
		}
		if got := q.Prefixes[""] + "x"; got != c.want {
			t.Errorf("%s: :x is %q, want %q", c.decl, got, c.want)
		}
	}
}
