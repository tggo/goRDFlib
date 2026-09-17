package iri

import "strings"

// Relativize returns the shortest reference that Resolve(base, ref) turns back
// into target, for serializers that write @base. It reports false, and returns
// target unchanged, when no relative reference does that: a different scheme
// or authority, a base without a scheme, or a target that resolution would
// normalise (for example one with dot segments).
//
// Every candidate is checked by resolving it, so a relative reference is only
// returned when a parser reading it against the same base produces exactly
// target. Network-path references ("//host/x") are not attempted.
func Relativize(base, target string) (string, bool) {
	b, t := split(base), split(target)
	if !b.hasScheme || !t.hasScheme || b.scheme != t.scheme ||
		b.hasAuthority != t.hasAuthority || b.authority != t.authority {
		return target, false
	}

	var tail strings.Builder // query and fragment, as written in target
	if t.hasQuery {
		tail.WriteByte('?')
		tail.WriteString(t.query)
	}
	if t.hasFragment {
		tail.WriteByte('#')
		tail.WriteString(t.fragment)
	}
	suffix := tail.String()

	best := target
	try := func(ref string) {
		if len(ref) < len(best) && Resolve(base, ref) == target {
			best = ref
		}
	}

	// Same document: "", "#frag", "?query#frag".
	if t.path == b.path {
		try("")
		if t.hasFragment {
			try("#" + t.fragment)
		}
		try(suffix)
	}

	// Relative to the base's directory and each of its ancestors: "x", "../x".
	if dir := strings.LastIndexByte(b.path, '/'); dir >= 0 {
		up := ""
		for d := b.path[:dir+1]; ; up += "../" {
			if rest, ok := strings.CutPrefix(t.path, d); ok {
				try(up + pathSegmentRef(rest) + suffix)
			}
			i := strings.LastIndexByte(strings.TrimSuffix(d, "/"), '/')
			if i < 0 {
				break
			}
			d = d[:i+1]
		}
	}

	// Absolute path: "/x".
	if t.hasAuthority && strings.HasPrefix(t.path, "/") && !strings.HasPrefix(t.path, "//") {
		try(t.path + suffix)
	}

	return best, best != target
}

// pathSegmentRef makes a relative path safe to write as a reference: an empty
// path becomes "./", and a first segment containing ':' is prefixed with "./"
// so it is not read as a scheme (RFC 3986 §4.2).
func pathSegmentRef(p string) string {
	first, _, _ := strings.Cut(p, "/")
	if p == "" || strings.Contains(first, ":") {
		return "./" + p
	}
	return p
}
