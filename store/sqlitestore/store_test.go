package sqlitestore

import (
	"sync"
	"testing"

	"github.com/tggo/goRDFlib/plugin"
	"github.com/tggo/goRDFlib/term"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

var (
	alice  = term.NewURIRefUnsafe("http://example.org/Alice")
	bob    = term.NewURIRefUnsafe("http://example.org/Bob")
	carol  = term.NewURIRefUnsafe("http://example.org/Carol")
	name   = term.NewURIRefUnsafe("http://example.org/name")
	age    = term.NewURIRefUnsafe("http://example.org/age")
	knows  = term.NewURIRefUnsafe("http://example.org/knows")
	label  = term.NewURIRefUnsafe("http://example.org/label")
	graph1 = term.NewURIRefUnsafe("http://example.org/graph1")
	graph2 = term.NewURIRefUnsafe("http://example.org/graph2")
	graph3 = term.NewURIRefUnsafe("http://example.org/graph3")
)

// ---------------------------------------------------------------------------
// New / constructor
// ---------------------------------------------------------------------------

func TestNewNoOptions(t *testing.T) {
	_, err := New()
	if err == nil {
		t.Fatal("expected error when no options provided")
	}
}

func TestNewWithFile(t *testing.T) {
	path := t.TempDir() + "/test.db"
	s, err := New(WithFile(path))
	if err != nil {
		t.Fatalf("New(WithFile): %v", err)
	}
	s.Close()
}

func TestNewWithInMemory(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatalf("New(WithInMemory): %v", err)
	}
	s.Close()
}

// ---------------------------------------------------------------------------
// Add / Len
// ---------------------------------------------------------------------------

func TestAddAndLen(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}, nil)
	if got := s.Len(nil); got != 2 {
		t.Errorf("Len = %d, want 2", got)
	}
}

func TestDuplicateAdd(t *testing.T) {
	s := newTestStore(t)
	triple := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(triple, nil)
	s.Add(triple, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after duplicate = %d, want 1", got)
	}
}

func TestLenWithContext(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	t2 := term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}
	s.Add(t1, graph1)
	s.Add(t2, graph2)
	s.Add(t1, nil) // default graph

	if got := s.Len(graph1); got != 1 {
		t.Errorf("Len(graph1) = %d, want 1", got)
	}
	if got := s.Len(graph2); got != 1 {
		t.Errorf("Len(graph2) = %d, want 1", got)
	}
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len(nil) = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// AddN (batch)
// ---------------------------------------------------------------------------

func TestAddN(t *testing.T) {
	s := newTestStore(t)
	quads := []term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}},
		{Triple: term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}},
	}
	s.AddN(quads)
	if got := s.Len(nil); got != 2 {
		t.Errorf("Len = %d, want 2", got)
	}
}

func TestAddNEmpty(t *testing.T) {
	s := newTestStore(t)
	s.AddN(nil)
	s.AddN([]term.Quad{})
	if got := s.Len(nil); got != 0 {
		t.Errorf("Len = %d, want 0", got)
	}
}

func TestAddNWithGraphs(t *testing.T) {
	s := newTestStore(t)
	quads := []term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, Graph: graph1},
		{Triple: term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, Graph: graph2},
		{Triple: term.Triple{Subject: carol, Predicate: name, Object: term.NewLiteral("Carol")}},
	}
	s.AddN(quads)
	if got := s.Len(graph1); got != 1 {
		t.Errorf("Len(graph1) = %d, want 1", got)
	}
	if got := s.Len(graph2); got != 1 {
		t.Errorf("Len(graph2) = %d, want 1", got)
	}
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len(nil) = %d, want 1", got)
	}
}

func TestAddNDuplicates(t *testing.T) {
	s := newTestStore(t)
	triple := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	quads := []term.Quad{
		{Triple: triple},
		{Triple: triple},
		{Triple: triple},
	}
	s.AddN(quads)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len = %d, want 1 (duplicates should be ignored)", got)
	}
}

func TestAddNLargeBatch(t *testing.T) {
	s := newTestStore(t)
	quads := make([]term.Quad, 500)
	for i := range quads {
		subj := term.NewURIRefUnsafe("http://example.org/" + string(rune('A'+i%26)))
		quads[i] = term.Quad{Triple: term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}}
	}
	s.AddN(quads)
	// 26 unique subjects * many objects = up to 500 unique triples
	if got := s.Len(nil); got == 0 {
		t.Error("expected some triples after large batch AddN")
	}
}

// ---------------------------------------------------------------------------
// Remove
// ---------------------------------------------------------------------------

func TestRemove(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}, nil)
	s.Remove(term.TriplePattern{Subject: alice, Predicate: &name, Object: term.NewLiteral("Alice")}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after remove = %d, want 1", got)
	}
}

func TestRemoveWildcard(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)
	s.Remove(term.TriplePattern{Subject: alice}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after wildcard remove = %d, want 1", got)
	}
}

