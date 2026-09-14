package shacl

import (
	"math/big"
	"regexp"
)

// Lexical validation of XSD literals for sh:datatype.
//
// SHACL 1.0 §4.1.2: a value node conforms to sh:datatype only if it is a
// literal with that datatype that is not ill-typed, at least for the datatypes
// SPARQL 1.1 supports. RDF 1.1 Concepts §5.3 takes the datatypes from XSD 1.1,
// so the grammars below are those of XSD 1.1 Part 2 (W3C Recommendation,
// 2012-04-05), cited by section.
//
// The whiteSpace facet is not applied: RDF 1.1 Concepts §5.3 defines the
// lexical space of an XSD datatype without the pre-lexical whitespace
// normalisation, so " 1" is not in the lexical space of xsd:integer.

// Date/time fragments, XSD 1.1 Part 2 §D.2.2 (productions [56]-[63]).
const (
	// yearFrag ::= '-'? (([1-9] digit digit digit+) | ('0' digit digit digit))
	xsdYearFrag = `(-?(?:[1-9]\d{3,}|0\d{3}))`
	// monthFrag ::= ('0' [1-9]) | ('1' [0-2])
	xsdMonthFrag = `(0[1-9]|1[0-2])`
	// dayFrag ::= ('0' [1-9]) | ([12] digit) | ('3' [01])
	xsdDayFrag = `(0[1-9]|[12]\d|3[01])`
	// hourFrag ':' minuteFrag ':' secondFrag | endOfDayFrag, where
	// secondFrag ::= ([0-5] digit) ('.' digit+)? and
	// endOfDayFrag ::= '24:00:00' ('.' '0'+)?
	xsdTimeFrag = `(?:(?:[01]\d|2[0-3]):[0-5]\d:[0-5]\d(?:\.\d+)?|24:00:00(?:\.0+)?)`
	// timezoneFrag ::= 'Z' | ('+' | '-') (('0' digit | '1' [0-3]) ':' minuteFrag | '14:00')
	xsdTimezoneFrag = `(?:Z|[+-](?:(?:0\d|1[0-3]):[0-5]\d|14:00))`
	xsdOptTimezone  = xsdTimezoneFrag + `?`
)

var (
	// §3.3.7 dateTime: yearFrag '-' monthFrag '-' dayFrag 'T' time timezoneFrag?
	xsdDateTimeRe = regexp.MustCompile(`^` + xsdYearFrag + `-` + xsdMonthFrag + `-` + xsdDayFrag + `T` + xsdTimeFrag + xsdOptTimezone + `$`)
	// §3.4.28 dateTimeStamp: a dateTime whose timezone is required.
	xsdDateTimeStampRe = regexp.MustCompile(`^` + xsdYearFrag + `-` + xsdMonthFrag + `-` + xsdDayFrag + `T` + xsdTimeFrag + xsdTimezoneFrag + `$`)
	// §3.3.8 time
	xsdTimeRe = regexp.MustCompile(`^` + xsdTimeFrag + xsdOptTimezone + `$`)
	// §3.3.9 date
	xsdDateRe = regexp.MustCompile(`^` + xsdYearFrag + `-` + xsdMonthFrag + `-` + xsdDayFrag + xsdOptTimezone + `$`)
	// §3.3.10 gYearMonth
	xsdGYearMonthRe = regexp.MustCompile(`^` + xsdYearFrag + `-` + xsdMonthFrag + xsdOptTimezone + `$`)
	// §3.3.11 gYear
	xsdGYearRe = regexp.MustCompile(`^` + xsdYearFrag + xsdOptTimezone + `$`)
	// §3.3.12 gMonthDay: '--' monthFrag '-' dayFrag timezoneFrag?
	xsdGMonthDayRe = regexp.MustCompile(`^--` + xsdMonthFrag + `-` + xsdDayFrag + xsdOptTimezone + `$`)
	// §3.3.13 gDay: '---' dayFrag timezoneFrag?
	xsdGDayRe = regexp.MustCompile(`^---` + xsdDayFrag + xsdOptTimezone + `$`)
	// §3.3.14 gMonth: '--' monthFrag timezoneFrag?
	xsdGMonthRe = regexp.MustCompile(`^--` + xsdMonthFrag + xsdOptTimezone + `$`)

	// §3.3.6 duration. The pattern admits an empty duration ("P", "PT") and
	// a bare "T"; isValidDuration rejects those, as the grammar requires at
	// least one field and a 'T' followed by at least one time field.
	xsdDurationRe = regexp.MustCompile(`^-?P(\d+Y)?(\d+M)?(\d+D)?(T(\d+H)?(\d+M)?((?:\d+(?:\.\d*)?|\.\d+)S)?)?$`)
	// §3.4.26 yearMonthDuration: '-'? 'P' duYearMonthFrag
	xsdYearMonthDurationRe = regexp.MustCompile(`^-?P(?:\d+Y(?:\d+M)?|\d+M)$`)

	// §3.3.3 decimal: ('+' | '-')? (digit+ ('.' digit*)? | '.' digit+)
	xsdDecimalRe = regexp.MustCompile(`^[+-]?(?:\d+(?:\.\d*)?|\.\d+)$`)
	// §3.3.4 float / §3.3.5 double: decimal, optionally with an integer
	// exponent, or one of the special values (XSD 1.1 admits "+INF").
	xsdFloatRe = regexp.MustCompile(`^(?:[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?|[+-]?INF|NaN)$`)
	// §3.4.13 integer: ('+' | '-')? digit+
	xsdIntegerRe = regexp.MustCompile(`^[+-]?\d+$`)

	// §3.3.15 hexBinary: ([0-9a-fA-F]{2})*
	xsdHexBinaryRe = regexp.MustCompile(`^(?:[0-9a-fA-F]{2})*$`)
	// §3.3.16 base64Binary, the regular expression given in the section.
	xsdBase64BinaryRe = regexp.MustCompile(`^((([A-Za-z0-9+/] ?){4})*(([A-Za-z0-9+/] ?){3}[A-Za-z0-9+/]|([A-Za-z0-9+/] ?){2}[AEIMQUYcgkosw048] ?=|[A-Za-z0-9+/] ?[AQgw] ?= ?=))?$`)
)

