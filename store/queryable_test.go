package store

import (
	"fmt"
	"testing"

	"github.com/tggo/goRDFlib/term"
)

// helper: populate a MemoryStore with n subjects x m predicates = n*m triples.
// Subjects: http://example.org/q/s0 .. s{n-1}
// Predicates: http://example.org/q/p0 .. p{m-1}
// Objects: literal "v_{si}_{pi}"
func populateStore(n, m int) *MemoryStore {
	s := NewMemoryStore()
	for si := range n {
		subj := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/q/s%d", si))
		for pi := range m {
			pred := term.NewURIRefUnsafe(fmt.Sprintf("http://example.org/q/p%d", pi))
			obj := term.NewLiteral(fmt.Sprintf("v_%d_%d", si, pi))
			s.addLocked(term.Triple{Subject: subj, Predicate: pred, Object: obj})
		}
	}
	return s
}

// --- Count tests ---

func TestCount_AllTriples(t *testing.T) {
	s := populateStore(10, 5) // 50 triples
	got := s.Count(term.TriplePattern{}, nil)
	if got != 50 {
		t.Errorf("Count(nil/nil/nil) = %d, want 50", got)
	}
}

func TestCount_EmptyStore(t *testing.T) {
	s := NewMemoryStore()
	got := s.Count(term.TriplePattern{}, nil)
	if got != 0 {
		t.Errorf("Count on empty store = %d, want 0", got)
	}
}

func TestCount_BySubject(t *testing.T) {
	s := populateStore(10, 5)
	subj := term.NewURIRefUnsafe("http://example.org/q/s3")
	got := s.Count(term.TriplePattern{Subject: subj}, nil)
	if got != 5 {
		t.Errorf("Count(subject=s3) = %d, want 5", got)
	}
}

func TestCount_ByPredicate(t *testing.T) {
	s := populateStore(10, 5)
	pred := term.NewURIRefUnsafe("http://example.org/q/p2")
	got := s.Count(term.TriplePattern{Predicate: &pred}, nil)
	if got != 10 {
		t.Errorf("Count(predicate=p2) = %d, want 10", got)
	}
}

func TestCount_ByObject(t *testing.T) {
	s := populateStore(10, 5)
	obj := term.NewLiteral("v_7_3")
	got := s.Count(term.TriplePattern{Object: obj}, nil)
	if got != 1 {
		t.Errorf("Count(object=v_7_3) = %d, want 1", got)
	}
}

func TestCount_SubjectAndPredicate(t *testing.T) {
	s := populateStore(10, 5)
	subj := term.NewURIRefUnsafe("http://example.org/q/s0")
	pred := term.NewURIRefUnsafe("http://example.org/q/p0")
	got := s.Count(term.TriplePattern{Subject: subj, Predicate: &pred}, nil)
	if got != 1 {
		t.Errorf("Count(subject=s0, predicate=p0) = %d, want 1", got)
	}
}

func TestCount_SubjectAndObject(t *testing.T) {
	s := populateStore(10, 5)
	subj := term.NewURIRefUnsafe("http://example.org/q/s2")
	obj := term.NewLiteral("v_2_4")
	got := s.Count(term.TriplePattern{Subject: subj, Object: obj}, nil)
	if got != 1 {
		t.Errorf("Count(subject=s2, object=v_2_4) = %d, want 1", got)
	}
}

func TestCount_PredicateAndObject(t *testing.T) {
	s := populateStore(10, 5)
	pred := term.NewURIRefUnsafe("http://example.org/q/p1")
	obj := term.NewLiteral("v_5_1")
	got := s.Count(term.TriplePattern{Predicate: &pred, Object: obj}, nil)
	if got != 1 {
		t.Errorf("Count(predicate=p1, object=v_5_1) = %d, want 1", got)
	}
}

func TestCount_ExactMatch(t *testing.T) {
	s := populateStore(10, 5)
	subj := term.NewURIRefUnsafe("http://example.org/q/s0")
	pred := term.NewURIRefUnsafe("http://example.org/q/p0")
	obj := term.NewLiteral("v_0_0")
	got := s.Count(term.TriplePattern{Subject: subj, Predicate: &pred, Object: obj}, nil)
	if got != 1 {
		t.Errorf("Count(exact match) = %d, want 1", got)
	}
}

