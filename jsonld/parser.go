package jsonld

import (
	"bytes"
	"context"
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
// during expansion. The triples are added only after the whole document has
// converted; a store failure while adding them stops Parse and is returned
// (wrapped), leaving the triples added so far in the graph. ctx is checked
// before the conversion, which cannot be interrupted, and while adding.
func Parse(ctx context.Context, g *rdflibgo.Graph, r io.Reader, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return parse(r, &cfg, graphSink(ctx, g))
}

// sink receives the statements of a successful parse, in order. An error
// stops the delivery and is returned by the parse.
type sink func(statement) error

// graphSink adds each statement to g.
func graphSink(ctx context.Context, g *rdflibgo.Graph) sink {
	return func(st statement) error {
		if err := g.Add(ctx, st.s, st.p, st.o); err != nil {
			return fmt.Errorf("jsonld: adding to graph: %w", err)
		}
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

// addDataset is addDatasetWith for one Parse call.
func addDataset(out sink, ds *ld.RDFDataset, cfg *config, src []byte) error {
	return addDatasetWith(out, ds, cfg, newConverter(cfg.preserveBlankNodeIDs, cfg.unbounded), src)
}

// addDataset adds the RDF json-gold produced to g. It builds terms directly
// from the dataset (datasetStatements) when every node is one the N-Quads
// parser would accept unchanged, and otherwise serializes the dataset to
// N-Quads and runs parseNQuadsInto, which owns all error, skip and
// line-length behavior. cfg.forceTextPath (tests only) takes the second route
// unconditionally.
func addDatasetWith(out sink, ds *ld.RDFDataset, cfg *config, conv *converter, src []byte) error {
	if err := dropEmptyNodes(ds, cfg); err != nil {
		return err
	}
	if err := checkJSONLiterals(ds); err != nil {
		return err
	}
	if !cfg.forceTextPath {
		if stmts, ok := conv.statements(ds); ok {
			if len(stmts) == 0 {
				return nil
			}
			return commit(out, stmts, cfg, src)
		}
	}
	var sb strings.Builder
	if err := guard("serializing expanded RDF", func() error {
		return (&ld.NQuadRDFSerializer{}).SerializeTo(&sb, ds)
	}); err != nil {
		return fmt.Errorf("json-ld: serializing expanded RDF: %w", err)
	}
	if sb.Len() == 0 {
		return nil
	}
	return parseNQuadsWith(out, sb.String(), cfg, conv, src)
}

// errEmptyNode is the reason given for a statement dropEmptyNodes removes.
var errEmptyNode = fmt.Errorf("%w: an ill-formed IRI in a list left json-gold with no node", term.ErrInvalidIRI)

// ErrUnsupportedJSONLiteral is returned when the document holds a JSON
// literal ("@type": "@json") whose value json-gold cannot convert. Its
// canonicaliser accepts only a JSON object; for an array, a string or null
// it used to put its own error message into the graph as
// the literal's value, with no error. Wrapping the value in an object is the
// workaround.
var ErrUnsupportedJSONLiteral = errors.New("jsonld: the JSON-LD processor cannot convert a JSON literal (@type @json) holding an array, a string or null; wrap the value in an object")

// jsonLiteralFailures are the messages json-gold writes into an rdf:JSON
// literal's value instead of reporting an error (ld/node.go, objectToRDF).
// A real canonical JSON value starts with one of {["0-9tfn-, so none of
// these can be one.
var jsonLiteralFailures = []string{"JSON Canonicalization error ", "JSON Marshal error "}

// checkJSONLiterals turns json-gold's in-band JSON literal failures into
// ErrUnsupportedJSONLiteral.
func checkJSONLiterals(ds *ld.RDFDataset) error {
	for _, qs := range ds.Graphs {
		for _, q := range qs {
			l, ok := q.Object.(ld.Literal)
			if !ok || l.Datatype != ld.RDFJSONLiteral {
				continue
			}
			for _, prefix := range jsonLiteralFailures {
				if strings.HasPrefix(l.Value, prefix) {
					return fmt.Errorf("%w (%s)", ErrUnsupportedJSONLiteral, l.Value)
				}
			}
		}
	}
	return nil
}

// dropEmptyNodes removes the statements json-gold produced with a missing
// node. An ill-formed IRI as a list element, such as {"@list": [{"@id":
// "%"}]}, converts to no node at all, and json-gold still emits the
// rdf:first statement for it; its N-Quads serializer then panicked on the
// nil. The statement is one with an ill-formed IRI (JSON-LD 1.1 API §8.1),
// so it is dropped and reported like the others, or fails WithStrictIRIs.
func dropEmptyNodes(ds *ld.RDFDataset, cfg *config) error {
	for name, qs := range ds.Graphs {
		kept := qs[:0]
		for _, q := range qs {
			if q.Subject != nil && q.Predicate != nil && q.Object != nil {
				kept = append(kept, q)
				continue
			}
			if cfg.strictIRIs {
				return fmt.Errorf("json-ld: %s: %w", describeQuad(q), errEmptyNode)
			}
			if cfg.skipHandler != nil {
				cfg.skipHandler(describeQuad(q), errEmptyNode)
			}
		}
		ds.Graphs[name] = kept
	}
	return nil
}

// describeQuad renders a quad for a SkipHandler, with "?" for a missing node.
func describeQuad(q *ld.Quad) string {
	part := func(n ld.Node) string {
		switch v := n.(type) {
		case nil:
			return "?"
		case ld.IRI:
			return "<" + v.Value + ">"
		case ld.BlankNode:
			return v.Attribute
		default:
			return fmt.Sprintf("%q", n.GetValue())
		}
	}
	return part(q.Subject) + " " + part(q.Predicate) + " " + part(q.Object) + " ."
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
	return parseNQuadsWith(out, nqStr, cfg, newConverter(cfg.preserveBlankNodeIDs, cfg.unbounded), src)
}

// parseNQuadsWith is parseNQuadsInto with the blank-node labels of conv: the
// N-Quads parser keeps json-gold's labels and conv maps them, exactly as the
// fast path does.
func parseNQuadsWith(out sink, nqStr string, cfg *config, conv *converter, src []byte) error {
	nqOpts := make([]nq.Option, 0, 4)
	nqOpts = append(nqOpts, nq.WithPreserveBlankNodeIDs())
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
		if b, ok := s.(rdflibgo.BNode); ok {
			s = conv.bnode(b.Value())
		}
		if b, ok := o.(rdflibgo.BNode); ok {
			o = conv.bnode(b.Value())
		}
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
