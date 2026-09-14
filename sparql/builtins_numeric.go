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
	if n.kind == numInteger {
		n.i = new(big.Int).Abs(n.i)
	}
	n.f = math.Abs(n.f)
	return numericLiteral(n)
}

// roundLike applies ROUND, CEIL or FLOOR. An integer is returned unchanged;
// other types keep their type (XPath fn:round, fn:ceiling, fn:floor).
func roundLike(t rdflibgo.Term, f func(float64) float64) rdflibgo.Term {
	n, ok := numericOf(t)
	if !ok {
		return nil
	}
	if n.kind != numInteger {
		n.f = f(n.f)
	}
	return numericLiteral(n)
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

func mathCeil(f float64) float64  { return math.Ceil(f) }
func mathFloor(f float64) float64 { return math.Floor(f) }

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
