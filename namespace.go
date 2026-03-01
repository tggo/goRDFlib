package rdflibgo

import (
	"fmt"
	"strings"
	"sync"
)

// Namespace represents an RDF namespace — a base URI to which local names are appended.
// Ported from: rdflib.namespace.Namespace
type Namespace struct {
	base string
}

// NewNamespace creates a new Namespace with the given base URI.
func NewNamespace(base string) Namespace {
	return Namespace{base: base}
}

// Term creates a URIRef by appending name to the namespace base.
// Ported from: rdflib.namespace.Namespace.__getattr__
func (ns Namespace) Term(name string) URIRef {
	return NewURIRefUnsafe(ns.base + name)
}

// Base returns the base URI string.
func (ns Namespace) Base() string {
	return ns.base
}

// Contains checks if a URI belongs to this namespace.
// Ported from: rdflib.namespace.Namespace.__contains__
func (ns Namespace) Contains(uri string) bool {
	return strings.HasPrefix(uri, ns.base)
}

// URIRef returns the namespace base as a URIRef.
func (ns Namespace) URIRef() URIRef {
	return NewURIRefUnsafe(ns.base)
}

// ClosedNamespace only allows predefined terms.
// Ported from: rdflib.namespace.ClosedNamespace
type ClosedNamespace struct {
	base  string
	terms map[string]URIRef
}

// NewClosedNamespace creates a ClosedNamespace with a fixed set of allowed terms.
func NewClosedNamespace(base string, terms []string) ClosedNamespace {
	m := make(map[string]URIRef, len(terms))
	for _, t := range terms {
		m[t] = NewURIRefUnsafe(base + t)
	}
	return ClosedNamespace{base: base, terms: m}
}

// Term returns the URIRef for the given term name, or error if not defined.
// Ported from: rdflib.namespace.ClosedNamespace.__getattr__
func (ns ClosedNamespace) Term(name string) (URIRef, error) {
	if u, ok := ns.terms[name]; ok {
		return u, nil
	}
	return URIRef{}, fmt.Errorf("term %q not in closed namespace %s", name, ns.base)
}

// MustTerm panics if the term is not defined. For use with known-good constants.
func (ns ClosedNamespace) MustTerm(name string) URIRef {
	u, err := ns.Term(name)
	if err != nil {
		panic(err)
	}
	return u
}

// Base returns the base URI string.
func (ns ClosedNamespace) Base() string {
	return ns.base
}

// --- NamespaceManager ---

// NSManager manages prefix ↔ namespace bindings.
// Ported from: rdflib.namespace.NamespaceManager
type NSManager struct {
	mu    sync.RWMutex
	store Store
	cache map[string][3]string // uri → [prefix, ns, local]
	genID int                  // for auto-generated prefixes
}

// NewNSManager creates a new NamespaceManager backed by the given store.
func NewNSManager(store Store) *NSManager {
	return &NSManager{
		store: store,
		cache: make(map[string][3]string),
	}
}

// Bind associates a prefix with a namespace.
// Ported from: rdflib.namespace.NamespaceManager.bind
func (m *NSManager) Bind(prefix string, namespace URIRef, override bool) {
	if override {
		m.store.Bind(prefix, namespace)
	} else {
		if _, ok := m.store.Namespace(prefix); !ok {
			m.store.Bind(prefix, namespace)
		}
	}
	// Invalidate cache
	m.mu.Lock()
	m.cache = make(map[string][3]string)
	m.mu.Unlock()
}

// Prefix implements NamespaceManager interface for use with Term.N3().
func (m *NSManager) Prefix(uri string) (string, bool) {
	qn, err := m.QName(uri)
	if err != nil {
		return "", false
	}
	return qn, true
}

// QName returns the prefixed form of a URI (e.g. "foaf:Person").
// Ported from: rdflib.namespace.NamespaceManager.qname
func (m *NSManager) QName(uri string) (string, error) {
	prefix, _, local, err := m.ComputeQName(uri)
	if err != nil {
		return "", err
	}
	return prefix + ":" + local, nil
}

// ComputeQName splits a URI into prefix, namespace, and local name.
// Ported from: rdflib.namespace.NamespaceManager.compute_qname
func (m *NSManager) ComputeQName(uri string) (prefix, ns, local string, err error) {
	m.mu.RLock()
	if cached, ok := m.cache[uri]; ok {
		m.mu.RUnlock()
		return cached[0], cached[1], cached[2], nil
	}
	m.mu.RUnlock()

	// Split URI into namespace + local name
	nsStr, localName := splitURI(uri)
	if nsStr == "" {
		return "", "", "", fmt.Errorf("cannot compute qname for %q", uri)
	}

	nsRef := NewURIRefUnsafe(nsStr)

	// Look up existing prefix
	if p, ok := m.store.Prefix(nsRef); ok {
		prefix = p
	} else {
		// Auto-generate prefix
		m.mu.Lock()
		for {
			m.genID++
			prefix = fmt.Sprintf("ns%d", m.genID)
			if _, exists := m.store.Namespace(prefix); !exists {
				break
			}
		}
		m.mu.Unlock()
		m.store.Bind(prefix, nsRef)
	}

	m.mu.Lock()
	m.cache[uri] = [3]string{prefix, nsStr, localName}
	m.mu.Unlock()

	return prefix, nsStr, localName, nil
}

// ExpandCURIE expands a prefixed name to a full URIRef.
// Ported from: rdflib.namespace.NamespaceManager.expand_curie
func (m *NSManager) ExpandCURIE(curie string) (URIRef, error) {
	parts := strings.SplitN(curie, ":", 2)
	if len(parts) != 2 {
		return URIRef{}, fmt.Errorf("invalid CURIE: %q", curie)
	}
	ns, ok := m.store.Namespace(parts[0])
	if !ok {
		return URIRef{}, fmt.Errorf("prefix %q not bound", parts[0])
	}
	return NewURIRefUnsafe(ns.Value() + parts[1]), nil
}

// Namespaces returns all prefix→namespace bindings.
// Ported from: rdflib.namespace.NamespaceManager.namespaces
func (m *NSManager) Namespaces() NamespaceIterator {
	return m.store.Namespaces()
}

// splitURI splits a URI into namespace and local name at the last '#' or '/'.
// Ported from: rdflib.namespace.split_uri (simplified)
func splitURI(uri string) (ns, local string) {
	// Try '#' first
	if i := strings.LastIndex(uri, "#"); i >= 0 {
		return uri[:i+1], uri[i+1:]
	}
	// Then '/'
	if i := strings.LastIndex(uri, "/"); i >= 0 {
		return uri[:i+1], uri[i+1:]
	}
	return "", uri
}
