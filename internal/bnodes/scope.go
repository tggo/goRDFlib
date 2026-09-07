// Package bnodes gives document-local blank node labels identities that can be
// merged into a graph unchanged.
package bnodes

import "github.com/tggo/goRDFlib/term"

// Scope maps the blank node labels of one document to fresh identities, the way
// rdflib's parsers do. Its zero value preserves labels; parsers create a fresh
// scope with New for each document.
//
// A label is replaced, not prefixed. Prefixing (scope + "_" + label) would keep
// the source label readable but makes every parse → serialize → parse hop grow
// the label by another prefix, without bound, and a pipeline that moves data
// through N-Triples several times does exactly that.
//
// Not safe for concurrent use: a parse runs on one goroutine and owns its scope.
type Scope struct {
	labels map[string]term.BNode // nil preserves labels
}

// New allocates a fresh scope unless the caller explicitly preserves labels.
func New(preserve bool) Scope {
	if preserve {
		return Scope{}
	}
	return Scope{labels: make(map[string]term.BNode, 16)}
}

// Label returns the identity of a labelled blank node in this scope: the same
// node for a repeated label, a fresh generated node for a new one.
func (s Scope) Label(label string) term.BNode {
	if s.labels == nil {
		return term.NewBNode(label)
	}
	if b, ok := s.labels[label]; ok {
		return b
	}
	b := term.NewBNode()
	s.labels[label] = b
	return b
}
