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

// integerRange is the value range of a type derived from xsd:integer; a nil
// bound is unbounded.
type integerRange struct{ min, max *big.Int }

func bigPow2(n uint) *big.Int { return new(big.Int).Lsh(big.NewInt(1), n) }

func signedRange(bits uint) integerRange {
	max := new(big.Int).Sub(bigPow2(bits-1), big.NewInt(1))
	return integerRange{min: new(big.Int).Neg(bigPow2(bits - 1)), max: max}
}

func unsignedRange(bits uint) integerRange {
	return integerRange{min: big.NewInt(0), max: new(big.Int).Sub(bigPow2(bits), big.NewInt(1))}
}

// integerTypes lists xsd:integer and the types derived from it that SPARQL 1.1
// §17.1 names as numeric, with their XSD value ranges. A literal outside its
// type's range ("1200"^^xsd:byte) is ill-typed.
var integerTypes = map[rdflibgo.URIRef]integerRange{
	rdflibgo.XSDInteger:       {},
	xsd("nonPositiveInteger"): {max: big.NewInt(0)},
	xsd("negativeInteger"):    {max: big.NewInt(-1)},
	rdflibgo.XSDLong:          signedRange(64),
	rdflibgo.XSDInt:           signedRange(32),
	xsd("short"):              signedRange(16),
	xsd("byte"):               signedRange(8),
	xsd("nonNegativeInteger"): {min: big.NewInt(0)},
	xsd("unsignedLong"):       unsignedRange(64),
	xsd("unsignedInt"):        unsignedRange(32),
	xsd("unsignedShort"):      unsignedRange(16),
	xsd("unsignedByte"):       unsignedRange(8),
	xsd("positiveInteger"):    {min: big.NewInt(1)},
}

func xsd(local string) rdflibgo.URIRef {
	return rdflibgo.NewURIRefUnsafe(rdflibgo.XSDNamespace + local)
}

// numericKind returns the promotion kind of a numeric datatype, and false for
// a datatype that is not numeric. Every type derived from xsd:integer promotes
// to xsd:integer (§17.3, XPath 2.0 §B.1 type promotion).
func numericKind(dt rdflibgo.URIRef) (numKind, bool) {
	if _, ok := integerTypes[dt]; ok {
		return numInteger, true
	}
	switch dt {
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
	r    *big.Rat // the exact value when kind == numDecimal
}

// exact returns the exact value of an integer or decimal. xsd:decimal is
// arbitrary precision (XSD 1.1 §3.3.3); float64 kept 16 significant digits,
// so 12345678901234567.5 + 1 came out as ...68.0.
func (n numeric) exact() *big.Rat {
	if n.r != nil {
		return n.r
	}
	if n.i != nil {
		return new(big.Rat).SetInt(n.i)
	}
	r, _ := new(big.Rat).SetString(strconv.FormatFloat(n.f, 'f', -1, 64))
	return r
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
		r := integerTypes[l.Datatype()]
		if (r.min != nil && i.Cmp(r.min) < 0) || (r.max != nil && i.Cmp(r.max) > 0) {
			return numeric{}, false
		}
		f, _ := new(big.Float).SetInt(i).Float64()
		return numeric{kind: numInteger, f: f, i: i}, true
	case numDecimal:
		if !isDecimalLexical(lex) {
			return numeric{}, false
		}
		r, ok := new(big.Rat).SetString(decimalForRat(lex))
		if !ok {
			return numeric{}, false
		}
		f, _ := r.Float64()
		return numeric{kind: numDecimal, f: f, r: r}, true
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

// numericLiteral builds the result of a numeric operation of the given kind.
// It returns nil (an error) for a decimal that is not finite, which XPath
// op:numeric-divide raises for division by zero.
func numericLiteral(n numeric) rdflibgo.Term {
	switch n.kind {
	case numInteger:
		return rdflibgo.NewLiteral(n.i.String(), rdflibgo.WithDatatype(rdflibgo.XSDInteger))
	case numDecimal:
		if n.r == nil && (math.IsNaN(n.f) || math.IsInf(n.f, 0)) {
			return nil
		}
		return rdflibgo.NewLiteral(formatRat(n.exact()), rdflibgo.WithDatatype(rdflibgo.XSDDecimal))
	case numFloat:
		return rdflibgo.NewLiteral(formatFloatLexical(float64(float32(n.f)), 32), rdflibgo.WithDatatype(rdflibgo.XSDFloat))
	default:
		return rdflibgo.NewLiteral(formatFloatLexical(n.f, 64), rdflibgo.WithDatatype(rdflibgo.XSDDouble))
	}
}

// formatFloatLexical writes an xsd:float / xsd:double lexical form. Go's own
// "+Inf" and "NaN" spellings are replaced by the XSD ones.
func formatFloatLexical(f float64, bits int) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "INF"
	case math.IsInf(f, -1):
		return "-INF"
	}
	return strconv.FormatFloat(f, 'g', -1, bits)
}

