package jsonld

import (
	"bytes"
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
	return parse(r, &cfg, graphSink(g))
}

// sink receives the statements of a successful parse, in order. An error
// stops the delivery and is returned by the parse.
type sink func(statement) error

// graphSink adds each statement to g.
func graphSink(g *rdflibgo.Graph) sink {
	return func(st statement) error {
		g.Add(st.s, st.p, st.o)
		return nil
	}
}

// parse expands r and hands its statements to out, only once the whole
// document has been converted.
func parse(r io.Reader, cfg *config, out sink) error {
	base := cfg.base

	// Provenance needs the source twice: once to expand it into RDF, and once
	// to find out where its identifiers were written. Without the option the
	// reader is consumed exactly as before.
	var src []byte
	if cfg.provenance != nil {
		var err error
		src, err = io.ReadAll(r)
		if err != nil {
			return fmt.Errorf("jsonld: reading input: %w", err)
		}
		r = bytes.NewReader(src)
	}

	doc, err := decodeDocument(r)
	if err != nil {
		return err
	}

	// Convert to N-Quads via json-gold
	proc := ld.NewJsonLdProcessor()
	ldOpts := ld.NewJsonLdOptions(base)
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

	var result any
	// json-gold is not defensive about every malformed document and can panic
	// rather than return an error; see ErrProcessorPanic.
	if err := guard("expansion to RDF", func() error {
		var err error
		result, err = proc.ToRDF(doc, ldOpts)
		return err
	}); err != nil {
		return err
	}
	ds, ok := result.(*ld.RDFDataset)
	if !ok {
		if result == nil {
			return nil // empty result
		}
		return fmt.Errorf("json-ld: unexpected ToRDF result type %T", result)
	}

	return addDataset(out, ds, cfg, src)
}

// addDataset adds the RDF json-gold produced to g. It builds terms directly
// from the dataset (datasetStatements) when every node is one the N-Quads
// parser would accept unchanged, and otherwise serializes the dataset to
// N-Quads and runs parseNQuadsInto, which owns all error, skip and
// line-length behavior. cfg.forceTextPath (tests only) takes the second route
// unconditionally.
func addDataset(out sink, ds *ld.RDFDataset, cfg *config, src []byte) error {
	if !cfg.forceTextPath {
		if stmts, ok := datasetStatements(ds, cfg.preserveBlankNodeIDs, cfg.unbounded); ok {
			if len(stmts) == 0 {
				return nil
			}
			return commit(out, stmts, cfg, src)
		}
	}
	var sb strings.Builder
	if err := (&ld.NQuadRDFSerializer{}).SerializeTo(&sb, ds); err != nil {
		return fmt.Errorf("json-ld: serializing expanded RDF: %w", err)
	}
	if sb.Len() == 0 {
		return nil
	}
	return parseNQuadsInto(out, sb.String(), cfg, src)
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
func parseNQuadsInto(out sink, nqStr string, cfg *config, src []byte) error {
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

	// One statement per N-Quads line.
	stmts := make([]statement, 0, strings.Count(nqStr, "\n")+1)
	collect := func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, _ rdflibgo.Term) error {
		stmts = append(stmts, statement{s, p, o})
		return nil
	}
	if err := nq.ParseStream(strings.NewReader(nqStr), collect, nqOpts...); err != nil {
		return err
	}
	return commit(out, stmts, cfg, src)
}

// commit hands stmts to out and reports provenance for them.
func commit(out sink, stmts []statement, cfg *config, src []byte) error {
	if cfg.provenance == nil {
		for _, st := range stmts {
			if err := out(st); err != nil {
				return err
			}
		}
		return nil
	}
	// The line numbers of the intermediate RDF are meaningless to the
	// caller — they belong to a document nobody wrote. What is reported is
	// the line of the source node object that declared the subject.
	lines := buildSubjectLines(src, cfg.base, cfg.documentLoader, cfg.expandContext)
	for _, st := range stmts {
		if err := out(st); err != nil {
			return err
		}
		if line := lines[term.TermKey(st.s)]; line > 0 {
			cfg.provenance(st.s, st.p, st.o, line)
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
