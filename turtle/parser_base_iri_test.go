package turtle

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// TestParserBaseIRIResolution pins RFC 3986 §5.2 resolution on IRI strings
// (RFC 3987 §6.5). net/url percent-encoded "é", invented an authority for an
// opaque base (rdflib #1216) and left "a%zz" unresolved.
func TestParserBaseIRIResolution(t *testing.T) {
	cases := []struct{ base, ref, want string }{
		{"http://a/b/c/d;p?q", "é", "http://a/b/c/é"},
		{"example:", "Category", "example:Category"},
		{"urn:isbn:123", "Category", "urn:Category"},
		{"http://a/b/", "a%zz", "http://a/b/a%zz"},
		{"http://e/", "foo%2Fbar%23x%20y", "http://e/foo%2Fbar%23x%20y"},
		{"http://a/b/c/d;p?q", "../../../g", "http://a/g"},
		{"http://a/b/c/d;p?q", "g?y/../x", "http://a/b/c/g?y/../x"},
		{"http://a/b", "#", "http://a/b#"},
	}
	for _, tc := range cases {
		for _, via := range []string{"option", "@base"} {
			g := rdflibgo.NewGraph()
			src := "<" + tc.ref + "> <http://x/p> <http://x/o> ."
			var opts []Option
			if via == "option" {
				opts = append(opts, WithBase(tc.base))
			} else {
				src = "@base <" + tc.base + "> .\n" + src
			}
			if err := Parse(g, strings.NewReader(src), opts...); err != nil {
				t.Fatalf("base %q ref %q via %s: %v", tc.base, tc.ref, via, err)
			}
			var got string
			g.Triples(nil, nil, nil)(func(tr rdflibgo.Triple) bool {
				got = tr.Subject.String()
				return false
			})
			if got != tc.want {
				t.Errorf("base %q ref %q via %s: got %q, want %q", tc.base, tc.ref, via, got, tc.want)
			}
		}
	}
}
