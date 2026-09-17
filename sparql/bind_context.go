package sparql

import (
	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/store"
)

// Binding the context to stores (issue #35).
//
// A store that implements store.ContextBinder receives the evaluation's context
// on every call: a query binds its graphs once, before evaluating; an update
// binds each graph where it takes it from the Dataset, because operations
// create and drop graphs in Dataset.NamedGraphs, and those changes must land in
// the caller's map, not in a bound copy of it. Graphs on stores that take no
// context are used as they are, without allocating.

// bind returns g with the evaluation's context bound to its store. The result
// is cached, so the graphs of one evaluation are bound once each.
func (e *evalCtx) bind(g *rdflibgo.Graph) *rdflibgo.Graph {
	if e == nil || g == nil {
		return g
	}
	if _, ok := g.Store().(store.ContextBinder); !ok {
		return g
	}
	if b, ok := e.bound[g]; ok {
		return b
	}
	if e.bound == nil {
		e.bound = make(map[*rdflibgo.Graph]*rdflibgo.Graph)
	}
	b := g.BindContext(e.ctx)
	e.bound[g] = b
	return b
}

// bindNamed returns named with every graph bound, and whether any graph needed
// binding; when none did it returns named itself. The result is only read,
// never written back.
func (e *evalCtx) bindNamed(named map[string]*rdflibgo.Graph) (map[string]*rdflibgo.Graph, bool) {
	var out map[string]*rdflibgo.Graph
	for iri, g := range named {
		b := e.bind(g)
		if b == g && out == nil {
			continue
		}
		if out == nil {
			out = make(map[string]*rdflibgo.Graph, len(named))
			for k, v := range named {
				out[k] = v
			}
		}
		out[iri] = b
	}
	if out == nil {
		return named, false
	}
	return out, true
}

// bindQuery binds g and the query's named graphs. The caller's query is not
// changed: when a named graph is bound, the query is copied.
func (e *evalCtx) bindQuery(g *rdflibgo.Graph, q *ParsedQuery) (*rdflibgo.Graph, *ParsedQuery) {
	g = e.bind(g)
	if named, changed := e.bindNamed(q.NamedGraphs); changed {
		qc := *q
		qc.NamedGraphs = named
		q = &qc
	}
	return g, q
}
