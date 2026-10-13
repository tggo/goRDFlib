package integration_test

import (
	"sync"
	"testing"

	. "github.com/tggo/goRDFlib"
	"github.com/tggo/goRDFlib/term"
)

func makeTestGraph(t *testing.T) (*Graph, URIRef, URIRef, Literal) {
	t.Helper()
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("hello")
	g.Add(t.Context(), s, p, o)
	return g, s, p, o
}

// --- Fix 1: Graph.Set atomicity ---

func TestGraphSetAtomic(t *testing.T) {
	// Verify that Set performs remove+add atomically: concurrent readers should
	// never see a state where the old value is removed but new value not yet added.
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, NewLiteral("initial"))

	const iterations = 1000
	var wg sync.WaitGroup

	// Writer: repeatedly Set to a new value.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			g.Set(t.Context(), s, p, NewLiteral(i))
		}
	}()

	// Reader: check that there is always exactly 1 triple for (s, p, *).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			count := 0
			g.Triples(t.Context(), s, &p, nil)(func(Triple, error) bool {
				count++
				return true
			})
			if count != 1 {
				t.Errorf("Set not atomic: found %d triples for (s, p, *)", count)
				return
			}
		}
	}()

	wg.Wait()
}

func TestStoreSetMethod(t *testing.T) {
	// Verify MemoryStore.Set works correctly.
	s := NewMemoryStore()
	sub, _ := NewURIRef("http://example.org/s")
	pred, _ := NewURIRef("http://example.org/p")
	s.Add(t.Context(), Triple{Subject: sub, Predicate: pred, Object: NewLiteral("old1")}, nil)
	s.Add(t.Context(), Triple{Subject: sub, Predicate: pred, Object: NewLiteral("old2")}, nil)
	if mustV(s.Len(t.Context(), nil)) != 2 {
		t.Fatalf("expected 2, got %d", mustV(s.Len(t.Context(), nil)))
	}

	// Set should remove both old values and add the new one.
	s.Set(t.Context(), Triple{Subject: sub, Predicate: pred, Object: NewLiteral("new")}, nil)
	if mustV(s.Len(t.Context(), nil)) != 1 {
		t.Fatalf("expected 1 after Set, got %d", mustV(s.Len(t.Context(), nil)))
	}

	var found Triple
	s.Triples(t.Context(), TriplePattern{Subject: sub, Predicate: &pred}, nil)(func(tr Triple, _ error) bool {
		found = tr
		return false
	})
	if found.Object.(Literal).Value() != "new" {
		t.Errorf("expected object 'new', got %v", found.Object)
	}
}

// --- Fix 2: termKey efficiency ---

func TestTermKeyDoesNotUseN3(t *testing.T) {
	// Verify that termKey for URIRef does NOT wrap in angle brackets (N3 format).
	u, _ := NewURIRef("http://example.org/foo")
	k := term.TermKey(u)
	if k == "<http://example.org/foo>" {
		t.Error("termKey should not use N3 format for URIRef")
	}
	if k != "U:http://example.org/foo" {
		t.Errorf("unexpected termKey: %q", k)
	}

	b := NewBNode("xyz")
	k = term.TermKey(b)
	if k != "B:xyz" {
		t.Errorf("unexpected BNode termKey: %q", k)
	}
}

func TestTermKeyStillWorksForLookups(t *testing.T) {
	// Verify store operations still work correctly with new termKey.
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("hello")
	g.Add(t.Context(), s, p, o)

	if !mustV(g.Contains(t.Context(), s, p, o)) {
		t.Error("lookup should work with new termKey")
	}
	g.Remove(t.Context(), s, &p, o)
	if mustV(g.Len(t.Context())) != 0 {
		t.Error("remove should work with new termKey")
	}
}

// --- Fix 3: Collection cycle detection ---

