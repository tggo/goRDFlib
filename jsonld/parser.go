package jsonld

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/ntsyntax"
	"github.com/tggo/goRDFlib/nq"

	"github.com/piprate/json-gold/ld"
)

// Parse parses a JSON-LD document into the given graph.
// It uses piprate/json-gold to expand the document to N-Quads, then parses those into the graph.
// Options: WithBase, WithDocumentLoader, WithSkipInvalidIRIs, WithUnboundedLines.
func Parse(g *rdflibgo.Graph, r io.Reader, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	base := cfg.base

	// Decode JSON
	var doc any
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return err
	}

	// Convert to N-Quads via json-gold
	proc := ld.NewJsonLdProcessor()
	ldOpts := ld.NewJsonLdOptions(base)
	ldOpts.Format = "application/n-quads"
	if cfg.documentLoader != nil {
		ldOpts.DocumentLoader = cfg.documentLoader
	}

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
	return parseNQuadsInto(g, nqStr, &cfg)
}

// parseNQuadsInto parses the expanded N-Quads into g, honoring cfg.skipInvalidIRI.
// When set, lines that fail because of an invalid IRI (ntsyntax.ErrInvalidIRI)
// are skipped instead of aborting the parse.
func parseNQuadsInto(g *rdflibgo.Graph, nqStr string, cfg *config) error {
	nqOpts := make([]nq.Option, 0, 2)
	if cfg.unbounded {
		nqOpts = append(nqOpts, nq.WithUnboundedLines())
	}
	if cfg.skipInvalidIRI {
		skipInvalid := func(lineNum int, line string, err error) (string, bool) {
			if errors.Is(err, ntsyntax.ErrInvalidIRI) {
				return "", false // skip this triple, continue parsing
			}
			// Re-surface anything that isn't an invalid IRI by re-parsing the
			// unmodified line, which fails the same way and aborts.
			return line, true
		}
		nqOpts = append(nqOpts, nq.WithErrorHandler(skipInvalid))
	}
	return nq.Parse(g, strings.NewReader(nqStr), nqOpts...)
}