// numericArithmetic applies +, -, * or / after type promotion (§17.3): the
// result has the kind of the wider operand, integer / integer is a decimal,
// and division by zero is an error for integers and decimals but INF or NaN
// for floats and doubles (XPath op:numeric-divide).
func numericArithmetic(op string, a, b numeric) rdflibgo.Term {
	kind := max(a.kind, b.kind)
	if op == "/" && kind == numInteger {
		kind = numDecimal
	}
	if kind == numInteger {
		r := new(big.Int)
		switch op {
		case "+":
			r.Add(a.i, b.i)
		case "-":
			r.Sub(a.i, b.i)
		case "*":
			r.Mul(a.i, b.i)
		}
		return numericLiteral(numeric{kind: numInteger, i: r})
	}
	if kind == numDecimal {
		x, y := a.exact(), b.exact()
		r := new(big.Rat)
		switch op {
		case "+":
			r.Add(x, y)
		case "-":
			r.Sub(x, y)
		case "*":
			r.Mul(x, y)
		case "/":
			if y.Sign() == 0 {
				return nil
			}
			r.Quo(x, y)
		}
		return numericLiteral(numeric{kind: numDecimal, r: r})
	}
	var r float64
	switch op {
	case "+":
		r = a.f + b.f
	case "-":
		r = a.f - b.f
	case "*":
		r = a.f * b.f
	case "/":
		r = a.f / b.f
	}
	return numericLiteral(numeric{kind: kind, f: r})
}

// decimalDigits is how many fractional digits a decimal that has no finite
// expansion (1/3) is written with. XSD leaves it to the implementation (at
// least 18 digits of the value must be supported); Oxigraph writes 18.
const decimalDigits = 18

// decimalForRat adapts an xsd:decimal lexical form for big.Rat.SetString,
// which rejects a leading '+' and a trailing '.' ("1." is a valid decimal).
func decimalForRat(lex string) string {
	lex = strings.TrimPrefix(lex, "+")
	if strings.HasSuffix(lex, ".") {
		lex += "0"
	}
	return lex
}

// formatRat writes r in the canonical xsd:decimal form of XSD 1.1
// (§3.3.3.2): no leading '+', no leading or trailing zeros, and no decimal
// point for an integral value ("178", not "178.0"). A value with no finite
// decimal expansion is rounded to decimalDigits fractional digits.
func formatRat(r *big.Rat) string {
	digits := decimalDigits
	if d := finiteDigits(r.Denom()); d >= 0 {
		digits = d
	}
	s := r.FloatString(digits)
	if strings.Contains(s, ".") {
		s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	}
	if s == "-0" {
		s = "0"
	}
	return s
}

// finiteDigits returns the number of fractional digits of a fraction with
// denominator d, or -1 when it has no finite decimal expansion (d has a
// prime factor other than 2 and 5).
func finiteDigits(d *big.Int) int {
	two, five := big.NewInt(2), big.NewInt(5)
	n := new(big.Int).Set(d)
	m := new(big.Int)
	var twos, fives int
	for n.Cmp(big.NewInt(1)) > 0 {
		if m.Mod(n, two).Sign() == 0 {
			n.Quo(n, two)
			twos++
		} else if m.Mod(n, five).Sign() == 0 {
			n.Quo(n, five)
			fives++
		} else {
			return -1
		}
	}
	return max(twos, fives)
}
