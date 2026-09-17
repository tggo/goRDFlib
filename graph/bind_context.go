package graph

import (
	"context"

	"github.com/tggo/goRDFlib/store"
)

// BindContext returns a graph with the same identifier and base whose store
// calls carry ctx, when its store implements store.ContextBinder. Otherwise it
// returns g itself, so an unbound store costs no allocation. The returned graph
// reads and writes the same data as g.
func (g *Graph) BindContext(ctx context.Context) *Graph {
	if g == nil || ctx == nil {
		return g
	}
	b, ok := g.store.(store.ContextBinder)
	if !ok {
		return g
	}
	c := *g
	c.store = b.BindContext(ctx)
	return &c
}
