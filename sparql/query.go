package sparql

import (
	"fmt"

	rdflibgo "github.com/tggo/goRDFlib"
)

var queryCache *QueryCache // package-level, nil = disabled

// EnableQueryCache enables LRU caching of parsed SPARQL queries.
// Cached ParsedQuery values are safe to reuse because EvalQuery deep-copies
// mutable state (prefixes) before evaluation.
func EnableQueryCache(capacity int) {
	queryCache = NewQueryCache(capacity)
}

// DisableQueryCache disables and clears the query cache.
func DisableQueryCache() {
	queryCache = nil
}

// Query executes a SPARQL query against the graph.
func Query(g *rdflibgo.Graph, query string, initBindings ...map[string]rdflibgo.Term) (*Result, error) {
	var q *ParsedQuery
	if queryCache != nil {
		q = queryCache.Get(query)
	}
	if q == nil {
		var err error
		q, err = Parse(query)
		if err != nil {
			return nil, fmt.Errorf("sparql parse error: %w", err)
		}
		if queryCache != nil {
			queryCache.Put(query, q)
		}
	}

	var bindings map[string]rdflibgo.Term
	if len(initBindings) > 0 {
		bindings = initBindings[0]
	}

	return EvalQuery(g, q, bindings)
}

// Update executes a SPARQL Update request against a dataset.
func Update(ds *Dataset, update string) error {
	u, err := ParseUpdate(update)
	if err != nil {
		return fmt.Errorf("sparql update parse error: %w", err)
	}
	return EvalUpdate(ds, u)
}
