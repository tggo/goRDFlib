package graph

import (
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// Graph is an RDF graph backed by a Store.
// Safe for concurrent reads when backed by a thread-safe Store (MemoryStore,
// BadgerStore, SQLiteStore). Write operations require external synchronization.
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

// WithIdentifier sets the graph identifier.
func WithIdentifier(id term.Term) GraphOption {
	return func(g *Graph) { g.identifier = id }
}

// WithBase sets the base URI for relative resolution.
func WithBase(base string) GraphOption {
	return func(g *Graph) { g.base = base }
}

// NewGraph creates a new Graph.
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
		g.identifier = term.NewBNode()
	}
	// Default namespace bindings
	g.Bind("rdf", term.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#"))
	g.Bind("rdfs", term.NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#"))
	g.Bind("xsd", term.NewURIRefUnsafe(term.XSDNamespace))
	g.Bind("owl", term.NewURIRefUnsafe("http://www.w3.org/2002/07/owl#"))
	return g
}

// NewGraphFromStore creates a Graph with an explicit store and identifier,
// without the default namespace bindings. Used internally by ConjunctiveGraph/Dataset.
func NewGraphFromStore(s store.Store, id term.Term) *Graph {
	return &Graph{store: s, identifier: id}
}

// Store returns the underlying store.
func (g *Graph) Store() store.Store { return g.store }

// Identifier returns the graph identifier.
func (g *Graph) Identifier() term.Term { return g.identifier }

// Add adds a triple to the graph. Returns the graph for chaining.
// Ported from: rdflib.graph.Graph.add
func (g *Graph) Add(s term.Subject, p term.URIRef, o term.Term) *Graph {
	g.store.Add(term.Triple{Subject: s, Predicate: p, Object: o}, g.identifier)
	return g
}

// Remove removes triples matching the pattern (nil = wildcard).
// Ported from: rdflib.graph.Graph.remove
func (g *Graph) Remove(s term.Subject, p *term.URIRef, o term.Term) *Graph {
	g.store.Remove(term.TriplePattern{Subject: s, Predicate: p, Object: o}, g.identifier)
	return g
}

// Set atomically removes all triples with the same subject and predicate, then
// adds the new triple. The remove and add are performed under a single store
// lock so concurrent callers never observe an intermediate state.
func (g *Graph) Set(s term.Subject, p term.URIRef, o term.Term) *Graph {
	g.store.Set(term.Triple{Subject: s, Predicate: p, Object: o}, g.identifier)
	return g
}

// Len returns the number of triples in the graph.
// Ported from: rdflib.graph.Graph.__len__
func (g *Graph) Len() int {
	return g.store.Len(g.identifier)
}

// Contains checks if a triple pattern exists in the graph.
// Ported from: rdflib.graph.Graph.__contains__
func (g *Graph) Contains(s term.Subject, p term.URIRef, o term.Term) bool {
	found := false
	g.store.Triples(term.TriplePattern{Subject: s, Predicate: &p, Object: o}, g.identifier)(func(term.Triple) bool {
		found = true
		return false // stop after first
	})
	return found
}

// Triples iterates over triples matching the pattern (nil = wildcard).
// Ported from: rdflib.graph.Graph.triples
func (g *Graph) Triples(s term.Subject, p *term.URIRef, o term.Term) store.TripleIterator {
	return g.store.Triples(term.TriplePattern{Subject: s, Predicate: p, Object: o}, g.identifier)
}

// Subjects returns unique subjects matching the pattern.
// Ported from: rdflib.graph.Graph.subjects
func (g *Graph) Subjects(p *term.URIRef, o term.Term) store.TermIterator {
	return func(yield func(term.Term) bool) {
		seen := make(map[string]struct{})
		g.Triples(nil, p, o)(func(t term.Triple) bool {
			k := term.TermKey(t.Subject)
			if _, dup := seen[k]; !dup {
				seen[k] = struct{}{}
				return yield(t.Subject)
			}
			return true
		})
	}
}

// Predicates returns unique predicates matching the pattern.
// Ported from: rdflib.graph.Graph.predicates
func (g *Graph) Predicates(s term.Subject, o term.Term) store.TermIterator {
	return func(yield func(term.Term) bool) {
		seen := make(map[string]struct{})
		g.Triples(s, nil, o)(func(t term.Triple) bool {
			k := term.TermKey(t.Predicate)
			if _, dup := seen[k]; !dup {
				seen[k] = struct{}{}
				return yield(t.Predicate)
			}
			return true
		})
	}
}

// Objects returns unique objects matching the pattern.
// Ported from: rdflib.graph.Graph.objects
func (g *Graph) Objects(s term.Subject, p *term.URIRef) store.TermIterator {
	return func(yield func(term.Term) bool) {
		seen := make(map[string]struct{})
		g.Triples(s, p, nil)(func(t term.Triple) bool {
			k := term.TermKey(t.Object)
			if _, dup := seen[k]; !dup {
				seen[k] = struct{}{}
				return yield(t.Object)
			}
			return true
		})
	}
}

// SubjectPredicates yields unique (subject, predicate) pairs for a given object.
// Ported from: rdflib.graph.Graph.subject_predicates
func (g *Graph) SubjectPredicates(o term.Term) store.TermPairIterator {
	return func(yield func(term.Term, term.Term) bool) {
		seen := make(map[string]struct{})
		g.Triples(nil, nil, o)(func(t term.Triple) bool {
			k := term.TermKey(t.Subject) + "|" + term.TermKey(t.Predicate)
			if _, dup := seen[k]; !dup {
				seen[k] = struct{}{}
				return yield(t.Subject, t.Predicate)
			}
			return true
		})
	}
}

// SubjectObjects yields unique (subject, object) pairs for a given predicate.
// Ported from: rdflib.graph.Graph.subject_objects
func (g *Graph) SubjectObjects(p *term.URIRef) store.TermPairIterator {
	return func(yield func(term.Term, term.Term) bool) {
		seen := make(map[string]struct{})
		g.Triples(nil, p, nil)(func(t term.Triple) bool {
			k := term.TermKey(t.Subject) + "|" + term.TermKey(t.Object)
			if _, dup := seen[k]; !dup {
				seen[k] = struct{}{}
				return yield(t.Subject, t.Object)
			}
			return true
		})
	}
}

// PredicateObjects yields unique (predicate, object) pairs for a given subject.
// Ported from: rdflib.graph.Graph.predicate_objects
func (g *Graph) PredicateObjects(s term.Subject) store.TermPairIterator {
	return func(yield func(term.Term, term.Term) bool) {
		seen := make(map[string]struct{})
		g.Triples(s, nil, nil)(func(t term.Triple) bool {
			k := term.TermKey(t.Predicate) + "|" + term.TermKey(t.Object)
			if _, dup := seen[k]; !dup {
				seen[k] = struct{}{}
				return yield(t.Predicate, t.Object)
			}
			return true
		})
	}
}

// Value returns a single term matching the pattern.
// Exactly two of s, p, o must be provided (non-nil); the third is the wildcard.
// If the contract is violated (all nil, or all non-nil), Value returns (nil, false).
func (g *Graph) Value(s term.Subject, p *term.URIRef, o term.Term) (term.Term, bool) {
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
		return nil, false
	}

	var result term.Term
	found := false

	if s == nil {
		// Looking for subject
		g.Subjects(p, o)(func(t term.Term) bool {
			result = t
			found = true
			return false
		})
	} else if p == nil {
		// Looking for predicate
		g.Predicates(s, o)(func(t term.Term) bool {
			result = t
			found = true
			return false
		})
	} else {
		// Looking for object
		g.Objects(s, p)(func(t term.Term) bool {
			result = t
			found = true
			return false
		})
	}
	return result, found
}

