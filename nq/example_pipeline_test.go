package nq_test

import (
	"os"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/jsonld"
	"github.com/tggo/goRDFlib/nq"
)

// Each JSON-LD document goes into its own named graph, its blank nodes become
// stable Skolem IRIs, and the result is streamed as N-Quads. Running it again
// over the same documents writes the same bytes.
func ExampleWriter_namedGraphs() {
	docs := map[string]string{
		"urn:example:doc:1": `{"@context": {"@vocab": "https://schema.org/"},
			"@id": "https://example.org/place/1", "name": "Well 1",
			"geo": {"latitude": 35.1, "longitude": -106.6}}`,
	}

	w := nq.NewWriter(os.Stdout)
	for graphIRI, doc := range docs {
		id, _ := rdflibgo.NewURIRef(graphIRI)
		g := rdflibgo.NewGraph(rdflibgo.WithIdentifier(id))
		if err := jsonld.Parse(g, strings.NewReader(doc)); err != nil {
			panic(err)
		}
		sk := g.Skolemize("https://example.org", rdflibgo.WithStableSkolemIDs())
		if err := w.WriteGraph(sk); err != nil {
			panic(err)
		}
	}
	_ = w.Flush()
	// Unordered output:
	// <https://example.org/place/1> <https://schema.org/geo> <https://example.org/.well-known/genid/69d8afbc8ddc9d8689e52a1bc23609b3> <urn:example:doc:1> .
	// <https://example.org/place/1> <https://schema.org/name> "Well 1" <urn:example:doc:1> .
	// <https://example.org/.well-known/genid/69d8afbc8ddc9d8689e52a1bc23609b3> <https://schema.org/longitude> "-1.066E2"^^<http://www.w3.org/2001/XMLSchema#double> <urn:example:doc:1> .
	// <https://example.org/.well-known/genid/69d8afbc8ddc9d8689e52a1bc23609b3> <https://schema.org/latitude> "3.51E1"^^<http://www.w3.org/2001/XMLSchema#double> <urn:example:doc:1> .
}