// Value space bounds of the bounded integer types, XSD 1.1 Part 2 §3.4.14-§3.4.25.
var xsdIntegerBounds = map[string][2]*big.Int{
	XSD + "long":          {bigInt("-9223372036854775808"), bigInt("9223372036854775807")},
	XSD + "int":           {big.NewInt(-2147483648), big.NewInt(2147483647)},
	XSD + "short":         {big.NewInt(-32768), big.NewInt(32767)},
	XSD + "byte":          {big.NewInt(-128), big.NewInt(127)},
	XSD + "unsignedLong":  {big.NewInt(0), bigInt("18446744073709551615")},
	XSD + "unsignedInt":   {big.NewInt(0), big.NewInt(4294967295)},
	XSD + "unsignedShort": {big.NewInt(0), big.NewInt(65535)},
	XSD + "unsignedByte":  {big.NewInt(0), big.NewInt(255)},
}

func bigInt(s string) *big.Int {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("shacl: bad integer bound " + s)
	}
	return n
}

// isWellFormedLiteral reports whether t is a literal whose lexical form is in
// the lexical space of its datatype.
//
// A datatype not listed here is accepted: SHACL 1.0 §4.1.2 requires the
// ill-typed check only for the datatypes a SPARQL 1.1 processor supports, and
// for an unknown datatype there is no lexical space to check against.
func isWellFormedLiteral(t Term) bool {
	if !t.IsLiteral() {
		return false
	}
	dt := t.Datatype()
	val := t.Value()
	switch dt {
	case XSD + "integer":
		return isValidInteger(val)
	case XSD + "nonNegativeInteger", XSD + "positiveInteger",
		XSD + "nonPositiveInteger", XSD + "negativeInteger":
		return isValidInteger(val) && isInIntegerRange(val, dt)
	case XSD + "long", XSD + "int", XSD + "short", XSD + "byte",
		XSD + "unsignedLong", XSD + "unsignedInt", XSD + "unsignedShort", XSD + "unsignedByte":
		if !isValidInteger(val) {
			return false
		}
		b := xsdIntegerBounds[dt]
		n, _ := new(big.Int).SetString(val, 10) // isValidInteger guarantees it parses
		return n.Cmp(b[0]) >= 0 && n.Cmp(b[1]) <= 0
	case XSD + "boolean":
		// §3.3.2 boolean: 'true' | 'false' | '1' | '0'
		return val == "true" || val == "false" || val == "1" || val == "0"
	case XSD + "decimal":
		return isValidDecimal(val)
	case XSD + "float", XSD + "double":
		return isValidFloat(val)
	case XSD + "dateTime":
		return isValidDateTime(val)
	case XSD + "dateTimeStamp":
		return isValidDateTimeStamp(val)
	case XSD + "date":
		return isValidDate(val)
	case XSD + "time":
		return xsdTimeRe.MatchString(val)
	case XSD + "gYearMonth":
		return xsdGYearMonthRe.MatchString(val)
	case XSD + "gYear":
		return xsdGYearRe.MatchString(val)
	case XSD + "gMonthDay":
		// §3.3.12: the day must not exceed the most days the month can have,
		// so --02-29 is allowed and --04-31 is not.
		m := xsdGMonthDayRe.FindStringSubmatch(val)
		return m != nil && atoi2(m[2]) <= daysInMonth("2000", atoi2(m[1]))
	case XSD + "gDay":
		return xsdGDayRe.MatchString(val)
	case XSD + "gMonth":
		return xsdGMonthRe.MatchString(val)
	case XSD + "duration":
		return isValidDuration(val)
	case XSD + "yearMonthDuration":
		return xsdYearMonthDurationRe.MatchString(val)
	case XSD + "dayTimeDuration":
		// §3.4.27: '-'? 'P' duDayTimeFrag — a duration with no Y or M before T.
		m := xsdDurationRe.FindStringSubmatch(val)
		return m != nil && m[1] == "" && m[2] == "" && isValidDuration(val)
	case XSD + "hexBinary":
		return xsdHexBinaryRe.MatchString(val)
	case XSD + "base64Binary":
		return xsdBase64BinaryRe.MatchString(val)
	}
	return true
}

