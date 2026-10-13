package graph_test

import (
	"testing"

	"github.com/tggo/goRDFlib/graph"
	"github.com/tggo/goRDFlib/namespace"
	"github.com/tggo/goRDFlib/store"
	"github.com/tggo/goRDFlib/term"
)

// ─── graph.go ───────────────────────────────────────────────────────────────

func TestNewGraphFromStore(t *testing.T) {
	s := store.NewMemoryStore()
	id, _ := term.NewURIRef("http://example.org/g")
	g := graph.NewGraphFromStore(s, id)
	if g.Store() != s {
		t.Error("wrong store")
	}
	if g.Identifier().(term.URIRef) != id {
		t.Error("wrong identifier")
	}
	// NewGraphFromStore should not add default namespace bindings, so Len starts 0.
	if mustV(g.Len(t.Context())) != 0 {
		t.Errorf("expected empty graph, got %d", mustV(g.Len(t.Context())))
	}
}

func TestGraphBase(t *testing.T) {
	g := graph.NewGraph(graph.WithBase("http://base.example.org/"))
	if g.Base() != "http://base.example.org/" {
		t.Errorf("unexpected base: %q", g.Base())
	}
}

func TestGraphBaseEmpty(t *testing.T) {
	g := graph.NewGraph()
	if g.Base() != "" {
		t.Errorf("expected empty base, got %q", g.Base())
	}
}

// PredicateObjects - currently 0% covered.
func TestGraphPredicateObjects(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p1, _ := term.NewURIRef("http://example.org/p1")
	p2, _ := term.NewURIRef("http://example.org/p2")
	g.Add(t.Context(), s, p1, term.NewLiteral("a"))
	g.Add(t.Context(), s, p2, term.NewLiteral("b"))

	count := 0
	g.PredicateObjects(t.Context(), s)(func(pair_ store.TermPair, _ error) bool {
		count++
		return true
	})
	if count != 2 {
		t.Errorf("expected 2 predicate-object pairs, got %d", count)
	}
}

func TestGraphPredicateObjectsEarlyStop(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p1, _ := term.NewURIRef("http://example.org/p1")
	p2, _ := term.NewURIRef("http://example.org/p2")
	g.Add(t.Context(), s, p1, term.NewLiteral("a"))
	g.Add(t.Context(), s, p2, term.NewLiteral("b"))

	count := 0
	g.PredicateObjects(t.Context(), s)(func(pair_ store.TermPair, _ error) bool {
		count++
		return false // stop after first
	})
	if count != 1 {
		t.Errorf("expected early stop after 1, got %d", count)
	}
}

// Dedup: same predicate+object added twice should appear once.
func TestGraphPredicateObjectsDedup(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	o := term.NewLiteral("v")
	g.Add(t.Context(), s, p, o)
	// Add same triple again — underlying store de-dupes or not, but our iterator dedupes.
	g.Add(t.Context(), s, p, o)

	count := 0
	g.PredicateObjects(t.Context(), s)(func(pair_ store.TermPair, _ error) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected 1 (deduplicated), got %d", count)
	}
}

// Subjects/Predicates/Objects with duplicate suppression.
func TestGraphSubjectsDuplicateSuppression(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	o1 := term.NewLiteral("a")
	o2 := term.NewLiteral("b")
	g.Add(t.Context(), s, p, o1)
	g.Add(t.Context(), s, p, o2)

	// Both triples have the same subject; Subjects() should yield it once.
	count := 0
	g.Subjects(t.Context(), &p, nil)(func(term.Term, error) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected 1 unique subject, got %d", count)
	}
}

func TestGraphPredicatesDuplicateSuppression(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, term.NewLiteral("a"))
	g.Add(t.Context(), s, p, term.NewLiteral("b"))

	count := 0
	g.Predicates(t.Context(), s, nil)(func(term.Term, error) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected 1 unique predicate, got %d", count)
	}
}

func TestGraphObjectsDuplicateSuppression(t *testing.T) {
	g := graph.NewGraph()
	s1, _ := term.NewURIRef("http://example.org/s1")
	s2, _ := term.NewURIRef("http://example.org/s2")
	p, _ := term.NewURIRef("http://example.org/p")
	o := term.NewLiteral("same")
	g.Add(t.Context(), s1, p, o)
	g.Add(t.Context(), s2, p, o)

	count := 0
	g.Objects(t.Context(), nil, &p)(func(term.Term, error) bool { count++; return true })
	if count != 1 {
		t.Errorf("expected 1 unique object, got %d", count)
	}
}

