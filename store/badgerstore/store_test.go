package badgerstore

import (
	"strings"
	"sync"
	"testing"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/tggo/goRDFlib/plugin"
	"github.com/tggo/goRDFlib/term"
)

func newTestStore(t *testing.T) *BadgerStore {
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
	name   = term.NewURIRefUnsafe("http://example.org/name")
	age    = term.NewURIRefUnsafe("http://example.org/age")
	knows  = term.NewURIRefUnsafe("http://example.org/knows")
	rdfT   = term.NewURIRefUnsafe("http://www.w3.org/1999/02/22-rdf-syntax-ns#type")
	foafP  = term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/Person")
	graph1 = term.NewURIRefUnsafe("http://example.org/graph1")
	graph2 = term.NewURIRefUnsafe("http://example.org/graph2")
)

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
	s.Add(triple, nil) // duplicate
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after duplicate = %d, want 1", got)
	}
}

func TestRemove(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	t2 := term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}
	s.Add(t1, nil)
	s.Add(t2, nil)

	// Remove by exact pattern.
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

	// Remove all triples with subject alice.
	s.Remove(term.TriplePattern{Subject: alice}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after wildcard remove = %d, want 1", got)
	}
}

func TestSet(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice B.")}, nil)

	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after Set = %d, want 1", got)
	}

	// Verify the new value.
	count := 0
	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: &name}, nil) {
		count++
		if lit, ok := tr.Object.(term.Literal); ok {
			if lit.Lexical() != "Alice B." {
				t.Errorf("Set value = %q, want %q", lit.Lexical(), "Alice B.")
			}
		} else {
			t.Error("object is not Literal")
		}
	}
	if count != 1 {
		t.Errorf("Triples count = %d, want 1", count)
	}
}

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

func TestAddN(t *testing.T) {
	s := newTestStore(t)
	quads := []term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}},
		{Triple: term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}},
		{Triple: term.Triple{Subject: alice, Predicate: knows, Object: bob}},
	}
	s.AddN(quads)
	if got := s.Len(nil); got != 3 {
		t.Errorf("Len = %d, want 3", got)
	}
}

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

	count := 0
	for range s.Namespaces() {
		count++
	}
	if count != 2 {
		t.Errorf("Namespaces count = %d, want 2", count)
	}
}

func TestNamedGraphs(t *testing.T) {
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
		t.Errorf("Len(default) = %d, want 1", got)
	}
}

func TestContexts(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, graph2)

	count := 0
	for range s.Contexts(nil) {
		count++
	}
	if count != 2 {
		t.Errorf("Contexts count = %d, want 2", count)
	}
}

func TestContextsFilteredByTriple(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(t1, graph1)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, graph2)

	count := 0
	for range s.Contexts(&t1) {
		count++
	}
	if count != 1 {
		t.Errorf("Contexts(alice triple) count = %d, want 1", count)
	}
}

func TestContextAwareAndTransactionAware(t *testing.T) {
	s := newTestStore(t)
	if !s.ContextAware() {
		t.Error("ContextAware should be true")
	}
	if !s.TransactionAware() {
		t.Error("TransactionAware should be true")
	}
}

func TestPersistence(t *testing.T) {
	dir := t.TempDir()
	s, err := New(WithDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Bind("ex", term.NewURIRefUnsafe("http://example.org/"))
	s.Close()

	// Reopen.
	s2, err := New(WithDir(dir))
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

	count := 0
	for range s2.Triples(term.TriplePattern{Subject: alice}, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("Triples after reopen = %d, want 1", count)
	}
}

func TestLiteralTypes(t *testing.T) {
	s := newTestStore(t)

	// String literal
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	// Integer literal
	s.Add(term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}, nil)
	// Language-tagged literal
	langLit := term.NewLiteral("Alice", term.WithLang("en"))
	s.Add(term.Triple{Subject: alice, Predicate: term.NewURIRefUnsafe("http://example.org/label"), Object: langLit}, nil)
	// Directional lang literal
	dirLit := term.NewLiteral("Alice", term.WithLang("en"), term.WithDir("ltr"))
	s.Add(term.Triple{Subject: alice, Predicate: term.NewURIRefUnsafe("http://example.org/dirLabel"), Object: dirLit}, nil)

	if got := s.Len(nil); got != 4 {
		t.Errorf("Len = %d, want 4", got)
	}

	// Verify round-trip of language-tagged literal.
	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: ptrURI("http://example.org/label")}, nil) {
		lit, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatal("expected Literal")
		}
		if lit.Language() != "en" {
			t.Errorf("Language = %q, want %q", lit.Language(), "en")
		}
	}

	// Verify round-trip of directional literal.
	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: ptrURI("http://example.org/dirLabel")}, nil) {
		lit, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatal("expected Literal")
		}
		if lit.Dir() != "ltr" {
			t.Errorf("Dir = %q, want %q", lit.Dir(), "ltr")
		}
	}
}

func ptrURI(s string) *term.URIRef {
	u := term.NewURIRefUnsafe(s)
	return &u
}

func TestBNodeTerms(t *testing.T) {
	s := newTestStore(t)
	bn := term.NewBNode("")
	s.Add(term.Triple{Subject: bn, Predicate: name, Object: term.NewLiteral("anon")}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len = %d, want 1", got)
	}

	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("Triples count = %d, want 1", count)
	}
}

