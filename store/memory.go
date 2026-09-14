package store

import (
	"sync"

	"github.com/tggo/goRDFlib/term"
)

// MemoryStore is a thread-safe, context-aware in-memory triple store. Each
// graph — the default graph and every named graph — has its own three indices
// (SPO, POS, OSP), so reads, counts and removals never cross graph boundaries.
// It follows the context conventions documented on Store.
//
// All methods are safe for concurrent use.
// Ported from: rdflib.plugins.stores.memory.Memory
type MemoryStore struct {
	mu sync.RWMutex

	// def is the default graph. It always exists, so the common case of a
	// store used as a single graph pays for one pointer and no map lookup.
	def *tripleIndex
	// named maps the TermKey of a graph name to its graph. A named graph is
	// created on first write and dropped when its last triple is removed, so
	// Contexts reports exactly the graphs that hold data.
	named map[string]*namedGraph

	// Namespace bindings
	nsPrefix map[string]term.URIRef // prefix → namespace
	nsURI    map[string]string      // namespace → prefix
}

// namedGraph is a named graph's name together with its triples.
type namedGraph struct {
	name term.Term
	idx  *tripleIndex
}

// NewMemoryStore creates a new empty in-memory store.
// Ported from: rdflib.plugins.stores.memory.Memory.__init__
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		def:      newTripleIndex(),
		named:    make(map[string]*namedGraph),
		nsPrefix: make(map[string]term.URIRef),
		nsURI:    make(map[string]string),
	}
}

// ContextAware reports whether this store supports named graphs.
func (m *MemoryStore) ContextAware() bool { return true }

// TransactionAware reports whether this store supports transactions.
func (m *MemoryStore) TransactionAware() bool { return false }

// isDefaultContext reports whether ctx addresses the default graph under the
// Store context conventions.
func isDefaultContext(ctx term.Term) bool {
	if ctx == nil {
		return true
	}
	_, isBNode := ctx.(term.BNode)
	return isBNode
}

// index returns the graph addressed by ctx for reading, or nil if a named
// graph does not exist. Caller must hold at least m.mu.RLock().
func (m *MemoryStore) index(ctx term.Term) *tripleIndex {
	if isDefaultContext(ctx) {
		return m.def
	}
	if g := m.named[term.TermKey(ctx)]; g != nil {
		return g.idx
	}
	return nil
}

// writeIndex returns the graph addressed by ctx, creating a named graph if
// needed. Caller must hold m.mu.Lock().
func (m *MemoryStore) writeIndex(ctx term.Term) *tripleIndex {
	if isDefaultContext(ctx) {
		return m.def
	}
	k := term.TermKey(ctx)
	g := m.named[k]
	if g == nil {
		g = &namedGraph{name: ctx, idx: newTripleIndex()}
		m.named[k] = g
	}
	return g.idx
}

// dropIfEmpty forgets a named graph that no longer holds triples. Caller must
// hold m.mu.Lock().
func (m *MemoryStore) dropIfEmpty(ctx term.Term) {
	if isDefaultContext(ctx) {
		return
	}
	k := term.TermKey(ctx)
	if g := m.named[k]; g != nil && g.idx.count == 0 {
		delete(m.named, k)
	}
}

// Add inserts a triple into the graph addressed by context.
// Ported from: rdflib.plugins.stores.memory.Memory.add
func (m *MemoryStore) Add(t term.Triple, context term.Term) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeIndex(context).add(t)
}

// Set atomically removes all triples matching (s, p, *) from the graph
// addressed by context and adds the new triple, under a single write lock.
func (m *MemoryStore) Set(t term.Triple, context term.Term) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.writeIndex(context)
	idx.removeMatching(term.TriplePattern{Subject: t.Subject, Predicate: &t.Predicate})
	idx.add(t)
}

// AddN atomically batch-adds quads, each to the graph named by its Graph field.
// Ported from: rdflib.store.Store.addN
func (m *MemoryStore) AddN(quads []term.Quad) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, q := range quads {
		m.writeIndex(q.Graph).add(q.Triple)
	}
}

// Remove deletes triples matching the pattern from the graph addressed by
// context. The match and delete happen under a single write lock.
// Ported from: rdflib.plugins.stores.memory.Memory.remove
func (m *MemoryStore) Remove(pattern term.TriplePattern, context term.Term) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.index(context)
	if idx == nil {
		return
	}
	idx.removeMatching(pattern)
	m.dropIfEmpty(context)
}