func isValidInteger(s string) bool { return xsdIntegerRe.MatchString(s) }

// isValidDecimal accepts "1." and ".5", which XSD 1.1 §3.3.3 admits.
func isValidDecimal(s string) bool { return xsdDecimalRe.MatchString(s) }

func isValidFloat(s string) bool { return xsdFloatRe.MatchString(s) }

func isValidDate(s string) bool {
	m := xsdDateRe.FindStringSubmatch(s)
	return m != nil && atoi2(m[3]) <= daysInMonth(m[1], atoi2(m[2]))
}

func isValidDateTime(s string) bool {
	m := xsdDateTimeRe.FindStringSubmatch(s)
	return m != nil && atoi2(m[3]) <= daysInMonth(m[1], atoi2(m[2]))
}

func isValidDateTimeStamp(s string) bool {
	m := xsdDateTimeStampRe.FindStringSubmatch(s)
	return m != nil && atoi2(m[3]) <= daysInMonth(m[1], atoi2(m[2]))
}

// isValidDuration applies the constraints of §3.3.6 that the pattern cannot:
// at least one field is present, and a 'T' is followed by at least one of
// hours, minutes or seconds.
func isValidDuration(s string) bool {
	m := xsdDurationRe.FindStringSubmatch(s)
	if m == nil {
		return false
	}
	hasDate := m[1] != "" || m[2] != "" || m[3] != ""
	hasT := m[4] != ""
	hasTime := m[5] != "" || m[6] != "" || m[7] != ""
	if hasT && !hasTime {
		return false
	}
	return hasDate || hasTime
}

// atoi2 converts a one- or two-digit string the regular expressions have
// already restricted to digits.
func atoi2(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		n = n*10 + int(s[i]-'0')
	}
	return n
}

// daysInMonth applies the Day-of-month Values constraint of XSD 1.1 Part 2 (the
// seven-property model, Appendix D.2; also daysInMonth in §E.3.2): February
// has 29 days when the year is divisible by 400, or by 4 but not by 100. Year
// is the lexical year, which may carry a sign and any number of digits; only
// its remainder modulo 400 matters, so it is never parsed as a whole. XSD 1.1
// numbers years astronomically (0000 is 1 BCE), so the rule applies to the
// year value as written, sign included.
func daysInMonth(year string, month int) int {
	switch month {
	case 4, 6, 9, 11:
		return 30
	case 2:
		rem := 0
		for i := 0; i < len(year); i++ {
			if c := year[i]; c >= '0' && c <= '9' {
				rem = (rem*10 + int(c-'0')) % 400
			}
		}
		if rem%400 == 0 || (rem%4 == 0 && rem%100 != 0) {
			return 29
		}
		return 28
	}
	return 31
}

func isInIntegerRange(val, dt string) bool {
	n, ok := new(big.Int).SetString(val, 10)
	if !ok {
		return false
	}
	switch dt {
	case XSD + "nonNegativeInteger":
		return n.Sign() >= 0
	case XSD + "positiveInteger":
		return n.Sign() > 0
	case XSD + "nonPositiveInteger":
		return n.Sign() <= 0
	case XSD + "negativeInteger":
		return n.Sign() < 0
	}
	return true
}