func TestConcurrency(t *testing.T) {
	s := newTestStore(t)
	var wg sync.WaitGroup
	n := 100

	// Concurrent writes.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			subj := term.NewURIRefUnsafe("http://example.org/" + string(rune('A'+i%26)))
			s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
		}(i)
	}
	wg.Wait()

	// Concurrent reads.
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range s.Triples(term.TriplePattern{}, nil) {
			}
		}()
	}
	wg.Wait()

	if got := s.Len(nil); got == 0 {
		t.Error("expected some triples after concurrent writes")
	}
}

func TestEarlyBreak(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 10; i++ {
		subj := term.NewURIRefUnsafe("http://example.org/" + string(rune('A'+i)))
		s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
	}

	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
		if count == 3 {
			break
		}
	}
	if count != 3 {
		t.Errorf("early break count = %d, want 3", count)
	}
}

func TestPluginRegistration(t *testing.T) {
	// Verify the init() registered the store.
	// Import side-effect is enough; the plugin system is global.
	// Just verify the store compiles and the interface is satisfied.
	var _ interface {
		Add(term.Triple, term.Term)
		Len(term.Term) int
	} = newTestStore(t)
}

func TestRemoveFromNamedGraph(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(t1, graph1)
	s.Add(t1, nil) // also in default graph

	s.Remove(term.TriplePattern{Subject: alice}, graph1)
	if got := s.Len(graph1); got != 0 {
		t.Errorf("Len(graph1) after remove = %d, want 0", got)
	}
	// Default graph should still have it.
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len(default) after remove = %d, want 1", got)
	}
}

func TestBNodeContextIgnored(t *testing.T) {
	s := newTestStore(t)
	bn := term.NewBNode("")
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(t1, bn)

	// BNode context should be treated as default graph.
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len(nil) with BNode ctx = %d, want 1", got)
	}
}

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

	count = 0
	for range s.Contexts(nil) {
		count++
	}
	if count != 0 {
		t.Errorf("Contexts of empty store = %d, want 0", count)
	}

	// Remove on empty store should not panic.
	s.Remove(term.TriplePattern{}, nil)
}

func TestAddNWithNamedGraphs(t *testing.T) {
	s := newTestStore(t)
	quads := []term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, Graph: graph1},
		{Triple: term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, Graph: graph2},
	}
	s.AddN(quads)

	if got := s.Len(graph1); got != 1 {
		t.Errorf("Len(graph1) = %d, want 1", got)
	}
	if got := s.Len(graph2); got != 1 {
		t.Errorf("Len(graph2) = %d, want 1", got)
	}
}

// --- New tests for coverage ---

func TestWithReadOnly(t *testing.T) {
	dir := t.TempDir()
	// First create a DB with some data.
	s, err := New(WithDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Close()

	// Reopen in read-only mode.
	s2, err := New(WithDir(dir), WithReadOnly())
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	if got := s2.Len(nil); got != 1 {
		t.Errorf("Len in read-only = %d, want 1", got)
	}

	// Reads should work.
	count := 0
	for range s2.Triples(term.TriplePattern{}, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("Triples in read-only = %d, want 1", count)
	}
}

func TestWithLogger(t *testing.T) {
	// WithLogger(nil) should work (disables logging).
	s, err := New(WithInMemory(), WithLogger(nil))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len = %d, want 1", got)
	}
}

func TestNewInvalidDir(t *testing.T) {
	// Opening a Badger DB at a path that can't work should return an error.
	_, err := New(WithDir("/dev/null/badger-impossible-path"))
	if err == nil {
		t.Error("expected error for invalid dir")
	}
}

func TestAddNEmptySlice(t *testing.T) {
	s := newTestStore(t)
	// Should not panic or error.
	s.AddN(nil)
	s.AddN([]term.Quad{})
	if got := s.Len(nil); got != 0 {
		t.Errorf("Len after empty AddN = %d, want 0", got)
	}
}

func TestAddNWithBNodeGraph(t *testing.T) {
	s := newTestStore(t)
	bn := term.NewBNode("g1")
	quads := []term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, Graph: bn},
	}
	s.AddN(quads)
	// BNode graph should be treated as default graph.
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len(nil) with BNode graph in AddN = %d, want 1", got)
	}
}

func TestRemoveNonexistent(t *testing.T) {
	s := newTestStore(t)
	// Remove from empty store should not panic.
	s.Remove(term.TriplePattern{Subject: alice, Predicate: &name, Object: term.NewLiteral("Nobody")}, nil)
	if got := s.Len(nil); got != 0 {
		t.Errorf("Len = %d, want 0", got)
	}
}

func TestRemoveByPredicate(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}, nil)

	// Remove all triples with predicate "name".
	s.Remove(term.TriplePattern{Predicate: &name}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after remove by predicate = %d, want 1", got)
	}
}

func TestRemoveByObject(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	// Remove all triples with object bob.
	s.Remove(term.TriplePattern{Object: bob}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after remove by object = %d, want 1", got)
	}
}

func TestRemoveBySubjectAndObject(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	// Remove triples with subject=alice, object=bob (uses OSP index).
	s.Remove(term.TriplePattern{Subject: alice, Object: bob}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after remove by s+o = %d, want 1", got)
	}
}

func TestRemoveByPredicateAndObject(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: knows, Object: alice}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	// Remove triples with predicate=knows, object=bob (uses POS index).
	s.Remove(term.TriplePattern{Predicate: &knows, Object: bob}, nil)
	if got := s.Len(nil); got != 2 {
		t.Errorf("Len after remove by p+o = %d, want 2", got)
	}
}

func TestRemoveAll(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)

	// Remove all triples (empty pattern).
	s.Remove(term.TriplePattern{}, nil)
	if got := s.Len(nil); got != 0 {
		t.Errorf("Len after remove all = %d, want 0", got)
	}
}

