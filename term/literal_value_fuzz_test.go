package term

import (
	"math"
	"strings"
	"testing"
)

// refValueEqual is valueEqual without its fast paths (the identical-lexical
// shortcut and the int64 integer path): every value goes through the exact
// big.Int / big.Rat / float parse. The fast paths must agree with it on every
// input.
func refValueEqual(l, other Literal) bool {
	if l.datatype != other.datatype {
		return false
	}
	a, b := l.lexical, other.lexical
	if r, ok := integerDatatypes[l.datatype.Value()]; ok {
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

// valueEqualDatatypes are the datatypes valueEqual treats specially, plus one
// it does not.
var valueEqualDatatypes = func() []URIRef {
	dts := []URIRef{XSDDecimal, XSDDouble, XSDFloat, XSDBoolean, XSDDateTime, XSDString}
	for dt := range integerDatatypes {
		dts = append(dts, NewURIRefUnsafe(dt))
	}
	return dts
}()

// FuzzValueEqualFastPaths checks valueEqual against refValueEqual, and
// smallXSDInteger against parseXSDInteger, for arbitrary lexical forms.
func FuzzValueEqualFastPaths(f *testing.F) {
	for _, s := range []string{
		"0", "-0", "+0", "01", "1", "+1", "-1", "+-1", "", "+", "-",
		"127", "128", "-128", "-129", "255", "256", "32767", "2147483647", "2147483648",
		"999999999999999999", "1000000000000000000", "-999999999999999999",
		"9223372036854775807", "9223372036854775808", "-9223372036854775808",
		"18446744073709551615", "18446744073709551616", "000000000000000000001",
		"1.0", "1.", ".5", "1e3", "NaN", "INF", "-INF", "true", "1", "false",
		"2020-01-01T00:00:00Z", "2020-01-01T01:00:00+01:00", " 1", "1 ",
	} {
		f.Add(s, s)
		f.Add(s, "1")
		f.Add("0"+s, s)
	}
	f.Fuzz(func(t *testing.T, a, b string) {
		for _, dt := range valueEqualDatatypes {
			la := NewLiteral(a, WithDatatype(dt))
			lb := NewLiteral(b, WithDatatype(dt))
			if got, want := la.valueEqual(lb), refValueEqual(la, lb); got != want {
				t.Fatalf("%q vs %q ^^%s: valueEqual %v, reference %v", a, b, dt.Value(), got, want)
			}
			if r, ok := integerDatatypes[dt.Value()]; ok {
				v, okSmall, small := smallXSDInteger(a, r)
				big, okBig := parseXSDInteger(a, r)
				if small && okSmall != okBig {
					t.Fatalf("%q ^^%s: small path ok=%v, big path ok=%v", a, dt.Value(), okSmall, okBig)
				}
				if small && okSmall && (!big.IsInt64() || big.Int64() != v) {
					t.Fatalf("%q ^^%s: small path %d, big path %s", a, dt.Value(), v, big)
				}
			}
		}
	})
}

// The bounds smallXSDInteger compares against are the big.Int bounds clamped
// to int64; an unsignedLong's maximum does not fit and must clamp, not wrap.
func TestIntegerRangeInt64Bounds(t *testing.T) {
	for dt, r := range integerDatatypes {
		if r.min == nil && r.lo != math.MinInt64 || r.max == nil && r.hi != math.MaxInt64 {
			t.Errorf("%s: unbounded side not clamped: lo=%d hi=%d", dt, r.lo, r.hi)
		}
		if r.max != nil && !r.max.IsInt64() && r.hi != math.MaxInt64 {
			t.Errorf("%s: max %s does not fit int64 but hi=%d", dt, r.max, r.hi)
		}
	}
}
