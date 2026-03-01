package rdflibgo


// Graph is an RDF graph backed by a Store.
// Ported from: rdflib.graph.Graph
type Graph struct {
	store      Store
	identifier Term
	base       string
}

// GraphOption configures Graph construction.
type GraphOption func(*Graph)

// WithStore sets the backing store.
func WithStore(s Store) GraphOption {
	return func(g *Graph) { g.store = s }
}

// WithIdentifier sets the graph identifier.
func WithIdentifier(id Term) GraphOption {
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
		g.store = NewMemoryStore()
	}
	if g.identifier == nil {
		g.identifier = NewBNode()
	}
	// Default namespace bindings
	g.Bind("rdf", NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#"))
	g.Bind("rdfs", NewURIRefUnsafe("http://www.w3.org/2000/01/rdf-schema#"))
	g.Bind("xsd", NewURIRefUnsafe(XSDNamespace))
	g.Bind("owl", NewURIRefUnsafe("http://www.w3.org/2002/07/owl#"))
	return g
}

// Store returns the underlying store.
func (g *Graph) Store() Store { return g.store }

// Identifier returns the graph identifier.
func (g *Graph) Identifier() Term { return g.identifier }

// Add adds a triple to the graph. Returns the graph for chaining.
// Ported from: rdflib.graph.Graph.add
func (g *Graph) Add(s Subject, p URIRef, o Term) *Graph {
	g.store.Add(Triple{Subject: s, Predicate: p, Object: o}, g.identifier)
	return g
}

// Remove removes triples matching the pattern (nil = wildcard).
// Ported from: rdflib.graph.Graph.remove
func (g *Graph) Remove(s Subject, p *URIRef, o Term) *Graph {
	g.store.Remove(TriplePattern{Subject: s, Predicate: p, Object: o}, g.identifier)
	return g
}

// Set removes all triples with the same subject and predicate, then adds the new triple.
// Ported from: rdflib.graph.Graph.set
func (g *Graph) Set(s Subject, p URIRef, o Term) *Graph {
	g.Remove(s, &p, nil)
	g.Add(s, p, o)
	return g
}

// Len returns the number of triples in the graph.
// Ported from: rdflib.graph.Graph.__len__
func (g *Graph) Len() int {
	return g.store.Len(g.identifier)
}

// Contains checks if a triple pattern exists in the graph.
// Ported from: rdflib.graph.Graph.__contains__
func (g *Graph) Contains(s Subject, p URIRef, o Term) bool {
	found := false
	g.store.Triples(TriplePattern{Subject: s, Predicate: &p, Object: o}, g.identifier)(func(Triple) bool {
		found = true
		return false // stop after first
	})
	return found
}

// Triples iterates over triples matching the pattern (nil = wildcard).
// Ported from: rdflib.graph.Graph.triples
func (g *Graph) Triples(s Subject, p *URIRef, o Term) TripleIterator {
	return g.store.Triples(TriplePattern{Subject: s, Predicate: p, Object: o}, g.identifier)
}