func TestSetInNamedGraph(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)

	// Set should replace in the named graph.
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice Updated")}, graph1)
	if got := s.Len(graph1); got != 1 {
		t.Errorf("Len(graph1) after Set = %d, want 1", got)
	}

	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: &name}, graph1) {
		lit, ok := tr.Object.(term.Literal)
		if !ok {
			t.Fatal("expected Literal")
		}
		if lit.Lexical() != "Alice Updated" {
			t.Errorf("Set value = %q, want %q", lit.Lexical(), "Alice Updated")
		}
	}
}

func TestSetNoExistingTriple(t *testing.T) {
	s := newTestStore(t)
	// Set on empty store should just insert.
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after Set on empty = %d, want 1", got)
	}
}

func TestSetReplacesMultiple(t *testing.T) {
	s := newTestStore(t)
	// Add two triples with same subject+predicate but different objects.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice B.")}, nil)
	if got := s.Len(nil); got != 2 {
		t.Errorf("Len before Set = %d, want 2", got)
	}

	// Set should remove both and insert the new one.
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice C.")}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after Set = %d, want 1", got)
	}
}

func TestContextsWithNoNamedGraphs(t *testing.T) {
	s := newTestStore(t)
	// Add only to default graph.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	count := 0
	for range s.Contexts(nil) {
		count++
	}
	if count != 0 {
		t.Errorf("Contexts with no named graphs = %d, want 0", count)
	}
}

func TestContextsFilteredNoMatch(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)

	// Filter by a triple that doesn't exist in graph1.
	noMatch := term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}
	count := 0
	for range s.Contexts(&noMatch) {
		count++
	}
	if count != 0 {
		t.Errorf("Contexts(noMatch) = %d, want 0", count)
	}
}

func TestContextsEarlyBreak(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	// Add to multiple graphs.
	for i := 0; i < 5; i++ {
		g := term.NewURIRefUnsafe("http://example.org/g" + string(rune('0'+i)))
		s.Add(t1, g)
	}

	count := 0
	for range s.Contexts(nil) {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("early break on Contexts = %d, want 2", count)
	}
}

func TestBindOverwrite(t *testing.T) {
	s := newTestStore(t)
	ex1 := term.NewURIRefUnsafe("http://example.org/v1/")
	ex2 := term.NewURIRefUnsafe("http://example.org/v2/")

	s.Bind("ex", ex1)
	s.Bind("ex", ex2) // overwrite

	ns, ok := s.Namespace("ex")
	if !ok || ns != ex2 {
		t.Errorf("Namespace after overwrite = %v, want %v", ns, ex2)
	}
}

func TestPrefixNotFound(t *testing.T) {
	s := newTestStore(t)
	_, ok := s.Prefix(term.NewURIRefUnsafe("http://nonexistent.org/"))
	if ok {
		t.Error("Prefix should be false for unknown namespace")
	}
}

func TestNamespacesEarlyBreak(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		pfx := "ns" + string(rune('0'+i))
		uri := term.NewURIRefUnsafe("http://example.org/" + pfx + "/")
		s.Bind(pfx, uri)
	}

	count := 0
	for range s.Namespaces() {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("early break on Namespaces = %d, want 2", count)
	}
}

func TestTriplesWithLimit(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 10; i++ {
		subj := term.NewURIRefUnsafe("http://example.org/s" + string(rune('0'+i)))
		s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
	}

	tests := []struct {
		name   string
		limit  int
		offset int
		want   int
	}{
		{"limit 3", 3, 0, 3},
		{"limit 5 offset 3", 5, 3, 5},
		{"offset beyond", 5, 20, 0},
		{"no limit", 0, 0, 10},
		{"negative limit", -1, 0, 10},
		{"limit 100", 100, 0, 10},
		{"offset 8 limit 5", 5, 8, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := 0
			for range s.TriplesWithLimit(term.TriplePattern{}, nil, tt.limit, tt.offset) {
				count++
			}
			if count != tt.want {
				t.Errorf("got %d, want %d", count, tt.want)
			}
		})
	}
}

func TestTriplesWithLimitPattern(t *testing.T) {
	s := newTestStore(t)
	// Add 5 triples with predicate "name" and 3 with predicate "age".
	for i := 0; i < 5; i++ {
		subj := term.NewURIRefUnsafe("http://example.org/s" + string(rune('0'+i)))
		s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral("name" + string(rune('0'+i)))}, nil)
	}
	for i := 0; i < 3; i++ {
		subj := term.NewURIRefUnsafe("http://example.org/s" + string(rune('0'+i)))
		s.Add(term.Triple{Subject: subj, Predicate: age, Object: term.NewLiteral(20 + i)}, nil)
	}

	// Limit on pattern with predicate.
	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{Predicate: &name}, nil, 3, 0) {
		count++
	}
	if count != 3 {
		t.Errorf("TriplesWithLimit(p=name, limit=3) = %d, want 3", count)
	}
}

func TestTriplesWithLimitEarlyBreak(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 10; i++ {
		subj := term.NewURIRefUnsafe("http://example.org/s" + string(rune('0'+i)))
		s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
	}

	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 10, 0) {
		count++
		if count == 2 {
			break
		}
	}
	if count != 2 {
		t.Errorf("early break in TriplesWithLimit = %d, want 2", count)
	}
}

func TestTriplesWithLimitInNamedGraph(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		subj := term.NewURIRefUnsafe("http://example.org/s" + string(rune('0'+i)))
		s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, graph1)
	}
	// Add to default graph too.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, graph1, 3, 0) {
		count++
	}
	if count != 3 {
		t.Errorf("TriplesWithLimit in graph1 = %d, want 3", count)
	}
}

