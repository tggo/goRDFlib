package results

import "strings"

// Negotiate picks a result format for an HTTP Accept header (RFC 9110
// §12.5.1) among all four formats, in the server preference order JSON, XML,
// CSV, TSV. See NegotiateAmong for the rules.
func Negotiate(accept string) (Format, bool) {
	return NegotiateAmong(accept, allFormats...)
}

// Specificity of a media range match, higher wins.
const (
	matchNone      = -1
	matchAll       = 0 // */*
	matchType      = 1 // text/*
	matchAlias     = 2 // application/json for FormatJSON
	matchCanonical = 3 // application/sparql-results+json
)

// NegotiateAmong picks one of offers for an HTTP Accept header. offers is
// the server preference order; pass FormatJSON, FormatXML for an ASK result.
//
// Each offer takes the quality of the most specific media range that matches
// it: the canonical media type, then an alias (application/json,
// application/xml, text/xml), then type/*, then */*. A quality of 0 makes the
// offer unacceptable even when a less specific range would accept it. The
// offer with the highest quality wins; ties go to the more specific match,
// then to the earlier offer.
//
// An empty Accept header accepts anything and returns the first offer.
// Media ranges that do not parse, or carry an invalid q value, are ignored.
// It returns false when nothing is acceptable, which an endpoint answers with
// 406 Not Acceptable. Media type parameters other than q are ignored.
func NegotiateAmong(accept string, offers ...Format) (Format, bool) {
	if len(offers) == 0 {
		return 0, false
	}
	if strings.TrimSpace(accept) == "" {
		return offers[0], true
	}
	ranges := parseAccept(accept)

	best := Format(0)
	bestQ, bestSpec := 0, matchNone
	for _, f := range offers {
		q, spec := 0, matchNone
		for _, r := range ranges {
			s := r.match(f)
			if s < spec || s == matchNone {
				continue
			}
			if s > spec || r.q > q {
				q = r.q
			}
			spec = s
		}
		if spec == matchNone || q == 0 {
			continue
		}
		if q > bestQ || (q == bestQ && spec > bestSpec) {
			best, bestQ, bestSpec = f, q, spec
		}
	}
	return best, bestQ > 0
}

// mediaRange is one element of an Accept header. q is the quality in
// thousandths (0..1000), which keeps comparisons exact.
type mediaRange struct {
	typ, sub string
	full     string // typ + "/" + sub
	q        int
}

func (r mediaRange) match(f Format) int {
	mt := f.MediaType()
	if mt == "" {
		return matchNone
	}
	if r.typ == "*" {
		return matchAll
	}
	full := r.full
	if full == mt {
		return matchCanonical
	}
	if a, ok := aliases[full]; ok && a == f {
		return matchAlias
	}
	if r.sub == "*" && strings.HasPrefix(mt, r.typ+"/") {
		return matchType
	}
	return matchNone
}

// parseAccept splits an Accept header into media ranges. Commas and
// semicolons inside quoted parameter values do not split.
func parseAccept(accept string) []mediaRange {
	var out []mediaRange
	for _, elem := range splitQuoted(accept, ',') {
		parts := splitQuoted(elem, ';')
		typ, sub, ok := strings.Cut(strings.ToLower(strings.TrimSpace(parts[0])), "/")
		if !ok || !isToken(typ) || !isToken(sub) || (typ == "*" && sub != "*") {
			continue
		}
		r := mediaRange{typ: typ, sub: sub, full: typ + "/" + sub, q: 1000}
		valid := true
		for _, p := range parts[1:] {
			name, value, _ := strings.Cut(p, "=")
			if !strings.EqualFold(strings.TrimSpace(name), "q") {
				continue
			}
			q, ok := parseQValue(strings.TrimSpace(value))
			if !ok {
				valid = false
			}
			r.q = q
			break // parameters after q are accept-ext, not media type parameters
		}
		if valid {
			out = append(out, r)
		}
	}
	return out
}

// parseQValue parses qvalue = ( "0" [ "." 0*3DIGIT ] ) / ( "1" [ "." 0*3("0") ] )
// into thousandths.
func parseQValue(s string) (int, bool) {
	if s == "" || len(s) > 5 || (s[0] != '0' && s[0] != '1') {
		return 0, false
	}
	q := int(s[0]-'0') * 1000
	if len(s) == 1 {
		return q, true
	}
	if s[1] != '.' {
		return 0, false
	}
	scale := 100
	for i := 2; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, false
		}
		q += int(c-'0') * scale
		scale /= 10
	}
	if q > 1000 {
		return 0, false
	}
	return q, true
}

// isToken reports whether s is a non-empty RFC 9110 token.
func isToken(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0:
		default:
			return false
		}
	}
	return true
}

// splitQuoted splits s on sep outside double-quoted strings. It always
// returns at least one element.
func splitQuoted(s string, sep byte) []string {
	var out []string
	start, inQuote := 0, false
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == '\\' && inQuote:
			i++
		case c == '"':
			inQuote = !inQuote
		case c == sep && !inQuote:
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}
