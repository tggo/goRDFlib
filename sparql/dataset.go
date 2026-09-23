package sparql

import (
	"errors"
	"fmt"

	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/iri"
)

// ErrUnknownGraph is returned when a query's FROM or FROM NAMED clause names a
// graph the caller did not supply in ParsedQuery.NamedGraphs.
//
// The engine never fetches a graph from the Web: the IRIs of a dataset clause
// are resolved against NamedGraphs and nothing else. A caller that wants to
// serve them itself loads them into NamedGraphs before evaluating; a caller
// that wants the clause ignored — the behaviour of every release before this
// one — clears ParsedQuery.DatasetClause.
var ErrUnknownGraph = errors.New("sparql: the query names a graph that was not supplied")

// applyDatasetClause builds the dataset a query's FROM and FROM NAMED clauses
// describe (SPARQL 1.1 §13.2), out of the graphs the caller supplied in
// q.NamedGraphs:
//
//   - the default graph is the merge of the FROM graphs, and the graph passed
//     to the evaluator is not part of it;
//   - with FROM NAMED but no FROM, the default graph is empty;
//   - GRAPH ranges over the FROM NAMED graphs only.
//
// It returns the graph and query to evaluate. Without a dataset clause both
// are returned unchanged, so a query that declares no dataset keeps evaluating
// against exactly what the caller passed.
func applyDatasetClause(ec *evalCtx, g *rdflibgo.Graph, q *ParsedQuery) (*rdflibgo.Graph, *ParsedQuery, error) {
	if len(q.DatasetClause) == 0 {
		return g, q, nil
	}

	lookup := func(ref string) (*rdflibgo.Graph, string, error) {
		name := ref
		if q.BaseURI != "" && !iri.IsAbsolute(name) {
			name = iri.Resolve(q.BaseURI, name)
		}
		ng, ok := q.NamedGraphs[name]
		if !ok || ng == nil {
			return nil, name, fmt.Errorf("%w: <%s>", ErrUnknownGraph, name)
		}
		return ng, name, nil
	}

	def := rdflibgo.NewGraph()
	named := make(map[string]*rdflibgo.Graph, len(q.DatasetClause))
	merged := make(map[string]bool, len(q.DatasetClause))
	for _, dc := range q.DatasetClause {
		ng, name, err := lookup(dc.IRI)
		if err != nil {
			return nil, nil, err
		}
		if dc.Named {
			named[name] = ng
			continue
		}
		// FROM twice with the same IRI merges the graph once: a merge is a
		// set union, and the triples are the same triples.
		if merged[name] {
			continue
		}
		merged[name] = true
		for t := range ng.Triples(nil, nil, nil) {
			if ec.stop() {
				return nil, nil, ec.failure()
			}
			def.Add(t.Subject, t.Predicate, t.Object)
		}
	}

	qCopy := *q
	qCopy.NamedGraphs = named
	// The dataset is built: leaving the clause set would make a nested
	// evaluation build it a second time, against the graphs it just replaced.
	qCopy.DatasetClause = nil
	return def, &qCopy, nil
}
