package rdfxml

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// TestParserBaseIRIResolution pins RFC 3986 §5.2 resolution on IRI strings.
// The parser used to run every resolved IRI through url.PathUnescape, so
// "foo%2Fbar%23x%20y" became "foo/bar#x y" — a different resource, since RDF
// compares IRIs as strings (RDF 1.1 Concepts §3.2).
func TestParserBaseIRIResolution(t *testing.T) {
	cases := []struct{ base, ref, want string }{
		{"http://e/", "foo%2Fbar%23x%20y", "http://e/foo%2Fbar%23x%20y"},
		{"http://a/b/c/d;p?q", "é", "http://a/b/c/é"},
		{"example:", "Category", "example:Category"},
		{"urn:isbn:123", "Category", "urn:Category"},
		{"http://a/b/", "a%zz", "http://a/b/a%zz"},
		{"http://a/b/c/d;p?q#f", "", "http://a/b/c/d;p?q"},
	}
	for _, tc := range cases {
		src := `<rdf:RDF xmlns:rdf="` + rdfNS + `" xmlns:ex="http://e/"><rdf:Description rdf:about="` +
			tc.ref + `"><ex:p>v</ex:p></rdf:Description></rdf:RDF>`
		g := rdflibgo.NewGraph()
		if err := Parse(g, strings.NewReader(src), WithBase(tc.base)); err != nil {
			t.Fatalf("base %q ref %q: %v", tc.base, tc.ref, err)
		}
		var got string
		g.Triples(nil, nil, nil)(func(tr rdflibgo.Triple) bool {
			got = tr.Subject.String()
			return false
		})
		if got != tc.want {
			t.Errorf("base %q ref %q: got %q, want %q", tc.base, tc.ref, got, tc.want)
		}
	}
}
