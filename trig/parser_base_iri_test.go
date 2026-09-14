package trig

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
)

// TestParserBaseIRIResolution pins RFC 3986 §5.2 resolution on IRI strings
// (RFC 3987 §6.5), for triples, graph names and @base alike.
func TestParserBaseIRIResolution(t *testing.T) {
	cases := []struct{ base, ref, want string }{
		{"http://a/b/c/d;p?q", "é", "http://a/b/c/é"},
		{"example:", "Category", "example:Category"},
		{"urn:isbn:123", "Category", "urn:Category"},
		{"http://a/b/", "a%zz", "http://a/b/a%zz"},
		{"http://e/", "foo%2Fbar%23x%20y", "http://e/foo%2Fbar%23x%20y"},
	}
	for _, tc := range cases {
		ds := graph.NewDataset()
		src := "<" + tc.ref + "> { <" + tc.ref + "> <http://x/p> <http://x/o> . }"
		if err := ParseDataset(ds, strings.NewReader(src), WithBase(tc.base)); err != nil {
			t.Fatalf("base %q ref %q: %v", tc.base, tc.ref, err)
		}
		g := ds.Graph(rdflibgo.NewURIRefUnsafe(tc.want))
		if g.Len() != 1 {
			t.Errorf("base %q ref %q: graph %q holds %d triples, want 1", tc.base, tc.ref, tc.want, g.Len())
			continue
		}
		g.Triples(nil, nil, nil)(func(tr rdflibgo.Triple) bool {
			if got := tr.Subject.String(); got != tc.want {
				t.Errorf("base %q ref %q: subject %q, want %q", tc.base, tc.ref, got, tc.want)
			}
			return false
		})
	}
}
