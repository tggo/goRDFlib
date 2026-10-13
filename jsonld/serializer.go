package jsonld

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/nq"

	"github.com/piprate/json-gold/ld"
)

// Serialize serializes a Graph to JSON-LD format.
// It converts the graph to N-Quads (preserving named graph context), then uses
// piprate/json-gold to produce JSON-LD output.
// The compacted form (the default) uses a context built from the graph's
// namespace bindings, limited to prefixes that are valid JSON-LD terms and that
// some IRI in the graph uses; see compactionContext. It is compacted even when
// no binding applies.
// Options: WithBase, WithForm/WithExpanded, WithDocumentLoader.
// The graph is read completely and the document built before the first byte is
// written, so a store failure fails Serialize with nothing written.
func Serialize(ctx context.Context, g *rdflibgo.Graph, w io.Writer, opts ...Option) error {
	var cfg config
	for _, o := range opts {
		o(&cfg)
	}
	base := cfg.base

	// Serialize to N-Quads (not N-Triples) to preserve graph context
	var nqBuf bytes.Buffer
	if err := nq.Serialize(ctx, g, &nqBuf); err != nil {
		return err
	}

	// Convert N-Quads to JSON-LD via json-gold
	proc := ld.NewJsonLdProcessor()
	ldOpts := ld.NewJsonLdOptions(base)
	ldOpts.Format = "application/n-quads"
	if cfg.documentLoader != nil {
		ldOpts.DocumentLoader = cfg.documentLoader
	}

	// json-gold is not defensive about every input and can panic rather than
	// return an error; see ErrProcessorPanic.
	var doc any
	if err := guard("conversion from N-Quads", func() error {
		var err error
		doc, err = proc.FromRDF(nqBuf.String(), ldOpts)
		return err
	}); err != nil {
		return err
	}

	var output any = doc

	// Apply compaction unless expanded form is requested
	if cfg.form != FormExpanded {
		context, err := compactionContext(ctx, g)
		if err != nil {
			return err
		}
		// Compacted even when no prefix applies: the form should not flip to
		// expanded just because the graph's bindings are unused.
		var compacted any
		if err := guard("compaction", func() error {
			var err error
			compacted, err = proc.Compact(doc, context, ldOpts)
			return err
		}); err != nil {
			return err
		}
		output = compacted
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

// compactionContext builds the @context used for compaction from the graph's
// namespace bindings. It leaves out:
//
//   - the empty prefix (Turtle's "@prefix : <...>"). The empty string is not a
//     valid term (JSON-LD 1.1 API, Create Term Definition step 3), and json-gold
//     compacted to ":p" with it anyway, which read back as zero triples.
//   - prefixes that look like keywords ("@...") or contain ':', which a
//     processor rejects or reinterprets instead of treating as a term (Create
//     Term Definition steps 4-5 and 14).
//   - prefixes no IRI in the graph uses, so the output carries only what it
//     needs.
func compactionContext(ctx context.Context, g *rdflibgo.Graph) (map[string]any, error) {
	type binding struct{ prefix, ns string }
	var candidates []binding
	for b, err := range g.Namespaces(ctx) {
		if err != nil {
			return nil, fmt.Errorf("jsonld: reading namespaces: %w", err)
		}
		prefix, ns := b.Prefix, b.Namespace
		if prefix == "" || strings.HasPrefix(prefix, "@") || strings.Contains(prefix, ":") || ns.Value() == "" {
			continue
		}
		candidates = append(candidates, binding{prefix, ns.Value()})
	}
	context := make(map[string]any, len(candidates))
	if len(candidates) == 0 {
		return context, nil
	}
	used := func(iri string) {
		for _, c := range candidates {
			if len(iri) > len(c.ns) && strings.HasPrefix(iri, c.ns) {
				context[c.prefix] = c.ns
			}
		}
	}
	var visit func(t rdflibgo.Term)
	visit = func(t rdflibgo.Term) {
		switch v := t.(type) {
		case rdflibgo.URIRef:
			used(v.Value())
		case rdflibgo.Literal:
			// Plain and language-tagged strings carry no datatype in JSON-LD.
			if v.Language() == "" && v.Datatype() != rdflibgo.XSDString {
				used(v.Datatype().Value())
			}
		case rdflibgo.TripleTerm:
			visit(v.Subject())
			visit(v.Predicate())
			visit(v.Object())
		}
	}
	visit(g.Identifier()) // nq.Serialize writes a URIRef identifier as the graph name
	for t, err := range g.Triples(ctx, nil, nil, nil) {
		if err != nil {
			return nil, fmt.Errorf("jsonld: reading triples: %w", err)
		}
		visit(t.Subject)
		// FromRDF turns rdf:type with an IRI or blank node object into @type
		// (useRdfType is off), so that predicate never appears in the output.
		if _, isLit := t.Object.(rdflibgo.Literal); t.Predicate != rdflibgo.RDF.Type || isLit {
			visit(t.Predicate)
		}
		visit(t.Object)
	}
	return context, nil
}
