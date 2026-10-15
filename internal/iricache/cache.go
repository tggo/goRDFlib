// Package iricache shares one URIRef per IRI across a parse, for the IRI
// positions that repeat: predicates and datatypes (issue #53). A document
// with ten million triples and a dozen predicates otherwise allocates and
// retains ten million copies of each predicate.
//
// Subjects and objects are deliberately not cached: they are mostly
// distinct, so every lookup would miss, and a bounded cache would fill with
// them and keep the predicates out.
package iricache

import "github.com/tggo/goRDFlib/term"

// maxEntries bounds the cache. Vocabularies are small; a document that uses
// more distinct predicates than this keeps working, the rest are just not
// shared.
const maxEntries = 4096

// Cache maps an IRI to the URIRef built for it. The zero value is not
// usable; a nil *Cache is, and caches nothing. One Cache belongs to one
// parse and is not safe for concurrent use.
type Cache struct {
	m map[string]term.URIRef
}

// New returns an empty cache.
func New() *Cache {
	return &Cache{m: make(map[string]term.URIRef, 64)}
}

// Get returns the URIRef cached for iri.
func (c *Cache) Get(iri string) (term.URIRef, bool) {
	if c == nil {
		return term.URIRef{}, false
	}
	u, ok := c.m[iri]
	return u, ok
}

// Put caches u, which must have been built from u.Value() and validated as
// the position requires: a hit skips validation.
func (c *Cache) Put(u term.URIRef) {
	if c == nil || len(c.m) >= maxEntries {
		return
	}
	c.m[u.Value()] = u
}

// Intern returns the cached URIRef for iri, building it without validation
// (term.NewURIRefUnsafe) on a miss. For parsers whose grammar has already
// checked the IRI.
func (c *Cache) Intern(iri string) term.URIRef {
	if u, ok := c.Get(iri); ok {
		return u
	}
	u := term.NewURIRefUnsafe(iri)
	c.Put(u)
	return u
}
