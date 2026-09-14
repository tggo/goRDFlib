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
		fa, errA := strconv.ParseFloat(la.Lexical(), 64)
		fb, errB := strconv.ParseFloat(lb.Lexical(), 64)
		if errA == nil && errB == nil && isNumericDatatype(la.Datatype()) && isNumericDatatype(lb.Datatype()) {
			if math.IsNaN(fa) || math.IsNaN(fb) {
				return strings.Compare(a.N3(), b.N3())
			}
			if fa < fb {
				return -1
			}
			if fa > fb {
				return 1
			}
			return 0
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
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
