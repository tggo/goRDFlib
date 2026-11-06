package paths

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// With only the object bound, a sequence path is walked back from it (or
// forward, see backwardFrom): either way the pairs, repetitions included, are
// those of the unbound evaluation that end at the object.
func TestSequenceWithBoundObjectMatchesFilteredPairs(t *testing.T) {
	ctx := t.Context()
	g := graph.NewGraph()
	u := func(s string) term.URIRef { return term.NewURIRefUnsafe("urn:" + s) }
	typ, sub := u("type"), u("sub")
	// classes: C <- C1 <- C2, D; x0..x9 typed, x0 typed twice into C's subtree
	g.Add(ctx, u("C1"), sub, u("C"))
	g.Add(ctx, u("C2"), sub, u("C1"))
	for i := range 10 {
		x := u(fmt.Sprintf("x%d", i))
		g.Add(ctx, x, typ, u([]string{"C", "C1", "C2", "D"}[i%4]))
	}
	g.Add(ctx, u("x0"), typ, u("C2"))
	// two chains through different middles to the same pair: y -a-> m1 -b-> z, y -a-> m2 -b-> z
	g.Add(ctx, u("y"), u("a"), u("m1"))
	g.Add(ctx, u("y"), u("a"), u("m2"))
	g.Add(ctx, u("m1"), u("b"), u("z"))
	g.Add(ctx, u("m2"), u("b"), u("z"))
	// a first step rarer than the last step's triples into the object, which
	// keeps the forward walk: two r triples, twenty c triples into h
	for i := range 20 {
		g.Add(ctx, u(fmt.Sprintf("k%d", i)), u("c"), u("h"))
	}
	g.Add(ctx, u("y"), u("r"), u("k3"))
	g.Add(ctx, u("y"), u("r"), u("k7"))
	for name, tc := range map[string]struct {
		p   Path
		obj term.Term
	}{
		"type/sub*":  {Sequence(AsPath(typ), ZeroOrMore(AsPath(sub))), u("C")},
		"type/sub+":  {Sequence(AsPath(typ), OneOrMore(AsPath(sub))), u("C")},
		"type/sub":   {Sequence(AsPath(typ), AsPath(sub)), u("C")},
		"a/b twice":  {Sequence(AsPath(u("a")), AsPath(u("b"))), u("z")},
		"three":      {Sequence(AsPath(u("a")), AsPath(u("b")), Inv(AsPath(u("b")))), u("m1")},
		"rare first": {Sequence(AsPath(u("r")), AsPath(u("c"))), u("h")},
	} {
		t.Run(name, func(t *testing.T) {
			var all, bound []string
			for pr, err := range tc.p.Eval(ctx, g, nil, nil) {
				if err != nil {
					t.Fatal(err)
				}
				if pr.Second.N3() == tc.obj.N3() {
					all = append(all, pr.First.N3()+" "+pr.Second.N3())
				}
			}
			for pr, err := range tc.p.Eval(ctx, g, nil, tc.obj) {
				if err != nil {
					t.Fatal(err)
				}
				bound = append(bound, pr.First.N3()+" "+pr.Second.N3())
			}
			slices.Sort(all)
			slices.Sort(bound)
			if len(all) == 0 {
				t.Fatal("no pair: the test checks nothing")
			}
			if !slices.Equal(all, bound) {
				t.Errorf("object-bound pairs %v, want %v", bound, all)
			}
		})
	}
}

// readCountingStore counts the triples its iterators yield.
type readCountingStore struct {
	*store.MemoryStore
	reads int
}

func (s *readCountingStore) Triples(ctx context.Context, p term.TriplePattern, g term.Term) store.TripleIterator {
	return func(yield func(term.Triple, error) bool) {
		for t, err := range s.MemoryStore.Triples(ctx, p, g) {
			s.reads++
			if !yield(t, err) {
				return
			}
		}
	}
}

// The object-bound walk reads the smaller side: the instances of one class
// rather than every typed node, and the two triples of a rare first step
// rather than every triple into a shared object.
func TestSequenceWithBoundObjectReadsTheSmallerSide(t *testing.T) {
	ctx := t.Context()
	s := &readCountingStore{MemoryStore: store.NewMemoryStore()}
	g := graph.NewGraph(graph.WithStore(s))
	u := func(s string) term.URIRef { return term.NewURIRefUnsafe("urn:" + s) }
	typ, sub := u("type"), u("sub")
	for c := range 10 {
		g.Add(ctx, u(fmt.Sprintf("C%d", c)), sub, u("Top"))
	}
	for i := range 1000 {
		g.Add(ctx, u(fmt.Sprintf("x%d", i)), typ, u(fmt.Sprintf("C%d", i%10)))
		g.Add(ctx, u(fmt.Sprintf("k%d", i)), u("c"), u("h"))
	}
	g.Add(ctx, u("y"), u("r"), u("k1"))
	g.Add(ctx, u("y"), u("r"), u("k2"))
	for _, tc := range []struct {
		name     string
		p        Path
		obj      term.Term
		pairs    int
		maxReads int
	}{
		{"type/sub* one class", Sequence(AsPath(typ), ZeroOrMore(AsPath(sub))), u("C3"), 100, 200},
		{"rare first step", Sequence(AsPath(u("r")), AsPath(u("c"))), u("h"), 2, 10},
	} {
		s.reads = 0
		n := 0
		for _, err := range tc.p.Eval(ctx, g, nil, tc.obj) {
			if err != nil {
				t.Fatal(err)
			}
			n++
		}
		if n != tc.pairs {
			t.Errorf("%s: %d pairs, want %d", tc.name, n, tc.pairs)
		}
		if s.reads > tc.maxReads {
			t.Errorf("%s: read %d triples, want at most %d", tc.name, s.reads, tc.maxReads)
		}
	}
}

// failingCardinalityStore fails every Cardinality call.
type failingCardinalityStore struct {
	*store.MemoryStore
}

var _ store.CardinalityStore = (*failingCardinalityStore)(nil)

func (failingCardinalityStore) Cardinality(context.Context, term.TriplePattern, term.Term) (int, error) {
	return 0, errors.New("connection reset")
}

// A store error while choosing the direction reaches the caller.
func TestSequenceWithBoundObjectReportsACardinalityError(t *testing.T) {
	ctx := t.Context()
	g := graph.NewGraph(graph.WithStore(&failingCardinalityStore{store.NewMemoryStore()}))
	u := func(s string) term.URIRef { return term.NewURIRefUnsafe("urn:" + s) }
	g.Add(ctx, u("y"), u("a"), u("m"))
	g.Add(ctx, u("m"), u("b"), u("z"))
	var got error
	for _, err := range Sequence(AsPath(u("a")), AsPath(u("b"))).Eval(ctx, g, nil, u("z")) {
		if err != nil {
			got = err
		}
	}
	if got == nil || got.Error() != "connection reset" {
		t.Fatalf("got error %v, want the store's", got)
	}
}
