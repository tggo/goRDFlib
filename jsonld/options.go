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
	base           string
	form           OutputForm
	documentLoader ld.DocumentLoader
	skipInvalidIRI bool
	unbounded      bool
}

// Option configures JSON-LD parsing or serialization.
type Option func(*config)

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