func TestRemoveFromNamedGraph(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(t1, graph1)
	s.Add(t1, nil)
	s.Remove(term.TriplePattern{Subject: alice}, graph1)
	if got := s.Len(graph1); got != 0 {
		t.Errorf("Len(graph1) = %d, want 0", got)
	}
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len(default) = %d, want 1", got)
	}
}

func TestRemoveByPredicate(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}, nil)
	s.Remove(term.TriplePattern{Predicate: &name}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after remove by predicate = %d, want 1", got)
	}
}

func TestRemoveByObject(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: carol, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: carol}, nil)
	s.Remove(term.TriplePattern{Object: bob}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after remove by object = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// Set (replace)
// ---------------------------------------------------------------------------

func TestSet(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice B.")}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after Set = %d, want 1", got)
	}
	count := 0
	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: &name}, nil) {
		count++
		if lit, ok := tr.Object.(term.Literal); ok {
			if lit.Lexical() != "Alice B." {
				t.Errorf("Set value = %q, want %q", lit.Lexical(), "Alice B.")
			}
		}
	}
	if count != 1 {
		t.Errorf("Triples count = %d, want 1", count)
	}
}

func TestSetReplacesMultipleValues(t *testing.T) {
	s := newTestStore(t)
	// Add two triples with same subject+predicate, different objects.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice1")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice2")}, nil)
	if got := s.Len(nil); got != 2 {
		t.Fatalf("Len before Set = %d, want 2", got)
	}
	// Set should delete both and insert one.
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice3")}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after Set = %d, want 1", got)
	}
}

func TestSetInNamedGraph(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("New")}, graph1)
	// Only graph1 should be affected.
	if got := s.Len(graph1); got != 1 {
		t.Errorf("Len(graph1) = %d, want 1", got)
	}
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len(nil) = %d, want 1", got)
	}
}

func TestSetOnEmptyStore(t *testing.T) {
	s := newTestStore(t)
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after Set on empty = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// Triples (iteration, all patterns)
// ---------------------------------------------------------------------------

func TestTriplesAllPatterns(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)

	tests := []struct {
		name    string
		pattern term.TriplePattern
		want    int
	}{
		{"all", term.TriplePattern{}, 3},
		{"s", term.TriplePattern{Subject: alice}, 2},
		{"p", term.TriplePattern{Predicate: &name}, 2},
		{"o", term.TriplePattern{Object: bob}, 1},
		{"sp", term.TriplePattern{Subject: alice, Predicate: &name}, 1},
		{"so", term.TriplePattern{Subject: alice, Object: bob}, 1},
		{"po", term.TriplePattern{Predicate: &name, Object: term.NewLiteral("Alice")}, 1},
		{"spo", term.TriplePattern{Subject: alice, Predicate: &name, Object: term.NewLiteral("Alice")}, 1},
		{"no match", term.TriplePattern{Subject: bob, Predicate: &knows}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := 0
			for range s.Triples(tt.pattern, nil) {
				count++
			}
			if count != tt.want {
				t.Errorf("got %d, want %d", count, tt.want)
			}
		})
	}
}

func TestTriplesInNamedGraph(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	t2 := term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}
	s.Add(t1, graph1)
	s.Add(t2, nil)

	count := 0
	for range s.Triples(term.TriplePattern{}, graph1) {
		count++
	}
	if count != 1 {
		t.Errorf("Triples(graph1) = %d, want 1", count)
	}
}

func TestTriplesEarlyBreak(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 10; i++ {
		subj := term.NewURIRefUnsafe("http://example.org/" + string(rune('A'+i)))
		s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
	}
	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
		if count >= 3 {
			break
		}
	}
	if count != 3 {
		t.Errorf("early break count = %d, want 3", count)
	}
}

// ---------------------------------------------------------------------------
// TriplesWithLimit
// ---------------------------------------------------------------------------

func TestTriplesWithLimitBasic(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 10; i++ {
		subj := term.NewURIRefUnsafe("http://example.org/" + string(rune('A'+i)))
		s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
	}
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 3, 0) {
		count++
	}
	if count != 3 {
		t.Errorf("TriplesWithLimit(3,0) = %d, want 3", count)
	}
}

func TestTriplesWithLimitOffset(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 10; i++ {
		subj := term.NewURIRefUnsafe("http://example.org/" + string(rune('A'+i)))
		s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
	}
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 5, 7) {
		count++
	}
	if count != 3 {
		t.Errorf("TriplesWithLimit(5,7) = %d, want 3", count)
	}
}

func TestTriplesWithLimitZero(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 0, 0) {
		count++
	}
	if count != 0 {
		t.Errorf("TriplesWithLimit(0,0) = %d, want 0", count)
	}
}

func TestTriplesWithLimitWithPattern(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral(i)}, nil)
	}
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)

	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{Subject: alice}, nil, 3, 0) {
		count++
	}
	if count != 3 {
		t.Errorf("TriplesWithLimit with subject filter = %d, want 3", count)
	}
}

