package store_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// unionVocab is small on purpose: patterns, cycles and duplicates between the
// layers are common, which is where a union gets its answers wrong.
type unionVocab struct {
	nodes  []term.Term
	preds  []term.URIRef
	graphs []term.Term
}

func newUnionVocab() unionVocab {
	v := unionVocab{graphs: []term.Term{nil, term.NewURIRefUnsafe("http://example.org/g1")}}
	for i := range 5 {
		v.nodes = append(v.nodes, term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/n%d", i)))
	}
	v.nodes = append(v.nodes, term.NewBNode("b0"), term.NewLiteral("lit"))
	for i := range 4 {
		v.preds = append(v.preds, term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/p%d", i)))
	}
	return v
}

func (v unionVocab) triple(r *rand.Rand, preds []term.URIRef) term.Triple {
	subjects := v.nodes[:len(v.nodes)-1] // no literal subject
	s := subjects[r.IntN(len(subjects))].(term.Subject)
	return term.Triple{Subject: s, Predicate: preds[r.IntN(len(preds))], Object: v.nodes[r.IntN(len(v.nodes))]}
}

// patterns returns every bound/unbound combination over a sample of terms,
// including a term the data never uses.
func (v unionVocab) patterns() []term.TriplePattern {
	absent := term.NewURIRefUnsafe("http://example.org/absent")
	subj := []term.Subject{nil, v.nodes[0].(term.Subject), v.nodes[5].(term.Subject), absent}
	pred := []*term.URIRef{nil, &v.preds[0], &v.preds[3], &absent}
	obj := []term.Term{nil, v.nodes[1], v.nodes[6], absent}
	var out []term.TriplePattern
	for _, s := range subj {
		for _, p := range pred {
			for _, o := range obj {
				out = append(out, term.TriplePattern{Subject: s, Predicate: p, Object: o})
			}
		}
	}
	return out
}

func keysOf(t *testing.T, it store.TripleIterator) []string {
	t.Helper()
	var ks []string
	for tr, err := range it {
		if err != nil {
			t.Fatal(err)
		}
		ks = append(ks, term.TermKey(tr.Subject)+"\x00"+term.TermKey(tr.Predicate)+"\x00"+term.TermKey(tr.Object))
	}
	slices.Sort(ks)
	return ks
}

func termKeys(ts []term.Term) []string {
	ks := make([]string, len(ts))
	for i, x := range ts {
		ks[i] = term.TermKey(x)
	}
	slices.Sort(ks)
	return ks
}

// TestUnionStoreDifferential compares every read of a UnionStore, optional
// interfaces included, with a MemoryStore holding the union of its layers, on
// seeded random data. The over store overlaps the base, is sometimes empty,
// and sometimes uses only some predicates, so that Reachable both declines
// and hands the query to the base. The reference is the plain store with no
// union in it, which is the "off" side for this layer.
func TestUnionStoreDifferential(t *testing.T) {
	ctx := context.Background()
	v := newUnionVocab()
	answered, declined := 0, 0
	for seed := range uint64(200) {
		r := rand.New(rand.NewPCG(seed, 0x51))
		base, over, all := store.NewMemoryStore(), store.NewMemoryStore(), store.NewMemoryStore()
		var baseTriples []term.Triple
		for range r.IntN(25) {
			tr := v.triple(r, v.preds)
			g := v.graphs[r.IntN(2)]
			baseTriples = append(baseTriples, tr)
			mustAdd(t, base, tr, g)
			mustAdd(t, all, tr, g)
		}
		overPreds := v.preds[:1+r.IntN(len(v.preds))]
		nOver := 0
		switch r.IntN(4) {
		case 0: // empty over
		case 1:
			nOver = 1 + r.IntN(3)
		default:
			nOver = r.IntN(10)
		}
		for range nOver {
			tr := v.triple(r, overPreds)
			if len(baseTriples) > 0 && r.IntN(3) == 0 { // a triple both layers hold
				tr = baseTriples[r.IntN(len(baseTriples))]
			}
			g := v.graphs[r.IntN(2)]
			mustAdd(t, over, tr, g)
			mustAdd(t, all, tr, g)
		}
		u := store.NewUnion(base, over)

		for _, g := range v.graphs {
			for _, p := range v.patterns() {
				name := fmt.Sprintf("seed %d graph %v pattern %v", seed, g, p)
				want := keysOf(t, all.Triples(ctx, p, g))
				if got := keysOf(t, u.Triples(ctx, p, g)); !slices.Equal(got, want) {
					t.Fatalf("%s: Triples %v, want %v", name, got, want)
				}
				n, err := u.Count(ctx, p, g)
				if err != nil || n != len(want) {
					t.Fatalf("%s: Count %d, %v; want %d", name, n, err, len(want))
				}
				ex, err := u.Exists(ctx, p, g)
				if err != nil || ex != (len(want) > 0) {
					t.Fatalf("%s: Exists %v, %v; want %v", name, ex, err, len(want) > 0)
				}
				// windows: each the right size, no duplicates, all from the set
				for _, w := range [][2]int{{0, 0}, {1, 0}, {2, 1}, {3, len(want) - 1}, {5, len(want) + 1}} {
					limit, offset := w[0], max(w[1], 0)
					got := keysOf(t, u.TriplesWithLimit(ctx, p, g, limit, offset))
					size := max(len(want)-offset, 0)
					if limit > 0 {
						size = min(size, limit)
					}
					if len(got) != size || len(slices.Compact(slices.Clone(got))) != len(got) {
						t.Fatalf("%s: TriplesWithLimit(%d, %d) %v, want %d distinct of %v", name, limit, offset, got, size, want)
					}
					for _, k := range got {
						if _, ok := slices.BinarySearch(want, k); !ok {
							t.Fatalf("%s: TriplesWithLimit(%d, %d) yielded %q, not in the union", name, limit, offset, k)
						}
					}
				}
			}

			for _, start := range v.nodes {
				for range 3 {
					q := store.ReachabilityQuery{Start: start, Context: g, Inverse: r.IntN(2) == 0, Negated: r.IntN(3) == 0, MaxDepth: r.IntN(4)}
					for _, p := range v.preds {
						if r.IntN(2) == 0 {
							q.Predicates = append(q.Predicates, p)
						}
					}
					got, err := u.Reachable(ctx, q)
					if errors.Is(err, store.ErrReachabilityUnsupported) {
						declined++
						continue
					}
					if err != nil {
						t.Fatal(err)
					}
					answered++
					want, err := all.Reachable(ctx, q)
					if err != nil {
						t.Fatal(err)
					}
					if !slices.Equal(termKeys(got), termKeys(want)) {
						t.Fatalf("seed %d: Reachable(%+v) = %v, want %v", seed, q, termKeys(got), termKeys(want))
					}
				}
			}
		}
	}
	// both branches of Reachable must have run, or the test proves nothing
	if answered < 100 || declined < 100 {
		t.Fatalf("Reachable answered %d and declined %d queries; both must be exercised", answered, declined)
	}
}

func mustAdd(t *testing.T, s store.Store, tr term.Triple, g term.Term) {
	t.Helper()
	if err := s.Add(context.Background(), tr, g); err != nil {
		t.Fatal(err)
	}
}
