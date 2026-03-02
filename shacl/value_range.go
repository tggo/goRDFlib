package shacl

import (
	"math/big"
	"strings"
	"time"
)

// MinExclusiveConstraint implements sh:minExclusive.
type MinExclusiveConstraint struct{ Value Term }

// MinInclusiveConstraint implements sh:minInclusive.
type MinInclusiveConstraint struct{ Value Term }

// MaxExclusiveConstraint implements sh:maxExclusive.
type MaxExclusiveConstraint struct{ Value Term }

// MaxInclusiveConstraint implements sh:maxInclusive.
type MaxInclusiveConstraint struct{ Value Term }

func (c *MinExclusiveConstraint) ComponentIRI() string {
	return SH + "MinExclusiveConstraintComponent"
}
func (c *MinInclusiveConstraint) ComponentIRI() string {
	return SH + "MinInclusiveConstraintComponent"
}
func (c *MaxExclusiveConstraint) ComponentIRI() string {
	return SH + "MaxExclusiveConstraintComponent"
}
func (c *MaxInclusiveConstraint) ComponentIRI() string {
	return SH + "MaxInclusiveConstraintComponent"
}

func (c *MinExclusiveConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	return compareAll(shape, focusNode, valueNodes, c.Value, c.ComponentIRI(), func(cmp int) bool { return cmp > 0 })
}
func (c *MinInclusiveConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	return compareAll(shape, focusNode, valueNodes, c.Value, c.ComponentIRI(), func(cmp int) bool { return cmp >= 0 })
}
func (c *MaxExclusiveConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	return compareAll(shape, focusNode, valueNodes, c.Value, c.ComponentIRI(), func(cmp int) bool { return cmp < 0 })
}
func (c *MaxInclusiveConstraint) Evaluate(ctx *evalContext, shape *Shape, focusNode Term, valueNodes []Term) []ValidationResult {
	return compareAll(shape, focusNode, valueNodes, c.Value, c.ComponentIRI(), func(cmp int) bool { return cmp <= 0 })
}

func compareAll(shape *Shape, focusNode Term, valueNodes []Term, threshold Term, component string, valid func(int) bool) []ValidationResult {
	var results []ValidationResult
	for _, vn := range valueNodes {
		cmp, ok := compareLiterals(vn, threshold)
		if !ok || !valid(cmp) {
			results = append(results, makeResult(shape, focusNode, vn, component))
		}
	}
	return results
}

// compareLiterals compares two literal terms. Returns (cmp, ok) where cmp < 0 means a < b.
func compareLiterals(a, b Term) (int, bool) {
	if !a.IsLiteral() || !b.IsLiteral() {
		return 0, false
	}
	aDt := a.Datatype()
	bDt := b.Datatype()

	if isNumericType(aDt) && isNumericType(bDt) {
		return compareNumeric(a.Value(), b.Value())
	}

	if isDateType(aDt) && isDateType(bDt) {
		return compareDates(a.Value(), b.Value())
	}

	if aDt == bDt {
		return strings.Compare(a.Value(), b.Value()), true
	}

	return 0, false
}

func isNumericType(dt string) bool {
	switch dt {
	case XSD + "integer", XSD + "decimal", XSD + "float", XSD + "double",
		XSD + "int", XSD + "long", XSD + "short", XSD + "byte",
		XSD + "nonNegativeInteger", XSD + "positiveInteger",
		XSD + "nonPositiveInteger", XSD + "negativeInteger",
		XSD + "unsignedInt", XSD + "unsignedLong", XSD + "unsignedShort", XSD + "unsignedByte":
		return true
	}
	return false
}

func isDateType(dt string) bool {
	return dt == XSD+"date" || dt == XSD+"dateTime"
}

func compareNumeric(a, b string) (int, bool) {
	ra, ok1 := new(big.Rat).SetString(a)
	rb, ok2 := new(big.Rat).SetString(b)
	if !ok1 || !ok2 {
		return 0, false
	}
	return ra.Cmp(rb), true
}

func compareDates(a, b string) (int, bool) {
	// Per XSD spec, dateTimes with and without timezone are incomparable
	if hasTimezone(a) != hasTimezone(b) {
		return 0, false
	}

	formats := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02Z07:00",
		"2006-01-02",
	}
	ta, ok1 := parseTime(a, formats)
	tb, ok2 := parseTime(b, formats)
	if !ok1 || !ok2 {
		return 0, false
	}
	if ta.Before(tb) {
		return -1, true
	}
	if ta.After(tb) {
		return 1, true
	}
	return 0, true
}

func parseTime(s string, formats []string) (time.Time, bool) {
	for _, f := range formats {
		t, err := time.Parse(f, s)
		if err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func hasTimezone(s string) bool {
	if strings.HasSuffix(s, "Z") {
		return true
	}
	if len(s) >= 6 {
		c := s[len(s)-6]
		if c == '+' || c == '-' {
			return true
		}
	}
	return false
}