func TestTriplesWithLimitInNamedGraph(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		subj := term.NewURIRefUnsafe("http://example.org/" + string(rune('A'+i)))
		s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, graph1)
	}
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, graph1, 2, 0) {
		count++
	}
	if count != 2 {
		t.Errorf("TriplesWithLimit in graph1 = %d, want 2", count)
	}
}

func TestTriplesWithLimitEarlyBreak(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 10; i++ {
		subj := term.NewURIRefUnsafe("http://example.org/" + string(rune('A'+i)))
		s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
	}
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 10, 0) {
		count++
		if count >= 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("early break count = %d, want 2", count)
	}
}

// ---------------------------------------------------------------------------
// Count
// ---------------------------------------------------------------------------

func TestCountEmpty(t *testing.T) {
	s := newTestStore(t)
	if got := s.Count(term.TriplePattern{}, nil); got != 0 {
		t.Errorf("Count on empty = %d, want 0", got)
	}
}

func TestCountAll(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)

	if got := s.Count(term.TriplePattern{}, nil); got != 3 {
		t.Errorf("Count(all) = %d, want 3", got)
	}
}

func TestCountWithSubject(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)

	if got := s.Count(term.TriplePattern{Subject: alice}, nil); got != 2 {
		t.Errorf("Count(s=alice) = %d, want 2", got)
	}
}

func TestCountWithPredicate(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)

	if got := s.Count(term.TriplePattern{Predicate: &name}, nil); got != 2 {
		t.Errorf("Count(p=name) = %d, want 2", got)
	}
}

func TestCountWithObject(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: carol, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: carol}, nil)

	if got := s.Count(term.TriplePattern{Object: bob}, nil); got != 2 {
		t.Errorf("Count(o=bob) = %d, want 2", got)
	}
}

func TestCountWithSPO(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: carol}, nil)

	if got := s.Count(term.TriplePattern{Subject: alice, Predicate: &knows, Object: bob}, nil); got != 1 {
		t.Errorf("Count(spo) = %d, want 1", got)
	}
}

func TestCountInNamedGraph(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, graph1)
	s.Add(term.Triple{Subject: carol, Predicate: name, Object: term.NewLiteral("Carol")}, nil)

	if got := s.Count(term.TriplePattern{}, graph1); got != 2 {
		t.Errorf("Count(graph1) = %d, want 2", got)
	}
	if got := s.Count(term.TriplePattern{}, nil); got != 1 {
		t.Errorf("Count(default) = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// Exists
// ---------------------------------------------------------------------------

func TestExistsEmpty(t *testing.T) {
	s := newTestStore(t)
	if s.Exists(term.TriplePattern{}, nil) {
		t.Error("Exists on empty store should be false")
	}
}

func TestExistsAll(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	if !s.Exists(term.TriplePattern{}, nil) {
		t.Error("Exists({}) should be true")
	}
}

func TestExistsWithSubject(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	if !s.Exists(term.TriplePattern{Subject: alice}, nil) {
		t.Error("Exists(s=alice) should be true")
	}
	if s.Exists(term.TriplePattern{Subject: bob}, nil) {
		t.Error("Exists(s=bob) should be false")
	}
}

func TestExistsWithPredicate(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	if !s.Exists(term.TriplePattern{Predicate: &name}, nil) {
		t.Error("Exists(p=name) should be true")
	}
	if s.Exists(term.TriplePattern{Predicate: &knows}, nil) {
		t.Error("Exists(p=knows) should be false")
	}
}

func TestExistsWithObject(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	if !s.Exists(term.TriplePattern{Object: bob}, nil) {
		t.Error("Exists(o=bob) should be true")
	}
	if s.Exists(term.TriplePattern{Object: carol}, nil) {
		t.Error("Exists(o=carol) should be false")
	}
}

func TestExistsWithSPO(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	if !s.Exists(term.TriplePattern{Subject: alice, Predicate: &knows, Object: bob}, nil) {
		t.Error("Exists(alice knows bob) should be true")
	}
	if s.Exists(term.TriplePattern{Subject: alice, Predicate: &knows, Object: carol}, nil) {
		t.Error("Exists(alice knows carol) should be false")
	}
}

func TestExistsInNamedGraph(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)
	if !s.Exists(term.TriplePattern{Subject: alice}, graph1) {
		t.Error("Exists in graph1 should be true")
	}
	if s.Exists(term.TriplePattern{Subject: alice}, nil) {
		t.Error("Exists in default should be false")
	}
}

// ---------------------------------------------------------------------------
// Contexts
// ---------------------------------------------------------------------------

func TestContextsEmpty(t *testing.T) {
	s := newTestStore(t)
	count := 0
	for range s.Contexts(nil) {
		count++
	}
	if count != 0 {
		t.Errorf("Contexts on empty = %d, want 0", count)
	}
}

func TestContextsMultiple(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, graph2)
	s.Add(term.Triple{Subject: carol, Predicate: name, Object: term.NewLiteral("Carol")}, graph3)

	count := 0
	for range s.Contexts(nil) {
		count++
	}
	if count != 3 {
		t.Errorf("Contexts count = %d, want 3", count)
	}
}

