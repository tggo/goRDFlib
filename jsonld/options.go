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
