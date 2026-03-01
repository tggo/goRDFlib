package rdflibgo

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/piprate/json-gold/ld"
)

// JSONLDSerializer serializes a Graph to JSON-LD format.
// Ported from: rdflib.plugins.serializers.jsonld
// Uses piprate/json-gold as the JSON-LD processor, converting via N-Quads.
type JSONLDSerializer struct{}

func init() {
	RegisterSerializer("json-ld", func() Serializer { return &JSONLDSerializer{} })
	RegisterSerializer("jsonld", func() Serializer { return &JSONLDSerializer{} })
	RegisterSerializer("application/ld+json", func() Serializer { return &JSONLDSerializer{} })
}

func (s *JSONLDSerializer) Serialize(g *Graph, w io.Writer, base string) error {
	// First serialize to N-Quads
	var nqBuf bytes.Buffer
	ntSer := &NTriplesSerializer{}
	if err := ntSer.Serialize(g, &nqBuf, base); err != nil {
		return err
	}

	// Convert N-Quads to JSON-LD via json-gold
	proc := ld.NewJsonLdProcessor()
	opts := ld.NewJsonLdOptions(base)
	opts.Format = "application/n-quads"

	doc, err := proc.FromRDF(nqBuf.String(), opts)
	if err != nil {
		return err
	}

	// Build context from graph namespace bindings for compaction
	context := make(map[string]interface{})
	g.Namespaces()(func(prefix string, ns URIRef) bool {
		context[prefix] = ns.Value()
		return true
	})

	// Compact if we have namespace bindings
	var output interface{} = doc
	if len(context) > 0 {
		compacted, err := proc.Compact(doc, context, opts)
		if err == nil {
			output = compacted
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}
