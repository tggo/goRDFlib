package storetest

import (
	"context"
	"errors"
	"testing"

	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// testDoneContext pins the error half of the contract (issue #52): a call
// made with a context that is already done returns that context's error and
// changes nothing. Before v0.9 a store had no way to say a call failed, so a
// backend that could not reach its data answered as if the graph were empty.
func testDoneContext(t *testing.T, cfg Config) {
	ns := "http://example.org/ns/"
	t1 := term.Triple{Subject: Alice, Predicate: Knows, Object: Bob}
	t2 := term.Triple{Subject: Bob, Predicate: Knows, Object: Carol}
	t3 := term.Triple{Subject: Carol, Predicate: Knows, Object: Dave}
	done, cancel := context.WithCancel(context.Background())
	cancel()
	isDone := func(t *testing.T, what string, err error) {
		t.Helper()
		if !errors.Is(err, context.Canceled) {
			t.Errorf("%s with a cancelled context: err = %v, want context.Canceled", what, err)
		}
	}
	seed := func(t *testing.T) store.Store {
		s := cfg.New(t)
		if err := s.Add(t.Context(), t1, nil); err != nil {
			t.Fatal(err)
		}
		if err := s.Add(t.Context(), t2, Graph1); err != nil {
			t.Fatal(err)
		}
		if err := s.Bind(t.Context(), "ex", term.NewURIRefUnsafe(ns)); err != nil {
			t.Fatal(err)
		}
		return s
	}
	wantLen := func(t *testing.T, s store.Store, g term.Term, want int) {
		t.Helper()
		n, err := s.Len(t.Context(), g)
		if err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Errorf("Len(%v) = %d after a refused write, want %d", g, n, want)
		}
	}
	firstErr := func(seq func(func(term.Triple, error) bool)) error {
		var got error
		seq(func(_ term.Triple, err error) bool {
			got = err
			return false
		})
		return got
	}

	cfg.run(t, "writes are refused and change nothing", func(t *testing.T) {
		s := seed(t)
		isDone(t, "Add", s.Add(done, t3, nil))
		isDone(t, "AddN", s.AddN(done, []term.Quad{{Triple: t3, Graph: Graph1}}))
		isDone(t, "Remove", s.Remove(done, term.TriplePattern{}, nil))
		isDone(t, "Set", s.Set(done, term.Triple{Subject: t1.Subject, Predicate: t1.Predicate, Object: t3.Object}, nil))
		isDone(t, "Bind", s.Bind(done, "other", term.NewURIRefUnsafe(ns+"other/")))
		wantLen(t, s, nil, 1)
		wantLen(t, s, Graph1, 1)
		if _, ok, _ := s.Namespace(t.Context(), "other"); ok {
			t.Error("Bind with a cancelled context bound the prefix")
		}
	})

	cfg.run(t, "reads report the error", func(t *testing.T) {
		s := seed(t)
		_, err := s.Len(done, nil)
		isDone(t, "Len", err)
		isDone(t, "Triples", firstErr(s.Triples(done, term.TriplePattern{}, nil)))
		var cerr error
		s.Contexts(done, nil)(func(_ term.Term, err error) bool { cerr = err; return false })
		isDone(t, "Contexts", cerr)
		_, _, err = s.Namespace(done, "ex")
		isDone(t, "Namespace", err)
		_, _, err = s.Prefix(done, term.NewURIRefUnsafe(ns))
		isDone(t, "Prefix", err)
		var nerr error
		s.Namespaces(done)(func(_ store.NamespaceBinding, err error) bool { nerr = err; return false })
		isDone(t, "Namespaces", nerr)

		if q, ok := s.(store.QueryableStore); ok {
			isDone(t, "TriplesWithLimit", firstErr(q.TriplesWithLimit(done, term.TriplePattern{}, nil, 1, 0)))
			_, err := q.Count(done, term.TriplePattern{}, nil)
			isDone(t, "Count", err)
			_, err = q.Exists(done, term.TriplePattern{}, nil)
			isDone(t, "Exists", err)
		}
		if c, ok := s.(store.CardinalityStore); ok {
			_, err := c.Cardinality(done, term.TriplePattern{}, nil)
			isDone(t, "Cardinality", err)
		}
		if r, ok := s.(store.ReachabilityStore); ok {
			_, err := r.Reachable(done, store.ReachabilityQuery{Start: t1.Subject})
			isDone(t, "Reachable", err)
		}
		if sn, ok := s.(store.SnapshotStore); ok {
			_, release, err := sn.ReadSnapshot(done)
			isDone(t, "ReadSnapshot", err)
			if release != nil {
				release()
			}
		}
	})
}