func TestContextsExcludesDefaultGraph(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, graph1)

	count := 0
	for range s.Contexts(nil) {
		count++
	}
	if count != 1 {
		t.Errorf("Contexts should exclude default graph: got %d, want 1", count)
	}
}

func TestContextsFilteredByTriple(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(t1, graph1)
	s.Add(t1, graph2)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, graph3)

	count := 0
	for range s.Contexts(&t1) {
		count++
	}
	if count != 2 {
		t.Errorf("Contexts(alice triple) = %d, want 2", count)
	}
}

func TestContextsEarlyBreak(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		g := term.NewURIRefUnsafe("http://example.org/g" + string(rune('0'+i)))
		s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, g)
	}
	count := 0
	for range s.Contexts(nil) {
		count++
		if count >= 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("early break count = %d, want 2", count)
	}
}

// ---------------------------------------------------------------------------
// Namespace bindings
// ---------------------------------------------------------------------------

func TestNamespaceBindings(t *testing.T) {
	s := newTestStore(t)
	ex := term.NewURIRefUnsafe("http://example.org/")
	foaf := term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/")
	s.Bind("ex", ex)
	s.Bind("foaf", foaf)

	ns, ok := s.Namespace("ex")
	if !ok || ns != ex {
		t.Errorf("Namespace(ex) = %v, %v", ns, ok)
	}
	prefix, ok := s.Prefix(foaf)
	if !ok || prefix != "foaf" {
		t.Errorf("Prefix(foaf) = %q, %v", prefix, ok)
	}
	_, ok = s.Namespace("nonexistent")
	if ok {
		t.Error("Namespace(nonexistent) should be false")
	}
}

func TestBindOverwrite(t *testing.T) {
	s := newTestStore(t)
	ex1 := term.NewURIRefUnsafe("http://example.org/v1/")
	ex2 := term.NewURIRefUnsafe("http://example.org/v2/")
	s.Bind("ex", ex1)
	s.Bind("ex", ex2)

	ns, ok := s.Namespace("ex")
	if !ok || ns != ex2 {
		t.Errorf("Namespace(ex) after overwrite = %v, %v; want %v", ns, ok, ex2)
	}
}

func TestPrefixNotFound(t *testing.T) {
	s := newTestStore(t)
	_, ok := s.Prefix(term.NewURIRefUnsafe("http://example.org/unknown/"))
	if ok {
		t.Error("Prefix for unknown namespace should be false")
	}
}

func TestNamespacesIteration(t *testing.T) {
	s := newTestStore(t)
	ex := term.NewURIRefUnsafe("http://example.org/")
	foaf := term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/")
	rdf := term.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#")
	s.Bind("ex", ex)
	s.Bind("foaf", foaf)
	s.Bind("rdf", rdf)

	found := make(map[string]string)
	for prefix, ns := range s.Namespaces() {
		found[prefix] = ns.Value()
	}
	if len(found) != 3 {
		t.Errorf("Namespaces count = %d, want 3", len(found))
	}
	if found["ex"] != "http://example.org/" {
		t.Errorf("expected ex namespace")
	}
	if found["foaf"] != "http://xmlns.com/foaf/0.1/" {
		t.Errorf("expected foaf namespace")
	}
}

func TestNamespacesEarlyBreak(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		p := string(rune('a' + i))
		s.Bind(p, term.NewURIRefUnsafe("http://example.org/"+p+"/"))
	}
	count := 0
	for range s.Namespaces() {
		count++
		if count >= 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("early break count = %d, want 2", count)
	}
}

// ---------------------------------------------------------------------------
// Named graphs / BNode context
// ---------------------------------------------------------------------------

func TestNamedGraphs(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(t1, graph1)
	s.Add(t1, graph2)
	s.Add(t1, nil)

	if got := s.Len(graph1); got != 1 {
		t.Errorf("Len(graph1) = %d, want 1", got)
	}
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len(default) = %d, want 1", got)
	}
}

func TestBNodeContextIgnored(t *testing.T) {
	s := newTestStore(t)
	bn := term.NewBNode("")
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, bn)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len(nil) with BNode ctx = %d, want 1", got)
	}
	// BNode context should also be treated as default graph for queries.
	if got := s.Len(bn); got != 1 {
		t.Errorf("Len(bnode) = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// decodeRow — various term types
// ---------------------------------------------------------------------------

func TestDecodeRowBNode(t *testing.T) {
	s := newTestStore(t)
	bn := term.NewBNode("test1")
	s.Add(term.Triple{Subject: bn, Predicate: name, Object: term.NewLiteral("Test")}, nil)

	count := 0
	for tr := range s.Triples(term.TriplePattern{}, nil) {
		count++
		if _, ok := tr.Subject.(term.BNode); !ok {
			t.Errorf("expected BNode subject, got %T", tr.Subject)
		}
	}
	if count != 1 {
		t.Errorf("count = %d, want 1", count)
	}
}

func TestDecodeRowLiteralWithLang(t *testing.T) {
	s := newTestStore(t)
	lit := term.NewLiteral("Hallo", term.WithLang("de"))
	s.Add(term.Triple{Subject: alice, Predicate: label, Object: lit}, nil)

	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: &label}, nil) {
		l, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatal("expected Literal object")
		}
		if l.Language() != "de" {
			t.Errorf("Language = %q, want %q", l.Language(), "de")
		}
		if l.Lexical() != "Hallo" {
			t.Errorf("Lexical = %q, want %q", l.Lexical(), "Hallo")
		}
	}
}