func TestGraphSubjectPredicatesEarlyStop(t *testing.T) {
	g := graph.NewGraph()
	s1, _ := term.NewURIRef("http://example.org/s1")
	s2, _ := term.NewURIRef("http://example.org/s2")
	p, _ := term.NewURIRef("http://example.org/p")
	o, _ := term.NewURIRef("http://example.org/o")
	g.Add(t.Context(), s1, p, o)
	g.Add(t.Context(), s2, p, o)

	count := 0
	g.SubjectPredicates(t.Context(), o)(func(pair_ store.TermPair, _ error) bool { count++; return false })
	if count != 1 {
		t.Errorf("expected early stop at 1, got %d", count)
	}
}

func TestGraphSubjectObjectsEarlyStop(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(t.Context(), s, p, term.NewLiteral("a"))
	g.Add(t.Context(), s, p, term.NewLiteral("b"))

	count := 0
	g.SubjectObjects(t.Context(), &p)(func(pair_ store.TermPair, _ error) bool { count++; return false })
	if count != 1 {
		t.Errorf("expected early stop at 1, got %d", count)
	}
}

func TestGraphValueAllNil(t *testing.T) {
	g := graph.NewGraph()
	_, ok, _ := g.Value(t.Context(), nil, nil, nil)
	if ok {
		t.Error("all-nil should return false")
	}
}

func TestGraphValueAllNonNil(t *testing.T) {
	g := graph.NewGraph()
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	o := term.NewLiteral("v")
	_, ok, _ := g.Value(t.Context(), s, &p, o)
	if ok {
		t.Error("all-non-nil should return false")
	}
}

// ─── resource.go ────────────────────────────────────────────────────────────

func TestResourceBasic(t *testing.T) {
	g := graph.NewGraph()
	id, _ := term.NewURIRef("http://example.org/thing")
	r := graph.NewResource(g, id)

	if r.Graph() != g {
		t.Error("wrong graph")
	}
	if r.Identifier().(term.URIRef) != id {
		t.Error("wrong identifier")
	}
}

func TestResourceAdd(t *testing.T) {
	g := graph.NewGraph()
	id, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	r := graph.NewResource(g, id)
	r.Add(t.Context(), p, term.NewLiteral("hello"))

	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1 triple, got %d", mustV(g.Len(t.Context())))
	}
}

func TestResourceRemove(t *testing.T) {
	g := graph.NewGraph()
	id, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	r := graph.NewResource(g, id)
	r.Add(t.Context(), p, term.NewLiteral("hello"))
	r.Remove(t.Context(), p, term.NewLiteral("hello"))

	if mustV(g.Len(t.Context())) != 0 {
		t.Errorf("expected 0 after remove, got %d", mustV(g.Len(t.Context())))
	}
}

func TestResourceSet(t *testing.T) {
	g := graph.NewGraph()
	id, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	r := graph.NewResource(g, id)
	r.Add(t.Context(), p, term.NewLiteral("old"))
	r.Set(t.Context(), p, term.NewLiteral("new"))

	if mustV(g.Len(t.Context())) != 1 {
		t.Errorf("expected 1 after set, got %d", mustV(g.Len(t.Context())))
	}
	val, ok, _ := r.Value(t.Context(), p)
	if !ok || val.N3() != term.NewLiteral("new").N3() {
		t.Errorf("unexpected value: %v", val)
	}
}

