package shacl

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/term"
)

// ErrNilGraph is returned by NewOverlayGraph when base or rg is nil.
var ErrNilGraph = errors.New("shacl: nil graph")

// NewOverlayGraph returns a graph that reads as base plus the added triples,
// for validating many small variants of one large graph: each of many small
// changes tried on it, or each of many data graphs validated over a large
// shared vocabulary or reference data set.
//
// NewGraphFromRDF indexes every triple of the graph it wraps, so a variant
// re-indexes the whole base each time. Here base's indexes are built once (on
// the first overlay, or by NewGraphFromRDF) and shared read-only by every
// overlay over it; an overlay indexes only its added triples, and each lookup
// reads base then the overlay. An added triple base already holds is ignored.
//
// rg is the rdflib graph that SPARQL-based constraints and SHACL-AF query. It
// must hold exactly base's triples plus added: typically the store base wraps,
// with the added triples inserted for this validation and removed afterwards
// (only those it did not already hold). base's indexes are a snapshot and do
// not see those writes; base itself must not be used to validate while rg
// holds the added triples.
//
// The overlay is safe for concurrent reads, and any number of overlays may
// share one base. Add, Merge or InvalidateIndexes on the overlay drop the
// overlay indexes; the next read indexes rg in full, as NewGraphFromRDF does.
func NewOverlayGraph(ctx context.Context, base *Graph, rg *graph.Graph, added []term.Triple) (*Graph, error) {
	if base == nil || rg == nil {
		return nil, fmt.Errorf("%w: NewOverlayGraph needs a base graph and the rdflib graph (base's triples plus added) that SPARQL-based constraints query", ErrNilGraph)
	}
	under, err := base.ensureIndexesContext(ctx)
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
	out := wrap(rg, base.baseURI)
	out.idx.Store(idx)
	return out, nil
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
// returned as is (it may be an index's own slice: callers only read it); else
// a new slice, so that base's index slices are never appended to.
func concatClipped[T any](a, b []T) []T {
	switch {
	case len(b) == 0:
		return a
	case len(a) == 0:
		return b
	}
	out := make([]T, 0, len(a)+len(b))
	return append(append(out, a...), b...)
}