func TestDecodeRowLiteralWithDatatype(t *testing.T) {
	s := newTestStore(t)
	dt := term.NewURIRefUnsafe("http://www.w3.org/2001/XMLSchema#integer")
	lit := term.NewLiteral("42", term.WithDatatype(dt))
	s.Add(term.Triple{Subject: alice, Predicate: age, Object: lit}, nil)

	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: &age}, nil) {
		l, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatal("expected Literal object")
		}
		if l.Lexical() != "42" {
			t.Errorf("Lexical = %q, want %q", l.Lexical(), "42")
		}
		if l.Datatype() != dt {
			t.Errorf("Datatype = %v, want %v", l.Datatype(), dt)
		}
	}
}

func TestDecodeRowLiteralWithDir(t *testing.T) {
	s := newTestStore(t)
	lit := term.NewLiteral("Hello", term.WithLang("ar"), term.WithDir("rtl"))
	s.Add(term.Triple{Subject: alice, Predicate: label, Object: lit}, nil)

	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: &label}, nil) {
		l, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatal("expected Literal object")
		}
		if l.Dir() != "rtl" {
			t.Errorf("Dir = %q, want %q", l.Dir(), "rtl")
		}
	}
}

// NOTE: TripleTerm round-trip tests are omitted because TripleTerm keys
// contain NUL bytes (\x00) which are truncated by SQLite TEXT columns,
// making TripleTerms unsuitable for storage in the current schema.

func TestDecodeRowURIRefObject(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)

	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: &knows}, nil) {
		u, ok := tr.Object.(term.URIRef)
		if !ok {
			t.Fatalf("expected URIRef object, got %T", tr.Object)
		}
		if u != bob {
			t.Errorf("object = %v, want %v", u, bob)
		}
	}
}

// ---------------------------------------------------------------------------
// ContextAware / TransactionAware
// ---------------------------------------------------------------------------

func TestContextAwareAndTransactionAware(t *testing.T) {
	s := newTestStore(t)
	if !s.ContextAware() {
		t.Error("ContextAware should be true")
	}
	if !s.TransactionAware() {
		t.Error("TransactionAware should be true")
	}
}

// ---------------------------------------------------------------------------
// Persistence
// ---------------------------------------------------------------------------

func TestPersistence(t *testing.T) {
	path := t.TempDir() + "/test.db"
	s, err := New(WithFile(path))
	if err != nil {
		t.Fatal(err)
	}
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Bind("ex", term.NewURIRefUnsafe("http://example.org/"))
	s.Close()

	s2, err := New(WithFile(path))
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	if got := s2.Len(nil); got != 1 {
		t.Errorf("Len after reopen = %d, want 1", got)
	}
	ns, ok := s2.Namespace("ex")
	if !ok || ns.Value() != "http://example.org/" {
		t.Errorf("Namespace after reopen = %v, %v", ns, ok)
	}
}

// ---------------------------------------------------------------------------
// Concurrency
// ---------------------------------------------------------------------------

func TestConcurrency(t *testing.T) {
	s := newTestStore(t)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			subj := term.NewURIRefUnsafe("http://example.org/" + string(rune('A'+i%26)))
			s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
		}(i)
	}
	wg.Wait()
	if got := s.Len(nil); got == 0 {
		t.Error("expected some triples after concurrent writes")
	}
}

func TestConcurrentReadsAndWrites(t *testing.T) {
	s := newTestStore(t)
	// Pre-populate.
	for i := 0; i < 20; i++ {
		subj := term.NewURIRefUnsafe("http://example.org/" + string(rune('A'+i)))
		s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
	}

	var wg sync.WaitGroup
	// Concurrent writes.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			subj := term.NewURIRefUnsafe("http://example.org/new" + string(rune('A'+i)))
			s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
		}(i)
	}
	// Concurrent reads.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range s.Triples(term.TriplePattern{}, nil) {
			}
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Empty store operations
// ---------------------------------------------------------------------------

func TestEmptyStoreOperations(t *testing.T) {
	s := newTestStore(t)
	if got := s.Len(nil); got != 0 {
		t.Errorf("Len of empty store = %d, want 0", got)
	}
	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
	}
	if count != 0 {
		t.Errorf("Triples of empty store = %d, want 0", count)
	}
	s.Remove(term.TriplePattern{}, nil) // should not panic
}