func TestResourceObjects(t *testing.T) {
	g := graph.NewGraph()
	id, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	r := graph.NewResource(g, id)
	r.Add(t.Context(), p, term.NewLiteral("a"))
	r.Add(t.Context(), p, term.NewLiteral("b"))

	count := 0
	r.Objects(t.Context(), p)(func(term.Term, error) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestResourceSubjects(t *testing.T) {
	g := graph.NewGraph()
	id, _ := term.NewURIRef("http://example.org/o")
	s1, _ := term.NewURIRef("http://example.org/s1")
	s2, _ := term.NewURIRef("http://example.org/s2")
	p, _ := term.NewURIRef("http://example.org/p")
	g.Add(t.Context(), s1, p, id)
	g.Add(t.Context(), s2, p, id)

	r := graph.NewResource(g, id)
	count := 0
	r.Subjects(t.Context(), p)(func(term.Term, error) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

func TestResourceValue(t *testing.T) {
	g := graph.NewGraph()
	id, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	r := graph.NewResource(g, id)
	r.Add(t.Context(), p, term.NewLiteral("v"))

	val, ok, _ := r.Value(t.Context(), p)
	if !ok {
		t.Fatal("expected a value")
	}
	if val.N3() != term.NewLiteral("v").N3() {
		t.Errorf("unexpected value: %v", val)
	}
}

func TestResourceValueNotFound(t *testing.T) {
	g := graph.NewGraph()
	id, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")
	r := graph.NewResource(g, id)

	_, ok, _ := r.Value(t.Context(), p)
	if ok {
		t.Error("expected not found")
	}
}

func TestResourcePredicateObjects(t *testing.T) {
	g := graph.NewGraph()
	id, _ := term.NewURIRef("http://example.org/s")
	p1, _ := term.NewURIRef("http://example.org/p1")
	p2, _ := term.NewURIRef("http://example.org/p2")
	r := graph.NewResource(g, id)
	r.Add(t.Context(), p1, term.NewLiteral("a"))
	r.Add(t.Context(), p2, term.NewLiteral("b"))

	count := 0
	r.PredicateObjects(t.Context())(func(pair_ store.TermPair, _ error) bool { count++; return true })
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}
}

// ─── collection.go ──────────────────────────────────────────────────────────

func TestCollectionIndex(t *testing.T) {
	g := graph.NewGraph()
	col := graph.NewEmptyCollection(g)
	a := term.NewLiteral("apple")
	b := term.NewLiteral("banana")
	c := term.NewLiteral("cherry")
	col.Append(t.Context(), a)
	col.Append(t.Context(), b)
	col.Append(t.Context(), c)

	if mustV(col.Index(t.Context(), a)) != 0 {
		t.Errorf("expected 0, got %d", mustV(col.Index(t.Context(), a)))
	}
	if mustV(col.Index(t.Context(), b)) != 1 {
		t.Errorf("expected 1, got %d", mustV(col.Index(t.Context(), b)))
	}
	if mustV(col.Index(t.Context(), c)) != 2 {
		t.Errorf("expected 2, got %d", mustV(col.Index(t.Context(), c)))
	}
}

func TestCollectionIndexNotFound(t *testing.T) {
	g := graph.NewGraph()
	col := graph.NewEmptyCollection(g)
	col.Append(t.Context(), term.NewLiteral("a"))

	if mustV(col.Index(t.Context(), term.NewLiteral("nope"))) != -1 {
		t.Error("expected -1 for missing item")
	}
}

func TestCollectionIndexEmpty(t *testing.T) {
	g := graph.NewGraph()
	col := graph.NewEmptyCollection(g)
	if mustV(col.Index(t.Context(), term.NewLiteral("x"))) != -1 {
		t.Error("expected -1 for empty collection")
	}
}

func TestCollectionClear(t *testing.T) {
	g := graph.NewGraph()
	col := graph.NewEmptyCollection(g)
	col.Append(t.Context(), term.NewLiteral("a"))
	col.Append(t.Context(), term.NewLiteral("b"))
	col.Append(t.Context(), term.NewLiteral("c"))

	if mustV(col.Len(t.Context())) != 3 {
		t.Fatalf("precondition: expected 3, got %d", mustV(col.Len(t.Context())))
	}

	col.Clear(t.Context())

	if mustV(col.Len(t.Context())) != 0 {
		t.Errorf("expected 0 after clear, got %d", mustV(col.Len(t.Context())))
	}
}

func TestCollectionClearEmpty(t *testing.T) {
	g := graph.NewGraph()
	col := graph.NewEmptyCollection(g)
	col.Clear(t.Context()) // should not panic
}

func TestCollectionGetInBounds(t *testing.T) {
	g := graph.NewGraph()
	col := graph.NewEmptyCollection(g)
	a := term.NewLiteral("x")
	b := term.NewLiteral("y")
	col.Append(t.Context(), a)
	col.Append(t.Context(), b)

	v0, ok0, _ := col.Get(t.Context(), 0)
	if !ok0 || v0.N3() != a.N3() {
		t.Errorf("Get(0): expected %v, got %v ok=%v", a, v0, ok0)
	}
	v1, ok1, _ := col.Get(t.Context(), 1)
	if !ok1 || v1.N3() != b.N3() {
		t.Errorf("Get(1): expected %v, got %v ok=%v", b, v1, ok1)
	}
}

func TestCollectionIterEmptyList(t *testing.T) {
	g := graph.NewGraph()
	col := graph.NewEmptyCollection(g)
	count := 0
	col.Iter(t.Context())(func(term.Term, error) bool { count++; return true })
	if count != 0 {
		t.Errorf("expected 0, got %d", count)
	}
}

// Test getContainer with cycle detection: create a malformed circular list.
func TestCollectionGetContainerCycle(t *testing.T) {
	g := graph.NewGraph()
	n1 := term.NewBNode()
	n2 := term.NewBNode()
	// n1 -> n2 -> n1 (cycle)
	g.Add(t.Context(), n1, namespace.RDF.First, term.NewLiteral("first"))
	g.Add(t.Context(), n1, namespace.RDF.Rest, n2)
	g.Add(t.Context(), n2, namespace.RDF.First, term.NewLiteral("second"))
	g.Add(t.Context(), n2, namespace.RDF.Rest, n1) // cycle

	col := graph.NewCollection(g, n1)
	// Index 3 requires traversing past the cycle; should not hang, returns nil.
	_, ok, _ := col.Get(t.Context(), 3)
	if ok {
		t.Error("expected false for out-of-bounds on cyclic list")
	}
}

// Test end() cycle detection: create circular list, Clear should not loop.
func TestCollectionClearCycle(t *testing.T) {
	g := graph.NewGraph()
	n1 := term.NewBNode()
	n2 := term.NewBNode()
	g.Add(t.Context(), n1, namespace.RDF.First, term.NewLiteral("first"))
	g.Add(t.Context(), n1, namespace.RDF.Rest, n2)
	g.Add(t.Context(), n2, namespace.RDF.First, term.NewLiteral("second"))
	g.Add(t.Context(), n2, namespace.RDF.Rest, n1) // cycle

	col := graph.NewCollection(g, n1)
	col.Clear(t.Context()) // must not loop
}

// ─── conjunctive.go ─────────────────────────────────────────────────────────

func TestConjunctiveGraphWithStore(t *testing.T) {
	s := store.NewMemoryStore()
	cg := graph.NewConjunctiveGraph(graph.WithStore(s))
	if cg.Store() != s {
		t.Error("WithStore not applied to ConjunctiveGraph")
	}
}

func TestConjunctiveGraphWithIdentifier(t *testing.T) {
	id, _ := term.NewURIRef("http://example.org/ctx")
	cg := graph.NewConjunctiveGraph(graph.WithIdentifier(id))
	if cg.DefaultContext().Identifier().(term.URIRef) != id {
		t.Error("WithIdentifier not applied to ConjunctiveGraph")
	}
}

func TestConjunctiveGraphGetContextNonNil(t *testing.T) {
	cg := graph.NewConjunctiveGraph()
	id, _ := term.NewURIRef("http://example.org/g1")
	ctx := cg.GetContext(id)
	if ctx == nil {
		t.Fatal("expected non-nil graph")
	}
	if ctx.Identifier().(term.URIRef) != id {
		t.Errorf("wrong identifier: %v", ctx.Identifier())
	}
}

// ─── dataset.go ─────────────────────────────────────────────────────────────

func TestDatasetRemoveGraphNonExistent(t *testing.T) {
	ds := graph.NewDataset()
	id, _ := term.NewURIRef("http://example.org/nonexistent")
	// should not panic
	ds.RemoveGraph(t.Context(), id)
}

func TestDatasetRemoveGraphExisting(t *testing.T) {
	ds := graph.NewDataset()
	id, _ := term.NewURIRef("http://example.org/g1")
	s, _ := term.NewURIRef("http://example.org/s")
	p, _ := term.NewURIRef("http://example.org/p")

	ctx := ds.Graph(id)
	ctx.Add(t.Context(), s, p, term.NewLiteral("v"))

	// Confirm triple is there
	if mustV(ctx.Len(t.Context())) != 1 {
		t.Fatalf("precondition: expected 1, got %d", mustV(ctx.Len(t.Context())))
	}

	ds.RemoveGraph(t.Context(), id)

	// After removal the named graph should be gone from ds.Graphs()
	count := 0
	ds.Graphs()(func(g *graph.Graph) bool {
		if u, ok := g.Identifier().(term.URIRef); ok && u == id {
			count++
		}
		return true
	})
	if count != 0 {
		t.Error("removed graph still appears in Graphs()")
	}
}

func TestDatasetGraphsEarlyStop(t *testing.T) {
	ds := graph.NewDataset()
	g1, _ := term.NewURIRef("http://example.org/g1")
	g2, _ := term.NewURIRef("http://example.org/g2")
	ds.Graph(g1)
	ds.Graph(g2)

	count := 0
	ds.Graphs()(func(*graph.Graph) bool { count++; return false })
	if count != 1 {
		t.Errorf("expected early stop at 1, got %d", count)
	}
}

func TestDatasetWithOptions(t *testing.T) {
	s := store.NewMemoryStore()
	id, _ := term.NewURIRef("http://example.org/default")
	ds := graph.NewDataset(graph.WithStore(s), graph.WithIdentifier(id))
	if ds.Store() != s {
		t.Error("wrong store")
	}
	if ds.DefaultContext().Identifier().(term.URIRef) != id {
		t.Error("wrong default context identifier")
	}
}