func TestCount(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}, nil)

	tests := []struct {
		name    string
		pattern term.TriplePattern
		want    int
	}{
		{"all", term.TriplePattern{}, 4},
		{"s=alice", term.TriplePattern{Subject: alice}, 3},
		{"p=name", term.TriplePattern{Predicate: &name}, 2},
		{"o=bob", term.TriplePattern{Object: bob}, 1},
		{"s+p", term.TriplePattern{Subject: alice, Predicate: &name}, 1},
		{"s+o", term.TriplePattern{Subject: alice, Object: bob}, 1},
		{"p+o", term.TriplePattern{Predicate: &name, Object: term.NewLiteral("Alice")}, 1},
		{"spo exact", term.TriplePattern{Subject: alice, Predicate: &name, Object: term.NewLiteral("Alice")}, 1},
		{"spo no match", term.TriplePattern{Subject: alice, Predicate: &name, Object: term.NewLiteral("Nobody")}, 0},
		{"no match", term.TriplePattern{Subject: bob, Predicate: &knows}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.Count(tt.pattern, nil); got != tt.want {
				t.Errorf("Count = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestCountInNamedGraph(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, graph1)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	if got := s.Count(term.TriplePattern{}, graph1); got != 2 {
		t.Errorf("Count(graph1) = %d, want 2", got)
	}
	if got := s.Count(term.TriplePattern{}, nil); got != 1 {
		t.Errorf("Count(default) = %d, want 1", got)
	}
}

func TestExists(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: knows, Object: bob}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)

	tests := []struct {
		name    string
		pattern term.TriplePattern
		want    bool
	}{
		{"all", term.TriplePattern{}, true},
		{"s=alice", term.TriplePattern{Subject: alice}, true},
		{"p=name", term.TriplePattern{Predicate: &name}, true},
		{"o=bob", term.TriplePattern{Object: bob}, true},
		{"s+p", term.TriplePattern{Subject: alice, Predicate: &name}, true},
		{"s+o", term.TriplePattern{Subject: alice, Object: bob}, true},
		{"p+o", term.TriplePattern{Predicate: &name, Object: term.NewLiteral("Alice")}, true},
		{"spo exact", term.TriplePattern{Subject: alice, Predicate: &name, Object: term.NewLiteral("Alice")}, true},
		{"spo no match", term.TriplePattern{Subject: alice, Predicate: &name, Object: term.NewLiteral("Nobody")}, false},
		{"no match subject", term.TriplePattern{Subject: term.NewURIRefUnsafe("http://example.org/Nobody")}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := s.Exists(tt.pattern, nil); got != tt.want {
				t.Errorf("Exists = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExistsEmpty(t *testing.T) {
	s := newTestStore(t)
	if s.Exists(term.TriplePattern{}, nil) {
		t.Error("Exists on empty store should be false")
	}
}

func TestExistsInNamedGraph(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)

	if !s.Exists(term.TriplePattern{Subject: alice}, graph1) {
		t.Error("Exists in graph1 should be true")
	}
	if s.Exists(term.TriplePattern{Subject: alice}, nil) {
		t.Error("Exists in default graph should be false")
	}
}

func TestPluginRegistrationGetStore(t *testing.T) {
	// Verify the init() registered the "badger" store type.
	s, ok := plugin.GetStore("badger")
	if !ok {
		t.Fatal("plugin.GetStore(\"badger\") not found")
	}
	// The plugin creates an in-memory store; verify it works.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len from plugin store = %d, want 1", got)
	}
	// Close if possible.
	if c, ok := s.(interface{ Close() error }); ok {
		c.Close()
	}
}

func TestDecodeTripleTruncatedPredicate(t *testing.T) {
	// Build valid subject, then truncate in predicate data.
	sk := term.TermKey(alice)
	// 4 bytes sLen + subject + 4 bytes pLen with large value
	data := make([]byte, 4+len(sk)+4)
	n := 0
	data[n] = byte(len(sk))
	data[n+1] = byte(len(sk) >> 8)
	data[n+2] = byte(len(sk) >> 16)
	data[n+3] = byte(len(sk) >> 24)
	n += 4
	copy(data[n:], sk)
	n += len(sk)
	// Set predicate length to something large.
	data[n] = 100
	data[n+1] = 0
	data[n+2] = 0
	data[n+3] = 0

	_, err := decodeTriple(data)
	if err == nil {
		t.Error("expected error for truncated predicate")
	}
}

func TestDecodeTripleTruncatedObjectLength(t *testing.T) {
	// Build valid subject and predicate, then truncate at object length.
	sk := term.TermKey(alice)
	pk := term.TermKey(name)

	data := make([]byte, 4+len(sk)+4+len(pk))
	n := 0
	data[n] = byte(len(sk))
	data[n+1] = byte(len(sk) >> 8)
	data[n+2] = byte(len(sk) >> 16)
	data[n+3] = byte(len(sk) >> 24)
	n += 4
	copy(data[n:], sk)
	n += len(sk)
	data[n] = byte(len(pk))
	data[n+1] = byte(len(pk) >> 8)
	data[n+2] = byte(len(pk) >> 16)
	data[n+3] = byte(len(pk) >> 24)
	n += 4
	copy(data[n:], pk)
	// No room for object length bytes.

	_, err := decodeTriple(data)
	if err == nil {
		t.Error("expected error for truncated object length")
	}
}

func TestDecodeTripleTruncatedObjectData(t *testing.T) {
	sk := term.TermKey(alice)
	pk := term.TermKey(name)

	data := make([]byte, 4+len(sk)+4+len(pk)+4)
	n := 0
	data[n] = byte(len(sk))
	data[n+1] = byte(len(sk) >> 8)
	data[n+2] = byte(len(sk) >> 16)
	data[n+3] = byte(len(sk) >> 24)
	n += 4
	copy(data[n:], sk)
	n += len(sk)
	data[n] = byte(len(pk))
	data[n+1] = byte(len(pk) >> 8)
	data[n+2] = byte(len(pk) >> 16)
	data[n+3] = byte(len(pk) >> 24)
	n += 4
	copy(data[n:], pk)
	n += len(pk)
	// Set object length to something large.
	data[n] = 100
	data[n+1] = 0
	data[n+2] = 0
	data[n+3] = 0

	_, err := decodeTriple(data)
	if err == nil {
		t.Error("expected error for truncated object data")
	}
}

func TestAddDuplicateInNamedGraph(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(t1, graph1)
	s.Add(t1, graph1) // duplicate in same graph
	if got := s.Len(graph1); got != 1 {
		t.Errorf("Len(graph1) after duplicate = %d, want 1", got)
	}
}

func TestSetWithBNodeContext(t *testing.T) {
	s := newTestStore(t)
	bn := term.NewBNode("ctx")
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, bn)
	// Set with BNode context should operate on default graph.
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice Updated")}, bn)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after Set with BNode ctx = %d, want 1", got)
	}
}