// ---------------------------------------------------------------------------
// Literal types round-trip
// ---------------------------------------------------------------------------

func TestLiteralTypes(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}, nil)
	langLit := term.NewLiteral("Alice", term.WithLang("en"))
	s.Add(term.Triple{Subject: alice, Predicate: label, Object: langLit}, nil)

	if got := s.Len(nil); got != 3 {
		t.Errorf("Len = %d, want 3", got)
	}

	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: &label}, nil) {
		lit, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatal("expected Literal")
		}
		if lit.Language() != "en" {
			t.Errorf("Language = %q, want %q", lit.Language(), "en")
		}
	}
}

func TestLiteralBooleanRoundTrip(t *testing.T) {
	s := newTestStore(t)
	active := term.NewURIRefUnsafe("http://example.org/active")
	s.Add(term.Triple{Subject: alice, Predicate: active, Object: term.NewLiteral(true)}, nil)

	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: &active}, nil) {
		l, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatal("expected Literal")
		}
		if l.Lexical() != "true" {
			t.Errorf("Lexical = %q, want %q", l.Lexical(), "true")
		}
	}
}

func TestLiteralFloat64RoundTrip(t *testing.T) {
	s := newTestStore(t)
	weight := term.NewURIRefUnsafe("http://example.org/weight")
	s.Add(term.Triple{Subject: alice, Predicate: weight, Object: term.NewLiteral(3.14)}, nil)

	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: &weight}, nil) {
		l, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatal("expected Literal")
		}
		if l.Lexical() != "3.14" {
			t.Errorf("Lexical = %q, want %q", l.Lexical(), "3.14")
		}
	}
}

// ---------------------------------------------------------------------------
// Plugin registration
// ---------------------------------------------------------------------------

func TestPluginRegistration(t *testing.T) {
	st, ok := plugin.GetStore("sqlite")
	if !ok {
		t.Fatal("sqlite store not registered")
	}
	if st == nil {
		t.Fatal("sqlite store factory returned nil")
	}
	// Verify it's a SQLiteStore.
	sqlSt, ok := st.(*SQLiteStore)
	if !ok {
		t.Fatalf("expected *SQLiteStore, got %T", st)
	}
	defer sqlSt.Close()

	// Verify it works.
	sqlSt.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	if got := sqlSt.Len(nil); got != 1 {
		t.Errorf("Len = %d, want 1", got)
	}
}

// ---------------------------------------------------------------------------
// Close
// ---------------------------------------------------------------------------

func TestCloseIdempotent(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	// Second close should return an error (db already closed) but not panic.
	_ = s.Close()
}

// ---------------------------------------------------------------------------
// buildQuery (tested indirectly via TriplesWithLimit and Count)
// ---------------------------------------------------------------------------

func TestBuildQueryAllCombinations(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: knows, Object: carol}, nil)
	s.Add(term.Triple{Subject: carol, Predicate: name, Object: term.NewLiteral("Carol")}, nil)

	tests := []struct {
		name    string
		pattern term.TriplePattern
		wantN   int
	}{
		{"wildcard", term.TriplePattern{}, 5},
		{"s only", term.TriplePattern{Subject: alice}, 2},
		{"p only", term.TriplePattern{Predicate: &name}, 3},
		{"o only", term.TriplePattern{Object: bob}, 1},
		{"sp", term.TriplePattern{Subject: alice, Predicate: &name}, 1},
		{"so", term.TriplePattern{Subject: alice, Object: bob}, 1},
		{"po", term.TriplePattern{Predicate: &knows, Object: bob}, 1},
		{"spo", term.TriplePattern{Subject: alice, Predicate: &knows, Object: bob}, 1},
		{"no match", term.TriplePattern{Subject: carol, Predicate: &knows, Object: alice}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_count", func(t *testing.T) {
			if got := s.Count(tt.pattern, nil); got != tt.wantN {
				t.Errorf("Count = %d, want %d", got, tt.wantN)
			}
		})
		t.Run(tt.name+"_limit", func(t *testing.T) {
			count := 0
			for range s.TriplesWithLimit(tt.pattern, nil, 100, 0) {
				count++
			}
			if count != tt.wantN {
				t.Errorf("TriplesWithLimit = %d, want %d", count, tt.wantN)
			}
		})
		t.Run(tt.name+"_exists", func(t *testing.T) {
			got := s.Exists(tt.pattern, nil)
			want := tt.wantN > 0
			if got != want {
				t.Errorf("Exists = %v, want %v", got, want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Error paths (operations on closed DB should not panic)
// ---------------------------------------------------------------------------

func TestAddOnClosedDB(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	// Should log error but not panic.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
}

func TestAddNOnClosedDB(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	// Should log error but not panic.
	s.AddN([]term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}},
	})
}

func TestRemoveOnClosedDB(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	// Should log error but not panic.
	s.Remove(term.TriplePattern{Subject: alice}, nil)
}

func TestSetOnClosedDB(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	// Should log error but not panic.
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
}

func TestTriplesOnClosedDB(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	// Should return empty iterator without panicking.
	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
	}
	if count != 0 {
		t.Errorf("Triples on closed DB = %d, want 0", count)
	}
}

