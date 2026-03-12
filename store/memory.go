package store

import (
	"sync"

	"github.com/tggo/goRDFlib/term"
)

// MemoryStore is a thread-safe in-memory triple store with 3 indices (SPO, POS, OSP).
// All methods are safe for concurrent use.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory
type MemoryStore struct {
	mu sync.RWMutex

	// Triple indices: nested maps for efficient pattern matching.
	// Keys are TermKey() strings for map-key compatibility.
	spo map[string]map[string]map[string]term.Triple // subject → predicate → object → triple
	pos map[string]map[string]map[string]term.Triple // predicate → object → subject → triple
	osp map[string]map[string]map[string]term.Triple // object → subject → predicate → triple

	// Namespace bindings
	nsPrefix map[string]term.URIRef // prefix → namespace
	nsURI    map[string]string      // namespace → prefix

	count int
}

// NewMemoryStore creates a new empty in-memory store.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.__init__
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		spo:      make(map[string]map[string]map[string]term.Triple),
		pos:      make(map[string]map[string]map[string]term.Triple),
		osp:      make(map[string]map[string]map[string]term.Triple),
		nsPrefix: make(map[string]term.URIRef),
		nsURI:    make(map[string]string),
	}
}

// ContextAware reports whether this store supports named graphs.
func (m *MemoryStore) ContextAware() bool { return false }

// TransactionAware reports whether this store supports transactions.
func (m *MemoryStore) TransactionAware() bool { return false }

// Add inserts a triple into the store.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.add
func (m *MemoryStore) Add(t term.Triple, context term.Term) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addLocked(t)
}

// addLocked inserts a triple without acquiring the lock. Caller must hold m.mu.Lock().
func (m *MemoryStore) addLocked(t term.Triple) {
	sk, pk, ok := term.TermKey(t.Subject), term.TermKey(t.Predicate), term.TermKey(t.Object)

	// Check if already exists
	if po, exists := m.spo[sk]; exists {
		if o, exists := po[pk]; exists {
			if _, exists := o[ok]; exists {
				return
			}
		}
	}

	ensureInsert(m.spo, sk, pk, ok, t)
	ensureInsert(m.pos, pk, ok, sk, t)
	ensureInsert(m.osp, ok, sk, pk, t)
	m.count++
}

// ensureInsert inserts t into a 3-level nested map, creating intermediate maps as needed.
func ensureInsert(idx map[string]map[string]map[string]term.Triple, k1, k2, k3 string, t term.Triple) {
	if idx[k1] == nil {
		idx[k1] = make(map[string]map[string]term.Triple)
	}
	if idx[k1][k2] == nil {
		idx[k1][k2] = make(map[string]term.Triple)
	}
	idx[k1][k2][k3] = t
}

// Set atomically removes all triples matching (s, p, *) and adds the new triple
// under a single write lock.
func (m *MemoryStore) Set(t term.Triple, context term.Term) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Remove all triples with the same subject and predicate.
	pattern := term.TriplePattern{Subject: t.Subject, Predicate: &t.Predicate}
	var toRemove []term.Triple
	m.triplesLocked(pattern)(func(old term.Triple) bool {
		toRemove = append(toRemove, old)
		return true
	})
	for _, old := range toRemove {
		m.removeLocked(old)
	}

	m.addLocked(t)
}

// AddN atomically batch-adds quads.
// Ported from: rdflib.store.Store.addN
func (m *MemoryStore) AddN(quads []term.Quad) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, q := range quads {
		m.addLocked(q.Triple)
	}
}

// Remove deletes triples matching the pattern.
// The match and delete are performed under a single write lock to avoid TOCTOU races.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.remove
func (m *MemoryStore) Remove(pattern term.TriplePattern, context term.Term) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Collect matches under the same lock
	var toRemove []term.Triple
	m.triplesLocked(pattern)(func(t term.Triple) bool {
		toRemove = append(toRemove, t)
		return true
	})

	for _, t := range toRemove {
		m.removeLocked(t)
	}
}

// removeLocked removes a single triple from all indices. Caller must hold m.mu.Lock().
func (m *MemoryStore) removeLocked(t term.Triple) {
	sk, pk, ok := term.TermKey(t.Subject), term.TermKey(t.Predicate), term.TermKey(t.Object)

	// Check existence in SPO first — only decrement count if triple actually exists
	found := false
	if po, exists := m.spo[sk]; exists {
		if o, exists := po[pk]; exists {
			if _, exists := o[ok]; exists {
				found = true
				delete(o, ok)
				if len(o) == 0 {
					delete(po, pk)
				}
				if len(po) == 0 {
					delete(m.spo, sk)
				}
			}
		}
	}

	if !found {
		return
	}

	if os, exists := m.pos[pk]; exists {
		if s, exists := os[ok]; exists {
			delete(s, sk)
			if len(s) == 0 {
				delete(os, ok)
			}
			if len(os) == 0 {
				delete(m.pos, pk)
			}
		}
	}

	if sp, exists := m.osp[ok]; exists {
		if p, exists := sp[sk]; exists {
			delete(p, pk)
			if len(p) == 0 {
				delete(sp, sk)
			}
			if len(sp) == 0 {
				delete(m.osp, ok)
			}
		}
	}

	m.count--
}