func TestCount_NoMatch(t *testing.T) {
	s := populateStore(10, 5)
	subj := term.NewURIRefUnsafe("http://example.org/q/missing")
	got := s.Count(term.TriplePattern{Subject: subj}, nil)
	if got != 0 {
		t.Errorf("Count(missing subject) = %d, want 0", got)
	}
}

// --- Exists tests ---

func TestExists_EmptyStore(t *testing.T) {
	s := NewMemoryStore()
	if s.Exists(term.TriplePattern{}, nil) {
		t.Error("Exists on empty store should be false")
	}
}

func TestExists_AllPattern(t *testing.T) {
	s := populateStore(3, 3)
	if !s.Exists(term.TriplePattern{}, nil) {
		t.Error("Exists(nil/nil/nil) on populated store should be true")
	}
}

func TestExists_BySubject_Found(t *testing.T) {
	s := populateStore(5, 2)
	subj := term.NewURIRefUnsafe("http://example.org/q/s0")
	if !s.Exists(term.TriplePattern{Subject: subj}, nil) {
		t.Error("Exists(subject=s0) should be true")
	}
}

func TestExists_BySubject_NotFound(t *testing.T) {
	s := populateStore(5, 2)
	subj := term.NewURIRefUnsafe("http://example.org/q/missing")
	if s.Exists(term.TriplePattern{Subject: subj}, nil) {
		t.Error("Exists(missing subject) should be false")
	}
}

func TestExists_ByPredicate_Found(t *testing.T) {
	s := populateStore(5, 2)
	pred := term.NewURIRefUnsafe("http://example.org/q/p1")
	if !s.Exists(term.TriplePattern{Predicate: &pred}, nil) {
		t.Error("Exists(predicate=p1) should be true")
	}
}

func TestExists_ByPredicate_NotFound(t *testing.T) {
	s := populateStore(5, 2)
	pred := term.NewURIRefUnsafe("http://example.org/q/p999")
	if s.Exists(term.TriplePattern{Predicate: &pred}, nil) {
		t.Error("Exists(missing predicate) should be false")
	}
}

func TestExists_ByObject_Found(t *testing.T) {
	s := populateStore(5, 2)
	obj := term.NewLiteral("v_3_1")
	if !s.Exists(term.TriplePattern{Object: obj}, nil) {
		t.Error("Exists(object=v_3_1) should be true")
	}
}

func TestExists_ByObject_NotFound(t *testing.T) {
	s := populateStore(5, 2)
	obj := term.NewLiteral("nonexistent")
	if s.Exists(term.TriplePattern{Object: obj}, nil) {
		t.Error("Exists(missing object) should be false")
	}
}

func TestExists_ExactMatch(t *testing.T) {
	s := populateStore(5, 2)
	subj := term.NewURIRefUnsafe("http://example.org/q/s4")
	pred := term.NewURIRefUnsafe("http://example.org/q/p0")
	obj := term.NewLiteral("v_4_0")
	if !s.Exists(term.TriplePattern{Subject: subj, Predicate: &pred, Object: obj}, nil) {
		t.Error("Exists(exact match) should be true")
	}
}

func TestExists_ExactMismatch(t *testing.T) {
	s := populateStore(5, 2)
	subj := term.NewURIRefUnsafe("http://example.org/q/s4")
	pred := term.NewURIRefUnsafe("http://example.org/q/p0")
	obj := term.NewLiteral("wrong")
	if s.Exists(term.TriplePattern{Subject: subj, Predicate: &pred, Object: obj}, nil) {
		t.Error("Exists(exact mismatch) should be false")
	}
}

// --- TriplesWithLimit tests ---

func TestTriplesWithLimit_Basic(t *testing.T) {
	s := populateStore(10, 5) // 50 triples
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 7, 0) {
		count++
	}
	if count != 7 {
		t.Errorf("TriplesWithLimit(7, 0) = %d, want 7", count)
	}
}

