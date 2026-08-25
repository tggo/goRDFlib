package rdfxml

import (
	rdflibgo "github.com/tggo/goRDFlib"
)

type config struct {
	base       string
	provenance ProvenanceHandler
}

type Option func(*config)

func WithBase(base string) Option {
	return func(c *config) { c.base = base }
}

// ProvenanceHandler is called for each triple as it is added to the graph, with
// the 1-based line the triple was completed on.
//
// RDF/XML spreads a triple across a start tag, its content and an end tag, and
// the line reported is the one the parser had reached when the triple became
// known — the end of the property element for an element-form triple, the start
// element for an attribute-form one. That is the line a reader looks at to find
// the value, which is the point.
//
// The handler is called only for triples that are actually added; a parse that
// fails partway has already reported the triples it managed. The handler must
// not be nil when passed to WithProvenance.
//
// See package provenance for an index that collects these, and
// shacl.WithSourceLines for turning them into validation results that name a
// line.
type ProvenanceHandler func(s rdflibgo.Subject, p rdflibgo.URIRef, o rdflibgo.Term, lineNum int)

// WithProvenance sets a callback invoked for each triple with its 1-based
// source line number. See ProvenanceHandler for semantics. When unset there is
// zero overhead.
func WithProvenance(h ProvenanceHandler) Option {
	return func(c *config) { c.provenance = h }
}