// Subjects returns unique subjects matching the pattern.
// Ported from: rdflib.graph.Graph.subjects
func (g *Graph) Subjects(p *URIRef, o Term) TermIterator {
	return func(yield func(Term) bool) {
		seen := make(map[string]struct{})
		g.Triples(nil, p, o)(func(t Triple) bool {
			k := termKey(t.Subject)
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
func (g *Graph) Predicates(s Subject, o Term) TermIterator {
	return func(yield func(Term) bool) {
		seen := make(map[string]struct{})
		g.Triples(s, nil, o)(func(t Triple) bool {
			k := termKey(t.Predicate)
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
func (g *Graph) Objects(s Subject, p *URIRef) TermIterator {
	return func(yield func(Term) bool) {
		seen := make(map[string]struct{})
		g.Triples(s, p, nil)(func(t Triple) bool {
			k := termKey(t.Object)
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
func (g *Graph) SubjectPredicates(o Term) func(yield func(Term, Term) bool) {
	return func(yield func(Term, Term) bool) {
		seen := make(map[string]struct{})
		g.Triples(nil, nil, o)(func(t Triple) bool {
			k := termKey(t.Subject) + "|" + termKey(t.Predicate)
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
func (g *Graph) SubjectObjects(p *URIRef) func(yield func(Term, Term) bool) {
	return func(yield func(Term, Term) bool) {
		seen := make(map[string]struct{})
		g.Triples(nil, p, nil)(func(t Triple) bool {
			k := termKey(t.Subject) + "|" + termKey(t.Object)
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
func (g *Graph) PredicateObjects(s Subject) func(yield func(Term, Term) bool) {
	return func(yield func(Term, Term) bool) {
		seen := make(map[string]struct{})
		g.Triples(s, nil, nil)(func(t Triple) bool {
			k := termKey(t.Predicate) + "|" + termKey(t.Object)
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
// Ported from: rdflib.graph.Graph.value
func (g *Graph) Value(s Subject, p *URIRef, o Term) (Term, bool) {
	var result Term
	found := false

	if s == nil {
		// Looking for subject
		g.Subjects(p, o)(func(t Term) bool {
			result = t
			found = true
			return false
		})
	} else if p == nil {
		// Looking for predicate
		g.Predicates(s, o)(func(t Term) bool {
			result = t
			found = true
			return false
		})
	} else {
		// Looking for object
		g.Objects(s, p)(func(t Term) bool {
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
func (g *Graph) Bind(prefix string, namespace URIRef) {
	g.store.Bind(prefix, namespace)
}

// Namespaces returns an iterator over all namespace bindings.
// Ported from: rdflib.graph.Graph.namespaces
func (g *Graph) Namespaces() NamespaceIterator {
	return g.store.Namespaces()
}

// QName returns a prefixed name for the URI, or the original URI if no prefix matches.
// Ported from: rdflib.graph.Graph.qname
func (g *Graph) QName(uri string) string {
	// Try to find longest matching namespace
	bestPrefix := ""
	bestNS := ""
	g.store.Namespaces()(func(prefix string, ns URIRef) bool {
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
	g.Triples(nil, nil, nil)(func(t Triple) bool {
		result.Add(t.Subject, t.Predicate, t.Object)
		return true
	})
	other.Triples(nil, nil, nil)(func(t Triple) bool {
		result.Add(t.Subject, t.Predicate, t.Object)
		return true
	})
	return result
}

// Intersection returns a new graph with triples in both graphs.
// Ported from: rdflib.graph.Graph.__mul__
func (g *Graph) Intersection(other *Graph) *Graph {
	result := NewGraph()
	g.Triples(nil, nil, nil)(func(t Triple) bool {
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
	g.Triples(nil, nil, nil)(func(t Triple) bool {
		if !other.Contains(t.Subject, t.Predicate, t.Object) {
			result.Add(t.Subject, t.Predicate, t.Object)
		}
		return true
	})
	return result
}

// AllNodes returns all unique subjects and objects in the graph.
// Ported from: rdflib.graph.Graph.all_nodes
func (g *Graph) AllNodes() []Term {
	seen := make(map[string]Term)
	g.Triples(nil, nil, nil)(func(t Triple) bool {
		seen[termKey(t.Subject)] = t.Subject
		seen[termKey(t.Object)] = t.Object
		return true
	})
	nodes := make([]Term, 0, len(seen))
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
	var stack []Term
	stack = append(stack, allNodes[0])
	visited[termKey(allNodes[0])] = true

	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		// Follow outgoing edges (node as subject)
		if subj, ok := node.(Subject); ok {
			g.Objects(subj, nil)(func(t Term) bool {
				k := termKey(t)
				if !visited[k] {
					visited[k] = true
					stack = append(stack, t)
				}
				return true
			})
		}

		// Follow incoming edges (node as object)
		g.Subjects(nil, node)(func(t Term) bool {
			k := termKey(t)
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
