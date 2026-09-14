// Package iri resolves relative IRI references against a base IRI, for the
// Turtle, TriG and RDF/XML parsers.
//
// It implements RFC 3986 §5.2 (strict mode) directly on strings, as RFC 3987
// §6.5 requires for IRIs: no percent-decoding, no percent-encoding, and no
// validation beyond splitting the reference into its five components. RDF
// compares IRIs as strings (RDF 1.1 Concepts §3.2), so any rewriting of a
// character — encoding "é" as "%C3%A9", or decoding "%2F" to "/" — changes which
// resource the IRI denotes.
//
// net/url is deliberately not used: it percent-encodes non-ASCII characters,
// rejects malformed escapes such as "%zz" (leaving the reference unresolved),
// drops an empty fragment, and turns an opaque base such as "urn:isbn:123" into
// a hierarchical one.
//
// Safe for concurrent use; the package has no state.
package iri

import "strings"

// components is a reference split per RFC 3986 §3 / Appendix B. The has* flags
// distinguish an absent component from an empty one ("http://a?" has an empty
// query, "http://a" none), which the resolution algorithm depends on.
type components struct {
	scheme       string
	hasScheme    bool
	authority    string
	hasAuthority bool
	path         string
	query        string
	hasQuery     bool
	fragment     string
	hasFragment  bool
}

// split breaks s into components following the regular expression in RFC 3986
// Appendix B, except that a scheme is only recognised when it is syntactically
// valid (ALPHA *( ALPHA / DIGIT / "+" / "-" / "." )), so a relative reference
// such as "é:x" or "1:x" is not mistaken for an absolute one.
func split(s string) components {
	var c components
	if i := strings.IndexAny(s, ":/?#"); i > 0 && s[i] == ':' && validScheme(s[:i]) {
		c.scheme, c.hasScheme = s[:i], true
		s = s[i+1:]
	}
	if strings.HasPrefix(s, "//") {
		s = s[2:]
		end := strings.IndexAny(s, "/?#")
		if end < 0 {
			end = len(s)
		}
		c.authority, c.hasAuthority = s[:end], true
		s = s[end:]
	}
	if i := strings.IndexByte(s, '#'); i >= 0 {
		c.fragment, c.hasFragment = s[i+1:], true
		s = s[:i]
	}
	if i := strings.IndexByte(s, '?'); i >= 0 {
		c.query, c.hasQuery = s[i+1:], true
		s = s[:i]
	}
	c.path = s
	return c
}

func validScheme(s string) bool {
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z'):
		case i > 0 && ((ch >= '0' && ch <= '9') || ch == '+' || ch == '-' || ch == '.'):
		default:
			return false
		}
	}
	return s != ""
}

// IsAbsolute reports whether s starts with a syntactically valid scheme.
func IsAbsolute(s string) bool {
	return split(s).hasScheme
}

// Resolve returns ref resolved against base using the RFC 3986 §5.2.2 strict
// algorithm. RFC 3986 §5.1 requires the base to be absolute; if base has no
// scheme (it is empty, or relative, or malformed like "://x") there is nothing
// to resolve against and ref is returned unchanged.
func Resolve(base, ref string) string {
	b := split(base)
	if !b.hasScheme {
		return ref
	}
	r := split(ref)
	var t components

	switch {
	case r.hasScheme:
		t = r
		t.path = removeDotSegments(r.path)
	case r.hasAuthority:
		t = r
		t.path = removeDotSegments(r.path)
		t.scheme, t.hasScheme = b.scheme, b.hasScheme
	default:
		t.scheme, t.hasScheme = b.scheme, b.hasScheme
		t.authority, t.hasAuthority = b.authority, b.hasAuthority
		switch {
		case r.path == "":
			t.path = b.path
			if r.hasQuery {
				t.query, t.hasQuery = r.query, true
			} else {
				t.query, t.hasQuery = b.query, b.hasQuery
			}
		case strings.HasPrefix(r.path, "/"):
			t.path = removeDotSegments(r.path)
			t.query, t.hasQuery = r.query, r.hasQuery
		default:
			t.path = removeDotSegments(merge(b, r.path))
			t.query, t.hasQuery = r.query, r.hasQuery
		}
		t.fragment, t.hasFragment = r.fragment, r.hasFragment
	}
	return t.String()
}

// merge implements RFC 3986 §5.2.3.
func merge(b components, refPath string) string {
	if b.hasAuthority && b.path == "" {
		return "/" + refPath
	}
	if i := strings.LastIndexByte(b.path, '/'); i >= 0 {
		return b.path[:i+1] + refPath
	}
	return refPath
}

// removeDotSegments implements RFC 3986 §5.2.4.
func removeDotSegments(path string) string {
	if !strings.Contains(path, ".") {
		return path
	}
	in := path
	var out []string // output buffer as segments, each with its leading "/" if any
	for in != "" {
		switch {
		case strings.HasPrefix(in, "../"):
			in = in[3:]
		case strings.HasPrefix(in, "./"):
			in = in[2:]
		case strings.HasPrefix(in, "/./"):
			in = in[2:]
		case in == "/.":
			in = "/"
		case strings.HasPrefix(in, "/../"):
			in = in[3:]
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		case in == "/..":
			in = "/"
			if len(out) > 0 {
				out = out[:len(out)-1]
			}
		case in == "." || in == "..":
			in = ""
		default:
			// Move the first path segment, including its initial "/" if any
			// and up to but not including the next "/", to the output.
			start := 0
			if in[0] == '/' {
				start = 1
			}
			end := strings.IndexByte(in[start:], '/')
			if end < 0 {
				end = len(in)
			} else {
				end += start
			}
			out = append(out, in[:end])
			in = in[end:]
		}
	}
	return strings.Join(out, "")
}

// String recomposes the components per RFC 3986 §5.3.
func (c components) String() string {
	var sb strings.Builder
	sb.Grow(len(c.scheme) + len(c.authority) + len(c.path) + len(c.query) + len(c.fragment) + 5)
	if c.hasScheme {
		sb.WriteString(c.scheme)
		sb.WriteByte(':')
	}
	if c.hasAuthority {
		sb.WriteString("//")
		sb.WriteString(c.authority)
	}
	sb.WriteString(c.path)
	if c.hasQuery {
		sb.WriteByte('?')
		sb.WriteString(c.query)
	}
	if c.hasFragment {
		sb.WriteByte('#')
		sb.WriteString(c.fragment)
	}
	return sb.String()
}
