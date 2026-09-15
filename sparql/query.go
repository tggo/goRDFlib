package sparql

import (
	"context"
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
//
// It cannot be cancelled; QueryContext can.
func Query(g *rdflibgo.Graph, query string, initBindings ...map[string]rdflibgo.Term) (*Result, error) {
	return QueryContext(context.Background(), g, query, initBindings...)
}

// QueryContext parses and executes a SPARQL query against the graph, and stops
// evaluating once ctx is done. A stopped query returns a nil *Result and an
// error matching ErrQueryCancelled and ctx.Err() under errors.Is; see
// EvalQueryContext for what is and is not interrupted. Parsing is not
// interrupted: its cost is linear in the query text.
func QueryContext(ctx context.Context, g *rdflibgo.Graph, query string, initBindings ...map[string]rdflibgo.Term) (*Result, error) {
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

	return EvalQueryContext(ctx, g, q, bindings)
}

// Update executes a SPARQL Update request against a dataset.
//
// It cannot be cancelled; UpdateContext can.
func Update(ds *Dataset, update string) error {
	return UpdateContext(context.Background(), ds, update)
}

// UpdateContext parses and executes a SPARQL Update request against a dataset,
// and stops once ctx is done. See EvalUpdateContext for what a cancelled
// request can leave applied.
func UpdateContext(ctx context.Context, ds *Dataset, update string) error {
	u, err := ParseUpdate(update)
	if err != nil {
		return fmt.Errorf("sparql update parse error: %w", err)
	}
	return EvalUpdateContext(ctx, ds, u)
}
