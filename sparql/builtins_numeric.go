package sparql

import (
	"math"
	"math/big"

	rdflibgo "github.com/tggo/goRDFlib"
)

// Numeric functions of SPARQL 1.1 §17.4.4 and date/time functions of §17.4.5.

// fnAbs returns a value of the argument's (promoted) numeric type.
func fnAbs(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	n, ok := numericOf(a[0])
	if !ok {
		return nil
	}
	switch n.kind {
	case numInteger:
		n.i = new(big.Int).Abs(n.i)
	case numDecimal:
		n.r = new(big.Rat).Abs(n.exact())
	}
	n.f = math.Abs(n.f)
	return numericLiteral(n)
}

// roundMode names ROUND, CEIL or FLOOR. It carries the float function and
// selects the exact one for a decimal.
type roundMode int

const (
	roundHalfUp roundMode = iota // XPath fn:round
	roundCeil
	roundFloor
)

func (m roundMode) float(f float64) float64 {
	switch m {
	case roundCeil:
		return math.Ceil(f)
	case roundFloor:
		return math.Floor(f)
	}
	return xpathRound(f)
}

// roundLike applies ROUND, CEIL or FLOOR. An integer is returned unchanged;
// other types keep their type (XPath fn:round, fn:ceiling, fn:floor). A
// decimal is rounded exactly, on the rational: it may carry more digits than
// a float64 holds.
func roundLike(t rdflibgo.Term, m roundMode) rdflibgo.Term {
	n, ok := numericOf(t)
	if !ok {
		return nil
	}
	switch n.kind {
	case numInteger:
	case numDecimal:
		n.r = roundRat(n.exact(), m)
	default:
		n.f = m.float(n.f)
	}
	return numericLiteral(n)
}

// roundRat rounds r to an integer exactly.
func roundRat(r *big.Rat, m roundMode) *big.Rat {
	floor := func(x *big.Rat) *big.Int {
		q, rem := new(big.Int).QuoRem(x.Num(), x.Denom(), new(big.Int))
		if rem.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		}
		return q
	}
	var i *big.Int
	switch m {
	case roundFloor:
		i = floor(r)
	case roundCeil:
		i = new(big.Int).Neg(floor(new(big.Rat).Neg(r)))
	default: // half towards positive infinity
		i = floor(new(big.Rat).Add(r, big.NewRat(1, 2)))
	}
	return new(big.Rat).SetInt(i)
}

// xpathRound rounds half towards positive infinity, as XPath fn:round does:
// round(2.5) is 3 and round(-2.5) is -2. math.Round rounds half away from
// zero instead.
func xpathRound(f float64) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return f
	}
	return math.Floor(f + 0.5)
}

// dateTimeArg returns the lexical form of a literal whose datatype a date/time
// accessor accepts: xsd:dateTime always, xsd:date for YEAR/MONTH/DAY and
// xsd:time for HOURS/MINUTES/SECONDS.
func dateTimeArg(t rdflibgo.Term, part string) (string, bool) {
	l, ok := t.(rdflibgo.Literal)
	if !ok {
		return "", false
	}
	switch l.Datatype() {
	case rdflibgo.XSDDateTime:
		return l.Lexical(), true
	case rdflibgo.XSDDate:
		return l.Lexical(), part == "year" || part == "month" || part == "day" || part == "tz"
	case rdflibgo.XSDTime:
		return l.Lexical(), part == "hours" || part == "minutes" || part == "seconds" || part == "tz"
	}
	return "", false
}

func datePart(t rdflibgo.Term, part string) rdflibgo.Term {
	lex, ok := dateTimeArg(t, part)
	if !ok {
		return nil
	}
	if l := t.(rdflibgo.Literal); l.Datatype() == rdflibgo.XSDTime {
		lex = "1970-01-01T" + lex
	}
	v, ok := extractDatePart(lex, part)
	if !ok {
		return nil
	}
	dt := rdflibgo.XSDInteger
	if part == "seconds" {
		dt = rdflibgo.XSDDecimal
	}
	return rdflibgo.NewLiteral(v, rdflibgo.WithDatatype(dt))
}

func fnTimezone(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	lex, ok := dateTimeArg(a[0], "tz")
	if !ok {
		return nil
	}
	tz, ok := extractTimezone(lex)
	if !ok {
		return nil // §17.4.5.8: no timezone is an error
	}
	return rdflibgo.NewLiteral(tz, rdflibgo.WithDatatype(xsd("dayTimeDuration")))
}

func fnTZ(a []rdflibgo.Term, _ map[string]string) rdflibgo.Term {
	lex, ok := dateTimeArg(a[0], "tz")
	if !ok {
		return nil
	}
	tz, _ := extractTZ(lex)
	return rdflibgo.NewLiteral(tz)
}
