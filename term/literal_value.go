package term

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// This file implements Literal.ValueEqual: equality in the XSD value space.
//
// Rules, in order:
//
//   - Literals with different datatypes are never value-equal, even when XSD
//     would relate their values ("1"^^xsd:int and "1"^^xsd:integer).
//   - A literal whose lexical form is not in its datatype's lexical space
//     (ill-typed) is value-equal only to a literal with the identical lexical
//     form. It is never equal to a well-typed literal.
//   - rdf:langString and rdf:dirLangString compare lexical form, language tag
//     (case-insensitively) and base direction.
//   - Datatypes without a value-space implementation here compare lexical forms.

// integerRange bounds an integer-derived XSD datatype; nil means unbounded.
type integerRange struct{ min, max *big.Int }

func bigInt(s string) *big.Int {
	v, _ := new(big.Int).SetString(s, 10)
	return v
}

var integerDatatypes = map[string]integerRange{
	XSDNamespace + "integer":            {},
	XSDNamespace + "nonPositiveInteger": {max: bigInt("0")},
	XSDNamespace + "negativeInteger":    {max: bigInt("-1")},
	XSDNamespace + "nonNegativeInteger": {min: bigInt("0")},
	XSDNamespace + "positiveInteger":    {min: bigInt("1")},
	XSDNamespace + "long":               {min: bigInt("-9223372036854775808"), max: bigInt("9223372036854775807")},
	XSDNamespace + "int":                {min: bigInt("-2147483648"), max: bigInt("2147483647")},
	XSDNamespace + "short":              {min: bigInt("-32768"), max: bigInt("32767")},
	XSDNamespace + "byte":               {min: bigInt("-128"), max: bigInt("127")},
	XSDNamespace + "unsignedLong":       {min: bigInt("0"), max: bigInt("18446744073709551615")},
	XSDNamespace + "unsignedInt":        {min: bigInt("0"), max: bigInt("4294967295")},
	XSDNamespace + "unsignedShort":      {min: bigInt("0"), max: bigInt("65535")},
	XSDNamespace + "unsignedByte":       {min: bigInt("0"), max: bigInt("255")},
}

func (l Literal) valueEqual(other Literal) bool {
	if l.datatype != other.datatype {
		return false
	}
	a, b := l.lexical, other.lexical
	dt := l.datatype.Value()

	if r, ok := integerDatatypes[dt]; ok {
		x, okA := parseXSDInteger(a, r)
		y, okB := parseXSDInteger(b, r)
		return sameValue(okA, okB, a, b, func() bool { return x.Cmp(y) == 0 })
	}

	switch l.datatype {
	case XSDDecimal:
		x, okA := parseXSDDecimal(a)
		y, okB := parseXSDDecimal(b)
		return sameValue(okA, okB, a, b, func() bool { return x.Cmp(y) == 0 })
	case XSDDouble, XSDFloat:
		bits := 64
		if l.datatype == XSDFloat {
			bits = 32
		}
		x, okA := parseXSDFloat(a, bits)
		y, okB := parseXSDFloat(b, bits)
		// Floating-point equality: NaN is not equal to itself and 0 equals -0
		// (XSD 1.1 §3.3.4.1, the same as SPARQL's "=").
		return sameValue(okA, okB, a, b, func() bool { return x == y })
	case XSDBoolean:
		x, okA := parseXSDBoolean(a)
		y, okB := parseXSDBoolean(b)
		return sameValue(okA, okB, a, b, func() bool { return x == y })
	case XSDDateTime:
		x, okA := parseXSDDateTime(a)
		y, okB := parseXSDDateTime(b)
		return sameValue(okA, okB, a, b, func() bool { return x.equal(y) })
	case RDFLangString, RDFDirLangString:
		return a == b && strings.EqualFold(l.lang, other.lang) && l.dir == other.dir
	}
	return a == b
}