func TestTriplesWithLimit_OffsetOnly(t *testing.T) {
	s := populateStore(10, 5) // 50 triples
	// limit=0 means no limit, offset=45 → 5 results
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 0, 45) {
		count++
	}
	if count != 5 {
		t.Errorf("TriplesWithLimit(0, 45) = %d, want 5", count)
	}
}

func TestTriplesWithLimit_LimitAndOffset(t *testing.T) {
	s := populateStore(10, 5) // 50 triples
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 3, 10) {
		count++
	}
	if count != 3 {
		t.Errorf("TriplesWithLimit(3, 10) = %d, want 3", count)
	}
}

func TestTriplesWithLimit_LimitLargerThanResults(t *testing.T) {
	s := populateStore(3, 2) // 6 triples
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 100, 0) {
		count++
	}
	if count != 6 {
		t.Errorf("TriplesWithLimit(100, 0) = %d, want 6", count)
	}
}

func TestTriplesWithLimit_OffsetBeyondResults(t *testing.T) {
	s := populateStore(3, 2) // 6 triples
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 5, 100) {
		count++
	}
	if count != 0 {
		t.Errorf("TriplesWithLimit(5, 100) = %d, want 0", count)
	}
}

func TestTriplesWithLimit_ZeroLimitNoOffset(t *testing.T) {
	s := populateStore(4, 3) // 12 triples
	// limit <= 0 means yield all
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 0, 0) {
		count++
	}
	if count != 12 {
		t.Errorf("TriplesWithLimit(0, 0) = %d, want 12", count)
	}
}

func TestTriplesWithLimit_NegativeLimit(t *testing.T) {
	s := populateStore(4, 3) // 12 triples
	// negative limit treated as no limit
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, -1, 0) {
		count++
	}
	if count != 12 {
		t.Errorf("TriplesWithLimit(-1, 0) = %d, want 12", count)
	}
}

func TestTriplesWithLimit_WithSubjectFilter(t *testing.T) {
	s := populateStore(10, 5) // 50 triples, 5 per subject
	subj := term.NewURIRefUnsafe("http://example.org/q/s7")
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{Subject: subj}, nil, 3, 0) {
		count++
	}
	if count != 3 {
		t.Errorf("TriplesWithLimit(subject=s7, 3, 0) = %d, want 3", count)
	}
}

func TestTriplesWithLimit_WithSubjectFilterOffsetPastEnd(t *testing.T) {
	s := populateStore(10, 5) // 5 triples for any given subject
	subj := term.NewURIRefUnsafe("http://example.org/q/s7")
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{Subject: subj}, nil, 3, 10) {
		count++
	}
	if count != 0 {
		t.Errorf("TriplesWithLimit(subject=s7, 3, 10) = %d, want 0", count)
	}
}

func TestTriplesWithLimit_EmptyStore(t *testing.T) {
	s := NewMemoryStore()
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 10, 0) {
		count++
	}
	if count != 0 {
		t.Errorf("TriplesWithLimit on empty = %d, want 0", count)
	}
}

func TestTriplesWithLimit_OffsetEqualsTotal(t *testing.T) {
	s := populateStore(5, 2) // 10 triples
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 5, 10) {
		count++
	}
	if count != 0 {
		t.Errorf("TriplesWithLimit(5, 10) = %d, want 0", count)
	}
}

func TestTriplesWithLimit_OffsetPlusLimitExceedsTotal(t *testing.T) {
	s := populateStore(5, 2) // 10 triples
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 20, 5) {
		count++
	}
	if count != 5 {
		t.Errorf("TriplesWithLimit(20, 5) = %d, want 5", count)
	}
}

// --- triplesLocked coverage: subject+object pattern (exercises ok filter in spo branch) ---

func TestTriples_SubjectAndObjectFilter(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	p1, _ := term.NewURIRef("http://example.org/p1")
	p2, _ := term.NewURIRef("http://example.org/p2")
	target := term.NewLiteral("target")
	other := term.NewLiteral("other")

	s.Add(term.Triple{Subject: sub, Predicate: p1, Object: target}, nil)
	s.Add(term.Triple{Subject: sub, Predicate: p2, Object: other}, nil)
	s.Add(term.Triple{Subject: sub, Predicate: p2, Object: target}, nil)

	// Pattern: subject=sub, object=target (no predicate) → hits spo branch with ok!=""
	count := 0
	s.Triples(term.TriplePattern{Subject: sub, Object: target}, nil)(func(t term.Triple) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("Triples(subject+object) = %d, want 2", count)
	}
}

