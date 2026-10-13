package graph_test

import (
	"errors"
	"iter"
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/store/storetest"
	"github.com/tggo/goRDFlib/term"
)

// firstErr drains seq and returns its error, if any.
func firstErr[T any](seq iter.Seq2[T, error]) error {
	for _, err := range seq {
		if err != nil {
			return err
		}
	}
	return nil
}

// Issue #52: before v0.9 a store had no way to report a failure, so a graph
// over a store that could not be reached looked empty. Every Graph method
// that reaches the store must hand the store's error to its caller.
func TestGraphReportsStoreErrors(t *testing.T) {
	ctx := t.Context()
	s := term.NewURIRefUnsafe("http://example.org/s")
	p := term.NewURIRefUnsafe("http://example.org/p")
	o := term.NewURIRefUnsafe("http://example.org/o")

	fs := storetest.NewFailingStore(store.NewMemoryStore())
	g := graph.NewGraph(graph.WithStore(fs))
	if err := g.Add(ctx, s, p, o); err != nil {
		t.Fatal(err)
	}
	fs.Fail(nil)

	is := func(what string, err error) {
		t.Helper()
		if !errors.Is(err, storetest.ErrInjected) {
			t.Errorf("%s: err = %v, want the store's error", what, err)
		}
	}
	is("Add", g.Add(ctx, s, p, o))
	is("Remove", g.Remove(ctx, s, &p, o))
	is("Set", g.Set(ctx, s, p, o))
	_, err := g.Len(ctx)
	is("Len", err)
	_, err = g.Contains(ctx, s, p, o)
	is("Contains", err)
	is("Triples", firstErr(g.Triples(ctx, nil, nil, nil)))
	is("Subjects", firstErr(g.Subjects(ctx, nil, nil)))
	is("Predicates", firstErr(g.Predicates(ctx, nil, nil)))
	is("Objects", firstErr(g.Objects(ctx, nil, nil)))
	is("SubjectPredicates", firstErr(g.SubjectPredicates(ctx, nil)))
	is("SubjectObjects", firstErr(g.SubjectObjects(ctx, nil)))
	is("PredicateObjects", firstErr(g.PredicateObjects(ctx, nil)))
	_, _, err = g.Value(ctx, s, &p, nil)
	is("Value", err)
	is("Bind", g.Bind(ctx, "ex", p))
	is("Namespaces", firstErr(g.Namespaces(ctx)))
	_, err = g.QName(ctx, p.Value())
	is("QName", err)
	_, err = g.Union(ctx, graph.NewGraph())
	is("Union", err)
	_, err = g.Intersection(ctx, graph.NewGraph())
	is("Intersection", err)
	_, err = g.Difference(ctx, graph.NewGraph())
	is("Difference", err)
	_, err = g.AllNodes(ctx)
	is("AllNodes", err)
	_, err = g.Connected(ctx)
	is("Connected", err)
	_, err = g.Skolemize(ctx, "http://example.org/")
	is("Skolemize", err)
	_, err = g.DeSkolemize(ctx, "http://example.org/")
	is("DeSkolemize", err)

	c := graph.NewCollection(g, s)
	is("Collection.Append", c.Append(ctx, o))
	_, err = c.Len(ctx)
	is("Collection.Len", err)
	is("Collection.Iter", firstErr(c.Iter(ctx)))
	is("Collection.Clear", c.Clear(ctx))

	cg := graph.NewConjunctiveGraph(graph.WithStore(fs))
	_, err = cg.Len(ctx)
	is("ConjunctiveGraph.Len", err)
	is("ConjunctiveGraph.Triples", firstErr(cg.Triples(ctx, nil, nil, nil)))
	is("ConjunctiveGraph.Quads", firstErr(cg.Quads(ctx, nil, nil, nil)))
	is("ConjunctiveGraph.Remove", cg.Remove(ctx, nil, nil, nil, nil))
}

// A read that fails midway reports the error after the triples it did read;
// a consumer that stops at the first error never mistakes the prefix for the
// whole graph.
func TestGraphReadFailsMidway(t *testing.T) {
	ctx := t.Context()
	fs := storetest.NewFailingStore(store.NewMemoryStore())
	g := graph.NewGraph(graph.WithStore(fs))
	p := term.NewURIRefUnsafe("http://example.org/p")
	for _, n := range []string{"a", "b", "c", "d"} {
		if err := g.Add(ctx, term.NewURIRefUnsafe("http://example.org/"+n), p, term.NewLiteral(n)); err != nil {
			t.Fatal(err)
		}
	}
	fs.FailReadsAfter(2, nil)
	got := 0
	var gotErr error
	for _, err := range g.Triples(ctx, nil, nil, nil) {
		if err != nil {
			gotErr = err
			break
		}
		got++
	}
	if got != 2 || !errors.Is(gotErr, storetest.ErrInjected) {
		t.Fatalf("read %d triples, then %v; want 2 and the store's error", got, gotErr)
	}
}
