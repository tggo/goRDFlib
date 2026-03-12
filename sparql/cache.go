package sparql

import "sync"

// QueryCache is an LRU cache for parsed SPARQL queries.
// Safe for concurrent use.
type QueryCache struct {
	mu       sync.RWMutex
	capacity int
	entries  map[string]*cacheEntry
	order    []string // oldest first
}

type cacheEntry struct {
	query *ParsedQuery
}

// NewQueryCache creates a query cache with the given capacity.
func NewQueryCache(capacity int) *QueryCache {
	if capacity <= 0 {
		capacity = 256
	}
	return &QueryCache{
		capacity: capacity,
		entries:  make(map[string]*cacheEntry, capacity),
		order:    make([]string, 0, capacity),
	}
}

// Get returns a cached ParsedQuery for the exact query string, or nil.
func (c *QueryCache) Get(query string) *ParsedQuery {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if e, ok := c.entries[query]; ok {
		return e.query
	}
	return nil
}

// Put stores a ParsedQuery in the cache. Evicts the oldest entry if at capacity.
func (c *QueryCache) Put(query string, q *ParsedQuery) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[query]; ok {
		return // already cached
	}
	if len(c.entries) >= c.capacity {
		// Evict oldest
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.entries, oldest)
	}
	c.entries[query] = &cacheEntry{query: q}
	c.order = append(c.order, query)
}

// Len returns the number of cached entries.
func (c *QueryCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
