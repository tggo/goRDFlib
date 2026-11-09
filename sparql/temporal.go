package sparql

import (
	"strings"
	"time"

	rdflibgo "github.com/tggo/goRDFlib"
)

// temporal is a parsed xsd:dateTime, xsd:date or xsd:time. t is the instant
// when hasTZ is set; otherwise it is the wall-clock reading placed on the UTC
// timeline, which is only meaningful against another value without a timezone.
type temporal struct {
	t     time.Time
	hasTZ bool
}

// hasTimezone reports whether a date, dateTime or time lexical form ends in a
// timezone: Z or (+|-)hh:mm (XSD 1.1 section 3.3.7.1).
func hasTimezone(s string) bool {
	if strings.HasSuffix(s, "Z") {
		return true
	}
	n := len(s)
	return n >= 6 && (s[n-6] == '+' || s[n-6] == '-') && s[n-3] == ':'
}

func parseTemporal(l rdflibgo.Literal) (temporal, bool) {
	t, ok := parseDateTime(l.Lexical(), l.Datatype())
	if !ok {
		return temporal{}, false
	}
	return temporal{t: t, hasTZ: hasTimezone(l.Lexical())}, true
}

// maxTimezoneOffset is the largest offset XSD allows (+-14:00).
const maxTimezoneOffset = 14 * time.Hour

// compareTemporal orders two values of one temporal datatype. Two timezoned
// values, or two without, compare directly. A timezoned value against one
// without has no fixed order: the untimezoned value could lie anywhere within
// +-14 hours of its reading, so the order is determined only when the
// timezoned value lies outside that window (XSD 1.1 section 3.3.7.2; XPath
// F&O would use the implicit timezone instead, which SPARQL leaves undefined).
// determinate is false when the order is not fixed; cmp is then the order of
// the raw readings, usable for a total order such as ORDER BY but not for "=".
func compareTemporal(a, b temporal) (cmp int, determinate bool) {
	cmp = a.t.Compare(b.t)
	if a.hasTZ == b.hasTZ {
		return cmp, true
	}
	if a.hasTZ {
		c, det := compareTemporal(b, a)
		return -c, det
	}
	// a has no timezone, b has one: b's instant against a's reading +- 14h.
	switch {
	case b.t.Before(a.t.Add(-maxTimezoneOffset)):
		return cmp, true // b is earlier than a under every timezone
	case b.t.After(a.t.Add(maxTimezoneOffset)):
		return cmp, true
	}
	return cmp, false
}
