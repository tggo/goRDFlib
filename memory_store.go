package rdflibgo

import "sync"

// MemoryStore is a thread-safe in-memory triple store with 3 indices (SPO, POS, OSP).
// Ported from: rdflib.plugins.stores.memory.SimpleMemory
type MemoryStore struct {
	mu sync.RWMutex

	// Triple indices: nested maps for efficient pattern matching.
	// Keys are N3() strings of terms for map-key compatibility.
	spo map[string]map[string]map[string]Triple // subject → predicate → object → triple
	pos map[string]map[string]map[string]Triple // predicate → object → subject → triple
	osp map[string]map[string]map[string]Triple // object → subject → predicate → triple

	// Namespace bindings
	nsPrefix map[string]URIRef // prefix → namespace
	nsURI    map[string]string // namespace → prefix

	count int
}

// NewMemoryStore creates a new empty in-memory store.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.__init__
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		spo:      make(map[string]map[string]map[string]Triple),
		pos:      make(map[string]map[string]map[string]Triple),
		osp:      make(map[string]map[string]map[string]Triple),
		nsPrefix: make(map[string]URIRef),
		nsURI:    make(map[string]string),
	}
}

// ContextAware reports whether this store supports named graphs.
func (m *MemoryStore) ContextAware() bool { return false }

// TransactionAware reports whether this store supports transactions.
func (m *MemoryStore) TransactionAware() bool { return false }

// Add inserts a triple into the store.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.add
func (m *MemoryStore) Add(t Triple, context Term) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addLocked(t)
}

// addLocked inserts a triple without acquiring the lock. Caller must hold m.mu.Lock().
func (m *MemoryStore) addLocked(t Triple) {
	sk, pk, ok := termKey(t.Subject), termKey(t.Predicate), termKey(t.Object)

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
func ensureInsert(idx map[string]map[string]map[string]Triple, k1, k2, k3 string, t Triple) {
	if idx[k1] == nil {
		idx[k1] = make(map[string]map[string]Triple)
	}
	if idx[k1][k2] == nil {
		idx[k1][k2] = make(map[string]Triple)
	}
	idx[k1][k2][k3] = t
}

// AddN atomically batch-adds quads.
// Ported from: rdflib.store.Store.addN
func (m *MemoryStore) AddN(quads []Quad) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, q := range quads {
		m.addLocked(q.Triple)
	}
}

// Remove deletes triples matching the pattern.
// The match and delete are performed under a single write lock to avoid TOCTOU races.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.remove
func (m *MemoryStore) Remove(pattern TriplePattern, context Term) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Collect matches under the same lock
	var toRemove []Triple
	m.triplesLocked(pattern)(func(t Triple) bool {
		toRemove = append(toRemove, t)
		return true
	})

	for _, t := range toRemove {
		m.removeLocked(t)
	}
}

// removeLocked removes a single triple from all indices. Caller must hold m.mu.Lock().
func (m *MemoryStore) removeLocked(t Triple) {
	sk, pk, ok := termKey(t.Subject), termKey(t.Predicate), termKey(t.Object)

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
func (m *MemoryStore) Triples(pattern TriplePattern, context Term) TripleIterator {
	return func(yield func(Triple) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		m.triplesLocked(pattern)(yield)
	}
}

// triplesLocked returns matching triples without acquiring locks. Caller must hold at least RLock.
func (m *MemoryStore) triplesLocked(pattern TriplePattern) TripleIterator {
	return func(yield func(Triple) bool) {
		sk := optTermKey(pattern.Subject)
		pk := optPredKey(pattern.Predicate)
		ok := optTermKey(pattern.Object)

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
func (m *MemoryStore) Len(context Term) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.count
}

// Contexts returns an empty iterator (not context-aware).
func (m *MemoryStore) Contexts(triple *Triple) TermIterator {
	return func(yield func(Term) bool) {}
}

// Bind associates a prefix with a namespace.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.bind
func (m *MemoryStore) Bind(prefix string, namespace URIRef) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nsPrefix[prefix] = namespace
	m.nsURI[namespace.Value()] = prefix
}

// Namespace returns the namespace URI for a prefix.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.namespace
func (m *MemoryStore) Namespace(prefix string) (URIRef, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ns, ok := m.nsPrefix[prefix]
	return ns, ok
}

// Prefix returns the prefix for a namespace URI.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.prefix
func (m *MemoryStore) Prefix(namespace URIRef) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.nsURI[namespace.Value()]
	return p, ok
}

// Namespaces returns an iterator over all namespace bindings.
// Ported from: rdflib.plugins.stores.memory.SimpleMemory.namespaces
func (m *MemoryStore) Namespaces() NamespaceIterator {
	return func(yield func(string, URIRef) bool) {
		m.mu.RLock()
		defer m.mu.RUnlock()
		for prefix, ns := range m.nsPrefix {
			if !yield(prefix, ns) {
				return
			}
		}
	}
}

// termKey returns a stable string key for a term (its N3 representation).
func termKey(t Term) string {
	return t.N3()
}

func optTermKey(t Term) string {
	if t == nil {
		return ""
	}
	return t.N3()
}

func optPredKey(p *URIRef) string {
	if p == nil {
		return ""
	}
	return p.N3()
}
