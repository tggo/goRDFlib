package trig

import rdflibgo "github.com/tggo/goRDFlib"

type config struct {
	base                 string
	provenance           ProvenanceHandler
	preserveBlankNodeIDs bool
}

// Option configures TriG parsing or serialization.
type Option func(*config)

// ProvenanceHandler is called for each quad as it is added, with the 1-based
// source line number it was emitted from. The graph term is the identifier of
// the active graph (the default graph's identifier for triples outside a GRAPH
// block). Use it to map quads back to their origin in the input (e.g. so a
// SHACL report can cite a line number). The handler must not be nil when passed
// to WithProvenance.
//
// As in Turtle, a TriG statement can span multiple lines and a single line can
// expand to many triples (collections, blank-node property lists). The reported
// line is the parser's position at the point the triple is emitted, which is the
// best available approximation of its origin.
type ProvenanceHandler func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, graph rdflibgo.Term, lineNum int)

// WithPreserveBlankNodeIDs keeps labelled blank node IDs exactly as written in
// the input, including graph names. By default, each Parse or ParseDataset call
// gives labels a fresh document scope shared across all graphs. Use this option
// only when IDs must be stable: separate documents with the same label can then
// merge nodes and graph names. Anonymous blank nodes remain fresh.
func WithPreserveBlankNodeIDs() Option {
	return func(c *config) { c.preserveBlankNodeIDs = true }
}

// WithBase sets the base IRI for resolving relative IRIs.
func WithBase(base string) Option {
	return func(c *config) { c.base = base }
}

// WithProvenance sets a callback invoked for each quad with its 1-based source
// line number. See ProvenanceHandler for semantics. When unset there is zero
// per-triple overhead.
func WithProvenance(h ProvenanceHandler) Option {
	return func(c *config) { c.provenance = h }
}