func TestCollectionCyclicListIter(t *testing.T) {
	g := NewGraph()
	// Build a list: head -> node1 -> head (cycle)
	head := NewBNode("head")
	node1 := NewBNode("node1")

	g.Add(t.Context(), head, RDF.First, NewLiteral("a"))
	g.Add(t.Context(), head, RDF.Rest, node1)
	g.Add(t.Context(), node1, RDF.First, NewLiteral("b"))
	g.Add(t.Context(), node1, RDF.Rest, head) // cycle!

	col := NewCollection(g, head)

	// Iter should terminate (not loop forever) and return at most 2 items.
	count := 0
	col.Iter(t.Context())(func(Term, error) bool {
		count++
		if count > 100 {
			t.Fatal("infinite loop detected in Iter")
		}
		return true
	})
	if count != 2 {
		t.Errorf("expected 2 items from cyclic list Iter, got %d", count)
	}
}

func TestCollectionCyclicListLen(t *testing.T) {
	g := NewGraph()
	head := NewBNode("head")
	node1 := NewBNode("node1")

	g.Add(t.Context(), head, RDF.First, NewLiteral("a"))
	g.Add(t.Context(), head, RDF.Rest, node1)
	g.Add(t.Context(), node1, RDF.First, NewLiteral("b"))
	g.Add(t.Context(), node1, RDF.Rest, head)

	col := NewCollection(g, head)
	if mustV(col.Len(t.Context())) != 2 {
		t.Errorf("expected Len=2 for cyclic list, got %d", mustV(col.Len(t.Context())))
	}
}

func TestCollectionCyclicListIndex(t *testing.T) {
	g := NewGraph()
	head := NewBNode("head")
	node1 := NewBNode("node1")

	g.Add(t.Context(), head, RDF.First, NewLiteral("a"))
	g.Add(t.Context(), head, RDF.Rest, node1)
	g.Add(t.Context(), node1, RDF.First, NewLiteral("b"))
	g.Add(t.Context(), node1, RDF.Rest, head)

	col := NewCollection(g, head)
	if mustV(col.Index(t.Context(), NewLiteral("a"))) != 0 {
		t.Error("Index('a') should be 0")
	}
	if mustV(col.Index(t.Context(), NewLiteral("b"))) != 1 {
		t.Error("Index('b') should be 1")
	}
	if mustV(col.Index(t.Context(), NewLiteral("c"))) != -1 {
		t.Error("Index('c') should be -1 for missing item")
	}
}

func TestCollectionCyclicListClear(t *testing.T) {
	g := NewGraph()
	head := NewBNode("head")
	node1 := NewBNode("node1")

	g.Add(t.Context(), head, RDF.First, NewLiteral("a"))
	g.Add(t.Context(), head, RDF.Rest, node1)
	g.Add(t.Context(), node1, RDF.First, NewLiteral("b"))
	g.Add(t.Context(), node1, RDF.Rest, head)

	col := NewCollection(g, head)
	col.Clear(t.Context()) // should not loop forever

	// After clear, the list data should be gone.
	if mustV(g.Contains(t.Context(), head, RDF.First, NewLiteral("a"))) {
		t.Error("Clear should have removed rdf:first from head")
	}
}

func TestCollectionCyclicListEnd(t *testing.T) {
	g := NewGraph()
	head := NewBNode("head")
	node1 := NewBNode("node1")

	g.Add(t.Context(), head, RDF.First, NewLiteral("a"))
	g.Add(t.Context(), head, RDF.Rest, node1)
	g.Add(t.Context(), node1, RDF.First, NewLiteral("b"))
	g.Add(t.Context(), node1, RDF.Rest, head)

	col := NewCollection(g, head)
	// end() is unexported; verify cycle-safety via Len() which traverses the list.
	n, _ := col.Len(t.Context())
	if n == 0 {
		t.Error("Len() should return > 0 for cyclic list with items")
	}
}

// --- Fix 3b: Collection.end() returns nil for empty list ---

