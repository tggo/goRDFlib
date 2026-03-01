package rdflibgo

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// NamespaceManager is a placeholder for namespace prefix mappings (future phase).
type NamespaceManager interface {
	Prefix(uri string) (string, bool)
}

// Term is the interface implemented by all RDF term types.
type Term interface {
	N3(ns ...NamespaceManager) string
	String() string
	termType() string
}

// Subject can be URIRef or BNode.
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
func (u URIRef) Value() string    { return u.value }

func (u URIRef) String() string {
	return u.value
}

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

// invalidIRIChars contains characters not allowed in IRIs.
var invalidIRIChars = strings.NewReplacer() // we'll use a set instead

func isValidIRI(s string) bool {
	for _, c := range s {
		switch c {
		case '<', '>', '"', ' ', '{', '}', '|', '\\', '^', '`':
			return false
		}
	}
	return true
}

// NewURIRef creates a new URIRef, optionally resolving against a base IRI.
func NewURIRef(value string, base ...string) (URIRef, error) {
	if len(base) > 0 && base[0] != "" {
		b, err := url.Parse(base[0])
		if err != nil {
			return URIRef{}, fmt.Errorf("invalid base IRI: %w", err)
		}
		ref, err := url.Parse(value)
		if err != nil {
			return URIRef{}, fmt.Errorf("invalid IRI: %w", err)
		}
		value = b.ResolveReference(ref).String()
	}
	if !isValidIRI(value) {
		return URIRef{}, fmt.Errorf("invalid IRI: %q contains forbidden characters", value)
	}
	return URIRef{value: value}, nil
}

// --- BNode ---

// BNode represents a blank node.
// Ported from: rdflib.term.BNode
type BNode struct {
	value string
}

func (b BNode) subject()        {}
func (b BNode) termType() string { return "BNode" }
func (b BNode) Value() string    { return b.value }

func (b BNode) String() string {
	return b.value
}

func (b BNode) N3(ns ...NamespaceManager) string {
	return "_:" + b.value
}

// Skolemize returns a URIRef that deterministically represents this blank node.
func (b BNode) Skolemize(authority string) URIRef {
	if !strings.HasSuffix(authority, "/") {
		authority += "/"
	}
	return NewURIRefUnsafe(authority + ".well-known/genid/" + b.value)
}

// NewBNode creates a new BNode. If no id is provided, a unique one is generated.
func NewBNode(id ...string) BNode {
	if len(id) > 0 && id[0] != "" {
		return BNode{value: id[0]}
	}
	u := uuid.New()
	hex := strings.ReplaceAll(u.String(), "-", "")
	return BNode{value: "N" + hex}
}

// --- Variable ---

// Variable represents a query variable.
// Ported from: rdflib.term.Variable
type Variable struct {
	Name string
}

func (v Variable) termType() string { return "Variable" }

func (v Variable) String() string {
	return "?" + v.Name
}

func (v Variable) N3(ns ...NamespaceManager) string {
	return "?" + v.Name
}

// NewVariable creates a new Variable.
func NewVariable(name string) Variable {
	return Variable{Name: name}
}
