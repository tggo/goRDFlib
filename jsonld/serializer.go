package jsonld

import (
	"bytes"
	"encoding/json"
	"io"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/nq"

	"github.com/piprate/json-gold/ld"
)

// Serialize serializes a Graph to JSON-LD format.
// It converts the graph to N-Quads (preserving named graph context), then uses
// piprate/json-gold to produce JSON-LD output.
// Options: WithBase, WithForm/WithExpanded, WithDocumentLoader.
func Serialize(g *rdflibgo.Graph, w io.Writer, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	base := cfg.base

	// Serialize to N-Quads (not N-Triples) to preserve graph context
	var nqBuf bytes.Buffer
	if err := nq.Serialize(g, &nqBuf); err != nil {
		return err
	}

	// Convert N-Quads to JSON-LD via json-gold
	proc := ld.NewJsonLdProcessor()
	ldOpts := ld.NewJsonLdOptions(base)
	ldOpts.Format = "application/n-quads"
	if cfg.documentLoader != nil {
		ldOpts.DocumentLoader = cfg.documentLoader
	}

	doc, err := proc.FromRDF(nqBuf.String(), ldOpts)
	if err != nil {
		return err
	}

	var output any = doc

	// Apply compaction unless expanded form is requested
	if cfg.form != FormExpanded {
		// Build context from graph namespace bindings for compaction
		context := make(map[string]any)
		g.Namespaces()(func(prefix string, ns rdflibgo.URIRef) bool {
			context[prefix] = ns.Value()
			return true
		})

		if len(context) > 0 {
			compacted, err := proc.Compact(doc, context, ldOpts)
			if err != nil {
				return err
			}
			output = compacted
		}
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}
