package turtle

import rdflibgo "github.com/tggo/goRDFlib"

type config struct {
	base       string
	provenance ProvenanceHandler
}

// Option configures turtle parsing or serialization.
type Option func(*config)

// ProvenanceHandler is called for each triple as it is added to the graph, with
// the 1-based source line number the triple was emitted from. Use it to map
// triples back to their origin in the input (e.g. so a SHACL report can cite a
// line number). The handler must not be nil when passed to WithProvenance.
//
// Unlike line-oriented N-Triples, a Turtle statement can span multiple lines and
// a single line can expand to many triples (collections, blank-node property
// lists). The reported line is the parser's position at the point the triple is
// emitted, which is the best available approximation of its origin.
type ProvenanceHandler func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, lineNum int)

// WithBase sets the base IRI for resolving relative IRIs.
func WithBase(base string) Option {
	return func(c *config) { c.base = base }
}

// WithProvenance sets a callback invoked for each triple with its 1-based source
// line number. See ProvenanceHandler for semantics. When unset there is zero
// per-triple overhead.
func WithProvenance(h ProvenanceHandler) Option {
	return func(c *config) { c.provenance = h }
}
