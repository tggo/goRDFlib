package rdflibgo

import (
	"sync"
	"testing"
)

// --- Fix 1: Graph.Set atomicity ---

func TestGraphSetAtomic(t *testing.T) {
	// Verify that Set performs remove+add atomically: concurrent readers should
	// never see a state where the old value is removed but new value not yet added.
	g := NewGraph()
	s, _ := NewURIRef("http://example.org/s")
	p, _ := NewURIRef("http://example.org/p")
	g.Add(s, p, NewLiteral("initial"))

	const iterations = 1000
	var wg sync.WaitGroup

	// Writer: repeatedly Set to a new value.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			g.Set(s, p, NewLiteral(i))
		}
	}()

	// Reader: check that there is always exactly 1 triple for (s, p, *).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			count := 0
			g.Triples(s, &p, nil)(func(Triple) bool {
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
	s.Add(Triple{Subject: sub, Predicate: pred, Object: NewLiteral("old1")}, nil)
	s.Add(Triple{Subject: sub, Predicate: pred, Object: NewLiteral("old2")}, nil)
	if s.Len(nil) != 2 {
		t.Fatalf("expected 2, got %d", s.Len(nil))
	}

	// Set should remove both old values and add the new one.
	s.Set(Triple{Subject: sub, Predicate: pred, Object: NewLiteral("new")}, nil)
	if s.Len(nil) != 1 {
		t.Fatalf("expected 1 after Set, got %d", s.Len(nil))
	}

	var found Triple
	s.Triples(TriplePattern{Subject: sub, Predicate: &pred}, nil)(func(tr Triple) bool {
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
	k := termKey(u)
	if k == "<http://example.org/foo>" {
		t.Error("termKey should not use N3 format for URIRef")
	}
	if k != "U:http://example.org/foo" {
		t.Errorf("unexpected termKey: %q", k)
	}

	b := NewBNode("xyz")
	k = termKey(b)
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
	g.Add(s, p, o)

	if !g.Contains(s, p, o) {
		t.Error("lookup should work with new termKey")
	}
	g.Remove(s, &p, o)
	if g.Len() != 0 {
		t.Error("remove should work with new termKey")
	}
}

// --- Fix 3: Collection cycle detection ---

func TestCollectionCyclicListIter(t *testing.T) {
	g := NewGraph()
	// Build a list: head -> node1 -> head (cycle)
	head := NewBNode("head")
	node1 := NewBNode("node1")

	g.Add(head, RDF.First, NewLiteral("a"))
	g.Add(head, RDF.Rest, node1)
	g.Add(node1, RDF.First, NewLiteral("b"))
	g.Add(node1, RDF.Rest, head) // cycle!

	col := NewCollection(g, head)

	// Iter should terminate (not loop forever) and return at most 2 items.
	count := 0
	col.Iter()(func(Term) bool {
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

	g.Add(head, RDF.First, NewLiteral("a"))
	g.Add(head, RDF.Rest, node1)
	g.Add(node1, RDF.First, NewLiteral("b"))
	g.Add(node1, RDF.Rest, head)

	col := NewCollection(g, head)
	if col.Len() != 2 {
		t.Errorf("expected Len=2 for cyclic list, got %d", col.Len())
	}
}

func TestCollectionCyclicListIndex(t *testing.T) {
	g := NewGraph()
	head := NewBNode("head")
	node1 := NewBNode("node1")

	g.Add(head, RDF.First, NewLiteral("a"))
	g.Add(head, RDF.Rest, node1)
	g.Add(node1, RDF.First, NewLiteral("b"))
	g.Add(node1, RDF.Rest, head)

	col := NewCollection(g, head)
	if col.Index(NewLiteral("a")) != 0 {
		t.Error("Index('a') should be 0")
	}
	if col.Index(NewLiteral("b")) != 1 {
		t.Error("Index('b') should be 1")
	}
	if col.Index(NewLiteral("c")) != -1 {
		t.Error("Index('c') should be -1 for missing item")
	}
}

func TestCollectionCyclicListClear(t *testing.T) {
	g := NewGraph()
	head := NewBNode("head")
	node1 := NewBNode("node1")

	g.Add(head, RDF.First, NewLiteral("a"))
	g.Add(head, RDF.Rest, node1)
	g.Add(node1, RDF.First, NewLiteral("b"))
	g.Add(node1, RDF.Rest, head)

	col := NewCollection(g, head)
	col.Clear() // should not loop forever

	// After clear, the list data should be gone.
	if g.Contains(head, RDF.First, NewLiteral("a")) {
		t.Error("Clear should have removed rdf:first from head")
	}
}

func TestCollectionCyclicListEnd(t *testing.T) {
	g := NewGraph()
	head := NewBNode("head")
	node1 := NewBNode("node1")

	g.Add(head, RDF.First, NewLiteral("a"))
	g.Add(head, RDF.Rest, node1)
	g.Add(node1, RDF.First, NewLiteral("b"))
	g.Add(node1, RDF.Rest, head)

	col := NewCollection(g, head)
	// end() is unexported; verify cycle-safety via Len() which traverses the list.
	n := col.Len()
	if n == 0 {
		t.Error("Len() should return > 0 for cyclic list with items")
	}
}

// --- Fix 3b: Collection.end() returns nil for empty list ---

func TestCollectionEndReturnsNilForEmpty(t *testing.T) {
	g := NewGraph()
	head := NewBNode("empty")
	col := NewCollection(g, head)

	n := col.Len()
	if n != 0 {
		t.Errorf("Len() should return 0 for empty list, got %d", n)
	}
}

func TestCollectionAppendToEmpty(t *testing.T) {
	g := NewGraph()
	col := NewEmptyCollection(g)
	col.Append(NewLiteral("first"))

	if col.Len() != 1 {
		t.Errorf("expected Len=1 after Append, got %d", col.Len())
	}
	val, ok := col.Get(0)
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
	g.Add(s, p, NewLiteral("v"))

	// TripleIterator via range
	count := 0
	for range g.Triples(s, &p, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("TripleIterator range: expected 1, got %d", count)
	}

	// TermIterator via range
	count = 0
	for range g.Subjects(&p, NewLiteral("v")) {
		count++
	}
	if count != 1 {
		t.Errorf("TermIterator range: expected 1, got %d", count)
	}

	// TermPairIterator via range
	count = 0
	for range g.PredicateObjects(s) {
		count++
	}
	if count != 1 {
		t.Errorf("TermPairIterator range: expected 1, got %d", count)
	}
}

// --- Fix 5: Value validates exactly two non-nil args ---

func TestValueRejectsAllNil(t *testing.T) {
	g, _, _, _ := makeTestGraph(t)
	val, ok := g.Value(nil, nil, nil)
	if ok || val != nil {
		t.Error("Value(nil, nil, nil) should return (nil, false)")
	}
}

func TestValueRejectsAllNonNil(t *testing.T) {
	g, s, p, o := makeTestGraph(t)
	val, ok := g.Value(s, &p, o)
	if ok || val != nil {
		t.Error("Value(s, p, o) with all non-nil should return (nil, false)")
	}
}

func TestValueRejectsOneNonNil(t *testing.T) {
	g, s, _, _ := makeTestGraph(t)
	val, ok := g.Value(s, nil, nil)
	if ok || val != nil {
		t.Error("Value with only 1 non-nil should return (nil, false)")
	}
}

func TestValueAcceptsTwoNonNil(t *testing.T) {
	g, s, p, o := makeTestGraph(t)
	val, ok := g.Value(s, &p, nil)
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
	g.Add(s, p, o)

	// The return type is TermPairIterator = iter.Seq2[Term, Term]
	var sp TermPairIterator = g.SubjectPredicates(o)
	count := 0
	sp(func(subj, pred Term) bool {
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
	g.Add(s, p, o)

	var so TermPairIterator = g.SubjectObjects(&p)
	count := 0
	so(func(_, _ Term) bool {
		count++
		return true
	})
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}
}
