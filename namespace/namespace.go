package namespace

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
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
func (ns Namespace) Term(name string) term.URIRef {
	return term.NewURIRefUnsafe(ns.base + name)
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
func (ns Namespace) URIRef() term.URIRef {
	return term.NewURIRefUnsafe(ns.base)
}

// ClosedNamespace only allows predefined terms.
// Ported from: rdflib.namespace.ClosedNamespace
type ClosedNamespace struct {
	base  string
	terms map[string]term.URIRef
}

// NewClosedNamespace creates a ClosedNamespace with a fixed set of allowed terms.
func NewClosedNamespace(base string, terms []string) ClosedNamespace {
	m := make(map[string]term.URIRef, len(terms))
	for _, t := range terms {
		m[t] = term.NewURIRefUnsafe(base + t)
	}
	return ClosedNamespace{base: base, terms: m}
}

// Term returns the URIRef for the given term name, or error if not defined.
// Ported from: rdflib.namespace.ClosedNamespace.__getattr__
func (ns ClosedNamespace) Term(name string) (term.URIRef, error) {
	if u, ok := ns.terms[name]; ok {
		return u, nil
	}
	return term.URIRef{}, fmt.Errorf("%w: %q in %s", term.ErrTermNotInNamespace, name, ns.base)
}

// MustTerm panics if the term is not defined. For use with known-good constants.
func (ns ClosedNamespace) MustTerm(name string) term.URIRef {
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
// All methods are safe for concurrent use.
// Ported from: rdflib.namespace.NamespaceManager
type NSManager struct {
	mu    sync.RWMutex
	store store.Store
	cache map[string][3]string // uri → [prefix, ns, local]
	genID int                  // for auto-generated prefixes
}

// NewNSManager creates a new NamespaceManager backed by the given store.
func NewNSManager(s store.Store) *NSManager {
	return &NSManager{
		store: s,
		cache: make(map[string][3]string),
	}
}

// Bind associates a prefix with a namespace.
// The entire check-then-act sequence is protected by the mutex to prevent
// TOCTOU races when override=false.
// Ported from: rdflib.namespace.NamespaceManager.bind
func (m *NSManager) Bind(ctx context.Context, prefix string, namespace term.URIRef, override bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !override {
		_, ok, err := m.store.Namespace(ctx, prefix)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
	}
	if err := m.store.Bind(ctx, prefix, namespace); err != nil {
		return err
	}
	// Selectively invalidate cache entries whose namespace matches the newly bound one.
	nsStr := namespace.Value()
	for uri := range m.cache {
		if strings.HasPrefix(uri, nsStr) {
			delete(m.cache, uri)
		}
	}
	return nil
}

// Prefix implements the term.NamespaceManager interface for Term.N3. That
// interface has no context and no error, so the store is asked with
// context.Background() and a failed lookup reads as "no prefix".
func (m *NSManager) Prefix(uri string) (string, bool) {
	qn, err := m.QName(context.Background(), uri)
	if err != nil {
		return "", false
	}
	return qn, true
}

// QName returns the prefixed form of a URI (e.g. "foaf:Person").
// Ported from: rdflib.namespace.NamespaceManager.qname
func (m *NSManager) QName(ctx context.Context, uri string) (string, error) {
	prefix, _, local, err := m.ComputeQName(ctx, uri)
	if err != nil {
		return "", err
	}
	return prefix + ":" + local, nil
}

// ComputeQName splits a URI into prefix, namespace, and local name.
// If no prefix exists for the namespace, one is auto-generated (ns1, ns2, ...).
// Thread-safe: the generate-and-bind sequence is atomic.
// Ported from: rdflib.namespace.NamespaceManager.compute_qname
func (m *NSManager) ComputeQName(ctx context.Context, uri string) (prefix, ns, local string, err error) {
	m.mu.RLock()
	if cached, ok := m.cache[uri]; ok {
		m.mu.RUnlock()
		return cached[0], cached[1], cached[2], nil
	}
	m.mu.RUnlock()

	// Split URI into namespace + local name
	nsStr, localName := SplitURI(uri)
	if nsStr == "" {
		return "", "", "", fmt.Errorf("cannot compute qname for %q", uri)
	}

	nsRef := term.NewURIRefUnsafe(nsStr)

	// Look up existing prefix
	p, ok, err := m.store.Prefix(ctx, nsRef)
	if err != nil {
		return "", "", "", err
	}
	if ok {
		prefix = p
	} else if prefix, err = m.generatePrefix(ctx, nsRef); err != nil {
		return "", "", "", err
	}

	m.mu.Lock()
	m.cache[uri] = [3]string{prefix, nsStr, localName}
	m.mu.Unlock()

	return prefix, nsStr, localName, nil
}

// generatePrefix binds nsRef to the first free nsN prefix, unless another
// caller bound it meanwhile. The generate-and-bind sequence runs under the
// write lock so two callers cannot pick the same prefix.
func (m *NSManager) generatePrefix(ctx context.Context, nsRef term.URIRef) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p, ok, err := m.store.Prefix(ctx, nsRef); err != nil || ok {
		return p, err
	}
	for {
		m.genID++
		prefix := fmt.Sprintf("ns%d", m.genID)
		_, exists, err := m.store.Namespace(ctx, prefix)
		if err != nil {
			return "", err
		}
		if !exists {
			return prefix, m.store.Bind(ctx, prefix, nsRef)
		}
	}
}

// Absolutize resolves a URI against the store's known namespace bindings.
// If the URI is already absolute, it is returned as-is. If it looks like a
// CURIE (contains a colon with a bound prefix), it is expanded.
// Ported from: rdflib.namespace.NamespaceManager.absolutize
func (m *NSManager) Absolutize(ctx context.Context, uri string) (term.URIRef, error) {
	// If it looks like it could be a CURIE, try expanding it.
	if parts := strings.SplitN(uri, ":", 2); len(parts) == 2 {
		ns, ok, err := m.store.Namespace(ctx, parts[0])
		if err != nil {
			return term.URIRef{}, err
		}
		if ok {
			return term.NewURIRefUnsafe(ns.Value() + parts[1]), nil
		}
	}
	// Already absolute or no matching prefix — return as-is.
	return term.NewURIRefUnsafe(uri), nil
}

// ExpandCURIE expands a prefixed name (e.g. "foaf:Person") to a full URIRef.
// An empty prefix is valid and represents the default namespace binding
// (i.e., ":localname" looks up the "" prefix in the store).
// Ported from: rdflib.namespace.NamespaceManager.expand_curie
func (m *NSManager) ExpandCURIE(ctx context.Context, curie string) (term.URIRef, error) {
	parts := strings.SplitN(curie, ":", 2)
	if len(parts) != 2 {
		return term.URIRef{}, fmt.Errorf("%w: %q", term.ErrInvalidCURIE, curie)
	}
	ns, ok, err := m.store.Namespace(ctx, parts[0])
	if err != nil {
		return term.URIRef{}, err
	}
	if !ok {
		return term.URIRef{}, fmt.Errorf("%w: %q", term.ErrPrefixNotBound, parts[0])
	}
	return term.NewURIRefUnsafe(ns.Value() + parts[1]), nil
}

// Namespaces returns all prefix→namespace bindings.
// Ported from: rdflib.namespace.NamespaceManager.namespaces
func (m *NSManager) Namespaces(ctx context.Context) store.NamespaceIterator {
	return m.store.Namespaces(ctx)
}

// SplitURI splits a URI into namespace and local name at the last '#' or '/'.
// For URN-style URIs (e.g., "urn:isbn:12345") that contain no '#' or '/',
// the last ':' is used as the separator instead. If no separator is found
// at all, the namespace is empty and the entire URI is returned as the local name.
// Ported from: rdflib.namespace.split_uri (simplified)
func SplitURI(uri string) (ns, local string) {
	// Try '#' first
	if i := strings.LastIndex(uri, "#"); i >= 0 {
		return uri[:i+1], uri[i+1:]
	}
	// Then '/'
	if i := strings.LastIndex(uri, "/"); i >= 0 {
		return uri[:i+1], uri[i+1:]
	}
	// Fallback to ':' for URN-style URIs (e.g., urn:isbn:12345)
	if i := strings.LastIndex(uri, ":"); i >= 0 {
		return uri[:i+1], uri[i+1:]
	}
	return "", uri
}