func TestTriplesWithLimitOnClosedDB(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 10, 0) {
		count++
	}
	if count != 0 {
		t.Errorf("TriplesWithLimit on closed DB = %d, want 0", count)
	}
}

func TestLenOnClosedDB(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if got := s.Len(nil); got != 0 {
		t.Errorf("Len on closed DB = %d, want 0", got)
	}
}

func TestCountOnClosedDB(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if got := s.Count(term.TriplePattern{}, nil); got != 0 {
		t.Errorf("Count on closed DB = %d, want 0", got)
	}
}

func TestExistsOnClosedDB(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if s.Exists(term.TriplePattern{}, nil) {
		t.Error("Exists on closed DB should be false")
	}
}

func TestContextsOnClosedDB(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	count := 0
	for range s.Contexts(nil) {
		count++
	}
	if count != 0 {
		t.Errorf("Contexts on closed DB = %d, want 0", count)
	}
}

func TestBindOnClosedDB(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	// Should log error but not panic.
	s.Bind("ex", term.NewURIRefUnsafe("http://example.org/"))
}

func TestNamespacesOnClosedDB(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	count := 0
	for range s.Namespaces() {
		count++
	}
	if count != 0 {
		t.Errorf("Namespaces on closed DB = %d, want 0", count)
	}
}

// ---------------------------------------------------------------------------
// decodeRow error paths (unit test internal function directly)
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// New error paths
// ---------------------------------------------------------------------------