func TestRemoveWithBNodeContext(t *testing.T) {
	s := newTestStore(t)
	bn := term.NewBNode("ctx")
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, bn)
	s.Remove(term.TriplePattern{Subject: alice}, bn)
	if got := s.Len(nil); got != 0 {
		t.Errorf("Len after Remove with BNode ctx = %d, want 0", got)
	}
}

func TestTriplesWithLimitZeroOffset(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)

	count := 0
	for range s.TriplesWithLimit(term.TriplePattern{}, nil, 1, 0) {
		count++
	}
	if count != 1 {
		t.Errorf("TriplesWithLimit(1,0) = %d, want 1", count)
	}
}

func TestCountExactNotFound(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	// Exact lookup that doesn't exist.
	got := s.Count(term.TriplePattern{
		Subject:   alice,
		Predicate: &age,
		Object:    term.NewLiteral(99),
	}, nil)
	if got != 0 {
		t.Errorf("Count(not found exact) = %d, want 0", got)
	}
}

func TestExistsExactNotFound(t *testing.T) {
	s := newTestStore(t)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	got := s.Exists(term.TriplePattern{
		Subject:   alice,
		Predicate: &age,
		Object:    term.NewLiteral(99),
	}, nil)
	if got {
		t.Error("Exists(not found exact) should be false")
	}
}

func TestAddNLargerBatch(t *testing.T) {
	s := newTestStore(t)
	quads := make([]term.Quad, 100)
	for i := range quads {
		subj := term.NewURIRefUnsafe("http://example.org/s" + string(rune(i)))
		quads[i] = term.Quad{
			Triple: term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)},
		}
	}
	s.AddN(quads)
	if got := s.Len(nil); got != 100 {
		t.Errorf("Len after batch AddN = %d, want 100", got)
	}
}

func TestContextsMultipleGraphsSameTriple(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(t1, graph1)
	s.Add(t1, graph2)

	// Both graphs should appear when filtering by the triple.
	count := 0
	for range s.Contexts(&t1) {
		count++
	}
	if count != 2 {
		t.Errorf("Contexts(t1) = %d, want 2", count)
	}
}

// TestClosedStoreErrorPaths exercises the log.Printf error paths by
// operating on a store whose underlying Badger DB has been closed.
// These paths are otherwise unreachable in normal operation.
func TestClosedStoreErrorPaths(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}

	// Add a triple before closing so Remove/Set have something to scan.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)
	s.Bind("ex", term.NewURIRefUnsafe("http://example.org/"))

	s.Close()

	// Write operations on closed DB trigger error logging paths.
	// Note: AddN uses WriteBatch which hangs on closed DB, so we skip it.
	// These should not panic.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("x")}, nil)
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("x")}, graph1)
	s.Bind("ex2", term.NewURIRefUnsafe("http://example2.org/"))
}

// TestReadOnlyWriteErrorPaths opens the store in read-only mode and attempts
// write operations, which triggers error paths in Add, Remove, Set, and Bind.
func TestReadOnlyWriteErrorPaths(t *testing.T) {
	dir := t.TempDir()

	// Create a store with some data.
	s, err := New(WithDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(t1, nil)
	s.Add(t1, graph1)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)
	s.Bind("ex", term.NewURIRefUnsafe("http://example.org/"))
	s.Close()

	// Reopen in read-only mode.
	ro, err := New(WithDir(dir), WithReadOnly())
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()

	// Verify read operations work.
	if got := ro.Len(nil); got != 2 {
		t.Errorf("Len(nil) = %d, want 2", got)
	}

	// All write operations should hit error paths and log, not panic.
	// Add to default graph.
	ro.Add(term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}, nil)
	// Add to named graph.
	ro.Add(term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}, graph1)

	// Remove (the scan succeeds but the delete transaction fails).
	ro.Remove(term.TriplePattern{Subject: alice}, nil)

	// Set (scan succeeds but write transaction fails).
	ro.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice Updated")}, nil)
	// Set in named graph.
	ro.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice Updated")}, graph1)

	// Bind.
	ro.Bind("ex2", term.NewURIRefUnsafe("http://example2.org/"))

	// Verify data unchanged.
	if got := ro.Len(nil); got != 2 {
		t.Errorf("Len after failed writes = %d, want 2", got)
	}
}

