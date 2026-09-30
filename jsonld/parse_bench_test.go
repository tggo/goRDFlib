package jsonld

import (
	"strings"
	"testing"

	rdflibgo "github.com/tggo/goRDFlib"
)

// BenchmarkParse measures Parse on a schema.org-style document (inline
// context, so it needs no network) through the direct dataset conversion and
// through the N-Quads text path it replaced.
func BenchmarkParse(b *testing.B) {
	for _, size := range []struct {
		name string
		n    int
	}{{"small", 4}, {"large", 120}} {
		doc := generatedPlace(size.n)
		for _, path := range []struct {
			name string
			text bool
		}{{"dataset", false}, {"text", true}} {
			b.Run(size.name+"/"+path.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					g := rdflibgo.NewGraph()
					if err := Parse(g, strings.NewReader(doc), func(c *config) { c.forceTextPath = path.text }); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
