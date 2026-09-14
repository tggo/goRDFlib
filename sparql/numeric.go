package sparql

import (
	"math"
	"math/big"
	"strconv"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
)

// numKind is the position of a numeric type in the SPARQL 1.1 §17.3 type
// promotion order: integer → decimal → float → double.
type numKind int

const (
	numInteger numKind = iota
	numDecimal
	numFloat
	numDouble
)

// numericKind returns the promotion kind of a numeric datatype, and false for
// a datatype that is not numeric.
func numericKind(dt rdflibgo.URIRef) (numKind, bool) {
	switch dt {
	case rdflibgo.XSDInteger, rdflibgo.XSDInt, rdflibgo.XSDLong:
		return numInteger, true
	case rdflibgo.XSDDecimal:
		return numDecimal, true
	case rdflibgo.XSDFloat:
		return numFloat, true
	case rdflibgo.XSDDouble:
		return numDouble, true
	}
	return 0, false
}

func isNumericDatatype(dt rdflibgo.URIRef) bool {
	_, ok := numericKind(dt)
	return ok
}

// numeric is the value of a well-formed numeric literal.
type numeric struct {
	kind numKind
	f    float64  // the value, for every kind
	i    *big.Int // the exact value when kind == numInteger
}

// numericOf returns the value of a numeric literal. It returns false when t is
// not a literal with a numeric datatype, or when its lexical form is not in the
// lexical space of that datatype ("abc"^^xsd:integer). Such a literal is
// ill-typed: SPARQL 1.1 §17.3 has no operator mapping for it, so every caller
// must treat false as a type error rather than read the value as 0.
func numericOf(t rdflibgo.Term) (numeric, bool) {
	l, ok := t.(rdflibgo.Literal)
	if !ok {
		return numeric{}, false
	}
	kind, ok := numericKind(l.Datatype())
	if !ok {
		return numeric{}, false
	}
	lex := l.Lexical()
	switch kind {
	case numInteger:
		if !isIntegerLexical(lex) {
			return numeric{}, false
		}
		i, ok := new(big.Int).SetString(strings.TrimPrefix(lex, "+"), 10)
		if !ok {
			return numeric{}, false
		}
		f, _ := new(big.Float).SetInt(i).Float64()
		return numeric{kind: numInteger, f: f, i: i}, true
	case numDecimal:
		if !isDecimalLexical(lex) {
			return numeric{}, false
		}
		f, err := strconv.ParseFloat(lex, 64)
		if err != nil && !isRangeErr(err) {
			return numeric{}, false
		}
		return numeric{kind: numDecimal, f: f}, true
	default:
		f, ok := parseDoubleLexical(lex)
		if !ok {
			return numeric{}, false
		}
		return numeric{kind: kind, f: f}, true
	}
}

func isRangeErr(err error) bool {
	ne, ok := err.(*strconv.NumError)
	return ok && ne.Err == strconv.ErrRange
}

func isDigits(s string) bool {
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

func trimSign(s string) string {
	if s != "" && (s[0] == '+' || s[0] == '-') {
		return s[1:]
	}
	return s
}

// isIntegerLexical matches the xsd:integer lexical space: [\-+]?[0-9]+.
func isIntegerLexical(s string) bool {
	return isDigits(trimSign(s))
}

// isDecimalLexical matches the xsd:decimal lexical space:
// [\-+]?([0-9]+(\.[0-9]*)?|\.[0-9]+).
func isDecimalLexical(s string) bool {
	s = trimSign(s)
	intPart, frac, hasDot := strings.Cut(s, ".")
	if !hasDot {
		return isDigits(intPart)
	}
	if intPart == "" {
		return isDigits(frac)
	}
	return isDigits(intPart) && (frac == "" || isDigits(frac))
}

// parseDoubleLexical parses the xsd:float / xsd:double lexical space:
// a decimal mantissa with an optional exponent, or INF, +INF, -INF, NaN.
// strconv.ParseFloat alone is too lenient: it accepts "inf", "Infinity",
// hexadecimal mantissas and underscores.
func parseDoubleLexical(s string) (float64, bool) {
	switch s {
	case "INF", "+INF":
		return math.Inf(1), true
	case "-INF":
		return math.Inf(-1), true
	case "NaN":
		return math.NaN(), true
	}
	mantissa, exp, hasExp := strings.Cut(strings.Replace(s, "E", "e", 1), "e")
	if !isDecimalLexical(mantissa) || (hasExp && !isIntegerLexical(exp)) {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil && !isRangeErr(err) {
		return 0, false
	}
	return f, true
}
