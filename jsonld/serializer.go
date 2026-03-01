package jsonld

import (
	"bytes"
	"encoding/json"
	"io"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/nt"

	"github.com/piprate/json-gold/ld"
)

// Serialize serializes a Graph to JSON-LD format.
func Serialize(g *rdflibgo.Graph, w io.Writer, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	base := cfg.base

	// First serialize to N-Triples
	var nqBuf bytes.Buffer
	if err := nt.Serialize(g, &nqBuf); err != nil {
		return err
	}

	// Convert N-Quads to JSON-LD via json-gold
	proc := ld.NewJsonLdProcessor()
	ldOpts := ld.NewJsonLdOptions(base)
	ldOpts.Format = "application/n-quads"

	doc, err := proc.FromRDF(nqBuf.String(), ldOpts)
	if err != nil {
		return err
	}

	// Build context from graph namespace bindings for compaction
	context := make(map[string]interface{})
	g.Namespaces()(func(prefix string, ns rdflibgo.URIRef) bool {
		context[prefix] = ns.Value()
		return true
	})

	// Compact if we have namespace bindings
	var output interface{} = doc
	if len(context) > 0 {
		compacted, err := proc.Compact(doc, context, ldOpts)
		if err == nil {
			output = compacted
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}