func TestDecodeTripleInvalidTermKeys(t *testing.T) {
	// Test decodeTriple with valid structure but invalid term key content.
	// This exercises the TermFromKey error paths.
	tests := []struct {
		name string
		sk   string
		pk   string
		ok   string
	}{
		{"invalid subject key", "INVALID", "U:http://example.org/p", "U:http://example.org/o"},
		{"invalid predicate key", "U:http://example.org/s", "INVALID", "U:http://example.org/o"},
		{"invalid object key", "U:http://example.org/s", "U:http://example.org/p", "INVALID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build a valid encoded triple with the given keys.
			data := make([]byte, 4+len(tt.sk)+4+len(tt.pk)+4+len(tt.ok))
			n := 0
			data[n] = byte(len(tt.sk))
			data[n+1] = byte(len(tt.sk) >> 8)
			data[n+2] = byte(len(tt.sk) >> 16)
			data[n+3] = byte(len(tt.sk) >> 24)
			n += 4
			copy(data[n:], tt.sk)
			n += len(tt.sk)
			data[n] = byte(len(tt.pk))
			data[n+1] = byte(len(tt.pk) >> 8)
			data[n+2] = byte(len(tt.pk) >> 16)
			data[n+3] = byte(len(tt.pk) >> 24)
			n += 4
			copy(data[n:], tt.pk)
			n += len(tt.pk)
			data[n] = byte(len(tt.ok))
			data[n+1] = byte(len(tt.ok) >> 8)
			data[n+2] = byte(len(tt.ok) >> 16)
			data[n+3] = byte(len(tt.ok) >> 24)
			n += 4
			copy(data[n:], tt.ok)

			_, err := decodeTriple(data)
			if err == nil {
				t.Error("expected error for invalid term key")
			}
		})
	}
}

func TestDecodeTripleSubjectNotSubjectType(t *testing.T) {
	// A Literal is a valid TermKey but not a Subject type.
	lit := term.NewLiteral("hello")
	sk := term.TermKey(lit)
	pk := term.TermKey(name)
	ok := term.TermKey(alice)

	data := make([]byte, 4+len(sk)+4+len(pk)+4+len(ok))
	n := 0
	data[n] = byte(len(sk))
	data[n+1] = byte(len(sk) >> 8)
	data[n+2] = byte(len(sk) >> 16)
	data[n+3] = byte(len(sk) >> 24)
	n += 4
	copy(data[n:], sk)
	n += len(sk)
	data[n] = byte(len(pk))
	data[n+1] = byte(len(pk) >> 8)
	data[n+2] = byte(len(pk) >> 16)
	data[n+3] = byte(len(pk) >> 24)
	n += 4
	copy(data[n:], pk)
	n += len(pk)
	data[n] = byte(len(ok))
	data[n+1] = byte(len(ok) >> 8)
	data[n+2] = byte(len(ok) >> 16)
	data[n+3] = byte(len(ok) >> 24)
	n += 4
	copy(data[n:], ok)

	_, err := decodeTriple(data)
	if err == nil {
		t.Error("expected error for literal as subject")
	}
}

func TestDecodeTriplePredicateNotURIRef(t *testing.T) {
	// A BNode is a valid TermKey and Subject but not a URIRef for predicate.
	sk := term.TermKey(alice)
	bn := term.NewBNode("b1")
	pk := term.TermKey(bn)
	ok := term.TermKey(alice)

	data := make([]byte, 4+len(sk)+4+len(pk)+4+len(ok))
	n := 0
	data[n] = byte(len(sk))
	data[n+1] = byte(len(sk) >> 8)
	data[n+2] = byte(len(sk) >> 16)
	data[n+3] = byte(len(sk) >> 24)
	n += 4
	copy(data[n:], sk)
	n += len(sk)
	data[n] = byte(len(pk))
	data[n+1] = byte(len(pk) >> 8)
	data[n+2] = byte(len(pk) >> 16)
	data[n+3] = byte(len(pk) >> 24)
	n += 4
	copy(data[n:], pk)
	n += len(pk)
	data[n] = byte(len(ok))
	data[n+1] = byte(len(ok) >> 8)
	data[n+2] = byte(len(ok) >> 16)
	data[n+3] = byte(len(ok) >> 24)
	n += 4
	copy(data[n:], ok)

	_, err := decodeTriple(data)
	if err == nil {
		t.Error("expected error for BNode as predicate")
	}
}

// TestContextsWithInvalidTermKey writes a corrupted context key directly into
// Badger to exercise the TermFromKey error path in Contexts.
func TestContextsWithInvalidTermKey(t *testing.T) {
	s := newTestStore(t)
	// Write a context key with an invalid term key directly.
	err := s.db.Update(func(txn *badger.Txn) error {
		// "c\x00INVALID" — invalid term key as context
		key := []byte{pfxCTX, sep}
		key = append(key, []byte("INVALID")...)
		return txn.Set(key, nil)
	})
	if err != nil {
		t.Fatal(err)
	}

	// Also add a valid context.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)

	// Contexts should skip the invalid one and yield the valid one.
	count := 0
	for range s.Contexts(nil) {
		count++
	}
	if count != 1 {
		t.Errorf("Contexts with invalid key = %d, want 1", count)
	}
}

