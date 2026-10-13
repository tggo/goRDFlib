package paths

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// reachCache remembers the answers of store.ReachabilityStore for one
// evaluation. `rdf:type/rdfs:subClassOf*` asks the store once per typed node,
// but the second step starts from the classes, and there are far fewer of
// those: without the cache 340 items in 20 classes cost 340 round trips
// (tggo/rdflibgo-mongostore#3), with it 20.
//
// The answers are only valid while the data does not change, so a cache lives
// for one read-only evaluation: one Eval call (it is on the stopper), or one
// SPARQL query (WithReachabilityCache). Errors are never cached; a failed
// query falls back to the local traversal and is asked again next time.
//
// Safe for concurrent use. Cached slices are shared and must not be modified.
type reachCache struct {
	mu sync.Mutex
	m  map[string][]term.Term
}

type reachCacheKey struct{}

// WithReachabilityCache returns a context under which every property path
// evaluation shares one cache of store.ReachabilityStore answers. Use it for
// one read-only evaluation, such as a query: a cached answer does not see a
// write made after it. sparql's query entry points install it; updates do
// not, because an operation of an update reads what the previous one wrote.
func WithReachabilityCache(ctx context.Context) context.Context {
	if _, ok := ctx.Value(reachCacheKey{}).(*reachCache); ok {
		return ctx
	}
	return context.WithValue(ctx, reachCacheKey{}, &reachCache{})
}

// reachCacheIn returns the context's cache, or nil; a nil stopper cache is
// created on first use, so a traversal that never pushes down allocates none.
func reachCacheIn(ctx context.Context) *reachCache {
	c, _ := ctx.Value(reachCacheKey{}).(*reachCache)
	return c
}

// reachable answers q from the cache or from rs.
func (c *reachCache) reachable(ctx context.Context, rs store.ReachabilityStore, q store.ReachabilityQuery) ([]term.Term, error) {
	k := reachKey(q)
	c.mu.Lock()
	r, ok := c.m[k]
	c.mu.Unlock()
	if ok {
		return r, nil
	}
	r, err := rs.Reachable(ctx, q)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	if c.m == nil {
		c.m = make(map[string][]term.Term)
	}
	c.m[k] = r
	c.mu.Unlock()
	return r, nil
}

// reachKey identifies q. The separator is NUL, as in provenance: a literal's
// lexical form goes into TermKey verbatim, so a printable separator would let
// a crafted literal forge another query's key.
func reachKey(q store.ReachabilityQuery) string {
	var b strings.Builder
	b.WriteString(term.TermKey(q.Start))
	b.WriteByte(0)
	if q.Context != nil {
		b.WriteString(term.TermKey(q.Context))
	}
	b.WriteByte(0)
	if q.Inverse {
		b.WriteByte('i')
	}
	if q.Negated {
		b.WriteByte('n')
	}
	b.WriteByte(0)
	b.WriteString(strconv.Itoa(q.MaxDepth))
	for _, p := range q.Predicates {
		b.WriteByte(0)
		b.WriteString(p.Value())
	}
	return b.String()
}