// Triples returns matching triples.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.triples
func (m *MemoryStore) Triples(pattern term.TriplePattern, context term.Term) TripleIterator {
	return func(yield func(term.Triple) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		m.triplesLocked(pattern)(yield)
	}
}

// triplesLocked returns matching triples without acquiring locks. Caller must hold at least RLock.
func (m *MemoryStore) triplesLocked(pattern term.TriplePattern) TripleIterator {
	return func(yield func(term.Triple) bool) {
		sk := term.OptTermKey(pattern.Subject)
		pk := term.OptPredKey(pattern.Predicate)
		ok := term.OptTermKey(pattern.Object)

		switch {
		case sk != "" && pk != "" && ok != "":
			if po, exists := m.spo[sk]; exists {
				if o, exists := po[pk]; exists {
					if t, exists := o[ok]; exists {
						yield(t)
					}
				}
			}

		case sk != "":
			if po, exists := m.spo[sk]; exists {
				for pk2, o := range po {
					if pk != "" && pk2 != pk {
						continue
					}
					for ok2, t := range o {
						if ok != "" && ok2 != ok {
							continue
						}
						if !yield(t) {
							return
						}
					}
				}
			}

		case pk != "":
			if os, exists := m.pos[pk]; exists {
				for ok2, s := range os {
					if ok != "" && ok2 != ok {
						continue
					}
					for sk2, t := range s {
						if sk != "" && sk2 != sk {
							continue
						}
						if !yield(t) {
							return
						}
					}
				}
			}

		case ok != "":
			if sp, exists := m.osp[ok]; exists {
				for sk2, p := range sp {
					if sk != "" && sk2 != sk {
						continue
					}
					for pk2, t := range p {
						if pk != "" && pk2 != pk {
							continue
						}
						if !yield(t) {
							return
						}
					}
				}
			}

		default:
			for _, po := range m.spo {
				for _, o := range po {
					for _, t := range o {
						if !yield(t) {
							return
						}
					}
				}
			}
		}
	}
}

// Len returns the number of triples.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.__len__
func (m *MemoryStore) Len(context term.Term) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.count
}

// Contexts returns an empty iterator (not context-aware).
func (m *MemoryStore) Contexts(triple *term.Triple) TermIterator {
	return func(yield func(term.Term) bool) {}
}

// Bind associates a prefix with a namespace.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.bind
func (m *MemoryStore) Bind(prefix string, namespace term.URIRef) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nsPrefix[prefix] = namespace
	m.nsURI[namespace.Value()] = prefix
}

// Namespace returns the namespace URI for a prefix.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.namespace
func (m *MemoryStore) Namespace(prefix string) (term.URIRef, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ns, ok := m.nsPrefix[prefix]
	return ns, ok
}

// Prefix returns the prefix for a namespace URI.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.prefix
func (m *MemoryStore) Prefix(namespace term.URIRef) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.nsURI[namespace.Value()]
	return p, ok
}

// Namespaces returns an iterator over all namespace bindings.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.namespaces
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

// TriplesWithLimit returns matching triples skipping the first offset items then yielding
// up to limit items. If limit <= 0, all remaining items after offset are yielded.
// Safe for concurrent use.
func (m *MemoryStore) TriplesWithLimit(pattern term.TriplePattern, ctx term.Term, limit, offset int) TripleIterator {
	return func(yield func(term.Triple) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		skipped := 0
		yielded := 0
		m.triplesLocked(pattern)(func(t term.Triple) bool {
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

// Count returns the number of triples matching the pattern.
// Safe for concurrent use.
func (m *MemoryStore) Count(pattern term.TriplePattern, ctx term.Term) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	n := 0
	m.triplesLocked(pattern)(func(term.Triple) bool {
		n++
		return true
	})
	return n
}

// Exists reports whether at least one triple matching the pattern exists.
// Safe for concurrent use.
func (m *MemoryStore) Exists(pattern term.TriplePattern, ctx term.Term) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	found := false
	m.triplesLocked(pattern)(func(term.Triple) bool {
		found = true
		return false
	})
	return found
}

// Compile-time check: MemoryStore must implement QueryableStore.
var _ QueryableStore = (*MemoryStore)(nil)
