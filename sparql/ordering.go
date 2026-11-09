package sparql

import (
	"math"
	"strconv"
	"strings"
	"time"

	rdflibgo "github.com/tggo/goRDFlib"
)

// termTypeOrder returns a numeric order for term types per SPARQL ordering:
// Blanks < IRIs < Literals < TripleTerms
func termTypeOrder(t rdflibgo.Term) int {
	switch t.(type) {
	case rdflibgo.BNode:
		return 0
	case rdflibgo.URIRef:
		return 1
	case rdflibgo.Literal:
		return 2
	case rdflibgo.TripleTerm:
		return 3
	}
	return 4
}

func compareTermValues(a, b rdflibgo.Term) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	// Different term types: compare by type order
	aOrder := termTypeOrder(a)
	bOrder := termTypeOrder(b)
	if aOrder != bOrder {
		return aOrder - bOrder
	}

	// Same term type
	la, okA := a.(rdflibgo.Literal)
	lb, okB := b.(rdflibgo.Literal)
	if okA && okB {
		// A sort compares each value many times. Integers and decimals of at
		// most 15 characters (so at most 15 digits) are exact in float64, and
		// comparing their floats is what this did before decimals were exact.
		if c, ok := compareShortNumbers(la, lb); ok {
			return c
		}
		na, okNA := numericOf(la)
		nb, okNB := numericOf(lb)
		if okNA && okNB {
			if math.IsNaN(na.f) || math.IsNaN(nb.f) {
				return strings.Compare(a.N3(), b.N3())
			}
			// Exact for integers and decimals, so ORDER BY, MIN and MAX agree
			// with = and < on values float64 cannot tell apart.
			return compareNumbers(na, nb)
		}
		// Date/dateTime comparison
		if isDateDatatype(la.Datatype()) && isDateDatatype(lb.Datatype()) {
			if ta, tb, ok := parseDatePair(la, lb); ok {
				if ta.Before(tb) {
					return -1
				}
				if ta.After(tb) {
					return 1
				}
				return 0
			}
		}
	}

	// URIs: compare by value, not N3 (to avoid angle bracket interference)
	uA, aIsURI := a.(rdflibgo.URIRef)
	uB, bIsURI := b.(rdflibgo.URIRef)
	if aIsURI && bIsURI {
		return strings.Compare(uA.Value(), uB.Value())
	}

	// Triple terms: compare component by component
	ttA, aIsTT := a.(rdflibgo.TripleTerm)
	ttB, bIsTT := b.(rdflibgo.TripleTerm)
	if aIsTT && bIsTT {
		if c := compareTermValues(ttA.Subject(), ttB.Subject()); c != 0 {
			return c
		}
		if c := compareTermValues(ttA.Predicate(), ttB.Predicate()); c != 0 {
			return c
		}
		return compareTermValues(ttA.Object(), ttB.Object())
	}

	return strings.Compare(a.N3(), b.N3())
}

func isDateDatatype(dt rdflibgo.URIRef) bool {
	return dt == rdflibgo.XSDDateTime || dt == rdflibgo.XSDDate || dt == rdflibgo.XSDTime
}

// parseDatePair attempts to parse two date/dateTime/time literals into time.Time values.
func parseDatePair(a, b rdflibgo.Literal) (time.Time, time.Time, bool) {
	ta, okA := parseDateTime(a.Lexical(), a.Datatype())
	tb, okB := parseDateTime(b.Lexical(), b.Datatype())
	if okA && okB {
		return ta, tb, true
	}
	return time.Time{}, time.Time{}, false
}

func parseDateTime(s string, dt rdflibgo.URIRef) (time.Time, bool) {
	var formats []string
	switch dt {
	case rdflibgo.XSDDateTime:
		formats = []string{
			"2006-01-02T15:04:05Z07:00",
			"2006-01-02T15:04:05",
			"2006-01-02T15:04:05.999999999Z07:00",
			"2006-01-02T15:04:05.999999999",
		}
	case rdflibgo.XSDDate:
		formats = []string{
			"2006-01-02Z07:00",
			"2006-01-02",
		}
	case rdflibgo.XSDTime:
		formats = []string{
			"15:04:05Z07:00",
			"15:04:05",
			"15:04:05.999999999Z07:00",
			"15:04:05.999999999",
		}
	default:
		return time.Time{}, false
	}
	// 24:00:00 is the end of the day: the first instant of the next one (XSD 1.1
	// section 3.3.7.1). Go's parser stops at 23:59:59.
	endOfDay := false
	if dt == rdflibgo.XSDDateTime {
		if i := strings.Index(s, "T24:00:00"); i > 0 {
			rest := s[i+len("T24:00:00"):]
			frac := 0
			if strings.HasPrefix(rest, ".") {
				frac = 1
				for frac < len(rest) && rest[frac] >= '0' && rest[frac] <= '9' {
					frac++
				}
			}
			if strings.Trim(rest[:frac], ".0") == "" {
				s = s[:i] + "T00:00:00" + rest
				endOfDay = true
			}
		}
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			if endOfDay {
				t = t.AddDate(0, 0, 1)
			}
			return t, true
		}
	}
	return time.Time{}, false
}

// compareShortNumbers orders two integer or decimal literals of at most 15
// characters by float64, which is exact for them. It declines anything else.
func compareShortNumbers(a, b rdflibgo.Literal) (int, bool) {
	la, lb := a.Lexical(), b.Lexical()
	if len(la) > 15 || len(lb) > 15 {
		return 0, false
	}
	// Only xsd:integer and xsd:decimal, and only well-formed lexical forms:
	// ParseFloat also takes "1e5", "Inf" and "1_0", which numericOf (and so
	// FILTER) rejects, and a derived type has a range to check.
	if !shortNumberLexical(a) || !shortNumberLexical(b) {
		return 0, false
	}
	fa, errA := strconv.ParseFloat(la, 64)
	fb, errB := strconv.ParseFloat(lb, 64)
	if errA != nil || errB != nil {
		return 0, false
	}
	return cmpFloat(fa, fb), true
}

func shortNumberLexical(l rdflibgo.Literal) bool {
	switch l.Datatype() {
	case rdflibgo.XSDInteger:
		return isIntegerLexical(l.Lexical())
	case rdflibgo.XSDDecimal:
		return isDecimalLexical(l.Lexical())
	}
	return false
}
