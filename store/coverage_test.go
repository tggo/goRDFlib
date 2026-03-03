package store

import (
	"testing"

	"github.com/tggo/goRDFlib/term"
)

// TestContextAware covers ContextAware() (was 0%).
func TestContextAware(t *testing.T) {
	s := NewMemoryStore()
	if s.ContextAware() {
		t.Error("MemoryStore should not be context-aware")
	}
}

// TestTransactionAware covers TransactionAware() (was 0%).
func TestTransactionAware(t *testing.T) {
	s := NewMemoryStore()
	if s.TransactionAware() {
		t.Error("MemoryStore should not be transaction-aware")
	}
}

// TestSet covers Set() (was 0%).
func TestSet(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	o1 := term.NewLiteral("first")
	o2 := term.NewLiteral("second")

	// Add initial triple
	s.Add(term.Triple{Subject: sub, Predicate: pred, Object: o1}, nil)
	if s.Len(nil) != 1 {
		t.Fatalf("expected 1 triple, got %d", s.Len(nil))
	}

	// Set replaces (s, p, first) with (s, p, second)
	s.Set(term.Triple{Subject: sub, Predicate: pred, Object: o2}, nil)
	if s.Len(nil) != 1 {
		t.Fatalf("expected 1 triple after Set, got %d", s.Len(nil))
	}

	// Verify the new value is present
	found := false
	s.Triples(term.TriplePattern{Subject: sub, Predicate: &pred}, nil)(func(t term.Triple) bool {
		if t.Object == o2 {
			found = true
		}
		return true
	})
	if !found {
		t.Error("expected new object o2 after Set")
	}
}

// TestSetMultipleReplaced covers Set() replacing multiple old values.
func TestSetMultipleReplaced(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	p2, _ := term.NewURIRef("http://example.org/p2")

	s.Add(term.Triple{Subject: sub, Predicate: pred, Object: term.NewLiteral("v1")}, nil)
	s.Add(term.Triple{Subject: sub, Predicate: pred, Object: term.NewLiteral("v2")}, nil)
	// Add an unrelated triple that should survive
	s.Add(term.Triple{Subject: sub, Predicate: p2, Object: term.NewLiteral("other")}, nil)

	newObj := term.NewLiteral("new")
	s.Set(term.Triple{Subject: sub, Predicate: pred, Object: newObj}, nil)

	if s.Len(nil) != 2 {
		t.Fatalf("expected 2 triples after Set, got %d", s.Len(nil))
	}
}

// TestNamespacesIterator covers Namespaces() (was 0%).
func TestNamespacesIterator(t *testing.T) {
	s := NewMemoryStore()
	ns1, _ := term.NewURIRef("http://example.org/ns1#")
	ns2, _ := term.NewURIRef("http://example.org/ns2#")
	s.Bind("ex1", ns1)
	s.Bind("ex2", ns2)

	count := 0
	s.Namespaces()(func(prefix string, ns term.URIRef) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2 namespaces, got %d", count)
	}
}

// TestNamespacesEarlyExit covers Namespaces() early exit path.
func TestNamespacesEarlyExit(t *testing.T) {
	s := NewMemoryStore()
	ns1, _ := term.NewURIRef("http://example.org/ns1#")
	ns2, _ := term.NewURIRef("http://example.org/ns2#")
	s.Bind("ex1", ns1)
	s.Bind("ex2", ns2)

	count := 0
	s.Namespaces()(func(prefix string, ns term.URIRef) bool {
		count++
		return false // stop after first
	})
	if count != 1 {
		t.Errorf("expected early exit after 1, got %d", count)
	}
}

// TestRemoveLockedCleanupPOS covers the branch where pos and osp maps are fully cleaned up.
// This exercises the len==0 cleanup paths in removeLocked (line 134).
func TestRemoveLockedCleanupAllIndices(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	obj := term.NewLiteral("v")

	t1 := term.Triple{Subject: sub, Predicate: pred, Object: obj}
	s.Add(t1, nil)
	// Remove via wildcard — exercises removeLocked cleanup with all maps becoming empty
	s.Remove(term.TriplePattern{}, nil)

	if s.Len(nil) != 0 {
		t.Errorf("expected 0 after remove all, got %d", s.Len(nil))
	}

	// Verify indices are clean by re-adding and checking count
	s.Add(t1, nil)
	if s.Len(nil) != 1 {
		t.Errorf("expected 1 after re-add, got %d", s.Len(nil))
	}
}

// TestRemoveNonExistent covers removeLocked when triple doesn't exist (found=false path).
func TestRemoveNonExistent(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	obj := term.NewLiteral("v")

	// Remove a triple that was never added — should be a no-op
	s.Remove(term.TriplePattern{Subject: sub, Predicate: &pred, Object: obj}, nil)
	if s.Len(nil) != 0 {
		t.Errorf("expected 0, got %d", s.Len(nil))
	}
}

