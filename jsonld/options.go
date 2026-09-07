package jsonld

import "github.com/piprate/json-gold/ld"

// OutputForm specifies the JSON-LD output format.
type OutputForm int

const (
	// FormCompacted applies JSON-LD compaction using namespace bindings (default).
	FormCompacted OutputForm = iota
	// FormExpanded outputs expanded JSON-LD without compaction.
	FormExpanded
)

type config struct {
	base                 string
	form                 OutputForm
	documentLoader       ld.DocumentLoader
	skipInvalidIRI       bool
	unbounded            bool
	provenance           ProvenanceHandler
	preserveBlankNodeIDs bool
	expandContext        any
}

// Option configures JSON-LD parsing or serialization.
type Option func(*config)

// WithPreserveBlankNodeIDs keeps the blank-node labels in the intermediate
// N-Quads instead of scoping them to each Parse call. This restores the previous
// behavior, which can merge unrelated blank nodes when documents share a graph.
// It affects parsing only.
//
// JSON-gold relabels source blank nodes before it produces N-Quads and has no
// option to retain source labels. Thus this option preserves its generated
// labels, including those for anonymous nodes, not the source @id values.
func WithPreserveBlankNodeIDs() Option {
	return func(c *config) { c.preserveBlankNodeIDs = true }
}

// WithBase sets the base IRI for JSON-LD processing.
func WithBase(base string) Option {
	return func(c *config) { c.base = base }
}

// WithForm sets the output form for JSON-LD serialization (compact or expanded).
func WithForm(form OutputForm) Option {
	return func(c *config) { c.form = form }
}

// WithExpanded is a convenience option to request expanded JSON-LD output.
func WithExpanded() Option {
	return func(c *config) { c.form = FormExpanded }
}

// WithDocumentLoader sets a custom document loader for remote context resolution.
func WithDocumentLoader(loader ld.DocumentLoader) Option {
	return func(c *config) { c.documentLoader = loader }
}

// WithExpandContext supplies a context that is applied before the document's
// own @context when parsing (the expandContext option of the JSON-LD API,
// https://www.w3.org/TR/json-ld11-api/#dom-jsonldoptions-expandcontext).
//
// The value is what a JSON-LD context may be: a context definition
// (map[string]any), an IRI string that the document loader resolves, an array of
// those, or a whole document carrying a "@context" key, which is unwrapped. The
// document's own @context is processed on top of it, so a term the document
// defines wins over the same term defined here — this is the place for defaults
// and for pre-declaring terms a document relies on but never declares, not for
// overriding what the document says. Relative IRIs inside it resolve against
// WithBase.
//
// Provenance (WithProvenance) expands written identifiers through this context
// as well, so an @id that only makes sense given the expand context still gets
// its source line. Aliases of @id declared in an inline expand context are seen;
// those that arrive through an IRI-valued one are not, as with remote contexts.
//
// It affects parsing only; the serializer ignores it.
func WithExpandContext(ctx any) Option {
	return func(c *config) { c.expandContext = ctx }
}

// WithSkipInvalidIRIs makes parsing tolerant of syntactically invalid IRIs (e.g.
// a stray space, as in "schema: Dataset") that the JSON-LD expander emits into
// the intermediate N-Quads instead of dropping. The offending triple is silently
// skipped and parsing continues, rather than failing the whole document.
//
// This matches Python rdflib/pySHACL, which drop such triples before validation.
// The default (option unset) is strict: an invalid IRI is a hard error. Note that
// the bundled JSON-LD processor already drops most malformed IRIs on its own; this
// option is a safety net for IRIs that slip through to the N-Quads layer.
func WithSkipInvalidIRIs() Option {
	return func(c *config) { c.skipInvalidIRI = true }
}

// WithUnboundedLines parses intermediate N-Quads lines of arbitrary length,
// growing the read buffer as needed. This is useful when JSON-LD expansion
// produces very large literal values on a single N-Quads line.
func WithUnboundedLines() Option {
	return func(c *config) { c.unbounded = true }
}
