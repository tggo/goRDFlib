// Package bnodes assigns document-local labels identities that can be merged unchanged.
package bnodes

import "github.com/tggo/goRDFlib/term"

// Scope is an immutable parse scope, safe for concurrent use. Its zero value
// preserves labels; parsers create a fresh scope with New for each document.
type Scope struct {
	prefix string
}

// New allocates a fresh scope unless the caller explicitly preserves labels.
func New(preserve bool) Scope {
	if preserve {
		return Scope{}
	}
	return Scope{prefix: term.NewBNode().Value() + "_"}
}

// Label gives repeated labels the same identity without retaining a lookup map.
// The fixed-width UUID prefix separates scopes. Its underscore also separates
// labelled nodes from anonymous NewBNode identifiers, which contain no underscore.
func (s Scope) Label(label string) term.BNode {
	return term.NewBNode(s.prefix + label)
}