// Triples returns the triples matching the pattern in the graph addressed by
// context.
// Ported from: rdflib.plugins.stores.memory.Memory.triples
func (m *MemoryStore) Triples(pattern term.TriplePattern, context term.Term) TripleIterator {
	if isDefaultContext(context) {
		// Not capturing the context keeps the closure in the same allocation
		// size class as before named graphs existed: this is the hot path.
		return func(yield func(term.Triple) bool) {
			m.mu.RLock()
			defer m.mu.RUnlock()
			m.def.each(pattern, yield)
		}
	}
	return func(yield func(term.Triple) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		if idx := m.index(context); idx != nil {
			idx.each(pattern, yield)
		}
	}
}

// Len returns the number of triples in the graph addressed by context.
// Ported from: rdflib.plugins.stores.memory.Memory.__len__
func (m *MemoryStore) Len(context term.Term) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if idx := m.index(context); idx != nil {
		return idx.count
	}
	return 0
}

// Contexts returns the named graphs, or with a non-nil triple the named graphs
// that contain it. The default graph is never reported.
func (m *MemoryStore) Contexts(triple *term.Triple) TermIterator {
	return func(yield func(term.Term) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		for _, g := range m.named {
			if triple != nil && !g.idx.has(*triple) {
				continue
			}
			if !yield(g.name) {
				return
			}
		}
	}
}

// Bind associates a prefix with a namespace.
// Ported from: rdflib.plugins.stores.memory.Memory.bind
func (m *MemoryStore) Bind(prefix string, namespace term.URIRef) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nsPrefix[prefix] = namespace
	m.nsURI[namespace.Value()] = prefix
}

// Namespace returns the namespace URI for a prefix.
// Ported from: rdflib.plugins.stores.memory.Memory.namespace
func (m *MemoryStore) Namespace(prefix string) (term.URIRef, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ns, ok := m.nsPrefix[prefix]
	return ns, ok
}

// Prefix returns the prefix for a namespace URI.
// Ported from: rdflib.plugins.stores.memory.Memory.prefix
func (m *MemoryStore) Prefix(namespace term.URIRef) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.nsURI[namespace.Value()]
	return p, ok
}

// Namespaces returns an iterator over all namespace bindings.
// Ported from: rdflib.plugins.stores.memory.Memory.namespaces
func (m *MemoryStore) Namespaces() NamespaceIterator {
	return func(yield func(string, term.URIRef) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		for prefix, ns := range m.nsPrefix {
			if !yield(prefix, ns) {
				return
			}
		}
	}
}

// TriplesWithLimit returns matching triples in the graph addressed by ctx,
// skipping the first offset items then yielding up to limit items. If
// limit <= 0, all remaining items after offset are yielded.
// Safe for concurrent use.
func (m *MemoryStore) TriplesWithLimit(pattern term.TriplePattern, ctx term.Term, limit, offset int) TripleIterator {
	return func(yield func(term.Triple) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		idx := m.index(ctx)
		if idx == nil {
			return
		}
		skipped := 0
		yielded := 0
		idx.each(pattern, func(t term.Triple) bool {
			if skipped < offset {
				skipped++
				return true
			}
			if limit > 0 && yielded >= limit {
				return false
			}
			yielded++
			return yield(t)
		})
	}
}

// Count returns the number of triples matching the pattern in the graph
// addressed by ctx, from index sizes. Safe for concurrent use.
func (m *MemoryStore) Count(pattern term.TriplePattern, ctx term.Term) int {
	return m.Cardinality(pattern, ctx)
}

// Cardinality returns the number of triples matching the pattern in the graph
// addressed by ctx without visiting them. A pattern bound in two or three
// positions, only in the predicate, or not at all is answered in constant time;
// subject-only walks the subject's predicates and object-only the object's
// subjects, never the triples. Safe for concurrent use.
func (m *MemoryStore) Cardinality(pattern term.TriplePattern, ctx term.Term) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if idx := m.index(ctx); idx != nil {
		return idx.cardinality(pattern)
	}
	return 0
}

// Exists reports whether at least one triple matching the pattern exists in
// the graph addressed by ctx. Safe for concurrent use.
func (m *MemoryStore) Exists(pattern term.TriplePattern, ctx term.Term) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	idx := m.index(ctx)
	if idx == nil {
		return false
	}
	found := false
	idx.each(pattern, func(term.Triple) bool {
		found = true
		return false
	})
	return found
}

// Compile-time checks: MemoryStore implements the optional interfaces.
var (
	_ QueryableStore   = (*MemoryStore)(nil)
	_ CardinalityStore = (*MemoryStore)(nil)
)