// --- Namespace Operations ---

// Bind associates a prefix with a namespace.
// Ported from: rdflib.graph.Graph.bind
func (g *Graph) Bind(prefix string, ns term.URIRef) {
	g.store.Bind(prefix, ns)
}

// Namespaces returns an iterator over all namespace bindings.
// Ported from: rdflib.graph.Graph.namespaces
func (g *Graph) Namespaces() store.NamespaceIterator {
	return g.store.Namespaces()
}

// QName returns a prefixed name for the URI, or the original URI if no prefix matches.
// Ported from: rdflib.graph.Graph.qname
func (g *Graph) QName(uri string) string {
	// Try to find longest matching namespace
	bestPrefix := ""
	bestNS := ""
	g.store.Namespaces()(func(prefix string, ns term.URIRef) bool {
		nsStr := ns.Value()
		if len(nsStr) > len(bestNS) && len(uri) > len(nsStr) && uri[:len(nsStr)] == nsStr {
			bestPrefix = prefix
			bestNS = nsStr
		}
		return true
	})
	if bestNS != "" {
		return bestPrefix + ":" + uri[len(bestNS):]
	}
	return uri
}

// --- Set Operations ---

// Union returns a new graph containing triples from both graphs.
// Ported from: rdflib.graph.Graph.__iadd__
func (g *Graph) Union(other *Graph) *Graph {
	result := NewGraph()
	g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		result.Add(t.Subject, t.Predicate, t.Object)
		return true
	})
	other.Triples(nil, nil, nil)(func(t term.Triple) bool {
		result.Add(t.Subject, t.Predicate, t.Object)
		return true
	})
	return result
}

