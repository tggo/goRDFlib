package term

import "testing"

func TestValueEqualXSD(t *testing.T) {
	lit := func(lex string, dt URIRef) Literal { return NewLiteral(lex, WithDatatype(dt)) }
	dt := func(local string) URIRef { return NewURIRefUnsafe(XSDNamespace + local) }
	tests := []struct {
		name string
		a, b Literal
		want bool
	}{
		// decimals are exact
		{"decimal precision", lit("0.1", XSDDecimal), lit("0.10000000000000000001", XSDDecimal), false},
		{"decimal trailing zeros", lit("1.50", XSDDecimal), lit("+1.5", XSDDecimal), true},
		{"decimal trailing dot", lit("5.", XSDDecimal), lit("5", XSDDecimal), true},
		{"decimal leading dot", lit(".5", XSDDecimal), lit("0.5", XSDDecimal), true},
		{"decimal negative zero", lit("-0.0", XSDDecimal), lit("0", XSDDecimal), true},
		{"decimal exponent is ill-typed", lit("1e3", XSDDecimal), lit("1000", XSDDecimal), false},
		{"decimal lone dot", lit(".", XSDDecimal), lit("0", XSDDecimal), false},

		// integers beyond int64
		{"big integer", lit("10000000000000000000", XSDInteger), lit("010000000000000000000", XSDInteger), true},
		{"big integer differs", lit("10000000000000000000", XSDInteger), lit("10000000000000000001", XSDInteger), false},
		{"integer sign", lit("+1", XSDInteger), lit("1", XSDInteger), true},
		{"integer double sign", lit("+-1", XSDInteger), lit("-1", XSDInteger), false},
		{"int out of range is ill-typed", lit("2147483648", XSDInt), lit("02147483648", XSDInt), false},
		{"unsignedByte", lit("255", dt("unsignedByte")), lit("0255", dt("unsignedByte")), true},
		{"negativeInteger zero ill-typed", lit("0", dt("negativeInteger")), lit("-0", dt("negativeInteger")), false},

		// doubles and floats
		{"double INF", lit("INF", XSDDouble), lit("+INF", XSDDouble), true},
		{"double inf ill-typed", lit("inf", XSDDouble), lit("INF", XSDDouble), false},
		{"double Infinity ill-typed", lit("Infinity", XSDDouble), lit("INF", XSDDouble), false},
		{"double hex ill-typed", lit("0x1p3", XSDDouble), lit("8", XSDDouble), false},
		{"double NaN", lit("NaN", XSDDouble), lit("NaN", XSDDouble), false},
		{"double zero sign", lit("-0", XSDDouble), lit("0.0e0", XSDDouble), true},
		{"double exponent", lit("1.0", XSDDouble), lit("1.00", XSDDouble), true},
		{"double 1e3", lit("1e3", XSDDouble), lit("1000", XSDDouble), true},
		{"float rounding", lit("16777217", XSDFloat), lit("16777216", XSDFloat), true},
		{"double overflow rounds to INF", lit("1e400", XSDDouble), lit("INF", XSDDouble), true},

		// booleans
		{"boolean 1", lit("1", XSDBoolean), lit("true", XSDBoolean), true},
		{"boolean T ill-typed", lit("T", XSDBoolean), lit("true", XSDBoolean), false},
		{"boolean TRUE ill-typed", lit("TRUE", XSDBoolean), lit("true", XSDBoolean), false},

		// ill-typed identical lexical forms are the same term
		{"ill-typed identical", lit("abc", XSDInteger), lit("abc", XSDInteger), true},

		// dateTime
		{"dateTime Z vs +00:00", lit("2011-01-25T00:00:00Z", XSDDateTime), lit("2011-01-25T00:00:00+00:00", XSDDateTime), true},
		{"dateTime -00:00", lit("2011-01-25T00:00:00-00:00", XSDDateTime), lit("2011-01-25T00:00:00Z", XSDDateTime), true},
		{"dateTime offset", lit("2011-01-25T02:30:00+02:30", XSDDateTime), lit("2011-01-25T00:00:00Z", XSDDateTime), true},
		{"dateTime offset crosses day", lit("2011-01-24T20:00:00-04:00", XSDDateTime), lit("2011-01-25T00:00:00Z", XSDDateTime), true},
		{"dateTime 24:00", lit("2011-12-31T24:00:00Z", XSDDateTime), lit("2012-01-01T00:00:00Z", XSDDateTime), true},
		{"dateTime fraction", lit("2011-01-25T00:00:00.500Z", XSDDateTime), lit("2011-01-25T00:00:00.5Z", XSDDateTime), true},
		{"dateTime fraction zero", lit("2011-01-25T00:00:00.000Z", XSDDateTime), lit("2011-01-25T00:00:00Z", XSDDateTime), true},
		{"dateTime differs", lit("2011-01-25T00:00:01Z", XSDDateTime), lit("2011-01-25T00:00:00Z", XSDDateTime), false},
		{"dateTime timezone vs none", lit("2011-01-25T00:00:00Z", XSDDateTime), lit("2011-01-25T00:00:00", XSDDateTime), false},
		{"dateTime no timezone", lit("2011-01-25T00:00:00", XSDDateTime), lit("2011-01-25T00:00:00.0", XSDDateTime), true},
		{"dateTime Feb 30 ill-typed", lit("2011-02-30T00:00:00Z", XSDDateTime), lit("2011-03-02T00:00:00Z", XSDDateTime), false},
		{"dateTime leap day", lit("2012-02-29T00:00:00Z", XSDDateTime), lit("2012-02-28T24:00:00Z", XSDDateTime), true},
		{"dateTime negative year", lit("-0001-03-01T00:00:00Z", XSDDateTime), lit("-0001-02-28T24:00:00Z", XSDDateTime), true},
		{"dateTime offset 14:30 ill-typed", lit("2011-01-25T00:00:00+14:30", XSDDateTime), lit("2011-01-24T09:30:00Z", XSDDateTime), false},

		// language-tagged strings keep their tag
		{"langString tag", NewLiteral("a", WithLang("en")), NewLiteral("a", WithLang("fr")), false},
		{"langString same", NewLiteral("a", WithLang("en-GB")), NewLiteral("a", WithLang("en-gb")), true},
		{"dirLangString dir", NewLiteral("a", WithLang("ar"), WithDir("rtl")), NewLiteral("a", WithLang("ar"), WithDir("ltr")), false},
	}
	for _, tt := range tests {
		if got := tt.a.ValueEqual(tt.b); got != tt.want {
			t.Errorf("%s: %s ValueEqual %s = %v, want %v", tt.name, tt.a.N3(), tt.b.N3(), got, tt.want)
		}
		if got := tt.b.ValueEqual(tt.a); got != tt.want {
			t.Errorf("%s (reversed): got %v, want %v", tt.name, got, tt.want)
		}
	}
}