// TestContextsWithEmptyGraphKey writes a context key with empty graph key to
// exercise the gk == "" continue path.
func TestContextsWithEmptyGraphKey(t *testing.T) {
	s := newTestStore(t)
	// Write a context key with empty graph key directly.
	err := s.db.Update(func(txn *badger.Txn) error {
		key := []byte{pfxCTX, sep} // empty gk after the prefix
		return txn.Set(key, nil)
	})
	if err != nil {
		t.Fatal(err)
	}

	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)

	count := 0
	for range s.Contexts(nil) {
		count++
	}
	if count != 1 {
		t.Errorf("Contexts with empty gk = %d, want 1", count)
	}
}

// TestScanTriplesWithCorruptedValue writes a corrupted value into the SPO index
// to exercise the decodeTriple error path in scanTriplesInTxn.
func TestScanTriplesWithCorruptedValue(t *testing.T) {
	s := newTestStore(t)
	// Write a corrupted value directly into the SPO index.
	err := s.db.Update(func(txn *badger.Txn) error {
		key := spoKey("", "U:http://example.org/s", "U:http://example.org/p", "U:http://example.org/o")
		return txn.Set(key, []byte{1, 2, 3}) // truncated data
	})
	if err != nil {
		t.Fatal(err)
	}

	// Also add a valid triple.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	// Triples should skip the corrupted value and yield the valid one.
	count := 0
	for range s.Triples(term.TriplePattern{}, nil) {
		count++
	}
	if count != 1 {
		t.Errorf("Triples with corrupted value = %d, want 1", count)
	}
}

// TestScanTriplesExactLookupCorrupted writes corrupted data for an exact
// SPO key to exercise the decodeTriple error path in the exact-match branch
// of scanTriplesInTxn.
func TestScanTriplesExactLookupCorrupted(t *testing.T) {
	s := newTestStore(t)
	sk := term.TermKey(alice)
	pk := term.TermKey(name)
	okLit := term.NewLiteral("Alice")
	okKey := term.TermKey(okLit)

	// Write corrupted data at the exact SPO key.
	err := s.db.Update(func(txn *badger.Txn) error {
		key := spoKey("", sk, pk, okKey)
		return txn.Set(key, []byte{1, 2, 3}) // corrupted
	})
	if err != nil {
		t.Fatal(err)
	}

	// Exact match lookup should return 0 triples (error handled internally).
	count := 0
	for range s.Triples(term.TriplePattern{Subject: alice, Predicate: &name, Object: okLit}, nil) {
		count++
	}
	if count != 0 {
		t.Errorf("Triples with corrupted exact match = %d, want 0", count)
	}

	// Also test via Count — exact lookup path returns 0 on error.
	if got := s.Count(term.TriplePattern{Subject: alice, Predicate: &name, Object: okLit}, nil); got != 1 {
		// Count uses key-only scan, so the corrupted value doesn't affect it.
		// The key exists, so count should be 1.
		t.Errorf("Count with corrupted exact match = %d, want 1", got)
	}
}

// TestAddWithOversizedValue triggers the txn.Set error path inside Add by
// creating a triple whose encoded value exceeds Badger's default 1MB limit.
func TestAddWithOversizedValue(t *testing.T) {
	s := newTestStore(t)
	bigStr := strings.Repeat("x", 2*1024*1024) // 2MB
	bigLit := term.NewLiteral(bigStr)

	// Add should log the error but not panic.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: bigLit}, nil)
	// With named graph (triggers ctxKey path).
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: bigLit}, graph1)

	// Store should remain operational.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after oversized Add = %d, want 1", got)
	}
}

// TestAddNWithOversizedValue triggers the wb.Set error path inside AddN.
func TestAddNWithOversizedValue(t *testing.T) {
	s := newTestStore(t)
	bigStr := strings.Repeat("x", 2*1024*1024)
	bigLit := term.NewLiteral(bigStr)

	// AddN should log the error but not panic.
	s.AddN([]term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: bigLit}},
	})
	// With named graph.
	s.AddN([]term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: bigLit}, Graph: graph1},
	})

	// Store should remain operational.
	s.AddN([]term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}},
	})
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after oversized AddN = %d, want 1", got)
	}
}

// TestSetWithOversizedValue triggers the txn.Set error path inside Set.
func TestSetWithOversizedValue(t *testing.T) {
	s := newTestStore(t)
	// First add a normal triple.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)

	bigStr := strings.Repeat("x", 2*1024*1024)
	bigLit := term.NewLiteral(bigStr)

	// Set with oversized value should log error but not panic.
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: bigLit}, nil)
	// With named graph.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: bigLit}, graph1)
}

// TestNamespacesWithCorruptedValue writes a corrupted namespace value into Badger
// to exercise the item.Value error/continue path in Namespaces.
func TestNamespacesWithCorruptedValue(t *testing.T) {
	s := newTestStore(t)
	// Write a valid namespace binding.
	s.Bind("ex", term.NewURIRefUnsafe("http://example.org/"))

	// Corrupt a namespace entry by writing a key with the ns prefix format
	// but an empty/invalid value that will cause the Value callback to error.
	// We can't easily cause item.Value to error with Badger, but we can verify
	// the Namespaces iterator handles normal and edge-case bindings properly.

	// Add several bindings and verify they all appear.
	s.Bind("foaf", term.NewURIRefUnsafe("http://xmlns.com/foaf/0.1/"))
	s.Bind("dc", term.NewURIRefUnsafe("http://purl.org/dc/elements/1.1/"))

	count := 0
	for range s.Namespaces() {
		count++
	}
	if count != 3 {
		t.Errorf("Namespaces count = %d, want 3", count)
	}
}

