package sparql

import (
	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/internal/iri"
)

// applyDatasetClause builds the dataset a query's FROM and FROM NAMED clauses
// describe (SPARQL 1.1 §13.2), out of the graphs the caller supplied in
// q.NamedGraphs:
//
//   - the default graph is the merge of the FROM graphs, and the graph passed
//     to the evaluator is not part of it;
//   - with FROM NAMED but no FROM, the default graph is empty;
//   - GRAPH ranges over the FROM NAMED graphs only.
//
// The clause restricts the dataset the caller supplied; the engine never
// fetches a graph from the Web, and an IRI q.NamedGraphs has no graph for
// contributes an empty graph, as it does in Jena, RDF4J and rdflib. A caller
// that wants to serve those IRIs itself loads them into q.NamedGraphs before
// evaluating; a caller that wants the clause ignored — the behaviour of every
// release before this one — clears q.DatasetClause.
//
// Without a dataset clause the graph and query are returned unchanged, so a
// query that declares no dataset keeps evaluating against exactly what the
// caller passed.
func applyDatasetClause(ec *evalCtx, g *rdflibgo.Graph, q *ParsedQuery) (*rdflibgo.Graph, *ParsedQuery, error) {
	if len(q.DatasetClause) == 0 {
		return g, q, nil
	}

	def := rdflibgo.NewGraph()
	named := make(map[string]*rdflibgo.Graph, len(q.DatasetClause))
	merged := make(map[string]bool, len(q.DatasetClause))
	for _, dc := range q.DatasetClause {
		name := dc.IRI
		if q.BaseURI != "" && !iri.IsAbsolute(name) {
			name = iri.Resolve(q.BaseURI, name)
		}
		ng := q.NamedGraphs[name]
		if dc.Named {
			// An IRI with no graph behind it names no graph of the dataset,
			// so GRAPH ?g does not bind it — the same as Jena's dynamic
			// dataset. Binding it to an empty graph would invent a graph the
			// caller never supplied.
			if ng != nil {
				named[name] = ng
			}
			continue
		}
		// FROM twice with the same IRI merges the graph once: a merge is a
		// set union, and the triples are the same triples.
		if ng == nil || merged[name] {
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