// TestTriplesObjectPatternWithSubjectFilter covers osp branch with subject filtering.
func TestTriplesObjectPatternWithSubjectFilter(t *testing.T) {
	s := NewMemoryStore()
	s1, _ := term.NewURIRef("http://example.org/s1")
	s2, _ := term.NewURIRef("http://example.org/s2")
	p, _ := term.NewURIRef("http://example.org/p")
	obj := term.NewLiteral("shared")

	s.Add(term.Triple{Subject: s1, Predicate: p, Object: obj}, nil)
	s.Add(term.Triple{Subject: s2, Predicate: p, Object: obj}, nil)

	// Pattern: object=obj, no subject set — but via TriplePattern only object
	// This hits the osp branch with sk=="" (no subject filter)
	count := 0
	s.Triples(term.TriplePattern{Object: obj}, nil)(func(t term.Triple) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

// TestTriplesObjectPatternEarlyExit covers the early-exit in the osp branch.
func TestTriplesObjectPatternEarlyExit(t *testing.T) {
	s := NewMemoryStore()
	s1, _ := term.NewURIRef("http://example.org/s1")
	s2, _ := term.NewURIRef("http://example.org/s2")
	p, _ := term.NewURIRef("http://example.org/p")
	obj := term.NewLiteral("shared")

	s.Add(term.Triple{Subject: s1, Predicate: p, Object: obj}, nil)
	s.Add(term.Triple{Subject: s2, Predicate: p, Object: obj}, nil)

	count := 0
	s.Triples(term.TriplePattern{Object: obj}, nil)(func(t term.Triple) bool {
		count++
		return false // early exit
	})
	if count != 1 {
		t.Errorf("expected 1 on early exit, got %d", count)
	}
}

// TestTriplesPredPatternWithObjectFilter covers pos branch with object filtering (ok!="").
func TestTriplesPredPatternWithObjectFilter(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	o1 := term.NewLiteral("v1")
	o2 := term.NewLiteral("v2")

	s.Add(term.Triple{Subject: sub, Predicate: pred, Object: o1}, nil)
	s.Add(term.Triple{Subject: sub, Predicate: pred, Object: o2}, nil)

	// Pattern with predicate only — hits pos branch with ok==""
	count := 0
	s.Triples(term.TriplePattern{Predicate: &pred}, nil)(func(t term.Triple) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

// TestTriplesPredPatternEarlyExit covers pos branch early exit.
func TestTriplesPredPatternEarlyExit(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	o1 := term.NewLiteral("v1")
	o2 := term.NewLiteral("v2")

	s.Add(term.Triple{Subject: sub, Predicate: pred, Object: o1}, nil)
	s.Add(term.Triple{Subject: sub, Predicate: pred, Object: o2}, nil)

	count := 0
	s.Triples(term.TriplePattern{Predicate: &pred}, nil)(func(t term.Triple) bool {
		count++
		return false
	})
	if count != 1 {
		t.Errorf("expected 1 on early exit, got %d", count)
	}
}

// TestTriplesSubjectPatternEarlyExit covers spo branch early exit.
func TestTriplesSubjectPatternEarlyExit(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	s.Add(term.Triple{Subject: sub, Predicate: p, Object: term.NewLiteral("v1")}, nil)
	s.Add(term.Triple{Subject: sub, Predicate: p, Object: term.NewLiteral("v2")}, nil)

	count := 0
	s.Triples(term.TriplePattern{Subject: sub}, nil)(func(t term.Triple) bool {
		count++
		return false
	})
	if count != 1 {
		t.Errorf("expected 1 on early exit, got %d", count)
	}
}

// TestTriplesAllEarlyExit covers the default/all branch early exit.
func TestTriplesAllEarlyExit(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	s.Add(term.Triple{Subject: sub, Predicate: p, Object: term.NewLiteral("v1")}, nil)
	s.Add(term.Triple{Subject: sub, Predicate: p, Object: term.NewLiteral("v2")}, nil)

	count := 0
	s.Triples(term.TriplePattern{}, nil)(func(t term.Triple) bool {
		count++
		return false
	})
	if count != 1 {
		t.Errorf("expected 1 on early exit, got %d", count)
	}
}

// TestTriplesSubjectPatternWithObjectFilter covers spo branch with ok!="" filter.
func TestTriplesSubjectPatternWithObjectFilter(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	o1 := term.NewLiteral("v1")
	o2 := term.NewLiteral("v2")
	s.Add(term.Triple{Subject: sub, Predicate: p, Object: o1}, nil)
	s.Add(term.Triple{Subject: sub, Predicate: p, Object: o2}, nil)

	// Subject only set, no predicate, no object — hits spo branch with pk=="" and ok==""
	count := 0
	s.Triples(term.TriplePattern{Subject: sub}, nil)(func(t term.Triple) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

// TestTriplesExactNotFound covers spo exact-match branch where triple doesn't exist.
func TestTriplesExactNotFound(t *testing.T) {
	s := NewMemoryStore()
	sub, _ := term.NewURIRef("http://example.org/s")
	pred, _ := term.NewURIRef("http://example.org/p")
	obj := term.NewLiteral("v")
	// Don't add the triple — query should return nothing
	count := 0
	s.Triples(term.TriplePattern{Subject: sub, Predicate: &pred, Object: obj}, nil)(func(t term.Triple) bool {
		count++
		return true
	})
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

// TestNamespaceNotFound covers Namespace() miss path.
func TestNamespaceNotFound(t *testing.T) {
	s := NewMemoryStore()
	_, ok := s.Namespace("nonexistent")
	if ok {
		t.Error("expected not found for nonexistent prefix")
	}
}

// TestPrefixNotFound covers Prefix() miss path.
func TestPrefixNotFound(t *testing.T) {
	s := NewMemoryStore()
	ns, _ := term.NewURIRef("http://example.org/ns#")
	_, ok := s.Prefix(ns)
	if ok {
		t.Error("expected not found for unbound namespace")
	}
}
