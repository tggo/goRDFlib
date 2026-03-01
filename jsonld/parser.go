package jsonld

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/nq"

	"github.com/piprate/json-gold/ld"
)

// Parse parses a JSON-LD document into the given graph.
func Parse(g *rdflibgo.Graph, r io.Reader, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	base := cfg.base

	// Decode JSON
	var doc interface{}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return err
	}

	// Convert to N-Quads via json-gold
	proc := ld.NewJsonLdProcessor()
	ldOpts := ld.NewJsonLdOptions(base)
	ldOpts.Format = "application/n-quads"

	nquads, err := proc.ToRDF(doc, ldOpts)
	if err != nil {
		return err
	}

	nqStr, ok := nquads.(string)
	if !ok {
		if nquads == nil {
			return nil // empty result
		}
		return fmt.Errorf("json-ld: unexpected ToRDF result type %T", nquads)
	}
	if nqStr == "" {
		return nil
	}

	// Parse the N-Quads into the graph
	return nq.Parse(g, strings.NewReader(nqStr))
}