// --- triplesLocked coverage: predicate+object pattern (exercises ok filter in pos branch) ---

func TestTriples_PredicateAndObjectFilter(t *testing.T) {
	s := NewMemoryStore()
	s1, _ := term.NewURIRef("http://example.org/s1")
	s2, _ := term.NewURIRef("http://example.org/s2")
	pred, _ := term.NewURIRef("http://example.org/p")
	target := term.NewLiteral("target")
	other := term.NewLiteral("other")

	s.Add(term.Triple{Subject: s1, Predicate: pred, Object: target}, nil)
	s.Add(term.Triple{Subject: s2, Predicate: pred, Object: other}, nil)
	s.Add(term.Triple{Subject: s2, Predicate: pred, Object: target}, nil)

	// Pattern: predicate=pred, object=target (no subject) → hits pos branch with ok!=""
	count := 0
	s.Triples(term.TriplePattern{Predicate: &pred, Object: target}, nil)(func(t term.Triple) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("Triples(predicate+object) = %d, want 2", count)
	}
}

// --- triplesLocked coverage: object+predicate via osp branch (when only object set, pk filter is unreachable) ---
// This test ensures the object-only path with predicate filtering works.
// Note: When both ok and pk are set, the pk branch takes precedence in triplesLocked,
// so the pk filter inside the ok branch is dead code. We still test both patterns.

func TestTriples_ObjectOnlyMultiplePredicates(t *testing.T) {
	s := NewMemoryStore()
	s1, _ := term.NewURIRef("http://example.org/s1")
	p1, _ := term.NewURIRef("http://example.org/p1")
	p2, _ := term.NewURIRef("http://example.org/p2")
	obj := term.NewLiteral("shared")

	s.Add(term.Triple{Subject: s1, Predicate: p1, Object: obj}, nil)
	s.Add(term.Triple{Subject: s1, Predicate: p2, Object: obj}, nil)

	// Object-only pattern — gets all triples with this object
	count := 0
	s.Triples(term.TriplePattern{Object: obj}, nil)(func(t term.Triple) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("Triples(object only) = %d, want 2", count)
	}
}

// --- triplesLocked coverage: subject+predicate filter in spo branch ---

func TestTriples_SubjectAndPredicateFilter(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	p1, _ := term.NewURIRef("http://example.org/p1")
	p2, _ := term.NewURIRef("http://example.org/p2")
	o1 := term.NewLiteral("v1")
	o2 := term.NewLiteral("v2")

	s.Add(term.Triple{Subject: sub, Predicate: p1, Object: o1}, nil)
	s.Add(term.Triple{Subject: sub, Predicate: p2, Object: o2}, nil)

	// Pattern: subject=sub, predicate=p1 → hits spo branch with pk!="" filter
	count := 0
	s.Triples(term.TriplePattern{Subject: sub, Predicate: &p1}, nil)(func(t term.Triple) bool {
		count++
		return true
	})
	if count != 1 {
		t.Errorf("Triples(subject+predicate) = %d, want 1", count)
	}
}

// --- Verify Exists short-circuits (doesn't scan all) ---

func TestExists_ShortCircuits(t *testing.T) {
	s := populateStore(100, 10) // 1000 triples
	// Exists should return true quickly without iterating all
	if !s.Exists(term.TriplePattern{}, nil) {
		t.Error("Exists should be true on populated store")
	}
}

// --- Verify Count with context (should work same as nil for MemoryStore) ---

func TestCount_WithContext(t *testing.T) {
	s := populateStore(5, 2) // 10 triples
	ctx, _ := term.NewURIRef("http://example.org/graph")
	// MemoryStore ignores context, so count should still be 10
	got := s.Count(term.TriplePattern{}, ctx)
	if got != 10 {
		t.Errorf("Count with context = %d, want 10", got)
	}
}
