package rdflibgo

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// NamespaceManager provides prefix lookup for compact term representations.
type NamespaceManager interface {
	Prefix(uri string) (string, bool)
}

// Term is the interface implemented by all RDF term types.
type Term interface {
	// N3 returns the N-Triples/N3 representation of the term.
	// An optional NamespaceManager can be provided for prefixed output.
	N3(ns ...NamespaceManager) string

	// String returns a human-readable string representation.
	String() string

	// Equal returns true if this term is identical to other.
	Equal(other Term) bool

	// termType is a sealed marker preventing external implementations.
	termType() string
}

// Subject can be URIRef or BNode.
// The subject() marker method restricts implementations to URIRef and BNode.
type Subject interface {
	Term
	subject()
}

// Predicate is always URIRef.
type Predicate = URIRef

// --- URIRef ---

// URIRef represents an IRI reference.
// Ported from: rdflib.term.URIRef
type URIRef struct {
	value string
}

func (u URIRef) subject()        {}
func (u URIRef) termType() string { return "URIRef" }

// Value returns the IRI string.
func (u URIRef) Value() string { return u.value }

// String returns the IRI string.
func (u URIRef) String() string { return u.value }

// Equal returns true if other is a URIRef with the same value.
func (u URIRef) Equal(other Term) bool {
	if o, ok := other.(URIRef); ok {
		return u.value == o.value
	}
	return false
}

// N3 returns the N-Triples representation: <iri>.
// If a NamespaceManager is provided and can abbreviate the IRI, the prefixed form is returned.
func (u URIRef) N3(ns ...NamespaceManager) string {
	if len(ns) > 0 && ns[0] != nil {
		if prefix, ok := ns[0].Prefix(u.value); ok {
			return prefix
		}
	}
	return "<" + u.value + ">"
}

// Defrag returns a new URIRef without the fragment identifier.
func (u URIRef) Defrag() URIRef {
	if i := strings.Index(u.value, "#"); i >= 0 {
		return URIRef{value: u.value[:i]}
	}
	return u
}

// Fragment returns the fragment identifier (without #), or empty string.
func (u URIRef) Fragment() string {
	if i := strings.Index(u.value, "#"); i >= 0 {
		return u.value[i+1:]
	}
	return ""
}

// isValidIRI checks that an IRI does not contain forbidden characters per RFC 3987.
// Forbidden: < > " space { } | \ ^ `
// Ported from: rdflib.term._is_valid_uri
func isValidIRI(s string) bool {
	for _, c := range s {
		switch c {
		case '<', '>', '"', ' ', '{', '}', '|', '\\', '^', '`':
			return false
		}
	}
	return true
}

// NewURIRef creates a new URIRef, validating that it contains no forbidden characters.
// Ported from: rdflib.term.URIRef.__new__
func NewURIRef(value string) (URIRef, error) {
	if !isValidIRI(value) {
		return URIRef{}, fmt.Errorf("%w: %q contains forbidden characters", ErrInvalidIRI, value)
	}
	return URIRef{value: value}, nil
}

// NewURIRefWithBase creates a new URIRef by resolving value against a base IRI.
// Ported from: rdflib.term.URIRef.__new__ with base parameter
func NewURIRefWithBase(value, base string) (URIRef, error) {
	if base != "" {
		b, err := url.Parse(base)
		if err != nil {
			return URIRef{}, fmt.Errorf("%w: invalid base %q: %v", ErrInvalidIRI, base, err)
		}
		ref, err := url.Parse(value)
		if err != nil {
			return URIRef{}, fmt.Errorf("%w: %q: %v", ErrInvalidIRI, value, err)
		}
		value = b.ResolveReference(ref).String()
	}
	return NewURIRef(value)
}

// --- BNode ---

// BNode represents a blank node.
// Ported from: rdflib.term.BNode
type BNode struct {
	value string
}

func (b BNode) subject()        {}
func (b BNode) termType() string { return "BNode" }

// Value returns the blank node identifier.
func (b BNode) Value() string { return b.value }

// String returns the blank node identifier.
func (b BNode) String() string { return b.value }

// Equal returns true if other is a BNode with the same identifier.
func (b BNode) Equal(other Term) bool {
	if o, ok := other.(BNode); ok {
		return b.value == o.value
	}
	return false
}

// N3 returns the N-Triples representation: _:id.
func (b BNode) N3(ns ...NamespaceManager) string {
	return "_:" + b.value
}

// Skolemize returns a URIRef that deterministically represents this blank node.
// Ported from: rdflib.term.BNode.skolemize
func (b BNode) Skolemize(authority string) URIRef {
	if !strings.HasSuffix(authority, "/") {
		authority += "/"
	}
	return NewURIRefUnsafe(authority + ".well-known/genid/" + b.value)
}

// NewBNode creates a new BNode with a unique auto-generated identifier.
// The id format is "N" + 32 hex chars from a UUID4, matching Python rdflib's default.
// Ported from: rdflib.term.BNode.__new__
func NewBNode(id ...string) BNode {
	if len(id) > 0 && id[0] != "" {
		return BNode{value: id[0]}
	}
	u := uuid.New()
	return BNode{value: "N" + hex.EncodeToString(u[:])}
}

// --- Variable ---

// Variable represents a SPARQL query variable.
// Ported from: rdflib.term.Variable
type Variable struct {
	Name string
}

func (v Variable) termType() string { return "Variable" }

// String returns "?name".
func (v Variable) String() string {
	return "?" + v.Name
}

// N3 returns "?name".
func (v Variable) N3(ns ...NamespaceManager) string {
	return "?" + v.Name
}

// Equal returns true if other is a Variable with the same name.
func (v Variable) Equal(other Term) bool {
	if o, ok := other.(Variable); ok {
		return v.Name == o.Name
	}
	return false
}

// NewVariable creates a new Variable.
func NewVariable(name string) Variable {
	return Variable{Name: name}
}
