package rdflibgo

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/piprate/json-gold/ld"
)

// JSONLDParser parses JSON-LD documents into a Graph.
// Ported from: rdflib.plugins.parsers.jsonld
// Uses piprate/json-gold as the JSON-LD processor, converting via N-Quads.
type JSONLDParser struct{}

func init() {
	RegisterParser("json-ld", func() Parser { return &JSONLDParser{} })
	RegisterParser("jsonld", func() Parser { return &JSONLDParser{} })
	RegisterParser("application/ld+json", func() Parser { return &JSONLDParser{} })
}

func (p *JSONLDParser) Parse(g *Graph, r io.Reader, base string) error {
	// Decode JSON
	var doc interface{}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return err
	}

	// Convert to N-Quads via json-gold
	proc := ld.NewJsonLdProcessor()
	opts := ld.NewJsonLdOptions(base)
	opts.Format = "application/n-quads"

	nquads, err := proc.ToRDF(doc, opts)
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
	nqParser := &NQuadsParser{}
	return nqParser.Parse(g, strings.NewReader(nqStr), base)
}