// sameValue combines the parse results of two lexical forms: two valid values
// are compared by eq, two ill-typed forms only by identity, and a valid value
// never equals an ill-typed one.
func sameValue(okA, okB bool, a, b string, eq func() bool) bool {
	switch {
	case okA && okB:
		return eq()
	case !okA && !okB:
		return a == b
	default:
		return false
	}
}

// parseXSDInteger accepts the xsd:integer lexical space [\-+]?[0-9]+ and the
// datatype's value range.
func parseXSDInteger(s string, r integerRange) (*big.Int, bool) {
	digits := strings.TrimLeft(s, "+-")
	if len(s)-len(digits) > 1 || !allDigits(digits) {
		return nil, false
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return nil, false
	}
	if (r.min != nil && v.Cmp(r.min) < 0) || (r.max != nil && v.Cmp(r.max) > 0) {
		return nil, false
	}
	return v, true
}

// parseXSDDecimal accepts (\+|-)?([0-9]+(\.[0-9]*)?|\.[0-9]+) and returns the
// exact value. Comparing through float64 made 0.1 equal 0.10000000000000000001.
func parseXSDDecimal(s string) (*big.Rat, bool) {
	body := s
	if body != "" && (body[0] == '+' || body[0] == '-') {
		body = body[1:]
	}
	intPart, frac, hasDot := strings.Cut(body, ".")
	if !allDigits(intPart) && intPart != "" || !allDigits(frac) && frac != "" {
		return nil, false
	}
	if intPart == "" && (frac == "" || !hasDot) {
		return nil, false
	}
	v, ok := new(big.Rat).SetString(s)
	return v, ok
}

// parseXSDFloat accepts the xsd:double / xsd:float lexical space: a decimal
// mantissa with an optional exponent, or INF, +INF, -INF, NaN. strconv alone
// also accepts "inf", "Infinity" and hex floats, which are ill-typed.
func parseXSDFloat(s string, bitSize int) (float64, bool) {
	switch s {
	case "INF", "+INF":
		return math.Inf(1), true
	case "-INF":
		return math.Inf(-1), true
	case "NaN":
		return math.NaN(), true
	}
	mantissa, exp := s, ""
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mantissa, exp = s[:i], s[i+1:]
		if exp != "" && (exp[0] == '+' || exp[0] == '-') {
			exp = exp[1:]
		}
		if exp == "" || !allDigits(exp) {
			return 0, false
		}
	}
	if _, ok := parseXSDDecimal(mantissa); !ok {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, bitSize)
	if err != nil {
		// Out of range rounds to ±INF / 0 per XSD 1.1 §3.3.4.2; strconv reports
		// that as ErrRange with the rounded value.
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return v, true
		}
		return 0, false
	}
	return v, true
}

