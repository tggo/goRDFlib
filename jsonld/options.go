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
	strictIRIs           bool
	skipHandler          SkipHandler
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

// WithSkipInvalidIRIs used to make parsing tolerant of ill-formed IRIs (e.g. a
// stray space, as in "schema: Dataset") that the JSON-LD expander emits into the
// intermediate N-Quads. Dropping those statements is now the default, as the
// JSON-LD 1.1 API requires (§8.1), so this option does nothing.
//
// Deprecated: skipping is the default. Use WithSkipHandler to see what was
// dropped, or WithStrictIRIs to fail instead.
func WithSkipInvalidIRIs() Option {
	return func(c *config) {}
}

// SkipHandler receives each statement dropped while parsing because it holds an
// ill-formed IRI. statement is the intermediate N-Quads line json-gold produced
// (not a line of the JSON-LD source) and err says which IRI was rejected and
// why: errors.Is(err, nt.ErrRelativeIRI) for a relative IRI, and nt.ErrInvalidIRI
// or rdflibgo.ErrInvalidIRI for a character no IRI may contain.
type SkipHandler func(statement string, err error)

// WithSkipHandler sets a callback for statements dropped because of an
// ill-formed IRI. JSON-LD 1.1 API §8.1 drops such statements silently (for
// instance "@type": "@id" on a node object, rdflib #2168); the handler is how a
// caller finds out that data was left out. It has no effect with WithStrictIRIs.
func WithSkipHandler(h SkipHandler) Option {
	return func(c *config) { c.skipHandler = h }
}

// WithStrictIRIs makes an ill-formed IRI in the expanded document a parse error
// instead of a dropped statement. The graph is left unchanged when that happens.
// This departs from JSON-LD 1.1 API §8.1; use it when silently losing a
// statement is worse than rejecting the document.
func WithStrictIRIs() Option {
	return func(c *config) { c.strictIRIs = true }
}

// WithUnboundedLines parses intermediate N-Quads lines of arbitrary length,
// growing the read buffer as needed. This is useful when JSON-LD expansion
// produces very large literal values on a single N-Quads line.
func WithUnboundedLines() Option {
	return func(c *config) { c.unbounded = true }
}
