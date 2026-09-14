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
// Options: WithBase, WithDocumentLoader, WithExpandContext, WithSkipHandler,
// WithStrictIRIs, WithUnboundedLines, WithPreserveBlankNodeIDs, WithProvenance.
// Statements with ill-formed IRIs are dropped (JSON-LD 1.1 API §8.1); see
// WithSkipHandler. The graph is changed only when Parse succeeds. Blank nodes share
// one scope per Parse call by default, including anonymous nodes generated
// during expansion.
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
	// Work around json-gold dropping the "#" of a relative @vocab; see
	// resolveEmptyFragmentVocab.
	var docBase string
	ldOpts.ExpandContext, docBase = fixExpandContextVocab(cfg.expandContext, base)
	resolveEmptyFragmentVocab(doc, docBase)

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

// parseNQuadsInto parses the expanded N-Quads into g.
//
// A statement with an ill-formed IRI is dropped, as JSON-LD 1.1 API §8.1
// (Deserialize JSON-LD to RDF) requires: json-gold still emits some of them,
// such as <@id> for "@type": "@id" on a node object, or the relative IRI of a
// value that does not resolve. Each dropped statement goes to cfg.skipHandler.
// With cfg.strictIRIs the first one is an error instead.
//
// Statements are collected first and added to g only once the whole document
// has been read, so a failed parse leaves g as it was. Provenance is reported
// for the same statements after they are added.
func parseNQuadsInto(g *rdflibgo.Graph, nqStr string, cfg *config, src []byte) error {
	nqOpts := make([]nq.Option, 0, 4)
	if cfg.preserveBlankNodeIDs {
		nqOpts = append(nqOpts, nq.WithPreserveBlankNodeIDs())
	}
	if cfg.unbounded {
		nqOpts = append(nqOpts, nq.WithUnboundedLines())
	}
	if !cfg.strictIRIs {
		skipIllFormed := func(lineNum int, line string, err error) (string, bool) {
			if isIllFormedIRI(err) {
				if cfg.skipHandler != nil {
					cfg.skipHandler(line, err)
				}
				return "", false // drop this statement, continue parsing
			}
			// Re-surface anything that isn't an ill-formed IRI by re-parsing
			// the unmodified line, which fails the same way and aborts.
			return line, true
		}
		nqOpts = append(nqOpts, nq.WithErrorHandler(skipIllFormed))
	}

	type statement struct {
		s rdflibgo.Subject
		p rdflibgo.URIRef
		o rdflibgo.Term
	}
	var stmts []statement
	var provLines []int // parallel to stmts; 0 when the subject has no line
	var lines subjectLines
	if cfg.provenance != nil {
		// The line numbers of the intermediate N-Quads are meaningless to the
		// caller — they belong to a document nobody wrote. What is reported is
		// the line of the source node object that declared the subject, which
		// is why the N-Quads line is discarded here.
		lines = buildSubjectLines(src, cfg.base, cfg.documentLoader, cfg.expandContext)
	}
	collect := func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, _ rdflibgo.Term) error {
		stmts = append(stmts, statement{s, p, o})
		if cfg.provenance != nil {
			provLines = append(provLines, lines[term.TermKey(s)])
		}
		return nil
	}
	if err := nq.ParseStream(strings.NewReader(nqStr), collect, nqOpts...); err != nil {
		return err
	}
	for i, st := range stmts {
		g.Add(st.s, st.p, st.o)
		if cfg.provenance != nil && provLines[i] > 0 {
			cfg.provenance(st.s, st.p, st.o, provLines[i])
		}
	}
	return nil
}

// isIllFormedIRI reports whether err is an N-Quads parse failure caused by an
// IRI that is not well-formed: relative, or holding a character no IRI may
// contain.
func isIllFormedIRI(err error) bool {
	return errors.Is(err, ntsyntax.ErrRelativeIRI) || errors.Is(err, ntsyntax.ErrInvalidIRI) || errors.Is(err, term.ErrInvalidIRI)
}
