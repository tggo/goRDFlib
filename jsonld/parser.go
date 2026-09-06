package jsonld

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/ntsyntax"
	"github.com/tggo/goRDFlib/nq"
	"github.com/tggo/goRDFlib/term"

	"github.com/piprate/json-gold/ld"
)

// Parse parses a JSON-LD document into the given graph.
// It uses piprate/json-gold to expand the document to N-Quads, then parses those into the graph.
// Options: WithBase, WithDocumentLoader, WithExpandContext, WithSkipInvalidIRIs,
// WithUnboundedLines, WithProvenance.
func Parse(g *rdflibgo.Graph, r io.Reader, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	base := cfg.base

	// Provenance needs the source twice: once to expand it into RDF, and once
	// to find out where its identifiers were written. Without the option the
	// reader is consumed exactly as before.
	var src []byte
	if cfg.provenance != nil {
		var err error
		src, err = io.ReadAll(r)
		if err != nil {
			return err
		}
		r = bytes.NewReader(src)
	}

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
	// Applied before the document's own @context; see WithExpandContext.
	ldOpts.ExpandContext = cfg.expandContext

	var nquads any
	// json-gold is not defensive about every malformed document and can panic
	// rather than return an error; see ErrProcessorPanic.
	if err := guard("expansion to N-Quads", func() error {
		var err error
		nquads, err = proc.ToRDF(doc, ldOpts)
		return err
	}); err != nil {
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
	return parseNQuadsInto(g, nqStr, &cfg, src)
}

// parseNQuadsInto parses the expanded N-Quads into g, honoring cfg.skipInvalidIRI.
// When set, lines that fail because of an invalid IRI (ntsyntax.ErrInvalidIRI)
// are skipped instead of aborting the parse.
func parseNQuadsInto(g *rdflibgo.Graph, nqStr string, cfg *config, src []byte) error {
	nqOpts := make([]nq.Option, 0, 3)
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
	if cfg.provenance != nil {
		// The line numbers of the intermediate N-Quads are meaningless to the
		// caller — they belong to a document nobody wrote. What is reported is
		// the line of the source node object that declared the subject, which
		// is why the N-Quads line is discarded here.
		lines := buildSubjectLines(src, cfg.base, cfg.documentLoader, cfg.expandContext)
		handler := cfg.provenance
		nqOpts = append(nqOpts, nq.WithProvenance(
			func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, _ rdflibgo.Term, _ int) {
				if line, ok := lines[term.TermKey(s)]; ok {
					handler(s, p, o, line)
				}
			}))
	}
	return nq.Parse(g, strings.NewReader(nqStr), nqOpts...)
}
