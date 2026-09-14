package iri

import "testing"

// TestResolveRFC3986Examples runs every example in RFC 3986 §5.4.1 (normal)
// and §5.4.2 (abnormal, strict parser).
func TestResolveRFC3986Examples(t *testing.T) {
	const base = "http://a/b/c/d;p?q"
	cases := map[string]string{
		// §5.4.1 normal examples
		"g:h":     "g:h",
		"g":       "http://a/b/c/g",
		"./g":     "http://a/b/c/g",
		"g/":      "http://a/b/c/g/",
		"/g":      "http://a/g",
		"//g":     "http://g",
		"?y":      "http://a/b/c/d;p?y",
		"g?y":     "http://a/b/c/g?y",
		"#s":      "http://a/b/c/d;p?q#s",
		"g#s":     "http://a/b/c/g#s",
		"g?y#s":   "http://a/b/c/g?y#s",
		";x":      "http://a/b/c/;x",
		"g;x":     "http://a/b/c/g;x",
		"g;x?y#s": "http://a/b/c/g;x?y#s",
		"":        "http://a/b/c/d;p?q",
		".":       "http://a/b/c/",
		"./":      "http://a/b/c/",
		"..":      "http://a/b/",
		"../":     "http://a/b/",
		"../g":    "http://a/b/g",
		"../..":   "http://a/",
		"../../":  "http://a/",
		"../../g": "http://a/g",

		// §5.4.2 abnormal examples
		"../../../g":    "http://a/g",
		"../../../../g": "http://a/g",
		"/./g":          "http://a/g",
		"/../g":         "http://a/g",
		"g.":            "http://a/b/c/g.",
		".g":            "http://a/b/c/.g",
		"g..":           "http://a/b/c/g..",
		"..g":           "http://a/b/c/..g",
		"./../g":        "http://a/b/g",
		"./g/.":         "http://a/b/c/g/",
		"g/./h":         "http://a/b/c/g/h",
		"g/../h":        "http://a/b/c/h",
		"g;x=1/./y":     "http://a/b/c/g;x=1/y",
		"g;x=1/../y":    "http://a/b/c/y",
		"g?y/./x":       "http://a/b/c/g?y/./x",
		"g?y/../x":      "http://a/b/c/g?y/../x",
		"g#s/./x":       "http://a/b/c/g#s/./x",
		"g#s/../x":      "http://a/b/c/g#s/../x",
		"http:g":        "http:g", // strict parser
	}
	for ref, want := range cases {
		if got := Resolve(base, ref); got != want {
			t.Errorf("Resolve(%q, %q) = %q, want %q", base, ref, got, want)
		}
	}
}

// TestResolveIRIs covers what net/url got wrong for RDF parsers.
func TestResolveIRIs(t *testing.T) {
	cases := []struct{ base, ref, want string }{
		// RFC 3987 §6.5: resolve IRIs without percent-encoding them.
		{"http://a/b/c/d;p?q", "é", "http://a/b/c/é"},
		{"http://a/b/", "日本/語?q=ü#frag", "http://a/b/日本/語?q=ü#frag"},
		// Opaque bases (RFC 3986 §5.2.3: a base path without "/" is dropped
		// entirely when merging; no authority is invented).
		{"example:", "Category", "example:Category"},
		{"urn:isbn:123", "Category", "urn:Category"},
		{"urn:isbn:123", "#x", "urn:isbn:123#x"},
		// A malformed escape is not this function's business: it is resolved
		// like any other character.
		{"http://a/b/", "a%zz", "http://a/b/a%zz"},
		// Escapes are never decoded (RDF 1.1 Concepts §3.2 compares strings).
		{"http://e/", "foo%2Fbar%23x%20y", "http://e/foo%2Fbar%23x%20y"},
		// An empty fragment survives.
		{"http://a/b", "#", "http://a/b#"},
		{"http://a/b#old", "", "http://a/b"},
		// Authority with an empty path.
		{"http://a", "b", "http://a/b"},
		{"http://a?q", "?", "http://a?"},
		// No base, or a base that is not absolute (RFC 3986 §5.1).
		{"", "rel", "rel"},
		{"://bad-url", "rel", "rel"},
		{"/a/b/", "rel", "rel"},
		// A colon after a non-scheme first segment does not make it absolute.
		{"http://a/b/", "1:x", "http://a/b/1:x"},
	}
	for _, tc := range cases {
		if got := Resolve(tc.base, tc.ref); got != tc.want {
			t.Errorf("Resolve(%q, %q) = %q, want %q", tc.base, tc.ref, got, tc.want)
		}
	}
}

func TestIsAbsolute(t *testing.T) {
	for s, want := range map[string]bool{
		"http://a": true, "urn:x": true, "g:h": true, "a+b.c-d:x": true,
		"g": false, "/g": false, "//g": false, ":x": false, "1a:x": false, "é:x": false, "a/b:c": false,
	} {
		if got := IsAbsolute(s); got != want {
			t.Errorf("IsAbsolute(%q) = %v, want %v", s, got, want)
		}
	}
}