func TestCollectionEndReturnsNilForEmpty(t *testing.T) {
	g := NewGraph()
	head := NewBNode("empty")
	col := NewCollection(g, head)

	n, _ := col.Len(t.Context())
	if n != 0 {
		t.Errorf("Len() should return 0 for empty list, got %d", n)
	}
}

func TestCollectionAppendToEmpty(t *testing.T) {
	g := NewGraph()
	col := NewEmptyCollection(g)
	col.Append(t.Context(), NewLiteral("first"))

	if mustV(col.Len(t.Context())) != 1 {
		t.Errorf("expected Len=1 after Append, got %d", mustV(col.Len(t.Context())))
	}
	val, ok, _ := col.Get(t.Context(), 0)
	if !ok || val.(Literal).Value() != "first" {
		t.Errorf("expected 'first', got %v", val)
	}
}

// --- Fix 4: iter.Seq/iter.Seq2 type aliases ---

func TestIteratorTypesAreRangeCompatible(t *testing.T) {
	// This test verifies that TripleIterator, TermIterator, TermPairIterator
	// work with range-over-func syntax (Go 1.23+).
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, NewLiteral("v"))

	// TripleIterator via range
	count := 0
	for range g.Triples(t.Context(), s, &p, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("TripleIterator range: expected 1, got %d", count)
	}

	// TermIterator via range
	count = 0
	for range g.Subjects(t.Context(), &p, NewLiteral("v")) {
		count++
	}
	if count != 1 {
		t.Errorf("TermIterator range: expected 1, got %d", count)
	}

	// TermPairIterator via range
	count = 0
	for range g.PredicateObjects(t.Context(), s) {
		count++
	}
	if count != 1 {
		t.Errorf("TermPairIterator range: expected 1, got %d", count)
	}
}

// --- Fix 5: Value validates exactly two non-nil args ---

func TestValueRejectsAllNil(t *testing.T) {
	g, _, _, _ := makeTestGraph(t)
	val, ok, _ := g.Value(t.Context(), nil, nil, nil)
	if ok || val != nil {
		t.Error("Value(nil, nil, nil) should return (nil, false)")
	}
}

func TestValueRejectsAllNonNil(t *testing.T) {
	g, s, p, o := makeTestGraph(t)
	val, ok, _ := g.Value(t.Context(), s, &p, o)
	if ok || val != nil {
		t.Error("Value(s, p, o) with all non-nil should return (nil, false)")
	}
}

func TestValueRejectsOneNonNil(t *testing.T) {
	g, s, _, _ := makeTestGraph(t)
	val, ok, _ := g.Value(t.Context(), s, nil, nil)
	if ok || val != nil {
		t.Error("Value with only 1 non-nil should return (nil, false)")
	}
}

func TestValueAcceptsTwoNonNil(t *testing.T) {
	g, s, p, o := makeTestGraph(t)
	val, ok, _ := g.Value(t.Context(), s, &p, nil)
	if !ok || val.N3() != o.N3() {
		t.Errorf("Value(s, p, nil) should work, got %v, %v", val, ok)
	}
}

// --- TermPairIterator return type ---

func TestSubjectPredicatesReturnType(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("v")
	g.Add(t.Context(), s, p, o)

	// The return type is TermPairIterator = iter.Seq2[Term, Term]
	var sp TermPairIterator = g.SubjectPredicates(t.Context(), o)
	count := 0
	sp(func(pair_ TermPair, _ error) bool {
		count++
		return true
	})
	if count != 1 {
		t.Errorf("expected 1 pair, got %d", count)
	}
}

func TestSubjectObjectsReturnType(t *testing.T) {
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	o := NewLiteral("v")
	g.Add(t.Context(), s, p, o)

	var so TermPairIterator = g.SubjectObjects(t.Context(), &p)
	count := 0
	so(func(pair_ TermPair, _ error) bool {
		count++
		return true
	})
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}
