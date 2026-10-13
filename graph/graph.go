package graph

import (
	"context"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// Graph is an RDF graph backed by a Store.
//
// Every method that reaches the store takes a context.Context and reports the
// store's failure as an error; iterators yield it as a (zero, err) pair and
// stop. Safe for concurrent reads when backed by a thread-safe Store
// (MemoryStore, BadgerStore, SQLiteStore). Write operations require external
// synchronization.
// Ported from: rdflib.graph.Graph
type Graph struct {
	store      store.Store
	identifier term.Term
	base       string
}

// GraphOption configures Graph construction.
type GraphOption func(*Graph)

// WithStore sets the backing store.
func WithStore(s store.Store) GraphOption {
	return func(g *Graph) { g.store = s }
}

// DefaultGraph is the identifier of a graph created without WithIdentifier,
// and of a Dataset's default context. Stores address the default graph with it;
// see store.DefaultGraph.
var DefaultGraph = store.DefaultGraph

// WithIdentifier sets the graph identifier. Any term other than DefaultGraph —
// an IRI or a blank node — names a graph, so graphs over one store with
// different identifiers hold different triples.
func WithIdentifier(id term.Term) GraphOption {
	return func(g *Graph) { g.identifier = id }
}

// WithBase sets the base URI for relative resolution.
func WithBase(base string) GraphOption {
	return func(g *Graph) { g.base = base }
}

// defaultBindings are the prefixes a new graph starts with.
var defaultBindings = []store.NamespaceBinding{
	{Prefix: "rdf", Namespace: term.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#")},
	{Prefix: "rdfs", Namespace: term.NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#")},
	{Prefix: "xsd", Namespace: term.NewURIRefUnsafe(term.XSDNamespace)},
	{Prefix: "owl", Namespace: term.NewURIRefUnsafe("http://www.w3.org/2002/07/owl#")},
}

// NewGraph creates a new Graph.
//
// A graph over a MemoryStore (the default) starts with the rdf, rdfs, xsd and
// owl prefixes bound. A graph over any other store does not: binding is a
// write, which can fail, and a constructor has no error to report it with.
// Call BindDefaults for them.
// Ported from: rdflib.graph.Graph.__init__
func NewGraph(opts ...GraphOption) *Graph {
	g := &Graph{}
	for _, opt := range opts {
		opt(g)
	}
	if g.store == nil {
		g.store = store.NewMemoryStore()
	}
	if g.identifier == nil {
		g.identifier = store.DefaultGraph
	}
	if _, ok := g.store.(*store.MemoryStore); ok {
		// A MemoryStore fails only on a done context, and this one never is.
		_ = g.BindDefaults(context.Background())
	}
	return g
}

// BindDefaults binds the rdf, rdfs, xsd and owl prefixes, which NewGraph
// binds by itself only for a MemoryStore.
func (g *Graph) BindDefaults(ctx context.Context) error {
	for _, b := range defaultBindings {
		if err := g.store.Bind(ctx, b.Prefix, b.Namespace); err != nil {
			return err
		}
	}
	return nil
}

// NewGraphFromStore creates a Graph with an explicit store and identifier,
// without the default namespace bindings. Used internally by ConjunctiveGraph/Dataset.
func NewGraphFromStore(s store.Store, id term.Term) *Graph {
	return &Graph{store: s, identifier: id}
}

// Store returns the underlying store.
func (g *Graph) Store() store.Store { return g.store }

// Identifier returns the graph identifier: DefaultGraph for an unnamed graph.
func (g *Graph) Identifier() term.Term { return g.identifier }

// Add adds a triple to the graph.
// Ported from: rdflib.graph.Graph.add
func (g *Graph) Add(ctx context.Context, s term.Subject, p term.URIRef, o term.Term) error {
	return g.store.Add(ctx, term.Triple{Subject: s, Predicate: p, Object: o}, g.identifier)
}

// Remove removes triples matching the pattern (nil = wildcard).
// Ported from: rdflib.graph.Graph.remove
func (g *Graph) Remove(ctx context.Context, s term.Subject, p *term.URIRef, o term.Term) error {
	return g.store.Remove(ctx, term.TriplePattern{Subject: s, Predicate: p, Object: o}, g.identifier)
}

// Set atomically removes all triples with the same subject and predicate, then
// adds the new triple. The remove and add are performed under a single store
// lock so concurrent callers never observe an intermediate state.
func (g *Graph) Set(ctx context.Context, s term.Subject, p term.URIRef, o term.Term) error {
	return g.store.Set(ctx, term.Triple{Subject: s, Predicate: p, Object: o}, g.identifier)
}

// Len returns the number of triples in the graph.
// Ported from: rdflib.graph.Graph.__len__
func (g *Graph) Len(ctx context.Context) (int, error) {
	return g.store.Len(ctx, g.identifier)
}

// Contains checks if a triple pattern exists in the graph.
// Ported from: rdflib.graph.Graph.__contains__
func (g *Graph) Contains(ctx context.Context, s term.Subject, p term.URIRef, o term.Term) (bool, error) {
	for _, err := range g.store.Triples(ctx, term.TriplePattern{Subject: s, Predicate: &p, Object: o}, g.identifier) {
		return err == nil, err
	}
	return false, nil
}

// Triples iterates over triples matching the pattern (nil = wildcard).
// Ported from: rdflib.graph.Graph.triples
func (g *Graph) Triples(ctx context.Context, s term.Subject, p *term.URIRef, o term.Term) store.TripleIterator {
	return g.store.Triples(ctx, term.TriplePattern{Subject: s, Predicate: p, Object: o}, g.identifier)
}

// distinct yields pick(t) for each triple of seq, once per key(t), and passes
// an error through.
func distinct[T any](seq store.TripleIterator, key func(term.Triple) string, pick func(term.Triple) T) func(yield func(T, error) bool) {
	return func(yield func(T, error) bool) {
		seen := make(map[string]struct{})
		for t, err := range seq {
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			k := key(t)
			if _, dup := seen[k]; dup {
				continue
			}
			seen[k] = struct{}{}
			if !yield(pick(t), nil) {
				return
			}
		}
	}
}

func subjectOf(t term.Triple) term.Term   { return t.Subject }
func predicateOf(t term.Triple) term.Term { return t.Predicate }
func objectOf(t term.Triple) term.Term    { return t.Object }

// Subjects returns unique subjects matching the pattern.
// Ported from: rdflib.graph.Graph.subjects
func (g *Graph) Subjects(ctx context.Context, p *term.URIRef, o term.Term) store.TermIterator {
	return distinct(g.Triples(ctx, nil, p, o), func(t term.Triple) string { return term.TermKey(t.Subject) }, subjectOf)
}

// Predicates returns unique predicates matching the pattern.
// Ported from: rdflib.graph.Graph.predicates
func (g *Graph) Predicates(ctx context.Context, s term.Subject, o term.Term) store.TermIterator {
	return distinct(g.Triples(ctx, s, nil, o), func(t term.Triple) string { return term.TermKey(t.Predicate) }, predicateOf)
}

// Objects returns unique objects matching the pattern.
// Ported from: rdflib.graph.Graph.objects
func (g *Graph) Objects(ctx context.Context, s term.Subject, p *term.URIRef) store.TermIterator {
	return distinct(g.Triples(ctx, s, p, nil), func(t term.Triple) string { return term.TermKey(t.Object) }, objectOf)
}

// SubjectPredicates yields unique (subject, predicate) pairs for a given object.
// Ported from: rdflib.graph.Graph.subject_predicates
func (g *Graph) SubjectPredicates(ctx context.Context, o term.Term) store.TermPairIterator {
	return distinct(g.Triples(ctx, nil, nil, o),
		func(t term.Triple) string { return term.TermKey(t.Subject) + "|" + term.TermKey(t.Predicate) },
		func(t term.Triple) store.TermPair { return store.TermPair{First: t.Subject, Second: t.Predicate} })
}

// SubjectObjects yields unique (subject, object) pairs for a given predicate.
// Ported from: rdflib.graph.Graph.subject_objects
func (g *Graph) SubjectObjects(ctx context.Context, p *term.URIRef) store.TermPairIterator {
	return distinct(g.Triples(ctx, nil, p, nil),
		func(t term.Triple) string { return term.TermKey(t.Subject) + "|" + term.TermKey(t.Object) },
		func(t term.Triple) store.TermPair { return store.TermPair{First: t.Subject, Second: t.Object} })
}

// PredicateObjects yields unique (predicate, object) pairs for a given subject.
// Ported from: rdflib.graph.Graph.predicate_objects
func (g *Graph) PredicateObjects(ctx context.Context, s term.Subject) store.TermPairIterator {
	return distinct(g.Triples(ctx, s, nil, nil),
		func(t term.Triple) string { return term.TermKey(t.Predicate) + "|" + term.TermKey(t.Object) },
		func(t term.Triple) store.TermPair { return store.TermPair{First: t.Predicate, Second: t.Object} })
}

// Value returns a single term matching the pattern.
// Exactly two of s, p, o must be provided (non-nil); the third is the wildcard.
// If the contract is violated (all nil, or all non-nil), Value returns (nil, false, nil).
func (g *Graph) Value(ctx context.Context, s term.Subject, p *term.URIRef, o term.Term) (term.Term, bool, error) {
	// Count non-nil arguments to validate the contract.
	nNonNil := 0
	if s != nil {
		nNonNil++
	}
	if p != nil {
		nNonNil++
	}
	if o != nil {
		nNonNil++
	}
	if nNonNil != 2 {
		return nil, false, nil
	}
	for t, err := range g.Triples(ctx, s, p, o) {
		if err != nil {
			return nil, false, err
		}
		switch {
		case s == nil:
			return t.Subject, true, nil
		case p == nil:
			return t.Predicate, true, nil
		default:
			return t.Object, true, nil
		}
	}
	return nil, false, nil
}

// --- Namespace Operations ---

// Bind associates a prefix with a namespace.
// Ported from: rdflib.graph.Graph.bind
func (g *Graph) Bind(ctx context.Context, prefix string, ns term.URIRef) error {
	return g.store.Bind(ctx, prefix, ns)
}

// Namespaces returns an iterator over all namespace bindings.
// Ported from: rdflib.graph.Graph.namespaces
func (g *Graph) Namespaces(ctx context.Context) store.NamespaceIterator {
	return g.store.Namespaces(ctx)
}

// QName returns a prefixed name for the URI, or the original URI if no prefix matches.
// Ported from: rdflib.graph.Graph.qname
func (g *Graph) QName(ctx context.Context, uri string) (string, error) {
	// Try to find longest matching namespace
	bestPrefix := ""
	bestNS := ""
	for b, err := range g.store.Namespaces(ctx) {
		if err != nil {
			return "", err
		}
		nsStr := b.Namespace.Value()
		if len(nsStr) > len(bestNS) && len(uri) > len(nsStr) && uri[:len(nsStr)] == nsStr {
			bestPrefix = b.Prefix
			bestNS = nsStr
		}
	}
	if bestNS != "" {
		return bestPrefix + ":" + uri[len(bestNS):], nil
	}
	return uri, nil
}

// --- Set Operations ---

// addAll adds the triples of src to dst for which keep returns true.
func addAll(ctx context.Context, dst, src *Graph, keep func(term.Triple) (bool, error)) error {
	for t, err := range src.Triples(ctx, nil, nil, nil) {
		if err != nil {
			return err
		}
		if keep != nil {
			ok, err := keep(t)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
		}
		if err := dst.Add(ctx, t.Subject, t.Predicate, t.Object); err != nil {
			return err
		}
	}
	return nil
}

// Union returns a new in-memory graph containing triples from both graphs.
// Ported from: rdflib.graph.Graph.__iadd__
func (g *Graph) Union(ctx context.Context, other *Graph) (*Graph, error) {
	result := NewGraph()
	if err := addAll(ctx, result, g, nil); err != nil {
		return nil, err
	}
	if err := addAll(ctx, result, other, nil); err != nil {
		return nil, err
	}
	return result, nil
}

// Intersection returns a new in-memory graph with triples in both graphs.
// Ported from: rdflib.graph.Graph.__mul__
func (g *Graph) Intersection(ctx context.Context, other *Graph) (*Graph, error) {
	result := NewGraph()
	err := addAll(ctx, result, g, func(t term.Triple) (bool, error) {
		return other.Contains(ctx, t.Subject, t.Predicate, t.Object)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Difference returns a new in-memory graph with triples in g but not in other.
// Ported from: rdflib.graph.Graph.__isub__
func (g *Graph) Difference(ctx context.Context, other *Graph) (*Graph, error) {
	result := NewGraph()
	err := addAll(ctx, result, g, func(t term.Triple) (bool, error) {
		in, err := other.Contains(ctx, t.Subject, t.Predicate, t.Object)
		return !in, err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// AllNodes returns all unique subjects and objects in the graph.
// Ported from: rdflib.graph.Graph.all_nodes
func (g *Graph) AllNodes(ctx context.Context) ([]term.Term, error) {
	seen := make(map[string]term.Term)
	for t, err := range g.Triples(ctx, nil, nil, nil) {
		if err != nil {
			return nil, err
		}
		seen[term.TermKey(t.Subject)] = t.Subject
		seen[term.TermKey(t.Object)] = t.Object
	}
	nodes := make([]term.Term, 0, len(seen))
	for _, v := range seen {
		nodes = append(nodes, v)
	}
	return nodes, nil
}

// Connected returns true if the graph is connected (all nodes reachable from any starting node).
// Ported from: rdflib.graph.Graph.connected
func (g *Graph) Connected(ctx context.Context) (bool, error) {
	allNodes, err := g.AllNodes(ctx)
	if err != nil {
		return false, err
	}
	if len(allNodes) == 0 {
		return true, nil
	}

	visited := make(map[string]bool)
	var stack []term.Term
	stack = append(stack, allNodes[0])
	visited[term.TermKey(allNodes[0])] = true
	visit := func(seq store.TermIterator) error {
		for t, err := range seq {
			if err != nil {
				return err
			}
			k := term.TermKey(t)
			if !visited[k] {
				visited[k] = true
				stack = append(stack, t)
			}
		}
		return nil
	}

	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// Follow outgoing edges (node as subject)
		if subj, ok := node.(term.Subject); ok {
			if err := visit(g.Objects(ctx, subj, nil)); err != nil {
				return false, err
			}
		}
		// Follow incoming edges (node as object)
		if err := visit(g.Subjects(ctx, nil, node)); err != nil {
			return false, err
		}
	}

	return len(visited) == len(allNodes), nil
}

// Base returns the base URI set on this graph.
func (g *Graph) Base() string { return g.base }
