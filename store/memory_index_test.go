package store

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/tggo/goRDFlib/term"
)

// TestMemoryIndexAgreesWithSet runs random adds and removes against a plain
// set and compares every pattern shape after each step. The term alphabets
// are small enough that levels grow past smallMapMax, turn into Go maps and
// shrink back, so every singleton/leaf/level transition is exercised.
func TestMemoryIndexAgreesWithSet(t *testing.T) {
	ctx := context.Background()
	r := rand.New(rand.NewPCG(1, 53))
	subj := make([]term.URIRef, 12)
	pred := make([]term.URIRef, 3)
	obj := make([]term.Term, 12)
	for i := range pred {
		pred[i] = term.NewURIRefUnsafe(fmt.Sprintf("urn:p%d", i))
	}
	for i := range subj {
		subj[i] = term.NewURIRefUnsafe(fmt.Sprintf("urn:s%d", i))
		obj[i] = term.NewLiteral(fmt.Sprintf("o%d", i))
	}
	obj[0], obj[1] = subj[0], subj[1] // objects that are also subjects
	obj[2] = term.NewBNode("b")
	obj[3] = term.NewLiteral("o3", term.WithLang("en"))
	obj[4] = term.NewLiteral(4)
	obj[5] = term.NewTripleTerm(subj[2], pred[0], subj[3])

	s := NewMemoryStore()
	want := map[string]term.Triple{}
	key := func(tr term.Triple) string {
		return term.TermKey(tr.Subject) + "\x00" + term.TermKey(tr.Predicate) + "\x00" + term.TermKey(tr.Object)
	}
	random := func() term.Triple {
		return term.Triple{Subject: subj[r.IntN(len(subj))], Predicate: pred[r.IntN(len(pred))], Object: obj[r.IntN(len(obj))]}
	}

	for step := range 4000 {
		tr := random()
		// Bias towards adding early and removing late, so the store both
		// fills up and drains.
		if r.IntN(4000) > step {
			if err := s.Add(ctx, tr, nil); err != nil {
				t.Fatal(err)
			}
			want[key(tr)] = tr
		} else {
			if err := s.Remove(ctx, term.TriplePattern{Subject: tr.Subject, Predicate: &tr.Predicate, Object: tr.Object}, nil); err != nil {
				t.Fatal(err)
			}
			delete(want, key(tr))
		}
		if step%37 != 0 {
			continue
		}
		probe := random()
		for mask := range 8 {
			var pat term.TriplePattern
			if mask&1 != 0 {
				pat.Subject = probe.Subject
			}
			if mask&2 != 0 {
				pat.Predicate = &probe.Predicate
			}
			if mask&4 != 0 {
				pat.Object = probe.Object
			}
			exp := 0
			for _, w := range want {
				if (pat.Subject == nil || term.TermKey(w.Subject) == term.TermKey(pat.Subject)) &&
					(pat.Predicate == nil || w.Predicate == *pat.Predicate) &&
					(pat.Object == nil || term.TermKey(w.Object) == term.TermKey(pat.Object)) {
					exp++
				}
			}
			got := map[string]bool{}
			for tr, err := range s.Triples(ctx, pat, nil) {
				if err != nil {
					t.Fatal(err)
				}
				if _, ok := want[key(tr)]; !ok || got[key(tr)] {
					t.Fatalf("step %d mask %d: unexpected or repeated %v", step, mask, tr)
				}
				got[key(tr)] = true
			}
			if len(got) != exp {
				t.Fatalf("step %d mask %d: %d triples, want %d", step, mask, len(got), exp)
			}
			if c := s.def.cardinality(pat); c != exp {
				t.Fatalf("step %d mask %d: cardinality %d, want %d", step, mask, c, exp)
			}
		}
		for _, inverse := range []bool{false, true} {
			q := ReachabilityQuery{Start: probe.Subject, Predicates: []term.URIRef{probe.Predicate}, Inverse: inverse}
			got, err := s.Reachable(ctx, q)
			if err != nil {
				t.Fatal(err)
			}
			if exp := naiveReachable(want, q); !sameKeys(got, exp) {
				t.Fatalf("step %d inverse %v: Reachable %v, want %v", step, inverse, got, exp)
			}
		}
	}
	if n, _ := s.Len(ctx, nil); n != len(want) {
		t.Fatalf("Len %d, want %d", n, len(want))
	}
}

// naiveReachable is a breadth-first search over the model set.
func naiveReachable(set map[string]term.Triple, q ReachabilityQuery) map[string]bool {
	out := map[string]bool{}
	frontier := []string{term.TermKey(q.Start)}
	seen := map[string]bool{frontier[0]: true}
	for len(frontier) > 0 {
		var next []string
		for _, n := range frontier {
			for _, tr := range set {
				if tr.Predicate != q.Predicates[0] {
					continue
				}
				from, to := term.TermKey(tr.Subject), term.TermKey(tr.Object)
				if q.Inverse {
					from, to = to, from
				}
				if from != n {
					continue
				}
				out[to] = true
				if !seen[to] {
					seen[to] = true
					next = append(next, to)
				}
			}
		}
		frontier = next
	}
	return out
}

func sameKeys(got []term.Term, want map[string]bool) bool {
	if len(got) != len(want) {
		return false
	}
	for _, g := range got {
		if !want[term.TermKey(g)] {
			return false
		}
	}
	return true
}