// parseXSDBoolean accepts exactly the XSD boolean lexical space: true, false,
// 1, 0. strconv.ParseBool also accepts "T", "TRUE", "t", which are ill-typed.
func parseXSDBoolean(s string) (bool, bool) {
	switch s {
	case "true", "1":
		return true, true
	case "false", "0":
		return false, true
	}
	return false, false
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// dateTimeValue is a point on the XSD timeline: days since 0000-03-01 in the
// proleptic Gregorian calendar, seconds into the day, and the fractional
// seconds as a digit string without trailing zeros. A timezoned value is
// normalized to UTC.
type dateTimeValue struct {
	days     int64
	seconds  int64
	frac     string
	timezone bool
}

// equal compares two dateTime values. Values with and without a timezone are
// never equal: XSD 1.1 §3.3.7.2 makes their order indeterminate unless they are
// more than 14 hours apart, and SPARQL raises a type error for "=" on such a
// pair. Returning false keeps ValueEqual a total function.
func (v dateTimeValue) equal(o dateTimeValue) bool {
	return v.timezone == o.timezone && v.days == o.days && v.seconds == o.seconds && v.frac == o.frac
}

// parseXSDDateTime parses the XSD 1.1 xsd:dateTime lexical space (§3.3.7):
// -?YYYY-MM-DDThh:mm:ss(.s+)?(Z|(+|-)hh:mm)?, including 24:00:00 as the first
// instant of the next day. Years longer than 12 digits are treated as
// ill-typed so the day arithmetic cannot overflow.
func parseXSDDateTime(s string) (dateTimeValue, bool) {
	var v dateTimeValue
	rest := s
	neg := false
	if strings.HasPrefix(rest, "-") {
		neg, rest = true, rest[1:]
	}
	yearStr, rest, ok := strings.Cut(rest, "-")
	if !ok || len(yearStr) < 4 || len(yearStr) > 12 || !allDigits(yearStr) || (len(yearStr) > 4 && yearStr[0] == '0') {
		return v, false
	}
	year, _ := strconv.ParseInt(yearStr, 10, 64)
	if neg {
		year = -year
	}
	// MM-DDThh:mm:ss
	if len(rest) < 14 || rest[2] != '-' || rest[5] != 'T' || rest[8] != ':' || rest[11] != ':' {
		return v, false
	}
	month, ok1 := twoDigits(rest[0:2])
	day, ok2 := twoDigits(rest[3:5])
	hour, ok3 := twoDigits(rest[6:8])
	minute, ok4 := twoDigits(rest[9:11])
	second, ok5 := twoDigits(rest[12:14])
	if !(ok1 && ok2 && ok3 && ok4 && ok5) {
		return v, false
	}
	rest = rest[14:]
	if strings.HasPrefix(rest, ".") {
		i := 1
		for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
			i++
		}
		if i == 1 {
			return v, false
		}
		v.frac = strings.TrimRight(rest[1:i], "0")
		rest = rest[i:]
	}
	if month < 1 || month > 12 || day < 1 || day > daysInMonth(year, month) || minute > 59 || second > 59 {
		return v, false
	}
	if hour == 24 {
		if minute != 0 || second != 0 || v.frac != "" {
			return v, false
		}
	} else if hour > 23 {
		return v, false
	}
	offset := int64(0)
	switch {
	case rest == "":
	case rest == "Z":
		v.timezone = true
	case len(rest) == 6 && (rest[0] == '+' || rest[0] == '-') && rest[3] == ':':
		th, okH := twoDigits(rest[1:3])
		tm, okM := twoDigits(rest[4:6])
		if !okH || !okM || tm > 59 || th > 14 || (th == 14 && tm != 0) {
			return v, false
		}
		offset = int64(th*60+tm) * 60
		if rest[0] == '-' {
			offset = -offset
		}
		v.timezone = true
	default:
		return v, false
	}
	v.days = daysFromCivil(year, month, day)
	v.seconds = int64(hour*3600+minute*60+second) - offset
	// Carry into days so equal instants have one representation.
	v.days += floorDiv(v.seconds, 86400)
	v.seconds -= floorDiv(v.seconds, 86400) * 86400
	return v, true
}

func twoDigits(s string) (int, bool) {
	if len(s) != 2 || !allDigits(s) {
		return 0, false
	}
	return int(s[0]-'0')*10 + int(s[1]-'0'), true
}

func isLeapYear(y int64) bool {
	return y%4 == 0 && (y%100 != 0 || y%400 == 0)
}

func daysInMonth(y int64, m int) int {
	switch m {
	case 2:
		if isLeapYear(y) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	}
	return 31
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

// daysFromCivil counts days from 0000-03-01 in the proleptic Gregorian
// calendar (H. Hinnant's algorithm). XSD 1.1 years are astronomical: 0000 is
// 1 BCE.
func daysFromCivil(y int64, m, d int) int64 {
	if m <= 2 {
		y--
	}
	era := floorDiv(y, 400)
	yoe := y - era*400
	mp := int64((m + 9) % 12)
	doy := (153*mp+2)/5 + int64(d) - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146097 + doe
}