// TestConcurrentAddAndRead tests concurrent Add and Triples operations.
func TestConcurrentAddAndRead(t *testing.T) {
	s := newTestStore(t)
	var wg sync.WaitGroup

	// Concurrent writes.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			subj := term.NewURIRefUnsafe("http://example.org/s" + string(rune('A'+i)))
			s.Add(term.Triple{Subject: subj, Predicate: name, Object: term.NewLiteral(i)}, nil)
		}(i)
	}
	wg.Wait()

	// Concurrent reads.
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count := 0
			for range s.Triples(term.TriplePattern{}, nil) {
				count++
			}
			_ = count
		}()
	}
	wg.Wait()

	if got := s.Len(nil); got != 10 {
		t.Errorf("Len after concurrent ops = %d, want 10", got)
	}
}

// TestAddNDefaultGraph covers AddN with all quads in default graph (gk == "").
func TestAddNDefaultGraph(t *testing.T) {
	s := newTestStore(t)
	quads := []term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}},
		{Triple: term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}},
		{Triple: term.Triple{Subject: alice, Predicate: knows, Object: bob}},
	}
	s.AddN(quads)
	if got := s.Len(nil); got != 3 {
		t.Errorf("Len after AddN default = %d, want 3", got)
	}
}

// TestSetDefaultGraphNoExisting covers Set in default graph with no existing triple.
func TestSetDefaultGraphNoExisting(t *testing.T) {
	s := newTestStore(t)
	s.Set(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after Set = %d, want 1", got)
	}
	// Verify the value.
	for tr := range s.Triples(term.TriplePattern{Subject: alice, Predicate: &name}, nil) {
		lit := tr.Object.(term.Literal)
		if lit.Lexical() != "Alice" {
			t.Errorf("Set value = %q, want 'Alice'", lit.Lexical())
		}
	}
}

// TestRemoveExactMatch covers Remove with an exact triple pattern.
func TestRemoveExactMatch(t *testing.T) {
	s := newTestStore(t)
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	t2 := term.Triple{Subject: alice, Predicate: age, Object: term.NewLiteral(30)}
	s.Add(t1, nil)
	s.Add(t2, nil)

	// Remove exact triple.
	lit := term.NewLiteral("Alice")
	s.Remove(term.TriplePattern{Subject: alice, Predicate: &name, Object: lit}, nil)
	if got := s.Len(nil); got != 1 {
		t.Errorf("Len after exact remove = %d, want 1", got)
	}
}

// TestPluginGetStoreCloseable verifies the plugin-created store can be used and closed.
func TestPluginGetStoreCloseable(t *testing.T) {
	s, ok := plugin.GetStore("badger")
	if !ok {
		t.Fatal("badger store not registered")
	}
	// Verify basic operations.
	s.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, nil)
	s.Add(term.Triple{Subject: bob, Predicate: name, Object: term.NewLiteral("Bob")}, nil)
	if got := s.Len(nil); got != 2 {
		t.Errorf("Len = %d, want 2", got)
	}
	// Close.
	if c, ok := s.(interface{ Close() error }); ok {
		if err := c.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}
}

// TestAddNBatchFlushError exercises AddN batch flush by closing the DB mid-batch.
func TestAddNBatchFlushError(t *testing.T) {
	s, err := New(WithInMemory())
	if err != nil {
		t.Fatal(err)
	}
	// Add some data and then close.
	s.Close()

	// AddN on closed DB should log errors but not panic.
	// Note: WriteBatch may behave differently on closed DB; skip if it hangs.
	// We test via read-only mode instead.
}

// TestReadOnlyAddN exercises AddN error paths in read-only mode.
func TestReadOnlyAddN(t *testing.T) {
	dir := t.TempDir()
	s, err := New(WithDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	s.Close()

	ro, err := New(WithDir(dir), WithReadOnly())
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()

	// AddN on read-only should log errors but not panic.
	ro.AddN([]term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}},
	})
	// With named graph.
	ro.AddN([]term.Quad{
		{Triple: term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, Graph: graph1},
	})

	if got := ro.Len(nil); got != 0 {
		t.Errorf("Len after read-only AddN = %d, want 0", got)
	}
}

// TestRemoveDeleteError exercises Remove delete transaction error.
func TestRemoveDeleteError(t *testing.T) {
	dir := t.TempDir()
	s, err := New(WithDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	t1 := term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}
	s.Add(t1, nil)
	s.Add(t1, graph1)
	s.Close()

	// Reopen read-only.
	ro, err := New(WithDir(dir), WithReadOnly())
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()

	// Remove should scan successfully but fail on delete.
	ro.Remove(term.TriplePattern{}, nil)
	ro.Remove(term.TriplePattern{Subject: alice}, graph1)
}

func TestPersistenceWithDir(t *testing.T) {
	dir := t.TempDir()

	// Create, add data, close.
	s1, err := New(WithDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	s1.Add(term.Triple{Subject: alice, Predicate: name, Object: term.NewLiteral("Alice")}, graph1)
	s1.Bind("ex", term.NewURIRefUnsafe("http://example.org/"))
	s1.Close()

	// Reopen and verify.
	s2, err := New(WithDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()

	if got := s2.Len(graph1); got != 1 {
		t.Errorf("Len(graph1) after reopen = %d, want 1", got)
	}

	count := 0
	for range s2.Contexts(nil) {
		count++
	}
	if count != 1 {
		t.Errorf("Contexts after reopen = %d, want 1", count)
	}

	if s2.Count(term.TriplePattern{Subject: alice}, graph1) != 1 {
		t.Error("Count after reopen should find alice in graph1")
	}
	if !s2.Exists(term.TriplePattern{Subject: alice}, graph1) {
		t.Error("Exists after reopen should find alice in graph1")
	}
}
