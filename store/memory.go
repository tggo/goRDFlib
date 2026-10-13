package store

import (
	"context"
	"sync"

	"github.com/tggo/goRDFlib/term"
)

// MemoryStore is a thread-safe, context-aware in-memory triple store. Each
// graph — the default graph and every named graph — has its own three indices
// (SPO, POS, OSP), so reads, counts and removals never cross graph boundaries.
// It follows the context conventions documented on Store.
//
// All methods are safe for concurrent use. MemoryStore never fails on its
// own: the only error it returns is that of a context that is already done
// when a call starts.
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
// Store context conventions: nil or DefaultGraph. A blank node names a graph.
func isDefaultContext(ctx term.Term) bool {
	return IsDefaultGraph(ctx)
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

// Add inserts a triple into the graph addressed by graph.
// Ported from: rdflib.plugins.stores.memory.Memory.add
func (m *MemoryStore) Add(ctx context.Context, t term.Triple, graph term.Term) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeIndex(graph).add(t)
	return nil
}

// Set atomically removes all triples matching (s, p, *) from the graph
// addressed by graph and adds the new triple, under a single write lock.
func (m *MemoryStore) Set(ctx context.Context, t term.Triple, graph term.Term) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.writeIndex(graph)
	idx.removeMatching(term.TriplePattern{Subject: t.Subject, Predicate: &t.Predicate})
	idx.add(t)
	return nil
}

// AddN atomically batch-adds quads, each to the graph named by its Graph field.
// Ported from: rdflib.store.Store.addN
func (m *MemoryStore) AddN(ctx context.Context, quads []term.Quad) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, q := range quads {
		m.writeIndex(q.Graph).add(q.Triple)
	}
	return nil
}

// Remove deletes triples matching the pattern from the graph addressed by
// graph. The match and delete happen under a single write lock.
// Ported from: rdflib.plugins.stores.memory.Memory.remove
func (m *MemoryStore) Remove(ctx context.Context, pattern term.TriplePattern, graph term.Term) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := m.index(graph)
	if idx == nil {
		return nil
	}
	idx.removeMatching(pattern)
	m.dropIfEmpty(graph)
	return nil
}

// Triples returns the triples matching the pattern in the graph addressed by
// graph.
// Ported from: rdflib.plugins.stores.memory.Memory.triples
func (m *MemoryStore) Triples(ctx context.Context, pattern term.TriplePattern, graph term.Term) TripleIterator {
	if err := ctx.Err(); err != nil {
		return ErrorIterator[term.Triple](err)
	}
	if isDefaultContext(graph) {
		// Not capturing the graph keeps the closure in a smaller allocation
		// size class: this is the hot path.
		return func(yield func(term.Triple, error) bool) {
			m.mu.RLock()
			defer m.mu.RUnlock()
			m.def.each(pattern, yield)
		}
	}
	return func(yield func(term.Triple, error) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		if idx := m.index(graph); idx != nil {
			idx.each(pattern, yield)
		}
	}
}

// Len returns the number of triples in the graph addressed by graph.
// Ported from: rdflib.plugins.stores.memory.Memory.__len__
func (m *MemoryStore) Len(ctx context.Context, graph term.Term) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if idx := m.index(graph); idx != nil {
		return idx.count, nil
	}
	return 0, nil
}

// Contexts returns the named graphs, or with a non-nil triple the named graphs
// that contain it. The default graph is never reported.
func (m *MemoryStore) Contexts(ctx context.Context, triple *term.Triple) TermIterator {
	if err := ctx.Err(); err != nil {
		return ErrorIterator[term.Term](err)
	}
	return func(yield func(term.Term, error) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		for _, g := range m.named {
			if triple != nil && !g.idx.has(*triple) {
				continue
			}
			if !yield(g.name, nil) {
				return
			}
		}
	}
}

// Bind associates a prefix with a namespace.
// Ported from: rdflib.plugins.stores.memory.Memory.bind
func (m *MemoryStore) Bind(ctx context.Context, prefix string, namespace term.URIRef) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nsPrefix[prefix] = namespace
	m.nsURI[namespace.Value()] = prefix
	return nil
}

// Namespace returns the namespace URI for a prefix.
// Ported from: rdflib.plugins.stores.memory.Memory.namespace
func (m *MemoryStore) Namespace(ctx context.Context, prefix string) (term.URIRef, bool, error) {
	if err := ctx.Err(); err != nil {
		return term.URIRef{}, false, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	ns, ok := m.nsPrefix[prefix]
	return ns, ok, nil
}

// Prefix returns the prefix for a namespace URI.
// Ported from: rdflib.plugins.stores.memory.Memory.prefix
func (m *MemoryStore) Prefix(ctx context.Context, namespace term.URIRef) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.nsURI[namespace.Value()]
	return p, ok, nil
}

// Namespaces returns an iterator over all namespace bindings.
// Ported from: rdflib.plugins.stores.memory.Memory.namespaces
func (m *MemoryStore) Namespaces(ctx context.Context) NamespaceIterator {
	if err := ctx.Err(); err != nil {
		return ErrorIterator[NamespaceBinding](err)
	}
	return func(yield func(NamespaceBinding, error) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		for prefix, ns := range m.nsPrefix {
			if !yield(NamespaceBinding{Prefix: prefix, Namespace: ns}, nil) {
				return
			}
		}
	}
}

// TriplesWithLimit returns matching triples in the graph addressed by graph,
// skipping the first offset items then yielding up to limit items. If
// limit <= 0, all remaining items after offset are yielded.
// Safe for concurrent use.
func (m *MemoryStore) TriplesWithLimit(ctx context.Context, pattern term.TriplePattern, graph term.Term, limit, offset int) TripleIterator {
	if err := ctx.Err(); err != nil {
		return ErrorIterator[term.Triple](err)
	}
	return func(yield func(term.Triple, error) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		idx := m.index(graph)
		if idx == nil {
			return
		}
		skipped := 0
		yielded := 0
		idx.each(pattern, func(t term.Triple, _ error) bool {
			if skipped < offset {
				skipped++
				return true
			}
			if limit > 0 && yielded >= limit {
				return false
			}
			yielded++
			return yield(t, nil)
		})
	}
}

// Count returns the number of triples matching the pattern in the graph
// addressed by graph, from index sizes. Safe for concurrent use.
func (m *MemoryStore) Count(ctx context.Context, pattern term.TriplePattern, graph term.Term) (int, error) {
	return m.Cardinality(ctx, pattern, graph)
}

// Cardinality returns the number of triples matching the pattern in the graph
// addressed by graph without visiting them. A pattern bound in two or three
// positions, only in the predicate, or not at all is answered in constant time;
// subject-only walks the subject's predicates and object-only the object's
// subjects, never the triples. Safe for concurrent use.
func (m *MemoryStore) Cardinality(ctx context.Context, pattern term.TriplePattern, graph term.Term) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	if idx := m.index(graph); idx != nil {
		return idx.cardinality(pattern), nil
	}
	return 0, nil
}

// Exists reports whether at least one triple matching the pattern exists in
// the graph addressed by graph. Safe for concurrent use.
func (m *MemoryStore) Exists(ctx context.Context, pattern term.TriplePattern, graph term.Term) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	idx := m.index(graph)
	if idx == nil {
		return false, nil
	}
	found := false
	idx.each(pattern, func(term.Triple, error) bool {
		found = true
		return false
	})
	return found, nil
}

// Compile-time checks: MemoryStore implements the optional interfaces.
var (
	_ QueryableStore   = (*MemoryStore)(nil)
	_ CardinalityStore = (*MemoryStore)(nil)
)