// Intersection returns a new graph with triples in both graphs.
// Ported from: rdflib.graph.Graph.__mul__
func (g *Graph) Intersection(other *Graph) *Graph {
	result := NewGraph()
	g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		if other.Contains(t.Subject, t.Predicate, t.Object) {
			result.Add(t.Subject, t.Predicate, t.Object)
		}
		return true
	})
	return result
}

// Difference returns a new graph with triples in g but not in other.
// Ported from: rdflib.graph.Graph.__isub__
func (g *Graph) Difference(other *Graph) *Graph {
	result := NewGraph()
	g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		if !other.Contains(t.Subject, t.Predicate, t.Object) {
			result.Add(t.Subject, t.Predicate, t.Object)
		}
		return true
	})
	return result
}

// AllNodes returns all unique subjects and objects in the graph.
// Ported from: rdflib.graph.Graph.all_nodes
func (g *Graph) AllNodes() []term.Term {
	seen := make(map[string]term.Term)
	g.Triples(nil, nil, nil)(func(t term.Triple) bool {
		seen[term.TermKey(t.Subject)] = t.Subject
		seen[term.TermKey(t.Object)] = t.Object
		return true
	})
	nodes := make([]term.Term, 0, len(seen))
	for _, v := range seen {
		nodes = append(nodes, v)
	}
	return nodes
}

// Connected returns true if the graph is connected (all nodes reachable from any starting node).
// Ported from: rdflib.graph.Graph.connected
func (g *Graph) Connected() bool {
	allNodes := g.AllNodes()
	if len(allNodes) == 0 {
		return true
	}

	visited := make(map[string]bool)
	var stack []term.Term
	stack = append(stack, allNodes[0])
	visited[term.TermKey(allNodes[0])] = true

	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// Follow outgoing edges (node as subject)
		if subj, ok := node.(term.Subject); ok {
			g.Objects(subj, nil)(func(t term.Term) bool {
				k := term.TermKey(t)
				if !visited[k] {
					visited[k] = true
					stack = append(stack, t)
				}
				return true
			})
		}

		// Follow incoming edges (node as object)
		g.Subjects(nil, node)(func(t term.Term) bool {
			k := term.TermKey(t)
			if !visited[k] {
				visited[k] = true
				stack = append(stack, t)
			}
			return true
		})
	}

	return len(visited) == len(allNodes)
}

// Base returns the base URI set on this graph.
func (g *Graph) Base() string { return g.base }