func TestNewWithInvalidPath(t *testing.T) {
	// Use a path that's a directory, not a file — should fail during pragmas or schema.
	_, err := New(WithFile("/dev/null/impossible"))
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestNewWithEmptyDSN(t *testing.T) {
	_, err := New(WithFile(""))
	if err == nil {
		t.Error("expected error for empty DSN")
	}
}

func TestNewBothOptions(t *testing.T) {
	// WithFile takes precedence if both given, InMemory flag is overridden.
	path := t.TempDir() + "/both.db"
	s, err := New(WithFile(path), WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
}

func TestDecodeRowInvalidSubject(t *testing.T) {
	_, err := decodeRow("INVALID_KEY", "U:http://example.org/p", "U:http://example.org/o")
	if err == nil {
		t.Error("expected error for invalid subject key")
	}
}

func TestDecodeRowInvalidPredicate(t *testing.T) {
	_, err := decodeRow("U:http://example.org/s", "INVALID_KEY", "U:http://example.org/o")
	if err == nil {
		t.Error("expected error for invalid predicate key")
	}
}

func TestDecodeRowInvalidObject(t *testing.T) {
	_, err := decodeRow("U:http://example.org/s", "U:http://example.org/p", "INVALID_KEY")
	if err == nil {
		t.Error("expected error for invalid object key")
	}
}

func TestDecodeRowPredicateNotURIRef(t *testing.T) {
	// BNode key as predicate (not a valid predicate type).
	_, err := decodeRow("U:http://example.org/s", "B:bnode1", "U:http://example.org/o")
	if err == nil {
		t.Error("expected error for non-URIRef predicate")
	}
}

func TestDecodeRowSubjectNotSubject(t *testing.T) {
	// Literal as subject (not a valid subject type).
	_, err := decodeRow(`L:"hello"`, "U:http://example.org/p", "U:http://example.org/o")
	if err == nil {
		t.Error("expected error for Literal as subject")
	}
}

func TestDecodeRowValidTriple(t *testing.T) {
	tr, err := decodeRow("U:http://example.org/s", "U:http://example.org/p", "U:http://example.org/o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tr.Subject.N3() != "<http://example.org/s>" {
		t.Errorf("subject = %v", tr.Subject)
	}
}

func TestDecodeRowBNodeSubject(t *testing.T) {
	tr, err := decodeRow("B:bn1", "U:http://example.org/p", "U:http://example.org/o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tr.Subject.(term.BNode); !ok {
		t.Errorf("expected BNode subject, got %T", tr.Subject)
	}
}

func TestDecodeRowLiteralObject(t *testing.T) {
	tr, err := decodeRow("U:http://example.org/s", "U:http://example.org/p", `L:"hello"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := tr.Object.(term.Literal); !ok {
		t.Errorf("expected Literal object, got %T", tr.Object)
	}
}

func TestDecodeRowLiteralWithLangObject(t *testing.T) {
	tr, err := decodeRow("U:http://example.org/s", "U:http://example.org/p", `L:"hello"@en`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if l, ok := tr.Object.(term.Literal); ok {
		if l.Language() != "en" {
			t.Errorf("Language = %q, want %q", l.Language(), "en")
		}
	} else {
		t.Errorf("expected Literal, got %T", tr.Object)
	}
}

// ---------------------------------------------------------------------------
// Bad data in DB (covers decodeRow error paths in iterators)
// ---------------------------------------------------------------------------

func TestTriplesWithBadSubjectKey(t *testing.T) {
	s := newTestStore(t)
	// Insert a row with an invalid subject key directly.
	_, err := s.db.Exec(`INSERT INTO triples (subject, predicate, object, graph) VALUES (?, ?, ?, ?)`,
		"INVALID", "U:http://example.org/p", "U:http://example.org/o", "")
	if err != nil {
		t.Fatal(err)
	}
	// Also insert a valid row.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	// Triples iterator should skip the bad row and return the valid one.
	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("Triples with bad row = %d, want 1 (bad row skipped)", count)
	}
}

func TestTriplesWithLimitWithBadRow(t *testing.T) {
	s := newTestStore(t)
	_, err := s.db.Exec(`INSERT INTO triples (subject, predicate, object, graph) VALUES (?, ?, ?, ?)`,
		"INVALID", "U:http://example.org/p", "U:http://example.org/o", "")
	if err != nil {
		t.Fatal(err)
	}
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 10, 0) {
		count++
	}
	if count != 1 {
		t.Errorf("TriplesWithLimit with bad row = %d, want 1", count)
	}
}

func TestSetAfterDropTable(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	// Drop the triples table to force errors in Set's tx.Exec calls.
	s.db.Exec("DROP TABLE triples")
	// Should log error but not panic.
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("New")}, nil)
}

func TestAddNAfterDropTable(t *testing.T) {
	s := newTestStore(t)
	// Drop the triples table to force errors in AddN's tx.Prepare.
	s.db.Exec("DROP TABLE triples")
	// Should log error but not panic.
	s.AddN([]term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}},
	})
}

func TestContextsWithBadGraphKey(t *testing.T) {
	s := newTestStore(t)
	// Insert a row with an invalid graph key.
	_, err := s.db.Exec(`INSERT INTO triples (subject, predicate, object, graph) VALUES (?, ?, ?, ?)`,
		"U:http://example.org/s", "U:http://example.org/p", "U:http://example.org/o", "INVALID_GRAPH")
	if err != nil {
		t.Fatal(err)
	}
	// Also insert a valid named graph.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)

	count := 0
	for range s.Contexts(nil) {
		count++
	}
	// The invalid graph should be skipped by TermFromKey, valid graph returned.
	if count != 1 {
		t.Errorf("Contexts with bad graph = %d, want 1", count)
	}
}

func TestAddNExecError(t *testing.T) {
	s := newTestStore(t)

	// Use a trigger to fail the exec after prepare succeeds.
	s.db.Exec(`CREATE TRIGGER reject_addn_insert BEFORE INSERT ON triples
		BEGIN SELECT RAISE(ABORT, 'forced exec error'); END`)

	// AddN's prepare will succeed, but exec will fail due to the trigger.
	s.AddN([]term.Quad{
		{Triple: term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}},
	})
}

func TestSetDeleteError(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	// Rename to break the delete in Set.
	s.db.Exec("ALTER TABLE triples RENAME TO triples_broken")
	// Set should fail at delete step and rollback.
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("New")}, nil)
}

func TestSetInsertError(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	// Add a trigger that rejects inserts to force the insert step to fail
	// while the delete step succeeds.
	s.db.Exec(`CREATE TRIGGER reject_insert BEFORE INSERT ON triples
		BEGIN SELECT RAISE(ABORT, 'forced insert error'); END`)

	// Set's delete will succeed, but the insert will fail due to the trigger.
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("New")}, nil)
	// The transaction should be rolled back, so the original data should still be gone
	// (or rolled back depending on when the error occurs).
}

// ---------------------------------------------------------------------------
// Integration: Add, query, remove, verify
// ---------------------------------------------------------------------------

func TestFullLifecycle(t *testing.T) {
	s := newTestStore(t)

	// Add some data.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)

	// Verify count.
	if got := s.Len(nil); got != 3 {
		t.Fatalf("Len = %d, want 3", got)
	}

	// Query with pattern.
	if got := s.Count(term.TriplePattern{Subject: alice}, nil); got != 2 {
		t.Errorf("Count(alice) = %d, want 2", got)
	}

	// Check existence.
	if !s.Exists(term.TriplePattern{Subject: alice, Predicate: &knows, Object: bob}, nil) {
		t.Error("alice knows bob should exist")
	}

	// Set replaces.
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice Updated")}, nil)
	if got := s.Len(nil); got != 3 {
		t.Errorf("Len after Set = %d, want 3", got)
	}

	// Remove.
	s.Remove(term.TriplePattern{Subject: alice, Predicate: &name}, nil)
	if got := s.Len(nil); got != 2 {
		t.Errorf("Len after Remove = %d, want 2", got)
	}

	// Remove all.
	s.Remove(term.TriplePattern{}, nil)
	if got := s.Len(nil); got != 0 {
		t.Errorf("Len after Remove all = %d, want 0", got)
	}
}
