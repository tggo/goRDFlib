package shacl

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// ErrNilGraph is returned by Overlay on a nil graph.
var ErrNilGraph = errors.New("shacl: nil graph")

// ErrReadOnly is returned by Add and Merge on an overlay graph (Overlay). It
// is the same error as store.ErrReadOnly, which the overlay's SPARQL store
// returns for its own writes.
var ErrReadOnly = store.ErrReadOnly

// overlay is what an overlay graph keeps besides its indexes.
type overlay struct {
	base  *Graph
	added []term.Triple
}

// Overlay returns a graph that reads as g plus the added triples, for
// validating many small variants of one large graph: each of many small
// changes tried on it, or each of many data graphs validated over a large
// shared vocabulary or reference data set.
//
// NewGraphFromRDF indexes every triple of the graph it wraps, so a variant
// built by adding to the store and wrapping it again re-indexes the whole of
// it each time. Here g's indexes are built once (on the first overlay, or by
// NewGraphFromRDF) and shared read-only by every overlay over it; an overlay
// indexes only its added triples, and each lookup reads g, then the overlay.
// An added triple g already holds is ignored. The overlay keeps its own copy
// of the reifier lookups (SHACL 1.2) and reads g's namespace bindings.
//
// Nothing is written to g or to its store. SPARQL-based constraints and
// SHACL-AF see g plus added through a read-only union of g's store and a small
// in-memory one holding the added triples g's store does not hold, built on
// the first such query. The two share no triple, so the union skips the dedup
// of store.UnionStore and gives the SPARQL planner the cardinalities, counts
// and reachability of g's store, when it has them. So overlays over one base
// validate concurrently, SPARQL shapes included, without any store the caller
// has to keep in step. Added triples belong to the graph g wraps (the default
// graph of a plain graph).
//
// The overlay is read-only: Add and Merge return ErrReadOnly, and
// InvalidateIndexes does nothing (it has no error to return, and re-indexing
// would defeat the point). g must not be changed — Add, Merge, or writes to its
// store followed by InvalidateIndexes — while any overlay over it is in use:
// the overlays read g's indexes and its store as they are. Breaking this is not
// detected: a triple that reaches g's store after an overlay first answered a
// SPARQL query is then in both of its layers, and that overlay counts and
// yields it twice. Overlays are safe for concurrent reads, and an overlay may
// itself be the base of another.
func (g *Graph) Overlay(ctx context.Context, added []term.Triple) (*Graph, error) {
	if g == nil {
		return nil, fmt.Errorf("%w: Overlay needs a base graph", ErrNilGraph)
	}
	under, err := g.ensureIndexesContext(ctx)
	if err != nil {
		return nil, err
	}
	idx := &graphIndexes{
		spo:   make(map[ikey]map[ikey][]Term),
		pos:   make(map[ikey]map[ikey][]Term),
		p:     make(map[ikey][]Triple),
		ns:    under.ns,
		under: under,
	}
	// reifier lookups read these two directly: start from base's (a copy, so
	// added entries never write into base's backing arrays)
	idx.reifies = append([]reifyEntry(nil), under.reifies...)
	if under.reifiesBy != nil {
		idx.reifiesBy = maps.Clone(under.reifiesBy)
		for k, v := range idx.reifiesBy {
			idx.reifiesBy[k] = v[:len(v):len(v)]
		}
	}
	for _, t := range added {
		idx.addOver(t)
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	out := &Graph{baseURI: g.baseURI, over: &overlay{base: g, added: slices.Clone(added)}}
	out.idx.Store(idx)
	return out, nil
}

// rdfOverlay builds the rdflib graph of an overlay: a read-only union of the
// base's graph and a memory store holding the added triples the base does not
// hold (newOverlayStore).
func (g *Graph) rdfOverlay(ctx context.Context) (*graph.Graph, error) {
	brg, err := g.over.base.rdf(ctx)
	if err != nil {
		return nil, err
	}
	id := brg.Identifier()
	base := brg.Store()
	extra := store.NewMemoryStore()
	empty := true
	for _, t := range g.over.added {
		held, err := holds(ctx, base, t, id)
		if err != nil {
			return nil, fmt.Errorf("shacl: building the store of an overlay graph: %w", err)
		}
		if held {
			continue
		}
		if err := extra.Add(ctx, t, id); err != nil {
			return nil, fmt.Errorf("shacl: building the store of an overlay graph: %w", err)
		}
		empty = false
	}
	return graph.NewGraph(graph.WithStore(newOverlayStore(base, extra, empty)),
		graph.WithIdentifier(id), graph.WithBase(g.baseURI)), nil
}

// holds reports whether s holds t in graph.
func holds(ctx context.Context, s store.Store, t term.Triple, graph term.Term) (bool, error) {
	p := t.Predicate
	for _, err := range s.Triples(ctx, term.TriplePattern{Subject: t.Subject, Predicate: &p, Object: t.Object}, graph) {
		return err == nil, err
	}
	return false, nil
}

// addOver indexes one added triple of an overlay, unless the overlay (its base
// included) already holds it.
func (idx *graphIndexes) addOver(t term.Triple) {
	s := fromRDFLib(t.Subject)
	p := fromRDFLib(t.Predicate)
	o := fromRDFLib(t.Object)
	sk, pk, ok := indexKey(s), indexKey(p), indexKey(o)
	for _, have := range idx.objects(sk, pk) {
		if indexKey(have) == ok {
			return
		}
	}
	if tt, isTT := t.Object.(term.TripleTerm); isTT && t.Predicate.Value() == RDF+"reifies" {
		idx.reifies = append(idx.reifies, reifyEntry{s, tt})
		if idx.reifiesBy == nil {
			idx.reifiesBy = make(map[ikey][]term.TripleTerm)
		}
		idx.reifiesBy[sk] = append(idx.reifiesBy[sk], tt)
	}
	sp := idx.spo[sk]
	if sp == nil {
		sp = make(map[ikey][]Term)
		idx.spo[sk] = sp
	}
	sp[pk] = append(sp[pk], o)
	idx.n++
	po := idx.pos[pk]
	if po == nil {
		po = make(map[ikey][]Term)
		idx.pos[pk] = po
	}
	po[ok] = append(po[ok], s)
	idx.p[pk] = append(idx.p[pk], Triple{Subject: s, Predicate: p, Object: o})
}

// len is the number of distinct triples, an overlay's base included.
func (idx *graphIndexes) len() int {
	n := 0
	for ; idx != nil; idx = idx.under {
		n += idx.n
	}
	return n
}

// concatClipped returns a followed by b. When one is empty the other is
// returned without copying (it may be an index's own slice), but clipped to
// its length: an append by a caller then reallocates instead of writing into
// spare capacity that the base index, or another overlay over it, can still
// see. Callers must not write to the elements either; none does (sorting is
// always done on a copy), and TestOverlayReturnedSlicesAreIsolated holds the
// append half of that.
func concatClipped[T any](a, b []T) []T {
	switch {
	case len(b) == 0:
		return clip(a)
	case len(a) == 0:
		return clip(b)
	}
	out := make([]T, 0, len(a)+len(b))
	return append(append(out, a...), b...)
}

// clip limits the capacity of s to its length.
func clip[T any](s []T) []T { return s[:len(s):len(s)] }
