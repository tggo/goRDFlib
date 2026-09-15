package endpoint

import (
	"mime"
	"strings"
)

// offer is one representation the handler can produce: a media type and the
// aliases that also select it.
type offer struct {
	mediaType string
	aliases   []string
}

// Specificity of a media range match, higher wins.
const (
	matchNone      = -1
	matchAll       = 0 // */*
	matchType      = 1 // text/*
	matchAlias     = 2
	matchCanonical = 3
)

type mediaRange struct {
	full, typ string
	sub       string
	q         int // thousandths
}

// negotiate picks one of offers for an Accept header (RFC 9110 §12.5.1),
// following the rules of results.NegotiateAmong: each offer takes the quality
// of its most specific matching range, q=0 excludes, ties go to the more
// specific match and then to the earlier offer, and an empty header selects
// the first offer. It returns -1 when nothing is acceptable.
func negotiate(accept string, offers []offer) int {
	if len(offers) == 0 {
		return -1
	}
	if strings.TrimSpace(accept) == "" {
		return 0
	}
	ranges := parseAccept(accept)
	best, bestQ, bestSpec := -1, 0, matchNone
	for i, o := range offers {
		q, spec := 0, matchNone
		for _, r := range ranges {
			s := r.match(o)
			if s == matchNone || s < spec {
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
			best, bestQ, bestSpec = i, q, spec
		}
	}
	return best
}

func (r mediaRange) match(o offer) int {
	if r.typ == "*" {
		return matchAll
	}
	if r.full == o.mediaType {
		return matchCanonical
	}
	for _, a := range o.aliases {
		if r.full == a {
			return matchAlias
		}
	}
	if r.sub == "*" && strings.HasPrefix(o.mediaType, r.typ+"/") {
		return matchType
	}
	return matchNone
}

func parseAccept(accept string) []mediaRange {
	var out []mediaRange
	for _, elem := range strings.Split(accept, ",") {
		parts := strings.Split(elem, ";")
		full := strings.ToLower(strings.TrimSpace(parts[0]))
		typ, sub, ok := strings.Cut(full, "/")
		if !ok || typ == "" || sub == "" || (typ == "*" && sub != "*") {
			continue
		}
		r := mediaRange{full: full, typ: typ, sub: sub, q: 1000}
		valid := true
		for _, p := range parts[1:] {
			name, value, _ := strings.Cut(p, "=")
			if !strings.EqualFold(strings.TrimSpace(name), "q") {
				continue
			}
			q, ok := parseQ(strings.TrimSpace(value))
			valid = ok
			r.q = q
			break
		}
		if valid {
			out = append(out, r)
		}
	}
	return out
}

func parseQ(s string) (int, bool) {
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
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		q += int(s[i]-'0') * scale
		scale /= 10
	}
	return q, q <= 1000
}

// mediaType splits a Content-Type header into its lower-cased media type and
// its charset parameter ("" when absent). A header that does not parse yields
// the lower-cased text before the first ';' so that the caller can still
// report it.
func mediaType(header string) (mt, charset string) {
	if header == "" {
		return "", ""
	}
	t, params, err := mime.ParseMediaType(header)
	if err != nil {
		t, _, _ = strings.Cut(header, ";")
		return strings.ToLower(strings.TrimSpace(t)), ""
	}
	return t, strings.ToLower(params["charset"])
}

// utf8Charset reports whether a charset parameter is absent or names UTF-8,
// the only encoding SPARQL Protocol bodies may use.
func utf8Charset(cs string) bool {
	return cs == "" || cs == "utf-8" || cs == "utf8"
}
