package sparql

import (
	rdflibgo "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
)

// snapshotQueryGraphs points the query's graphs at one read snapshot per store
// that offers one (store.SnapshotStore), so the whole evaluation reads from a
// single transaction instead of opening one per lookup. It returns the graph
// and query to evaluate and a function that releases the snapshots.
//
// The caller's query is not changed: when a named graph is redirected, the
// query is copied with a new NamedGraphs map. Graphs on stores without
// snapshots are used as they are.
func snapshotQueryGraphs(g *rdflibgo.Graph, q *ParsedQuery) (*rdflibgo.Graph, *ParsedQuery, func()) {
	snaps := make(map[store.Store]store.Store)
	var releases []func()
	redirect := func(gr *rdflibgo.Graph) *rdflibgo.Graph {
		if gr == nil {
			return nil
		}
		st := gr.Store()
		ss, ok := st.(store.SnapshotStore)
		if !ok {
			return gr
		}
		snap, seen := snaps[st]
		if !seen {
			var release func()
			snap, release = ss.ReadSnapshot()
			snaps[st] = snap
			releases = append(releases, release)
		}
		return graph.NewGraphFromStore(snap, gr.Identifier())
	}

	g = redirect(g)
	if anySnapshot(q.NamedGraphs) {
		named := make(map[string]*rdflibgo.Graph, len(q.NamedGraphs))
		for iri, ng := range q.NamedGraphs {
			named[iri] = redirect(ng)
		}
		qc := *q
		qc.NamedGraphs = named
		q = &qc
	}
	return g, q, func() {
		for _, r := range releases {
			r()
		}
	}
}

func anySnapshot(graphs map[string]*rdflibgo.Graph) bool {
	for _, g := range graphs {
		if g == nil {
			continue
		}
		if _, ok := g.Store().(store.SnapshotStore); ok {
			return true
		}
	}
	return false
}
